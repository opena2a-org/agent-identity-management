-- Migration 128: re-issue the comment on isolation_attestations.verified.
--
-- 108_add_isolation_attestation_verification.sql was edited after release so that the
-- column comment describes the behaviour instead of naming a planning entry. Both runners
-- (cmd/server at start and cmd/migrate) skip a migration whose file name is already
-- recorded in schema_migrations, so a database that applied 108 before the edit keeps the
-- old comment while a new database gets the new one. This sets the same text on both.
--
-- COMMENT ON replaces the previous comment, so running this on a database that already
-- carries the new text changes nothing.

COMMENT ON COLUMN isolation_attestations.verified IS
    'TRUE only when an INDEPENDENT source corroborated THIS row''s posture. Bound to the row, '
    'never to the agent: a newer attestation starts unverified and inherits nothing. No write '
    'path sets this TRUE yet; once an independent source exists, TEE attestation and '
    'orchestrator/host metadata may, an HMA static scan may not, the SDK never.';
