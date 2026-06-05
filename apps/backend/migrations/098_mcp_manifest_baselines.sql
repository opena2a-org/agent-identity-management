-- Migration 098: MCP manifest baselines
--
-- Append-only snapshots of an MCP server's tool manifest, identified by a
-- canonical hash over (capability type, name, schema hash). The newest row
-- per server has is_current = TRUE and is the baseline that later captures
-- diff against. Capability detail already lives in mcp_server_capabilities;
-- this table records the aggregate manifest hash so a server quietly adding,
-- removing, or reshaping a tool after it was trusted becomes detectable.

CREATE TABLE IF NOT EXISTS mcp_manifest_baselines (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mcp_server_id  UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    manifest_hash  TEXT NOT NULL,
    entries        JSONB NOT NULL DEFAULT '[]',
    tool_count     INTEGER NOT NULL DEFAULT 0,
    resource_count INTEGER NOT NULL DEFAULT 0,
    prompt_count   INTEGER NOT NULL DEFAULT 0,
    captured_from  TEXT NOT NULL CHECK (captured_from IN ('well_known', 'attestation')),
    captured_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    is_current     BOOLEAN NOT NULL DEFAULT TRUE
);

-- One current baseline per server. Partial unique index allows the full
-- append-only history while guaranteeing a single is_current row.
CREATE UNIQUE INDEX IF NOT EXISTS idx_mcp_manifest_baselines_current
    ON mcp_manifest_baselines (mcp_server_id)
    WHERE is_current = TRUE;

CREATE INDEX IF NOT EXISTS idx_mcp_manifest_baselines_server_time
    ON mcp_manifest_baselines (mcp_server_id, captured_at DESC);
