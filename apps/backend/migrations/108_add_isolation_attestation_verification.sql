-- Verification columns on isolation_attestations (trust factor 9).
--
-- Factor 9 is fed by an agent SDK self-report: the agent names its own sandbox, network,
-- filesystem and process isolation and the server scores the posture. The server already
-- computes the score (the agent cannot inject one) and rejects unrecognized enum values,
-- but nothing checks whether the claim is TRUE. An agent that reports
-- firecracker + airgap + readonly + full earns 1.0 on a 10%-weight factor for the cost of
-- four strings.
--
-- The fix has two halves. The scoring half is read-side and needs no schema: an unverified
-- report is clipped to the commodity-container tier (0.65) and any report older than 90 days
-- stops counting. This migration is the other half — the place a verification would be
-- recorded once an independent source exists.
--
-- `verified` is bound to the ROW, not to the agent, and that is the load-bearing part. A
-- verification describes one report of one deployment at one moment. Binding it to the agent
-- would let a deployment verified in March vouch for a posture reported in September; binding
-- it to the row means a re-attestation (a newer reported_at) supersedes its predecessor and
-- starts unverified, with no carry-forward. The 90-day expiry closes the same door from the
-- other side.
--
-- NOTHING WRITES verified = TRUE TODAY, and that is deliberate rather than unfinished. There
-- is no independent verification source yet; adding a writable flag before there is something
-- honest to write into it would produce a column that means "someone said so", which is the
-- exact weakness the column exists to fix. The ingest path hard-sets false and the repository
-- INSERT writes the literal FALSE. When a source does arrive, the evidence classes that may
-- set this are TEE attestation and orchestrator/host metadata. An HMA static scan may NOT:
-- it reads a declared surface, not the running one, so it would launder a second self-report
-- into a verification. The SDK may never, being the self-report under check.
--
-- DEFAULT FALSE + NOT NULL means every existing row reads as unverified without a backfill,
-- which is the truth about all of them.

ALTER TABLE isolation_attestations
    ADD COLUMN IF NOT EXISTS verified     BOOLEAN     NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS verified_by  TEXT        NULL,
    ADD COLUMN IF NOT EXISTS verified_at  TIMESTAMPTZ NULL;

COMMENT ON COLUMN isolation_attestations.verified IS
    'TRUE only when an INDEPENDENT source corroborated THIS row''s posture. Bound to the row, '
    'never to the agent: a newer attestation starts unverified and inherits nothing. No write '
    'path sets this TRUE (roadmap aim-isolation-verification Phase 2); TEE attestation and '
    'orchestrator/host metadata may, an HMA static scan may not, the SDK never.';

COMMENT ON COLUMN isolation_attestations.verified_by IS
    'Identity of the independent verifier that corroborated the posture. NULL while verified '
    'is FALSE, which is every row today.';

COMMENT ON COLUMN isolation_attestations.verified_at IS
    'When the independent verification happened. Distinct from reported_at, which is when the '
    'agent made the claim; the 90-day scoring expiry keys off reported_at, not this.';
