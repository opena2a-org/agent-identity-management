-- Create default admin user on first startup
-- Default credentials: admin@localhost / admin
-- User MUST change password on first login

DO $$
DECLARE
    default_org_id UUID;
    admin_exists BOOLEAN;
BEGIN
    -- Check if any users exist (if yes, skip admin creation)
    SELECT EXISTS(SELECT 1 FROM users LIMIT 1) INTO admin_exists;

    IF NOT admin_exists THEN
        -- Get the first organization (or create a default one)
        SELECT id INTO default_org_id FROM organizations ORDER BY created_at LIMIT 1;

        -- If no organization exists, create a default one
        IF default_org_id IS NULL THEN
            INSERT INTO organizations (
                id,
                name,
                status,
                created_at,
                updated_at
            ) VALUES (
                gen_random_uuid(),
                'Default Organization',
                'active',
                NOW(),
                NOW()
            ) RETURNING id INTO default_org_id;
        END IF;

        -- Create default admin user
        -- Password hash is bcrypt hash of "admin"
        INSERT INTO users (
            id,
            organization_id,
            email,
            name,
            role,
            status,
            password_hash,
            force_password_change,
            created_at,
            updated_at
        ) VALUES (
            gen_random_uuid(),
            default_org_id,
            'admin@localhost',
            'Default Administrator',
            'admin',
            'active',
            '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy', -- bcrypt hash of "admin"
            TRUE, -- Force password change on first login
            NOW(),
            NOW()
        );

        RAISE NOTICE 'Default admin user created with email: admin@localhost and password: admin (MUST be changed on first login)';
    ELSE
        RAISE NOTICE 'Users already exist in database, skipping default admin creation';
    END IF;
END $$;
