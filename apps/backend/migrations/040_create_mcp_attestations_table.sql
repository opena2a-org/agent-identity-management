-- Migration: Create mcp_attestations table for Agent Attestation (THE KEY INNOVATION)
-- Created: 2025-10-23
-- Purpose: Store cryptographically signed attestations from verified agents confirming MCP identity

CREATE TABLE IF NOT EXISTS mcp_attestations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,

    -- Attestation payload (what the agent verified)
    -- Example:
    -- {
    --   "agent_id": "uuid",
    --   "mcp_url": "https://api.anthropic.com/mcp",
    --   "mcp_name": "Anthropic MCP",
    --   "capabilities_found": ["prompt", "completion", "tool_use"],
    --   "connection_successful": true,
    --   "health_check_passed": true,
    --   "connection_latency_ms": 45,
    --   "timestamp": "2025-10-23T18:00:00Z",
    --   "sdk_version": "1.0.0"
    -- }
    attestation_data JSONB NOT NULL,

    -- Ed25519 signature of attestation_data (signed by agent's private key)
    signature TEXT NOT NULL,

    -- Verification status
    signature_verified BOOLEAN DEFAULT FALSE,
    verified_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ NOT NULL,
    is_valid BOOLEAN DEFAULT TRUE,

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- One attestation per agent per MCP per time period
    -- We allow re-attestation by same agent at different times
    CONSTRAINT unique_attestation_per_verification UNIQUE(mcp_server_id, agent_id, verified_at)
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_mcp_attestations_mcp ON mcp_attestations(mcp_server_id);
CREATE INDEX IF NOT EXISTS idx_mcp_attestations_agent ON mcp_attestations(agent_id);
CREATE INDEX IF NOT EXISTS idx_mcp_attestations_valid ON mcp_attestations(is_valid, verified_at);
CREATE INDEX IF NOT EXISTS idx_mcp_attestations_expires ON mcp_attestations(expires_at);
CREATE INDEX IF NOT EXISTS idx_mcp_attestations_verified ON mcp_attestations(signature_verified);

-- Composite index for confidence score calculation (valid attestations for an MCP)
CREATE INDEX IF NOT EXISTS idx_mcp_attestations_mcp_valid ON mcp_attestations(mcp_server_id, is_valid, verified_at);

-- Add comments
COMMENT ON TABLE mcp_attestations IS 'Cryptographically signed attestations from verified agents confirming MCP identity and functionality';
COMMENT ON COLUMN mcp_attestations.attestation_data IS 'JSON payload containing what the agent verified (capabilities, connection test, health check, etc.)';
COMMENT ON COLUMN mcp_attestations.signature IS 'Ed25519 signature of attestation_data signed with agent private key';
COMMENT ON COLUMN mcp_attestations.signature_verified IS 'Whether the signature has been verified against agent public key';
COMMENT ON COLUMN mcp_attestations.verified_at IS 'When the attestation signature was verified by AIM backend';
COMMENT ON COLUMN mcp_attestations.expires_at IS 'When this attestation expires (default: 30 days from verification)';
COMMENT ON COLUMN mcp_attestations.is_valid IS 'Whether this attestation is still valid (not expired, not revoked)';
