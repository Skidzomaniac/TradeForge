-- Per-window throughput samples.
CREATE TABLE IF NOT EXISTS tps_samples (
    time              TIMESTAMPTZ      NOT NULL,
    contestant_id     TEXT             NOT NULL,
    test_id           TEXT             NOT NULL,
    orders_per_second DOUBLE PRECISION NOT NULL,
    window_size_ms    INT              NOT NULL DEFAULT 1000
);

SELECT create_hypertable('tps_samples', 'time', if_not_exists => TRUE);

CREATE INDEX IF NOT EXISTS idx_tps_samples_time_contestant
    ON tps_samples (time, contestant_id);
