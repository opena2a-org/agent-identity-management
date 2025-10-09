-- Migration: Create detection tables for SDK and Direct API detection
-- Created: 2025-10-09
-- Purpose: Track MCP detections from SDKs and Direct API calls

-- Table 1: agent_mcp_detections
-- Purpose: Cache detection results from all methods with confidence scores
CREATE TABLE IF NOT EXISTS agent_mcp_detections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_name VARCHAR(255) NOT NULL,
    detection_method VARCHAR(50) NOT NULL,
    confidence_score DECIMAL(5,2) NOT NULL CHECK (confidence_score >= 0 AND confidence_score <= 100),
    details JSONB,
    sdk_version VARCHAR(50),
    first_detected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT agent_mcp_detections_unique UNIQUE (agent_id, mcp_server_name, detection_method),
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

-- Indexes for agent_mcp_detections
CREATE INDEX idx_detections_agent_lookup ON agent_mcp_detections(agent_id);
CREATE INDEX idx_detections_mcp_lookup ON agent_mcp_detections(agent_id, mcp_server_name);
CREATE INDEX idx_detections_method ON agent_mcp_detections(detection_method);
CREATE INDEX idx_detections_last_seen ON agent_mcp_detections(last_seen_at DESC);

-- Table 2: sdk_installations
-- Purpose: Track which agents have SDK installed and their status
CREATE TABLE IF NOT EXISTS sdk_installations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    sdk_language VARCHAR(50) NOT NULL,
    sdk_version VARCHAR(50) NOT NULL,
    installed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_heartbeat_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    auto_detect_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Constraints
    CONSTRAINT sdk_installations_unique UNIQUE (agent_id),
    CONSTRAINT valid_sdk_language CHECK (
        sdk_language IN (
            'javascript',
            'typescript',
            'python',
            'go'
        )
    )
);

-- Indexes for sdk_installations
CREATE INDEX idx_sdk_agent ON sdk_installations(agent_id);
CREATE INDEX idx_sdk_heartbeat ON sdk_installations(last_heartbeat_at DESC);
CREATE INDEX idx_sdk_language ON sdk_installations(sdk_language);

-- Comments for documentation
COMMENT ON TABLE agent_mcp_detections IS 'Caches detection results from all detection methods (manual, Claude config, SDK, Direct API) with confidence scores';
COMMENT ON TABLE sdk_installations IS 'Tracks SDK installations per agent including version, language, and heartbeat status';

COMMENT ON COLUMN agent_mcp_detections.detection_method IS 'Detection method: manual, claude_config, sdk_import, sdk_runtime, direct_api';
COMMENT ON COLUMN agent_mcp_detections.confidence_score IS 'Confidence score 0-100. Higher score = more confident detection';
COMMENT ON COLUMN agent_mcp_detections.details IS 'Method-specific metadata (import path, command, args, etc.)';
COMMENT ON COLUMN agent_mcp_detections.sdk_version IS 'SDK version that reported this detection (if applicable)';

COMMENT ON COLUMN sdk_installations.sdk_language IS 'Programming language: javascript, typescript, python, go';
COMMENT ON COLUMN sdk_installations.last_heartbeat_at IS 'Last time SDK reported activity. Used to detect inactive agents';
COMMENT ON COLUMN sdk_installations.auto_detect_enabled IS 'Whether auto-detection is enabled for this agent';
