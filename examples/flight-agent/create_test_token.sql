-- Create a fresh SDK token for testing
-- This simulates what happens when user downloads SDK from portal

DO $$
DECLARE
    v_token_id TEXT;
    v_user_id UUID := 'a533002a-a268-4aab-a484-c97a76da6ea4';
    v_org_id UUID;
    v_refresh_token TEXT;
    v_token_hash TEXT;
BEGIN
    -- Get organization ID
    SELECT organization_id INTO v_org_id
    FROM users
    WHERE id = v_user_id;

    -- Generate new token ID
    v_token_id := gen_random_uuid()::TEXT;

    -- Create a test refresh token (in production, this comes from OAuth flow)
    v_refresh_token := 'test_refresh_token_' || v_token_id;

    -- Hash the token (SHA-256)
    v_token_hash := encode(digest(v_refresh_token, 'sha256'), 'hex');

    -- Insert new SDK token
    INSERT INTO sdk_tokens (
        id,
        user_id,
        organization_id,
        token_hash,
        token_id,
        device_name,
        expires_at,
        created_at
    ) VALUES (
        gen_random_uuid(),
        v_user_id,
        v_org_id,
        v_token_hash,
        v_token_id,
        'Flight Agent - Test Token',
        NOW() + INTERVAL '90 days',
        NOW()
    );

    -- Output the token ID for reference
    RAISE NOTICE 'Created SDK token: %', v_token_id;
    RAISE NOTICE 'Token hash: %', substring(v_token_hash, 1, 30);
END $$;

-- Verify token was created
SELECT
    id,
    token_id,
    device_name,
    revoked_at IS NULL as is_active,
    expires_at,
    created_at
FROM sdk_tokens
WHERE device_name = 'Flight Agent - Test Token'
ORDER BY created_at DESC
LIMIT 1;
