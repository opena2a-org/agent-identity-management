-- Migration 094: Mirror the latest mcp_trust_scores.score back to
-- mcp_servers.trust_score atomically via an AFTER INSERT OR UPDATE
-- trigger on mcp_trust_scores.
--
-- The audit doc § 30 observed that `mcp_trust_calculator.go:486-496`
-- writes to both tables sequentially without a transaction. If the
-- second write fails, the row is fresh and the column is stale.
-- Same dual-source-of-truth class as #163 (agents.trust_score vs
-- trust_scores.score), which migration 093 closes.
--
-- `mcp_trust_scores` carries the full factor breakdown + confidence
-- and is the source of truth. `mcp_servers.trust_score` is the
-- denormalized cache for hot-path list rendering of MCP servers.
-- This trigger keeps the cache consistent.
--
-- Closes #170.
--
-- Per the [CHIEF-CA] decision in
-- `todo/2026-05-24-counter-drift-cluster-chief-ca.md`: denormalized
-- cache derived from a detail table uses an AFTER INSERT/UPDATE
-- trigger on the detail table that maintains the parent cache.
-- Same family as migration 091 (capability_violation_count) and
-- migration 093 (agents.trust_score).

CREATE OR REPLACE FUNCTION mirror_mcp_trust_score_to_server()
RETURNS TRIGGER AS $$
BEGIN
    -- AFTER INSERT: NEW row is by definition the latest snapshot.
    -- AFTER UPDATE: only mirror if NEW row IS the latest for this
    -- MCP server. Application code only updates the latest row,
    -- but the guard prevents a future ad-hoc UPDATE on a
    -- historical row from clobbering the cache.
    IF TG_OP = 'INSERT'
       OR NEW.created_at = (
           SELECT MAX(created_at)
           FROM mcp_trust_scores
           WHERE mcp_server_id = NEW.mcp_server_id
       )
    THEN
        UPDATE mcp_servers
        SET trust_score = NEW.score,
            updated_at = NOW()
        WHERE id = NEW.mcp_server_id
          AND trust_score IS DISTINCT FROM NEW.score;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_mirror_mcp_trust_score_to_server ON mcp_trust_scores;

CREATE TRIGGER trg_mirror_mcp_trust_score_to_server
    AFTER INSERT OR UPDATE ON mcp_trust_scores
    FOR EACH ROW
    EXECUTE FUNCTION mirror_mcp_trust_score_to_server();

-- One-shot backfill: reconcile MCP servers whose cached trust_score
-- differs from their latest mcp_trust_scores.score.
UPDATE mcp_servers m
SET trust_score = sub.latest_score,
    updated_at = NOW()
FROM (
    SELECT DISTINCT ON (mcp_server_id) mcp_server_id, score AS latest_score
    FROM mcp_trust_scores
    ORDER BY mcp_server_id, created_at DESC
) sub
WHERE m.id = sub.mcp_server_id
  AND m.trust_score IS DISTINCT FROM sub.latest_score;
