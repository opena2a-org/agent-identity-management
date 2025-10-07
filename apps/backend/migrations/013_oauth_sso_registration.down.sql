-- Rollback OAuth/SSO registration tables

-- Drop indexes first
DROP INDEX IF EXISTS idx_users_oauth_provider;
DROP INDEX IF EXISTS idx_oauth_connections_provider;
DROP INDEX IF EXISTS idx_oauth_connections_user;
DROP INDEX IF EXISTS idx_registration_requests_provider;
DROP INDEX IF EXISTS idx_registration_requests_email;
DROP INDEX IF EXISTS idx_registration_requests_org;
DROP INDEX IF EXISTS idx_registration_requests_status;

-- Drop columns from users table
ALTER TABLE users DROP COLUMN IF EXISTS email_verified;
ALTER TABLE users DROP COLUMN IF EXISTS oauth_user_id;
ALTER TABLE users DROP COLUMN IF EXISTS oauth_provider;

-- Drop tables
DROP TABLE IF EXISTS oauth_connections;
DROP TABLE IF EXISTS user_registration_requests;
