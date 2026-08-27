# TradeForge: Architecture and Design Document

A distributed benchmarking and hosting platform that evaluates contestant-submitted
trading infrastructure under realistic, high-velocity market load. Contestants upload
an order book implementation, the platform compiles it into a hardened sandbox,
drives it with a fleet of simulated trading bots, validates every fill against an
authoritative reference matching engine, and ranks all contestants on a live
leaderboard scored by throughput, tail latency, and correctness.

This document describes the system design, the rationale behind each major decision,
the data contracts between services, the failure modes the platform is built to
survive, and the assumptions the design rests on. It is meant to be read by an
engineer who needs to operate, extend, or audit the platform, not as marketing.

> For a file-by-file verification of the implementation, see
> [docs/AUDIT.md](docs/AUDIT.md).

---

## 1. System Overview

The platform is a polyglot, event-driven set of microservices written in Go, fronted
by a React single-page application. The unit of work is a "test": a single contestant
submission is loaded into an isolated container, subjected to a bounded burst of
concurrent order traffic, and scored. Many tests can run concurrently, one isolated
container per contestant.

The design has three hard requirements that shape every component:

1. **Results must be trustworthy.** A contestant controls the code running inside the
   sandbox and is motivated to cheat. The platform never trusts the contestant's
   self-reported fills. Correctness is decided by a reference matching engine the
   platform owns, not by the submission.

2. **The platform must survive its own load.** The bot fleet deliberately generates
   peak market volatility. The measurement and scoring path cannot fall behind, run
   out of memory, or lose data when a single pod restarts. Backpressure and bounded
   memory are first-class concerns, not afterthoughts.

3. **A compromised submission must stay contained.** A contestant who escapes the
   sandbox must not be able to reach Kafka, Redis, the databases, or other
   contestants. Isolation is layered so that a single bypassed control is not a full
   breach.

The platform is operable as a single-host Docker Compose stack for development and as
a horizontally scaled Kubernetes deployment for a live contest.

---

## 2. High-Level Architecture

The services and their responsibilities:

| Service | Language | Responsibility |
|---|---|---|
| submission-api | Go | Authenticated REST ingress. Accepts uploads, streams them to object storage, records submissions, queues build jobs, serves the read path for the leaderboard and contestant-facing data. |
| build-worker | Go | Consumes build jobs, compiles a submission inside a multi-stage Docker build, launches a hardened sandbox container, waits for its health check, and reports the result. |
| orchestrator | Go | Owns the test lifecycle state machine. Drives the transition from pending to running to completed or failed, writes heartbeats for crash recovery, and triggers final scoring. |
| bot-fleet | Go | Spawns the load generator. Multiple bot personas send concurrent orders and cancels to the contestant container and emit per-order telemetry. |
| telemetry-ingester | Go | Consumes the telemetry firehose, replays orders through the authoritative reference order book to judge correctness, computes latency percentiles and throughput, and persists history and live metrics. |
| leaderboard-api | Go | Computes the composite score, ranks contestants, and fans the ranked leaderboard out to every connected browser over WebSocket. |
| frontend | React, TypeScript | Live leaderboard, submission flow, results breakdown, progress and prediction view, and the operations console. |

The supporting infrastructure: Apache Kafka (or Redpanda) for the durable event
backbone, Redis for live metrics and pub/sub fan-out, TimescaleDB for time-series
history, PostgreSQL for submission and test state, and MinIO (or S3) for uploaded
artifacts.

Communication style is deliberately mixed. The durable, high-volume paths
(build jobs, telemetry, test control) run over Kafka, because at-least-once delivery
and replay are required for trustworthy scoring. The live read path
(leaderboard updates) runs over Redis pub/sub plus WebSocket, because freshness
matters more than durability there and a dropped frame is corrected by the next tick.

---

## 3. End-to-End Data Flow

A submission travels through the system in the following order.

1. **Upload.** A contestant sends a multipart POST to the submission-api with their
   API key. The handler validates the language and filename, enforces minimum and
   maximum upload sizes, streams the bytes to object storage without buffering the
   whole file in memory, writes a submission row to PostgreSQL with status
   `pending`, and publishes a `BuildJob` to the `build-jobs` Kafka topic. The contestant
   receives a submission id to poll.

