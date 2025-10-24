-- Migration: Create detections table for MCP server detection audit trail
-- Created: 2025-10-23
-- Purpose: Store all MCP detection events from SDK for audit and analytics

CREATE TABLE IF NOT EXISTS detections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_name VARCHAR(255) NOT NULL,
    detection_method VARCHAR(50) NOT NULL,
    confidence_score DECIMAL(5,2) NOT NULL DEFAULT 0.00,
    details JSONB DEFAULT '{}'::jsonb,
    sdk_version VARCHAR(50),
    is_significant BOOLEAN DEFAULT FALSE,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT detections_confidence_check CHECK (confidence_score >= 0 AND confidence_score <= 100)
);

-- Create indexes for detections
CREATE INDEX IF NOT EXISTS idx_detections_agent_id ON detections(agent_id);
CREATE INDEX IF NOT EXISTS idx_detections_mcp_server_name ON detections(mcp_server_name);
CREATE INDEX IF NOT EXISTS idx_detections_detection_method ON detections(detection_method);
CREATE INDEX IF NOT EXISTS idx_detections_is_significant ON detections(is_significant);
CREATE INDEX IF NOT EXISTS idx_detections_detected_at ON detections(detected_at);
CREATE INDEX IF NOT EXISTS idx_detections_created_at ON detections(created_at);

-- Composite index for significance checking (used in deduplication)
CREATE INDEX IF NOT EXISTS idx_detections_agent_mcp_method ON detections(agent_id, mcp_server_name, detection_method, detected_at DESC);

-- Add comments explaining detection fields
COMMENT ON TABLE detections IS 'Audit trail of all MCP server detection events from SDK';
COMMENT ON COLUMN detections.mcp_server_name IS 'Name of the detected MCP server';
COMMENT ON COLUMN detections.detection_method IS 'How the MCP was detected (e.g., auto_sdk, manual, claude_config)';
COMMENT ON COLUMN detections.confidence_score IS 'Detection confidence score (0-100)';
COMMENT ON COLUMN detections.details IS 'Additional metadata about the detection';
COMMENT ON COLUMN detections.is_significant IS 'Whether this detection is significant enough to update agent.talks_to';
COMMENT ON COLUMN detections.detected_at IS 'When the MCP was detected by the SDK';

