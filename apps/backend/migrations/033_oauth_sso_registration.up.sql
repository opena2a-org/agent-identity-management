-- OAuth/SSO and User Self-Registration
-- Enables enterprise SSO integration (Google, Microsoft, Okta)
-- and self-service user registration with admin approval

-- User registration requests (pending admin approval)
CREATE TABLE IF NOT EXISTS user_registration_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) NOT NULL,
    first_name VARCHAR(100),
    last_name VARCHAR(100),
    oauth_provider VARCHAR(50) NOT NULL, -- google, microsoft, okta
    oauth_user_id VARCHAR(255) NOT NULL, -- provider's user ID
    organization_id UUID REFERENCES organizations(id),
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, approved, rejected
    requested_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by UUID REFERENCES users(id),
    rejection_reason TEXT,

    -- OAuth profile data (for admin review)
    profile_picture_url TEXT,
    oauth_email_verified BOOLEAN DEFAULT FALSE,

    -- Additional metadata
    metadata JSONB, -- store additional OAuth profile data

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(oauth_provider, oauth_user_id)
);

-- OAuth connections (linked to existing users)
CREATE TABLE IF NOT EXISTS oauth_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL, -- google, microsoft, okta
    provider_user_id VARCHAR(255) NOT NULL, -- provider's unique user ID
    provider_email VARCHAR(255),

    -- Security: Store only hashed tokens
    access_token_hash VARCHAR(255), -- SHA-256 hash of access token
    refresh_token_hash VARCHAR(255), -- SHA-256 hash of refresh token
    token_expires_at TIMESTAMPTZ,

    -- OAuth profile data
    profile_data JSONB, -- store full OAuth profile for reference

    -- Tracking
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(provider, provider_user_id),
    UNIQUE(user_id, provider) -- one connection per provider per user
);

-- Indexes for performance
CREATE INDEX idx_registration_requests_status ON user_registration_requests(status);
CREATE INDEX idx_registration_requests_org ON user_registration_requests(organization_id);
CREATE INDEX idx_registration_requests_email ON user_registration_requests(email);
CREATE INDEX idx_registration_requests_provider ON user_registration_requests(oauth_provider);
CREATE INDEX idx_oauth_connections_user ON oauth_connections(user_id);
CREATE INDEX idx_oauth_connections_provider ON oauth_connections(provider);

-- Add OAuth provider to users table (nullable for backward compatibility)
ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_provider VARCHAR(50);
ALTER TABLE users ADD COLUMN IF NOT EXISTS oauth_user_id VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT FALSE;

-- Create index for OAuth lookups
CREATE INDEX IF NOT EXISTS idx_users_oauth_provider ON users(oauth_provider, oauth_user_id) WHERE oauth_provider IS NOT NULL;

-- Comments for documentation
COMMENT ON TABLE user_registration_requests IS 'Self-service registration requests pending admin approval';
COMMENT ON TABLE oauth_connections IS 'OAuth/SSO connections linked to user accounts';
COMMENT ON COLUMN oauth_connections.access_token_hash IS 'SHA-256 hash of OAuth access token (never store plain text)';
COMMENT ON COLUMN oauth_connections.refresh_token_hash IS 'SHA-256 hash of OAuth refresh token (never store plain text)';
