-- Migration 119: debts for records a reduction could not append.
--
-- A reduction (a state change that removes or narrows what an agent, key or
-- user may do) commits even when its audit record cannot be appended to the
-- organization's chain: refusing it would keep a privilege someone chose to
-- remove. It commits with one record_debts row instead, in the same
-- transaction, and the debt settler appends the record later and deletes the
-- row in the transaction that appends it. No settled row is kept.
--
-- A row is a pending copy of the record it stands for, not a record. The late
-- record is built from the row alone: every column becomes exactly one member
-- of it (internal/record/store, debtColumns), so nothing a record carries is
-- lost while it waits, and no column holds anything the record does not.
--
-- Columns, by the part of the record the member they become sits in:
--   retained  id (event_id), record_type (type), occurred_at, state_space,
--             trigger_type, admin_action, resource_type, and, for an
--             operator's act only, actor_class (actor), operator_subcommand,
--             operator_reason_code and build_commit
--   tenant    organization_id, subject_agent_id, verification_ref, trace_id,
--             parent_id, and resource_id, previous_state and new_state when
--             the resource type does not name a person
--   personal  actor, request_trace_id, reason, and resource_id,
--             previous_state and new_state when the resource type names a
--             person
--
-- Every statement that reads or writes this table names one organization.
-- Nothing removes a debt by cascade: an organization with open debts cannot
-- be removed until they are settled.
--
-- No existing table changes and no existing row is read or written.
-- Idempotent: every object is created only if absent.

CREATE TABLE IF NOT EXISTS record_debts (
    id                   UUID PRIMARY KEY,
    organization_id      UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    record_type          TEXT NOT NULL
        CHECK (record_type ~ '^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$'
               AND record_type <> 'opena2a.chain_genesis'),
    occurred_at          TIMESTAMPTZ NOT NULL,
    state_space          TEXT CHECK (state_space IN ('agent', 'verification', 'appraisal')),
    trigger_type         TEXT CHECK (trigger_type ~ '^[a-z][a-z0-9_]*$'),
    admin_action         TEXT CHECK (admin_action ~ '^[a-z][a-z0-9_]*$'),
    resource_type        TEXT CHECK (resource_type ~ '^[a-z][a-z0-9_]*$'),
    actor_class          TEXT CHECK (actor_class = 'operator_command'),
    operator_subcommand  TEXT CHECK (operator_subcommand ~ '^[a-z][a-z0-9_]*$'),
    operator_reason_code TEXT CHECK (operator_reason_code ~ '^[a-z][a-z0-9_]*$'),
    build_commit         TEXT CHECK (build_commit ~ '^[0-9a-f]{40}$' OR build_commit = 'unstamped'),
    subject_agent_id     UUID,
    verification_ref     TEXT,
    trace_id             TEXT,
    parent_id            TEXT,
    resource_id          TEXT,
    previous_state       JSONB,
    new_state            JSONB,
    actor                TEXT,
    request_trace_id     TEXT,
    reason               TEXT,
    CONSTRAINT record_debts_operator_members_together CHECK (
        (actor_class IS NULL AND operator_subcommand IS NULL
         AND operator_reason_code IS NULL AND build_commit IS NULL)
        OR (actor_class IS NOT NULL AND operator_subcommand IS NOT NULL
            AND operator_reason_code IS NOT NULL AND build_commit IS NOT NULL
            AND actor IS NULL))
);

CREATE INDEX IF NOT EXISTS record_debts_organization_occurred
    ON record_debts (organization_id, occurred_at, id);

COMMENT ON TABLE record_debts IS
    'Open debts: records of committed reductions not yet appended to their chain; a row is deleted when its record is appended';
COMMENT ON COLUMN record_debts.id IS
    'The late record''s event_id';
COMMENT ON COLUMN record_debts.occurred_at IS
    'When the reduction committed; the late record carries it as opena2a.occurred_at';
COMMENT ON COLUMN record_debts.actor_class IS
    'operator_command for an operator''s act, whose actor names no person; NULL otherwise, with the three operator columns';
