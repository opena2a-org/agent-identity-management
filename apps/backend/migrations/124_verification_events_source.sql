-- Migration 124: record who stands behind a verification event's outcome.
--
-- Until now no column told a verification AIM performed apart from an outcome
-- someone reported to it. The agent's own report of an action's outcome
-- (POST /agents/:id/log-capability/:audit_id) was written with the agent's
-- `success` as the row's status, and the trust readers counted it the same as
-- a check the server ran.
--
-- `source` is set by the server, never from a request body:
--   service          the server ran the check and set the outcome
--   system           a server-internal process recorded the event
--   caller_reported  an API caller's claim about the agent
--   agent_reported   the agent's own report about itself
--
-- Trust scoring reads only an allowlist of observed sources (service, system).
-- A row written before this column existed has source NULL and keeps counting
-- as it did; a NULL row created after the column exists is a write defect and
-- counts nothing. The instant that separates the two is recorded once, below,
-- so the readers do not depend on this file's name or number.
--
-- No CHECK on the values: an unknown source is stored and counted as not
-- observed, which is the reader's default. Idempotent.

ALTER TABLE verification_events ADD COLUMN IF NOT EXISTS source TEXT;

COMMENT ON COLUMN verification_events.source IS
    'Who stands behind the outcome: service, system, caller_reported or agent_reported. '
    'Set by the server. NULL only on rows written before the column existed.';

CREATE TABLE IF NOT EXISTS verification_event_source_cutover (
    singleton       BOOLEAN     PRIMARY KEY DEFAULT TRUE CHECK (singleton),
    source_added_at TIMESTAMPTZ NOT NULL
);

INSERT INTO verification_event_source_cutover (singleton, source_added_at)
VALUES (TRUE, now())
ON CONFLICT (singleton) DO NOTHING;

COMMENT ON TABLE verification_event_source_cutover IS
    'One row: when verification_events.source was added. A NULL-source event created before '
    'this instant predates the column; one created after it is a write defect.';
