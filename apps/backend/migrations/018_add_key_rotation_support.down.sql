-- Rollback key rotation support

-- Drop indexes
DROP INDEX IF EXISTS idx_agents_key_expires_at;
DROP INDEX IF EXISTS idx_agents_grace_period;

-- Remove columns
ALTER TABLE agents DROP COLUMN IF EXISTS key_created_at;
ALTER TABLE agents DROP COLUMN IF EXISTS key_expires_at;
ALTER TABLE agents DROP COLUMN IF EXISTS key_rotation_grace_until;
ALTER TABLE agents DROP COLUMN IF EXISTS previous_public_key;
ALTER TABLE agents DROP COLUMN IF EXISTS rotation_count;
