-- Migration 104: make [0,1] the single canonical scale for MCP trust scores,
-- and enforce it in the schema.
--
-- `mcp_servers.trust_score` (mig 004) is DECIMAL(5,2) with no CHECK
-- constraint, so it has been accepting three mutually incompatible scales:
--
--   [0,1]   the 8-factor calculator clamps its output to [0,1]
--           (`mcp_trust_calculator.go`), and `mcp_trust_scores.score` is
--           DECIMAL(5,4) CHECK (score >= 0 AND score <= 1) (mig 051).
--   [0,100] mig 061 declared this column a 0-100 field, and
--           `mcp_service.go` wrote the literal 75.0 into it.
--   [0,10]  `tag_service.suggestTagsForTrustScore` branches on >= 8.0 / >= 5.0.
--
-- Migration 094 made `mcp_trust_scores` the source of truth and mirrors it
-- with `SET trust_score = NEW.score` — unscaled. So the cache column must
-- carry the SAME scale as its source, not a transform of it; a transformed
-- copy is the defect class 094 was written to close (#170).
--
-- [0,1] is also the scale every other trust value in this schema already
-- uses: `agents.trust_score` (CHECK 0..1, mig 031), `trust_scores.score`,
-- and the `MinTrustScore` policy thresholds the evaluator compares against.
--
-- Per the [CHIEF-CDS] decision + [CHIEF-CA] second opinion of 2026-08-04 in
-- `todo/COUNCIL_LEDGER.md`.

-- Audit trail: report what is about to be rewritten.
DO $$
DECLARE
    server_count INTEGER;
    server_max   NUMERIC;
    history_count INTEGER;
BEGIN
    SELECT COUNT(*), MAX(trust_score) INTO server_count, server_max
    FROM mcp_servers WHERE trust_score > 1.0;

    SELECT COUNT(*) INTO history_count
    FROM mcp_trust_score_history WHERE trust_score > 1.0;

    RAISE NOTICE 'mcp_servers: % rows above 1.0 (max %), normalizing to [0,1]',
        server_count, server_max;
    RAISE NOTICE 'mcp_trust_score_history: % rows above 1.0, normalizing to [0,1]',
        history_count;
END $$;

-- 1. Drop the audit trigger BEFORE touching any value.
--
--    `mcp_trust_score_change_trigger` (mig 051) records every change to
--    `mcp_servers.trust_score` into `mcp_trust_score_history`. A rescaling is
--    not a change in trust, but the trigger cannot tell the difference: left
--    installed, it writes an audit row claiming each server's score fell from
--    75.00 to 0.75. That is a fabricated security event, and in an audit
--    trail it is worse than the defect being fixed.
--
--    Postgres also refuses to alter the type of a column named in a trigger's
--    AFTER UPDATE OF list, so the trigger has to come off for step 3 anyway.
DROP TRIGGER IF EXISTS mcp_trust_score_change_trigger ON mcp_servers;

-- 2. Normalize existing out-of-scale values BEFORE narrowing the column type.
--    Anything above 1.0 was written on the 0-100 scale (in practice: the
--    hardcoded 75.0). Narrowing first would raise numeric_value_out_of_range.
UPDATE mcp_servers
SET trust_score = trust_score / 100.0
WHERE trust_score > 1.0;

-- 3. Widen the fractional precision to match the source of truth.
--    At DECIMAL(5,2) the mig 094 mirror trigger silently rounds a calculated
--    0.8234 down to 0.82, so the cache disagrees with `mcp_trust_scores.score`
--    on every write — #170 reopened by rounding.
ALTER TABLE mcp_servers
ALTER COLUMN trust_score TYPE DECIMAL(5,4);

-- 4. Restore the audit trigger, unchanged.
CREATE TRIGGER mcp_trust_score_change_trigger
    AFTER UPDATE OF trust_score ON mcp_servers
    FOR EACH ROW
    EXECUTE FUNCTION record_mcp_trust_score_change();

-- 5. Enforce the scale so an out-of-range literal can never be stored again.
ALTER TABLE mcp_servers
DROP CONSTRAINT IF EXISTS mcp_servers_trust_score_range_check;

ALTER TABLE mcp_servers
ADD CONSTRAINT mcp_servers_trust_score_range_check
CHECK (trust_score >= 0.0 AND trust_score <= 1.0);

-- 6. Return `mcp_trust_score_history` to [0,1], undoing mig 061.
--    Mig 061 moved this table to 0-100 to accommodate the 75.0 literal that
--    this migration removes. The table is populated by
--    `record_mcp_trust_score_change` (mig 051), which copies
--    `mcp_servers.trust_score` on every change — so leaving it on the old
--    scale would mean the audit trail of a score disagreed with the score.
--    No Go code writes it directly; the repository only SELECTs from it at
--    `mcp_trust_score_repository.go:150`.
UPDATE mcp_trust_score_history
SET trust_score = trust_score / 100.0
WHERE trust_score > 1.0;

UPDATE mcp_trust_score_history
SET previous_score = previous_score / 100.0
WHERE previous_score IS NOT NULL AND previous_score > 1.0;

ALTER TABLE mcp_trust_score_history
DROP CONSTRAINT IF EXISTS mcp_trust_score_history_trust_score_check;

ALTER TABLE mcp_trust_score_history
DROP CONSTRAINT IF EXISTS mcp_trust_score_history_previous_score_check;

ALTER TABLE mcp_trust_score_history
ALTER COLUMN trust_score TYPE DECIMAL(5,4);

ALTER TABLE mcp_trust_score_history
ALTER COLUMN previous_score TYPE DECIMAL(5,4);

ALTER TABLE mcp_trust_score_history
ADD CONSTRAINT mcp_trust_score_history_trust_score_check
CHECK (trust_score >= 0.0 AND trust_score <= 1.0);

ALTER TABLE mcp_trust_score_history
ADD CONSTRAINT mcp_trust_score_history_previous_score_check
CHECK (previous_score IS NULL OR (previous_score >= 0.0 AND previous_score <= 1.0));

-- 7. Document the canonical scale on the column itself, so the next reader
--    does not have to reconstruct it from three disagreeing call sites.
COMMENT ON COLUMN mcp_servers.trust_score IS
    'Denormalized cache of the latest mcp_trust_scores.score. Scale [0,1], '
    'enforced by mcp_servers_trust_score_range_check. Written only by the '
    '8-factor calculator via the mig 094 mirror trigger — never by a literal.';
