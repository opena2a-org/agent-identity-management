-- Migration: Add missing user table columns
-- Created: 2025-10-20
-- Purpose: Add all missing columns that were added manually during deployment

-- Add status column for user account status tracking
ALTER TABLE users ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'active';

-- Add deleted_at for soft deletes
ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ;

-- Add approval tracking columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS approved_by UUID;
ALTER TABLE users ADD COLUMN IF NOT EXISTS approved_at TIMESTAMPTZ;

-- Add password reset columns
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_reset_token VARCHAR(255);
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_reset_expires_at TIMESTAMPTZ;

-- Create index on status for filtering
CREATE INDEX IF NOT EXISTS idx_users_status ON users(status);

-- Create index on password_reset_token for lookups
CREATE INDEX IF NOT EXISTS idx_users_password_reset_token ON users(password_reset_token);

-- Add foreign key constraint for approved_by
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'users_approved_by_fkey'
        AND table_name = 'users'
    ) THEN
        ALTER TABLE users ADD CONSTRAINT users_approved_by_fkey
        FOREIGN KEY (approved_by) REFERENCES users(id);
    END IF;
END$$;

-- Add comment explaining status values
COMMENT ON COLUMN users.status IS 'User account status: active, pending_approval, deactivated';

-- Add comment explaining password reset token
COMMENT ON COLUMN users.password_reset_token IS 'Token for password reset workflow (hashed)';
COMMENT ON COLUMN users.password_reset_expires_at IS 'Expiration time for password reset token';
