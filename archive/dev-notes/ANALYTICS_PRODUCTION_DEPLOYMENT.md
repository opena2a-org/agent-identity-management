# 🚀 Analytics Production Deployment Guide

## Overview

This guide walks through deploying the enterprise-grade analytics system to production on Azure Container Apps.

**What's Being Deployed**:
- Migration 010: Analytics database tables (api_calls, agent_activity_metrics, etc.)
- Analytics tracking middleware: Real-time API call logging
- Updated analytics handlers: Real data from database
- Automatic aggregation triggers

---

## Prerequisites

✅ Azure CLI installed and logged in
✅ Access to Azure subscription
✅ AIM backend already deployed to Azure Container Apps
✅ Database credentials available

**Production URLs**:
- Backend: https://aim-prod-backend.gentleflower-1d39c80e.canadacentral.azurecontainerapps.io
- Frontend: https://aim-prod-frontend.gentleflower-1d39c80e.canadacentral.azurecontainerapps.io

---

## Deployment Steps

### Step 1: Build New Docker Image with Analytics

```bash
# Navigate to project root
cd /Users/decimai/workspace/agent-identity-management

# Build backend Docker image
docker build \
  -f infrastructure/docker/Dockerfile.backend \
  -t aim-backend:analytics \
  .

# Tag for Azure Container Registry (if using ACR)
# Replace with your ACR name
docker tag aim-backend:analytics <your-acr>.azurecr.io/aim-backend:latest
docker push <your-acr>.azurecr.io/aim-backend:latest
```

### Step 2: Update Azure Container App

Option A: Via Azure Portal
1. Go to Azure Portal → Container Apps
2. Find `aim-prod-backend`
3. Click "Revisions" → "Create new revision"
4. Update image to latest
5. Click "Create"

Option B: Via Azure CLI
```bash
# Get resource group and app name
RESOURCE_GROUP="aim-prod-rg"
APP_NAME="aim-prod-backend"

# Update container app with new image
az containerapp update \
  --name $APP_NAME \
  --resource-group $RESOURCE_GROUP \
  --image <your-acr>.azurecr.io/aim-backend:latest
```

### Step 3: Apply Migration 010

**Option A: Exec into Running Container** (Recommended)

```bash
# Get the container app pod name
RESOURCE_GROUP="aim-prod-rg"
APP_NAME="aim-prod-backend"

# Get revision name
REVISION=$(az containerapp revision list \
  --name $APP_NAME \
  --resource-group $RESOURCE_GROUP \
  --query "[0].name" -o tsv)

# Exec into container
az containerapp exec \
  --name $APP_NAME \
  --resource-group $RESOURCE_GROUP \
  --revision $REVISION \
  --command /bin/sh

# Inside the container, run migration
cd /root
export POSTGRES_HOST="<your-postgres-server>.postgres.database.azure.com"
export POSTGRES_PORT="5432"
export POSTGRES_USER="<your-admin-user>"
export POSTGRES_PASSWORD="<your-password>"
export POSTGRES_DB="identity"
export POSTGRES_SSL_MODE="require"

# Build migration tool (if not pre-built)
# Or copy migration SQL and run manually

# Run migration manually
apk add --no-cache postgresql-client
psql "host=$POSTGRES_HOST port=$POSTGRES_PORT user=$POSTGRES_USER password=$POSTGRES_PASSWORD dbname=$POSTGRES_DB sslmode=$POSTGRES_SSL_MODE" \
  -f migrations/010_create_analytics_tables.sql

# Verify migration applied
psql "host=$POSTGRES_HOST port=$POSTGRES_PORT user=$POSTGRES_USER password=$POSTGRES_PASSWORD dbname=$POSTGRES_DB sslmode=$POSTGRES_SSL_MODE" \
  -c "SELECT * FROM schema_migrations WHERE version LIKE '%010%';"

# Exit container
exit
```

