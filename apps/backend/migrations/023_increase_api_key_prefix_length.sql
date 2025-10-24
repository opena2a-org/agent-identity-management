-- Migration: Increase API key prefix length from VARCHAR(8) to VARCHAR(16)
-- Issue: API key generation fails with "pq: value too long for type character varying(8)"
-- Solution: Increase prefix column size to accommodate longer prefixes
-- Date: 2025-10-22

ALTER TABLE api_keys
ALTER COLUMN prefix TYPE VARCHAR(16);

-- Add comment for clarity
COMMENT ON COLUMN api_keys.prefix IS 'First 16 characters of API key for display and identification purposes';