2. **Build.** A build-worker consumes the `BuildJob`, downloads the artifact from
   object storage, inspects the archive for path traversal and decompression-bomb
   patterns before extraction, extracts it with a hard per-entry byte cap, and runs a
   language-specific multi-stage Docker build. The resulting image is scanned for
   high and critical CVEs. If the image is clean, the worker launches a hardened
   sandbox container and waits for the contestant's health endpoint to return success.
   The worker publishes a `BuildResult` with the sandbox container address and marks
   the submission `ready` or `failed`.

3. **Test start.** The contestant (or an operator) starts a test. The orchestrator
   creates a test row, transitions it to `running`, resolves the sandbox address, and
   publishes a `StartTest` event to the bot fleet. The orchestrator begins writing a
   heartbeat for that test so a second orchestrator instance can recover it if this
   one dies.

4. **Load generation.** The bot fleet receives `StartTest`, calls the contestant's
   reset endpoint so the contestant book and the reference book both begin empty,
   then spawns the configured number of bots split across personas. Each bot sends
   orders and cancels over a shared, warm HTTP connection pool, measures the HTTP
   round-trip latency for each request, and emits an `OrderEvent` to the
   `bot-telemetry` Kafka topic.

5. **Ingestion and validation.** The telemetry-ingester consumes `bot-telemetry`,
   holds events briefly in a reorder buffer, sorts each batch by sequence number,
   and replays the orders for each contestant through the authoritative reference
   order book. The reference decides what the correct fill should have been, and the
   ingester compares that to the fill the contestant actually returned. It records
   latency into a bounded histogram, increments a throughput counter, writes raw
   samples to TimescaleDB in bulk, and publishes a rolling per-contestant metric
   snapshot to Redis once per second.

6. **Scoring and fan-out.** The leaderboard-api reads the per-contestant metric
   snapshots from Redis on a fixed interval, computes a composite score with min-max
   normalization across the active field, ranks the contestants, and publishes the
   ranked leaderboard to a Redis pub/sub channel. Every leaderboard-api pod subscribes
   to that channel and fans the update out to its connected WebSocket clients, so the
   read path scales horizontally behind a load balancer.

7. **Test completion.** When the duration elapses, the bot fleet stops, the
   orchestrator transitions the test to `completed`, writes a final per-test summary
   row, and stops the heartbeat. The leaderboard reflects the contestant's final
   numbers.

---

## 4. The Sandbox Engine

The sandbox is the single most security-sensitive component, because it runs
arbitrary contestant code. The threat model assumes the contestant is hostile: they
may try to escape the container, reach internal infrastructure, exhaust shared
resources, or cheat the scoring.

Isolation is layered so that no single control is the only thing standing between a
malicious submission and the rest of the platform.

1. **Network isolation (primary control).** The contestant container is attached only
   to an internal Docker network that has no route off the host (`internal: true` in
   Compose) or, on Kubernetes, a namespace whose default-deny NetworkPolicy blocks all
   egress and allows ingress only from the bot fleet on the order-book port. The bot
   fleet and build-worker are dual-homed: they sit on both the internal contestant
   network (to reach the sandbox by container address) and the platform network (to
   reach Kafka and the databases). The contestant container is never placed on the
   platform network. This is the control that matters most: even if every other layer
   is bypassed, a compromised container still cannot reach Kafka, Redis, PostgreSQL,
   or MinIO, because there is no network path.

2. **seccomp.** The container runs under a seccomp profile whose default action is to
   return an error for any syscall not on an explicit allow list sized for a network
   server. Dangerous syscalls (process creation beyond the configured limit, ptrace,
   mount, kernel module operations, and roughly two hundred others) are denied. The
   launcher loads the profile as required configuration. If the profile cannot be read,
   the launch fails closed rather than silently running without a profile.

3. **AppArmor.** A mandatory-access-control profile denies writes outside the writable
   temporary mount, denies reads and writes to the host process and system
   pseudo-filesystems, and denies ptrace. The profile is applied to the container at
   launch.

4. **Linux capabilities and privilege.** The container drops all capabilities, runs
   with `no-new-privileges`, runs as an unprivileged user rather than root, and uses a
   read-only root filesystem with a single small, non-executable, size-capped
   temporary mount for scratch space.

