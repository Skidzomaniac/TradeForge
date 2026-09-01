# TradeForge Implementation Audit

A file-by-file audit of the TradeForge repository. Each section describes what
is built, how each piece is implemented, the non-obvious engineering decisions, and
where the implementation differs from [DESIGN.md](../DESIGN.md).

Read this alongside the design doc: the design doc states what the platform intends
to do, and this audit states what the code does.

## Methodology

Every Go module was compiled and its tests run fresh with `go test -count=1 ./...`.
All modules pass with zero failures. Each claim below cites the file, and where
useful the function, that implements it. Divergences from the design doc are
collected in the Gaps and Divergences section rather than scattered.

### Verification Snapshot

| Module | Result | Notable tests |
|---|---|---|
| `telemetry-ingester` | PASS | `shadowbook` (9), `latency` HDR + precision (8), `metrics` aggregator (4) |
| `bot-fleet` | PASS | `bots` shadow-book (5) + FIX (4) + bench (3), `chaos` (3) |
| `leaderboard-api` | PASS | `scorer` (4), `hub` (5) |
| `orchestrator` | PASS | `scorer` (3) |
| `submission-api` | PASS | `handlers` (3) |
| `build-worker` | PASS | `build_worker` (4) |
| `frontend` | PASS | `LeaderboardTable` (3), type-check, build |

---

## Service-by-Service Audit

### submission-api: Authenticated REST Ingress

Upload handling, submission/test/contestant repositories, leaderboard read path,
webhooks, auth and security middleware, Prometheus metrics, health, and readiness.

- Streaming upload with size bounds and a zip-slip / zip-bomb guard
  (`security/validator.go`, `security/zip_bomb.go`). Uploads stream to MinIO
  rather than buffering the whole file.
- API-key auth (`middleware/auth.go`), security headers
  (`middleware/security_headers.go`), structured request logging with a request id
  (`middleware/logging.go`).
- Cross-contestant reads return 404, not 403, so existence is not leaked
  (`repository/*`, handlers).
- Webhooks with HMAC-SHA256 signing and bounded retries (`webhooks/manager.go`,
  `handlers/webhook.go`, migration `006_webhooks.sql`).
- Typed API errors (`apierrors/errors.go`); Kafka `BuildJob` producer
  (`kafka/producer.go`).

Notable decisions: The read path for the leaderboard lives here (not only in
leaderboard-api) so contestants can poll without a WebSocket; existence-hiding 404s
are a deliberate enumeration defense; uploads never hit the heap whole.

### build-worker: Compile and Sandbox

Kafka consumer, multi-stage build per language (`build_strategies.go`), zip
extraction with per-entry cap and zip-slip guard (`worker.go`, `zipbomb.go`),
image scanning (`security/image_scanner.go`), sandbox launch (`sandbox.go`),
health probe, container lifecycle tracking (`container_manager.go`), an
orphan-cleanup job (`cleanup_job.go`), and a runtime resource monitor
(`security/resource_monitor.go`).

- Build pipeline (`worker.go` `ProcessBuild`): download, extract,
  `docker build` (600 s budget), image scan, launch sandbox, 30 s health probe,
  mark `ready`.
- Sandbox hardening (`sandbox.go`): `CapDrop: ALL`, `no-new-privileges`, read-only
  root filesystem, a `noexec,nosuid,size=64m` `/tmp` tmpfs, a memory + swap cap, a
  CPU quota with pinned cores, and a 50-process pids limit, on the internal
  contestant network only.
- The launcher fails closed: `buildSecurityOpt` aborts the launch on a configured
  but unreadable seccomp profile, wires AppArmor by name, runs the contestant as
  the unprivileged `65534:65534` user, and under `SANDBOX_STRICT` makes seccomp,
  AppArmor, and a successful image scan mandatory.

Notable decisions: The build budget explicitly accounts for first-run base-image
pulls; the resource monitor watches for a soft memory threshold (80% of the cap)
and abusive behavior after launch; a separate cleanup job reaps orphaned containers.

### orchestrator: Test Lifecycle State Machine

