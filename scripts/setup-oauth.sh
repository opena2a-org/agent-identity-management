#!/bin/bash

# OAuth Setup Automation Script
# This script automatically creates Google OAuth credentials and configures the backend

set -e  # Exit on any error

echo "🔐 Agent Identity Management - OAuth Setup Automation"
echo "===================================================="
echo ""

# Check if gcloud is installed
if ! command -v gcloud &> /dev/null; then
    echo "❌ Error: gcloud CLI is not installed"
    echo ""
    echo "Please install gcloud CLI first:"
    echo "  brew install --cask google-cloud-sdk"
    echo "  OR"
    echo "  Visit: https://cloud.google.com/sdk/docs/install"
    exit 1
fi

# Check if user is logged in
if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" &> /dev/null; then
    echo "📝 Please log in to Google Cloud..."
    gcloud auth login
fi

# Get or create project
echo ""
echo "📦 Setting up Google Cloud Project..."
PROJECT_ID=$(gcloud config get-value project 2>/dev/null || echo "")

if [ -z "$PROJECT_ID" ]; then
    echo "No project selected. Creating new project..."
    PROJECT_ID="aim-$(date +%s)"
    gcloud projects create $PROJECT_ID --name="Agent Identity Management"
    gcloud config set project $PROJECT_ID
    echo "✅ Created project: $PROJECT_ID"
else
    echo "✅ Using existing project: $PROJECT_ID"
fi

# Enable required APIs
echo ""
echo "🔧 Enabling required Google Cloud APIs..."
gcloud services enable iamcredentials.googleapis.com
gcloud services enable cloudresourcemanager.googleapis.com
echo "✅ APIs enabled"

# Create OAuth consent screen (if not exists)
echo ""
echo "🎨 Configuring OAuth consent screen..."
# Note: This requires manual setup in Google Cloud Console for the first time
# We'll provide instructions instead

echo ""
echo "⚠️  MANUAL STEP REQUIRED (one-time setup):"
echo "   1. Go to: https://console.cloud.google.com/apis/credentials/consent?project=$PROJECT_ID"
echo "   2. Select 'External' user type"
echo "   3. Fill in:"
echo "      - App name: Agent Identity Management"
echo "      - User support email: [your-email]"
echo "      - Developer contact: [your-email]"
echo "   4. Click 'Save and Continue' through all steps"
echo ""
read -p "Press Enter after completing OAuth consent screen setup..."

# Create OAuth 2.0 Client ID
echo ""
echo "🔑 Creating OAuth 2.0 Client ID..."

# Create the OAuth client
OAUTH_CLIENT_JSON=$(gcloud alpha iap oauth-clients create \
    --project=$PROJECT_ID \
    --display_name="Agent Identity Management" \
    --format=json 2>/dev/null || echo "{}")

if [ "$OAUTH_CLIENT_JSON" == "{}" ]; then
    echo "⚠️  Alternative method: Creating via OAuth brands..."

    # Try creating via brands API
    BRAND=$(gcloud alpha iap oauth-brands list --project=$PROJECT_ID --format="value(name)" 2>/dev/null | head -1 || echo "")

    if [ -z "$BRAND" ]; then
        echo "❌ Could not create OAuth client automatically"
        echo ""
        echo "📝 MANUAL STEPS:"
        echo "   1. Go to: https://console.cloud.google.com/apis/credentials?project=$PROJECT_ID"
        echo "   2. Click 'Create Credentials' → 'OAuth client ID'"
        echo "   3. Application type: 'Web application'"
        echo "   4. Name: 'Agent Identity Management'"
        echo "   5. Authorized redirect URIs:"
        echo "      - http://localhost:8080/api/v1/auth/callback/google"
        echo "      - http://localhost:3000/api/auth/callback/google"
        echo "   6. Click 'Create'"
        echo "   7. Copy the Client ID and Client Secret"
        echo ""
        read -p "Enter Client ID: " CLIENT_ID
        read -p "Enter Client Secret: " CLIENT_SECRET
    fi
else
    CLIENT_ID=$(echo $OAUTH_CLIENT_JSON | jq -r '.name' | awk -F'/' '{print $NF}')
    CLIENT_SECRET=$(echo $OAUTH_CLIENT_JSON | jq -r '.secret')
fi

# Validate credentials
if [ -z "$CLIENT_ID" ] || [ -z "$CLIENT_SECRET" ]; then
    echo "❌ Error: Failed to get OAuth credentials"
    exit 1
fi

echo "✅ OAuth Client ID created successfully!"
echo ""

# Update backend .env file
ENV_FILE="/Users/decimai/workspace/agent-identity-management/apps/backend/.env"

echo "📝 Updating backend .env file..."

# Backup existing .env
if [ -f "$ENV_FILE" ]; then
    cp "$ENV_FILE" "$ENV_FILE.backup.$(date +%s)"
    echo "✅ Backed up existing .env"
fi

# Update or add OAuth credentials
if grep -q "GOOGLE_CLIENT_ID=" "$ENV_FILE" 2>/dev/null; then
    # Update existing
    sed -i '' "s|GOOGLE_CLIENT_ID=.*|GOOGLE_CLIENT_ID=$CLIENT_ID|" "$ENV_FILE"
    sed -i '' "s|GOOGLE_CLIENT_SECRET=.*|GOOGLE_CLIENT_SECRET=$CLIENT_SECRET|" "$ENV_FILE"
else
    # Add new
    echo "" >> "$ENV_FILE"
    echo "# Google OAuth (auto-configured)" >> "$ENV_FILE"
    echo "GOOGLE_CLIENT_ID=$CLIENT_ID" >> "$ENV_FILE"
    echo "GOOGLE_CLIENT_SECRET=$CLIENT_SECRET" >> "$ENV_FILE"
    echo "GOOGLE_REDIRECT_URL=http://localhost:8080/api/v1/auth/callback/google" >> "$ENV_FILE"
fi

echo "✅ Backend .env updated with OAuth credentials"
echo ""

# Verify .env file
echo "📋 Current OAuth configuration:"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
grep "GOOGLE_" "$ENV_FILE" || echo "No Google OAuth config found"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

echo "🎉 OAuth setup complete!"
echo ""
echo "Next steps:"
echo "  1. Run database migrations: cd apps/backend && go run cmd/migrate/main.go up"
echo "  2. Start backend: cd apps/backend && go run cmd/server/main.go"
echo "  3. Start frontend: cd apps/web && npm run dev"
echo "  4. Open: http://localhost:3000"
echo ""
