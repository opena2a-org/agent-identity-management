-- Migration: Create agent_mcp_connections table for bidirectional Agent ↔ MCP relationships
-- Created: 2025-10-23
-- Purpose: Track connections between agents and MCP servers with attestation state

CREATE TABLE IF NOT EXISTS agent_mcp_connections (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    detection_id UUID REFERENCES agent_mcp_detections(id) ON DELETE SET NULL,

    -- Connection type: how this connection was established
    connection_type VARCHAR(50) NOT NULL CHECK (
        connection_type IN ('auto_detected', 'user_registered', 'attested')
    ),

    -- Timestamps
    first_connected_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_attested_at TIMESTAMPTZ,

    -- Attestation tracking
    attestation_count INT DEFAULT 0,

    -- Status
    is_active BOOLEAN NOT NULL DEFAULT TRUE,

    -- Metadata
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    -- Ensure one connection per agent-MCP pair
    UNIQUE(agent_id, mcp_server_id)
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_agent_mcp_connections_agent ON agent_mcp_connections(agent_id);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_connections_mcp ON agent_mcp_connections(mcp_server_id);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_connections_detection ON agent_mcp_connections(detection_id);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_connections_type ON agent_mcp_connections(connection_type);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_connections_active ON agent_mcp_connections(is_active);

-- Composite index for common queries
CREATE INDEX IF NOT EXISTS idx_agent_mcp_connections_agent_active ON agent_mcp_connections(agent_id, is_active);
CREATE INDEX IF NOT EXISTS idx_agent_mcp_connections_mcp_active ON agent_mcp_connections(mcp_server_id, is_active);

-- Add comments
COMMENT ON TABLE agent_mcp_connections IS 'Bidirectional relationships between agents and MCP servers';
COMMENT ON COLUMN agent_mcp_connections.connection_type IS 'How connection was established: auto_detected, user_registered, or attested';
COMMENT ON COLUMN agent_mcp_connections.detection_id IS 'Reference to original detection (if applicable)';
COMMENT ON COLUMN agent_mcp_connections.attestation_count IS 'Number of times this agent has attested this MCP';
COMMENT ON COLUMN agent_mcp_connections.last_attested_at IS 'When agent last attested this MCP';
