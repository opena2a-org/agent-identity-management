# 🚀 AIM Deployment Checklist

**Purpose**: Ensure complete, consistent deployments with zero manual fixes

---

## ✅ Pre-Deployment Verification

Before running `./scripts/deploy-azure-production.sh`, verify:

- [ ] **All migrations exist** in `apps/backend/migrations/`:
  - [ ] 001_initial_schema.sql (base tables)
  - [ ] 002_add_missing_user_columns.sql (user status, approvals, password reset)
  - [ ] 002_fix_alerts_schema.up.sql (alert indexes)
  - [ ] 003_add_missing_agent_columns.sql (agent metadata)
  - [ ] 004_create_mcp_servers_table.sql (MCP server registry)
  - [ ] 005_create_verification_events_table.sql (verification tracking)
  - [ ] 006_add_password_hash_column.sql (password authentication)
  - [ ] 007_create_system_config_table.sql (system configuration)
  - [ ] 008_create_sdk_tokens_table.sql (SDK token management)
  - [ ] 009_create_security_policies_table.sql (security policies)

- [ ] **Deployment script includes**:
  - [ ] Database migration step (applies 001-009)
  - [ ] Bootstrap step (creates admin user)
  - [ ] Security policy backfill step (creates 3 default policies)

- [ ] **Backend binaries buildable**:
  - [ ] `apps/backend/cmd/server` (main API server)
  - [ ] `apps/backend/cmd/bootstrap` (admin user creation)
  - [ ] `apps/backend/cmd/backfill_policies` (default security policies)

---

## 🔧 Deployment Script Verification

The deployment script **MUST** include these steps in order:

### Step 1-10: Infrastructure (Automatic)
✅ Resource group, ACR, PostgreSQL, Redis, Container Apps, Backend, Frontend

### Step 11: Database Migrations (Automatic)
```bash
# Migrations 001-009 applied automatically via backend startup
# OR manually via bootstrap script
```

### Step 12: Bootstrap Admin User (Automatic)
```bash
cd apps/backend
go build -o /tmp/aim-bootstrap ./cmd/bootstrap
/tmp/aim-bootstrap \
  --admin-email="admin@opena2a.org" \
  --admin-password="Admin2025!Secure" \
  --admin-name="System Administrator" \
  --org-name="OpenA2A" \
  --org-domain="opena2a.org" \
  --database-url="postgresql://..." \
  --yes
```

### Step 13: Create Default Security Policies (Automatic) ⚠️ CRITICAL
```bash
cd apps/backend
go build -o /tmp/aim-backfill-policies ./cmd/backfill_policies
DATABASE_URL="postgresql://..." /tmp/aim-backfill-policies
```

**⚠️ IF THIS STEP IS MISSING**: Security Policies page will show 0 policies!

---

## 📊 Post-Deployment Verification

After deployment completes, verify ALL of the following:

### 1. Backend Health
```bash
curl https://aim-prod-backend.*.azurecontainerapps.io/health
# Expected: {"status": "healthy"}
```

### 2. Database Tables (13 required)
```sql
\dt
# Expected tables:
# - agents
# - alerts
# - api_keys
# - audit_logs
# - mcp_servers
# - organizations
# - schema_migrations
# - sdk_tokens          ← MUST EXIST
# - security_policies   ← MUST EXIST
# - system_config       ← MUST EXIST
# - trust_scores
# - users
# - verification_events
```

### 3. Database Migrations (10 required)
```sql
SELECT version FROM schema_migrations ORDER BY applied_at;
# Expected: 001 through 009 (10 migrations total including 002_fix)
```

### 4. Admin User Created
```sql
SELECT id, email, name, role, email_verified
FROM users
WHERE role = 'admin';
# Expected: 1 row, email_verified = true
```

### 5. Default Security Policies (3 required)
```sql
SELECT name, priority, enforcement_action, is_enabled
FROM security_policies
ORDER BY priority DESC;
# Expected: 3 rows
# - Capability Violation Detection (priority 100)
# - Low Trust Score Monitoring (priority 90)
# - Unusual Activity Detection (priority 80)
```

### 6. Frontend Pages Working

**SDK Tokens Page**:
- URL: `/dashboard/sdk-tokens`
- Expected: Shows "No SDK tokens found" (empty state with 0 active, 0 usage, 0 revoked)
- ❌ Failure: 500 error "Failed to list SDK tokens"

**Security Policies Page**:
- URL: `/dashboard/admin/security-policies`
- Expected: Shows 3 policies (3 total, 3 enabled, 3 monitoring, 0 blocking)
- ❌ Failure: Shows 0 policies or empty state

### 7. Login Flow
```bash
# Try logging in with admin credentials
curl -X POST https://aim-prod-backend.*.azurecontainerapps.io/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@opena2a.org","password":"Admin2025!Secure"}'
# Expected: {"token": "...", "user": {...}}
```

