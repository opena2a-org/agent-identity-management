-- Migration: Rollback detections audit table
-- Created: 2025-10-09

DROP INDEX IF EXISTS idx_detections_audit_method;
DROP INDEX IF EXISTS idx_detections_audit_timestamp;
DROP INDEX IF EXISTS idx_detections_audit_significant;
DROP INDEX IF EXISTS idx_detections_audit_agent_mcp;
DROP INDEX IF EXISTS idx_detections_audit_agent;
DROP TABLE IF EXISTS detections;
