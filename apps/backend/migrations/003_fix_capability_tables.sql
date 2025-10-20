-- Migration 003: Fix missing agent_capabilities and capability_violations tables
-- These tables were defined in 007_add_capability_verification.sql but may not have been applied
-- This ensures they exist for capability request functionality

-- Create agent_capabilities table if it doesn't exist
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

-- Create capability_violations table if it doesn't exist
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

-- Add comments for documentation
COMMENT ON TABLE agent_capabilities IS 'Stores registered capabilities for agents and MCP servers';
COMMENT ON TABLE capability_violations IS 'Tracks attempts to perform actions outside registered capability scope';
