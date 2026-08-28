CREATE TABLE IF NOT EXISTS tests (
    id                       TEXT PRIMARY KEY,
    submission_id            TEXT        NOT NULL REFERENCES submissions(id),
    contestant_id            TEXT        NOT NULL,
    status                   TEXT        NOT NULL DEFAULT 'pending',
    -- status values: pending, running, stopping, completed, failed
    started_at               TIMESTAMPTZ,
    ended_at                 TIMESTAMPTZ,
    final_score              DOUBLE PRECISION,
    failure_reason           TEXT,
    orchestrator_instance_id TEXT,   -- owning orchestrator, for crash recovery
    last_heartbeat_at        TIMESTAMPTZ, -- for orphan detection
    created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tests_contestant ON tests (contestant_id);
CREATE INDEX IF NOT EXISTS idx_tests_status ON tests (status);
CREATE INDEX IF NOT EXISTS idx_tests_heartbeat ON tests (status, last_heartbeat_at);
