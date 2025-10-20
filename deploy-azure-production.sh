#!/bin/bash
set -e

# AIM Production Deployment Script - Automated Azure Deployment
# This script deploys Agent Identity Management to Azure Container Apps with PostgreSQL
# Last Updated: October 20, 2025

echo "🚀 Starting AIM production deployment to Azure..."
echo ""

# ============================================================================
# CONFIGURATION - Modify these values as needed
# ============================================================================

RESOURCE_GROUP="aim-production-rg"
LOCATION="canadacentral"  # canadacentral supports PostgreSQL flexible servers
ACR_NAME="aimprodregistry"
DB_SERVER="aim-prod-db-$(date +%s)"
REDIS_NAME="aim-prod-redis-$(date +%s)"
ENV_NAME="aim-prod-env"
BACKEND_APP="aim-backend"
FRONTEND_APP="aim-frontend"

# Admin Configuration
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@opena2a.org}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-Admin2025!Secure}"
ADMIN_NAME="${ADMIN_NAME:-System Administrator}"
ORG_NAME="${ORG_NAME:-OpenA2A}"
ORG_DOMAIN="${ORG_DOMAIN:-opena2a.org}"

# Secrets
POSTGRES_PASSWORD="AIM2025!Secure"
JWT_SECRET=$(openssl rand -base64 32)
REDIS_PASSWORD=$(openssl rand -base64 32)

echo "📋 Configuration:"
echo "  Resource Group: $RESOURCE_GROUP"
echo "  Location: $LOCATION"
echo "  ACR: $ACR_NAME"
echo ""

# ============================================================================
# STEP 1: Create Resource Group
# ============================================================================
echo "1️⃣  Creating resource group..."
az group create \
  --name $RESOURCE_GROUP \
  --location $LOCATION

# ============================================================================
# STEP 2: Create Azure Container Registry
# ============================================================================
echo "2️⃣  Creating Azure Container Registry..."
az acr create \
  --name $ACR_NAME \
  --resource-group $RESOURCE_GROUP \
  --sku Basic \
  --admin-enabled true

# Get ACR credentials
ACR_USERNAME=$(az acr credential show --name $ACR_NAME --query username -o tsv)
ACR_PASSWORD=$(az acr credential show --name $ACR_NAME --query passwords[0].value -o tsv)

# Login to ACR
echo "🔐 Logging into ACR..."
az acr login --name $ACR_NAME

# ============================================================================
# STEP 3: Build and Push Docker Images
# ============================================================================
echo "3️⃣  Building and pushing Docker images..."

BACKEND_VERSION=$(date +%Y%m%d-%H%M%S)
FRONTEND_VERSION=$(date +%Y%m%d-%H%M%S)

echo "  📦 Building backend (version: $BACKEND_VERSION)..."
docker buildx build \
  --platform linux/amd64 \
  --no-cache \
  -f apps/backend/infrastructure/docker/Dockerfile.backend \
  -t $ACR_NAME.azurecr.io/aim-backend:$BACKEND_VERSION \
  -t $ACR_NAME.azurecr.io/aim-backend:latest \
  --push .

echo "  📦 Building frontend (version: $FRONTEND_VERSION)..."
docker buildx build \
  --platform linux/amd64 \
  --no-cache \
  -f apps/backend/infrastructure/docker/Dockerfile.frontend \
  --build-arg NEXT_PUBLIC_API_URL=https://$BACKEND_APP.yellowbush-08563eac.$LOCATION.azurecontainerapps.io \
  -t $ACR_NAME.azurecr.io/aim-frontend:$FRONTEND_VERSION \
  -t $ACR_NAME.azurecr.io/aim-frontend:latest \
  --push .

# ============================================================================
# STEP 4: Create PostgreSQL Database
# ============================================================================
echo "4️⃣  Creating PostgreSQL database..."
az postgres flexible-server create \
  --name $DB_SERVER \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION \
  --admin-user aimadmin \
  --admin-password "$POSTGRES_PASSWORD" \
  --sku-name Standard_B1ms \
  --tier Burstable \
  --version 16 \
  --storage-size 32 \
  --public-access 0.0.0.0-255.255.255.255

# Create database
az postgres flexible-server db create \
  --resource-group $RESOURCE_GROUP \
  --server-name $DB_SERVER \
  --database-name identity

# Get database host
DB_HOST=$(az postgres flexible-server show \
  --name $DB_SERVER \
  --resource-group $RESOURCE_GROUP \
  --query fullyQualifiedDomainName -o tsv)

# ============================================================================
# STEP 5: Create Redis Cache (Optional - for production caching)
# ============================================================================
echo "5️⃣  Creating Redis cache..."
az redis create \
  --name $REDIS_NAME \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION \
  --sku Basic \
  --vm-size c0

# Get Redis credentials
REDIS_HOST=$(az redis show --name $REDIS_NAME --resource-group $RESOURCE_GROUP --query hostName -o tsv)
REDIS_PASSWORD=$(az redis list-keys --name $REDIS_NAME --resource-group $RESOURCE_GROUP --query primaryKey -o tsv)

# ============================================================================
# STEP 6: Create Container Apps Environment
# ============================================================================
echo "6️⃣  Creating Container Apps environment..."
az containerapp env create \
  --name $ENV_NAME \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION

