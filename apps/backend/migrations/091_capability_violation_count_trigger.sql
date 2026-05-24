-- Migration 091: AFTER INSERT trigger on capability_violations to keep
-- agents.capability_violation_count consistent with the actual row count.
--
-- Replaces the application-layer `IncrementViolationCount` pattern, which
-- was paired with only 2 of 5 `CreateViolation` call sites in
-- `internal/application/agent_service.go`, leaving the counter stale on
-- the monitoring-mode and SDK-LogActivity paths.
--
-- Closes #168 (capability_violation_count drift) and #166 (count not
-- ticking on the dashboard, which was a symptom of #168).
--
-- Per the [CHIEF-CA] decision in
-- `todo/2026-05-24-counter-drift-cluster-chief-ca.md`:
-- aggregate-of-child-rows fields use an AFTER INSERT trigger on the
-- child table; this is the canonical fix pattern for that shape.

CREATE OR REPLACE FUNCTION bump_capability_violation_count()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE agents
    SET capability_violation_count = COALESCE(capability_violation_count, 0) + 1,
        updated_at = NOW()
    WHERE id = NEW.agent_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_bump_capability_violation_count ON capability_violations;

CREATE TRIGGER trg_bump_capability_violation_count
    AFTER INSERT ON capability_violations
    FOR EACH ROW
    EXECUTE FUNCTION bump_capability_violation_count();

-- One-shot backfill: reconcile any existing drift between
-- capability_violation_count and the actual row count.
-- Safe to re-run; the UPDATE is idempotent.
UPDATE agents a
SET capability_violation_count = sub.cnt,
    updated_at = NOW()
FROM (
    SELECT agent_id, COUNT(*)::INTEGER AS cnt
    FROM capability_violations
    GROUP BY agent_id
) sub
WHERE a.id = sub.agent_id
  AND a.capability_violation_count IS DISTINCT FROM sub.cnt;

-- Agents with zero violations: ensure the counter is 0, not NULL, so
-- subsequent trigger fires increment from a known base.
UPDATE agents
SET capability_violation_count = 0
WHERE capability_violation_count IS NULL;
