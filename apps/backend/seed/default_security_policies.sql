-- AIM Default Security Policies Seed Data
-- These policies are created for every new organization
-- Run this after creating an organization and admin user

-- Note: This is a template. Replace {org_id} and {admin_user_id} with actual values

DO $$
DECLARE
    v_org_id UUID;
    v_admin_user_id UUID;
BEGIN
    -- Get the organization ID (assumes one organization, or use specific domain)
    SELECT id INTO v_org_id FROM organizations LIMIT 1;

    -- Get admin user ID for this organization
    SELECT id INTO v_admin_user_id
    FROM users
    WHERE organization_id = v_org_id AND role = 'admin'
    ORDER BY created_at ASC
    LIMIT 1;

    -- Only insert if we found both org and admin
    IF v_org_id IS NOT NULL AND v_admin_user_id IS NOT NULL THEN

        -- Check if policies already exist for this organization
        IF NOT EXISTS (SELECT 1 FROM security_policies WHERE organization_id = v_org_id) THEN

            -- Policy 1: Capability Violation Detection
            INSERT INTO security_policies (
                organization_id, name, description, policy_type,
                enforcement_action, severity_threshold, rules,
                applies_to, is_enabled, priority, created_by
            ) VALUES (
                v_org_id,
                'Capability Violation Detection',
                'Alerts when agents attempt actions beyond their defined capabilities (e.g., EchoLeak attacks)',
                'capability_violation',
                'alert_only',
                'high',
                '{"check_capability_match":true,"block_unauthorized":false}',
                'all_agents',
                true,
                100,
                v_admin_user_id
            );

            -- Policy 2: Low Trust Score Monitoring
            INSERT INTO security_policies (
                organization_id, name, description, policy_type,
                enforcement_action, severity_threshold, rules,
                applies_to, is_enabled, priority, created_by
            ) VALUES (
                v_org_id,
                'Low Trust Score Monitoring',
                'Monitors agents with trust scores below threshold for suspicious behavior',
                'trust_score_low',
                'alert_only',
                'medium',
                '{"trust_threshold":70.0,"monitor_low_trust":true,"block_low_trust":false}',
                'all_agents',
                true,
                90,
                v_admin_user_id
            );

            -- Policy 3: Unusual Activity Detection
            INSERT INTO security_policies (
                organization_id, name, description, policy_type,
                enforcement_action, severity_threshold, rules,
                applies_to, is_enabled, priority, created_by
            ) VALUES (
                v_org_id,
                'Unusual Activity Detection',
                'Detects anomalous patterns in agent behavior (rate limits, unusual timing, etc.)',
                'unusual_activity',
                'alert_only',
                'medium',
                '{"rate_limit_threshold":100,"detect_anomalies":true,"block_anomalies":false}',
                'all_agents',
                true,
                80,
                v_admin_user_id
            );

            RAISE NOTICE '✓ Created 3 default security policies for organization %', v_org_id;
        ELSE
            RAISE NOTICE '⏭  Security policies already exist for organization %', v_org_id;
        END IF;
    ELSE
        RAISE NOTICE '⚠  Skipping seed: organization or admin user not found';
    END IF;
END $$;
