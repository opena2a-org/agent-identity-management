-- Migration 095: Derive a2a_trust_scores.peer_trust_average and
-- unique_peers_count from a2a_peer_trust rows via an AFTER
-- INSERT/UPDATE/DELETE trigger.
--
-- The audit doc § 29 observed that a2a_repository.go:1700-1727
-- writes peer_trust_average and unique_peers_count directly to
-- a2a_trust_scores instead of computing them from the actual
-- a2a_peer_trust rows. If any peer-trust row is deleted, modified,
-- or fails to persist, the aggregates lie.
--
-- This trigger makes the aggregates a derived view of the peer
-- rows. Application writes to those columns become a hint; the
-- trigger recomputes them from ground truth after every peer row
-- mutation.
--
-- Closes #169.
--
-- Per the [CHIEF-CA] decision in
-- `todo/2026-05-24-counter-drift-cluster-chief-ca.md`: aggregate
-- of child rows uses an AFTER INSERT/UPDATE/DELETE trigger on the
-- child table. Same family as migration 091
-- (capability_violation_count) — that one was a COUNT, this one is
-- COUNT(DISTINCT) + AVG.

CREATE OR REPLACE FUNCTION recompute_a2a_peer_aggregates()
RETURNS TRIGGER AS $$
DECLARE
    affected_agent UUID;
    new_avg FLOAT;
    new_count INT;
BEGIN
    -- INSERT and UPDATE expose NEW; DELETE only OLD. UPDATE that
    -- moves a row between agent_ids (rare but possible) is handled
    -- by recomputing for BOTH the old and the new agent.
    IF TG_OP = 'DELETE' THEN
        affected_agent := OLD.agent_id;
    ELSE
        affected_agent := NEW.agent_id;
    END IF;

    SELECT
        AVG(peer_trust_score) FILTER (WHERE peer_trust_score IS NOT NULL),
        COUNT(DISTINCT peer_agent_id)
    INTO new_avg, new_count
    FROM a2a_peer_trust
    WHERE agent_id = affected_agent;

    -- Upsert: a2a_trust_scores has UNIQUE(agent_id) per the table
    -- definition (see application code at lines 1709-1725 which
    -- uses ON CONFLICT (agent_id)). If no row exists yet, create
    -- one with only the aggregate fields set.
    INSERT INTO a2a_trust_scores (
        agent_id, peer_trust_average, unique_peers_count,
        created_at, updated_at
    )
    VALUES (affected_agent, new_avg, COALESCE(new_count, 0), NOW(), NOW())
    ON CONFLICT (agent_id) DO UPDATE SET
        peer_trust_average = EXCLUDED.peer_trust_average,
        unique_peers_count = EXCLUDED.unique_peers_count,
        updated_at = NOW()
    WHERE a2a_trust_scores.peer_trust_average IS DISTINCT FROM EXCLUDED.peer_trust_average
       OR a2a_trust_scores.unique_peers_count   IS DISTINCT FROM EXCLUDED.unique_peers_count;

    -- For UPDATE that moves a row across agents, recompute the
    -- OLD agent too.
    IF TG_OP = 'UPDATE' AND OLD.agent_id IS DISTINCT FROM NEW.agent_id THEN
        SELECT
            AVG(peer_trust_score) FILTER (WHERE peer_trust_score IS NOT NULL),
            COUNT(DISTINCT peer_agent_id)
        INTO new_avg, new_count
        FROM a2a_peer_trust
        WHERE agent_id = OLD.agent_id;

        INSERT INTO a2a_trust_scores (
            agent_id, peer_trust_average, unique_peers_count,
            created_at, updated_at
        )
        VALUES (OLD.agent_id, new_avg, COALESCE(new_count, 0), NOW(), NOW())
        ON CONFLICT (agent_id) DO UPDATE SET
            peer_trust_average = EXCLUDED.peer_trust_average,
            unique_peers_count = EXCLUDED.unique_peers_count,
            updated_at = NOW()
        WHERE a2a_trust_scores.peer_trust_average IS DISTINCT FROM EXCLUDED.peer_trust_average
           OR a2a_trust_scores.unique_peers_count   IS DISTINCT FROM EXCLUDED.unique_peers_count;
    END IF;

    -- AFTER triggers return value is ignored for table modification;
    -- conventional to return NEW/OLD anyway.
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    ELSE
        RETURN NEW;
    END IF;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_recompute_a2a_peer_aggregates ON a2a_peer_trust;

CREATE TRIGGER trg_recompute_a2a_peer_aggregates
    AFTER INSERT OR UPDATE OR DELETE ON a2a_peer_trust
    FOR EACH ROW
    EXECUTE FUNCTION recompute_a2a_peer_aggregates();

-- One-shot backfill: reconcile any agents whose a2a_trust_scores
-- aggregates differ from the actual peer rows.
INSERT INTO a2a_trust_scores (
    agent_id, peer_trust_average, unique_peers_count,
    created_at, updated_at
)
SELECT
    p.agent_id,
    AVG(p.peer_trust_score) FILTER (WHERE p.peer_trust_score IS NOT NULL),
    COUNT(DISTINCT p.peer_agent_id),
    NOW(),
    NOW()
FROM a2a_peer_trust p
GROUP BY p.agent_id
ON CONFLICT (agent_id) DO UPDATE SET
    peer_trust_average = EXCLUDED.peer_trust_average,
    unique_peers_count = EXCLUDED.unique_peers_count,
    updated_at = NOW()
WHERE a2a_trust_scores.peer_trust_average IS DISTINCT FROM EXCLUDED.peer_trust_average
   OR a2a_trust_scores.unique_peers_count   IS DISTINCT FROM EXCLUDED.unique_peers_count;