5. **Resource caps.** Memory and swap are capped (so the container cannot consume host
   memory), CPU is pinned to a fixed core set with a hard quota (so one contestant
   cannot starve another), and the process count is capped (so fork bombs are bounded).

6. **Image scanning.** The built image is scanned for high and critical CVEs before
   the sandbox is allowed to start. A critical finding blocks the launch.

7. **Runtime monitoring.** A resource monitor watches the container and kills it if it
   becomes unhealthy or abusive. Any outbound traffic on a port other than the
   order-book port is treated as a red flag.

The build itself is also isolated: the compile step runs in a multi-stage Docker build
so that build-time tools never reach the runtime image, and the final image carries
only the compiled artifact and its minimal runtime.

---

## 5. Latency Measurement

Latency is measured as the wall-clock HTTP round trip from the moment a bot writes the
request to the moment it reads the acknowledgment. The bot captures a high-resolution
timestamp immediately before the send and immediately after the ack, and the
difference in microseconds is the recorded latency. This measures exactly what the
contest cares about: how fast the contestant's server acknowledges an order under
load, including the contestant's own queuing and lock contention.

Measurement runs in the bot, not inside the sandbox, so the contestant cannot influence
the timer. The shared, pre-warmed connection pool means the handshake cost is paid once
and is not charged to each order, so the recorded latency reflects request handling
rather than connection setup.

The platform deliberately does not instrument the contestant's process internals. The
order-book API contract is a black box: the contestant exposes endpoints, and the
platform measures and validates only what crosses that boundary. This keeps the
contract language-agnostic and prevents the platform from depending on contestant
cooperation for honest measurement.

---

## 6. Bot Fleet

The bot fleet is the load generator. It spawns a configurable number of bots, split
across personas that together approximate a realistic mix of market participants:

- **Market maker** posts symmetric two-sided limit quotes around a drifting mid price
  and cancels and re-quotes on a short cycle. This is the source of resting liquidity.
- **Aggressive taker** crosses the spread with marketable orders, consuming the
  liquidity the makers post.
- **Spammer** sends a high rate of small orders and cancels to stress throughput and
  lock contention.
- **Whale** sends occasional very large orders that sweep multiple price levels, to
  test multi-level matching and partial-fill handling.
- **FIX bot** speaks a self-contained FIX 4.2 message format with correct body length,
  checksum, and sequence numbers, to exercise a protocol path beyond plain REST.

All bots targeting one contestant share a single tuned HTTP transport. A new client per
bot would exhaust file descriptors and add a handshake to every measurement, so the
transport is created once per test run, with bounded idle and per-host connection
limits, and shared by every bot goroutine.

A per-target circuit breaker protects both the platform and the measurement quality. If
the contestant's server starts failing, the breaker opens after a failure-rate
threshold is crossed, rejects requests fast for a cooldown, then probes recovery in a
half-open state. The breaker has a minimum-traffic guard so that a server which is
merely slow to warm up does not trip it before the test has really begun.

Each bot run is bounded by the test duration and is cancellable. On a stop signal the
bot fleet cancels all bot goroutines and waits for them to drain.

---

## 7. Telemetry and Validation

This is where trustworthiness is enforced. The ingester has two jobs that must not be
conflated: measuring performance, and judging correctness.

### 7.1 Reordering

Telemetry from distributed bots does not arrive at the ingester in any meaningful order.
Kafka preserves order only within a partition, and events for one contestant are spread
across partitions and bots. Before judging correctness, the ingester holds incoming
events in a short reorder window and sorts each flush by sequence number, so that the
reference replay sees a stable, deterministic order rather than raw arrival order.

The reorder window is a deliberate latency-for-determinism trade. A longer window gives
a more complete ordering at the cost of freshness; a shorter window keeps the
leaderboard live at the cost of occasionally replaying a late event out of order.

### 7.2 Authoritative reference order book

The reference order book is a price-time-priority matching engine the platform owns.
For each contestant the ingester maintains one reference book and replays that
contestant's orders through it in sequence order. The reference book is reset whenever a
new test begins, mirroring the reset the bot fleet issues to the contestant, so both
books start from the same empty state.

