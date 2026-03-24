-- Add trust attenuation fields to delegation chains
-- Each delegation hop reduces effective trust by the attenuation factor
-- Chain fails if effective trust drops below the minimum threshold

ALTER TABLE a2a_delegation ADD COLUMN IF NOT EXISTS trust_attenuation_factor DOUBLE PRECISION NOT NULL DEFAULT 0.8;
ALTER TABLE a2a_delegation ADD COLUMN IF NOT EXISTS min_delegated_trust_score DOUBLE PRECISION NOT NULL DEFAULT 0.3;

COMMENT ON COLUMN a2a_delegation.trust_attenuation_factor IS 'Multiplier applied to trust score at each delegation hop (0.0-1.0, default 0.8)';
COMMENT ON COLUMN a2a_delegation.min_delegated_trust_score IS 'Minimum effective trust score below which delegation chain is invalid (0.0-1.0, default 0.3)';
