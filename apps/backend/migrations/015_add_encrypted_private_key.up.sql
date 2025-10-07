-- Add encrypted_private_key column to agents table for automatic key generation
-- This enables AIM to automatically generate and securely store Ed25519 key pairs
-- Private keys are encrypted with AES-256-GCM before storage

ALTER TABLE agents
ADD COLUMN IF NOT EXISTS encrypted_private_key TEXT,
ADD COLUMN IF NOT EXISTS key_algorithm VARCHAR(50) DEFAULT 'Ed25519';

-- Add comment for documentation
COMMENT ON COLUMN agents.encrypted_private_key IS 'AES-256-GCM encrypted private key for agent authentication. Never exposed through API.';
COMMENT ON COLUMN agents.key_algorithm IS 'Cryptographic algorithm used for key pair (Ed25519)';

-- Add index for faster key lookups (though private keys should rarely be queried)
CREATE INDEX IF NOT EXISTS idx_agents_key_algorithm ON agents(key_algorithm);
