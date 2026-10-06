-- Migration 120: an agent's A2A task, message and consent rows are deleted with it.
--
-- Migration 066 created five references to agents(id) with no ON DELETE rule:
-- a2a_tasks.client_agent_id and remote_agent_id, a2a_messages.sender_agent_id,
-- and a2a_consent_records.grantor_agent_id and recipient_agent_id. Every other
-- per-agent A2A table cascades. With the default rule, DELETE FROM agents fails
-- with a foreign-key violation for any agent that has one such row, so the
-- agent could not be deleted at all. These five now cascade.
--
-- Cascading a2a_tasks reaches one more reference with no rule:
-- a2a_agent_attestations.task_id (migration 068). An attestation between two
-- agents that still exist is kept; it only loses the link to the deleted task,
-- as a2a_call_chain.task_id already does (migration 070). task_id is nullable
-- and names a task, not a person.
--
-- The existing constraints are found by table, column and referenced table
-- rather than by name, so a deployment whose constraint names differ is still
-- converged. Re-running this migration drops and re-adds the same constraints.

DO $$
DECLARE
    fk record;
BEGIN
    FOR fk IN
        SELECT c.conrelid::regclass AS tbl, c.conname
        FROM pg_constraint c
        JOIN pg_attribute a
          ON a.attrelid = c.conrelid
         AND a.attnum = c.conkey[1]
        WHERE c.contype = 'f'
          AND cardinality(c.conkey) = 1
          AND (c.conrelid, a.attname::text, c.confrelid) IN (
              ('a2a_tasks'::regclass,              'client_agent_id',    'agents'::regclass),
              ('a2a_tasks'::regclass,              'remote_agent_id',    'agents'::regclass),
              ('a2a_messages'::regclass,           'sender_agent_id',    'agents'::regclass),
              ('a2a_consent_records'::regclass,    'grantor_agent_id',   'agents'::regclass),
              ('a2a_consent_records'::regclass,    'recipient_agent_id', 'agents'::regclass),
              ('a2a_agent_attestations'::regclass, 'task_id',            'a2a_tasks'::regclass)
          )
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', fk.tbl, fk.conname);
    END LOOP;
END
$$;

ALTER TABLE a2a_tasks
    ADD CONSTRAINT a2a_tasks_client_agent_id_fkey
        FOREIGN KEY (client_agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    ADD CONSTRAINT a2a_tasks_remote_agent_id_fkey
        FOREIGN KEY (remote_agent_id) REFERENCES agents(id) ON DELETE CASCADE;

ALTER TABLE a2a_messages
    ADD CONSTRAINT a2a_messages_sender_agent_id_fkey
        FOREIGN KEY (sender_agent_id) REFERENCES agents(id) ON DELETE CASCADE;

ALTER TABLE a2a_consent_records
    ADD CONSTRAINT a2a_consent_records_grantor_agent_id_fkey
        FOREIGN KEY (grantor_agent_id) REFERENCES agents(id) ON DELETE CASCADE,
    ADD CONSTRAINT a2a_consent_records_recipient_agent_id_fkey
        FOREIGN KEY (recipient_agent_id) REFERENCES agents(id) ON DELETE CASCADE;

ALTER TABLE a2a_agent_attestations
    ADD CONSTRAINT a2a_agent_attestations_task_id_fkey
        FOREIGN KEY (task_id) REFERENCES a2a_tasks(id) ON DELETE SET NULL;
