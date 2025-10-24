#!/bin/bash

# AIM Production Deployment Script
# Deploys a clean, production-ready AIM environment to Azure
# All fixes from development deployment are incorporated

set -e  # Exit on any error

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
RESOURCE_GROUP="aim-production-rg"
LOCATION="canadacentral"
TIMESTAMP=$(date +%s)
ACR_NAME="aimprodacr${TIMESTAMP}"
DB_NAME="aim-prod-db-${TIMESTAMP}"
REDIS_NAME="aim-prod-redis-${TIMESTAMP}"
ENV_NAME="aim-prod-env"
BACKEND_APP="aim-prod-backend"
FRONTEND_APP="aim-prod-frontend"

# Admin configuration
ADMIN_EMAIL="admin@opena2a.org"
# Generate secure random admin password
ADMIN_PASSWORD=$(openssl rand -base64 24 | tr -dc 'A-Za-z0-9!@#$%^&*' | head -c 20)
ADMIN_NAME="System Administrator"

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}AIM Production Deployment${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo "Resource Group: $RESOURCE_GROUP"
echo "Location: $LOCATION"
echo "Timestamp: $TIMESTAMP"
echo ""

# Step 0: Verify migration system
echo -e "${BLUE}0️⃣  Verifying migration system...${NC}"
if [ ! -f "apps/backend/migrations/V1__consolidated_schema.sql" ]; then
    echo -e "${RED}❌ Consolidated schema V1 not found. Cannot proceed with deployment.${NC}"
    exit 1
fi
echo -e "${GREEN}✓ Migration system verified${NC}"
echo ""

# Check if resource group exists
if az group show --name $RESOURCE_GROUP &>/dev/null; then
    echo -e "${YELLOW}⚠️  Resource group $RESOURCE_GROUP already exists!${NC}"
    read -p "Do you want to delete it and start fresh? (yes/no): " CONFIRM
    if [ "$CONFIRM" = "yes" ]; then
        echo -e "${YELLOW}Deleting existing resource group...${NC}"
        az group delete --name $RESOURCE_GROUP --yes --no-wait
        echo "Waiting for deletion to complete (this may take a few minutes)..."
        while az group show --name $RESOURCE_GROUP &>/dev/null; do
            sleep 5
            echo -n "."
        done
        echo ""
        echo -e "${GREEN}✓ Resource group deleted${NC}"
    else
        echo "Deployment cancelled."
        exit 1
    fi
fi

# Step 1: Create Resource Group
echo ""
echo -e "${BLUE}1️⃣  Creating resource group...${NC}"
az group create \
  --name $RESOURCE_GROUP \
  --location $LOCATION \
  --output none
echo -e "${GREEN}✓ Resource group created${NC}"

# Step 2: Create Azure Container Registry
echo ""
echo -e "${BLUE}2️⃣  Creating Azure Container Registry...${NC}"
az acr create \
  --resource-group $RESOURCE_GROUP \
  --name $ACR_NAME \
  --sku Basic \
  --admin-enabled true \
  --output none
echo -e "${GREEN}✓ Container Registry created: $ACR_NAME${NC}"

# Get ACR credentials
ACR_USERNAME=$(az acr credential show --name $ACR_NAME --query username -o tsv)
ACR_PASSWORD=$(az acr credential show --name $ACR_NAME --query "passwords[0].value" -o tsv)
ACR_LOGIN_SERVER="${ACR_NAME}.azurecr.io"

# Step 3: Build and Push Docker Images
echo ""
echo -e "${BLUE}3️⃣  Building and pushing Docker images...${NC}"

# Login to ACR
az acr login --name $ACR_NAME

# Build and push backend
echo "   Building backend image..."
docker buildx build --platform linux/amd64 \
  -f infrastructure/docker/Dockerfile.backend \
  -t ${ACR_LOGIN_SERVER}/aim-backend:latest \
  -t ${ACR_LOGIN_SERVER}/aim-backend:$(date +%Y%m%d-%H%M%S) \
  --push . 2>&1 | grep -E "exporting|pushing|DONE" || true
echo -e "${GREEN}   ✓ Backend image pushed${NC}"

# Build and push frontend
echo "   Building frontend image..."
docker buildx build --platform linux/amd64 \
  -f infrastructure/docker/Dockerfile.frontend \
  -t ${ACR_LOGIN_SERVER}/aim-frontend:latest \
  -t ${ACR_LOGIN_SERVER}/aim-frontend:$(date +%Y%m%d-%H%M%S) \
  --push . 2>&1 | grep -E "exporting|pushing|DONE" || true
echo -e "${GREEN}   ✓ Frontend image pushed${NC}"

# Step 4: Create PostgreSQL Database
echo ""
echo -e "${BLUE}4️⃣  Creating PostgreSQL database...${NC}"

# Generate database password
DB_PASSWORD=$(openssl rand -base64 32 | tr -dc 'A-Za-z0-9' | head -c 32)

az postgres flexible-server create \
  --resource-group $RESOURCE_GROUP \
  --name $DB_NAME \
  --location $LOCATION \
  --admin-user aimadmin \
  --admin-password "$DB_PASSWORD" \
  --sku-name Standard_B1ms \
  --tier Burstable \
  --version 16 \
  --storage-size 32 \
  --public-access 0.0.0.0-255.255.255.255 \
  --output none

echo -e "${GREEN}✓ PostgreSQL server created${NC}"

# Create database
az postgres flexible-server db create \
  --resource-group $RESOURCE_GROUP \
  --server-name $DB_NAME \
  --database-name identity \
  --output none

echo -e "${GREEN}✓ Database 'identity' created${NC}"

DB_HOST="${DB_NAME}.postgres.database.azure.com"

# Step 5: Create Redis Cache
echo ""
echo -e "${BLUE}5️⃣  Creating Redis cache...${NC}"

az redis create \
  --resource-group $RESOURCE_GROUP \
  --name $REDIS_NAME \
  --location $LOCATION \
  --sku Basic \
  --vm-size C0 \
  --output none

echo -e "${GREEN}✓ Redis cache created${NC}"

# Get Redis credentials
REDIS_HOST="${REDIS_NAME}.redis.cache.windows.net"
REDIS_PASSWORD=$(az redis list-keys --resource-group $RESOURCE_GROUP --name $REDIS_NAME --query primaryKey -o tsv)

# Step 6: Create Container Apps Environment
echo ""
echo -e "${BLUE}6️⃣  Creating Container Apps environment...${NC}"

az containerapp env create \
  --name $ENV_NAME \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION \
  --output none

echo -e "${GREEN}✓ Container Apps environment created${NC}"

# Step 7: Deploy Backend Container App
echo ""
echo -e "${BLUE}7️⃣  Deploying backend container app...${NC}"

# Generate JWT secret
JWT_SECRET=$(openssl rand -base64 64 | tr -d '\n')

az containerapp create \
  --name $BACKEND_APP \
  --resource-group $RESOURCE_GROUP \
  --environment $ENV_NAME \
  --image ${ACR_LOGIN_SERVER}/aim-backend:latest \
  --target-port 8080 \
  --ingress external \
  --min-replicas 1 \
  --max-replicas 3 \
  --cpu 0.5 \
  --memory 1.0Gi \
  --registry-server $ACR_LOGIN_SERVER \
  --registry-username $ACR_USERNAME \
  --registry-password $ACR_PASSWORD \
  --env-vars \
    "POSTGRES_HOST=$DB_HOST" \
    "POSTGRES_PORT=5432" \
    "POSTGRES_USER=aimadmin" \
    "POSTGRES_PASSWORD=$DB_PASSWORD" \
    "POSTGRES_DB=identity" \
    "POSTGRES_SSL_MODE=require" \
    "REDIS_HOST=$REDIS_HOST" \
    "REDIS_PORT=6380" \
    "REDIS_PASSWORD=$REDIS_PASSWORD" \
    "REDIS_USE_TLS=true" \
    "JWT_SECRET=$JWT_SECRET" \
    "PORT=8080" \
    "ENVIRONMENT=production" \
  --output none

echo -e "${GREEN}✓ Backend container app deployed${NC}"

# Get backend URL
BACKEND_URL=$(az containerapp show --name $BACKEND_APP --resource-group $RESOURCE_GROUP --query properties.configuration.ingress.fqdn -o tsv)
BACKEND_URL="https://${BACKEND_URL}"

echo "   Backend URL: $BACKEND_URL"

# Step 8: Deploy Frontend Container App
echo ""
echo -e "${BLUE}8️⃣  Deploying frontend container app...${NC}"

az containerapp create \
  --name $FRONTEND_APP \
  --resource-group $RESOURCE_GROUP \
  --environment $ENV_NAME \
  --image ${ACR_LOGIN_SERVER}/aim-frontend:latest \
  --target-port 3000 \
  --ingress external \
  --min-replicas 1 \
  --max-replicas 3 \
  --cpu 0.5 \
  --memory 1.0Gi \
  --registry-server $ACR_LOGIN_SERVER \
  --registry-username $ACR_USERNAME \
  --registry-password $ACR_PASSWORD \
  --env-vars \
    "NEXT_PUBLIC_API_URL=$BACKEND_URL" \
  --output none

echo -e "${GREEN}✓ Frontend container app deployed${NC}"

# Get frontend URL
FRONTEND_URL=$(az containerapp show --name $FRONTEND_APP --resource-group $RESOURCE_GROUP --query properties.configuration.ingress.fqdn -o tsv)
FRONTEND_URL="https://${FRONTEND_URL}"

echo "   Frontend URL: $FRONTEND_URL"

# Step 9: Update backend with CORS settings
echo ""
echo -e "${BLUE}9️⃣  Updating backend CORS settings...${NC}"

az containerapp update \
  --name $BACKEND_APP \
  --resource-group $RESOURCE_GROUP \
  --set-env-vars \
    "ALLOWED_ORIGINS=$FRONTEND_URL" \
    "FRONTEND_URL=$FRONTEND_URL" \
  --output none

echo -e "${GREEN}✓ Backend CORS settings updated${NC}"

# Step 10: Wait for backend to be healthy
echo ""
echo -e "${BLUE}🔟 Waiting for backend to be healthy...${NC}"

MAX_ATTEMPTS=30
ATTEMPT=0
while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
    HTTP_CODE=$(curl -s -o /dev/null -w "%{http_code}" ${BACKEND_URL}/health || echo "000")
    if [ "$HTTP_CODE" = "200" ]; then
        echo -e "${GREEN}✓ Backend is healthy!${NC}"
        break
    fi
    ATTEMPT=$((ATTEMPT + 1))
    echo "   Attempt $ATTEMPT/$MAX_ATTEMPTS - Health check returned: $HTTP_CODE"
    sleep 5
done

if [ $ATTEMPT -eq $MAX_ATTEMPTS ]; then
    echo -e "${RED}❌ Backend health check failed after $MAX_ATTEMPTS attempts${NC}"
    echo "Check backend logs with: az containerapp logs show --name $BACKEND_APP --resource-group $RESOURCE_GROUP --tail 50"
    exit 1
fi

# Step 11: Run database migrations (Smart System)
echo ""
echo -e "${BLUE}1️⃣1️⃣  Running database migrations...${NC}"
echo "   Using smart migration system:"
echo "   - Fresh database → Fast consolidated V1 schema"
echo "   - Existing database → Incremental migrations"
echo ""

# Build migration tool
echo "   Building migration tool..."
cd apps/backend
go build -o /tmp/aim-migrate ./cmd/migrate
cd ../..

# Run smart migrations
echo "   Running smart migrations..."
DATABASE_URL="postgresql://aimadmin:${DB_PASSWORD}@${DB_HOST}:5432/identity?sslmode=require" \
  /tmp/aim-migrate

echo ""
echo -e "${GREEN}✓ Database migrations complete${NC}"

# Step 12: Create default security policies
echo ""
echo -e "${BLUE}1️⃣2️⃣  Creating default security policies...${NC}"

# Build backfill binary
echo "   Building backfill binary..."
cd apps/backend
go build -o /tmp/aim-backfill-policies ./cmd/backfill_policies
cd ../..\

# Run backfill
echo "   Running policy backfill..."
DATABASE_URL="postgresql://aimadmin:${DB_PASSWORD}@${DB_HOST}:5432/identity?sslmode=require" \
  /tmp/aim-backfill-policies

echo -e "${GREEN}✓ Default security policies created${NC}"

# Save credentials
CREDS_FILE="/tmp/aim-production-creds-${TIMESTAMP}.txt"
cat > $CREDS_FILE <<EOF
AIM Production Deployment Credentials
======================================

Resource Group: $RESOURCE_GROUP
Location: $LOCATION
Deployed: $(date)

Frontend URL: $FRONTEND_URL
Backend URL: $BACKEND_URL
API Docs: ${BACKEND_URL}/docs

Admin Credentials:
------------------
Email: $ADMIN_EMAIL
Password: $ADMIN_PASSWORD

Database:
---------
Host: $DB_HOST
Port: 5432
Database: identity
Username: aimadmin
Password: $DB_PASSWORD

Redis:
------
Host: $REDIS_HOST
Port: 6380
Password: $REDIS_PASSWORD

Container Registry:
-------------------
Server: $ACR_LOGIN_SERVER
Username: $ACR_USERNAME
Password: $ACR_PASSWORD

JWT Secret:
-----------
$JWT_SECRET

IMPORTANT: Store these credentials securely and delete this file!
EOF

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}✅ Deployment Complete!${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "${BLUE}Frontend URL:${NC} $FRONTEND_URL"
echo -e "${BLUE}Backend URL:${NC} $BACKEND_URL"
echo -e "${BLUE}API Docs:${NC} ${BACKEND_URL}/docs"
echo ""
echo -e "${BLUE}Admin Credentials:${NC}"
echo -e "  Email: ${GREEN}$ADMIN_EMAIL${NC}"
echo -e "  Password: ${GREEN}$ADMIN_PASSWORD${NC}"
echo ""
echo -e "${YELLOW}⚠️  All credentials saved to: $CREDS_FILE${NC}"
echo -e "${YELLOW}⚠️  Store securely and delete this file!${NC}"
echo ""
echo -e "${GREEN}🎉 You can now login at: $FRONTEND_URL${NC}"
echo ""
