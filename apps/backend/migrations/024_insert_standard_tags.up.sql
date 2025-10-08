-- Insert 10 curated standard enterprise tags
-- These tags enable rich compliance features users will love and pay for

-- Note: This migration inserts standard tags for ALL organizations
-- The system_user_id should be replaced with actual system user or first admin user

DO $$
DECLARE
    org_id UUID;
    sys_user_id UUID;
BEGIN
    -- For each organization, insert standard tags
    FOR org_id IN SELECT id FROM organizations LOOP
        -- Get first admin user for this org (or use first user)
        SELECT id INTO sys_user_id FROM users
        WHERE organization_id = org_id
        ORDER BY created_at ASC
        LIMIT 1;

        -- Skip if no users in org
        IF sys_user_id IS NULL THEN
            CONTINUE;
        END IF;

        -- 1. Environment: Production
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'environment', 'production', 'environment', 'Production environment', '#10B981', true, 1, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 1;

        -- 2. Environment: Staging
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'environment', 'staging', 'environment', 'Staging environment', '#F59E0B', true, 2, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 2;

        -- 3. Environment: Development
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'environment', 'development', 'environment', 'Development environment', '#3B82F6', true, 3, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 3;

        -- 4. Data Classification: Public
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'classification', 'public', 'data_classification', 'Public data - no restrictions', '#10B981', true, 4, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 4;

        -- 5. Data Classification: Internal
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'classification', 'internal', 'data_classification', 'Internal use only', '#F59E0B', true, 5, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 5;

        -- 6. Data Classification: Confidential
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'classification', 'confidential', 'data_classification', 'Confidential data - restricted access', '#EF4444', true, 6, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 6;

        -- 7. Compliance: SOC2
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'compliance', 'soc2', 'custom', 'SOC 2 compliance required', '#8B5CF6', true, 7, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 7;

        -- 8. Compliance: HIPAA
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'compliance', 'hipaa', 'custom', 'HIPAA compliance required', '#EC4899', true, 8, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 8;

        -- 9. Compliance: GDPR
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'compliance', 'gdpr', 'custom', 'GDPR compliance required', '#06B6D4', true, 9, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 9;

        -- 10. Critical: Business Critical
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'priority', 'critical', 'custom', 'Business critical - requires extra monitoring', '#DC2626', true, 10, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 10;

    END LOOP;
END $$;
