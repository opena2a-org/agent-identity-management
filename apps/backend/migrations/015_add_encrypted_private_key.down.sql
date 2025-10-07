-- Rollback migration for encrypted private key storage
-- Removes encrypted_private_key and key_algorithm columns from agents table

DROP INDEX IF EXISTS idx_agents_key_algorithm;

ALTER TABLE agents
DROP COLUMN IF EXISTS encrypted_private_key,
DROP COLUMN IF EXISTS key_algorithm;
