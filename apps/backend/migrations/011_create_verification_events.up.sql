-- Create verification_events table (AIVF-style monitoring)
-- This tracks real-time cryptographic verification events for security monitoring
CREATE TABLE IF NOT EXISTS verification_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    agent_name VARCHAR(255) NOT NULL,

    -- Verification details
    protocol VARCHAR(50) NOT NULL, -- MCP, A2A, ACP, DID, OAuth, SAML
    verification_type VARCHAR(50) NOT NULL, -- identity, capability, permission, trust
    status VARCHAR(50) NOT NULL, -- success, failed, pending, timeout
    result VARCHAR(50), -- verified, denied, expired

    -- Cryptographic proof (for audit trail)
    signature TEXT,
    message_hash TEXT,
    nonce VARCHAR(255),
    public_key TEXT,

    -- Metrics
    confidence DECIMAL(5, 4) DEFAULT 0.0000 CHECK (confidence >= 0 AND confidence <= 1),
    trust_score DECIMAL(5, 2) DEFAULT 0.00 CHECK (trust_score >= 0 AND trust_score <= 100),
    duration_ms INTEGER NOT NULL DEFAULT 0,

    -- Error handling
    error_code VARCHAR(50),
    error_reason TEXT,

    -- Initiator information (who triggered this verification)
    initiator_type VARCHAR(50), -- user, agent, system, scheduler
    initiator_id UUID,
    initiator_name VARCHAR(255),
    initiator_ip INET,

    -- Context
    action VARCHAR(255), -- What action was being verified
    resource_type VARCHAR(100), -- What resource was accessed
    resource_id VARCHAR(255),
    location VARCHAR(255), -- Geographic location or service endpoint

    -- Timestamps
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Additional data
    details TEXT,
    metadata JSONB DEFAULT '{}'::jsonb
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_verification_events_organization_id ON verification_events(organization_id);
CREATE INDEX IF NOT EXISTS idx_verification_events_agent_id ON verification_events(agent_id);
CREATE INDEX IF NOT EXISTS idx_verification_events_protocol ON verification_events(protocol);
CREATE INDEX IF NOT EXISTS idx_verification_events_status ON verification_events(status);
CREATE INDEX IF NOT EXISTS idx_verification_events_created_at ON verification_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_verification_events_verification_type ON verification_events(verification_type);
CREATE INDEX IF NOT EXISTS idx_verification_events_initiator ON verification_events(initiator_type, initiator_id);

-- Add GIN index for metadata JSON queries
CREATE INDEX IF NOT EXISTS idx_verification_events_metadata ON verification_events USING GIN (metadata);

-- Add comments
COMMENT ON TABLE verification_events IS 'Real-time cryptographic verification events for security monitoring (AIVF-style)';
COMMENT ON COLUMN verification_events.protocol IS 'Verification protocol: MCP, A2A, ACP, DID, OAuth, SAML';
COMMENT ON COLUMN verification_events.verification_type IS 'Type of verification: identity, capability, permission, trust';
COMMENT ON COLUMN verification_events.status IS 'Verification status: success, failed, pending, timeout';
COMMENT ON COLUMN verification_events.signature IS 'Cryptographic signature for audit trail';
COMMENT ON COLUMN verification_events.confidence IS 'Confidence level of verification (0.0-1.0)';
COMMENT ON COLUMN verification_events.trust_score IS 'Trust score at time of verification (0-100)';
COMMENT ON COLUMN verification_events.duration_ms IS 'Time taken to complete verification in milliseconds';
COMMENT ON COLUMN verification_events.initiator_type IS 'Who triggered verification: user, agent, system, scheduler';
COMMENT ON COLUMN verification_events.action IS 'Action being verified (e.g., File Access, API Call, Data Query)';
COMMENT ON COLUMN verification_events.metadata IS 'Additional metadata about the verification event';
