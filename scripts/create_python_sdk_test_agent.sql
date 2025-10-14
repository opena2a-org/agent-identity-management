-- Create Python SDK Test Agent
-- This script creates a test agent for Python SDK validation with proper keys

-- First, let's check if the agent already exists and delete it if needed
DELETE FROM agent_capabilities WHERE agent_id IN (
    SELECT id FROM agents WHERE name = 'python-sdk-test-agent'
);

DELETE FROM agent_connections WHERE agent_id IN (
    SELECT id FROM agents WHERE name = 'python-sdk-test-agent'
);

DELETE FROM api_keys WHERE agent_id IN (
    SELECT id FROM agents WHERE name = 'python-sdk-test-agent'
);

DELETE FROM agents WHERE name = 'python-sdk-test-agent';

-- Get the default organization and user
-- Replace these with actual values from your database
DO $$
DECLARE
    v_org_id UUID;
    v_user_id UUID;
    v_agent_id UUID;
    v_api_key_id UUID;
    v_public_key TEXT;
    v_api_key_hash TEXT;
BEGIN
    -- Get first organization (you may need to adjust this)
    SELECT id INTO v_org_id FROM organizations LIMIT 1;

    -- Get first user from that organization
    SELECT id INTO v_user_id FROM users WHERE organization_id = v_org_id LIMIT 1;

    -- Generate a new UUID for the agent
    v_agent_id := gen_random_uuid();

    -- Generate Ed25519 public key (this is a test key - replace with actual generated key)
    -- For now, using a placeholder - will be updated by Python script
    v_public_key := 'PLACEHOLDER_WILL_BE_REPLACED';

    -- Create the agent
    INSERT INTO agents (
        id,
        organization_id,
        name,
        display_name,
        description,
        agent_type,
        version,
        public_key,
        status,
        created_by,
        created_at,
        updated_at,
        is_active,
        trust_score
    ) VALUES (
        v_agent_id,
        v_org_id,
        'python-sdk-test-agent',
        'Python SDK Test Agent',
        'Test agent for Python SDK validation and capability detection',
        'ai_agent',
        '1.0.0',
        v_public_key,
        'pending',
        v_user_id,
        NOW(),
        NOW(),
        true,
        0.0
    );

    -- Generate API key ID
    v_api_key_id := gen_random_uuid();

    -- Create a test API key (hash of 'python_test_key_12345')
    -- This is SHA-256 hash - in production, generate properly
    v_api_key_hash := encode(digest('python_test_key_12345', 'sha256'), 'hex');

    -- Insert API key
    INSERT INTO api_keys (
        id,
        organization_id,
        agent_id,
        name,
        key_hash,
        key_prefix,
        created_by,
        created_at,
        expires_at,
        is_active
    ) VALUES (
        v_api_key_id,
        v_org_id,
        v_agent_id,
        'Python SDK Test Key',
        v_api_key_hash,
        'aim_test',
        v_user_id,
        NOW(),
        NULL,  -- No expiration
        true
    );

    -- Output the agent ID and API key info for use in tests
    RAISE NOTICE 'Python SDK Test Agent created successfully!';
    RAISE NOTICE 'Agent ID: %', v_agent_id;
    RAISE NOTICE 'Organization ID: %', v_org_id;
    RAISE NOTICE 'API Key ID: %', v_api_key_id;
    RAISE NOTICE 'Test API Key (unhashed): python_test_key_12345';
    RAISE NOTICE '';
    RAISE NOTICE 'Update your Python test script with:';
    RAISE NOTICE 'AGENT_ID = "%"', v_agent_id;
    RAISE NOTICE 'API_KEY = "aim_test_python_test_key_12345"  # Use actual generated key';

END $$;
