-- A2A Agent-to-Agent Attestations
-- Migration 067: Multi-agent consensus for skill verification

-- ============================================================================
-- A2A Agent Attestations
-- Agents attest to each other's skill capabilities after successful tasks
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_agent_attestations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    -- Participants
    attesting_agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    attested_agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- What was attested
    skill_id VARCHAR(100),
    task_id UUID REFERENCES a2a_tasks(id),

    -- Attestation result
    task_completed BOOLEAN NOT NULL DEFAULT TRUE,
    response_time_ms INT,
    quality_score FLOAT,  -- 0-1, optional subjective rating

    -- Cryptographic proof
    attestation_signature TEXT NOT NULL,
    challenge VARCHAR(64),

    -- Trust weight (attesting agent's trust at time of attestation)
    attester_trust_score FLOAT,

    -- Validity
    verified_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    is_valid BOOLEAN DEFAULT TRUE,
    revoked_at TIMESTAMPTZ,
    revoked_reason TEXT,

    created_at TIMESTAMPTZ DEFAULT NOW(),

    CONSTRAINT no_self_attestation CHECK (attesting_agent_id != attested_agent_id)
);

CREATE INDEX IF NOT EXISTS idx_a2a_attestations_attested ON a2a_agent_attestations(attested_agent_id, is_valid);
CREATE INDEX IF NOT EXISTS idx_a2a_attestations_skill ON a2a_agent_attestations(attested_agent_id, skill_id) WHERE is_valid = TRUE;
CREATE INDEX IF NOT EXISTS idx_a2a_attestations_expiry ON a2a_agent_attestations(expires_at) WHERE is_valid = TRUE;

-- ============================================================================
-- A2A Revoked Agents
-- Quick lookup for compromised/banned agents
-- ============================================================================
CREATE TABLE IF NOT EXISTS a2a_revoked_agents (
    agent_id UUID PRIMARY KEY REFERENCES agents(id) ON DELETE CASCADE,
    revoked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reason TEXT,
    revoked_by UUID REFERENCES users(id)
);

CREATE INDEX IF NOT EXISTS idx_a2a_revoked_agents ON a2a_revoked_agents(revoked_at DESC);

-- ============================================================================
-- Add verified status to skills
-- ============================================================================
ALTER TABLE a2a_skills ADD COLUMN IF NOT EXISTS is_verified BOOLEAN DEFAULT FALSE;
ALTER TABLE a2a_skills ADD COLUMN IF NOT EXISTS verified_at TIMESTAMPTZ;
ALTER TABLE a2a_skills ADD COLUMN IF NOT EXISTS attestation_count INT DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_a2a_skills_verified ON a2a_skills(agent_id, is_verified) WHERE is_verified = TRUE;

-- ============================================================================
-- Comments
-- ============================================================================
COMMENT ON TABLE a2a_agent_attestations IS 'Agent-to-agent attestations for multi-agent consensus verification';
COMMENT ON TABLE a2a_revoked_agents IS 'List of revoked agents for quick lookup during A2A verification';
COMMENT ON COLUMN a2a_skills.is_verified IS 'True when skill has reached consensus threshold of attestations';
