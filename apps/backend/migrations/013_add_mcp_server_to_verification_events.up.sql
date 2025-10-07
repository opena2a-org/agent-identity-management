-- Add mcp_server_id to verification_events to support MCP server verifications
ALTER TABLE verification_events
ADD COLUMN mcp_server_id UUID NULL;

-- Add index for mcp_server_id lookups
CREATE INDEX idx_verification_events_mcp_server_id ON verification_events(mcp_server_id);

-- Add foreign key constraint to mcp_servers table
ALTER TABLE verification_events
ADD CONSTRAINT verification_events_mcp_server_id_fkey
FOREIGN KEY (mcp_server_id) REFERENCES mcp_servers(id) ON DELETE CASCADE;

-- Make agent_id nullable since we now support both agent and MCP server verifications
ALTER TABLE verification_events
ALTER COLUMN agent_id DROP NOT NULL;

-- Add check constraint to ensure either agent_id or mcp_server_id is set (not both, not neither)
ALTER TABLE verification_events
ADD CONSTRAINT verification_events_target_check
CHECK (
  (agent_id IS NOT NULL AND mcp_server_id IS NULL) OR
  (agent_id IS NULL AND mcp_server_id IS NOT NULL)
);

-- Update agent_name column to be nullable as well
ALTER TABLE verification_events
ALTER COLUMN agent_name DROP NOT NULL;
