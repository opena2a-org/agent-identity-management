-- Migration 099: MCP manifest drift records
--
-- One row per detected change between a server's previous current baseline and
-- a new manifest capture. added/removed/changed hold capability keys
-- ("type:name"). severity follows the domain heuristic: any high-risk surface
-- (exec/filesystem/network/credential) added or changed => high; other
-- removals/changes => medium; benign additions => low.

CREATE TABLE IF NOT EXISTS mcp_manifest_drift (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    mcp_server_id UUID NOT NULL REFERENCES mcp_servers(id) ON DELETE CASCADE,
    prev_hash     TEXT NOT NULL,
    new_hash      TEXT NOT NULL,
    added_tools   JSONB NOT NULL DEFAULT '[]',
    removed_tools JSONB NOT NULL DEFAULT '[]',
    changed_tools JSONB NOT NULL DEFAULT '[]',
    severity      TEXT NOT NULL CHECK (severity IN ('low', 'medium', 'high')),
    source        TEXT NOT NULL CHECK (source IN ('well_known', 'attestation')),
    detected_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged  BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS idx_mcp_manifest_drift_server_time
    ON mcp_manifest_drift (mcp_server_id, detected_at DESC);

-- Surface unacknowledged high-severity drift cheaply for alerting.
CREATE INDEX IF NOT EXISTS idx_mcp_manifest_drift_open_high
    ON mcp_manifest_drift (mcp_server_id)
    WHERE acknowledged = FALSE AND severity = 'high';
