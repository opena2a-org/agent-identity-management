-- Migration: Add missing agent table columns
-- Created: 2025-10-20
-- Purpose: Add capability-based access control and key rotation columns

-- Add capability-based access control columns
ALTER TABLE agents ADD COLUMN IF NOT EXISTS talks_to JSONB DEFAULT '[]'::jsonb;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS capabilities JSONB DEFAULT '[]'::jsonb;

-- Add encrypted private key (stored encrypted, never exposed in API)
ALTER TABLE agents ADD COLUMN IF NOT EXISTS encrypted_private_key TEXT;

-- Add key algorithm column (was missing from initial migration)
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_algorithm VARCHAR(50) DEFAULT 'RSA-4096';

-- Add key metadata columns
ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_capability_check_at TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS capability_violation_count INTEGER DEFAULT 0;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS is_compromised BOOLEAN DEFAULT FALSE;

-- Add key rotation support columns
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_created_at TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_expires_at TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_rotation_grace_until TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS previous_public_key TEXT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS rotation_count INTEGER DEFAULT 0;

-- Create indexes for new columns
CREATE INDEX IF NOT EXISTS idx_agents_talks_to ON agents USING gin(talks_to);
CREATE INDEX IF NOT EXISTS idx_agents_capabilities ON agents USING gin(capabilities);
CREATE INDEX IF NOT EXISTS idx_agents_is_compromised ON agents(is_compromised);
CREATE INDEX IF NOT EXISTS idx_agents_key_expires_at ON agents(key_expires_at);

-- Add comments explaining new columns
COMMENT ON COLUMN agents.talks_to IS 'JSONB array of MCP server names/IDs this agent can communicate with';
COMMENT ON COLUMN agents.capabilities IS 'JSONB array of agent capabilities (e.g., ["file:read", "api:call"])';
COMMENT ON COLUMN agents.encrypted_private_key IS 'Encrypted private key (AES-256), never exposed in API';
COMMENT ON COLUMN agents.key_algorithm IS 'Cryptographic algorithm used for agent keys (e.g., RSA-4096, Ed25519)';
COMMENT ON COLUMN agents.is_compromised IS 'Whether this agent has been marked as compromised';
COMMENT ON COLUMN agents.key_created_at IS 'When the current key pair was created';
COMMENT ON COLUMN agents.key_expires_at IS 'When the current key expires (for automatic rotation)';
COMMENT ON COLUMN agents.key_rotation_grace_until IS 'Grace period end for old key during rotation';
COMMENT ON COLUMN agents.previous_public_key IS 'Previous public key (used during grace period verification)';
COMMENT ON COLUMN agents.rotation_count IS 'Number of times keys have been rotated';
