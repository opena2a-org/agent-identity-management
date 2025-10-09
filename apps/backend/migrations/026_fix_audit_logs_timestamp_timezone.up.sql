-- Fix audit_logs timestamp column to use timestamptz (with timezone)
-- This ensures timestamps are stored in UTC and correctly calculated
ALTER TABLE audit_logs
  ALTER COLUMN timestamp TYPE timestamptz USING timestamp AT TIME ZONE 'UTC';

-- Update default to use timezone-aware now()
ALTER TABLE audit_logs
  ALTER COLUMN timestamp SET DEFAULT now();

-- Add comment explaining the fix
COMMENT ON COLUMN audit_logs.timestamp IS 'UTC timestamp with timezone. Fixed in migration 026 to resolve timezone calculation issues.';
