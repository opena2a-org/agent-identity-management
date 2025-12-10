-- Add metadata column to agents table for custom agent properties
-- This column stores custom metadata like model, department, owner, environment
ALTER TABLE agents
ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';

-- Add comment for documentation
COMMENT ON COLUMN agents.metadata IS 'Custom metadata for the agent (e.g., model, department, owner, environment)';

-- Create index for efficient JSONB queries
CREATE INDEX IF NOT EXISTS idx_agents_metadata ON agents USING gin(metadata);