The reference is the definition of correct. The contestant's self-reported fill is
treated as a claim to be checked, never as ground truth. This is what makes the score
resistant to cheating: a submission that fabricates fills is compared against what
should have happened and is marked incorrect.

### 7.3 Correctness comparison and concurrency

There is an inherent tension the design must acknowledge honestly. The contestant's
server processes many concurrent requests, and the true order in which it processed
interleaved orders from different bots is not observable from outside. A single
linear reference replay cannot perfectly reproduce a concurrent server's internal
ordering. The comparison therefore distinguishes two kinds of disagreement:

- **Hard violations**, which no legitimate implementation can produce regardless of
  internal ordering: an overfill (filling more quantity than existed), a fill at a
  price that never existed in the book, a negative quantity, or a claimed fill on an
  order that could not have crossed. These are always marked incorrect.
- **Ordering-sensitive disagreements**, where the reference and the contestant differ
  only because of legitimate concurrency (for example, a resting order that the
  reference had not yet placed when an incoming order arrived). These are tolerated.

To keep the toleration bands from becoming a loophole, the design separates correctness
from concurrency: a dedicated correctness phase drives the contestant with a single
serialized order stream so the true processing order is known and the comparison is
exact, while throughput and latency are measured separately under full concurrency. The
composite correctness rate is computed from the serialized phase and the hard-violation
checks, not from the permissive concurrent comparison alone.

### 7.4 Performance aggregation

For each contestant the ingester maintains a rolling latency histogram, a throughput
counter, and correctness counts.

Latency percentiles are computed from a logarithmic-bucket high-dynamic-range histogram
rather than by sorting samples. Sorting is order n log n per window and keeps every
sample in memory, which fails under a high event rate. The histogram records in
constant time into a fixed, small set of buckets (a few thousand buckets covering
microseconds to seconds at a fixed relative precision) and scans once for a percentile.
Memory per contestant is bounded at tens of kilobytes regardless of event volume, which
is the property that lets the ingester run with many contestants at once.

A sliding window of per-second histograms keeps the live percentiles reflecting the
recent past, and a cumulative histogram preserves percentiles after a test ends so the
final leaderboard shows real values.

Throughput is tracked per contestant in a ring of one-second buckets. Each bucket
carries the second it represents, so a bucket whose timestamp does not match the second
being read is treated as empty rather than as stale data from a previous minute. This
prevents inactivity gaps from inflating the reported rate.

### 7.5 Persistence

Raw per-order samples are written to TimescaleDB in bulk using the COPY protocol, which
sustains a row rate that batched single-row inserts cannot. Because COPY cannot apply an
on-conflict clause, deduplication of any replayed events is handled at query time using
the natural key of the event rather than a unique constraint at write time.

Live rolling metrics are written to Redis once per second as a per-contestant hash, and
the contestant is added to the active set the leaderboard reads from.

### 7.6 Delivery semantics

The telemetry consumer commits Kafka offsets only after the corresponding events have
been durably handled, not on read. Committing on read would mark events consumed before
they are persisted, so a pod restart would silently drop the in-flight buffer and
corrupt scores. Committing after the handler completes gives at-least-once delivery: a
restart re-reads the uncommitted tail, which is why deduplication is handled at query
time.

---

## 8. Real-Time Leaderboard

The leaderboard-api computes the score and fans it out.

Scoring reads the per-contestant metric snapshots from Redis on a fixed interval. It
normalizes throughput and tail latency across the active field with min-max
normalization, combines them with the correctness rate into a composite score, ranks
the field, and applies the tie-break rules. The disqualification rule (a submission with
fewer than the minimum valid order count is removed from the ranked field) is applied
during scoring so that a contestant cannot top the board on a handful of fast orders.

Fan-out is built to scale horizontally. The scorer publishes the ranked leaderboard to a
single Redis pub/sub channel. Every leaderboard-api pod, including the one that ran the
scorer, receives the update through its own subscriber and broadcasts it to its
connected clients. The scorer never broadcasts directly to its own clients, so there is
no double delivery, and pods can be added behind a load balancer freely.

