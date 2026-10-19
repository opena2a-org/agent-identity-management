-- Migration 135: a user named in another row can be deleted, and the rows the
-- organization keeps stay when the user is deleted.
--
-- Eighteen references to users(id) had no ON DELETE rule (sixteen) or ON DELETE
-- RESTRICT (webhooks.created_by and capability_requests.requested_by). DELETE FROM
-- users failed with a foreign-key violation for a user named in any of them, such as
-- the user who created an agent, an API key or an MCP server, so that user could not
-- be deleted. Two more cascaded, security_policies.created_by and tags.created_by:
-- deleting a user deleted the security policies and the tags the user had created,
-- and with a tag its links to agents and MCP servers.
--
-- Two of the eighteen now cascade. Each is a credential that acts as its user, and
-- goes with the user as sdk_tokens.user_id and bootstrap_tokens.created_by already do:
--
--   api_keys.created_by    the API key middleware sets the request's user_id to it,
--                          and AuthService.ValidateAPIKey reads it as the key's owner
--   device_codes.user_id   the user who approved a CLI sign-in; polling an approved
--                          code mints a token pair for that user
--
-- The other sixteen, security_policies.created_by and tags.created_by are set to NULL.
-- Each names who created, changed, collected, requested, approved, reviewed,
-- acknowledged, granted, revoked or unlocked a row the organization keeps; a cascade
-- deletes that row with the user, and for a2a_revoked_agents.revoked_by it would
-- delete a revocation. agents.created_by, mcp_servers.created_by,
-- webhooks.created_by, compliance_evidence.collected_by,
-- capability_requests.requested_by, security_policies.created_by and tags.created_by
-- were NOT NULL and now accept NULL. Setting a reference to NULL updates its row, so
-- the row's BEFORE UPDATE triggers run: an agent's updated_at becomes the time of the
-- delete.
--
-- The thirteen other references to users(id) keep their rule.
--
-- The existing constraints are found by table, column and referenced table rather
-- than by name, as in migration 120. Re-running this migration drops and re-adds the
-- same constraints.

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
          AND c.confrelid = 'users'::regclass
          AND (c.conrelid, a.attname::text) IN (
              ('api_keys'::regclass,                   'created_by'),
              ('device_codes'::regclass,               'user_id'),
              ('users'::regclass,                      'approved_by'),
              ('agents'::regclass,                     'created_by'),
              ('mcp_servers'::regclass,                'created_by'),
              ('webhooks'::regclass,                   'created_by'),
              ('compliance_evidence'::regclass,        'collected_by'),
              ('capability_requests'::regclass,        'requested_by'),
              ('alerts'::regclass,                     'acknowledged_by'),
              ('user_registration_requests'::regclass, 'reviewed_by'),
              ('agent_capabilities'::regclass,         'granted_by'),
              ('auth_lockouts'::regclass,              'unlocked_by'),
              ('a2a_policies'::regclass,               'created_by'),
              ('a2a_policies'::regclass,               'updated_by'),
              ('a2a_revoked_agents'::regclass,         'revoked_by'),
              ('a2a_security_settings'::regclass,      'created_by'),
              ('a2a_security_settings'::regclass,      'updated_by'),
              ('a2a_delegation'::regclass,             'created_by'),
              ('security_policies'::regclass,          'created_by'),
              ('tags'::regclass,                       'created_by')
          )
    LOOP
        EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %I', fk.tbl, fk.conname);
    END LOOP;
END
$$;

-- Credentials that act as their user: deleted with the user.

ALTER TABLE api_keys
    ADD CONSTRAINT api_keys_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE device_codes
    ADD CONSTRAINT device_codes_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Rows the organization keeps: the reference to the user is set to NULL.

ALTER TABLE agents
    ALTER COLUMN created_by DROP NOT NULL,
    ADD CONSTRAINT agents_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE mcp_servers
    ALTER COLUMN created_by DROP NOT NULL,
    ADD CONSTRAINT mcp_servers_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE webhooks
    ALTER COLUMN created_by DROP NOT NULL,
    ADD CONSTRAINT webhooks_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE compliance_evidence
    ALTER COLUMN collected_by DROP NOT NULL,
    ADD CONSTRAINT compliance_evidence_collected_by_fkey
        FOREIGN KEY (collected_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE capability_requests
    ALTER COLUMN requested_by DROP NOT NULL,
    ADD CONSTRAINT capability_requests_requested_by_fkey
        FOREIGN KEY (requested_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE users
    ADD CONSTRAINT users_approved_by_fkey
        FOREIGN KEY (approved_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE alerts
    ADD CONSTRAINT alerts_acknowledged_by_fkey
        FOREIGN KEY (acknowledged_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE user_registration_requests
    ADD CONSTRAINT user_registration_requests_reviewed_by_fkey
        FOREIGN KEY (reviewed_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE agent_capabilities
    ADD CONSTRAINT agent_capabilities_granted_by_fkey
        FOREIGN KEY (granted_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE auth_lockouts
    ADD CONSTRAINT auth_lockouts_unlocked_by_fkey
        FOREIGN KEY (unlocked_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE a2a_policies
    ADD CONSTRAINT a2a_policies_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    ADD CONSTRAINT a2a_policies_updated_by_fkey
        FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE a2a_revoked_agents
    ADD CONSTRAINT a2a_revoked_agents_revoked_by_fkey
        FOREIGN KEY (revoked_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE a2a_security_settings
    ADD CONSTRAINT a2a_security_settings_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL,
    ADD CONSTRAINT a2a_security_settings_updated_by_fkey
        FOREIGN KEY (updated_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE a2a_delegation
    ADD CONSTRAINT a2a_delegation_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE security_policies
    ALTER COLUMN created_by DROP NOT NULL,
    ADD CONSTRAINT security_policies_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;

ALTER TABLE tags
    ALTER COLUMN created_by DROP NOT NULL,
    ADD CONSTRAINT tags_created_by_fkey
        FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL;
