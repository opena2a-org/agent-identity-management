-- Rollback: Restore 3-tag limit triggers (if needed for testing)

-- Remove new columns
ALTER TABLE tags DROP COLUMN IF EXISTS display_order;
ALTER TABLE tags DROP COLUMN IF EXISTS is_standard;

-- Recreate trigger function for agents
CREATE OR REPLACE FUNCTION enforce_community_edition_agent_tag_limit()
RETURNS TRIGGER AS $$
BEGIN
    IF (SELECT COUNT(*) FROM agent_tags WHERE agent_id = NEW.agent_id) >= 3 THEN
        RAISE EXCEPTION 'Community Edition: Maximum 3 tags per agent. Upgrade to Enterprise for unlimited tags.';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Recreate trigger for agents
CREATE TRIGGER enforce_agent_tag_limit
BEFORE INSERT ON agent_tags
FOR EACH ROW
EXECUTE FUNCTION enforce_community_edition_agent_tag_limit();

-- Recreate trigger function for MCP servers
CREATE OR REPLACE FUNCTION enforce_community_edition_mcp_tag_limit()
RETURNS TRIGGER AS $$
BEGIN
    IF (SELECT COUNT(*) FROM mcp_server_tags WHERE mcp_server_id = NEW.mcp_server_id) >= 3 THEN
        RAISE EXCEPTION 'Community Edition: Maximum 3 tags per MCP server. Upgrade to Enterprise for unlimited tags.';
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Recreate trigger for MCP servers
CREATE TRIGGER enforce_mcp_server_tag_limit
BEFORE INSERT ON mcp_server_tags
FOR EACH ROW
EXECUTE FUNCTION enforce_community_edition_mcp_tag_limit();