**Option B: Run Migration Locally Against Production DB** (Simpler)

```bash
# From your local machine
cd /Users/decimai/workspace/agent-identity-management/apps/backend

# Set production database connection
export POSTGRES_HOST="<your-postgres-server>.postgres.database.azure.com"
export POSTGRES_PORT="5432"
export POSTGRES_USER="<your-admin-user>@<your-postgres-server>"  # Azure requires @server-name suffix
export POSTGRES_PASSWORD="<your-password>"
export POSTGRES_DB="identity"
export POSTGRES_SSL_MODE="require"

# Run migration tool
go run cmd/migrate/main.go up

# Expected output:
# ⏭️  Skipping 001_create_initial_schema.sql (already applied)
# ⏭️  Skipping 002_add_oauth_tables.sql (already applied)
# ...
# 🔄 Applying 010_create_analytics_tables.sql...
# ✅ Applied 010_create_analytics_tables.sql
# ✅ Migrations completed successfully

# Verify migration status
go run cmd/migrate/main.go status

# Expected output:
# 📋 Migration Status:
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
# ✅ Applied  001_create_initial_schema.sql
# ✅ Applied  002_add_oauth_tables.sql
# ...
# ✅ Applied  010_create_analytics_tables.sql
# ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

### Step 4: Verify Analytics Tables Created

```bash
# Connect to production database
psql "host=$POSTGRES_HOST port=$POSTGRES_PORT user=$POSTGRES_USER password=$POSTGRES_PASSWORD dbname=$POSTGRES_DB sslmode=$POSTGRES_SSL_MODE"

# Check tables exist
\dt

# Should see:
# api_calls
# agent_activity_metrics
# organization_daily_metrics
# trust_score_history

# Check triggers exist
SELECT tgname, tgtype FROM pg_trigger WHERE tgname LIKE '%aggregate%' OR tgname LIKE '%trust_score%';

# Expected:
# trigger_aggregate_agent_metrics
# trigger_log_trust_score

# Exit psql
\q
```

### Step 5: Restart Backend Container App

```bash
# Restart to activate analytics middleware
az containerapp revision restart \
  --name $APP_NAME \
  --resource-group $RESOURCE_GROUP \
  --revision $REVISION

# Monitor logs
az containerapp logs show \
  --name $APP_NAME \
  --resource-group $RESOURCE_GROUP \
  --follow

# Look for successful startup
# Should see: "🚀 Server started on :8080"
```

---

## Verification

### Test 1: Backend Health Check

```bash
curl https://aim-prod-backend.gentleflower-1d39c80e.canadacentral.azurecontainerapps.io/api/v1/status | jq '.'

# Expected:
# {
#   "environment": "production",
#   "status": "operational",
#   "services": {
#     "database": "healthy",
#     "email": "unavailable",
#     "redis": "not configured"
#   },
#   "version": "1.0.0"
# }
```

### Test 2: Make Some API Calls (Generate Analytics Data)

```bash
# Login to get auth token
AUTH_TOKEN=$(curl -s -X POST \
  https://aim-prod-backend.gentleflower-1d39c80e.canadacentral.azurecontainerapps.io/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@opena2a.org",
    "password": "Admin2025!Secure"
  }' | jq -r '.token')

echo "Auth Token: $AUTH_TOKEN"

# Make a few API calls to generate analytics data
for i in {1..10}; do
  curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
    https://aim-prod-backend.gentleflower-1d39c80e.canadacentral.azurecontainerapps.io/api/v1/agents > /dev/null
  echo "API call $i completed"
done
```

### Test 3: Check Analytics Endpoints Return Real Data

```bash
# Get usage statistics
curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
  "https://aim-prod-backend.gentleflower-1d39c80e.canadacentral.azurecontainerapps.io/api/v1/analytics/usage?period=day" \
  | jq '.'

