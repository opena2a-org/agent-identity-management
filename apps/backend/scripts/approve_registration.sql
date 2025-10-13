-- Script to approve a pending OAuth registration request
-- Usage: Replace 'your-email@example.com' with your actual email address

-- Step 1: Check pending registration requests
SELECT 
    id,
    email,
    first_name,
    last_name,
    oauth_provider,
    status,
    requested_at
FROM user_registration_requests 
WHERE email = 'your-email@example.com' AND status = 'pending'
ORDER BY requested_at DESC;

-- Step 2: Find or create organization (replace the email domain)
INSERT INTO organizations (id, name, domain, plan_type, max_agents, max_users, is_active, auto_approve_sso, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    'example.com', -- Replace with your email domain
    'example.com', -- Replace with your email domain  
    'free',
    100,
    10,
    true,
    true,
    NOW(),
    NOW()
) ON CONFLICT (domain) DO NOTHING;

-- Step 3: Create user from registration request
WITH reg_request AS (
    SELECT * FROM user_registration_requests 
    WHERE email = 'your-email@example.com' AND status = 'pending'
    ORDER BY requested_at DESC
    LIMIT 1
),
org AS (
    SELECT id as org_id FROM organizations 
    WHERE domain = 'example.com' -- Replace with your email domain
    LIMIT 1
),
user_count AS (
    SELECT COUNT(*) as count FROM users u, org o WHERE u.organization_id = o.org_id
)
INSERT INTO users (
    id,
    organization_id,
    email,
    name,
    role,
    provider,
    provider_id,
    email_verified,
    avatar_url,
    status,
    created_at,
    updated_at
)
SELECT 
    gen_random_uuid(),
    org.org_id,
    reg_request.email,
    COALESCE(
        NULLIF(CONCAT(reg_request.first_name, ' ', reg_request.last_name), ' '),
        NULLIF(reg_request.first_name, ''),
        reg_request.email
    ),
    CASE WHEN user_count.count = 0 THEN 'admin' ELSE 'viewer' END,
    reg_request.oauth_provider,
    reg_request.oauth_user_id,
    reg_request.oauth_email_verified,
    reg_request.profile_picture_url,
    'active',
    NOW(),
    NOW()
FROM reg_request, org, user_count;

-- Step 4: Mark registration request as approved
UPDATE user_registration_requests 
SET 
    status = 'approved',
    reviewed_at = NOW(),
    updated_at = NOW()
WHERE email = 'your-email@example.com' AND status = 'pending';

-- Step 5: Verify the user was created
SELECT 
    u.id,
    u.email,
    u.name,
    u.role,
    u.status,
    o.name as organization
FROM users u
JOIN organizations o ON u.organization_id = o.id
WHERE u.email = 'your-email@example.com';
