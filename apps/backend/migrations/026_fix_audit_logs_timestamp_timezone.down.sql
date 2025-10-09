-- Rollback: Revert timestamp column to timestamp without time zone
-- WARNING: This will lose timezone information
ALTER TABLE audit_logs
  ALTER COLUMN timestamp TYPE timestamp USING timestamp::timestamp without time zone;

-- Revert default
ALTER TABLE audit_logs
  ALTER COLUMN timestamp SET DEFAULT now();

-- Remove comment
COMMENT ON COLUMN audit_logs.timestamp IS NULL;
