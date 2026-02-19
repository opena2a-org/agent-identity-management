-- Fix: Update default admin password hash to match documented default password
-- The previous hash in migration 013 was for an unknown password, making the
-- admin account inaccessible without ADMIN_PASSWORD env var.
-- New hash corresponds to: AIM2025!Secure (bcrypt cost=12)
-- Admin is still forced to change password on first login (force_password_change=TRUE)

UPDATE users
SET password_hash = '$2a$12$UbRtBE0U9Ry36Bdl04YWDuXe3lIw14aZaxQ8B6bbA4P7peLRski66'
WHERE email = 'admin@opena2a.org'
  AND role = 'admin'
  AND password_hash = '$2b$12$EYtPexZSmNuHT/bzVhoLWOjNM9ZvdPbclV/f7KQx9otOde07.0WXG';
