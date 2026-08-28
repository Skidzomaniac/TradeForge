-- Final per-test rollups, written when a test completes.
CREATE TABLE IF NOT EXISTS test_summaries (
    test_id            TEXT PRIMARY KEY,
    contestant_id      TEXT             NOT NULL,
    started_at         TIMESTAMPTZ      NOT NULL,
    ended_at           TIMESTAMPTZ,
    p50_latency_us     BIGINT,
    p90_latency_us     BIGINT,
    p99_latency_us     BIGINT,
    peak_tps           DOUBLE PRECISION,
    avg_tps            DOUBLE PRECISION,
    total_orders       BIGINT DEFAULT 0,
    correct_orders     BIGINT DEFAULT 0,
    timed_out_orders   BIGINT DEFAULT 0,
    correctness_rate   DOUBLE PRECISION,
    composite_score    DOUBLE PRECISION,
    status             TEXT NOT NULL DEFAULT 'running'
);

CREATE INDEX IF NOT EXISTS idx_test_summaries_contestant
    ON test_summaries (contestant_id);
