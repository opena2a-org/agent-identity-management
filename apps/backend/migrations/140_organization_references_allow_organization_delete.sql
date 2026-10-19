-- Migration 140: an organization can be deleted with the rows that name it.
--
-- The account deletion purge deletes an organization once it has deleted the
-- organization's last user (UserRepository.PurgeDeletionRequests). Three
-- references to organizations(id) had no ON DELETE rule, so deleting an
-- organization named in a row of theirs failed with a foreign-key violation:
--
--   a2a_consent_records.organization_id             consents to data sharing
--                                                   between agents, recorded in
--                                                   the organization, each with
--                                                   the identifier of the user
--                                                   who gave it
--   community_intelligence_reports.organization_id  the organization's record of
--                                                   the reports it sent
--   device_codes.organization_id                    the organization of an
--                                                   approved CLI sign-in
--
-- Each now has ON DELETE CASCADE: the rows are the organization's, and go with
-- it as its agents, MCP servers and audit log rows already do.
--
-- record_chains.organization_id and record_debts.organization_id keep ON DELETE
-- RESTRICT (migrations 118 and 119): a signed audit record chain is not removed
-- by cascade, so an organization that has one is not deleted.
--
-- The existing constraints are found by table, column and referenced table
-- rather than by name, as in migration 135. Re-running this migration drops and
-- re-adds the same constraints.

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
          AND c.confrelid = 'organizations'::regclass
          AND (c.conrelid, a.attname::text) IN (
              ('a2a_consent_records'::regclass,            'organization_id'),
              ('community_intelligence_reports'::regclass, 'organization_id'),
              ('device_codes'::regclass,                   'organization_id')
          )
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', fk.tbl, fk.conname);
    END LOOP;
END
$$;

ALTER TABLE a2a_consent_records
    ADD CONSTRAINT a2a_consent_records_organization_id_fkey
        FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE community_intelligence_reports
    ADD CONSTRAINT community_intelligence_reports_organization_id_fkey
        FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE device_codes
    ADD CONSTRAINT device_codes_organization_id_fkey
        FOREIGN KEY (organization_id) REFERENCES organizations(id) ON DELETE CASCADE;
