-- Make registered_by_agent nullable to support user-registered MCP servers
-- MCP servers can be registered by users (registered_by_agent = NULL) or by agents

ALTER TABLE mcp_servers ALTER COLUMN registered_by_agent DROP NOT NULL;

COMMENT ON COLUMN mcp_servers.registered_by_agent IS 'Agent that registered this MCP server (NULL if registered by a user)';
