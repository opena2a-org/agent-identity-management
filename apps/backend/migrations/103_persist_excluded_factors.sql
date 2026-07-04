-- Migration 103: Persist excluded factors on trust_scores
--
-- AIP-SPEC section 6.1 composition (exclude-and-redistribute with the
-- anti-gaming cap) makes the set of excluded factors part of what a stored
-- composite means: a capped or renormalized score is not reproducible from
-- the row without knowing which factors were excluded. Until now
-- TrustScore.ExcludedFactors was API-response-only, so stored rows held
-- placeholder factor values with no record of the exclusion set (audit gap).
--
-- Companion repository change also starts writing/reading the
-- execution_isolation factor column (added in migration 090 but never
-- referenced by the Go repository, silently dropping factor 9 on write).

ALTER TABLE trust_scores
    ADD COLUMN IF NOT EXISTS excluded_factors TEXT[] NOT NULL DEFAULT '{}';

COMMENT ON COLUMN trust_scores.excluded_factors IS
    'Factor keys excluded from the composite under AIP-SPEC 6.1 (no data or un-wired source; weights redistributed with the anti-gaming cap). Empty array = all nine factors measured. Rows predating migration 103 read as empty.';
