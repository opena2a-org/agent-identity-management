-- Migration: Create detections audit table for full audit trail
-- Created: 2025-10-09
-- Purpose: Store EVERY detection event for compliance, analytics, and audit trail
--
-- Architecture Decision:
-- - detections: Full audit trail (stores every detection, never updates)
-- - agent_mcp_detections: Aggregated state (upserted only for "significant" detections)

-- Table: detections
-- Purpose: Immutable audit log of ALL detection events
CREATE TABLE IF NOT EXISTS detections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_name VARCHAR(255) NOT NULL,
    detection_method VARCHAR(50) NOT NULL,
    confidence_score DECIMAL(5,2) NOT NULL CHECK (confidence_score >= 0 AND confidence_score <= 100),
    details JSONB,
    sdk_version VARCHAR(50),
    is_significant BOOLEAN NOT NULL DEFAULT FALSE,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT valid_detection_method CHECK (
        detection_method IN (
            'manual',
            'claude_config',
            'sdk_import',
            'sdk_runtime',
            'direct_api'
        )
    )
);

-- Indexes for detections
CREATE INDEX idx_detections_audit_agent ON detections(agent_id);
CREATE INDEX idx_detections_audit_agent_mcp ON detections(agent_id, mcp_server_name);
CREATE INDEX idx_detections_audit_significant ON detections(agent_id, mcp_server_name, detection_method, is_significant);
CREATE INDEX idx_detections_audit_timestamp ON detections(detected_at DESC);
CREATE INDEX idx_detections_audit_method ON detections(detection_method);

-- Comments for documentation
COMMENT ON TABLE detections IS 'Immutable audit log of ALL detection events. Never updated, only inserted. Used for compliance, analytics, and forensics.';
COMMENT ON COLUMN detections.is_significant IS 'Whether this detection was deemed "significant" by server-side deduplication rules and triggered trust score updates';
COMMENT ON COLUMN detections.detected_at IS 'When the detection was reported to AIM';
COMMENT ON COLUMN detections.details IS 'Method-specific metadata (import path, command, args, etc.)';
COMMENT ON COLUMN detections.sdk_version IS 'SDK version that reported this detection (if applicable)';
