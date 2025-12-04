-- Migration: Add audit_id to alerts table for investigation workflow
-- This allows alerts to link directly to their triggering audit log entry

-- Add audit_id column (nullable to support existing alerts)
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS audit_id UUID;

-- Add index for fast lookups
CREATE INDEX IF NOT EXISTS idx_alerts_audit_id ON alerts(audit_id);

-- Add foreign key constraint (optional, as audit logs may be rotated)
-- Not adding FK to allow audit log cleanup without affecting alerts

-- Add agent_name column for denormalized display (avoids JOINs)
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS agent_name VARCHAR(255);

-- Add metadata column for additional context (JSON)
ALTER TABLE alerts ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}';

COMMENT ON COLUMN alerts.audit_id IS 'Reference to the audit log entry that triggered this alert';
COMMENT ON COLUMN alerts.agent_name IS 'Denormalized agent name for display without JOIN';
COMMENT ON COLUMN alerts.metadata IS 'Additional context about the alert (action, resource, trust score, etc.)';
