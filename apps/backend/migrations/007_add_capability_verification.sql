-- Migration 007: Add Capability Verification System
-- This migration adds tables and columns for automatic capability verification

-- Add public key and capability tracking columns to agents table
ALTER TABLE agents
ADD COLUMN IF NOT EXISTS public_key TEXT,
ADD COLUMN IF NOT EXISTS key_algorithm VARCHAR(20) DEFAULT 'Ed25519',
ADD COLUMN IF NOT EXISTS last_capability_check_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS capability_violation_count INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS is_compromised BOOLEAN DEFAULT false;

-- Create agent_capabilities table
CREATE TABLE IF NOT EXISTS agent_capabilities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability_type VARCHAR(100) NOT NULL,
    capability_scope JSONB,
    granted_by UUID REFERENCES users(id),
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT unique_agent_capability UNIQUE(agent_id, capability_type)
);

CREATE INDEX IF NOT EXISTS idx_agent_capabilities_agent_id ON agent_capabilities(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_capabilities_type ON agent_capabilities(capability_type);
CREATE INDEX IF NOT EXISTS idx_agent_capabilities_revoked ON agent_capabilities(revoked_at) WHERE revoked_at IS NULL;

-- Create capability_violations table
CREATE TABLE IF NOT EXISTS capability_violations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    attempted_capability VARCHAR(100) NOT NULL,
    registered_capabilities JSONB,
    severity VARCHAR(20) NOT NULL DEFAULT 'medium',
    trust_score_impact INT NOT NULL DEFAULT -10,
    is_blocked BOOLEAN NOT NULL DEFAULT false,
    source_ip VARCHAR(45),
    request_metadata JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT valid_severity CHECK (severity IN ('low', 'medium', 'high', 'critical'))
);

CREATE INDEX IF NOT EXISTS idx_capability_violations_agent_id ON capability_violations(agent_id);
CREATE INDEX IF NOT EXISTS idx_capability_violations_created_at ON capability_violations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_capability_violations_severity ON capability_violations(severity);
CREATE INDEX IF NOT EXISTS idx_capability_violations_blocked ON capability_violations(is_blocked);

-- Add comment for documentation
COMMENT ON TABLE agent_capabilities IS 'Stores registered capabilities for agents and MCP servers';
COMMENT ON TABLE capability_violations IS 'Tracks attempts to perform actions outside registered capability scope';
COMMENT ON COLUMN agents.public_key IS 'Public key for signature verification (Ed25519, RSA, or ECDSA)';
COMMENT ON COLUMN agents.is_compromised IS 'Flag indicating potential compromise based on capability violations';
