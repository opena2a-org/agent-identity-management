-- Migration: Add API key tracking columns for agents and MCP servers
-- This enables admins to track and revoke API keys used to create agents/MCPs

-- Add API key tracking to agents
ALTER TABLE agents ADD COLUMN IF NOT EXISTS created_by_api_key_id UUID;

-- Add API key tracking to MCP servers
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS created_by_api_key_id UUID;

-- Create indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_agents_created_by_api_key_id ON agents(created_by_api_key_id);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_created_by_api_key_id ON mcp_servers(created_by_api_key_id);

-- Add comments explaining the columns
COMMENT ON COLUMN agents.created_by_api_key_id IS 'API key used to create this agent (for revocation tracking)';
COMMENT ON COLUMN mcp_servers.created_by_api_key_id IS 'API key used to create this MCP server (for revocation tracking)';
