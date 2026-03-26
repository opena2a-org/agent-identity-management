-- Migration 080: Access Attestations (Level 9)
-- Immutable audit log for every authorized access.
-- Written to transparency log for verifiable history.

CREATE TABLE IF NOT EXISTS access_attestations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL,
    atc_version TEXT DEFAULT '',
    capability TEXT NOT NULL,
    resource_path TEXT DEFAULT '',
    attributes_accessed TEXT[] DEFAULT '{}',
    drift_score_at FLOAT DEFAULT 0.0,
    intent_class TEXT DEFAULT '',
    intent_confidence FLOAT DEFAULT 0.0,
    bac_hash TEXT DEFAULT '',                  -- behavioral attestation claim hash
    fga_outcome TEXT NOT NULL CHECK (fga_outcome IN ('ALLOW', 'DENY', 'DENY_INTENT', 'DENY_CONTEXT', 'DENY_CHAIN', 'DENY_ATTRIBUTE')),
    fga_steps_triggered TEXT[] DEFAULT '{}',
    transparency_log_idx BIGINT,               -- index in Registry transparency log
    accessed_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_access_attestations_agent
    ON access_attestations (agent_id, accessed_at DESC);
CREATE INDEX IF NOT EXISTS idx_access_attestations_outcome
    ON access_attestations (fga_outcome) WHERE fga_outcome != 'ALLOW';
