-- Fix trust_score_history table to match mcp_servers trust_score scale (0-100 instead of 0-1)
-- The mcp_servers table uses numeric(5,2) for 0-100 scale
-- But mcp_trust_score_history was using numeric(5,4) with 0-1 scale
-- This causes "numeric field overflow" errors when trust_score is set to values like 75.0

-- Drop the old check constraints
ALTER TABLE mcp_trust_score_history DROP CONSTRAINT IF EXISTS mcp_trust_score_history_trust_score_check;
ALTER TABLE mcp_trust_score_history DROP CONSTRAINT IF EXISTS mcp_trust_score_history_previous_score_check;

-- Change the column types to match mcp_servers (numeric(5,2) for 0-100 scale)
ALTER TABLE mcp_trust_score_history ALTER COLUMN trust_score TYPE numeric(5,2);
ALTER TABLE mcp_trust_score_history ALTER COLUMN previous_score TYPE numeric(5,2);

-- Add new check constraints for 0-100 scale
ALTER TABLE mcp_trust_score_history ADD CONSTRAINT mcp_trust_score_history_trust_score_check
    CHECK (trust_score >= 0 AND trust_score <= 100);

ALTER TABLE mcp_trust_score_history ADD CONSTRAINT mcp_trust_score_history_previous_score_check
    CHECK (previous_score IS NULL OR (previous_score >= 0 AND previous_score <= 100));

-- Update any existing data (multiply by 100 to convert from 0-1 scale to 0-100 scale)
UPDATE mcp_trust_score_history SET trust_score = trust_score * 100 WHERE trust_score <= 1;
UPDATE mcp_trust_score_history SET previous_score = previous_score * 100 WHERE previous_score IS NOT NULL AND previous_score <= 1;
