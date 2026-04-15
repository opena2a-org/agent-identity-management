#!/usr/bin/env bash
set -euo pipefail

# AIM Azure Infrastructure Bootstrap
# Creates all Azure resources for production. Run once, then use deploy-azure.yml for updates.
#
# Prerequisites:
#   - az cli authenticated (az login)
#   - openssl for secret generation
#
# Usage:
#   ./infrastructure/scripts/bootstrap-azure.sh
#
# After running:
#   1. Set AZURE_CREDENTIALS secret in GitHub repo (from output)
#   2. Set AIM_BACKEND_URL in Vercel project (from output)
#   3. Push to main to trigger deploy-azure.yml

RESOURCE_GROUP="${AIM_RESOURCE_GROUP:-aim-production-rg}"
LOCATION="${AIM_LOCATION:-canadacentral}"
ENV_NAME="aim-prod-env"
BACKEND_APP="aim-prod-backend"
FRONTEND_APP="aim-prod-frontend"
SHORT_ID=$(openssl rand -hex 3)
DB_SERVER="aim-prod-db-${SHORT_ID}"
REDIS_NAME="aim-prod-redis-${SHORT_ID}"
BACKEND_IMAGE="ghcr.io/opena2a-org/aim-server:latest"
FRONTEND_IMAGE="ghcr.io/opena2a-org/aim-dashboard:latest"

info()  { printf '\033[1;34m[AIM]\033[0m %s\n' "$*"; }
ok()    { printf '\033[1;32m[AIM]\033[0m %s\n' "$*"; }
warn()  { printf '\033[1;33m[AIM]\033[0m %s\n' "$*"; }
error() { printf '\033[1;31m[AIM]\033[0m %s\n' "$*" >&2; exit 1; }

# --- Pre-flight ---
command -v az >/dev/null 2>&1 || error "Azure CLI not found. Install: https://aka.ms/installazurecli"
command -v openssl >/dev/null 2>&1 || error "openssl not found"
az account show > /dev/null 2>&1 || error "Not logged in to Azure. Run: az login"

SUBSCRIPTION_ID=$(az account show --query id -o tsv)
info "Subscription: $SUBSCRIPTION_ID"
info "Resource Group: $RESOURCE_GROUP"
info "Location: $LOCATION"
echo ""

# --- Generate secrets ---
PG_PASSWORD=$(openssl rand -hex 16)
REDIS_PASSWORD=$(openssl rand -hex 16)
JWT_SECRET=$(openssl rand -hex 32)
KEYVAULT_KEY=$(openssl rand -base64 32)
ADMIN_PASSWORD=$(openssl rand -base64 18)

# --- Step 1: Resource Group ---
info "Creating resource group..."
az group create --name "$RESOURCE_GROUP" --location "$LOCATION" --output none
ok "Resource group ready"

# --- Step 2: PostgreSQL ---
info "Creating PostgreSQL Flexible Server..."
az postgres flexible-server create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$DB_SERVER" \
  --location "$LOCATION" \
  --admin-user postgres \
  --admin-password "$PG_PASSWORD" \
  --sku-name Standard_B1ms \
  --tier Burstable \
  --storage-size 32 \
  --version 16 \
  --yes \
  --output none

# Allow Azure services
az postgres flexible-server firewall-rule create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$DB_SERVER" \
  --rule-name AllowAzure \
  --start-ip-address 0.0.0.0 \
  --end-ip-address 0.0.0.0 \
  --output none

# Create database
az postgres flexible-server db create \
  --resource-group "$RESOURCE_GROUP" \
  --server-name "$DB_SERVER" \
  --database-name identity \
  --output none

DB_HOST="${DB_SERVER}.postgres.database.azure.com"
ok "PostgreSQL ready: $DB_HOST"

# --- Step 3: Redis ---
info "Creating Redis Cache..."
az redis create \
  --resource-group "$RESOURCE_GROUP" \
  --name "$REDIS_NAME" \
  --location "$LOCATION" \
  --sku Basic \
  --vm-size C0 \
  --output none

REDIS_HOST="${REDIS_NAME}.redis.cache.windows.net"
REDIS_KEY=$(az redis list-keys --resource-group "$RESOURCE_GROUP" --name "$REDIS_NAME" --query primaryKey -o tsv)
ok "Redis ready: $REDIS_HOST"

# --- Step 4: Container App Environment ---
info "Creating Container Apps environment..."
az containerapp env create \
  --name "$ENV_NAME" \
  --resource-group "$RESOURCE_GROUP" \
  --location "$LOCATION" \
  --output none
ok "Container Apps environment ready"

