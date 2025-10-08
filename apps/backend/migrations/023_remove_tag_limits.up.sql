-- Remove Community Edition 3-tag limit triggers
-- Philosophy: Unlimited tags enable better compliance features that users will pay for

-- Drop triggers
DROP TRIGGER IF EXISTS enforce_agent_tag_limit ON agent_tags;
DROP TRIGGER IF EXISTS enforce_mcp_server_tag_limit ON mcp_server_tags;

-- Drop trigger functions
DROP FUNCTION IF EXISTS enforce_community_edition_agent_tag_limit();
DROP FUNCTION IF EXISTS enforce_community_edition_mcp_tag_limit();

-- Add is_standard column to identify curated enterprise tags
ALTER TABLE tags ADD COLUMN IF NOT EXISTS is_standard BOOLEAN DEFAULT false;

-- Add display_order for standard tag ordering
ALTER TABLE tags ADD COLUMN IF NOT EXISTS display_order INTEGER;

-- Create index for standard tags
CREATE INDEX IF NOT EXISTS idx_tags_standard ON tags(is_standard, display_order);
