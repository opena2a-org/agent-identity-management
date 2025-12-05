-- Migration: Add user tracking columns to mcp_servers table
-- This enables complete audit trail for who created/updated MCP servers

-- Add creator details columns
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS created_by_name VARCHAR(255);
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS created_by_email VARCHAR(255);
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS created_by_sdk_token_id UUID;

-- Add updater details columns
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS updated_by UUID;
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS updated_by_name VARCHAR(255);
ALTER TABLE mcp_servers ADD COLUMN IF NOT EXISTS updated_by_email VARCHAR(255);

-- Create indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_mcp_servers_created_by ON mcp_servers(created_by);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_updated_by ON mcp_servers(updated_by);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_created_by_sdk_token_id ON mcp_servers(created_by_sdk_token_id);

-- Add comments explaining the columns
COMMENT ON COLUMN mcp_servers.created_by_name IS 'Name of the user who created this MCP server (denormalized)';
COMMENT ON COLUMN mcp_servers.created_by_email IS 'Email of the user who created this MCP server (denormalized)';
COMMENT ON COLUMN mcp_servers.created_by_sdk_token_id IS 'SDK token used to create this MCP server (for revocation tracking)';
COMMENT ON COLUMN mcp_servers.updated_by IS 'UUID of the user who last updated this MCP server';
COMMENT ON COLUMN mcp_servers.updated_by_name IS 'Name of the user who last updated this MCP server (denormalized)';
COMMENT ON COLUMN mcp_servers.updated_by_email IS 'Email of the user who last updated this MCP server (denormalized)';

-- Backfill existing MCP servers with user information from the users table
UPDATE mcp_servers m
SET
    created_by_name = u.name,
    created_by_email = u.email
FROM users u
WHERE m.created_by = u.id
AND m.created_by_name IS NULL;
