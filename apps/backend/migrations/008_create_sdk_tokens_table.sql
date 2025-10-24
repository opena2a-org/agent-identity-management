-- Migration: Create sdk_tokens table
-- Created: 2025-10-20
-- Purpose: Add SDK token tracking table for refresh token management

CREATE TABLE IF NOT EXISTS sdk_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    token_id VARCHAR(255) NOT NULL UNIQUE,
    device_name TEXT,
    device_fingerprint TEXT,
    ip_address VARCHAR(45),
    user_agent TEXT,
    last_used_at TIMESTAMPTZ,
    last_ip_address VARCHAR(45),
    usage_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revoke_reason TEXT,
    metadata JSONB
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_user_id ON sdk_tokens(user_id);
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_organization_id ON sdk_tokens(organization_id);
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_token_hash ON sdk_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_token_id ON sdk_tokens(token_id);
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_expires_at ON sdk_tokens(expires_at) WHERE revoked_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_sdk_tokens_revoked_at ON sdk_tokens(revoked_at) WHERE revoked_at IS NOT NULL;

-- Add comments
COMMENT ON TABLE sdk_tokens IS 'SDK refresh tokens for secure token management and revocation';
COMMENT ON COLUMN sdk_tokens.token_hash IS 'SHA-256 hash of the refresh token';
COMMENT ON COLUMN sdk_tokens.token_id IS 'JWT token ID (jti claim) for lookup';
COMMENT ON COLUMN sdk_tokens.device_name IS 'User-provided device name for identification';
COMMENT ON COLUMN sdk_tokens.device_fingerprint IS 'Browser/device fingerprint for security';
COMMENT ON COLUMN sdk_tokens.usage_count IS 'Number of times token has been used';
COMMENT ON COLUMN sdk_tokens.metadata IS 'Additional token metadata in JSON format';
