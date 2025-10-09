-- Rollback: Revert the timestamp correction
-- WARNING: This assumes the +6 hour adjustment is being rolled back

UPDATE audit_logs
SET timestamp = timestamp - INTERVAL '6 hours'
WHERE timestamp < NOW() - INTERVAL '1 minute';

-- Remove comment
COMMENT ON TABLE audit_logs IS NULL;