# Expected (after making API calls above):
# {
#   "period": "day",
#   "total_agents": 5,
#   "active_agents": 3,
#   "api_calls": 10,           # ✅ Should be >= 10 (real data!)
#   "data_volume": 0.05,       # ✅ Real data volume in MB
#   "uptime": 99.9
# }

# Get trust score trends
curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
  "https://aim-prod-backend.gentleflower-1d39c80e.canadacentral.azurecontainerapps.io/api/v1/analytics/trust-score-trends?weeks=4" \
  | jq '.'

# Get verification activity
curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
  "https://aim-prod-backend.gentleflower-1d39c80e.canadacentral.azurecontainerapps.io/api/v1/analytics/verification-activity?months=6" \
  | jq '.'

# Get agent activity
curl -s -H "Authorization: Bearer $AUTH_TOKEN" \
  "https://aim-prod-backend.gentleflower-1d39c80e.canadacentral.azurecontainerapps.io/api/v1/analytics/agents/activity?limit=10" \
  | jq '.'

# Expected (after making API calls):
# {
#   "activities": [
#     {
#       "agent_id": "...",
#       "agent_name": "...",
#       "status": "verified",
#       "trust_score": 85.5,
#       "last_active": "2025-10-20T...",
#       "api_calls": 10,        # ✅ Real count!
#       "data_processed": 0.05  # ✅ Real MB!
#     }
#   ],
#   "total": 5
# }
```

### Test 4: Verify Database Tables Have Data

```bash
# Connect to database
psql "host=$POSTGRES_HOST port=$POSTGRES_PORT user=$POSTGRES_USER password=$POSTGRES_PASSWORD dbname=$POSTGRES_DB sslmode=$POSTGRES_SSL_MODE"

# Check api_calls table
SELECT COUNT(*) as total_api_calls FROM api_calls;

# Expected: >= 10 (from test API calls above)

# Check recent API calls
SELECT
  method,
  endpoint,
  status_code,
  duration_ms,
  called_at
FROM api_calls
ORDER BY called_at DESC
LIMIT 10;

# Check agent activity metrics (auto-aggregated)
SELECT
  agent_id,
  hour_timestamp,
  api_calls_count,
  data_processed_bytes
FROM agent_activity_metrics
ORDER BY hour_timestamp DESC
LIMIT 5;

# Check trust score history
SELECT COUNT(*) FROM trust_score_history;

# Exit
\q
```

---

## Troubleshooting

### Issue: Migration Fails with "table already exists"

**Cause**: Migration 010 was partially applied

**Solution**:
```sql
-- Check which tables exist
SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename LIKE '%api_calls%';

-- If api_calls exists, mark migration as applied manually
INSERT INTO schema_migrations (version)
VALUES ('010_create_analytics_tables.sql')
ON CONFLICT (version) DO NOTHING;
```

### Issue: Analytics endpoints return 0 for api_calls

**Cause**: Analytics middleware not activated yet

**Solution**:
1. Verify new Docker image was deployed
2. Restart container app
3. Check logs for middleware registration: `app.Use(middleware.AnalyticsTracking(db))`
4. Make API calls and wait ~1 minute for data to appear

### Issue: No data in agent_activity_metrics

**Cause**: Triggers not working or no authenticated API calls made

**Solution**:
```sql
-- Check if trigger exists
SELECT tgname FROM pg_trigger WHERE tgname = 'trigger_aggregate_agent_metrics';

-- Check if api_calls have agent_id (required for aggregation)
SELECT COUNT(*) FROM api_calls WHERE agent_id IS NOT NULL;

-- Manually test trigger
INSERT INTO api_calls (
  organization_id, agent_id, method, endpoint, status_code,
  duration_ms, called_at
) VALUES (
  '<org-id>', '<agent-id>', 'GET', '/test', 200, 50, NOW()
);

