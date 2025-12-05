-- Add default MCP security policies for OpenA2A Admin organization
-- Purpose: Provide out-of-the-box MCP security policies enabled by default
-- Migration: 052_add_default_mcp_policies.sql
-- Date: 2025-12-04

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

        -- Policy 1: MCP Domain Allowlist
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
            'Trusted MCP Server Domains',
            'Allow MCP servers only from trusted domains (localhost, internal networks, major cloud providers)',
            'mcp_allowlist',
            'alert_only',
            'medium',
            '{
                "allowedDomains": ["localhost", "127.0.0.1", "*.local", "*.internal", "*.amazonaws.com", "*.azure.com", "*.googleapis.com"],
                "allowedNames": [],
                "allowedCapabilities": ["tools", "resources", "prompts"],
                "requireVerified": false,
                "minTrustScore": 0,
                "minAttestations": 0
            }'::jsonb,
            'all',
            true,
            100,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        -- Policy 2: MCP Malicious Domain Blocklist
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
            'Blocked MCP Domains',
            'Block MCP servers from known malicious or untrusted domains',
            'mcp_blocklist',
            'block_and_alert',
            'critical',
            '{
                "blockedDomains": ["*.malware.com", "*.suspicious.io", "*.untrusted.net"],
                "blockedNames": ["test-malicious", "unsafe-mcp"],
                "blockedUrls": []
            }'::jsonb,
            'all',
            true,
            50,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        -- Policy 3: Required MCP Capabilities
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
            'MCP Capability Requirements',
            'Ensure MCP servers declare required capabilities and do not expose dangerous ones',
            'mcp_capabilities',
            'alert_only',
            'medium',
            '{
                "requiredCapabilities": [],
                "forbiddenCapabilities": ["system_exec", "file_delete", "network_admin"],
                "maxCapabilities": 20,
                "requireCapabilityDeclaration": true
            }'::jsonb,
            'all',
            true,
            150,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        -- Policy 4: Unverified MCP Server Policy
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
            'Unverified MCP Server Restrictions',
            'Alert when agents connect to unverified MCP servers',
            'mcp_unverified',
            'alert_only',
            'low',
            '{
                "action": "alert",
                "allowPendingVerification": true,
                "maxUnverifiedConnections": 5,
                "requireVerificationWithin": "7d",
                "notifyOnNewConnection": true
            }'::jsonb,
            'all',
            true,
            200,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        -- Policy 5: MCP Trust Score Requirements
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
            'MCP Minimum Trust Score',
            'Alert when MCP servers fall below minimum trust score threshold',
            'mcp_allowlist',
            'alert_only',
            'medium',
            '{
                "allowedDomains": ["*"],
                "allowedNames": [],
                "allowedCapabilities": [],
                "requireVerified": false,
                "minTrustScore": 50,
                "minAttestations": 1
            }'::jsonb,
            'all',
            true,
            175,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        -- Policy 6: High-Risk MCP Server Block
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
            'High-Risk MCP Server Block',
            'Block connections to MCP servers with critically low trust scores',
            'mcp_allowlist',
            'block_and_alert',
            'critical',
            '{
                "allowedDomains": ["*"],
                "allowedNames": [],
                "allowedCapabilities": [],
                "requireVerified": true,
                "minTrustScore": 30,
                "minAttestations": 2
            }'::jsonb,
            'all',
            true,
            25,
            admin_user_id
        ) ON CONFLICT DO NOTHING;

        RAISE NOTICE 'Successfully added 6 default MCP security policies for organization %', admin_org_id;
    ELSE
        RAISE NOTICE 'Skipping - admin organization or admin user not found';
    END IF;
END $$;
