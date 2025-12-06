-- Migration: Add SDK execution status fields to verification_events table
-- This allows the SDK to report whether a function was actually executed after verification

-- Add execution status columns
ALTER TABLE verification_events ADD COLUMN IF NOT EXISTS executed BOOLEAN;
ALTER TABLE verification_events ADD COLUMN IF NOT EXISTS strict_mode BOOLEAN;
ALTER TABLE verification_events ADD COLUMN IF NOT EXISTS executed_at TIMESTAMPTZ;
ALTER TABLE verification_events ADD COLUMN IF NOT EXISTS execution_error TEXT;

-- Add index for querying by execution status
CREATE INDEX IF NOT EXISTS idx_verification_events_executed ON verification_events(executed) WHERE executed IS NOT NULL;

-- Add comment explaining the columns
COMMENT ON COLUMN verification_events.executed IS 'Whether the decorated function was actually executed by the SDK';
COMMENT ON COLUMN verification_events.strict_mode IS 'Whether the SDK was in strict mode when verification was requested';
COMMENT ON COLUMN verification_events.executed_at IS 'Timestamp when the function execution completed';
COMMENT ON COLUMN verification_events.execution_error IS 'Error message if execution failed';
