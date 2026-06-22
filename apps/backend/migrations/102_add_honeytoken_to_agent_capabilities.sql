-- Migration 102: Mark a granted capability as a honeytoken (issue #293)
-- A honeytoken capability is a decoy that no legitimate workflow should ever
-- exercise. Any verification request that matches it is a high-confidence
-- compromise indicator: the verification path raises a high-severity alert and
-- records an audit event. The flag is operator-set only (never derived from the
-- capability type and never settable via the SDK self-register path), so it never
-- fires in normal operation. Defaults to FALSE so every existing grant is a
-- non-honeytoken and behaviour is unchanged until an operator opts a grant in.

ALTER TABLE agent_capabilities
    ADD COLUMN IF NOT EXISTS honeytoken BOOLEAN NOT NULL DEFAULT FALSE;

-- Partial index: honeytoken grants are rare, so index only the TRUE rows. Keeps
-- operator "which capabilities are honeytokens" lookups cheap without weighing on
-- the hot verification path (which filters by agent_id, not honeytoken).
CREATE INDEX IF NOT EXISTS idx_agent_capabilities_honeytoken
    ON agent_capabilities (agent_id)
    WHERE honeytoken = TRUE;
