# AIM Database Migration Guide

**Last Updated**: October 21, 2025

## Overview

This guide ensures smooth database migrations for fresh deployments and updates to the Agent Identity Management (AIM) platform.

## Migration Order

Migrations are numbered and must be executed in order. All migrations are idempotent and safe to run multiple times.

### Core Schema Migrations (001-010)

1. **001_initial_schema.sql** - Base tables (organizations, users, agents, api_keys, audit_logs, alerts)
2. **002_add_missing_user_columns.sql** - User table enhancements (provider, provider_id, etc.)
3. **003_add_missing_agent_columns.sql** - Agent table enhancements (capabilities, trust_score, etc.)
4. **004_create_mcp_servers_table.sql** - MCP server registration support
5. **005_create_verification_events_table.sql** - Agent verification tracking
6. **006_add_password_hash_column.sql** - Password authentication support
7. **007_create_system_config_table.sql** - System configuration storage
8. **008_create_sdk_tokens_table.sql** - SDK token management
9. **009_create_security_policies_table.sql** - Security policy framework
10. **010_create_analytics_tables.sql** - Analytics and metrics tracking

### Feature Migrations (011-015)

11. **011_add_password_reset_fields.sql** - Password reset functionality
12. **012_create_user_registration_requests_table.sql** - User registration workflow
13. **013_create_default_admin_user.sql** - Default admin account creation
14. **014_fix_alerts_schema.sql** - Alerts table schema corrections
15. **015_add_default_security_policies.sql** - Default security policies seed data

## Fresh Deployment Checklist

### Prerequisites
- PostgreSQL 15+ database instance
- Database connection with admin privileges
- SMTP server credentials for email notifications

### Step 1: Environment Variables

Ensure these environment variables are set:

```bash
# Database (REQUIRED)
POSTGRES_HOST="your-db-host.postgres.database.azure.com"
POSTGRES_PORT="5432"
POSTGRES_USER="aimadmin"
POSTGRES_PASSWORD="your-secure-password"
POSTGRES_DB="identity"
POSTGRES_SSL_MODE="require"  # CRITICAL for Azure PostgreSQL

# Frontend URL (REQUIRED for emails)
FRONTEND_URL="https://your-frontend-url.azurecontainerapps.io"

# Email SMTP (REQUIRED for notifications)
SMTP_HOST="smtp.gmail.com"
SMTP_PORT="587"
SMTP_USERNAME="info@opena2a.org"
SMTP_PASSWORD="your-smtp-password"
SUPPORT_EMAIL="support@opena2a.org"

# Redis (OPTIONAL - backend gracefully handles failures)
REDIS_HOST="your-redis.redis.cache.windows.net"
REDIS_PORT="6380"
REDIS_PASSWORD="your-redis-password"
```

### Step 2: Run Migrations

All migrations are in `apps/backend/migrations/` and are automatically executed on backend startup.

**Automatic Migration** (Recommended):
```bash
# Migrations run automatically when backend starts
docker compose up -d backend
# or
go run apps/backend/cmd/server/main.go
```

**Manual Migration** (If needed):
```bash
# Using Supabase MCP (if available)
mcp__supabase__apply_migration({
  project_id: "your-project-id",
  name: "001_initial_schema",
  query: "<SQL content>"
})

# Using psql directly
for f in apps/backend/migrations/*.sql; do
  psql "$DATABASE_URL" -f "$f"
done
```

### Step 3: Verify Migrations

Check that all tables exist:

```sql
SELECT tablename FROM pg_tables
WHERE schemaname = 'public'
ORDER BY tablename;
```

Expected tables:
- `alerts`
- `agents`
- `analytics_agent_activity`
- `analytics_trust_scores`
- `api_keys`
- `audit_logs`
- `mcp_servers`
- `organizations`
- `sdk_tokens`
- `security_policies`
- `system_config`
- `user_registration_requests`
- `users`
- `verification_events`

