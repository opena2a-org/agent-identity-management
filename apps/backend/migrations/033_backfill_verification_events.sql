-- Migration: Backfill verification events for existing verified agents
-- Description: Creates verification events for all existing verified agents to populate the dashboard chart
-- This is a one-time migration to fix the "No Activity Data" issue

-- Insert verification events for all verified agents
INSERT INTO verification_events (
    id,
    organization_id,
    agent_id,
    protocol,
    verification_type,
    status,
    result,
    confidence,
    duration_ms,
    initiator_type,
    started_at,
    completed_at,
    created_at
)
SELECT
    gen_random_uuid(),                          -- Generate new UUID for event
    a.organization_id,                          -- Organization from agent
    a.id,                                       -- Agent ID
    'A2A',                                      -- Protocol (A2A for Ed25519)
    'identity',                                 -- Verification type
    'success',                                  -- Status (success for verified agents)
    'verified',                                 -- Result
    1.0,                                        -- Confidence (100% for auto-verified)
    0,                                          -- Duration (instant)
    'system',                                   -- Initiator (system for auto-verification)
    a.created_at,                               -- Use agent creation time as verification time
    a.created_at,                               -- Completed at same time
    a.created_at                                -- Created at
FROM agents a
WHERE a.status = 'verified'
    AND NOT EXISTS (
        -- Don't create duplicates if event already exists for this agent
        SELECT 1 FROM verification_events ve
        WHERE ve.agent_id = a.id
    );

-- Verify the results
DO $$
DECLARE
    event_count INTEGER;
BEGIN
    SELECT COUNT(*) INTO event_count FROM verification_events;
    RAISE NOTICE '✅ Backfill complete. Total verification events: %', event_count;
END $$;
