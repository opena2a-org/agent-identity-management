-- Create mcp_server_keys table for storing public keys associated with MCP servers
-- Referenced by mcp_repository.go AddPublicKey and GetVerificationStatus queries
CREATE TABLE IF NOT EXISTS mcp_server_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    public_key TEXT NOT NULL,
    key_type VARCHAR(50) NOT NULL DEFAULT 'ed25519',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mcp_server_keys_server_id ON mcp_server_keys(server_id);
