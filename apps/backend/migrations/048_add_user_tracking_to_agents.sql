-- Migration: Add user tracking columns to agents table
-- This enables complete audit trail for who created/updated agents

-- Add creator details columns
ALTER TABLE agents ADD COLUMN IF NOT EXISTS created_by_name VARCHAR(255);
ALTER TABLE agents ADD COLUMN IF NOT EXISTS created_by_email VARCHAR(255);
ALTER TABLE agents ADD COLUMN IF NOT EXISTS created_by_sdk_token_id UUID;

-- Add updater details columns
ALTER TABLE agents ADD COLUMN IF NOT EXISTS updated_by UUID;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS updated_by_name VARCHAR(255);
ALTER TABLE agents ADD COLUMN IF NOT EXISTS updated_by_email VARCHAR(255);

-- Create indexes for faster lookups
CREATE INDEX IF NOT EXISTS idx_agents_created_by ON agents(created_by);
CREATE INDEX IF NOT EXISTS idx_agents_updated_by ON agents(updated_by);
CREATE INDEX IF NOT EXISTS idx_agents_created_by_sdk_token_id ON agents(created_by_sdk_token_id);

-- Add comments explaining the columns
COMMENT ON COLUMN agents.created_by_name IS 'Name of the user who created this agent (denormalized)';
COMMENT ON COLUMN agents.created_by_email IS 'Email of the user who created this agent (denormalized)';
COMMENT ON COLUMN agents.created_by_sdk_token_id IS 'SDK token used to create this agent (for revocation tracking)';
COMMENT ON COLUMN agents.updated_by IS 'UUID of the user who last updated this agent';
COMMENT ON COLUMN agents.updated_by_name IS 'Name of the user who last updated this agent (denormalized)';
COMMENT ON COLUMN agents.updated_by_email IS 'Email of the user who last updated this agent (denormalized)';

-- Backfill existing agents with user information from the users table
UPDATE agents a
SET
    created_by_name = u.name,
    created_by_email = u.email
FROM users u
WHERE a.created_by = u.id
AND a.created_by_name IS NULL;