The WebSocket hub protects the broadcast from any single slow client. Each client has a
bounded send buffer. The broadcast performs a non-blocking send; if a client's buffer is
full, the hub closes and drops that client rather than letting it stall the broadcast
for everyone else. The hub also caps the total client count, sends the most recent
snapshot to a client on connect so a new viewer sees state immediately, and runs a
ping-pong keepalive so dead connections are reaped.

The frontend keeps a small reconnection-capable WebSocket manager with exponential
backoff, falls back to the REST snapshot when the socket is unavailable, and derives the
socket scheme from the page scheme so it works behind TLS. Rank changes are diffed on
arrival so the UI can animate movement.

---

## 9. Chaos Engineering

The platform is tested against the failures it is built to survive, rather than assuming
they will not happen.

- **Contestant server failures** are simulated with a mock server that can inject delays,
  errors, and dropped connections, used to verify the circuit breaker opens, the bot
  fleet stays bounded, and the latency measurement degrades gracefully rather than
  hanging.
- **Pod restarts** during a running test are exercised to verify the orchestrator's
  heartbeat and orphan detection recover the test rather than leaving it stuck.
- **Backpressure** is exercised by driving the ingester past its sustainable rate to
  confirm that it sheds load in a bounded way (dropping the oldest samples with a
  counter) rather than growing memory without limit.

---

## 10. Inter-Service Communication

The durable backbone is Kafka. The topics and their contracts:

- `build-jobs` carries `BuildJob` messages from submission-api to build-worker.
- `build-results` carries `BuildResult` messages back.
- test control (`StartTest`, `StopTest`, `TestHeartbeat`) carries orchestrator-to-fleet
  commands and liveness.
- `bot-telemetry` carries the high-volume `OrderEvent` firehose from the bot fleet to
  the ingester, partitioned so that a contestant's events hash to a stable partition set
  and the ingester can consume in parallel.

The live read path uses Redis: per-contestant metric hashes, the active-contestant set,
the `leaderboard:updates` pub/sub channel, and the cached latest snapshot.

Message schemas are defined as protobuf contracts shared across services, with the
durable Kafka path as the source of truth. Optional gRPC and GraphQL contracts exist for
the low-latency and streaming surfaces, but the Kafka contracts remain authoritative.

---

## 11. Data Stores

| Store | Purpose | Why this store |
|---|---|---|
| PostgreSQL | Submission and test state, contestant records | Strong consistency and transactions for the state machine and the submission lifecycle. |
| TimescaleDB | Raw per-order latency and throughput samples, per-test summaries | A time-series hypertable with compression handles the highest-volume table efficiently and supports the historical analysis queries. |
| Redis | Live per-contestant metrics, active set, pub/sub fan-out, rate-limit counters, read-through cache | In-memory speed for the per-second live path and a natural pub/sub primitive for horizontal fan-out. |
| MinIO or S3 | Uploaded submission artifacts | Object storage keeps large binaries out of the database and is streamed directly rather than buffered. |

PostgreSQL holds the small, consistency-critical state. TimescaleDB holds the large,
append-only history. Redis holds the ephemeral live view. Object storage holds the
artifacts. Each store is used for the one job it is best at.

---

## 12. Infrastructure as Code

The platform is reproducible from code, not from manual setup.

- **Docker Compose** brings up the full single-host stack (infrastructure, every
  service, and the frontend) for development, plus an infrastructure-only variant for
  running services by hand.
- **Helm charts**, one per service, parameterize the Kubernetes deployment, including
  resource requests and limits, horizontal pod autoscaling for the bot fleet and
  ingester, and the security context for each pod.
- **Raw Kubernetes manifests** cover the cross-cutting concerns: namespaces with a
  PodSecurity baseline, default-deny network policies for the contestant namespace,
  monitoring (ServiceMonitors and alert rules), tracing (an OpenTelemetry collector and
  a tracing backend), and logging.
- **Terraform** provisions the cloud substrate, with remote state encrypted in object
  storage and locked, and workload identity so that no long-lived cloud credentials live
  in pods.

Sandbox controls are also code: the seccomp profile, the AppArmor profile, and the
network policies are versioned alongside the services they protect.

---

## 13. CI/CD Pipeline

Three pipelines guard the main branch.

- **Continuous integration** runs static analysis and the test matrix across every Go
  module, type-checks and builds the frontend, and on the main branch builds and pushes
  the service images.
