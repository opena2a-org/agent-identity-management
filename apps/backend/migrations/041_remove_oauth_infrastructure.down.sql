-- Rollback Migration 041: Remove OAuth Infrastructure
-- Description: Recreate OAuth tables, columns, and indexes if rollback is needed

-- Recreate OAuth columns in user_registration_requests
ALTER TABLE user_registration_requests
ADD COLUMN IF NOT EXISTS oauth_provider VARCHAR(50),
ADD COLUMN IF NOT EXISTS oauth_user_id TEXT,
ADD COLUMN IF NOT EXISTS profile_picture_url TEXT,
ADD COLUMN IF NOT EXISTS oauth_email_verified BOOLEAN DEFAULT FALSE,
ADD COLUMN IF NOT EXISTS metadata JSONB;

-- Recreate OAuth columns in users
ALTER TABLE users
ADD COLUMN IF NOT EXISTS oauth_provider VARCHAR(50),
ADD COLUMN IF NOT EXISTS oauth_user_id TEXT,
ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT FALSE;

-- Recreate oauth_connections table
CREATE TABLE IF NOT EXISTS oauth_connections (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider VARCHAR(50) NOT NULL,
    provider_user_id TEXT NOT NULL,
    access_token_hash TEXT,
    refresh_token_hash TEXT,
    access_token_expires_at TIMESTAMPTZ,
    email TEXT,
    email_verified BOOLEAN DEFAULT FALSE,
    profile_data JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    CONSTRAINT unique_oauth_connection UNIQUE (provider, provider_user_id)
);

-- Recreate indexes
CREATE INDEX IF NOT EXISTS idx_registration_oauth_unique
ON user_registration_requests(oauth_provider, oauth_user_id)
WHERE oauth_provider IS NOT NULL AND oauth_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_oauth_provider
ON users(oauth_provider, oauth_user_id)
WHERE oauth_provider IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_registration_requests_provider
ON user_registration_requests(oauth_provider)
WHERE oauth_provider IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_oauth_connections_user
ON oauth_connections(user_id);

CREATE INDEX IF NOT EXISTS idx_oauth_connections_provider
ON oauth_connections(provider, provider_user_id);

-- Drop the simplified email-only unique index
DROP INDEX IF EXISTS idx_registration_email_unique;

-- Recreate the original unique constraint
CREATE UNIQUE INDEX IF NOT EXISTS idx_registration_email_pending_unique
ON user_registration_requests(LOWER(email))
WHERE status = 'pending';

-- Update table comment
COMMENT ON TABLE user_registration_requests IS 'User registration requests (OAuth and email/password) pending admin approval';
