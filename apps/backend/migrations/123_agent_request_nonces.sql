-- Migration 123: admission store for signed action-request statements.
--
-- POST /api/v1/verifications (and its SDK mount) accepts a signed statement
-- that names a nonce. A statement is admitted by ONE statement that checks
-- its timestamp against the database clock and inserts (agent_id, nonce),
-- inserting nothing if the pair exists; no admitted pair is ever admitted
-- again while its row exists. The table is shared by both mounts and every
-- replica.
--
-- nonce holds the 16 decoded bytes. expires_at is the signed timestamp plus
-- the 30 second window, the last instant a request carrying the nonce could
-- be accepted. admitted_at is the database clock reading the window was
-- checked against; it has no default, because now() would be the
-- transaction's start rather than the statement's.
--
-- organization_id is the organization of the agent whose registered key
-- verified the statement, taken from the server's own lookup. The purge
-- deletes rows past expires_at + 31 seconds one organization at a time, on
-- the database clock, so its index leads with organization_id.
--
-- A row is never returned, exported or copied into a record; only admission
-- and the purge read the table. Idempotent.

CREATE TABLE IF NOT EXISTS agent_request_nonces (
    agent_id        UUID        NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    organization_id UUID        NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    nonce           BYTEA       NOT NULL CHECK (octet_length(nonce) = 16),
    expires_at      TIMESTAMPTZ NOT NULL,
    admitted_at     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (agent_id, nonce)
);

CREATE INDEX IF NOT EXISTS idx_agent_request_nonces_org_expires_at
    ON agent_request_nonces (organization_id, expires_at);

COMMENT ON TABLE agent_request_nonces IS
    'Nonces of admitted signed action-request statements; a row lives until the first purge after expires_at + 31 s';
