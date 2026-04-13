-- Migration: Add missing reason and updated_at columns to verification_events
-- The expiration cleanup job references these columns but they were never added
-- to the original schema (005_create_verification_events_table.sql)

ALTER TABLE verification_events ADD COLUMN IF NOT EXISTS reason TEXT;
ALTER TABLE verification_events ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

COMMENT ON COLUMN verification_events.reason IS 'Reason for the verification result (e.g., expiration reason)';
COMMENT ON COLUMN verification_events.updated_at IS 'Timestamp of the last update to this verification event';
