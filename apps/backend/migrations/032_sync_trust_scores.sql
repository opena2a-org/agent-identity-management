-- Migration: Sync agent trust scores with latest calculated values
-- Created: 2025-11-09
-- Description: Fixes trust score synchronization bug where agents.trust_score
--              didn't match the latest trust_scores.score calculation
--
-- Background:
-- Bug #1: trust_calculator.go wasn't updating agents.trust_score field
-- Bug #2: capability_service.go was using wrong scale (0-100 vs 0.0-1.0)
-- Result: agents.trust_score could be 0 while trust_scores had correct value
--
-- This migration syncs agents.trust_score from the latest trust_scores entry
-- for each agent, ensuring consistency going forward.

-- Sync trust scores for agents that have calculated scores
UPDATE agents a
SET trust_score = ts.latest_score,
    updated_at = NOW()
FROM (
    SELECT DISTINCT ON (agent_id)
        agent_id,
        score as latest_score
    FROM trust_scores
    ORDER BY agent_id, created_at DESC
) ts
WHERE a.id = ts.agent_id
  AND ABS(a.trust_score - ts.latest_score) > 0.01; -- Only update if difference > 1% (accounts for NUMERIC(5,2) rounding)

-- Log the synchronization results
DO $$
DECLARE
    sync_count INTEGER;
BEGIN
    SELECT COUNT(*)
    INTO sync_count
    FROM agents a
    JOIN LATERAL (
        SELECT score
        FROM trust_scores
        WHERE agent_id = a.id
        ORDER BY created_at DESC
        LIMIT 1
    ) ts ON ABS(a.trust_score - ts.score) > 0.001;

    IF sync_count > 0 THEN
        RAISE NOTICE 'Synchronized % agent trust scores', sync_count;
    ELSE
        RAISE NOTICE 'All agent trust scores already in sync';
    END IF;
END $$;

-- Verify all agents are now in sync
DO $$
DECLARE
    mismatch_count INTEGER;
    total_agents INTEGER;
BEGIN
    -- Count agents with trust_scores entries
    SELECT COUNT(DISTINCT a.id)
    INTO total_agents
    FROM agents a
    WHERE EXISTS (
        SELECT 1 FROM trust_scores WHERE agent_id = a.id
    );

    -- Count agents still out of sync (using same logic as UPDATE)
    SELECT COUNT(*)
    INTO mismatch_count
    FROM agents a
    JOIN (
        SELECT DISTINCT ON (agent_id)
            agent_id,
            score as latest_score
        FROM trust_scores
        ORDER BY agent_id, created_at DESC
    ) ts ON a.id = ts.agent_id
    WHERE ABS(a.trust_score - ts.latest_score) > 0.01;

    IF mismatch_count > 0 THEN
        RAISE WARNING 'Found % agents still out of sync after migration', mismatch_count;
    ELSE
        RAISE NOTICE 'Migration successful: % agents verified in sync ✓', total_agents;
    END IF;
END $$;
