-- Rollback migration: Remove mcp_server_id support from verification_events

-- Remove check constraint
ALTER TABLE verification_events
DROP CONSTRAINT IF EXISTS verification_events_target_check;

-- Make agent_name NOT NULL again
ALTER TABLE verification_events
ALTER COLUMN agent_name SET NOT NULL;

-- Make agent_id NOT NULL again (this will fail if there are records with NULL agent_id)
ALTER TABLE verification_events
ALTER COLUMN agent_id SET NOT NULL;

-- Remove foreign key constraint
ALTER TABLE verification_events
DROP CONSTRAINT IF EXISTS verification_events_mcp_server_id_fkey;

-- Remove index
DROP INDEX IF EXISTS idx_verification_events_mcp_server_id;

-- Remove column
ALTER TABLE verification_events
DROP COLUMN IF EXISTS mcp_server_id;
