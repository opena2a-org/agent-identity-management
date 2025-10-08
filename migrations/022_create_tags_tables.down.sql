-- Drop triggers first
DROP TRIGGER IF EXISTS enforce_agent_tag_limit ON agent_tags;
DROP TRIGGER IF EXISTS enforce_mcp_server_tag_limit ON mcp_server_tags;

-- Drop trigger functions
DROP FUNCTION IF EXISTS enforce_community_edition_agent_tag_limit();
DROP FUNCTION IF EXISTS enforce_community_edition_mcp_tag_limit();

-- Drop junction tables
DROP TABLE IF EXISTS agent_tags;
DROP TABLE IF EXISTS mcp_server_tags;

-- Drop main tags table
DROP TABLE IF EXISTS tags;
