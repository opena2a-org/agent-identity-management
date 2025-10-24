-- Add default security policies for OpenA2A Admin organization
-- Purpose: Provide out-of-the-box security policies for demonstration
-- Migration: 015_add_default_security_policies.sql
-- Date: 2025-10-21

-- First, get the admin organization ID and admin user ID
DO $$
DECLARE
    admin_org_id UUID;
    admin_user_id UUID;
BEGIN
    -- Get OpenA2A Admin organization ID
    SELECT id INTO admin_org_id FROM organizations WHERE name = 'OpenA2A Admin' LIMIT 1;

    -- Get admin user ID
    SELECT id INTO admin_user_id FROM users WHERE email = 'admin@opena2a.org' LIMIT 1;

    -- Only proceed if both IDs are found
    IF admin_org_id IS NOT NULL AND admin_user_id IS NOT NULL THEN

        -- Policy 1: Low Trust Score Alert
        INSERT INTO security_policies (
            id,
            organization_id,
            name,
            description,
            policy_type,
            enforcement_action,
            severity_threshold,
            rules,
            applies_to,
            is_enabled,
            priority,
            created_by
        ) VALUES (
            gen_random_uuid(),
            admin_org_id,
            'Low Trust Score Alert',
            'Alert when agent trust score falls below 70',
            'trust_score_low',
            'alert_only',
            'medium',
            '{"threshold": 70, "duration": "1h", "notify": ["admin"]}'::jsonb,
            'all',
            true,
            100,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        -- Policy 2: Capability Violation Detection
        INSERT INTO security_policies (
            id,
            organization_id,
            name,
            description,
            policy_type,
            enforcement_action,
            severity_threshold,
            rules,
            applies_to,
            is_enabled,
            priority,
            created_by
        ) VALUES (
            gen_random_uuid(),
            admin_org_id,
            'Capability Violation Detection',
            'Detect and alert when agents exceed their defined capabilities',
            'capability_violation',
            'block_and_alert',
            'high',
            '{"check_permissions": true, "strict_mode": false}'::jsonb,
            'all',
            true,
            200,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        -- Policy 3: Unusual Activity Monitoring
        INSERT INTO security_policies (
            id,
            organization_id,
            name,
            description,
            policy_type,
            enforcement_action,
            severity_threshold,
            rules,
            applies_to,
            is_enabled,
            priority,
            created_by
        ) VALUES (
            gen_random_uuid(),
            admin_org_id,
            'Unusual Activity Monitoring',
            'Monitor for unusual agent behavior patterns (API call spikes, unusual times)',
            'unusual_activity',
            'alert_only',
            'medium',
            '{"api_rate_threshold": 1000, "time_window": "1h", "check_off_hours": true}'::jsonb,
            'all',
            true,
            80,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        -- Policy 4: Critical Trust Score Block
        INSERT INTO security_policies (
            id,
            organization_id,
            name,
            description,
            policy_type,
            enforcement_action,
            severity_threshold,
            rules,
            applies_to,
            is_enabled,
            priority,
            created_by
        ) VALUES (
            gen_random_uuid(),
            admin_org_id,
            'Critical Trust Score Block',
            'Block agents with critically low trust scores (below 50)',
            'trust_score_low',
            'block_and_alert',
            'critical',
            '{"threshold": 50, "auto_disable": true, "notify": ["admin", "security"]}'::jsonb,
            'all',
            true,
            300,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        -- Policy 5: Failed Authentication Monitoring
        INSERT INTO security_policies (
            id,
            organization_id,
            name,
            description,
            policy_type,
            enforcement_action,
            severity_threshold,
            rules,
            applies_to,
            is_enabled,
            priority,
            created_by
        ) VALUES (
            gen_random_uuid(),
            admin_org_id,
            'Failed Authentication Monitoring',
            'Alert on repeated authentication failures',
            'auth_failure',
            'alert_only',
            'medium',
            '{"max_attempts": 5, "time_window": "15m", "lockout_duration": "30m"}'::jsonb,
            'all',
            true,
            150,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        -- Policy 6: Data Exfiltration Detection
        INSERT INTO security_policies (
            id,
            organization_id,
            name,
            description,
            policy_type,
            enforcement_action,
            severity_threshold,
            rules,
            applies_to,
            is_enabled,
            priority,
            created_by
        ) VALUES (
            gen_random_uuid(),
            admin_org_id,
            'Data Exfiltration Detection',
            'Detect potential data exfiltration through unusual data transfer patterns',
            'data_exfiltration',
            'block_and_alert',
            'high',
            '{"data_threshold_mb": 100, "time_window": "1h", "check_destinations": true}'::jsonb,
            'all',
            true,
            250,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        RAISE NOTICE 'Successfully added 6 default security policies for organization %', admin_org_id;
    ELSE
        RAISE NOTICE 'Skipping - admin organization or admin user not found';
    END IF;
END $$;
