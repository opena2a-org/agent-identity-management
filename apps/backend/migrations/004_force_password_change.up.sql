-- Add force_password_change column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS force_password_change BOOLEAN NOT NULL DEFAULT FALSE;

-- Create index for efficient queries
CREATE INDEX IF NOT EXISTS idx_users_force_password_change ON users(force_password_change) WHERE force_password_change = TRUE;

-- Add comment
COMMENT ON COLUMN users.force_password_change IS 'Indicates if user must change password on next login';
