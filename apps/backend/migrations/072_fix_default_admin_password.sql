-- Legacy migration — preserved for migration-history integrity, no-op for B2+ deploys.
--
-- Original purpose (pre-2026-05-20): correct the unknown-password bcrypt hash
-- that migration 013 originally seeded, replacing it with the hash for the
-- documented default `AIM2025!Secure`.
--
-- Post-B2 (2026-05-20): migration 013 no longer seeds the admin user at all.
-- The admin user is now created by `aim-bootstrap --default` with a randomly
-- generated password. The UPDATE below targets `password_hash = '$2b$12$EYt...'`
-- which is no longer written by any seed path, so this migration is a no-op
-- for fresh deploys.
--
-- For existing deploys that ran this migration before B2 landed, the UPDATE
-- already executed and the row's password is `AIM2025!Secure` (which the
-- operator was forced to change via force_password_change=TRUE). Those deploys
-- are unaffected by re-running this migration (UPDATE matches zero rows).

UPDATE users
SET password_hash = '$2a$12$UbRtBE0U9Ry36Bdl04YWDuXe3lIw14aZaxQ8B6bbA4P7peLRski66'
WHERE email = 'admin@opena2a.org'
  AND role = 'admin'
  AND password_hash = '$2b$12$EYtPexZSmNuHT/bzVhoLWOjNM9ZvdPbclV/f7KQx9otOde07.0WXG';
