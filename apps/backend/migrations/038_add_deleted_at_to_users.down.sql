-- Remove indexes
DROP INDEX IF EXISTS idx_users_org_deleted;
DROP INDEX IF EXISTS idx_users_deleted_at;

-- Remove column
ALTER TABLE users DROP COLUMN IF EXISTS deleted_at;
