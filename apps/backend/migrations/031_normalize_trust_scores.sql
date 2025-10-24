-- Migration: Normalize trust scores to 0-1 decimal scale
-- Issue: Some trust scores stored as 0-100 scale, should be 0-1 scale
-- Fix: Divide any score > 1.0 by 100 to normalize to 0-1 scale

-- First, let's see what we're dealing with (for audit trail)
DO $$
DECLARE
    bad_score_count INTEGER;
    min_score DECIMAL(5,2);
    max_score DECIMAL(5,2);
    avg_score DECIMAL(5,2);
BEGIN
    -- Count scores that need normalization (> 1.0)
    SELECT COUNT(*), MIN(trust_score), MAX(trust_score), AVG(trust_score)
    INTO bad_score_count, min_score, max_score, avg_score
    FROM trust_score_history
    WHERE trust_score > 1.0;

    RAISE NOTICE 'Found % trust scores > 1.0 that need normalization', bad_score_count;
    RAISE NOTICE 'Min bad score: %, Max bad score: %, Avg bad score: %', min_score, max_score, avg_score;
END $$;

-- Normalize trust scores in trust_score_history table
-- Any score > 1.0 is assumed to be on 0-100 scale and divided by 100
UPDATE trust_score_history
SET trust_score = trust_score / 100.0
WHERE trust_score > 1.0;

-- Normalize trust scores in agents table (trust_score)
UPDATE agents
SET trust_score = trust_score / 100.0
WHERE trust_score > 1.0;

-- Add a check constraint to prevent future bad data
-- This ensures all trust scores are between 0 and 1
ALTER TABLE trust_score_history
DROP CONSTRAINT IF EXISTS trust_score_range_check;

ALTER TABLE trust_score_history
ADD CONSTRAINT trust_score_range_check
CHECK (trust_score >= 0.0 AND trust_score <= 1.0);

ALTER TABLE agents
DROP CONSTRAINT IF EXISTS trust_score_range_check;

ALTER TABLE agents
ADD CONSTRAINT trust_score_range_check
CHECK (trust_score >= 0.0 AND trust_score <= 1.0);

-- Log results for audit trail
DO $$
DECLARE
    history_count INTEGER;
    agents_count INTEGER;
    history_avg DECIMAL(5,2);
    agents_avg DECIMAL(5,2);
BEGIN
    -- Get stats after normalization
    SELECT COUNT(*), AVG(trust_score)
    INTO history_count, history_avg
    FROM trust_score_history;

    SELECT COUNT(*), AVG(trust_score)
    INTO agents_count, agents_avg
    FROM agents
    WHERE trust_score IS NOT NULL;

    RAISE NOTICE 'Normalization complete!';
    RAISE NOTICE 'Trust score history: % records, avg score: %', history_count, history_avg;
    RAISE NOTICE 'Agents current scores: % records, avg score: %', agents_count, agents_avg;
END $$;