---

## 🚨 Common Failure Points

### Failure: SDK Tokens Page Shows 500 Error
**Symptom**: Console error "Failed to list SDK tokens"
**Cause**: `sdk_tokens` table doesn't exist
**Fix**: Migration 008 was not applied
**Prevention**: Ensure deployment script applies migrations 001-009

### Failure: Security Policies Page Shows 0 Policies
**Symptom**: Page shows "No security policies configured"
**Cause**: Backfill tool was not run
**Fix**: Run `/tmp/aim-backfill-policies` manually
**Prevention**: Ensure deployment script includes Step 13 (backfill)

### Failure: Bootstrap Fails - "password_hash column does not exist"
**Symptom**: Bootstrap script fails during admin user creation
**Cause**: Migration 006 was not applied
**Fix**: Apply migration 006 manually
**Prevention**: Ensure deployment script applies migrations 001-009

### Failure: Bootstrap Fails - "system_config table does not exist"
**Symptom**: Bootstrap script fails during system config update
**Cause**: Migration 007 was not applied
**Fix**: Apply migration 007 manually
**Prevention**: Ensure deployment script applies migrations 001-009

---

## 📋 Required Tables & Their Migrations

| Table | Migration | Purpose |
|-------|-----------|---------|
| organizations | 001 | Organizations |
| users | 001 + 002 + 006 | Users with password auth |
| agents | 001 + 003 | AI agents |
| api_keys | 001 | API keys |
| trust_scores | 001 | Trust scores |
| audit_logs | 001 | Audit trail |
| alerts | 001 + 002_fix | Alerts |
| mcp_servers | 004 | MCP servers |
| verification_events | 005 | Verification history |
| system_config | 007 | System configuration |
| sdk_tokens | 008 | SDK tokens |
| security_policies | 009 | Security policies |
| schema_migrations | auto | Migration tracking |

---

## 🔐 Required Columns in Users Table

The `users` table MUST have these columns for authentication:

```sql
-- From migration 001
id, organization_id, email, name, avatar_url, role, provider, provider_id,
last_login_at, created_at, updated_at

-- From migration 002
status, deleted_at, approved_by, approved_at,
password_reset_token, password_reset_expires_at

-- From migration 006 (CRITICAL for password login)
password_hash, email_verified, force_password_change
```

**❌ Missing `password_hash`**: Bootstrap will fail
**❌ Missing `email_verified`**: Bootstrap will fail
**❌ Missing `force_password_change`**: Bootstrap will fail

---

## 🎯 Success Criteria

Deployment is considered successful when:

- ✅ All 13 tables exist in database
- ✅ All 10 migrations applied
- ✅ 1 admin user exists with `email_verified = true`
- ✅ 3 default security policies exist and are enabled
- ✅ Backend /health endpoint returns 200
- ✅ Frontend loads without errors
- ✅ SDK Tokens page shows empty state (not 500 error)
- ✅ Security Policies page shows 3 policies (not 0)
- ✅ Login with admin credentials succeeds
- ✅ No manual psql commands required

---

## 🛠️ Emergency Manual Fixes

**IF** something goes wrong during deployment:

### Missing sdk_tokens Table
```bash
PGPASSWORD='xxx' psql -h <host> -U aimadmin -d identity \
  -f apps/backend/migrations/008_create_sdk_tokens_table.sql

# Record in migration tracker
PGPASSWORD='xxx' psql -h <host> -U aimadmin -d identity \
  -c "INSERT INTO schema_migrations (version, applied_at)
      VALUES ('008_create_sdk_tokens_table.sql', NOW());"
```

### Missing security_policies Table
```bash
PGPASSWORD='xxx' psql -h <host> -U aimadmin -d identity \
  -f apps/backend/migrations/009_create_security_policies_table.sql

# Record in migration tracker
PGPASSWORD='xxx' psql -h <host> -U aimadmin -d identity \
  -c "INSERT INTO schema_migrations (version, applied_at)
      VALUES ('009_create_security_policies_table.sql', NOW());"
```

### Missing Default Security Policies
```bash
cd apps/backend
go build -o /tmp/backfill ./cmd/backfill_policies
DATABASE_URL="postgresql://..." /tmp/backfill
```

---

## 📖 Reference Documents

- `PRODUCTION_FIXES_LOG.md` - Complete history of all manual fixes applied
- `PRODUCTION_DEPLOYMENT_COMPLETE.md` - Final status of production deployment
- `scripts/deploy-azure-production.sh` - Automated deployment script
- `apps/backend/migrations/` - All database migrations

---

**Last Updated**: 2025-10-20
**Deployment Version**: v1.0 (with migrations 001-009 + backfill)
**Status**: ✅ Production deployments should be fully automated
