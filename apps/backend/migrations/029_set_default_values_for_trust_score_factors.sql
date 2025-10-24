-- Migration: Set default values for trust score factor columns
-- Purpose: Allow NULL values for the 8-factor columns until they are properly calculated
-- This is a temporary fix to allow capability reports to be stored

-- Alter columns to allow NULL temporarily (using NEW 8-factor column names from migration 019)
ALTER TABLE trust_scores
  ALTER COLUMN verification_status DROP NOT NULL,
  ALTER COLUMN uptime DROP NOT NULL,
  ALTER COLUMN success_rate DROP NOT NULL,
  ALTER COLUMN security_alerts DROP NOT NULL,
  ALTER COLUMN compliance DROP NOT NULL,
  ALTER COLUMN age DROP NOT NULL,
  ALTER COLUMN drift_detection DROP NOT NULL,
  ALTER COLUMN user_feedback DROP NOT NULL;

-- Set default values for existing rows that might be NULL
UPDATE trust_scores
SET
  verification_status = COALESCE(verification_status, 0),
  uptime = COALESCE(uptime, 0),
  success_rate = COALESCE(success_rate, 0),
  security_alerts = COALESCE(security_alerts, 0),
  compliance = COALESCE(compliance, 0),
  age = COALESCE(age, 0),
  drift_detection = COALESCE(drift_detection, 0),
  user_feedback = COALESCE(user_feedback, 0);
