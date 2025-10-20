-- Create MCP server capabilities table to track tools, resources, and prompts
-- exposed by MCP servers for automatic capability detection

CREATE TABLE mcp_server_capabilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,

    -- Capability identification
    name VARCHAR(255) NOT NULL, -- e.g., "get_weather", "search_code", "generate_tests"
    capability_type VARCHAR(50) NOT NULL CHECK (capability_type IN ('tool', 'resource', 'prompt')),
    description TEXT,

    -- Capability details (stored as JSONB for flexibility)
    -- For tools: inputSchema, returns
    -- For resources: uri, mimeTypes
    -- For prompts: arguments, template
    capability_schema JSONB,

    -- Auto-detection metadata
    detected_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    last_verified_at TIMESTAMP WITH TIME ZONE,
    is_active BOOLEAN NOT NULL DEFAULT true,

    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),

    -- Ensure unique capability per server
    CONSTRAINT unique_mcp_server_capability UNIQUE (mcp_server_id, capability_type, name)
);

-- Indexes for efficient querying
CREATE INDEX idx_mcp_capabilities_server_id ON mcp_server_capabilities(mcp_server_id);
CREATE INDEX idx_mcp_capabilities_type ON mcp_server_capabilities(capability_type);
CREATE INDEX idx_mcp_capabilities_active ON mcp_server_capabilities(is_active) WHERE is_active = true;
CREATE INDEX idx_mcp_capabilities_name ON mcp_server_capabilities(name);

-- Add comment
COMMENT ON TABLE mcp_server_capabilities IS 'Automatically detected capabilities (tools, resources, prompts) exposed by MCP servers';
COMMENT ON COLUMN mcp_server_capabilities.capability_type IS 'Type of capability: tool (function), resource (data), or prompt (template)';
COMMENT ON COLUMN mcp_server_capabilities.capability_schema IS 'JSON schema defining capability interface (input/output for tools, URI patterns for resources, arguments for prompts)';
COMMENT ON COLUMN mcp_server_capabilities.detected_at IS 'When this capability was first auto-detected';
COMMENT ON COLUMN mcp_server_capabilities.last_verified_at IS 'Last time this capability was verified to still exist';
