-- Migration 118: storage for signed audit record chains.
--
-- An organization's audit records form one hash chain. Each record is a DSSE
-- envelope over the canonical bytes of its retained part, signed with the
-- deployment's record key; each names its chain, its sequence number and the
-- SHA-256 of its predecessor (internal/record). This migration adds the two
-- tables a writer appends to:
--
--   record_chains   one row per chain: its organization, the key its next
--                   record is signed with, and its head (the newest record's
--                   sequence number and hash). A writer takes the append lock
--                   by locking this row, so appends to one chain are ordered
--                   and appends to different chains never wait on each other.
--
--   audit_records   one row per record: its place in the chain, the signed
--                   envelope's payload and signature, and the two erasable
--                   parts with their salts. Erasing a part sets it and its
--                   salt to NULL and leaves the chain verifying.
--
-- Nothing removes a chain or a record by cascade. Removing an organization
-- that has a chain fails until the chain is removed by a path that records
-- the removal; a cascade would delete the evidence with no trace.
--
-- No existing table changes, no existing row is read or written, and no
-- writer appends to these tables yet. Idempotent: every object is created
-- only if absent.

CREATE TABLE IF NOT EXISTS record_chains (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE RESTRICT,
    key_id          TEXT NOT NULL CHECK (key_id ~ '^[0-9a-f]{64}$'),
    head_seq        BIGINT NOT NULL CHECK (head_seq >= 0),
    head_hash       TEXT NOT NULL CHECK (head_hash ~ '^[0-9a-f]{64}$'),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT record_chains_one_per_organization UNIQUE (organization_id)
);

COMMENT ON TABLE record_chains IS
    'One signed audit record chain per organization; locking a row is the append lock of its chain';
COMMENT ON COLUMN record_chains.key_id IS
    'Lowercase hex SHA-256 of the public key the chain''s next record is signed with';
COMMENT ON COLUMN record_chains.head_seq IS
    'Sequence number of the newest record of the chain';
COMMENT ON COLUMN record_chains.head_hash IS
    'Lowercase hex SHA-256 of the newest record''s canonical bytes';

CREATE TABLE IF NOT EXISTS audit_records (
    chain_id      UUID NOT NULL REFERENCES record_chains(id) ON DELETE RESTRICT,
    seq           BIGINT NOT NULL CHECK (seq >= 0),
    event_id      UUID NOT NULL,
    record_type   TEXT NOT NULL CHECK (record_type <> ''),
    recorded_at   TIMESTAMPTZ NOT NULL,
    record_hash   TEXT NOT NULL CHECK (record_hash ~ '^[0-9a-f]{64}$'),
    payload_type  TEXT NOT NULL
        CHECK (payload_type = 'application/vnd.opena2a.audit-record.v1+json'),
    payload       BYTEA NOT NULL CHECK (octet_length(payload) > 0),
    key_id        TEXT NOT NULL CHECK (key_id ~ '^[0-9a-f]{64}$'),
    signature     BYTEA NOT NULL CHECK (octet_length(signature) > 0),
    tenant_part   BYTEA,
    tenant_salt   BYTEA,
    personal_part BYTEA,
    personal_salt BYTEA,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (chain_id, seq),
    CONSTRAINT audit_records_event_unique_per_chain UNIQUE (chain_id, event_id),
    CONSTRAINT audit_records_hash_is_payload_digest
        CHECK (record_hash = encode(sha256(payload), 'hex')),
    CONSTRAINT audit_records_tenant_part_has_salt
        CHECK ((tenant_part IS NULL) = (tenant_salt IS NULL)
               AND (tenant_salt IS NULL OR octet_length(tenant_salt) = 32)),
    CONSTRAINT audit_records_personal_part_has_salt
        CHECK ((personal_part IS NULL) = (personal_salt IS NULL)
               AND (personal_salt IS NULL OR octet_length(personal_salt) = 32))
);

COMMENT ON TABLE audit_records IS
    'Signed audit records, one row per position of a record chain';
COMMENT ON COLUMN audit_records.recorded_at IS
    'The record''s own timestamp member, as signed; created_at is when the row was inserted';
COMMENT ON COLUMN audit_records.payload IS
    'Canonical bytes of the record''s retained part: the DSSE payload the signature covers';
COMMENT ON COLUMN audit_records.key_id IS
    'Key identifier of the envelope''s one signature';
COMMENT ON COLUMN audit_records.tenant_part IS
    'Erasable tenant part; NULL when absent or erased, together with tenant_salt';
COMMENT ON COLUMN audit_records.personal_part IS
    'Erasable personal part; NULL when absent or erased, together with personal_salt';
