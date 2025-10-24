-- Simple approval script for devtesting934@gmail.com
-- This will create the user account using existing table structure

-- Step 1: Create or find organization for gmail.com domain
INSERT INTO organizations (id, name, domain, plan_type, max_agents, max_users, is_active, created_at, updated_at)
VALUES (
    gen_random_uuid(),
    'gmail.com',
    'gmail.com',
    'free',
    100,
    10,
    true,
    NOW(),
    NOW()
) ON CONFLICT (domain) DO NOTHING;

-- Step 2: Create user account from the registration request
WITH org AS (
    SELECT id as org_id FROM organizations WHERE domain = 'gmail.com' LIMIT 1
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
    oauth_provider,
    oauth_user_id,
    created_at,
    updated_at
)
SELECT 
    gen_random_uuid(),
    org.org_id,
    'devtesting934@gmail.com',
    'Testing Dev',
    CASE WHEN user_count.count = 0 THEN 'admin' ELSE 'viewer' END,
    'google',
    '101319206384440375246',
    true,
    'https://lh3.googleusercontent.com/a/ACg8ocKA4rAejUpVcrZljWwfeSHNnYll9RQVDNFmMuIYXK2ElPCmGA=s96-c',
    'google',
    '101319206384440375246',
    NOW(),
    NOW()
FROM org, user_count
WHERE NOT EXISTS (
    SELECT 1 FROM users WHERE email = 'devtesting934@gmail.com'
);

-- Step 3: Mark registration request as approved
UPDATE user_registration_requests 
SET 
    status = 'approved',
    reviewed_at = NOW(),
    updated_at = NOW()
WHERE id = 'f3e79cff-d049-446a-ae60-ad8e99b4caf7';

-- Step 4: Verify the user was created
SELECT 
    u.id,
    u.email,
    u.name,
    u.role,
    o.name as organization,
    'User created successfully!' as message
FROM users u
JOIN organizations o ON u.organization_id = o.id
WHERE u.email = 'devtesting934@gmail.com';
