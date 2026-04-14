-- Migration: Remove test users created during AIM Cloud bring-up before
-- platform-admin allowlist was enforced (PR-A).
--
-- Context: Prior to this PR, the registration service auto-approved the first
-- user from any new email domain as admin of an auto-created org. During cloud
-- bring-up testing on 2026-04-13/14, two test users were created via this path:
--   - deploy-test@opena2a.org (admin of auto-created opena2a.org org)
--   - sec-audit-2026-04-13@opena2a.org (pending registration request)
--
-- Both are test data with no real value. Removing them so the post-PR-A state
-- starts clean: no users exist until info@opena2a.org registers as platform
-- admin via the new AIM_PLATFORM_ADMINS allowlist.
--
-- Cascading deletes via FK constraints will clean up: agents, api_keys,
-- audit logs scoped to those users/orgs.

DELETE FROM user_registration_requests
 WHERE email IN ('deploy-test@opena2a.org', 'sec-audit-2026-04-13@opena2a.org');

DELETE FROM users
 WHERE email IN ('deploy-test@opena2a.org', 'sec-audit-2026-04-13@opena2a.org');

-- Remove the auto-created opena2a.org organization if it has no remaining users.
-- Safe-by-construction: this only matches if the test-user delete above emptied it.
DELETE FROM organizations
 WHERE domain = 'opena2a.org'
   AND NOT EXISTS (SELECT 1 FROM users WHERE organization_id = organizations.id);
