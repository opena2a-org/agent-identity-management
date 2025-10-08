-- Expand standard tags from 10 to 20
-- Adding 10 more enterprise-grade standard tags for comprehensive coverage

DO $$
DECLARE
    org_id UUID;
    sys_user_id UUID;
BEGIN
    -- For each organization, insert additional standard tags
    FOR org_id IN SELECT id FROM organizations LOOP
        -- Get first admin user for this org
        SELECT id INTO sys_user_id FROM users
        WHERE organization_id = org_id
        ORDER BY created_at ASC
        LIMIT 1;

        -- Skip if no users in org
        IF sys_user_id IS NULL THEN
            CONTINUE;
        END IF;

        -- 11. Environment: Testing
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'environment', 'testing', 'environment', 'Testing environment', '#A855F7', true, 11, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 11;

        -- 12. Region: US East
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'region', 'us-east', 'custom', 'US East region', '#3B82F6', true, 12, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 12;

        -- 13. Region: US West
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'region', 'us-west', 'custom', 'US West region', '#2563EB', true, 13, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 13;

        -- 14. Region: EU
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'region', 'eu', 'custom', 'European Union region', '#1D4ED8', true, 14, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 14;

        -- 15. Team: Engineering
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'team', 'engineering', 'custom', 'Engineering team', '#0EA5E9', true, 15, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 15;

        -- 16. Team: Data Science
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'team', 'data-science', 'custom', 'Data Science team', '#14B8A6', true, 16, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 16;

        -- 17. Team: Security
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'team', 'security', 'custom', 'Security team', '#DC2626', true, 17, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 17;

        -- 18. Cost Center: Billable
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'cost-center', 'billable', 'custom', 'Billable to customers', '#10B981', true, 18, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 18;

        -- 19. Cost Center: Internal
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'cost-center', 'internal', 'custom', 'Internal cost center', '#F59E0B', true, 19, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 19;

        -- 20. Status: Experimental
        INSERT INTO tags (organization_id, key, value, category, description, color, is_standard, display_order, created_by)
        VALUES (org_id, 'status', 'experimental', 'custom', 'Experimental feature - not production ready', '#A855F7', true, 20, sys_user_id)
        ON CONFLICT (organization_id, key, value) DO UPDATE SET is_standard = true, display_order = 20;

    END LOOP;
END $$;
