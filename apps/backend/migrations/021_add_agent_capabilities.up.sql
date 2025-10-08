-- Add capabilities column for agent capability tracking
-- capabilities stores a list of agent capabilities (e.g., ["file:read", "file:write", "api:call"])
ALTER TABLE agents ADD COLUMN IF NOT EXISTS capabilities JSONB DEFAULT '[]'::JSONB;

-- Add index for better query performance
CREATE INDEX IF NOT EXISTS idx_agents_capabilities ON agents USING GIN (capabilities);

-- Add comment
COMMENT ON COLUMN agents.capabilities IS 'List of agent capabilities (e.g., ["file:read", "api:call"]) for fine-grained permission control';
