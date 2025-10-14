-- Revert trust score precision to original values
-- Change back to numeric(4,3) which only allows -9.999 to 9.999

-- Update trust_scores table (must be done first due to foreign key constraints)
ALTER TABLE trust_scores ALTER COLUMN confidence TYPE numeric(4,3);
ALTER TABLE trust_scores ALTER COLUMN age_score TYPE numeric(4,3);
ALTER TABLE trust_scores ALTER COLUMN update_frequency TYPE numeric(4,3);
ALTER TABLE trust_scores ALTER COLUMN security_audit TYPE numeric(4,3);
ALTER TABLE trust_scores ALTER COLUMN community_trust TYPE numeric(4,3);
ALTER TABLE trust_scores ALTER COLUMN documentation_score TYPE numeric(4,3);
ALTER TABLE trust_scores ALTER COLUMN repository_quality TYPE numeric(4,3);
ALTER TABLE trust_scores ALTER COLUMN certificate_validity TYPE numeric(4,3);
ALTER TABLE trust_scores ALTER COLUMN verification_status TYPE numeric(4,3);
ALTER TABLE trust_scores ALTER COLUMN score TYPE numeric(4,3);

-- Update agents table
ALTER TABLE agents ALTER COLUMN trust_score TYPE numeric(4,3);

-- Add comments for documentation
COMMENT ON COLUMN agents.trust_score IS 'Agent trust score';
COMMENT ON COLUMN trust_scores.score IS 'Calculated trust score';