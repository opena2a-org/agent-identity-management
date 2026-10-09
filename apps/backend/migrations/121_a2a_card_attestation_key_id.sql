-- Migration 121: an agent card attestation records the key that signed it.
--
-- Card attestations are signed with the server's card-attestation key, whose
-- public key is published at /.well-known/jwks.json. attestation_key_id is that
-- key's kid (hex SHA-256 of the public key) and attestation_alg its JOSE
-- algorithm, so a verifier can pick the key without trying each one.
--
-- Both columns are nullable. Attestations issued before this migration keep
-- their signature and read NULL here; they expire within their validity
-- window, and refreshing one records the key that re-signed it. Idempotent:
-- each column is added only if absent.

ALTER TABLE a2a_agent_cards ADD COLUMN IF NOT EXISTS attestation_key_id VARCHAR(64);
ALTER TABLE a2a_agent_cards ADD COLUMN IF NOT EXISTS attestation_alg VARCHAR(20);

COMMENT ON COLUMN a2a_agent_cards.attestation_key_id IS
    'kid of the card-attestation key that signed attestation_signature; NULL for attestations issued before key IDs were recorded';
COMMENT ON COLUMN a2a_agent_cards.attestation_alg IS
    'JOSE algorithm of attestation_signature (EdDSA); NULL for attestations issued before key IDs were recorded';
