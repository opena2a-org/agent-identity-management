-- Migration: Create verification events table
-- Created: 2025-10-20
-- Purpose: Add real-time verification event tracking for monitoring and analytics

-- Verification Events table
CREATE TABLE IF NOT EXISTS verification_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Target can be either an Agent or MCP Server (one must be set, not both)
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    agent_name VARCHAR(255),
    mcp_server_id UUID REFERENCES mcp_servers(id) ON DELETE CASCADE,
    mcp_server_name VARCHAR(255),

    -- Verification details
    protocol VARCHAR(50) NOT NULL,
    verification_type VARCHAR(50) NOT NULL,
    status VARCHAR(50) NOT NULL,
    result VARCHAR(50),

    -- Cryptographic proof
    signature TEXT,
    message_hash TEXT,
    nonce VARCHAR(255),
    public_key TEXT,

    -- Metrics
    confidence DECIMAL(5,4) DEFAULT 0.0000,
    trust_score DECIMAL(5,2) DEFAULT 0.00,
    duration_ms INTEGER DEFAULT 0,

    -- Error handling
    error_code VARCHAR(100),
    error_reason TEXT,

    -- Initiator information
    initiator_type VARCHAR(50) NOT NULL,
    initiator_id UUID,
    initiator_name VARCHAR(255),
    initiator_ip VARCHAR(45),

    -- Context
    action VARCHAR(255),
    resource_type VARCHAR(100),
    resource_id VARCHAR(255),
    location VARCHAR(255),

    -- Configuration Drift Detection (WHO and WHAT)
    current_mcp_servers JSONB DEFAULT '[]'::jsonb,
    current_capabilities JSONB DEFAULT '[]'::jsonb,
    drift_detected BOOLEAN DEFAULT FALSE,
    mcp_server_drift JSONB DEFAULT '[]'::jsonb,
    capability_drift JSONB DEFAULT '[]'::jsonb,

    -- Timestamps
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Additional data
    details TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,

    -- Constraints
    CHECK (
        (agent_id IS NOT NULL AND mcp_server_id IS NULL) OR
        (agent_id IS NULL AND mcp_server_id IS NOT NULL)
    )
);

-- Create indexes for verification events
CREATE INDEX IF NOT EXISTS idx_verification_events_organization_id ON verification_events(organization_id);
CREATE INDEX IF NOT EXISTS idx_verification_events_agent_id ON verification_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_verification_events_mcp_server_id ON verification_events(mcp_server_id);
CREATE INDEX IF NOT EXISTS idx_verification_events_status ON verification_events(status);
CREATE INDEX IF NOT EXISTS idx_verification_events_result ON verification_events(result);
CREATE INDEX IF NOT EXISTS idx_verification_events_protocol ON verification_events(protocol);
CREATE INDEX IF NOT EXISTS idx_verification_events_verification_type ON verification_events(verification_type);
CREATE INDEX IF NOT EXISTS idx_verification_events_initiator_type ON verification_events(initiator_type);
CREATE INDEX IF NOT EXISTS idx_verification_events_drift_detected ON verification_events(drift_detected);
CREATE INDEX IF NOT EXISTS idx_verification_events_created_at ON verification_events(created_at);
CREATE INDEX IF NOT EXISTS idx_verification_events_started_at ON verification_events(started_at);

-- GIN indexes for JSONB columns
CREATE INDEX IF NOT EXISTS idx_verification_events_current_mcp_servers ON verification_events USING gin(current_mcp_servers);
CREATE INDEX IF NOT EXISTS idx_verification_events_current_capabilities ON verification_events USING gin(current_capabilities);
CREATE INDEX IF NOT EXISTS idx_verification_events_mcp_server_drift ON verification_events USING gin(mcp_server_drift);
CREATE INDEX IF NOT EXISTS idx_verification_events_capability_drift ON verification_events USING gin(capability_drift);
CREATE INDEX IF NOT EXISTS idx_verification_events_metadata ON verification_events USING gin(metadata);

-- Add comments explaining verification event fields
COMMENT ON TABLE verification_events IS 'Real-time verification events for monitoring agent and MCP server activity';
COMMENT ON COLUMN verification_events.agent_id IS 'Agent being verified (mutually exclusive with mcp_server_id)';
COMMENT ON COLUMN verification_events.mcp_server_id IS 'MCP server being verified (mutually exclusive with agent_id)';
COMMENT ON COLUMN verification_events.protocol IS 'Verification protocol used: MCP, A2A, ACP, DID, OAuth, SAML';
COMMENT ON COLUMN verification_events.verification_type IS 'Type of verification: identity, capability, permission, trust';
COMMENT ON COLUMN verification_events.status IS 'Status of verification: success, failed, pending, timeout';
COMMENT ON COLUMN verification_events.result IS 'Result of verification: verified, denied, expired';
COMMENT ON COLUMN verification_events.confidence IS 'Confidence score (0.0000-1.0000) of the verification';
COMMENT ON COLUMN verification_events.trust_score IS 'Trust score (0.00-100.00) at time of verification';
COMMENT ON COLUMN verification_events.duration_ms IS 'Verification duration in milliseconds';
COMMENT ON COLUMN verification_events.current_mcp_servers IS 'JSONB array of MCP servers being communicated with at runtime';
COMMENT ON COLUMN verification_events.current_capabilities IS 'JSONB array of capabilities being used at runtime';
COMMENT ON COLUMN verification_events.drift_detected IS 'Whether configuration drift was detected';
COMMENT ON COLUMN verification_events.mcp_server_drift IS 'JSONB array of unregistered MCP servers detected';
COMMENT ON COLUMN verification_events.capability_drift IS 'JSONB array of undeclared capabilities detected';
COMMENT ON COLUMN verification_events.metadata IS 'Additional structured data as JSONB';
