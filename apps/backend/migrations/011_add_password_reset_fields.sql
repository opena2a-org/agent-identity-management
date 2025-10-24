-- Migration 011: Add password reset expiration field
-- This migration adds the password_reset_expires field to support password reset flow

-- Add password_reset_expires column if it doesn't exist
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_reset_expires TIMESTAMPTZ;

-- Create index for efficient token expiration checks
CREATE INDEX IF NOT EXISTS idx_users_password_reset_expires ON users(password_reset_expires) WHERE password_reset_expires IS NOT NULL;

-- Add comment
COMMENT ON COLUMN users.password_reset_expires IS 'Expiration timestamp for password reset token';

-- Log migration completion
DO $$
BEGIN
    RAISE NOTICE '✅ Migration 011 completed: Added password_reset_expires field';
END $$;
