-- Migration 142: challenges that an agent signs to show it holds its private key.
--
-- POST /api/v1/agents/registration-challenges issues a random 32-byte challenge,
-- bound to the organization and the user that asked, valid for five minutes. A
-- registration that names the challenge in its possessionProof consumes it once,
-- whatever the outcome of the check: used_at is set by the first registration
-- that presents the id and never cleared.
--
-- The challenge is stored as the raw bytes. An expired row is deleted by the
-- expiration cleanup job that runs every five minutes.
--
-- Re-running this migration keeps the table and the index.

CREATE TABLE IF NOT EXISTS agent_registration_challenges (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    principal_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    challenge       BYTEA NOT NULL CHECK (octet_length(challenge) = 32),
    issued_at       TIMESTAMPTZ NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    used_at         TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_agent_registration_challenges_expires_at
    ON agent_registration_challenges (expires_at);
