-- Migration: Update trust_scores table schema for 8-factor algorithm
-- Purpose: Migrate from 9-factor to 8-factor trust scoring system

-- Drop old columns
ALTER TABLE trust_scores DROP COLUMN IF EXISTS certificate_validity;
ALTER TABLE trust_scores DROP COLUMN IF EXISTS repository_quality;
ALTER TABLE trust_scores DROP COLUMN IF EXISTS documentation_score;
ALTER TABLE trust_scores DROP COLUMN IF EXISTS community_trust;
ALTER TABLE trust_scores DROP COLUMN IF EXISTS security_audit;
ALTER TABLE trust_scores DROP COLUMN IF EXISTS update_frequency;
ALTER TABLE trust_scores DROP COLUMN IF EXISTS age_score;
ALTER TABLE trust_scores DROP COLUMN IF EXISTS capability_risk;

-- Add new 8-factor columns
ALTER TABLE trust_scores ADD COLUMN IF NOT EXISTS uptime DECIMAL(5,4) DEFAULT 0.0;
ALTER TABLE trust_scores ADD COLUMN IF NOT EXISTS success_rate DECIMAL(5,4) DEFAULT 0.0;
ALTER TABLE trust_scores ADD COLUMN IF NOT EXISTS security_alerts DECIMAL(5,4) DEFAULT 0.0;
ALTER TABLE trust_scores ADD COLUMN IF NOT EXISTS compliance DECIMAL(5,4) DEFAULT 0.0;
ALTER TABLE trust_scores ADD COLUMN IF NOT EXISTS age DECIMAL(5,4) DEFAULT 0.0;
ALTER TABLE trust_scores ADD COLUMN IF NOT EXISTS drift_detection DECIMAL(5,4) DEFAULT 0.0;
ALTER TABLE trust_scores ADD COLUMN IF NOT EXISTS user_feedback DECIMAL(5,4) DEFAULT 0.0;

-- Update verification_status column to match new schema (already exists, just ensure it's there)
-- verification_status column already exists from original schema
