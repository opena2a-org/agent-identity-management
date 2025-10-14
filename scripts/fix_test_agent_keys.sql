-- Fix API key hashes for test agents
-- The backend uses: base64(sha256(apiKey))

-- Enable pgcrypto if not already enabled
CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
DECLARE
    go_api_key TEXT := 'aim_test_go_sdk_key_12345';
    js_api_key TEXT := 'aim_test_js_sdk_key_67890';
    go_key_hash TEXT;
    js_key_hash TEXT;
BEGIN
    -- Generate correct hashes: base64(sha256(key))
    go_key_hash := encode(digest(go_api_key, 'sha256'), 'base64');
    js_key_hash := encode(digest(js_api_key, 'sha256'), 'base64');

    -- Update Go SDK test agent API key
    UPDATE api_keys
    SET key_hash = go_key_hash
    WHERE agent_id = (
        SELECT id FROM agents WHERE name = 'go-sdk-test-agent'
    );

    -- Update JavaScript SDK test agent API key
    UPDATE api_keys
    SET key_hash = js_key_hash
    WHERE agent_id = (
        SELECT id FROM agents WHERE name = 'javascript-sdk-test-agent'
    );

    RAISE NOTICE 'API key hashes updated successfully!';
    RAISE NOTICE 'Go SDK API Key: %', go_api_key;
    RAISE NOTICE 'Go SDK Key Hash: %', go_key_hash;
    RAISE NOTICE 'JavaScript SDK API Key: %', js_api_key;
    RAISE NOTICE 'JavaScript SDK Key Hash: %', js_key_hash;
END $$;
