-- Migration 092: BEFORE UPDATE trigger on agents to set verified_at
-- when status transitions to 'verified' and verified_at is still NULL.
--
-- The application-layer `VerifyAgent` service (agent_service.go:637)
-- already sets `agent.VerifiedAt = now` before persisting. The gap
-- this trigger closes is the direct-SQL path: ops scripts, manual
-- admin DB sessions, future migrations, and any non-service writer
-- that does `UPDATE agents SET status = 'verified' WHERE ...`. The
-- audit doc § 10 observed agents with status='verified' but
-- verified_at NULL on the dashboard for exactly this reason.
--
-- Closes the verified_at half of #167.
--
-- Per the [CHIEF-CA] decision in
-- `todo/2026-05-24-counter-drift-cluster-chief-ca.md`:
-- row-internal coupling (two columns on the same row that must move
-- together) uses a BEFORE UPDATE trigger on the parent table; this
-- is the canonical fix pattern for that shape.

CREATE OR REPLACE FUNCTION set_verified_at_on_status_change()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'verified'
       AND (OLD.status IS NULL OR OLD.status <> 'verified')
       AND NEW.verified_at IS NULL THEN
        NEW.verified_at := NOW();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_set_verified_at ON agents;

CREATE TRIGGER trg_set_verified_at
    BEFORE UPDATE ON agents
    FOR EACH ROW
    EXECUTE FUNCTION set_verified_at_on_status_change();

-- One-shot backfill: for agents already in 'verified' state with a
-- NULL verified_at (the drifted rows the audit observed), set
-- verified_at to the row's updated_at as a best-available
-- approximation. updated_at is the closest timestamp we have to when
-- the status flip happened; we accept the approximation rather than
-- leave the dashboard rendering "Verified at: never" indefinitely.
UPDATE agents
SET verified_at = COALESCE(updated_at, created_at, NOW())
WHERE status = 'verified'
  AND verified_at IS NULL;