# ============================================================================
# STEP 7: Deploy Backend Container App
# ============================================================================
echo "7️⃣  Deploying backend container app..."
az containerapp create \
  --name $BACKEND_APP \
  --resource-group $RESOURCE_GROUP \
  --environment $ENV_NAME \
  --image $ACR_NAME.azurecr.io/aim-backend:$BACKEND_VERSION \
  --target-port 8080 \
  --ingress external \
  --registry-server $ACR_NAME.azurecr.io \
  --registry-username $ACR_USERNAME \
  --registry-password "$ACR_PASSWORD" \
  --cpu 0.5 \
  --memory 1Gi \
  --min-replicas 1 \
  --max-replicas 3 \
  --env-vars \
    "ENVIRONMENT=production" \
    "PORT=8080" \
    "LOG_LEVEL=info" \
    "POSTGRES_HOST=$DB_HOST" \
    "POSTGRES_PORT=5432" \
    "POSTGRES_USER=aimadmin" \
    "POSTGRES_PASSWORD=$POSTGRES_PASSWORD" \
    "POSTGRES_DB=identity" \
    "POSTGRES_SSL_MODE=require" \
    "REDIS_HOST=$REDIS_HOST" \
    "REDIS_PORT=6380" \
    "REDIS_PASSWORD=$REDIS_PASSWORD" \
    "JWT_SECRET=$JWT_SECRET" \
    "ALLOWED_ORIGINS=https://$FRONTEND_APP.yellowbush-08563eac.$LOCATION.azurecontainerapps.io" \
    "FRONTEND_URL=https://$FRONTEND_APP.yellowbush-08563eac.$LOCATION.azurecontainerapps.io"

# Get backend URL
BACKEND_URL=$(az containerapp show \
  --name $BACKEND_APP \
  --resource-group $RESOURCE_GROUP \
  --query properties.configuration.ingress.fqdn -o tsv)

# ============================================================================
# STEP 8: Deploy Frontend Container App
# ============================================================================
echo "8️⃣  Deploying frontend container app..."
az containerapp create \
  --name $FRONTEND_APP \
  --resource-group $RESOURCE_GROUP \
  --environment $ENV_NAME \
  --image $ACR_NAME.azurecr.io/aim-frontend:$FRONTEND_VERSION \
  --target-port 3000 \
  --ingress external \
  --registry-server $ACR_NAME.azurecr.io \
  --registry-username $ACR_USERNAME \
  --registry-password "$ACR_PASSWORD" \
  --cpu 0.5 \
  --memory 1Gi \
  --min-replicas 1 \
  --max-replicas 3 \
  --env-vars \
    "NEXT_PUBLIC_API_URL=https://$BACKEND_URL"

# Get frontend URL
FRONTEND_URL=$(az containerapp show \
  --name $FRONTEND_APP \
  --resource-group $RESOURCE_GROUP \
  --query properties.configuration.ingress.fqdn -o tsv)

# ============================================================================
# STEP 9: Bootstrap Admin User
# ============================================================================
echo ""
echo "9️⃣  Bootstrapping admin user..."
echo "  Waiting for backend to be ready..."
sleep 15

# Build bootstrap binary
echo "  Building bootstrap tool..."
cd apps/backend
go build -o /tmp/aim-bootstrap ./cmd/bootstrap
cd ../..

# Run bootstrap
DATABASE_URL="postgresql://aimadmin:$POSTGRES_PASSWORD@$DB_HOST:5432/identity?sslmode=require"
/tmp/aim-bootstrap \
  --admin-email "$ADMIN_EMAIL" \
  --admin-password "$ADMIN_PASSWORD" \
  --admin-name "$ADMIN_NAME" \
  --org-name "$ORG_NAME" \
  --org-domain "$ORG_DOMAIN" \
  --database-url "$DATABASE_URL" \
  --yes

# Clean up
rm /tmp/aim-bootstrap

# ============================================================================
# DEPLOYMENT COMPLETE
# ============================================================================
echo ""
echo "✅ Deployment complete!"
echo ""
echo "📋 Deployment Summary:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Frontend URL:  https://$FRONTEND_URL"
echo "Backend URL:   https://$BACKEND_URL"
echo "Database:      $DB_HOST"
echo "Redis:         $REDIS_HOST"
echo ""
echo "🔐 Admin Credentials (SAVE THESE!):"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Email:    $ADMIN_EMAIL"
echo "Password: $ADMIN_PASSWORD"
echo ""
echo "🔐 Database Credentials:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Postgres User: aimadmin"
echo "Postgres Pass: $POSTGRES_PASSWORD"
echo "JWT Secret:    $JWT_SECRET"
echo "Redis Pass:    $REDIS_PASSWORD"
echo ""
echo "🎯 Next Steps:"
echo "1. Visit https://$FRONTEND_URL and sign in"
echo "2. Change the default admin password immediately"
echo "3. Configure OAuth providers (Google, Microsoft, etc.)"
echo "4. Review security settings and CORS configuration"
echo ""
echo "📚 Documentation:"
echo "  - User Guide: https://github.com/opena2a-org/agent-identity-management"
echo "  - API Docs:   https://$BACKEND_URL/swagger"
echo ""
