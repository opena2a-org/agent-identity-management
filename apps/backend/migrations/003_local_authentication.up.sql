-- Migration for local authentication support
-- Adds password_hash field to users table and creates bootstrap tracking

-- Add password_hash column to users table (nullable for OAuth users)
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);

-- Add email_verified column for local users
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN NOT NULL DEFAULT FALSE;

-- Add password_reset_token and expiry for password reset flow
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_reset_token VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_reset_expires_at TIMESTAMP;

-- Create system_config table for bootstrap tracking
CREATE TABLE IF NOT EXISTS system_config (
    key VARCHAR(255) PRIMARY KEY,
    value TEXT NOT NULL,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Insert bootstrap status flag
INSERT INTO system_config (key, value, description)
VALUES ('bootstrap_completed', 'false', 'Indicates if initial admin bootstrap has been completed')
ON CONFLICT (key) DO NOTHING;

-- Insert system version
INSERT INTO system_config (key, value, description)
VALUES ('system_version', '1.0.0', 'Current system version')
ON CONFLICT (key) DO NOTHING;

-- Create index on password_reset_token for faster lookups
CREATE INDEX IF NOT EXISTS idx_users_password_reset_token ON users(password_reset_token);
CREATE INDEX IF NOT EXISTS idx_users_email_verified ON users(email_verified);

-- Update OAuth users to have email_verified = true
UPDATE users SET email_verified = TRUE WHERE provider IN ('google', 'microsoft', 'okta');

-- Comment on columns
COMMENT ON COLUMN users.password_hash IS 'Bcrypt hash of user password (null for OAuth-only users)';
COMMENT ON COLUMN users.email_verified IS 'Whether user email has been verified';
COMMENT ON COLUMN users.password_reset_token IS 'Token for password reset flow';
COMMENT ON COLUMN users.password_reset_expires_at IS 'Expiration timestamp for password reset token';
