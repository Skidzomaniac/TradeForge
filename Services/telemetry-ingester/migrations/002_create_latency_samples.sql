-- Raw per-order latency samples. The highest-volume table in the platform.
CREATE TABLE IF NOT EXISTS latency_samples (
    time          TIMESTAMPTZ NOT NULL,
    contestant_id TEXT        NOT NULL,
    test_id       TEXT        NOT NULL,
    bot_id        TEXT        NOT NULL,
    bot_persona   TEXT        NOT NULL,
    latency_us    BIGINT      NOT NULL,
    order_type    TEXT        NOT NULL,
    correct       BOOLEAN     NOT NULL,
    timed_out     BOOLEAN     NOT NULL DEFAULT false,
    order_id      TEXT,
    sent_at_ns    BIGINT
);

-- Convert to a hypertable partitioned on time.
SELECT create_hypertable('latency_samples', 'time', if_not_exists => TRUE);

-- Per-contestant time-range queries.
CREATE INDEX IF NOT EXISTS idx_latency_samples_time_contestant
    ON latency_samples (time, contestant_id);

-- Per-test queries.
CREATE INDEX IF NOT EXISTS idx_latency_samples_test
    ON latency_samples (test_id);

-- Persona breakdown analysis.
CREATE INDEX IF NOT EXISTS idx_latency_samples_contestant_persona
    ON latency_samples (contestant_id, bot_persona);

-- NOTE: bulk ingestion uses the COPY protocol (which cannot do ON CONFLICT),
-- so de-duplication of at-least-once replays is handled at query time via the
-- (sent_at_ns, bot_id, order_id) tuple rather than a unique constraint.
