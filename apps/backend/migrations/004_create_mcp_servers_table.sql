-- Migration: Create MCP servers table
-- Created: 2025-10-20
-- Purpose: Add MCP (Model Context Protocol) server registration and verification

-- MCP Servers table
CREATE TABLE IF NOT EXISTS mcp_servers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    url VARCHAR(512) NOT NULL,
    version VARCHAR(50),
    public_key TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    is_verified BOOLEAN DEFAULT FALSE,
    last_verified_at TIMESTAMPTZ,
    verification_url VARCHAR(512),
    capabilities JSONB DEFAULT '[]'::jsonb,
    trust_score DECIMAL(5,2) DEFAULT 0.00,
    registered_by_agent UUID REFERENCES agents(id) ON DELETE SET NULL,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(organization_id, url)
);

-- Create indexes for MCP servers
CREATE INDEX IF NOT EXISTS idx_mcp_servers_organization_id ON mcp_servers(organization_id);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_status ON mcp_servers(status);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_is_verified ON mcp_servers(is_verified);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_trust_score ON mcp_servers(trust_score);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_url ON mcp_servers(url);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_capabilities ON mcp_servers USING gin(capabilities);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_registered_by_agent ON mcp_servers(registered_by_agent);
CREATE INDEX IF NOT EXISTS idx_mcp_servers_created_by ON mcp_servers(created_by);

-- Add comments explaining MCP server fields
COMMENT ON TABLE mcp_servers IS 'Model Context Protocol (MCP) servers registered by agents';
COMMENT ON COLUMN mcp_servers.url IS 'MCP server endpoint URL';
COMMENT ON COLUMN mcp_servers.capabilities IS 'JSONB array of MCP server capabilities (e.g., ["tools", "prompts", "resources"])';
COMMENT ON COLUMN mcp_servers.registered_by_agent IS 'Agent that registered this MCP server (nullable - can be registered by users too)';
COMMENT ON COLUMN mcp_servers.verification_url IS 'URL used for cryptographic verification of the MCP server';
COMMENT ON COLUMN mcp_servers.public_key IS 'Public key for verifying MCP server identity';
