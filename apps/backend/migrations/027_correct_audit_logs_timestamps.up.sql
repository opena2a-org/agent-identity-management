-- Correct existing audit_logs timestamps that were stored in Mountain Time (UTC-6)
-- but are now marked as UTC after the column type change
--
-- The issue: timestamps like "2025-10-09 01:16:51" were in Mountain Time,
-- but after converting to timestamptz they became "2025-10-09 01:16:51+00" (UTC)
-- This makes them 6 hours early.
--
-- The fix: Add 6 hours to all existing timestamps to convert from Mountain Time to UTC

UPDATE audit_logs
SET timestamp = timestamp + INTERVAL '6 hours'
WHERE timestamp < NOW() - INTERVAL '1 minute';  -- Only fix historical data, not brand new entries

-- Add comment explaining the correction
COMMENT ON TABLE audit_logs IS 'Audit log timestamps corrected in migration 027. All timestamps now properly stored in UTC.';