-- Check if aggregation occurred
SELECT * FROM agent_activity_metrics WHERE agent_id = '<agent-id>';
```

### Issue: Database connection refused

**Cause**: Azure PostgreSQL firewall rules

**Solution**:
```bash
# Add your IP to firewall rules
az postgres flexible-server firewall-rule create \
  --resource-group $RESOURCE_GROUP \
  --name <your-postgres-server> \
  --rule-name "allow-my-ip" \
  --start-ip-address <your-ip> \
  --end-ip-address <your-ip>

# Or allow all Azure services
az postgres flexible-server firewall-rule create \
  --resource-group $RESOURCE_GROUP \
  --name <your-postgres-server> \
  --rule-name "allow-azure" \
  --start-ip-address 0.0.0.0 \
  --end-ip-address 0.0.0.0
```

---

## Performance Monitoring

### Monitor Database Size Growth

```sql
-- Check total database size
SELECT pg_size_pretty(pg_database_size('identity'));

-- Check analytics tables size
SELECT
  tablename,
  pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
  AND tablename IN ('api_calls', 'agent_activity_metrics', 'trust_score_history', 'organization_daily_metrics')
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

### Monitor API Call Volume

```sql
-- API calls per hour (last 24 hours)
SELECT
  DATE_TRUNC('hour', called_at) as hour,
  COUNT(*) as api_calls,
  AVG(duration_ms) as avg_duration_ms
FROM api_calls
WHERE called_at >= NOW() - INTERVAL '24 hours'
GROUP BY DATE_TRUNC('hour', called_at)
ORDER BY hour DESC;

-- Top endpoints by call count
SELECT
  endpoint,
  COUNT(*) as calls,
  AVG(duration_ms) as avg_ms,
  MAX(duration_ms) as max_ms
FROM api_calls
WHERE called_at >= NOW() - INTERVAL '7 days'
GROUP BY endpoint
ORDER BY calls DESC
LIMIT 10;
```

### Set Up Data Retention Policy (Optional)

```sql
-- Create function to cleanup old api_calls (keep 90 days)
CREATE OR REPLACE FUNCTION cleanup_old_api_calls()
RETURNS void AS $$
BEGIN
  DELETE FROM api_calls
  WHERE called_at < NOW() - INTERVAL '90 days';
END;
$$ LANGUAGE plpgsql;

-- Schedule via cron or manual execution
-- Note: Azure PostgreSQL Flexible Server supports pg_cron extension
SELECT cleanup_old_api_calls();
```

---

## Success Criteria

✅ Migration 010 applied successfully
✅ All 4 analytics tables created (api_calls, agent_activity_metrics, etc.)
✅ Triggers working (aggregate_agent_metrics, log_trust_score)
✅ Backend container restarted with new image
✅ Analytics middleware activated (tracking API calls)
✅ Analytics endpoints return real data (not 0)
✅ Database contains real api_calls records
✅ Hourly aggregation working in agent_activity_metrics

---

## Rollback Plan

If analytics causes issues, you can rollback:

### Step 1: Remove Analytics Middleware

```bash
# Redeploy previous Docker image without analytics
az containerapp update \
  --name $APP_NAME \
  --resource-group $RESOURCE_GROUP \
  --image <your-acr>.azurecr.io/aim-backend:previous-tag
```

### Step 2: Rollback Migration (Optional)

```sql
-- Connect to database
psql "..."

-- Drop analytics tables (data will be lost!)
DROP TABLE IF EXISTS api_calls CASCADE;
DROP TABLE IF EXISTS agent_activity_metrics CASCADE;
DROP TABLE IF EXISTS organization_daily_metrics CASCADE;
DROP TABLE IF EXISTS trust_score_history CASCADE;

-- Remove migration record
DELETE FROM schema_migrations WHERE version = '010_create_analytics_tables.sql';
```

---

**Last Updated**: October 20, 2025
**Status**: Ready for production deployment
**Estimated Deployment Time**: 15-20 minutes
