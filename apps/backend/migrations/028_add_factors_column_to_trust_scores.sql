-- Migration: Add factors JSONB column to trust_scores table
-- Purpose: Support capability reporting which needs to store factors as JSON

-- Add factors column for capability reporting
ALTER TABLE trust_scores ADD COLUMN IF NOT EXISTS factors JSONB DEFAULT '{}'::jsonb;

-- Add index for factors JSONB queries
CREATE INDEX IF NOT EXISTS idx_trust_scores_factors ON trust_scores USING gin(factors);
