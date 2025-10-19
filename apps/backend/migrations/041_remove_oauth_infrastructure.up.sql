-- Migration 041: Remove OAuth Infrastructure
-- Description: Remove all OAuth tables, columns, and indexes after transitioning to email/password authentication

-- Remove OAuth tables
DROP TABLE IF EXISTS oauth_connections CASCADE;

-- Remove OAuth columns from user_registration_requests
ALTER TABLE user_registration_requests
DROP COLUMN IF EXISTS oauth_provider,
DROP COLUMN IF EXISTS oauth_user_id,
DROP COLUMN IF EXISTS profile_picture_url,
DROP COLUMN IF EXISTS oauth_email_verified,
DROP COLUMN IF EXISTS metadata;

-- Add password_hash column for email/password authentication
ALTER TABLE user_registration_requests
ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- Remove OAuth columns from users
ALTER TABLE users
DROP COLUMN IF EXISTS oauth_provider,
DROP COLUMN IF EXISTS oauth_user_id,
DROP COLUMN IF EXISTS email_verified,
DROP COLUMN IF EXISTS provider,
DROP COLUMN IF EXISTS provider_id;

-- Drop OAuth-specific indexes and constraints
DROP INDEX IF EXISTS idx_registration_oauth_unique;
DROP INDEX IF EXISTS idx_users_oauth_provider;
DROP INDEX IF EXISTS idx_registration_requests_provider;
DROP INDEX IF EXISTS idx_oauth_connections_user;
DROP INDEX IF EXISTS idx_oauth_connections_provider;
DROP INDEX IF EXISTS idx_users_provider;

-- Drop unique constraint on provider/provider_id (no longer needed)
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_provider_provider_id_key;

-- Simplify unique constraint for email-based registrations
-- Drop old index if it exists
DROP INDEX IF EXISTS idx_registration_email_pending_unique;

-- Create new simplified unique index for email-based registrations
CREATE UNIQUE INDEX idx_registration_email_unique
ON user_registration_requests(LOWER(email))
WHERE status = 'pending';

-- Update table comment
COMMENT ON TABLE user_registration_requests IS 'Email-based user registration requests pending admin approval';
