# Automatic Database Migrations - Production Fix

## 🎯 Problem Solved

**Before**: Production deployments failed with database errors because migrations weren't automatically applied.

**Error Example**:
```
pq: column "force_password_change" does not exist
```

**After**: Database migrations run automatically on EVERY container startup. Zero manual intervention required.

---

## ✅ How It Works

### 1. Container Startup Flow

```
Container Start
  ↓
Database Connection
  ↓
🔄 Run Migrations (AUTOMATIC)
  ↓ (Success)
Start HTTP Server
  ↓
✅ Ready for Traffic
```

**If migrations fail**: Container exits immediately (fail-fast principle).

### 2. Migration Tracking

**Schema Migrations Table**:
```sql
CREATE TABLE schema_migrations (
    id SERIAL PRIMARY KEY,
    version VARCHAR(255) NOT NULL UNIQUE,
    applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

**How Tracking Works**:
1. On startup, backend reads `migrations/*.sql` files
2. Checks `schema_migrations` table for already-applied migrations
3. Applies only PENDING migrations (idempotent)
4. Records each migration in tracking table
5. Logs progress: `🔄 Applying 001_initial_schema.sql...`
6. Success: `✅ Applied 001_initial_schema.sql`

---

## 📂 File Locations

### Backend Code
- **Migration Runner**: `apps/backend/cmd/server/main.go` (lines 64-69, 1033-1177)
- **Migration Files**: `apps/backend/migrations/*.sql`
- **Dockerfile**: `apps/backend/infrastructure/docker/Dockerfile.backend` (line 36 - copies migrations)

### Key Functions
| Function | Purpose |
|----------|---------|
| `runMigrations(db)` | Executes all pending migrations |
| `createMigrationsTable(db)` | Creates tracking table if needed |
| `getMigrationFiles()` | Discovers .up.sql files in sorted order |
| `getAppliedMigrations(db)` | Gets list of already-applied migrations |
| `getMigrationVersion(filename)` | Extracts version from filename |

---

## 🚀 Deployment Commands

### Full Deployment (Backend + Migrations)

```bash
# 1. Login to Azure Container Registry
az acr login --name aimdemoregistry

# 2. Build backend with migrations (no-cache for guaranteed fresh build)
docker buildx build \
  --no-cache \
  --platform linux/amd64 \
  -f apps/backend/infrastructure/docker/Dockerfile.backend \
  -t aimdemoregistry.azurecr.io/aim-backend:latest \
  --push .

# 3. Deploy to Azure Container Apps
az containerapp update \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --image aimdemoregistry.azurecr.io/aim-backend:latest \
  --revision-suffix $(date +%s)

# 4. Verify migrations ran successfully
az containerapp logs show \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --tail 100 \
  --follow false | grep -E "(migration|Migration|✅|❌|🔄)"
```

### Expected Log Output

**✅ SUCCESS**:
```
2025-10-20T16:05:04Z ✅ Database connected
2025-10-20T16:05:04Z 🔄 Running database migrations...
2025-10-20T16:05:04Z 🔄 Applying 001_initial_schema.sql...
2025-10-20T16:05:04Z ✅ Applied 001_initial_schema.sql
2025-10-20T16:05:04Z 🔄 Applying 002_api_keys.up.sql...
2025-10-20T16:05:04Z ✅ Applied 002_api_keys.up.sql
2025-10-20T16:05:04Z ✅ Successfully applied 2 pending migration(s)
2025-10-20T16:05:04Z ✅ Database migrations completed successfully
2025-10-20T16:05:05Z 🚀 Agent Identity Management API starting on port 8080
```

**Already Applied** (Idempotent):
```
2025-10-20T16:05:04Z 🔄 Running database migrations...
2025-10-20T16:05:04Z ⏭️  Skipping 001_initial_schema.sql (already applied)
2025-10-20T16:05:04Z ⏭️  Skipping 002_api_keys.up.sql (already applied)
2025-10-20T16:05:04Z ℹ️  All migrations already applied (database is up to date)
2025-10-20T16:05:04Z ✅ Database migrations completed successfully
```

**❌ FAILURE** (Container exits):
```
2025-10-20T16:05:04Z 🔄 Running database migrations...
2025-10-20T16:05:04Z 🔄 Applying 001_initial_schema.sql...
2025-10-20T16:05:04Z ❌ Database migrations failed:failed to execute 001_initial_schema.sql: pq: syntax error at or near "CREATE"
```

---

## 🔧 Azure PostgreSQL Compatibility

### Problem: Extension Restrictions

Azure PostgreSQL **DOES NOT** allow user-managed extensions:
- ❌ `CREATE EXTENSION "uuid-ossp"` → **BLOCKED**
- ❌ `CREATE EXTENSION "pgcrypto"` → **BLOCKED**

### Solution: Built-in Functions

Azure PostgreSQL has `gen_random_uuid()` **built-in** (no extension needed):

**Before** (broke on Azure):
```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    ...
);
```

**After** (works everywhere):
```sql
-- Azure PostgreSQL has gen_random_uuid() built-in (no extension needed)

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ...
);
```

### Benefits
✅ Works on Azure PostgreSQL
✅ Works on AWS RDS PostgreSQL
✅ Works on GCP Cloud SQL PostgreSQL
✅ Works on self-hosted PostgreSQL 13+
✅ No extension management required

---

## 📝 Adding New Migrations

### Naming Convention
```
XXX_descriptive_name.sql
XXX_descriptive_name.up.sql    # Forward migration (preferred)
XXX_descriptive_name.down.sql  # Rollback (optional, not used by auto-migration)
```

**Examples**:
- `001_initial_schema.sql`
- `002_api_keys.up.sql`
- `003_trust_scores.up.sql`
- `004_force_password_change.up.sql`

### Migration Template

```sql
-- Migration: 005_add_feature_x.up.sql
-- Purpose: Add new table for feature X
-- Date: 2025-10-20

-- Create new table
CREATE TABLE feature_x (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Add index for performance
CREATE INDEX idx_feature_x_name ON feature_x(name);

-- Add foreign key to existing table
ALTER TABLE agents
ADD COLUMN feature_x_id UUID REFERENCES feature_x(id) ON DELETE SET NULL;
```

### Best Practices

1. **One Migration Per Feature**: Each feature/change gets its own migration file
2. **Backwards Compatible**: Avoid breaking existing queries if possible
3. **Test Locally First**: Run migration on local PostgreSQL before deploying
4. **Idempotent When Possible**: Use `CREATE TABLE IF NOT EXISTS` when appropriate
5. **Add Comments**: Explain WHY the migration exists

### Testing New Migrations

```bash
# 1. Create new migration file
echo "CREATE TABLE test_feature (...)" > apps/backend/migrations/006_test_feature.up.sql

# 2. Run locally with Docker Compose
docker-compose down -v  # Clean slate
docker-compose up -d postgres
docker-compose up backend  # Watch logs for migration

# 3. Verify migration applied
docker-compose exec postgres psql -U postgres -d identity -c "SELECT * FROM schema_migrations;"

# 4. If successful, commit and deploy
git add apps/backend/migrations/006_test_feature.up.sql
git commit -m "feat: Add test_feature migration"
git push
```

---

## 🐛 Troubleshooting

### Issue: "relation already exists"

**Cause**: Tables already created manually (expected on existing deployments).

**Solution**: This is NORMAL. Migrations are idempotent - they check what's already applied.

**Verify**:
```sql
SELECT version FROM schema_migrations ORDER BY applied_at;
```

If migration is listed, it's already applied (skip error is expected).

---

### Issue: Migration fails with syntax error

**Cause**: SQL syntax error in migration file.

**Solution**:
1. Test migration locally on PostgreSQL
2. Check for Azure-specific restrictions (no extensions!)
3. Fix SQL, rebuild, redeploy

**Debugging**:
```bash
# Check migration file syntax
cat apps/backend/migrations/001_initial_schema.sql | psql -U postgres -d identity

# Check backend logs for exact error
az containerapp logs show --name aim-backend --resource-group aim-demo-rg --tail 200 | grep "❌"
```

---

### Issue: Backend won't start after migration failure

**Cause**: Migration failed, backend exits (fail-fast by design).

**Solution**:
1. Check logs for exact SQL error
2. Fix migration file
3. Rebuild and redeploy
4. Container will restart with fixed migration

**Emergency Rollback** (if needed):
```sql
-- Connect to production database
psql -h aim-demo-db.postgres.database.azure.com -U aimadmin -d identity

-- Remove failed migration from tracking table
DELETE FROM schema_migrations WHERE version = '005_broken_migration.up.sql';

-- Redeploy with fixed migration
```

---

## 📊 Monitoring

### Check Migration Status

```bash
# View migration history in database
az containerapp exec \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --command "psql $DATABASE_URL -c 'SELECT version, applied_at FROM schema_migrations ORDER BY applied_at;'"
```

### Monitor Startup Logs

```bash
# Follow logs in real-time
az containerapp logs show \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --follow

# Filter for migration-related logs only
az containerapp logs show \
  --name aim-backend \
  --resource-group aim-demo-rg \
  --tail 500 | grep -E "(migration|Migration|Database|✅|❌|🔄)"
```

---

## 🎉 Success Criteria

✅ Backend container starts without errors
✅ Logs show "✅ Database migrations completed successfully"
✅ Login works (no "column does not exist" errors)
✅ `schema_migrations` table shows all applied migrations
✅ Open-source users can deploy with `docker-compose up` (zero manual DB setup)

---

## 🚀 For Open-Source Users

### Quick Start (No Manual DB Setup!)

```bash
# Clone repository
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management

# Start entire stack (backend + frontend + postgres + redis)
docker-compose up

# That's it! Migrations run automatically on backend startup.
```

### Environment Variables (Optional)

```bash
# .env file (optional - defaults work out of box)
POSTGRES_HOST=postgres
POSTGRES_PORT=5432
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_DB=identity
POSTGRES_SSL_MODE=disable
```

---

## 📚 Related Documentation

- **Frontend API URL Fix**: `DEPLOYMENT_FIX.md`
- **Backend Configuration**: `apps/backend/README.md`
- **Database Schema**: `apps/backend/migrations/001_initial_schema.sql`
- **Docker Compose**: `docker-compose.yml`

---

## 🙏 Credits

**Fixed**: October 20, 2025
**Commit**: d77d1d7
**Tested On**: Azure Container Apps + Azure PostgreSQL Flexible Server
**Works On**: Azure, AWS, GCP, Self-Hosted PostgreSQL 13+

---

**Last Updated**: October 20, 2025
**Project**: Agent Identity Management (OpenA2A)
**Repository**: https://github.com/opena2a-org/agent-identity-management
