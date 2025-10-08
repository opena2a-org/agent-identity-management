-- Add missing columns to alerts table
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS agent_id UUID REFERENCES agents(id) ON DELETE CASCADE;
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS status VARCHAR(20) DEFAULT 'active';
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb;
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

-- Update existing rows to have the new default values
UPDATE alerts SET status = 'active' WHERE status IS NULL AND is_acknowledged = false;
UPDATE alerts SET status = 'acknowledged' WHERE status IS NULL AND is_acknowledged = true;

-- Create additional indexes
CREATE INDEX IF NOT EXISTS idx_alerts_agent_id ON alerts(agent_id);
CREATE INDEX IF NOT EXISTS idx_alerts_status ON alerts(status);
CREATE INDEX IF NOT EXISTS idx_alerts_org_status ON alerts(organization_id, status);

-- Add comments
COMMENT ON COLUMN alerts.agent_id IS 'Associated agent (optional)';
COMMENT ON COLUMN alerts.status IS 'Alert status: active, acknowledged, dismissed, resolved';
COMMENT ON COLUMN alerts.metadata IS 'Additional context about the alert (JSON)';
