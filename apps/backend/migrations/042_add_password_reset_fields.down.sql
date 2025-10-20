-- Drop password reset indexes
DROP INDEX IF EXISTS idx_users_password_reset_expires_at;
DROP INDEX IF EXISTS idx_users_password_reset_token;

-- Drop password reset columns
ALTER TABLE users DROP COLUMN IF EXISTS password_reset_expires_at;
ALTER TABLE users DROP COLUMN IF EXISTS password_reset_token;
