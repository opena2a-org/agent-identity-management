#!/bin/bash
set -e

echo "=========================================="
echo "🚀 AIM Production Deployment Script"
echo "=========================================="
echo ""

# Configuration
ACR_NAME="aimdemoregistry"
RESOURCE_GROUP="aim-demo-rg"
BACKEND_APP="aim-backend"
FRONTEND_APP="aim-frontend"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Step 1: Verify we're on main branch
echo "📋 Step 1: Verifying branch..."
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo -e "${RED}❌ Error: Not on main branch (currently on $CURRENT_BRANCH)${NC}"
    echo "Please switch to main: git checkout main"
    exit 1
fi
echo -e "${GREEN}✅ On main branch${NC}"
echo ""

# Step 2: Pull latest changes
echo "📋 Step 2: Pulling latest changes..."
git pull origin main
echo -e "${GREEN}✅ Latest changes pulled${NC}"
echo ""

# Step 3: Get commit hash for tagging
COMMIT_HASH=$(git rev-parse --short HEAD)
echo "📋 Commit hash: $COMMIT_HASH"
echo ""

# Step 4: Login to ACR
echo "📋 Step 3: Logging into Azure Container Registry..."
az acr login --name $ACR_NAME
echo -e "${GREEN}✅ Logged into ACR${NC}"
echo ""

# Step 5: Build and push backend image (NO CACHE)
echo "📋 Step 4: Building backend Docker image (no cache)..."
echo "   Platform: linux/amd64 (Azure compatible)"
echo "   Tag: latest, $COMMIT_HASH"
docker buildx build --platform linux/amd64 \
  --no-cache \
  -t $ACR_NAME.azurecr.io/aim-backend:latest \
  -t $ACR_NAME.azurecr.io/aim-backend:$COMMIT_HASH \
  -f apps/backend/infrastructure/docker/Dockerfile.backend \
  --push .
echo -e "${GREEN}✅ Backend image built and pushed${NC}"
echo ""

# Step 6: Build and push frontend image (NO CACHE)
echo "📋 Step 5: Building frontend Docker image (no cache)..."
echo "   Platform: linux/amd64 (Azure compatible)"
echo "   Tag: latest, $COMMIT_HASH"
docker buildx build --platform linux/amd64 \
  --no-cache \
  -t $ACR_NAME.azurecr.io/aim-frontend:latest \
  -t $ACR_NAME.azurecr.io/aim-frontend:$COMMIT_HASH \
  -f apps/backend/infrastructure/docker/Dockerfile.frontend \
  --push .
echo -e "${GREEN}✅ Frontend image built and pushed${NC}"
echo ""

# Step 7: Update backend container app
echo "📋 Step 6: Updating backend container app..."
az containerapp update \
  --name $BACKEND_APP \
  --resource-group $RESOURCE_GROUP \
  --image $ACR_NAME.azurecr.io/aim-backend:latest \
  --output none
echo -e "${GREEN}✅ Backend container updated${NC}"
echo ""

# Step 8: Update frontend container app
echo "📋 Step 7: Updating frontend container app..."
az containerapp update \
  --name $FRONTEND_APP \
  --resource-group $RESOURCE_GROUP \
  --image $ACR_NAME.azurecr.io/aim-frontend:latest \
  --output none
echo -e "${GREEN}✅ Frontend container updated${NC}"
echo ""

# Step 9: Wait for containers to stabilize
echo "📋 Step 8: Waiting for containers to stabilize (30s)..."
sleep 30
echo ""

# Step 10: Verify backend health
echo "📋 Step 9: Verifying backend health..."
BACKEND_URL="https://aim-backend.wittydesert-756d026f.eastus2.azurecontainerapps.io/health"
BACKEND_STATUS=$(curl -s -o /dev/null -w "%{http_code}" $BACKEND_URL)
if [ "$BACKEND_STATUS" = "200" ]; then
    echo -e "${GREEN}✅ Backend health check passed (200 OK)${NC}"
else
    echo -e "${RED}❌ Backend health check failed (HTTP $BACKEND_STATUS)${NC}"
fi
echo ""

# Step 11: Verify frontend
echo "📋 Step 10: Verifying frontend..."
FRONTEND_URL="https://aim-frontend.wittydesert-756d026f.eastus2.azurecontainerapps.io"
FRONTEND_STATUS=$(curl -s -o /dev/null -w "%{http_code}" $FRONTEND_URL)
if [ "$FRONTEND_STATUS" = "200" ]; then
    echo -e "${GREEN}✅ Frontend check passed (200 OK)${NC}"
else
    echo -e "${RED}❌ Frontend check failed (HTTP $FRONTEND_STATUS)${NC}"
fi
echo ""

# Summary
echo "=========================================="
echo "📊 Deployment Summary"
echo "=========================================="
echo "Commit: $COMMIT_HASH"
echo "Backend: $ACR_NAME.azurecr.io/aim-backend:$COMMIT_HASH"
echo "Frontend: $ACR_NAME.azurecr.io/aim-frontend:$COMMIT_HASH"
echo ""
echo "Production URLs:"
echo "  Frontend: $FRONTEND_URL"
echo "  Backend:  $BACKEND_URL"
echo ""
if [ "$BACKEND_STATUS" = "200" ] && [ "$FRONTEND_STATUS" = "200" ]; then
    echo -e "${GREEN}🎉 Deployment successful!${NC}"
else
    echo -e "${YELLOW}⚠️  Deployment completed with warnings. Check logs for details.${NC}"
fi
echo "=========================================="
