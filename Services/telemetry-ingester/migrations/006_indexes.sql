-- Additional query-pattern indexes.
CREATE INDEX IF NOT EXISTS idx_latency_samples_contestant_time
    ON latency_samples (contestant_id, time DESC);
CREATE INDEX IF NOT EXISTS idx_latency_samples_test_time
    ON latency_samples (test_id, time DESC);
ANALYZE latency_samples;
