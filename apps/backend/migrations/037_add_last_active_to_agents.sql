-- Migration: Add last_active column to agents table
-- This column tracks when an agent last performed an action (verify-action call)
-- Different from verified_at which is set during registration/verification

-- Add last_active column (nullable initially, will be backfilled)
ALTER TABLE agents
ADD COLUMN IF NOT EXISTS last_active TIMESTAMP;

-- Backfill last_active with created_at for existing agents
UPDATE agents
SET last_active = created_at
WHERE last_active IS NULL;

-- Create index for faster queries on last_active (used in activity analytics)
CREATE INDEX IF NOT EXISTS idx_agents_last_active ON agents(last_active DESC);

-- Add comment for documentation
COMMENT ON COLUMN agents.last_active IS 'Timestamp of when agent last performed an action (updated on every verify-action call)';
