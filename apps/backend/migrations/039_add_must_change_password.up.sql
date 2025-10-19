-- Add force_password_change field to users table if it doesn't exist
-- Used to force password change on first login (e.g., default admin user)

ALTER TABLE users
ADD COLUMN IF NOT EXISTS force_password_change BOOLEAN DEFAULT FALSE;

-- Add index for quick lookup of users who must change password
CREATE INDEX IF NOT EXISTS idx_users_force_password_change
ON users(force_password_change)
WHERE force_password_change = TRUE;

-- Update comment
COMMENT ON COLUMN users.force_password_change IS 'If true, user must change password on next login (used for default admin account)';
