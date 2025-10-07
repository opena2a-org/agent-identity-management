-- Drop index
DROP INDEX IF EXISTS idx_users_force_password_change;

-- Remove force_password_change column
ALTER TABLE users DROP COLUMN IF EXISTS force_password_change;
