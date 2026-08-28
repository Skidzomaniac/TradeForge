-- Contestant webhook subscriptions (P97).
CREATE TABLE IF NOT EXISTS webhooks (
    id            TEXT PRIMARY KEY,
    contestant_id TEXT        NOT NULL,
    url           TEXT        NOT NULL,
    events        TEXT[]      NOT NULL DEFAULT '{}',
    secret        TEXT        NOT NULL,
    active        BOOLEAN     NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_webhooks_contestant ON webhooks (contestant_id);
