-- Revert MCP servers to be owned by users

-- Drop the agent foreign key constraint
ALTER TABLE mcp_servers DROP CONSTRAINT IF EXISTS mcp_servers_registered_by_agent_fkey;

-- Rename back to created_by
ALTER TABLE mcp_servers RENAME COLUMN registered_by_agent TO created_by;

-- Add back user foreign key constraint
ALTER TABLE mcp_servers
ADD CONSTRAINT mcp_servers_created_by_fkey
FOREIGN KEY (created_by) REFERENCES users(id);

-- Remove comment
COMMENT ON COLUMN mcp_servers.created_by IS NULL;