- **Integration** brings up the full stack, waits for health, runs an end-to-end script
  that exercises the upload-to-score pipeline, and tears down.
- **Security** runs image vulnerability scanning, static security analysis, and
  dependency auditing on every push to main and on a schedule.

Every Go module is pinned to the same toolchain version, and the linter configuration is
shared, so a green CI run means the same thing everywhere.

---

## 14. Composite Scoring Algorithm

The composite score weights three dimensions:

```
composite_score = 0.40 * normalized(throughput)
                + 0.40 * (1 - normalized(p99_latency))
                + 0.20 * correctness_rate
```

Throughput is the sustained orders per second the contestant handled. The p99 latency
is the tail acknowledgment time; it is inverted so that lower latency scores higher.
Correctness is the fraction of orders judged correct against the reference, over the
total excluding timeouts.

Throughput and p99 are min-max normalized across the active field so the score is
relative to the competition rather than to an absolute target, then the weighted sum is
scaled to a zero-to-one-hundred range.

Ties break on correctness first, then on p99 latency. A submission with fewer than the
minimum valid order count is disqualified before ranking, so the board cannot be topped
by a tiny, fast sample. The correctness rate counts correct orders over total orders,
excluding timeouts, so a contestant is not penalized in the correctness dimension for a
network timeout that is already reflected in latency and throughput.

The weighting reflects the contest's values: speed and throughput together are eighty
percent of the score because the contest is about high-performance infrastructure, but
correctness is a hard floor through the disqualification rule and the hard-violation
checks, so a fast but wrong engine cannot win.

---

## 15. Technology Decisions

- **Go for the services.** Predictable latency without a stop-the-world garbage pause
  long enough to distort measurement, cheap goroutine concurrency for the bot fleet and
  the fan-out hub, and static binaries that fit a minimal container.
- **React and TypeScript for the frontend.** A live, diff-animated leaderboard with a
  reconnecting socket is straightforward in React, and the type system catches contract
  drift against the service schemas at build time.
- **Kafka for the durable backbone.** Replayable, partitioned, at-least-once delivery is
  the property that makes scoring trustworthy through restarts. Redpanda is a
  drop-in alternative where a lighter operational footprint is preferred.
- **Redis for the live path.** Per-second metric writes and pub/sub fan-out need
  in-memory speed and a native publish-subscribe primitive.
- **TimescaleDB for history.** The latency-sample table is the highest-volume table in
  the platform; a time-partitioned hypertable with compression is the right tool, and it
  keeps the analysis queries in SQL.
- **A logarithmic-bucket histogram for percentiles.** Constant-time recording and
  bounded memory are the only way the percentile path keeps up at peak event rates with
  many concurrent contestants.

---

## 16. Architecture Decision Records

These are the decisions that keep results trustworthy and the platform standing under
load. Each is a deliberate trade, not an accident.

1. **Correctness is decided by a platform-owned reference, never by the submission.**
   Trade: the platform must maintain a correct matching engine. Benefit: the score
   cannot be faked.

2. **Correctness and concurrency are measured separately.** A serialized correctness
   phase gives an exact verdict; a concurrent phase gives realistic throughput and
   latency. Trade: a slightly more complex test plan. Benefit: the correctness rate is
   not a permissive guess, and the comparator cannot become a cheating loophole.

3. **Kafka offsets commit after the handler, not on read.** Trade: at-least-once
   delivery requires query-time deduplication. Benefit: a pod restart never silently
   drops in-flight telemetry.

4. **Percentiles use a bounded logarithmic histogram, not sorting.** Trade: percentiles
   are approximate to a fixed relative precision. Benefit: constant-time recording and
   tens of kilobytes per contestant, so the ingester scales to the full field.

5. **The contestant container has no network path to internal infrastructure.** Trade:
   the bot fleet and build-worker must be dual-homed. Benefit: a sandbox escape still
   reaches nothing of value.

6. **Sandbox controls fail closed.** If a required security profile cannot be loaded,
   the launch fails rather than running unprotected. Trade: a misconfigured profile
   blocks the contest until fixed. Benefit: the platform never silently runs hostile
   code without its protections.