### Step 4: Verify Default Data

**Default Admin User** (created by migration 013):
```sql
SELECT id, email, role, status FROM users WHERE email = 'admin@opena2a.org';
```

Expected result:
- Email: `admin@opena2a.org`
- Password: `AIM2025!Secure` (MUST be changed on first login)
- Role: `admin`
- Status: `active`
- Organization: `OpenA2A Admin`

**Default Security Policies** (created by migration 015):
```sql
SELECT name, policy_type, enforcement_action, priority
FROM security_policies
ORDER BY priority DESC;
```

Expected result: 6 policies
1. Critical Trust Score Block (priority 300)
2. Data Exfiltration Detection (priority 250)
3. **Capability Violation Detection** (priority 200) ← Only enforced policy in MVP
4. Failed Authentication Monitoring (priority 150)
5. Low Trust Score Alert (priority 100)
6. Unusual Activity Monitoring (priority 80)

## Troubleshooting

### Issue: "relation already exists"
**Solution**: Migration has already been run. This is safe - migrations use `IF NOT EXISTS` and `ON CONFLICT DO NOTHING`.

### Issue: Admin user not created
**Cause**: Migration 013 depends on organization existing first.
**Solution**: Ensure migrations run in order. Migration 001 creates organizations table.

### Issue: Security policies not created
**Cause**: Migration 015 depends on admin user existing.
**Solution**: Ensure migrations 001-014 have run successfully before 015.

### Issue: Password authentication failed
**Cause**: PostgreSQL credentials incorrect or SSL mode not set.
**Solution**:
1. Verify `POSTGRES_PASSWORD` is correct
2. For Azure PostgreSQL, MUST set `POSTGRES_SSL_MODE=require`

### Issue: Email notifications not working
**Cause**: SMTP credentials missing or incorrect.
**Solution**:
1. Verify `SMTP_HOST`, `SMTP_USERNAME`, `SMTP_PASSWORD` are set
2. Check backend logs for email sending errors
3. Check spam folder - automated emails often filtered

## Migration Safety

All migrations are designed to be:

1. **Idempotent**: Safe to run multiple times
2. **Non-destructive**: Use `IF NOT EXISTS`, `ON CONFLICT DO NOTHING`
3. **Backwards compatible**: New columns are nullable or have defaults
4. **Atomic**: Each migration is a single transaction

## Adding New Migrations

When adding new migrations:

1. **Naming**: Use format `XXX_description.sql` (e.g., `016_add_new_feature.sql`)
2. **Idempotency**: Always use `IF NOT EXISTS` or `ON CONFLICT DO NOTHING`
3. **Testing**: Test on fresh database before deploying
4. **Documentation**: Update this guide with new migration details

## Production Deployment

### Azure Container Apps Deployment

1. **Backend deployment triggers automatic migrations**:
```bash
az containerapp update \
  --name aim-prod-backend \
  --resource-group aim-production-rg \
  --image aimprodacr.azurecr.io/aim-backend:latest
```

2. **Verify migrations in logs**:
```bash
az containerapp logs show \
  --name aim-prod-backend \
  --resource-group aim-production-rg \
  --tail 100
```

3. **Check for migration success messages**:
```
✅ Migrating database schema...
✅ Database migration completed successfully
```

## Post-Migration Verification

After successful migration:

1. ✅ Admin user can log in at `/auth/login`
2. ✅ Security policies visible at `/dashboard/admin/security-policies`
3. ✅ New user registration workflow functional
4. ✅ Email notifications working (welcome + approval emails)
5. ✅ Capability violation enforcement active

## Support

For migration issues:
- Check backend logs for detailed error messages
- Verify all environment variables are set correctly
- Ensure PostgreSQL version is 15 or higher
- Confirm network connectivity to database (especially for Azure)

---

**Last Tested**: October 21, 2025
**Database**: PostgreSQL 16
**Deployment**: Azure Container Apps
