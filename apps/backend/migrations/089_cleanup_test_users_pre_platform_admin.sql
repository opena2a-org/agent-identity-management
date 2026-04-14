-- Migration: Remove test users and their auto-created organization from the
-- pre-PR-A bring-up window (2026-04-13/14).
--
-- Context: Before PR-A, registration auto-approved the first user from any
-- new email domain as admin of an auto-created org. Two test users landed
-- via this path during cloud bring-up:
--   - deploy-test@opena2a.org (admin of auto-created opena2a.org org)
--   - sec-audit-2026-04-13@opena2a.org (pending registration request)
--
-- Both are test data with no real value. This migration removes them and the
-- auto-created opena2a.org org so the post-PR-A state starts clean.
--
-- Strategy: delete the org -- ON DELETE CASCADE on every org_id FK takes care
-- of agents, mcp_servers, users, audit logs, etc. For FKs not on org_id (e.g.
-- agents.created_by → users.id, no CASCADE), null them out first. The whole
-- thing runs in a single transaction; any failure rolls back cleanly.

BEGIN;

-- 1. Null out user FKs that don't cascade, scoped to the test org's users.
--    Use DO blocks so the migration stays idempotent if the org is already gone
--    or if column shapes vary across environments.
DO $$
DECLARE
    test_org_id UUID;
    test_user_ids UUID[];
BEGIN
    SELECT id INTO test_org_id FROM organizations WHERE domain = 'opena2a.org' LIMIT 1;
    IF test_org_id IS NULL THEN
        RAISE NOTICE 'No opena2a.org test org found; nothing to clean up.';
        RETURN;
    END IF;

    SELECT array_agg(id) INTO test_user_ids FROM users WHERE organization_id = test_org_id;

    -- Null out NOT NULL FKs we can't satisfy after delete (no CASCADE, no SET NULL).
    -- These columns are NOT NULL in some migrations, so we delete the rows
    -- rather than null them.
    IF test_user_ids IS NOT NULL THEN
        DELETE FROM agents WHERE created_by = ANY(test_user_ids);
        DELETE FROM mcp_servers WHERE created_by = ANY(test_user_ids);
        -- Also clear the standalone pending registration request not in the org.
        DELETE FROM user_registration_requests
            WHERE email IN ('deploy-test@opena2a.org', 'sec-audit-2026-04-13@opena2a.org');
    END IF;

    -- 2. Delete the org. CASCADE handles users, audit logs, agents in other
    --    orgs (none expected), and all other org-scoped child rows.
    DELETE FROM organizations WHERE id = test_org_id;

    RAISE NOTICE 'Cleaned up test org % and % users.', test_org_id, COALESCE(array_length(test_user_ids, 1), 0);
END $$;

-- Belt-and-braces: catch any standalone pending request that was outside the org.
DELETE FROM user_registration_requests
 WHERE email IN ('deploy-test@opena2a.org', 'sec-audit-2026-04-13@opena2a.org');

COMMIT;