7. **The WebSocket fan-out drops slow clients rather than blocking.** Trade: a slow
   viewer misses frames and reconnects. Benefit: one slow client cannot stall the
   broadcast for everyone.

8. **The leaderboard read path scales through Redis pub/sub.** Trade: a frame can be
   dropped at the hub under burst. Benefit: pods are stateless and horizontally
   scalable, and the next tick corrects any dropped frame.

9. **The orchestrator recovers tests through heartbeats and orphan detection.** Trade:
   a recovery window of roughly a minute after a crash. Benefit: a pod restart is a brief
   blip, not a permanently stuck test.

---

## 17. Performance Characteristics

The design targets are set by the contest, not by what is convenient:

- **Ingestion** must keep Kafka consumer lag bounded under the full bot-fleet event
  rate. The bottleneck is the percentile and validation path, which is why both are
  constant-time per event and why the consumer group can be scaled out across
  partitions.
- **Memory** per contestant in the ingester is bounded at tens of kilobytes for the
  histogram plus small counters, so the ingester's footprint grows linearly and gently
  with the field size rather than exploding.
- **Build time** for a typical submission is expected to complete in under a minute,
  which is the budget the contest-day runbook is written against.
- **Leaderboard freshness** is one scoring interval, and the WebSocket fan-out is
  designed to push to many thousands of concurrent viewers per pod.

These are targets the chaos and load tests are meant to confirm, and the observability
stack is what proves them in real time during a contest.

---

## 18. Contestant Upload Flow

From the contestant's point of view the platform is a black-box order book grader.

1. Generate a starter for the chosen language. The starter implements the health and
   reset endpoints and stubs the order and cancel endpoints.
2. Implement the order-book API: an order endpoint that accepts limit and market buys
   and sells, a cancel endpoint, a health endpoint, and a reset endpoint. The semantics
   are price-time priority, immediate fill of crossing orders with the remainder
   resting, market orders consuming the best opposite-side liquidity, and limit fills
   reporting the resting price.
3. Test locally against the reference implementation, which is the same engine the
   platform validates against, so local agreement predicts platform correctness.
4. Submit the artifact with the API key. Poll the submission id until it is ready.
5. Start a test and watch the live leaderboard. Optimize for the score weighting:
   connection reuse, reduced lock contention, and avoiding a per-order allocation are
   the common wins.

The API contract is intentionally small and language-agnostic, so the contest measures
engineering quality rather than familiarity with a particular framework.

---

## 19. Observability

Every HTTP service exposes a metrics endpoint for scraping, a liveness endpoint, and a
readiness endpoint. The key metrics are the ones an operator watches during a contest:
request rate and duration by route on the ingress, tests started, completed, failed,
orphans recovered, and active tests on the orchestrator, events processed and correct
plus consumer lag on the ingester, and connected client count on the leaderboard.

Logs are structured with consistent dimensions (service, request id, contestant id,
test id) so a single test run can be traced across every service. Dashboards, alert
rules, distributed tracing, and centralized logging are provisioned as code alongside
the services.

---

## 20. Design Principles

The following principles govern the platform's architecture and scoring model.

- **Single source of truth for correctness.** The reference engine and the published
  API contract are kept in lockstep. The contract explicitly pins every edge case
  (sweep pricing, cancel-of-filled status), so the reference and the contract always
  agree. Contestants test locally against the same reference the platform uses.
- **Serialized correctness, concurrent performance.** The platform measures correctness
  in a serialized phase where the true processing order is known and the comparison is
  exact. Throughput and latency are measured under full concurrency. This separation
  produces the most accurate results for both dimensions.
- **Fair, consistent latency measurement.** Latency is the HTTP round trip from the
  bot to the sandbox. Every contestant runs under the same conditions on the same
  infrastructure, so the shared network component cancels out in the ranking. This
  produces a fair comparative measurement.
- **Relative scoring for a live leaderboard.** Min-max normalization scores each
  contestant relative to the current field, which is the correct model for a live
  competitive leaderboard. The final ranking is captured at the freeze point.
- **Two deployment targets.** The Compose stack provides a fully functional development
  environment. The Kubernetes deployment provides production-grade isolation,
  autoscaling, network policies, and pod security profiles. Both targets are
  provisioned as code and tested in CI.
