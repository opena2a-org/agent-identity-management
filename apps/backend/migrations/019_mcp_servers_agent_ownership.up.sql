-- Change MCP servers to be owned by agents instead of users
-- This enables agents to register their own MCP servers autonomously

-- Drop the old foreign key constraint
ALTER TABLE mcp_servers DROP CONSTRAINT IF EXISTS mcp_servers_created_by_fkey;

-- Rename created_by to registered_by_agent
ALTER TABLE mcp_servers RENAME COLUMN created_by TO registered_by_agent;

-- Add new foreign key constraint to agents table
ALTER TABLE mcp_servers
ADD CONSTRAINT mcp_servers_registered_by_agent_fkey
FOREIGN KEY (registered_by_agent) REFERENCES agents(id) ON DELETE SET NULL;

-- Update comment
COMMENT ON COLUMN mcp_servers.registered_by_agent IS 'Agent that registered this MCP server';
