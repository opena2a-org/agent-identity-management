-- Migration 133: credential revocation requests the Registry has not answered.
--
-- The Registry signs the Agent Trust Credentials POST /api/v1/agents/:id/atc
-- issues. When an agent is revoked, the server asks the Registry to revoke
-- them with POST /internal/atc/<agent id>/revoke. A row is one such request.
-- It is stored once the revocation is stored and before the first request is
-- sent, and it is deleted when the Registry answers 2xx or when the attempts
-- end. A request still waiting when a server stops stays here, and the next
-- server to start, or another replica, sends it.
--
-- agent_id is the agent the request names. It has no foreign key: a row
-- stays when the agent's row is deleted, as the Registry still holds the
-- credentials it issued for the agent.
--
-- request_id is replaced each time a request is stored again for the same
-- agent, as issuance does when the agent was revoked while the Registry was
-- issuing a credential. An answer to the earlier request then leaves the row,
-- and the later request is still sent. requested_at is when request_id was
-- stored.
--
-- attempts counts the requests started for request_id. next_attempt_at is
-- when the next one is due, on the database clock. A server that starts a
-- request first moves next_attempt_at past the request's timeout, so no other
-- server starts it while it is in flight, and a request whose server stopped
-- during it is due again once that time has passed.
--
-- No request reads or writes this table on behalf of an organization: the
-- revoke path stores a row, and the server's sender reads every row.
--
-- No existing table changes and no existing row is read or written.
-- Idempotent: every object is created only if absent.

CREATE TABLE IF NOT EXISTS registry_atc_revocations (
    agent_id        UUID        PRIMARY KEY,
    request_id      UUID        NOT NULL,
    requested_at    TIMESTAMPTZ NOT NULL,
    attempts        INTEGER     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_registry_atc_revocations_next_attempt_at
    ON registry_atc_revocations (next_attempt_at);

COMMENT ON TABLE registry_atc_revocations IS
    'Credential revocation requests for the Registry not yet answered 2xx; a row is deleted when the Registry revokes the credentials or the attempts end';
COMMENT ON COLUMN registry_atc_revocations.request_id IS
    'Replaced when the request is stored again for the agent; an answer to an earlier request_id leaves the row';
COMMENT ON COLUMN registry_atc_revocations.attempts IS
    'Requests started for request_id';
COMMENT ON COLUMN registry_atc_revocations.next_attempt_at IS
    'When the next request is due, on the database clock; moved past the request timeout while a request is in flight';
