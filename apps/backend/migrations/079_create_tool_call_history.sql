-- Migration 079: Tool Call History
-- Rolling 24h window of tool calls for FGA chain checks (Step 4).
-- Pruned by background job to keep table small.

CREATE TABLE IF NOT EXISTS tool_call_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    capability TEXT NOT NULL,
    object_path TEXT DEFAULT '',
    attributes_accessed TEXT[] DEFAULT '{}',
    data_class TEXT DEFAULT 'general',        -- general, pii, financial, credential, system
    authorized BOOLEAN NOT NULL DEFAULT TRUE,
    fga_steps_triggered TEXT[] DEFAULT '{}',  -- which FGA steps evaluated
    called_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_tool_call_history_agent_time
    ON tool_call_history (agent_id, called_at DESC);
-- Note: partial index with NOW() is not allowed (non-IMMUTABLE).
-- Pruning uses a plain index; the background job filters by time at query time.
CREATE INDEX IF NOT EXISTS idx_tool_call_history_prune
    ON tool_call_history (called_at);
