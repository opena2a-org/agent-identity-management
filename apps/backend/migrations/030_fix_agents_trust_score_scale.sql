-- Migration: Fix agents.trust_score column to support 0-100 scale
-- Purpose: Change from DECIMAL(4,3) to DECIMAL(5,2) to match other tables

-- Drop trigger that uses trust_score column
DROP TRIGGER IF EXISTS trigger_log_trust_score ON agents;

-- Alter agents.trust_score to support 0-100 scale
ALTER TABLE agents
  ALTER COLUMN trust_score TYPE DECIMAL(5,2);

-- Update existing trust scores to 0-100 scale (multiply by 100 if < 10)
UPDATE agents
SET trust_score = CASE
  WHEN trust_score < 10 THEN trust_score * 100
  ELSE trust_score
END;

-- Recreate the trigger with new column type
CREATE TRIGGER trigger_log_trust_score
AFTER UPDATE ON agents
FOR EACH ROW
WHEN (NEW.trust_score IS DISTINCT FROM OLD.trust_score)
EXECUTE FUNCTION log_trust_score_change();