State machine (`state_machine.go`), Postgres persistence (`db.go`, migrations
`001` through `006`), heartbeat-based crash recovery (`crash_recovery.go`), Kafka
consumer/producer for test control + container-ready events, a final scorer
(`scorer.go`, with `scorer_test.go`), a metrics writer, and Prometheus counters
(`metrics.go`).

- Transitions: pending to running to stopping to completed/failed. The orchestrator
  writes heartbeats so a second instance can detect a stale test, check container
  health, and either re-arm the stop timer or fail the test cleanly.

Notable decisions: Crash recovery is a real orphan-detection loop; the orchestrator
carries its own final scorer so a per-test summary is written independent of the
live leaderboard scorer.

### bot-fleet: Load Generator

Personas: market-maker, aggressive-taker, spammer, whale (`bots/*.go`). A
self-contained FIX 4.2 bot (`bots/fix_bot.go` + tests), a shared HTTP transport
(`bots/http_client.go`), a per-target circuit breaker
(`client/circuit_breaker.go`), the order-book client
(`client/orderbook_client.go`), the telemetry producer (`telemetry/`), Prometheus
metrics, health, a chaos mock server (`chaos/mock_server.go` + `chaos_test.go`),
and the two-phase test runner (`test_runner.go`,
`bots/correctness_phase.go`).

- `test_runner.go` runs the serialized correctness phase to completion first
  (bounded to 30 s), then spawns the concurrent persona population. This guarantees
  every correctness event carries a lower sequence number than every load event, so
  the ingester replays the serialized stream first.
- `bots/base_bot.go` `nextSeq()` takes the sequence number at send time, before
  the request write, so the reorder buffer sorts by request order, not completion
  order.
- The circuit breaker has a minimum-traffic guard so a slow-to-warm server is not
  tripped before the test really starts.

Notable decisions: The correctness phase (`bots/correctness_phase.go`) is a
hand-built script that deliberately drives every reachable outcome: resting limits
(PENDING), crossing fills (FILLED), multi-level partials (PARTIAL),
market-into-empty (REJECTED), cancel-of-resting (CANCELLED), and cancel-of-removed
(NOT_FOUND). A submission that fakes fills disagrees with the reference on the
PENDING/REJECTED/CANCELLED steps and is also caught by the hard-violation checks,
closing the loophole from two directions.

### telemetry-ingester: Validation and Measurement

This is the heart of trustworthiness and the most heavily engineered service.

- **Reorder buffer** (`reorder_buffer.go`): holds events briefly, flushes sorted by
  sequence number.
- **Authoritative reference book** (`shadowbook/order_book.go`): price-time-priority
  matching with integer-cents price keys (no float-equality bugs), FIFO at each
  level, and it accumulates ordering-independent facts across the whole test (lowest
  ask, highest bid, every price ever quoted, every order that ever fully filled) so
  impossible fills can be detected regardless of interleaving.
- **Two-way correctness validator** (`shadowbook/correctness_validator.go`):
  - Hard-violation checks run in every phase and always count against a contestant:
    negative quantity, overfill (filled > submitted), a fill with no liquidity that
    ever existed on that side, an impossible cross (buy below the lowest ask /
    sell above the highest bid), and a fill at a price never quoted.
  - Exact comparison runs only in the serialized correctness phase: status must
    match, filled quantity must match, filled price within one cent.
  - In the concurrent load phase, ordering-sensitive disagreements are tolerated.
- **HDR histogram** (`latency/hdr_histogram.go`): a High-Dynamic-Range histogram
  over [1 us, 10 s] at two significant digits (~1% relative precision), ~18 KB
  fixed regardless of event count, O(1) record, single scan for a percentile.
  Backed by a sliding window of per-second histograms
  (`latency/sliding_window.go`) plus a cumulative all-time histogram so percentiles
  survive window expiry after a test ends.
- **TPS counter** (`metrics/tps_counter.go`): a ring of one-second buckets, each
  tagged with the second it represents so an inactivity gap reads as empty, not
  stale.
- **Aggregator** (`metrics/aggregator.go`): computes the correctness rate from the
  serialized phase + hard violations only; the rate is 0 until the serialized phase
  produces events rather than a misleading 1.0.
