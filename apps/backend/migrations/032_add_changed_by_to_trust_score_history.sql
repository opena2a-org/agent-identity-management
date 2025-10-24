-- Migration: Add changed_by field to trust_score_history
-- Description: Track WHO changed the trust score for complete audit trail
-- Required for frontend Trust Score History UI
-- Created: 2025-10-22

-- Add changed_by column to track the user who triggered the change
ALTER TABLE trust_score_history
ADD COLUMN IF NOT EXISTS changed_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- Add index for querying by changed_by
CREATE INDEX IF NOT EXISTS idx_trust_score_history_changed_by
ON trust_score_history(changed_by)
WHERE changed_by IS NOT NULL;

-- Update the trigger function to capture the user who made the change
CREATE OR REPLACE FUNCTION log_trust_score_change()
RETURNS TRIGGER AS $$
DECLARE
    current_user_id UUID;
BEGIN
    IF NEW.trust_score IS DISTINCT FROM OLD.trust_score THEN
        -- Try to get current user from session context
        -- This will be NULL for automated system changes
        BEGIN
            current_user_id := current_setting('app.current_user_id', true)::UUID;
        EXCEPTION WHEN OTHERS THEN
            current_user_id := NULL;
        END;

        INSERT INTO trust_score_history (
            agent_id,
            organization_id,
            trust_score,
            previous_score,
            change_reason,
            changed_by
        )
        VALUES (
            NEW.id,
            NEW.organization_id,
            NEW.trust_score,
            OLD.trust_score,
            'automated_update',
            current_user_id  -- Will be NULL for system/automated changes
        );
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Comment for documentation
COMMENT ON COLUMN trust_score_history.changed_by IS 'User who triggered the trust score change (NULL for automated system changes)';
