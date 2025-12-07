-- Migration 056: Add agent_id column to audit_logs for agent-initiated actions
-- This allows proper audit logging of attestations and other agent actions

-- Make user_id nullable (agent-initiated actions don't have a user)
ALTER TABLE audit_logs
    ALTER COLUMN user_id DROP NOT NULL;

-- Add agent_id column (nullable, for agent-initiated actions)
ALTER TABLE audit_logs
    ADD COLUMN IF NOT EXISTS agent_id UUID REFERENCES agents(id) ON DELETE SET NULL;

-- Add index for agent_id queries
CREATE INDEX IF NOT EXISTS idx_audit_logs_agent_id ON audit_logs(agent_id);

-- Drop the foreign key constraint on user_id temporarily to allow NULL values
-- The foreign key will still exist but NULL is now allowed
