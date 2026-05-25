-- Migration 093: Mirror the latest trust_scores.score back to
-- agents.trust_score atomically via an AFTER INSERT OR UPDATE
-- trigger on trust_scores.
--
-- The audit doc § 5 observed the agents list showing 92% while the
-- agent detail page showed 61.3% for the SAME agent at the SAME
-- moment. Root cause: the dashboard list query reads
-- `agents.trust_score` (a hot-path denormalized cache), the detail
-- page reads the latest row from `trust_scores` (the factor
-- breakdown). Application code syncs both on calculate / direct
-- penalty paths, but any external writer that touches one without
-- the other (admin tool, script, manual SQL) produces a visible
-- split-brain.
--
-- The richer record is `trust_scores` (full 8-factor breakdown).
-- That is the source of truth; `agents.trust_score` is a cache.
-- This trigger keeps the cache consistent.
--
-- Closes #163.
--
-- Per the [CHIEF-CA] decision in
-- `todo/2026-05-24-counter-drift-cluster-chief-ca.md`: denormalized
-- cache derived from a detail table uses an AFTER INSERT/UPDATE
-- trigger on the detail table that maintains the parent cache.
-- Same family as migration 091 (capability_violation_count).

CREATE OR REPLACE FUNCTION mirror_trust_score_to_agent()
RETURNS TRIGGER AS $$
BEGIN
    -- AFTER INSERT: NEW row is by definition the latest snapshot
    -- (created_at defaults to NOW). Mirror unconditionally.
    --
    -- AFTER UPDATE: only mirror if NEW row IS the latest for this
    -- agent. UpdateScore() in trust_score_repository.go targets
    -- only the latest row, but a future ad-hoc UPDATE on a
    -- historical row must not overwrite the agent cache with a
    -- stale score.
    IF TG_OP = 'INSERT'
       OR NEW.created_at = (
           SELECT MAX(created_at)
           FROM trust_scores
           WHERE agent_id = NEW.agent_id
       )
    THEN
        UPDATE agents
        SET trust_score = NEW.score,
            updated_at = NOW()
        WHERE id = NEW.agent_id
          AND trust_score IS DISTINCT FROM NEW.score;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_mirror_trust_score_to_agent ON trust_scores;

CREATE TRIGGER trg_mirror_trust_score_to_agent
    AFTER INSERT OR UPDATE ON trust_scores
    FOR EACH ROW
    EXECUTE FUNCTION mirror_trust_score_to_agent();

-- One-shot backfill: reconcile agents whose cached trust_score
-- differs from their latest trust_scores.score.
UPDATE agents a
SET trust_score = sub.latest_score,
    updated_at = NOW()
FROM (
    SELECT DISTINCT ON (agent_id) agent_id, score AS latest_score
    FROM trust_scores
    ORDER BY agent_id, created_at DESC
) sub
WHERE a.id = sub.agent_id
  AND a.trust_score IS DISTINCT FROM sub.latest_score;
