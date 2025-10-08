-- Add talks_to column for simple capability tracking
-- talks_to stores a list of MCP server names/IDs this agent is allowed to communicate with
ALTER TABLE agents ADD COLUMN IF NOT EXISTS talks_to JSONB DEFAULT '[]'::JSONB;

-- Add index for better query performance
CREATE INDEX IF NOT EXISTS idx_agents_talks_to ON agents USING GIN (talks_to);

-- Add comment
COMMENT ON COLUMN agents.talks_to IS 'List of MCP server names/IDs this agent is allowed to communicate with (capability-based access control)';
