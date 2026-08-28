CREATE TABLE IF NOT EXISTS submissions (
    id              TEXT PRIMARY KEY,
    contestant_id   TEXT        NOT NULL,
    contestant_name TEXT        NOT NULL,
    language        TEXT        NOT NULL,
    s3_key          TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending',
    container_ip    TEXT,
    container_port  INT,
    container_id    TEXT,
    error_log       TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_submissions_contestant ON submissions (contestant_id);
CREATE INDEX IF NOT EXISTS idx_submissions_status ON submissions (status);