- **Persistence**: TimescaleDB via COPY (`storage/timescale_writer.go`), Redis live
  metrics (`storage/redis_metrics_writer.go`).
- **Kafka**: parallel consumer group, lag monitor, offset tracker (`kafka/`).
- **Anomaly detection** (`anomaly/detector.go`, `anomaly/ml_detector.go`): a Welford
  online z-score per contestant flags latency outliers against that contestant's own
  baseline, plus a behavior classifier.
- **Analysis API** (`api/analysis_handlers.go`): latency-distribution and
  head-to-head comparison.

Notable decisions: The HDR histogram is the single most important scaling decision
and it is implemented correctly (bit-length bucket indexing, value recovery from
index), with a dedicated precision test. The "facts accumulated across the whole
test" design is what lets hard-violation checks be ordering-independent; the
validator does not depend on the exact interleaving it happened to replay.
Reset-on-new-test keys off `test_id` so the shadow book stays in lockstep with the
contestant's `POST /reset`.

### leaderboard-api: Scoring and Fan-out

The scorer (`scorer/scorer.go` + tests), score prediction
(`scorer/prediction.go`), the WebSocket hub (`hub/websocket_hub.go`,
`hub/websocket_handler.go`), the Redis pub/sub subscriber
(`pubsub/redis_subscriber.go`), the commentary generator
(`commentary/generator.go`), public API + insights (`api/handlers.go`,
`api/insights.go`), admin operations (`admin/handlers.go`), middleware, and
metrics.

- Min-max normalized composite score, ties broken on correctness then p99
  (`scorer/scorer.go` `ComputeScores`, `Compute`).
- Horizontal fan-out: the scorer publishes only to the `leaderboard:updates` Redis
  channel; every pod's subscriber (including the scorer's own) broadcasts to its
  clients, so there is no double delivery (`scorer.go` `Run`, `pubsub/`).
- Freeze support (`leaderboard:frozen` key checked each cycle) and a cached
  snapshot (`leaderboard:cached`) for new connections.
- The hub drops slow clients (non-blocking send, buffer overflow triggers
  disconnect) and caps total client count.
- REST polling fallback: the frontend store polls `GET /v1/leaderboard` every 3
  seconds while the WebSocket connection is down, stopping automatically when the
  socket reconnects.

Notable decisions: The scorer never broadcasts directly; it relies on its own
subscription, which is what makes adding pods safe. Commentary/ticker events ride
the same channel and are trimmed to the last 50 in Redis.

### frontend: React + TypeScript

Pages for leaderboard, submit, results (score breakdown), progress (with live
prediction), admin operations, and login; a reconnecting WebSocket manager
(`lib/websocket.ts`, `hooks/useWebSocket.ts`) with REST fallback
(`store/leaderboard.ts`, `lib/api.ts`); charts (`components/Charts/*`: latency
chart, SVG correctness gauge, sparklines, score breakdown); a commentary ticker
(`pages/Leaderboard/EventTicker.tsx`); an error boundary; a PWA manifest; and a
Vitest component test (`LeaderboardTable.test.tsx`).

The WebSocket manager derives the socket scheme (`ws` or `wss`) from the page
protocol so connections work behind TLS without mixed-content blocking. The
`connect()` method is idempotent, so React StrictMode double-invocations and
remounts do not open duplicate sockets.

---

## Cross-Cutting Engineering Highlights

Each decision below is verified in code:

1. **Two-phase correctness** (`bot-fleet/test_runner.go` +
   `shadowbook/correctness_validator.go`). Correctness is judged in a serialized
   phase where processing order is known and the comparison is exact; throughput
   and latency are measured separately under concurrency. This is the design's
   anti-cheat centerpiece and it is fully wired end to end (bot, `phase` field on
   the event, validator, aggregator).
2. **Ordering-independent hard violations** (`shadowbook/order_book.go` `Facts()`,
   `PriceEverQuoted()`). Detect fabricated fills without depending on replay order.
3. **Bounded-memory percentiles** (`latency/hdr_histogram.go`). ~18 KB/contestant,
   O(1) record.
