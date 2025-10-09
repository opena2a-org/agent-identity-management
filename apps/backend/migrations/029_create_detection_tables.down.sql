-- Rollback migration: Drop detection tables
-- Created: 2025-10-09

-- Drop indexes first
DROP INDEX IF EXISTS idx_sdk_language;
DROP INDEX IF EXISTS idx_sdk_heartbeat;
DROP INDEX IF EXISTS idx_sdk_agent;

DROP INDEX IF EXISTS idx_detections_last_seen;
DROP INDEX IF EXISTS idx_detections_method;
DROP INDEX IF EXISTS idx_detections_mcp_lookup;
DROP INDEX IF EXISTS idx_detections_agent_lookup;

-- Drop tables
DROP TABLE IF EXISTS sdk_installations;
DROP TABLE IF EXISTS agent_mcp_detections;
