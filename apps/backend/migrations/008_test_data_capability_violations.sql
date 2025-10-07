-- Migration 008: Test Data for Capability Verification
-- This adds sample capabilities and violations for testing

-- Insert sample capabilities for existing agents
-- Note: Adjust agent IDs based on your actual test data
DO $$
DECLARE
    test_agent_id UUID;
    test_user_id UUID;
BEGIN
    -- Get first agent from database
    SELECT id INTO test_agent_id FROM agents LIMIT 1;

    -- Get first user from database
    SELECT id INTO test_user_id FROM users LIMIT 1;

    IF test_agent_id IS NOT NULL AND test_user_id IS NOT NULL THEN
        -- Grant file:read capability
        INSERT INTO agent_capabilities (agent_id, capability_type, granted_by, granted_at)
        VALUES (test_agent_id, 'file:read', test_user_id, NOW() - INTERVAL '30 days')
        ON CONFLICT (agent_id, capability_type) DO NOTHING;

        -- Grant file:write capability
        INSERT INTO agent_capabilities (agent_id, capability_type, granted_by, granted_at)
        VALUES (test_agent_id, 'file:write', test_user_id, NOW() - INTERVAL '30 days')
        ON CONFLICT (agent_id, capability_type) DO NOTHING;

        -- Grant api:call capability
        INSERT INTO agent_capabilities (agent_id, capability_type, granted_by, granted_at)
        VALUES (test_agent_id, 'api:call', test_user_id, NOW() - INTERVAL '30 days')
        ON CONFLICT (agent_id, capability_type) DO NOTHING;

        -- Create some capability violations (simulating unauthorized actions)

        -- Violation 1: Attempted database write without permission (CRITICAL)
        INSERT INTO capability_violations (
            agent_id, attempted_capability, registered_capabilities,
            severity, trust_score_impact, is_blocked, source_ip,
            request_metadata, created_at
        )
        VALUES (
            test_agent_id,
            'db:write',
            '{"capabilities": ["file:read", "file:write", "api:call"]}'::jsonb,
            'critical',
            -30,
            true,
            '192.168.1.100',
            '{"requestId": "req_12345", "endpoint": "/api/database/write", "payload_size": 1024}'::jsonb,
            NOW() - INTERVAL '2 hours'
        );

        -- Violation 2: Attempted user impersonation (HIGH)
        INSERT INTO capability_violations (
            agent_id, attempted_capability, registered_capabilities,
            severity, trust_score_impact, is_blocked, source_ip,
            request_metadata, created_at
        )
        VALUES (
            test_agent_id,
            'user:impersonate',
            '{"capabilities": ["file:read", "file:write", "api:call"]}'::jsonb,
            'high',
            -20,
            true,
            '192.168.1.100',
            '{"requestId": "req_12346", "target_user": "admin@example.com"}'::jsonb,
            NOW() - INTERVAL '5 hours'
        );

        -- Violation 3: Attempted data export without permission (MEDIUM)
        INSERT INTO capability_violations (
            agent_id, attempted_capability, registered_capabilities,
            severity, trust_score_impact, is_blocked, source_ip,
            request_metadata, created_at
        )
        VALUES (
            test_agent_id,
            'data:export',
            '{"capabilities": ["file:read", "file:write", "api:call"]}'::jsonb,
            'medium',
            -10,
            false,
            '192.168.1.101',
            '{"requestId": "req_12347", "data_type": "user_data", "record_count": 500}'::jsonb,
            NOW() - INTERVAL '1 day'
        );

        -- Violation 4: Attempted system admin action (CRITICAL)
        INSERT INTO capability_violations (
            agent_id, attempted_capability, registered_capabilities,
            severity, trust_score_impact, is_blocked, source_ip,
            request_metadata, created_at
        )
        VALUES (
            test_agent_id,
            'system:admin',
            '{"capabilities": ["file:read", "file:write", "api:call"]}'::jsonb,
            'critical',
            -30,
            true,
            '192.168.1.100',
            '{"requestId": "req_12348", "action": "modify_permissions", "target": "all_users"}'::jsonb,
            NOW() - INTERVAL '3 days'
        );

        -- Violation 5: Attempted file deletion (LOW - but still a violation)
        INSERT INTO capability_violations (
            agent_id, attempted_capability, registered_capabilities,
            severity, trust_score_impact, is_blocked, source_ip,
            request_metadata, created_at
        )
        VALUES (
            test_agent_id,
            'file:delete',
            '{"capabilities": ["file:read", "file:write", "api:call"]}'::jsonb,
            'low',
            -5,
            false,
            '192.168.1.102',
            '{"requestId": "req_12349", "file_path": "/tmp/test.txt"}'::jsonb,
            NOW() - INTERVAL '7 days'
        );

        -- Update agent's trust score and violation count based on violations
        UPDATE agents
        SET
            trust_score = GREATEST(0, trust_score - 95),  -- Subtract total impact (30+20+10+30+5)
            capability_violation_count = 5,
            is_compromised = true  -- Mark as compromised due to critical violations
        WHERE id = test_agent_id;

        RAISE NOTICE 'Test data created successfully for agent %', test_agent_id;
    ELSE
        RAISE NOTICE 'No agents or users found - skipping test data creation';
    END IF;
END $$;
