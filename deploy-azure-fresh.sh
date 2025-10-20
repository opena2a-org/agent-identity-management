#!/bin/bash
set -e

# AIM Production Deployment Script - Fresh Install
# This script deploys Agent Identity Management to Azure with automatic database migrations
# Last Updated: October 20, 2025

echo "🚀 Starting AIM production deployment to Azure..."
echo ""

# Configuration
RESOURCE_GROUP="aim-production-rg"
LOCATION="eastus2"
ACR_NAME="aimprodregistry"
DB_SERVER="aim-prod-db-$(date +%s)"
REDIS_NAME="aim-prod-redis-$(date +%s)"
ENV_NAME="aim-prod-env"
BACKEND_APP="aim-backend"
FRONTEND_APP="aim-frontend"

# Secrets
POSTGRES_PASSWORD="AIM2025!Secure"
JWT_SECRET=$(openssl rand -base64 32)
REDIS_PASSWORD=$(openssl rand -base64 32)

echo "📋 Configuration:"
echo "  Resource Group: $RESOURCE_GROUP"
echo "  Location: $LOCATION"
echo "  ACR: $ACR_NAME"
echo ""

# Step 1: Create Resource Group
echo "1️⃣  Creating resource group..."
az group create \
  --name $RESOURCE_GROUP \
  --location $LOCATION

# Step 2: Create Container Registry
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

# Step 3: Build and push Docker images
echo "3️⃣  Building and pushing Docker images..."

echo "  📦 Building backend..."
docker buildx build \
  --platform linux/amd64 \
  -f apps/backend/infrastructure/docker/Dockerfile.backend \
  -t $ACR_NAME.azurecr.io/aim-backend:latest \
  --push .

echo "  📦 Building frontend..."
docker buildx build \
  --platform linux/amd64 \
  -f apps/backend/infrastructure/docker/Dockerfile.frontend \
  -t $ACR_NAME.azurecr.io/aim-frontend:latest \
  --push .

# Step 4: Create PostgreSQL Database
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

# Step 5: Create Redis Cache
echo "5️⃣  Creating Redis cache..."
az redis create \
  --name $REDIS_NAME \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION \
  --sku Basic \
  --vm-size c0

# Get Redis host and password
REDIS_HOST=$(az redis show --name $REDIS_NAME --resource-group $RESOURCE_GROUP --query hostName -o tsv)
REDIS_PASSWORD=$(az redis list-keys --name $REDIS_NAME --resource-group $RESOURCE_GROUP --query primaryKey -o tsv)

# Step 6: Create Container Apps Environment
echo "6️⃣  Creating Container Apps environment..."
az containerapp env create \
  --name $ENV_NAME \
  --resource-group $RESOURCE_GROUP \
  --location $LOCATION

# Step 7: Deploy Backend Container App
echo "7️⃣  Deploying backend container app..."
az containerapp create \
  --name $BACKEND_APP \
  --resource-group $RESOURCE_GROUP \
  --environment $ENV_NAME \
  --image $ACR_NAME.azurecr.io/aim-backend:latest \
  --target-port 8080 \
  --ingress external \
  --registry-server $ACR_NAME.azurecr.io \
  --registry-username $ACR_USERNAME \
  --registry-password $ACR_PASSWORD \
  --cpu 0.5 \
  --memory 1Gi \
  --min-replicas 1 \
  --max-replicas 3 \
  --env-vars \
    ENVIRONMENT=production \
    PORT=8080 \
    LOG_LEVEL=info \
    POSTGRES_HOST=$DB_HOST \
    POSTGRES_PORT=5432 \
    POSTGRES_USER=aimadmin \
    POSTGRES_PASSWORD=$POSTGRES_PASSWORD \
    POSTGRES_DB=identity \
    POSTGRES_SSL_MODE=require \
    REDIS_HOST=$REDIS_HOST \
    REDIS_PORT=6380 \
    REDIS_PASSWORD=$REDIS_PASSWORD \
    JWT_SECRET=$JWT_SECRET \
    ALLOWED_ORIGINS=* \
    FRONTEND_URL=https://aim-frontend.wittydesert-756d026f.eastus2.azurecontainerapps.io

# Get backend URL
BACKEND_URL=$(az containerapp show \
  --name $BACKEND_APP \
  --resource-group $RESOURCE_GROUP \
  --query properties.configuration.ingress.fqdn -o tsv)

# Step 8: Deploy Frontend Container App
echo "8️⃣  Deploying frontend container app..."
az containerapp create \
  --name $FRONTEND_APP \
  --resource-group $RESOURCE_GROUP \
  --environment $ENV_NAME \
  --image $ACR_NAME.azurecr.io/aim-frontend:latest \
  --target-port 3000 \
  --ingress external \
  --registry-server $ACR_NAME.azurecr.io \
  --registry-username $ACR_USERNAME \
  --registry-password $ACR_PASSWORD \
  --cpu 0.5 \
  --memory 1Gi \
  --min-replicas 1 \
  --max-replicas 3

# Get frontend URL
FRONTEND_URL=$(az containerapp show \
  --name $FRONTEND_APP \
  --resource-group $RESOURCE_GROUP \
  --query properties.configuration.ingress.fqdn -o tsv)

# Step 9: Verify deployment
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
echo "🔐 Credentials (SAVE THESE!):"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Postgres Admin: aimadmin"
echo "Postgres Pass:  $POSTGRES_PASSWORD"
echo "JWT Secret:     $JWT_SECRET"
echo "Redis Password: $REDIS_PASSWORD"
echo ""
echo "🔍 Next Steps:"
echo "1. Check backend logs for migration success:"
echo "   az containerapp logs show --name $BACKEND_APP --resource-group $RESOURCE_GROUP --follow"
echo ""
echo "2. Verify migrations ran:"
echo "   Should see: ✅ Database migrations completed successfully"
echo ""
echo "3. Test login at https://$FRONTEND_URL"
echo ""
