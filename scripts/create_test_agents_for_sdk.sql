-- Create test agents for SDK capability detection testing
-- These agents are for local testing only

-- First, get the organization ID of the admin user
DO $$
DECLARE
    test_org_id UUID;
    admin_user_id UUID;
    go_agent_id UUID;
    js_agent_id UUID;
    go_api_key_id UUID;
    js_api_key_id UUID;
    go_public_key TEXT := 'fOQu+6FZXHXV5yZ0TYjN7wKHxQvZGBzX2xhE3KjY2sU=';
    go_private_key TEXT := 'MIGEAgEAMBAGByqGSM49AgEGBSuBBAAKBG0wawIBAQQgtestkeyforgotestagent1234567890abcdefghijklmn';
    js_public_key TEXT := 'iK4+LnR8pXhJ9zA1b3MvOxWqTcUfVnBmA7iN/jPsQdE=';
    js_private_key TEXT := 'MIGEAgEAMBAGByqGSM49AgEGBSuBBAAKBG0wawIBAQQgtestkeyforgotestagent0987654321zyxwvutsrqponm';
BEGIN
    -- Get admin user and their organization
    SELECT id, organization_id INTO admin_user_id, test_org_id
    FROM users WHERE email = 'admin@aim.test' LIMIT 1;

    IF test_org_id IS NULL THEN
        RAISE EXCEPTION 'Admin user not found or organization_id is null';
    END IF;

    -- Delete existing test agents if they exist
    DELETE FROM agents WHERE name IN ('go-sdk-test-agent', 'javascript-sdk-test-agent');

    -- Insert Go SDK test agent
    INSERT INTO agents (
        id, organization_id, name, display_name, agent_type,
        description, version, status, public_key, created_by,
        created_at, updated_at
    ) VALUES (
        uuid_generate_v4(), test_org_id, 'go-sdk-test-agent',
        'Go SDK Test Agent', 'ai_agent',
        'Test agent for Go SDK capability detection validation',
        '1.0.0', 'verified', go_public_key, admin_user_id,
        NOW(), NOW()
    ) RETURNING id INTO go_agent_id;

    -- Insert JavaScript SDK test agent
    INSERT INTO agents (
        id, organization_id, name, display_name, agent_type,
        description, version, status, public_key, created_by,
        created_at, updated_at
    ) VALUES (
        uuid_generate_v4(), test_org_id, 'javascript-sdk-test-agent',
        'JavaScript SDK Test Agent', 'ai_agent',
        'Test agent for JavaScript SDK capability detection validation',
        '1.0.0', 'verified', js_public_key, admin_user_id,
        NOW(), NOW()
    ) RETURNING id INTO js_agent_id;

    -- Create API keys for Go SDK test agent
    INSERT INTO api_keys (
        id, organization_id, agent_id, name, key_hash, prefix,
        is_active, created_by, created_at, expires_at
    ) VALUES (
        uuid_generate_v4(), test_org_id, go_agent_id,
        'Go SDK Test Key',
        encode(digest('aim_test_go_sdk_key_12345', 'sha256'), 'hex'),
        'aim_go_test',
        true, admin_user_id, NOW(), NOW() + INTERVAL '30 days'
    ) RETURNING id INTO go_api_key_id;

    -- Create API keys for JavaScript SDK test agent
    INSERT INTO api_keys (
        id, organization_id, agent_id, name, key_hash, prefix,
        is_active, created_by, created_at, expires_at
    ) VALUES (
        uuid_generate_v4(), test_org_id, js_agent_id,
        'JavaScript SDK Test Key',
        encode(digest('aim_test_js_sdk_key_67890', 'sha256'), 'hex'),
        'aim_js_test',
        true, admin_user_id, NOW(), NOW() + INTERVAL '30 days'
    ) RETURNING id INTO js_api_key_id;

    -- Store agent IDs and private keys in system_config for easy retrieval
    INSERT INTO system_config (key, value) VALUES
        ('go_sdk_test_agent_id', go_agent_id::TEXT),
        ('go_sdk_test_api_key', 'aim_test_go_sdk_key_12345'),
        ('go_sdk_test_private_key', go_private_key),
        ('js_sdk_test_agent_id', js_agent_id::TEXT),
        ('js_sdk_test_api_key', 'aim_test_js_sdk_key_67890'),
        ('js_sdk_test_private_key', js_private_key)
    ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();

    RAISE NOTICE 'Created test agents successfully!';
    RAISE NOTICE 'Go SDK Test Agent ID: %', go_agent_id;
    RAISE NOTICE 'Go SDK API Key: aim_test_go_sdk_key_12345';
    RAISE NOTICE 'JavaScript SDK Test Agent ID: %', js_agent_id;
    RAISE NOTICE 'JavaScript SDK API Key: aim_test_js_sdk_key_67890';
END $$;
