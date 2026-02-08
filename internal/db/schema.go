package db

// schemaV1 contains the initial table definitions for ADM.
// All timestamps are stored as RFC 3339 UTC strings.
const schemaV1 = `
CREATE TABLE IF NOT EXISTS agents (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    name          TEXT    NOT NULL UNIQUE,
    task          TEXT    NOT NULL DEFAULT '',
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL,
    last_seen_at  TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_agents_last_seen_at ON agents(last_seen_at);

CREATE TABLE IF NOT EXISTS messages (
    id            TEXT    PRIMARY KEY,
    sender_name   TEXT    NOT NULL,
    body          TEXT    NOT NULL,
    kind          TEXT    NOT NULL CHECK(kind IN ('direct', 'broadcast')),
    created_at    TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_messages_created_at ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_sender_created ON messages(sender_name, created_at);

CREATE TABLE IF NOT EXISTS message_receipts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    message_id      TEXT    NOT NULL REFERENCES messages(id),
    recipient_name  TEXT    NOT NULL,
    state           TEXT    NOT NULL CHECK(state IN ('pending', 'offered', 'delivered')),
    batch_token     TEXT,
    offered_at      TEXT,
    delivered_at    TEXT,
    created_at      TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_receipts_recipient_state ON message_receipts(recipient_name, state, created_at);
CREATE INDEX IF NOT EXISTS idx_receipts_recipient_token ON message_receipts(recipient_name, batch_token);
CREATE INDEX IF NOT EXISTS idx_receipts_message ON message_receipts(message_id);

CREATE TABLE IF NOT EXISTS sync_batches (
    token       TEXT    PRIMARY KEY,
    agent_name  TEXT    NOT NULL,
    created_at  TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_batches_agent ON sync_batches(agent_name, created_at);

CREATE TABLE IF NOT EXISTS claims (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    agent_name    TEXT    NOT NULL,
    path_pattern  TEXT    NOT NULL,
    path_norm     TEXT    NOT NULL,
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_claims_path ON claims(path_norm);
CREATE INDEX IF NOT EXISTS idx_claims_agent ON claims(agent_name);
`

// migrateV2 adds a UNIQUE constraint on (agent_name, path_norm) in claims.
// It first deduplicates any existing rows, then creates the unique index.
const migrateV2 = `
-- Remove duplicate claims, keeping the row with the latest updated_at.
DELETE FROM claims WHERE id NOT IN (
    SELECT MAX(id) FROM claims GROUP BY agent_name, path_norm
);

-- Enforce uniqueness going forward.
CREATE UNIQUE INDEX IF NOT EXISTS idx_claims_agent_path ON claims(agent_name, path_norm);
`
