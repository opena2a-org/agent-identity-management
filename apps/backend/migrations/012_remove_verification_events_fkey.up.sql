-- Drop foreign key constraint on agent_id since verification events
-- can be for agents, MCP servers, or other entity types
ALTER TABLE verification_events DROP CONSTRAINT IF EXISTS verification_events_agent_id_fkey;

-- Add index on agent_id for query performance (without FK constraint)
CREATE INDEX IF NOT EXISTS idx_verification_events_agent_id ON verification_events(agent_id);
