-- Restore foreign key constraint (this will fail if there are verification events
-- for non-existent agents, which is expected behavior)
ALTER TABLE verification_events ADD CONSTRAINT verification_events_agent_id_fkey
    FOREIGN KEY (agent_id) REFERENCES agents(id) ON DELETE CASCADE;

-- Drop the index since FK constraint creates its own index
DROP INDEX IF EXISTS idx_verification_events_agent_id;
