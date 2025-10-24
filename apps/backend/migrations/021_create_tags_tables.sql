-- Migration: Create Tags Management System
-- Created: 2025-10-22
-- Description: Sprint 1 - Tags Management System

-- Create tags table
CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value VARCHAR(255) NOT NULL,
    category VARCHAR(50) NOT NULL,
    description TEXT,
    color VARCHAR(7), -- Hex color (e.g., #3B82F6)
    created_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    UNIQUE(organization_id, key, value)
);

-- Create agent_tags junction table
CREATE TABLE IF NOT EXISTS agent_tags (
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (agent_id, tag_id)
);

-- Create mcp_server_tags junction table
CREATE TABLE IF NOT EXISTS mcp_server_tags (
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (mcp_server_id, tag_id)
);

-- Create indexes for better query performance
CREATE INDEX IF NOT EXISTS idx_tags_organization ON tags(organization_id);
CREATE INDEX IF NOT EXISTS idx_tags_category ON tags(category);
CREATE INDEX IF NOT EXISTS idx_agent_tags_agent ON agent_tags(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_tags_tag ON agent_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_mcp_server_tags_server ON mcp_server_tags(mcp_server_id);
CREATE INDEX IF NOT EXISTS idx_mcp_server_tags_tag ON mcp_server_tags(tag_id);

-- Create trigger function to enforce Community Edition 3-tag limit for agents
CREATE OR REPLACE FUNCTION enforce_community_edition_agent_tag_limit()
RETURNS TRIGGER AS $$
BEGIN
    IF (SELECT COUNT(*) FROM agent_tags WHERE agent_id = NEW.agent_id) >= 3 THEN
        RAISE EXCEPTION 'Community Edition: Maximum 3 tags per agent. Upgrade to Enterprise for unlimited tags.';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for agent tag limit
DROP TRIGGER IF EXISTS enforce_agent_tag_limit ON agent_tags;
CREATE TRIGGER enforce_agent_tag_limit
BEFORE INSERT ON agent_tags
FOR EACH ROW
EXECUTE FUNCTION enforce_community_edition_agent_tag_limit();

-- Create trigger function to enforce Community Edition 3-tag limit for MCP servers
CREATE OR REPLACE FUNCTION enforce_community_edition_mcp_tag_limit()
RETURNS TRIGGER AS $$
BEGIN
    IF (SELECT COUNT(*) FROM mcp_server_tags WHERE mcp_server_id = NEW.mcp_server_id) >= 3 THEN
        RAISE EXCEPTION 'Community Edition: Maximum 3 tags per MCP server. Upgrade to Enterprise for unlimited tags.';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for MCP server tag limit
DROP TRIGGER IF EXISTS enforce_mcp_server_tag_limit ON mcp_server_tags;
CREATE TRIGGER enforce_mcp_server_tag_limit
BEFORE INSERT ON mcp_server_tags
FOR EACH ROW
EXECUTE FUNCTION enforce_community_edition_mcp_tag_limit();

-- Add comments for documentation
COMMENT ON TABLE tags IS 'Tags for organizing agents and MCP servers (Sprint 1)';
COMMENT ON TABLE agent_tags IS 'Junction table linking agents to tags';
COMMENT ON TABLE mcp_server_tags IS 'Junction table linking MCP servers to tags';
