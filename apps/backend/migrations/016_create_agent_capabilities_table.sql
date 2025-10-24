-- Migration: Create agent_capabilities table
-- Purpose: Store agent capabilities with scope and permissions

CREATE TABLE IF NOT EXISTS agent_capabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability_type VARCHAR(100) NOT NULL,
    capability_scope JSONB DEFAULT '{}'::jsonb,
    granted_by UUID REFERENCES users(id),
    granted_at TIMESTAMP NOT NULL DEFAULT NOW(),
    revoked_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_agent_capabilities_agent_id ON agent_capabilities(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_capabilities_capability_type ON agent_capabilities(capability_type);
CREATE INDEX IF NOT EXISTS idx_agent_capabilities_granted_by ON agent_capabilities(granted_by);
CREATE INDEX IF NOT EXISTS idx_agent_capabilities_revoked_at ON agent_capabilities(revoked_at);
CREATE INDEX IF NOT EXISTS idx_agent_capabilities_created_at ON agent_capabilities(created_at DESC);

-- Add updated_at trigger
CREATE TRIGGER update_agent_capabilities_updated_at BEFORE UPDATE ON agent_capabilities
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