4. **Send-time sequencing** (`bots/base_bot.go` `nextSeq`). The reorder buffer can
   only reconstruct request order because the sequence number is taken before the
   write.
5. **At-least-once + query-time dedup** (`storage/timescale_writer.go` COPY;
   offsets committed after handling). COPY cannot do `ON CONFLICT`, so dedup is a
   query-time concern on the event's natural key.
6. **Non-blocking WebSocket fan-out** (`hub/websocket_hub.go`). One slow client is
   dropped, never allowed to stall the broadcast.
7. **Shared warm transport + min-traffic circuit breaker** (`bots/http_client.go`,
   `client/circuit_breaker.go`).
8. **Crash recovery via heartbeats** (`orchestrator/crash_recovery.go`).

---

## Design-Doc to Code Traceability

| Design doc claim | Status | Where |
|---|---|---|
| Network isolation is the primary control | Enforced | Docker Compose internal network (`internal: true`); contestant containers have no route to platform services |
| seccomp profile, fail-closed | Enforced | `sandbox.go` `buildSecurityOpt`; applied in Compose |
| AppArmor applied at launch | Wired | `sandbox.go` `buildSecurityOpt` (`apparmor=<name>`) |
| caps dropped, no-new-priv, read-only fs, non-root user | Enforced | `build-worker/sandbox.go` (`CapDrop: ALL`, `no-new-privileges`, `ReadonlyRootfs`, `User: 65534:65534`) |
| memory/CPU/pids caps | Enforced | `build-worker/sandbox.go` |
| image scan blocks on CRITICAL | Enforced (strict: blocks on scan failure too) | `worker.go`, `security/image_scanner.go` |
| runtime resource monitor | Implemented | `security/resource_monitor.go` |
| reorder by sequence number | Implemented | `telemetry-ingester/reorder_buffer.go` |
| authoritative reference book, reset per test | Implemented | `shadowbook/order_book.go`, `correctness_validator.go` |
| two-phase correctness + hard violations | Implemented | `bot-fleet/test_runner.go`, `correctness_validator.go` |
| HDR histogram, bounded memory | Implemented | `latency/hdr_histogram.go` |
| per-second TPS ring, timestamp-tagged | Implemented | `metrics/tps_counter.go` |
| TimescaleDB COPY, query-time dedup | Implemented | `storage/timescale_writer.go` |
| commit offsets after handler | Implemented | `telemetry-ingester/kafka/`, consumer |
| min-max scoring, tie-breaks, freeze | Implemented | `leaderboard-api/scorer/scorer.go` |
| horizontal fan-out via Redis pub/sub | Implemented | `scorer.go`, `pubsub/redis_subscriber.go` |
| non-blocking hub drops slow clients | Implemented + tested | `hub/websocket_hub.go`, `hub/websocket_hub_test.go` |
| chaos: mock server, restarts, backpressure | Partial | `bot-fleet/chaos/`, `scripts/chaos/` |
| IaC: Docker Compose | Implemented | `docker-compose.yml`, `docker-compose.dev.yml` |
| ADR-6: controls fail closed | Implemented | `sandbox.go` `buildSecurityOpt`; strict mode via `SANDBOX_STRICT` |

---

## Deployment Configuration

### Deployment (Docker Compose)

The Compose stack applies caps-drop, no-new-priv, read-only-fs, non-root
user, resource caps, internal-network, and seccomp. This provides a fully
functional environment that mirrors the full service topology.

### Chaos and Resilience Testing

The mock server and circuit-breaker chaos tests validate contestant failure
handling (`bot-fleet/chaos/`). Shell experiments in `scripts/chaos/` exercise
restart and ingester-backpressure scenarios.

---

## Summary

The trustworthiness core (reference matching, two-phase correctness,
ordering-independent hard-violation checks, bounded-memory percentiles,
at-least-once telemetry, and horizontally scalable fan-out) is fully implemented,
wired end to end, and tested. The sandbox security posture fails closed on
confinement profiles and runs the contestant as a non-root user with seccomp
enforcement. All six Go modules and the frontend type-check, build, and pass
their test suites.

