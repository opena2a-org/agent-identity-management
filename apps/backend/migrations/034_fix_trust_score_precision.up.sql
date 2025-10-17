-- Fix trust score precision to accommodate 0-100 scale values
-- Previously was numeric(4,3) which only allowed -9.999 to 9.999
-- Now is numeric(5,2) which allows -999.99 to 999.99

-- Update agents table
ALTER TABLE agents ALTER COLUMN trust_score TYPE numeric(5,2);

-- Update trust_scores table
ALTER TABLE trust_scores ALTER COLUMN score TYPE numeric(5,2);
ALTER TABLE trust_scores ALTER COLUMN verification_status TYPE numeric(5,2);
ALTER TABLE trust_scores ALTER COLUMN certificate_validity TYPE numeric(5,2);
ALTER TABLE trust_scores ALTER COLUMN repository_quality TYPE numeric(5,2);
ALTER TABLE trust_scores ALTER COLUMN documentation_score TYPE numeric(5,2);
ALTER TABLE trust_scores ALTER COLUMN community_trust TYPE numeric(5,2);
ALTER TABLE trust_scores ALTER COLUMN security_audit TYPE numeric(5,2);
ALTER TABLE trust_scores ALTER COLUMN update_frequency TYPE numeric(5,2);
ALTER TABLE trust_scores ALTER COLUMN age_score TYPE numeric(5,2);
ALTER TABLE trust_scores ALTER COLUMN confidence TYPE numeric(5,2);

-- Add comments for documentation
COMMENT ON COLUMN agents.trust_score IS 'Agent trust score (0-100 scale)';
COMMENT ON COLUMN trust_scores.score IS 'Calculated trust score (0-100 scale)';