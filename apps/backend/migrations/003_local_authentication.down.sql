-- Rollback migration for local authentication

-- Drop indexes
DROP INDEX IF EXISTS idx_users_email_verified;
DROP INDEX IF EXISTS idx_users_password_reset_token;

-- Drop system_config table
DROP TABLE IF EXISTS system_config;

-- Remove columns from users table
ALTER TABLE users DROP COLUMN IF EXISTS password_reset_expires_at;
ALTER TABLE users DROP COLUMN IF EXISTS password_reset_token;
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
