-- Migration: Add password-related columns to users table
-- Created: 2025-10-20
-- Purpose: Add missing password authentication columns

-- Add password_hash column for storing hashed passwords
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT;

-- Add email_verified column for email verification workflow
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified BOOLEAN DEFAULT FALSE;

-- Add force_password_change column for password rotation
ALTER TABLE users ADD COLUMN IF NOT EXISTS force_password_change BOOLEAN DEFAULT FALSE;

-- Create index on email for login lookups (if not exists)
CREATE INDEX IF NOT EXISTS idx_users_email_hash ON users(email, password_hash);

-- Create index on email_verified for filtering
CREATE INDEX IF NOT EXISTS idx_users_email_verified ON users(email_verified);

-- Add comments explaining columns
COMMENT ON COLUMN users.password_hash IS 'Bcrypt hashed password for local authentication (NULL for OAuth-only users)';
COMMENT ON COLUMN users.email_verified IS 'Whether user email has been verified';
COMMENT ON COLUMN users.force_password_change IS 'Whether user must change password on next login';
