-- ============================================================================
-- Migration 022: Create Tags Tables for Agents and MCP Servers
-- Author: Claude Sonnet 4.5
-- Date: October 8, 2025
-- Description: Basic tagging system for Community Edition (max 3 tags per asset)
-- ============================================================================

-- Tags table (organization-scoped)
CREATE TABLE IF NOT EXISTS tags (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    key VARCHAR(100) NOT NULL,
    value VARCHAR(255) NOT NULL,
    category VARCHAR(50) NOT NULL,
    description TEXT,
    color VARCHAR(7), -- Hex color for UI (e.g., '#3B82F6')
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID REFERENCES users(id),

    UNIQUE(organization_id, key, value),
    CHECK (category IN ('resource_type', 'environment', 'agent_type', 'data_classification', 'custom'))
);

-- Indexes for tag lookups
CREATE INDEX idx_tags_org_id ON tags(organization_id);
CREATE INDEX idx_tags_category ON tags(organization_id, category);
CREATE INDEX idx_tags_key_value ON tags(key, value);

-- Comments
COMMENT ON TABLE tags IS 'Organization-scoped tags for categorizing agents and MCP servers';
COMMENT ON COLUMN tags.category IS 'Tag category: resource_type, environment, agent_type, data_classification, custom';
COMMENT ON COLUMN tags.color IS 'Hex color code for UI display (e.g., #3B82F6 for blue)';

-- Agent tags (many-to-many)
CREATE TABLE IF NOT EXISTS agent_tags (
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    applied_by UUID REFERENCES users(id),

    PRIMARY KEY (agent_id, tag_id)
);

-- Indexes for agent tag queries
CREATE INDEX idx_agent_tags_agent_id ON agent_tags(agent_id);
CREATE INDEX idx_agent_tags_tag_id ON agent_tags(tag_id);

COMMENT ON TABLE agent_tags IS 'Many-to-many relationship between agents and tags';

-- MCP server tags (many-to-many)
CREATE TABLE IF NOT EXISTS mcp_server_tags (
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    tag_id UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    applied_by UUID REFERENCES users(id),

    PRIMARY KEY (mcp_server_id, tag_id)
);

-- Indexes for MCP server tag queries
CREATE INDEX idx_mcp_server_tags_mcp_id ON mcp_server_tags(mcp_server_id);
CREATE INDEX idx_mcp_server_tags_tag_id ON mcp_server_tags(tag_id);

COMMENT ON TABLE mcp_server_tags IS 'Many-to-many relationship between MCP servers and tags';

-- Function to enforce Community Edition tag limit (max 3 tags per asset)
CREATE OR REPLACE FUNCTION enforce_community_tag_limit()
RETURNS TRIGGER AS $$
DECLARE
    tag_count INT;
    org_tier VARCHAR(50);
BEGIN
    -- Get organization tier
    SELECT tier INTO org_tier
    FROM organizations
    WHERE id = (
        SELECT organization_id FROM agents WHERE id = NEW.agent_id
        UNION
        SELECT organization_id FROM mcp_servers WHERE id = NEW.mcp_server_id
    );

    -- Only enforce for Community tier
    IF org_tier = 'community' THEN
        -- Count existing tags
        SELECT COUNT(*) INTO tag_count
        FROM (
            SELECT agent_id FROM agent_tags WHERE agent_id = NEW.agent_id
            UNION ALL
            SELECT mcp_server_id FROM mcp_server_tags WHERE mcp_server_id = NEW.mcp_server_id
        ) AS all_tags;

        -- Enforce 3 tag limit
        IF tag_count >= 3 THEN
            RAISE EXCEPTION 'Community Edition limited to 3 tags per asset. Upgrade to Pro for unlimited tags.';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply trigger to agent_tags
CREATE TRIGGER enforce_agent_tag_limit
    BEFORE INSERT ON agent_tags
    FOR EACH ROW
    EXECUTE FUNCTION enforce_community_tag_limit();

-- Apply trigger to mcp_server_tags
CREATE TRIGGER enforce_mcp_tag_limit
    BEFORE INSERT ON mcp_server_tags
    FOR EACH ROW
    EXECUTE FUNCTION enforce_community_tag_limit();

-- Insert default tags for all organizations
-- (These are suggestions, users can create their own)
INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'resource_type',
    'filesystem',
    'resource_type',
    'File system operations (read, write, list)',
    '#10B981' -- green
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'resource_type',
    'database',
    'resource_type',
    'Database connections and queries',
    '#3B82F6' -- blue
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'resource_type',
    'api',
    'resource_type',
    'API and HTTP integrations',
    '#8B5CF6' -- purple
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'environment',
    'production',
    'environment',
    'Production environment',
    '#EF4444' -- red
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'environment',
    'development',
    'environment',
    'Development environment',
    '#F59E0B' -- yellow
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'environment',
    'staging',
    'environment',
    'Staging/pre-production environment',
    '#F97316' -- orange
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'agent_type',
    'customer-facing',
    'agent_type',
    'Customer-facing agent (chatbot, support)',
    '#EC4899' -- pink
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'agent_type',
    'autonomous',
    'agent_type',
    'Fully autonomous agent (no human oversight)',
    '#6366F1' -- indigo
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'data_classification',
    'pii',
    'data_classification',
    'Processes personally identifiable information',
    '#DC2626' -- dark red
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;

INSERT INTO tags (organization_id, key, value, category, description, color)
SELECT
    id,
    'data_classification',
    'public',
    'data_classification',
    'Public data only',
    '#059669' -- emerald
FROM organizations
ON CONFLICT (organization_id, key, value) DO NOTHING;
