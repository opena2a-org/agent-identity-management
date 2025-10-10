-- Migration 031: Drop agent_capability_reports table
-- Purpose: Rollback capability detection reports table
-- Date: October 10, 2025

-- Drop indexes first
DROP INDEX IF EXISTS idx_agent_capability_reports_agent_risk;
DROP INDEX IF EXISTS idx_agent_capability_reports_overall_risk_score;
DROP INDEX IF EXISTS idx_agent_capability_reports_detected_at;
DROP INDEX IF EXISTS idx_agent_capability_reports_risk_level;
DROP INDEX IF EXISTS idx_agent_capability_reports_agent_id;

-- Drop table
DROP TABLE IF EXISTS agent_capability_reports;
