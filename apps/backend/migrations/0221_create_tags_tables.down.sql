-- Rollback migration 022

-- Drop triggers
DROP TRIGGER IF EXISTS enforce_mcp_tag_limit ON mcp_server_tags;
DROP TRIGGER IF EXISTS enforce_agent_tag_limit ON agent_tags;

-- Drop function
DROP FUNCTION IF EXISTS enforce_community_tag_limit();

-- Drop tables (cascade removes indexes and constraints)
DROP TABLE IF EXISTS mcp_server_tags CASCADE;
DROP TABLE IF EXISTS agent_tags CASCADE;
DROP TABLE IF EXISTS tags CASCADE;
