-- Migration 110: contextRules.onUnavailable for the FGA context check (AIM-08).
--
-- The FGA engine (fga_engine.go, Step 3) reads fga_policies.context_rules as a
-- camelCase JSON document. The keys the engine reads are:
--
--   maxDriftScore     (number)  deny when the ASC drift score exceeds it
--   requireScanClean  (bool)    deny unless the ASC scan verdict is 'clean'
--   maxAlerts         (int)     deny when active alerts exceed it
--   minTrustLevel     (int)     deny when the trust level is below it
--   onUnavailable     (text)    'deny' | 'allow' — the decision when the ASC
--                               risk summary cannot be evaluated. Absent,
--                               unknown, or unparseable all fail closed to
--                               deny (an unevaluable rule is not satisfied).
--
-- (The snake_case examples in migration 078's column comment — e.g.
-- "max_drift_score" — are not keys the engine reads; the engine has always
-- parsed camelCase. This header is the corrected reference.)
--
-- The CHECK below rejects unknown onUnavailable values at write time so a
-- typo like "skip" cannot sit latently in a policy document: the engine
-- fail-closes on such a value anyway, but catching it at the INSERT/UPDATE
-- turns a silent future deny into an immediate, attributable error.
--
-- Numbering note: this tree's highest migration is 108 and aim-cloud's is
-- 109_row_level_security_tenant_isolation.sql; the runner tracks applied
-- migrations by filename, so 110 is the first number free in both trees and
-- the gap at 109 here is legal.

ALTER TABLE fga_policies
    ADD CONSTRAINT fga_policies_context_rules_on_unavailable_check
    CHECK (context_rules->>'onUnavailable' IS NULL OR context_rules->>'onUnavailable' IN ('deny','allow'));
