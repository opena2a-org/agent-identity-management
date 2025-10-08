-- Add key rotation tracking columns to agents table
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_created_at TIMESTAMPTZ DEFAULT NOW();
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_expires_at TIMESTAMPTZ DEFAULT (NOW() + INTERVAL '90 days');
ALTER TABLE agents ADD COLUMN IF NOT EXISTS key_rotation_grace_until TIMESTAMPTZ;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS previous_public_key TEXT;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS rotation_count INTEGER DEFAULT 0;

-- Update existing agents with default expiration dates
UPDATE agents
SET key_created_at = created_at,
    key_expires_at = created_at + INTERVAL '90 days',
    rotation_count = 0
WHERE key_created_at IS NULL;

-- Create index for expiration monitoring (find keys expiring soon)
CREATE INDEX IF NOT EXISTS idx_agents_key_expires_at ON agents(key_expires_at) WHERE key_expires_at IS NOT NULL;

-- Create index for grace period monitoring (find agents in rotation)
CREATE INDEX IF NOT EXISTS idx_agents_grace_period ON agents(key_rotation_grace_until) WHERE key_rotation_grace_until IS NOT NULL;

-- Add comments for documentation
COMMENT ON COLUMN agents.key_created_at IS 'Timestamp when the current Ed25519 keypair was created';
COMMENT ON COLUMN agents.key_expires_at IS 'Timestamp when the current keypair expires (default: 90 days from creation)';
COMMENT ON COLUMN agents.key_rotation_grace_until IS 'Grace period end time when both old and new public keys are valid (default: 24 hours)';
COMMENT ON COLUMN agents.previous_public_key IS 'Previous Ed25519 public key (valid during grace period for zero-downtime rotation)';
COMMENT ON COLUMN agents.rotation_count IS 'Number of times the keypair has been rotated (starts at 0)';
