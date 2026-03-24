-- Add per-capability minimum trust score threshold
-- Capabilities can now require a minimum trust score for execution
-- Default 0.0 means no restriction (backward compatible)

ALTER TABLE capability_definitions ADD COLUMN IF NOT EXISTS min_trust_score DOUBLE PRECISION NOT NULL DEFAULT 0.0;

-- Backfill based on existing risk levels
UPDATE capability_definitions SET min_trust_score = 0.0 WHERE risk_level = 'low';
UPDATE capability_definitions SET min_trust_score = 0.3 WHERE risk_level = 'medium';
UPDATE capability_definitions SET min_trust_score = 0.5 WHERE risk_level = 'high';
UPDATE capability_definitions SET min_trust_score = 0.7 WHERE risk_level = 'critical';

COMMENT ON COLUMN capability_definitions.min_trust_score IS 'Minimum agent trust score (0.0-1.0) required to execute this capability';
