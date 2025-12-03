-- Migration: Create attestation_challenges table
-- Purpose: Store server-generated nonces for proof of private key possession
-- This prevents replay attacks and ensures agents actually possess their private keys
-- during attestation

CREATE TABLE IF NOT EXISTS attestation_challenges (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    challenge VARCHAR(64) NOT NULL UNIQUE,      -- Base64-encoded random nonce (32 bytes = 44 chars base64)
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,    -- Challenge expires after 5 minutes
    used_at TIMESTAMP WITH TIME ZONE,                -- NULL until challenge is consumed (replay prevention)
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Index for fast challenge lookup and validation
CREATE INDEX idx_attestation_challenges_challenge ON attestation_challenges(challenge);

-- Index for cleaning up expired/used challenges
CREATE INDEX idx_attestation_challenges_expires_at ON attestation_challenges(expires_at);
CREATE INDEX idx_attestation_challenges_used_at ON attestation_challenges(used_at);

-- Index for agent-specific challenge queries
CREATE INDEX idx_attestation_challenges_agent_id ON attestation_challenges(agent_id);

-- Cleanup job: Delete challenges older than 1 hour (expired + buffer)
-- This can be called periodically from application code
COMMENT ON TABLE attestation_challenges IS 'Server-generated nonces for proof of private key possession during MCP attestation. Prevents replay attacks.';
