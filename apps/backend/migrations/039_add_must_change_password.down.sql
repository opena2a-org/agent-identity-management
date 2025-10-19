-- Rollback: Remove force_password_change field from users table

DROP INDEX IF EXISTS idx_users_force_password_change;

ALTER TABLE users
DROP COLUMN IF EXISTS force_password_change;
