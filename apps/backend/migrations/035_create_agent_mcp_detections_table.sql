-- Migration: Create agent_mcp_detections table for aggregated MCP detection state
-- Created: 2025-10-23
-- Purpose: Store aggregated state of MCP detections per agent (deduplication and aggregation)

CREATE TABLE IF NOT EXISTS agent_mcp_detections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_name VARCHAR(255) NOT NULL,
    detection_method VARCHAR(50) NOT NULL,
    confidence_score DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    details JSONB DEFAULT '{}'::jsonb,
    sdk_version VARCHAR(50),
    first_detected_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT agent_mcp_detections_confidence_check CHECK (confidence_score >= 0 AND confidence_score <= 100),
    UNIQUE(agent_id, mcp_server_name, detection_method)
);

-- Create indexes for agent_mcp_detections
CREATE INDEX IF NOT EXISTS idx_agent_mcp_detections_agent_id ON agent_mcp_detections(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_detections_mcp_server_name ON agent_mcp_detections(mcp_server_name);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_detections_detection_method ON agent_mcp_detections(detection_method);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_detections_first_detected_at ON agent_mcp_detections(first_detected_at);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_detections_last_seen_at ON agent_mcp_detections(last_seen_at);

-- Composite index for lookups
CREATE INDEX IF NOT EXISTS idx_agent_mcp_detections_agent_mcp ON agent_mcp_detections(agent_id, mcp_server_name);

-- Add comments
COMMENT ON TABLE agent_mcp_detections IS 'Aggregated state of MCP server detections per agent';
COMMENT ON COLUMN agent_mcp_detections.mcp_server_name IS 'Name of the detected MCP server';
COMMENT ON COLUMN agent_mcp_detections.detection_method IS 'Primary detection method for this MCP-agent pair';
COMMENT ON COLUMN agent_mcp_detections.first_detected_at IS 'When this MCP was first detected for this agent';
COMMENT ON COLUMN agent_mcp_detections.last_seen_at IS 'When this MCP was most recently detected';

