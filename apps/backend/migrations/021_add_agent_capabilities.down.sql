-- Rollback: Remove capabilities column
DROP INDEX IF EXISTS idx_agents_capabilities;
ALTER TABLE agents DROP COLUMN IF EXISTS capabilities;
