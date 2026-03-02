-- Add heartbeat tracking to agents
ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_heartbeat TIMESTAMPTZ;
CREATE INDEX IF NOT EXISTS idx_agents_last_heartbeat ON agents(last_heartbeat);
