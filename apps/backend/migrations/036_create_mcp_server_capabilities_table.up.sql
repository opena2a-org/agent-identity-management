-- Create MCP Server Capabilities table
CREATE TABLE IF NOT EXISTS mcp_server_capabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    capability_type VARCHAR(50) NOT NULL CHECK (capability_type IN ('tool', 'resource', 'prompt')),
    description TEXT,
    capability_schema JSONB,
    detected_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_verified_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(mcp_server_id, name, capability_type)
);

CREATE INDEX idx_mcp_server_capabilities_server_id ON mcp_server_capabilities(mcp_server_id);
CREATE INDEX idx_mcp_server_capabilities_type ON mcp_server_capabilities(capability_type);
CREATE INDEX idx_mcp_server_capabilities_active ON mcp_server_capabilities(is_active);
