-- Migration 122: an agent card attestation records the format it was signed in.
--
-- attestation_format holds the label of the signing format. The v2 label,
-- opena2a-aim/card-attestation/v2, marks an attestation whose signed payload
-- can be rebuilt from the served agent card: its issuedAt and expiresAt are the
-- stored timestamps, in UTC and whole seconds. docs/specs/card-attestation-v2.md
-- describes how a third party verifies one with /.well-known/jwks.json.
--
-- The column is nullable. Attestations issued before this migration read NULL:
-- their signed issuedAt was not stored, so only the server could have checked
-- them. They expire within their validity window, and refreshing one re-signs
-- it in the v2 format. Idempotent: the column is added only if absent.

ALTER TABLE a2a_agent_cards ADD COLUMN IF NOT EXISTS attestation_format VARCHAR(64);

COMMENT ON COLUMN a2a_agent_cards.attestation_format IS
    'Signing format label of attestation_signature (opena2a-aim/card-attestation/v2); NULL for attestations issued before the format was recorded';
