-- SDK Tokens Table for tracking and revoking SDK refresh tokens
-- This enables security features like token revocation and anomaly detection

CREATE TABLE IF NOT EXISTS sdk_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,

    -- Token identification (we store hash, not the actual token)
    token_hash TEXT NOT NULL UNIQUE,
    token_id TEXT NOT NULL UNIQUE,  -- JTI claim from JWT for easy lookup

    -- Device information
    device_name TEXT,
    device_fingerprint TEXT,

    -- Usage tracking
    ip_address TEXT,
    user_agent TEXT,
    last_used_at TIMESTAMPTZ,
    last_ip_address TEXT,
    usage_count INTEGER DEFAULT 0,

    -- Token lifecycle
    created_at TIMESTAMPTZ DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT,

    -- Metadata
    metadata JSONB DEFAULT '{}'::jsonb
);

-- Indexes for performance
CREATE INDEX idx_sdk_tokens_user_id ON sdk_tokens(user_id);
CREATE INDEX idx_sdk_tokens_organization_id ON sdk_tokens(organization_id);
CREATE INDEX idx_sdk_tokens_token_hash ON sdk_tokens(token_hash);
CREATE INDEX idx_sdk_tokens_token_id ON sdk_tokens(token_id);
CREATE INDEX idx_sdk_tokens_active ON sdk_tokens(user_id, expires_at) WHERE revoked_at IS NULL;
CREATE INDEX idx_sdk_tokens_last_used ON sdk_tokens(last_used_at DESC);

-- Add comments
COMMENT ON TABLE sdk_tokens IS 'Tracks SDK refresh tokens for security and revocation';
COMMENT ON COLUMN sdk_tokens.token_hash IS 'SHA-256 hash of refresh token for secure storage';
COMMENT ON COLUMN sdk_tokens.token_id IS 'JWT JTI claim for fast token lookup';
COMMENT ON COLUMN sdk_tokens.device_name IS 'User-friendly device name (e.g., "MacBook Pro")';
COMMENT ON COLUMN sdk_tokens.device_fingerprint IS 'Unique device identifier for anomaly detection';
COMMENT ON COLUMN sdk_tokens.usage_count IS 'Number of times token has been used';
COMMENT ON COLUMN sdk_tokens.revoke_reason IS 'Why token was revoked (user-initiated, security, expired, etc.)';
