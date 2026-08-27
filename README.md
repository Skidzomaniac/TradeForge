# TradeForge

A platform for evaluating order book implementations under realistic market load.
Contestants upload code, the platform builds it into a sandboxed container, drives
it with thousands of simulated trading bots, checks every fill against a reference
matching engine, and ranks everyone on a live leaderboard.

The whole stack runs locally with Docker Compose and is built to run on Kubernetes.

> **Design & audit.** [DESIGN.md](DESIGN.md) is the full architecture and design
> document: rationale, data contracts, failure modes, and architecture decision
> records. [docs/AUDIT.md](docs/AUDIT.md) is a file-by-file implementation audit
> covering what is built, how it works, and the non-obvious engineering decisions.

## Table of contents

- [How it works](#how-it-works)
- [Services](#services)
- [Repository layout](#repository-layout)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Make targets](#make-targets)
- [Contestant guide](#contestant-guide)
- [Order book API contract](#order-book-api-contract)
- [Reference order book](#reference-order-book)
- [Correctness and anti-cheat](#correctness-and-anti-cheat)
- [Scoring](#scoring)
- [Protobuf contracts](#protobuf-contracts)
- [Engineering decisions](#engineering-decisions)
- [Security model](#security-model)
- [Additional features](#additional-features)
- [Observability](#observability)
- [Continuous integration](#continuous-integration)
- [Contest day runbook](#contest-day-runbook)

## How it works

A submission travels through the system like this:

```
Contestant
   |
   v
Submission API  ->  MinIO (object storage)
   |
   v
Build Worker  ->  Docker sandbox (the contestant's order book)
   |
   v
Orchestrator  <->  Postgres (test state machine, crash recovery)
   |
   v
Bot Fleet  (market-maker, taker, spammer, whale, FIX bots)
   |
   |  bot-telemetry topic, Kafka, 16 partitions
   v
Telemetry Ingester  ->  TimescaleDB (history) + Redis (live metrics)
   |   (validate fills against the reference shadow book)
   v
Leaderboard API  ->  Redis + WebSocket fan-out
   |
   v
React frontend (live rankings)
```

1. A contestant uploads a zip or source file to the Submission API. It streams the
   file to MinIO, records the submission in Postgres, and queues a build job on Kafka.
2. The Build Worker compiles the code inside a multi-stage Docker build, then starts
   a hardened sandbox container and waits for its health check.
3. When a test starts, the Orchestrator drives a state machine (pending, running,
   stopping, completed, failed), writes heartbeats for crash recovery, and tells the
   Bot Fleet to begin.
4. The Bot Fleet runs two phases. First a **serialized correctness phase**: one logical
   sender drives a scripted order stream, one request at a time, so the true processing
   order is known. Then the **concurrent load phase**: bots of several personas hammer
   the server in parallel. Each request measures round-trip latency and emits a telemetry
   event (tagged with its phase) to Kafka.
5. The Telemetry Ingester reorders events by sequence number, replays them through an
   authoritative shadow order book, and judges correctness two ways: an exact comparison
   in the serialized phase, plus ordering-independent hard-violation checks in every
   phase. It computes percentiles with an HDR histogram and writes history to TimescaleDB
   and live metrics to Redis.
6. The Leaderboard API reads Redis, computes composite scores, and pushes ranked
   updates to every browser over WebSocket.

## Services

| Service | Language | Role |
|---|---|---|
| `submission-api` | Go | Receives uploads, queues build jobs, serves the REST API and leaderboard |
| `build-worker` | Go | Compiles submissions into hardened Docker sandboxes and manages their lifecycle |
| `orchestrator` | Go | Test lifecycle state machine, heartbeat crash recovery, scoring |
| `bot-fleet` | Go | Spawns market-maker, taker, spammer, whale, and FIX bots that load the order book |
| `telemetry-ingester` | Go | Validates fills, computes HDR-histogram percentiles, persists metrics |
| `leaderboard-api` | Go | Composite scoring plus WebSocket fan-out |

| `frontend` | React, TypeScript | Live leaderboard, submission flow, results, admin console |

## Repository layout

```
.
├── services/              Go microservices (one module each)
│   ├── submission-api/
│   ├── build-worker/
│   ├── orchestrator/
│   ├── bot-fleet/
│   ├── telemetry-ingester/
│   ├── leaderboard-api/

├── frontend/              React + Vite + TypeScript app
├── proto/                 Protobuf message contracts shared across services
├── cmd/
│   └── generate-template/ CLI that emits contestant starter code
├── testdata/
│   └── sample-orderbook/  Correct reference C++ order book
├── tests/                 Cross-service correctness and integration tests
├── scripts/               Local dev, migration, smoke test, verification scripts
├── infra/
│   ├── docker/            Sandbox seccomp and AppArmor profiles, nginx
│   ├── helm/              Helm charts per service
│   ├── k8s/               Raw manifests, network policies, monitoring, tracing
│   ├── kafka/             Topic creation and tuning notes
│   ├── grafana/           Dashboards
│   └── terraform/         Cloud provisioning
├── docker-compose.yml     Full local stack
├── docker-compose.dev.yml Infra-only stack for running services with `go run`
├── Makefile               Developer commands
└── .env.example           Every environment variable, documented
```

## Quick start

Prerequisites: Docker 24 or newer, Docker Compose v2, Go 1.26 or newer, Node 20 or newer.

Bring up the full stack (infrastructure, all services, and the frontend):

```bash
cp .env.example .env     # fill in local values, or use the defaults below
make up
make smoke-test          # end-to-end check once the first build is ready
```

Open the apps:

| App | URL |
|---|---|
| Frontend | http://localhost:3000 |
| Submission API | http://localhost:8080 |
| Orchestrator | http://localhost:8082 |
| Telemetry Ingester | http://localhost:8083 |
| Leaderboard API and WebSocket | http://localhost:8084 |
| MinIO console | http://localhost:9001 |

To run the Go services by hand against just the infrastructure:

```bash
make dev                 # Kafka, Redis, MinIO, TimescaleDB, Postgres only
make migrate && make seed
cd services/submission-api && go run .   # repeat per service
```

## Configuration

Every service reads its configuration from environment variables. The full list,
with a one-line description for each, lives in `.env.example`. Copy it to `.env`
and adjust as needed. Local Compose defaults work out of the box.

The important groups are:

- Kafka brokers and topic names (`KAFKA_BROKERS`, `BOT_TELEMETRY_TOPIC`, and friends).
- Datastores (`REDIS_URL`, `TIMESCALE_DSN`, `ORCHESTRATOR_DB_DSN`, MinIO credentials).
- Sandbox limits (`SANDBOX_MEMORY_LIMIT`, `SANDBOX_CPU_CORES`, `CONTESTANT_NETWORK`).
- Test defaults (`TEST_DEFAULT_DURATION_SECONDS`, `TEST_DEFAULT_BOT_COUNT`).
- Secrets (`ADMIN_API_KEY`, `INTERNAL_API_TOKEN`). Change these outside local dev.

Never commit `.env`. It holds credentials and is git-ignored.

## Make targets

| Target | What it does |
|---|---|
| `make up` | Start the full local stack |
| `make dev` | Start infrastructure only |
| `make down` | Stop all containers |
| `make logs SVC=submission-api` | Tail one service |
| `make migrate` | Apply SQL migrations to both databases |
| `make seed` | Seed local contestants |
| `make build` | Build Docker images for every service and the frontend |
| `make test` | Run unit tests for every Go module |
| `make test-race` | Run tests with the race detector |
| `make lint` | Run `go vet`, and `golangci-lint` if installed |
| `make proto` | Regenerate protobuf Go code |
| `make template LANG=cpp` | Generate a contestant starter (cpp, rust, go, or python) |
| `make smoke-test` | End-to-end check against a running stack |
| `make verify` | Full platform verification |
| `make clean` | Tear down the stack and remove coverage files |

Run `make help` for the complete list.

## Contestant guide

You implement an HTTP order book. The platform load-tests and scores it.

### 1. Generate a starter

```bash
make template LANG=cpp     # or rust, go, python; output lands in ./contestant-starter/
```

### 2. Implement the API

Your server must listen on port 8080 and implement `POST /order`, `POST /cancel`,
and `GET /health`. The full contract is below. The rules that matter:

- Price-time priority: at each price level, earlier orders fill first.
- Limit orders that cross fill immediately, and the remainder rests in the book.
- Limit fills must report the resting price exactly.
- Market orders take the best available opposite-side liquidity.

### 3. Test locally

```bash
ORDER_BOOK_URL=http://localhost:8080 bash tests/correctness/run.sh
```

Compare your behavior against the reference implementation in
`testdata/sample-orderbook/`.

### 4. Submit

```bash
curl -X POST http://localhost:8080/v1/submissions \
  -H "X-API-Key: $YOUR_KEY" \
  -F "file=@my-orderbook.zip" -F "language=cpp"
```

Poll the returned `submission_id` until its status is `ready`, then start a test
from the web UI or with `POST /v1/tests`. Watch your rank on the live leaderboard.

### 5. Optimize

You are scored 40 percent on throughput, 40 percent on inverse p99 latency, and
20 percent on correctness. Common wins: a connection-pool-friendly server, reduced
lock contention, and avoiding a heap allocation per order.

## Order book API contract

Your submission runs an HTTP server on port 8080. The bot fleet calls these
endpoints, and the telemetry ingester validates your fills against the reference
shadow order book.

### POST /order

Request:

```json
{ "order_id": "ord_xxx", "type": "LIMIT_BUY|LIMIT_SELL|MARKET_BUY|MARKET_SELL", "price": 100.50, "quantity": 10 }
```

Market orders omit `price`.

Response:

```json
{ "order_id": "ord_xxx", "status": "FILLED|PARTIAL|PENDING|REJECTED",
  "filled_price": 100.50, "filled_quantity": 10, "remaining_quantity": 0 }
```

### POST /cancel

Request:

```json
{ "order_id": "ord_xxx" }
```

Response:

```json
{ "order_id": "ord_xxx", "status": "CANCELLED|NOT_FOUND|ALREADY_FILLED" }
```

### GET /health

Must return HTTP 200 with `{ "status": "ok" }` within 3 seconds of startup. The
build worker waits up to 30 seconds for this before marking the submission ready.

### GET /orderbook (optional, for debugging)

```json
{ "bids": [ { "price": 100.0, "quantity": 10 } ], "asks": [ ] }
```

### POST /reset (test mode)

Clears all state. The bot fleet calls this at the start of each test so the
contestant book and the reference shadow book start from the same empty state.

### Semantics

- Price-time priority: at a given price level, earlier orders fill first.
- Market orders consume the best available opposite-side liquidity.
- Limit orders that cross fill immediately, and the remainder rests in the book.
- Fill prices for limit orders must match the resting price exactly.

### Timing

The bot fleet times out after 5 seconds. Latency is measured as the HTTP round trip,
so lower p99 latency scores higher.

### Minimal C++ example (cpp-httplib)

```cpp
#include "httplib.h"
int main() {
  httplib::Server s;
  s.Get("/health", [](const httplib::Request&, httplib::Response& r){
    r.set_content("{\"status\":\"ok\"}", "application/json");
  });
  // implement /order and /cancel
  s.listen("0.0.0.0", 8080);
}
```

## Reference order book

`testdata/sample-orderbook/` holds a correct, price-time-priority HTTP order book.
It serves two purposes: it is the platform's end-to-end test binary, and it is the
behavioral reference the telemetry ingester validates against.

It is self-contained and builds with nothing but a compiler:

```bash
g++ -O2 -std=c++17 -o orderbook main.cpp -lpthread
./orderbook        # listens on :8080
```

Because it has no external headers, it compiles inside the contestant sandbox
unchanged. It is deliberately simple: a single global mutex, no persistence, and
minimal JSON parsing tuned to the fixed request shapes. It is a reference, not a
competitive entry.

## Correctness and anti-cheat

The contestant controls the code and is motivated to cheat (for example, returning
`FILLED` for every order). Correctness is never taken from the submission's self-report;
it is decided by the platform's own reference matching engine. Two mechanisms make that
verdict trustworthy even though the contestant's server processes requests concurrently
and its true internal ordering is not observable from outside.

**1. A serialized correctness phase.** Before any concurrent load starts, the bot fleet
runs a single scripted order stream, one in-flight request at a time
(`bot-fleet/bots/correctness_phase.go`). Because each order is sent only after the
previous ack, the true processing order equals the send order, so the reference replay is
exact. The script deliberately exercises every reachable outcome  -  resting limits
(`PENDING`), crossing fills (`FILLED`), multi-level partials (`PARTIAL`), market-into-empty
(`REJECTED`), cancel-of-resting (`CANCELLED`), and cancel-of-removed (`NOT_FOUND`). In this
phase the comparison is strict: status must match, filled quantity must match, filled
price must be within one cent. This is the phase the **correctness rate** is computed
from, so an always-`FILLED` submission scores near zero
(`telemetry-ingester/shadowbook/correctness_validator.go`).

**2. Ordering-independent hard-violation checks (every phase).** During the concurrent
load phase the true ordering is unknowable, so ordering-sensitive disagreements are
tolerated. But some fills are impossible *regardless* of interleaving, and those always
count against a contestant: a negative quantity, an overfill (filling more than was
submitted), a fill on a side that never had liquidity, an impossible cross (buying below
the lowest ask ever quoted / selling above the highest bid ever quoted), or a fill at a
price never quoted in the book. The reference book accumulates these facts across the
whole test (`telemetry-ingester/shadowbook/order_book.go` `Facts()` / `PriceEverQuoted()`),
so the checks do not depend on the exact order the validator happened to replay.

The two phases are separated in time by sequence number: the serialized phase completes
before any load bot starts, so every correctness event sorts ahead of every load event in
the ingester's reorder buffer. Throughput and tail latency are measured in the concurrent
phase; correctness is judged from the serialized phase plus hard violations. This is what
keeps the toleration band from becoming a loophole.

## Scoring

```
composite_score = 0.40 * norm(tps)
                + 0.40 * (1 - norm(p99))
                + 0.20 * correctness_rate
```

Each metric is min-max normalized across contestants and the result is scaled to a
0 to 100 range. Ties break on correctness first, then p99 latency. A submission with
fewer than 100 valid orders is disqualified. The correctness rate counts correct
orders over total orders, excluding timeouts.

## Protobuf contracts

`proto/` holds the message definitions shared by every service. Regenerate the Go
code with `make proto` (which runs `proto/generate.sh`).

`submission.proto`:

| Message | Purpose | Key fields |
|---|---|---|
| `Submission` | Record of an uploaded program | `id`, `contestant_id`, `language`, `s3_key`, `submitted_at`, `status` |
| `BuildJob` | Queued build request on the `build-jobs` topic | `submission_id`, `s3_key`, `language` |
| `BuildResult` | Build outcome from the build worker | `submission_id`, `status`, `error_log`, `container_ip`, `container_port` |

`test_events.proto`:

| Message | Purpose | Key fields |
|---|---|---|
| `StartTest` | Tells the bot fleet to start a load test | `test_id`, `contestant_id`, `target_ip`, `target_port`, `duration_seconds`, `bot_count`, `bot_personas` |
| `StopTest` | Halts all bots for a test | `test_id`, `reason` |
| `TestHeartbeat` | Orchestrator liveness signal during a test | `test_id`, `timestamp_ns`, `active_bots` |

`telemetry.proto`:

| Message | Purpose | Key fields |
|---|---|---|
| `OrderEvent` | Per-order telemetry on the `bot-telemetry` topic | identity (`contestant_id`, `test_id`, `bot_id`, `order_id`), timing (`sent_at_ns`, `acked_at_ns`, `latency_us`), order details, correctness (`expected_fill`, `actual_fill`, `correct`, `timed_out`), `sequence_number` |
| `Fill` | Expected or actual execution | `price`, `quantity`, `status` |
| `LatencyWindow` | Aggregated per-window performance | `contestant_id`, window bounds, `p50_us`, `p90_us`, `p99_us`, `tps`, `correctness_rate` |

`leaderboard.proto`:

| Message | Purpose | Key fields |
|---|---|---|
| `LeaderboardEntry` | One ranked contestant row | `rank`, `contestant_id`, `contestant_name`, `score`, percentiles, `tps`, `correctness_rate`, `status` |
| `LeaderboardUpdate` | Full leaderboard snapshot | `timestamp`, `entries` |

`services.proto` and `schema.graphql` define optional gRPC and GraphQL contracts for
the low-latency and streaming paths. The durable Kafka path remains the source of
truth.

## Engineering decisions

These are the choices that keep results trustworthy under load. Each one is
implemented in the codebase. The headline trustworthiness decision  -  judging
correctness in a serialized phase plus ordering-independent hard-violation checks  -  has
its own section above ([Correctness and anti-cheat](#correctness-and-anti-cheat)); the
rest follow.

1. **Latency percentiles use an HDR histogram, not a sort.** A naive
   `sort(samples)[0.99*n]` is O(n log n) per window and keeps every sample in memory.
   At a million events per second it falls behind and eventually runs out of memory.
   The HDR histogram in `telemetry-ingester/latency/hdr_histogram.go` records in O(1)
   into fixed buckets and scans once for a percentile. Fixed memory, roughly ten times
   faster.

2. **Kafka offsets commit after the write, not before.** Auto-commit marks events
   consumed before they are persisted, so a crash loses them silently and corrupts
   scores. The build worker commits offsets only after the work completes, giving
   at-least-once delivery.

3. **Events are reordered by sequence number before validation.** Kafka arrival order
   is not the true send order across distributed bots. Without correction the shadow
   book produces false correctness failures. The reorder buffer in
   `telemetry-ingester/reorder_buffer.go` holds events briefly and sorts by the
   bot-assigned sequence number before the authoritative book replays them.

4. **WebSocket fan-out drops slow clients instead of blocking.** One slow client must
   not stall the broadcast for ten thousand others. The hub in
   `leaderboard-api/hub/websocket_hub.go` does a non-blocking send and disconnects
   clients whose buffer is full.

5. **Horizontal WebSocket scale uses Redis pub/sub.** The scorer publishes to a
   `leaderboard:updates` channel and every pod's hub subscribes and broadcasts. The
   scorer's own pod does not also broadcast directly, so there is no double delivery.
   Add pods freely behind a load balancer.

6. **TimescaleDB writes use COPY, not INSERT.** Bulk COPY in
   `storage/timescale_writer.go` sustains the row rate that batched INSERT cannot.
   Because COPY cannot do `ON CONFLICT`, deduplication of replayed events happens at
   query time rather than through a unique constraint.

7. **The orchestrator recovers from crashes.** Each running test writes a heartbeat.
   A second orchestrator detects stale heartbeats in `crash_recovery.go`, checks
   container health, and either re-registers the stop timer or fails the test cleanly.
   A pod restart becomes a roughly 60 second blip instead of a permanently stuck test.

8. **All bots share one warm connection pool.** A new HTTP client per bot exhausts
   file descriptors and adds handshake latency to every measurement. Bots targeting
   one container share a tuned transport in `bot-fleet/bots/http_client.go`.

9. **The circuit breaker has a minimum-traffic guard.** It only trips after at least
   20 requests, so a container that is slow to warm up does not trip it before the
   test really starts. See `client/circuit_breaker.go`.

## Security model

The primary adversary is a contestant who controls arbitrary code inside a sandbox
and may try to cheat the scoring or escape the sandbox to reach Kafka, Redis, or the
databases. A secondary adversary is an external attacker hitting the public API.

### Sandbox isolation

The design calls for seven layers (see [DESIGN.md §4](DESIGN.md#4-the-sandbox-engine)).
Here is what the code does, grouped by enforcement level. The full
breakdown is in [docs/AUDIT.md](docs/AUDIT.md#deployment-configuration).

**Always applied by the launcher** (`build-worker/sandbox.go`): drop all capabilities,
`no-new-privileges`, a read-only root filesystem, a `noexec,nosuid,size=64m` `/tmp`
tmpfs, a memory + swap cap, a CPU quota with pinned cores, a 50-process pids limit, and
the contestant process running as the unprivileged `65534:65534` user  -  on the internal
contestant network only.

**Confinement profiles, fail closed** (`sandbox.go` `buildSecurityOpt`): a configured
seccomp profile that cannot be read (or is empty) **aborts the launch** rather than
running the contestant unconfined. seccomp is applied in the dev Compose stack (the
profile is mounted at `/seccomp/contestant-profile.json`). AppArmor is wired by name
(`apparmor=trade-eval-contestant`) when `APPARMOR_PROFILE` is set. With
`SANDBOX_STRICT=true` (production), seccomp, AppArmor, **and** a successful image scan are
all mandatory and a missing one fails the build (ADR-6).

**Image scanning** (`security/image_scanner.go`): Trivy scans for HIGH/CRITICAL CVEs; a
CRITICAL finding blocks the launch. In strict mode a scan that cannot run (Trivy missing,
or the scan errors) also blocks. In non-strict dev a missing scanner logs and continues.

**Enforced in Kubernetes** (the production boundary): a default-deny `NetworkPolicy`
that blocks all egress and allows ingress only from the bot fleet on port 8080
(`infra/k8s/network-policies/contestant-netpol.yaml`), plus a PodSecurity `baseline`
namespace (`infra/k8s/security/`). **This network isolation is the control that matters
most**: even if every other layer is bypassed, a compromised container has no network
path to Kafka, Redis, Postgres, or MinIO.

**Runtime monitoring** (`security/resource_monitor.go`): the build worker watches the
container after launch and alerts on a soft memory threshold and abusive behavior.

> **Compose vs. production.** AppArmor and `SANDBOX_STRICT` are left off in the dev
> Compose stack because a dev host may not have the AppArmor profile loaded. The contest
> boundary is the Kubernetes deployment: load the profiles on the nodes, mount the
> seccomp JSON into the build-worker pod, and set `SANDBOX_STRICT=true`
> (`infra/helm/build-worker/values.yaml`).

### Scoring integrity

Correctness is judged by an authoritative shadow order book in the telemetry ingester,
replaying every contestant's orders in true send order. A contestant cannot fake
fills, because the reference engine decides what should have happened. Anomaly
detection flags suspiciously low latency or impossible perfect-correctness-at-speed
for manual review.

### API and infrastructure security

- API key authentication on all mutating endpoints. Cross-contestant reads return 404,
  not 403, so existence is not leaked.
- Per-contestant submission rate limit and per-IP leaderboard rate limit.
- Zip-slip guard and bounded extraction size on uploads.
- Constant-time comparison for admin and internal keys (`crypto/subtle`).
- IAM Roles for Service Accounts, so no long-lived cloud keys live in pods.
- Terraform remote state encrypted in S3 with DynamoDB locking.
- Secrets sourced from a secrets manager, never committed.

## Additional features

Beyond the core pipeline, the platform includes the following. Each is wired in and
covered by tests.

**Observability on every HTTP service.** Each exposes `/metrics` for Prometheus,
plus `/healthz` and `/readyz`:

- submission-api: request count and a duration histogram by route.
- orchestrator: counters for tests started, completed, and failed, an orphans-recovered
  counter, and an active-tests gauge.
- telemetry-ingester: events processed and correct, plus a Kafka consumer-lag gauge fed
  by the lag monitor.
- leaderboard-api: a WebSocket-connections gauge.

Grafana dashboards live in `infra/grafana/dashboards`, ServiceMonitors and alert rules
in `infra/k8s/monitoring`, Jaeger and an OpenTelemetry collector in `infra/k8s/tracing`,
and Fluent Bit, Elasticsearch, and Kibana values in `infra/helm/logging`.

**Anomaly detection** (`anomaly/ml_detector.go`): a Welford online z-score per
contestant flags latency outliers against that contestant's own baseline. A behavior
classifier publishes a label such as caching, inconsistent, or consistent high
performer to Redis each second.

**Historical analysis API**: `/v1/analysis/latency-distribution` returns histogram
buckets and `/v1/analysis/head-to-head` compares two contestants by percentile.

**Live commentary ticker** (`commentary/generator.go`): rank-change, milestone, and
close-race callouts published over the same WebSocket fan-out as a `ticker_event`
message and rendered as a scrolling ticker in the UI.

**Contestant insights** (`/v1/contestants/{id}/insights`): current rank, weakest
dimension against peers, and auto-generated improvement tips.

**Score prediction** (`/v1/contestants/{id}/prediction`): a least-squares trend
projection of the final score with a confidence based on sample count.

**Admin operations console** (`/admin/v1/system/status`, plus freeze, unfreeze, and
disqualify), surfaced in a frontend Operations Console page.

**Webhooks**: contestants register HMAC-SHA256-signed callbacks for build complete,
test complete, score update, and anomaly events, with bounded retries.

**FIX 4.2 bot** (`bots/fix_bot.go`): a self-contained message builder with correct
body length, checksum, and sequence numbers, with unit tests.

**Kafka operations**: a parallel consumer group, a lag monitor, and an offset tracker
under `telemetry-ingester/kafka`.

**Build-worker hardening**: a container resource monitor and a Trivy image scanner.
Critical CVEs block a sandbox from starting.

**Frontend**: Recharts components (a latency chart with reference bands, an SVG
correctness gauge, sparklines), animated rank-change row flashes, the commentary
ticker, leader hero stats, and a pulsing connection indicator. Pages cover the
leaderboard, submission, results with a score breakdown, progress with live
prediction, the admin operations console, and login. There is a PWA manifest, a React
error boundary, a mobile-responsive table, and Vitest component tests.

**Developer experience**: a `.dockerignore` per service for smaller builds, a
`.golangci.yml` covering errcheck, govet, staticcheck, unused, ineffassign, and
gosimple, every Go module pinned to go 1.26, and graceful shutdown on SIGTERM
everywhere.

## Observability

Quick reference once the stack is up:

```bash
make logs SVC=telemetry-ingester        # tail one service
make check-kafka-lag                    # consumer group lag
curl -s localhost:8083/metrics | head   # Prometheus metrics
curl -s localhost:8080/v1/health        # API health
```

Each service prints structured JSON logs. The key dimensions are `service`,
`request_id`, `contestant_id`, and `test_id`, which lets you trace one test run across
every service.

## Continuous integration

Three GitHub Actions workflows live in `.github/workflows/`:

- `ci.yml`: runs `go vet` across services, the Go test matrix, frontend type-check and
  build, then builds and pushes images to the container registry on the main branch.
- `integration.yml`: brings up the full Compose stack, waits for health, runs the
  end-to-end integration script, and tears down.
- `security.yml`: Trivy image scanning, gosec static analysis, and dependency auditing
  on a push to main and on a weekly schedule.

## Contest day runbook

### Twenty-four hours before

- `make test` is green across all modules and the frontend.
- `make build` succeeds for every image.
- `docker compose config` validates.
- Contestants are seeded (`make seed`) and API keys distributed.
- Kafka topic partition counts confirmed (`infra/kafka/topics.sh`).
- Smoke test passes on staging (`make smoke-test`).

### One hour before

- All pods running: `kubectl get pods -n trade-eval`.
- Kafka consumer lag near zero: `make check-kafka-lag`.
- TimescaleDB and Redis reachable.
- Bot fleet scaled to zero: `kubectl get hpa bot-fleet -n trade-eval`.

### Start

1. Announce submissions are open.
2. Watch the first builds drain, under 60 seconds each.
3. Run `bash scripts/verify-platform.sh`.

### Every fifteen minutes during

- Kafka consumer lag under 1000.
- No OOMKilled events.
- Leaderboard `updated_at` is fresh.
- No error logs in submission-api or orchestrator.

### Incident response

- Leaderboard stale: check leaderboard-api logs and the Redis `leaderboard:cached`
  key, then `kubectl rollout restart deployment/leaderboard-api`.
- Build stuck for more than five minutes: check build-worker logs and the Docker
  daemon.
- Kafka lag growing: `kubectl scale deployment telemetry-ingester --replicas=6`.
- Orchestrator crash: Kubernetes restarts it and orphan detection recovers running
  tests within about 60 seconds. Confirm none are stuck running via
  `/admin/active-tests`.

### Close

1. Wait for active tests to finish.
2. Freeze the leaderboard: `POST /admin/v1/leaderboard/freeze` with the admin key.
3. Export results and announce winners.
4. Scale the bot fleet to zero: `kubectl scale deployment bot-fleet --replicas=0`.