# --- Step 5: Backend ---
info "Deploying backend..."
az containerapp create \
  --name "$BACKEND_APP" \
  --resource-group "$RESOURCE_GROUP" \
  --environment "$ENV_NAME" \
  --image "$BACKEND_IMAGE" \
  --target-port 8080 \
  --ingress external \
  --min-replicas 1 \
  --max-replicas 3 \
  --cpu 0.5 \
  --memory 1.0Gi \
  --secrets \
    "pg-password=$PG_PASSWORD" \
    "redis-key=$REDIS_KEY" \
    "jwt-secret=$JWT_SECRET" \
    "keyvault-key=$KEYVAULT_KEY" \
    "admin-password=$ADMIN_PASSWORD" \
  --env-vars \
    "POSTGRES_HOST=$DB_HOST" \
    "POSTGRES_PORT=5432" \
    "POSTGRES_USER=postgres" \
    "POSTGRES_PASSWORD=secretref:pg-password" \
    "POSTGRES_DB=identity" \
    "POSTGRES_SSL_MODE=require" \
    "REDIS_HOST=$REDIS_HOST" \
    "REDIS_PORT=6380" \
    "REDIS_PASSWORD=secretref:redis-key" \
    "REDIS_USE_TLS=true" \
    "JWT_SECRET=secretref:jwt-secret" \
    "KEYVAULT_MASTER_KEY=secretref:keyvault-key" \
    "ADMIN_EMAIL=admin@opena2a.org" \
    "ADMIN_PASSWORD=secretref:admin-password" \
    "ENVIRONMENT=production" \
  --output none

BACKEND_FQDN=$(az containerapp show --name "$BACKEND_APP" --resource-group "$RESOURCE_GROUP" \
  --query "properties.configuration.ingress.fqdn" -o tsv)
BACKEND_URL="https://${BACKEND_FQDN}"
ok "Backend deployed: $BACKEND_URL"

# --- Step 6: Frontend ---
info "Deploying frontend..."
az containerapp create \
  --name "$FRONTEND_APP" \
  --resource-group "$RESOURCE_GROUP" \
  --environment "$ENV_NAME" \
  --image "$FRONTEND_IMAGE" \
  --target-port 3000 \
  --ingress external \
  --min-replicas 1 \
  --max-replicas 3 \
  --cpu 0.5 \
  --memory 1.0Gi \
  --env-vars \
    "NEXT_PUBLIC_API_URL=$BACKEND_URL" \
    "HOSTNAME=0.0.0.0" \
    "NODE_ENV=production" \
  --output none

FRONTEND_FQDN=$(az containerapp show --name "$FRONTEND_APP" --resource-group "$RESOURCE_GROUP" \
  --query "properties.configuration.ingress.fqdn" -o tsv)
ok "Frontend deployed: https://${FRONTEND_FQDN}"

# --- Step 7: Set CORS ---
info "Updating CORS and frontend URL..."
az containerapp update \
  --name "$BACKEND_APP" \
  --resource-group "$RESOURCE_GROUP" \
  --set-env-vars \
    "FRONTEND_URL=https://${FRONTEND_FQDN}" \
    "ALLOWED_ORIGINS=https://${FRONTEND_FQDN},https://aim.opena2a.org" \
  --output none
ok "CORS configured"

# --- Step 8: Health check ---
info "Waiting for backend health..."
for i in $(seq 1 30); do
  if curl -sf "${BACKEND_URL}/health" > /dev/null 2>&1; then
    ok "Backend is healthy"
    break
  fi
  printf "."
  sleep 5
done
echo ""
curl -sf "${BACKEND_URL}/health" || error "Backend health check failed after 150s"
curl -sf "${BACKEND_URL}/api/v1/status" | python3 -m json.tool 2>/dev/null || true

# --- Output ---
echo ""
echo "============================================"
ok "AIM Production Deployment Complete"
echo "============================================"
echo ""
echo "  Backend:   $BACKEND_URL"
echo "  Frontend:  https://${FRONTEND_FQDN}"
echo "  Admin:     admin@opena2a.org / $ADMIN_PASSWORD"
echo ""
echo "  Next steps:"
echo "  1. Set Vercel env var AIM_BACKEND_URL=$BACKEND_URL"
echo "  2. Add custom domain aim-api.opena2a.org to backend Container App"
echo "  3. Create GitHub Actions secret AZURE_CREDENTIALS:"
echo ""
echo "     az ad sp create-for-rbac --name aim-cd \\"
echo "       --role contributor \\"
echo "       --scopes /subscriptions/$SUBSCRIPTION_ID/resourceGroups/$RESOURCE_GROUP \\"
echo "       --sdk-auth"
echo ""

# Save credentials (restricted permissions)
CREDS_FILE="/tmp/aim-production-creds-$(date +%Y%m%d).txt"
(umask 077 && cat > "$CREDS_FILE" << CREDS
AIM Production Credentials ($(date))
=====================================
Backend URL: $BACKEND_URL
Frontend URL: https://${FRONTEND_FQDN}

PostgreSQL:
  Host: $DB_HOST
  Database: identity
  User: postgres
  Password: $PG_PASSWORD

Redis:
  Host: $REDIS_HOST
  Key: $REDIS_KEY

Admin:
  Email: admin@opena2a.org
  Password: $ADMIN_PASSWORD

JWT Secret: $JWT_SECRET
KeyVault Key: $KEYVAULT_KEY
CREDS
)
warn "Credentials saved to $CREDS_FILE (chmod 600)"
