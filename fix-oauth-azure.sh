#!/bin/bash

# Fix OAuth Configuration in Azure Container Apps
# This script updates the backend environment variables with real OAuth credentials

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🔧 Fixing OAuth Configuration in Azure${NC}"
echo ""

# Configuration
RESOURCE_GROUP="aim-demo-rg"
BACKEND_APP="aim-backend"
BACKEND_URL="https://aim-backend.wittydesert-756d026f.eastus2.azurecontainerapps.io"

# OAuth Credentials from .env file
GOOGLE_CLIENT_ID="635947637403-53ut3cjn43t6l93tlhq4jq6s60q1ojfv.apps.googleusercontent.com"
GOOGLE_CLIENT_SECRET="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"
GOOGLE_REDIRECT_URI="${BACKEND_URL}/api/v1/oauth/google/callback"

MICROSOFT_CLIENT_ID="2a27d1b1-a6ad-4a6a-8a5f-69ab28db957a"
MICROSOFT_CLIENT_SECRET="9IO8Q~FGCF7SevrTgZlS1Wb~xle9F5r~lz_aWdpo"
MICROSOFT_REDIRECT_URI="${BACKEND_URL}/api/v1/oauth/microsoft/callback"

OKTA_DOMAIN="integrator-3054094.okta.com"
OKTA_CLIENT_ID="0oaw6roiq3teWuiVo697"
OKTA_CLIENT_SECRET="Emm10sqLUdmCDc6qI8IdRKUTRIuAc7yA4DQtiQXZx3ryUy417fvWwZVh_bLwvP1u"
OKTA_REDIRECT_URI="${BACKEND_URL}/api/v1/auth/callback/okta"

echo -e "${YELLOW}📋 Configuration:${NC}"
echo "  Resource Group: $RESOURCE_GROUP"
echo "  Backend App: $BACKEND_APP"
echo "  Backend URL: $BACKEND_URL"
echo ""

echo -e "${YELLOW}🔐 OAuth Providers:${NC}"
echo "  Google Client ID: ${GOOGLE_CLIENT_ID:0:20}..."
echo "  Microsoft Client ID: ${MICROSOFT_CLIENT_ID:0:20}..."
echo "  Okta Domain: $OKTA_DOMAIN"
echo ""

# Update backend container app environment variables
echo -e "${GREEN}📝 Updating backend environment variables...${NC}"

az containerapp update \
  --name "$BACKEND_APP" \
  --resource-group "$RESOURCE_GROUP" \
  --set-env-vars \
    "GOOGLE_CLIENT_ID=$GOOGLE_CLIENT_ID" \
    "GOOGLE_CLIENT_SECRET=$GOOGLE_CLIENT_SECRET" \
    "GOOGLE_REDIRECT_URI=$GOOGLE_REDIRECT_URI" \
    "MICROSOFT_CLIENT_ID=$MICROSOFT_CLIENT_ID" \
    "MICROSOFT_CLIENT_SECRET=$MICROSOFT_CLIENT_SECRET" \
    "MICROSOFT_REDIRECT_URI=$MICROSOFT_REDIRECT_URI" \
    "OKTA_DOMAIN=$OKTA_DOMAIN" \
    "OKTA_CLIENT_ID=$OKTA_CLIENT_ID" \
    "OKTA_CLIENT_SECRET=$OKTA_CLIENT_SECRET" \
    "OKTA_REDIRECT_URI=$OKTA_REDIRECT_URI" \
  --output table

echo ""
echo -e "${GREEN}✅ OAuth configuration updated!${NC}"
echo ""
echo -e "${YELLOW}⚠️  IMPORTANT: Update OAuth Provider Redirect URIs${NC}"
echo ""
echo "You need to add these redirect URIs to your OAuth provider settings:"
echo ""
echo -e "${GREEN}Google Cloud Console:${NC}"
echo "  1. Go to: https://console.cloud.google.com/apis/credentials"
echo "  2. Edit OAuth 2.0 Client ID: ${GOOGLE_CLIENT_ID:0:20}..."
echo "  3. Add Authorized redirect URI: $GOOGLE_REDIRECT_URI"
echo ""
echo -e "${GREEN}Microsoft Azure Portal:${NC}"
echo "  1. Go to: https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade"
echo "  2. Find App: $MICROSOFT_CLIENT_ID"
echo "  3. Add Redirect URI: $MICROSOFT_REDIRECT_URI"
echo ""
echo -e "${GREEN}Okta Admin Console:${NC}"
echo "  1. Go to: https://$OKTA_DOMAIN/admin/apps"
echo "  2. Find App: $OKTA_CLIENT_ID"
echo "  3. Add Sign-in redirect URI: $OKTA_REDIRECT_URI"
echo ""
echo -e "${GREEN}🔄 Wait 2-3 minutes for container restart, then test:${NC}"
echo "  Frontend: https://aim-frontend.wittydesert-756d026f.eastus2.azurecontainerapps.io/signin"
echo ""
