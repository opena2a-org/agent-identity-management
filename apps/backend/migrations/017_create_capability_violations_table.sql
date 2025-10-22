-- Migration: Create capability_violations table
-- Purpose: Track capability violations for security monitoring and compliance

CREATE TABLE IF NOT EXISTS capability_violations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    attempted_capability VARCHAR(100) NOT NULL,
    registered_capabilities JSONB DEFAULT '{}'::jsonb,
    severity VARCHAR(50) NOT NULL,
    trust_score_impact INTEGER NOT NULL DEFAULT 0,
    is_blocked BOOLEAN NOT NULL DEFAULT TRUE,
    source_ip VARCHAR(45),
    request_metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_capability_violations_agent_id ON capability_violations(agent_id);
CREATE INDEX IF NOT EXISTS idx_capability_violations_severity ON capability_violations(severity);
CREATE INDEX IF NOT EXISTS idx_capability_violations_is_blocked ON capability_violations(is_blocked);
CREATE INDEX IF NOT EXISTS idx_capability_violations_created_at ON capability_violations(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_capability_violations_source_ip ON capability_violations(source_ip);
