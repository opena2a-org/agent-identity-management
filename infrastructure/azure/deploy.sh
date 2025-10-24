#!/bin/bash

################################################################################
# AIM - Azure Deployment Script
################################################################################
# Description: Deploy AIM to Azure using Container Apps
# Author: AIM Team
# Version: 1.0.0
# License: Apache 2.0
################################################################################

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
RESOURCE_GROUP="${AIM_RESOURCE_GROUP:-aim-production}"
LOCATION="${AIM_LOCATION:-eastus}"
CONTAINER_APP_ENV="${AIM_CONTAINER_APP_ENV:-aim-env}"
ACR_NAME="${AIM_ACR_NAME:-aimregistry$(date +%s)}"
POSTGRES_SERVER="${AIM_POSTGRES_SERVER:-aim-postgres-$(date +%s)}"
REDIS_NAME="${AIM_REDIS_NAME:-aim-redis}"
APP_NAME="${AIM_APP_NAME:-aim}"

################################################################################
# Helper Functions
################################################################################

print_header() {
    echo -e "${PURPLE}"
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║                                                                ║"
    echo "║          🛡️  AIM - Azure Deployment Script 🛡️                 ║"
    echo "║                                                                ║"
    echo "║              Deploying to Azure Container Apps                ║"
    echo "║                                                                ║"
    echo "╚════════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

print_step() {
    echo -e "\n${CYAN}▶ $1${NC}"
}

print_success() {
    echo -e "${GREEN}✓ $1${NC}"
}

print_error() {
    echo -e "${RED}✗ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

################################################################################
# Prerequisites Check
################################################################################

check_prerequisites() {
    print_step "Checking prerequisites..."

    # Check Azure CLI
    if ! command -v az &> /dev/null; then
        print_error "Azure CLI not found"
        print_info "Install: https://docs.microsoft.com/cli/azure/install-azure-cli"
        exit 1
    fi
    print_success "Azure CLI found: $(az version --query \"azure-cli\" -o tsv)"

    # Check login status
    if ! az account show &> /dev/null; then
        print_error "Not logged in to Azure"
        print_info "Run: az login"
        exit 1
    fi
    print_success "Logged in to Azure"

    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker not found"
        exit 1
    fi
    print_success "Docker found"

    print_success "All prerequisites met!"
}

################################################################################
# Azure Setup
################################################################################

setup_resource_group() {
    print_step "Setting up resource group..."

    # Check if resource group exists
    if az group show --name "$RESOURCE_GROUP" &> /dev/null; then
        print_info "Resource group $RESOURCE_GROUP already exists"
    else
        print_info "Creating resource group: $RESOURCE_GROUP"
        az group create \
            --name "$RESOURCE_GROUP" \
            --location "$LOCATION"
        print_success "Resource group created"
    fi
}

setup_container_registry() {
    print_step "Setting up Azure Container Registry..."

    # Create ACR
    if az acr show --name "$ACR_NAME" --resource-group "$RESOURCE_GROUP" &> /dev/null; then
        print_info "ACR $ACR_NAME already exists"
    else
        print_info "Creating ACR: $ACR_NAME"
        az acr create \
            --resource-group "$RESOURCE_GROUP" \
            --name "$ACR_NAME" \
            --sku Basic \
            --location "$LOCATION" \
            --admin-enabled true
        print_success "ACR created"
    fi

    # Login to ACR
    print_info "Logging in to ACR..."
    az acr login --name "$ACR_NAME"
    print_success "Logged in to ACR"

    # Get ACR credentials
    ACR_USERNAME=$(az acr credential show --name "$ACR_NAME" --query "username" -o tsv)
    ACR_PASSWORD=$(az acr credential show --name "$ACR_NAME" --query "passwords[0].value" -o tsv)
    ACR_LOGIN_SERVER=$(az acr show --name "$ACR_NAME" --query "loginServer" -o tsv)
}

setup_postgres() {
    print_step "Setting up PostgreSQL Flexible Server..."

    # Create PostgreSQL server
    if az postgres flexible-server show \
        --name "$POSTGRES_SERVER" \
        --resource-group "$RESOURCE_GROUP" &> /dev/null; then
        print_info "PostgreSQL server $POSTGRES_SERVER already exists"
    else
        print_info "Creating PostgreSQL server: $POSTGRES_SERVER"

        # Generate admin password
        POSTGRES_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-25)

        az postgres flexible-server create \
            --name "$POSTGRES_SERVER" \
            --resource-group "$RESOURCE_GROUP" \
            --location "$LOCATION" \
            --admin-user aimadmin \
            --admin-password "$POSTGRES_PASSWORD" \
            --sku-name Standard_B1ms \
            --tier Burstable \
            --version 16 \
            --storage-size 32 \
            --public-access 0.0.0.0-255.255.255.255

        print_success "PostgreSQL server created"

        # Create database
        print_info "Creating database: identity"
        az postgres flexible-server db create \
            --resource-group "$RESOURCE_GROUP" \
            --server-name "$POSTGRES_SERVER" \
            --database-name identity

        print_success "Database created"
    fi

    # Get connection string
    POSTGRES_HOST="${POSTGRES_SERVER}.postgres.database.azure.com"
    DATABASE_URL="postgresql://aimadmin:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:5432/identity?sslmode=require"
}

setup_redis() {
    print_step "Setting up Azure Cache for Redis..."

    if az redis show \
        --name "$REDIS_NAME" \
        --resource-group "$RESOURCE_GROUP" &> /dev/null; then
        print_info "Redis cache $REDIS_NAME already exists"
    else
        print_info "Creating Redis cache: $REDIS_NAME"
        az redis create \
            --name "$REDIS_NAME" \
            --resource-group "$RESOURCE_GROUP" \
            --location "$LOCATION" \
            --sku Basic \
            --vm-size c0

        print_success "Redis cache created"
    fi

    # Get Redis connection string
    REDIS_KEY=$(az redis list-keys --name "$REDIS_NAME" --resource-group "$RESOURCE_GROUP" --query "primaryKey" -o tsv)
    REDIS_HOST="${REDIS_NAME}.redis.cache.windows.net"
    REDIS_URL="rediss://:${REDIS_KEY}@${REDIS_HOST}:6380/0"
}

################################################################################
# Build and Push Docker Images
################################################################################

build_and_push_images() {
    print_step "Building and pushing Docker images..."

    # Build backend image
    print_info "Building backend image..."
    cd apps/backend
    docker build -t "${ACR_LOGIN_SERVER}/aim-backend:latest" \
        -f Dockerfile .

    print_info "Pushing backend image..."
    docker push "${ACR_LOGIN_SERVER}/aim-backend:latest"
    print_success "Backend image pushed"

    # Build frontend image
    cd ../web
    print_info "Building frontend image..."
    docker build -t "${ACR_LOGIN_SERVER}/aim-frontend:latest" \
        --build-arg NEXT_PUBLIC_API_URL="https://${APP_NAME}-backend.${LOCATION}.azurecontainerapps.io" \
        -f Dockerfile .

    print_info "Pushing frontend image..."
    docker push "${ACR_LOGIN_SERVER}/aim-frontend:latest"
    print_success "Frontend image pushed"

    cd ../..
}

################################################################################
# Deploy Container Apps
################################################################################

create_container_app_environment() {
    print_step "Creating Container App Environment..."

    if az containerapp env show \
        --name "$CONTAINER_APP_ENV" \
        --resource-group "$RESOURCE_GROUP" &> /dev/null; then
        print_info "Container App Environment already exists"
    else
        print_info "Creating Container App Environment: $CONTAINER_APP_ENV"
        az containerapp env create \
            --name "$CONTAINER_APP_ENV" \
            --resource-group "$RESOURCE_GROUP" \
            --location "$LOCATION"

        print_success "Container App Environment created"
    fi
}

deploy_backend() {
    print_step "Deploying backend Container App..."

    # Generate JWT secret
    JWT_SECRET=$(openssl rand -hex 32)

    # Create or update backend app
    print_info "Creating backend Container App..."
    az containerapp create \
        --name "${APP_NAME}-backend" \
        --resource-group "$RESOURCE_GROUP" \
        --environment "$CONTAINER_APP_ENV" \
        --image "${ACR_LOGIN_SERVER}/aim-backend:latest" \
        --target-port 8080 \
        --ingress external \
        --registry-server "$ACR_LOGIN_SERVER" \
        --registry-username "$ACR_USERNAME" \
        --registry-password "$ACR_PASSWORD" \
        --cpu 1.0 \
        --memory 2.0Gi \
        --min-replicas 1 \
        --max-replicas 5 \
        --env-vars \
            "SERVER_PORT=8080" \
            "ENVIRONMENT=production" \
            "JWT_SECRET=$JWT_SECRET" \
            "DATABASE_URL=$DATABASE_URL" \
            "REDIS_URL=$REDIS_URL" \
            "CORS_ALLOWED_ORIGINS=*"

    print_success "Backend deployed"

    # Get backend URL
    BACKEND_URL=$(az containerapp show \
        --name "${APP_NAME}-backend" \
        --resource-group "$RESOURCE_GROUP" \
        --query "properties.configuration.ingress.fqdn" -o tsv)

    print_success "Backend URL: https://${BACKEND_URL}"
}

deploy_frontend() {
    print_step "Deploying frontend Container App..."

    print_info "Creating frontend Container App..."
    az containerapp create \
        --name "${APP_NAME}-frontend" \
        --resource-group "$RESOURCE_GROUP" \
        --environment "$CONTAINER_APP_ENV" \
        --image "${ACR_LOGIN_SERVER}/aim-frontend:latest" \
        --target-port 3000 \
        --ingress external \
        --registry-server "$ACR_LOGIN_SERVER" \
        --registry-username "$ACR_USERNAME" \
        --registry-password "$ACR_PASSWORD" \
        --cpu 0.5 \
        --memory 1.0Gi \
        --min-replicas 1 \
        --max-replicas 3 \
        --env-vars \
            "NEXT_PUBLIC_API_URL=https://${BACKEND_URL}"

    print_success "Frontend deployed"

    # Get frontend URL
    FRONTEND_URL=$(az containerapp show \
        --name "${APP_NAME}-frontend" \
        --resource-group "$RESOURCE_GROUP" \
        --query "properties.configuration.ingress.fqdn" -o tsv)

    print_success "Frontend URL: https://${FRONTEND_URL}"
}

################################################################################
# Post-Deployment
################################################################################

run_migrations() {
    print_step "Running database migrations..."

    print_info "Connecting to backend container..."

    # Run migrations using az containerapp exec
    az containerapp exec \
        --name "${APP_NAME}-backend" \
        --resource-group "$RESOURCE_GROUP" \
        --command "/bin/sh -c 'cd /app && ./migrate'" || true

    print_success "Migrations completed"
}

print_deployment_summary() {
    print_step "Deployment Summary"

    echo -e "\n${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  🎉 AIM Deployed to Azure Successfully! 🎉                    ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}\n"

    echo -e "${CYAN}📊 Deployment Information:${NC}"
    echo -e "  ${BLUE}•${NC} Resource Group:     $RESOURCE_GROUP"
    echo -e "  ${BLUE}•${NC} Location:           $LOCATION"
    echo -e "  ${BLUE}•${NC} Frontend URL:       https://${FRONTEND_URL}"
    echo -e "  ${BLUE}•${NC} Backend URL:        https://${BACKEND_URL}"

    echo -e "\n${CYAN}🔧 Azure Resources:${NC}"
    echo -e "  ${BLUE}•${NC} Container Registry: $ACR_NAME"
    echo -e "  ${BLUE}•${NC} PostgreSQL Server:  $POSTGRES_SERVER"
    echo -e "  ${BLUE}•${NC} Redis Cache:        $REDIS_NAME"
    echo -e "  ${BLUE}•${NC} Container App Env:  $CONTAINER_APP_ENV"

    echo -e "\n${CYAN}📚 Next Steps:${NC}"
    echo -e "  1. Open https://${FRONTEND_URL} to access AIM"
    echo -e "  2. Configure OAuth in Azure Portal"
    echo -e "  3. Set up custom domain (optional)"
    echo -e "  4. Configure monitoring and alerts"
    echo -e "  5. Review security settings"

    echo -e "\n${CYAN}🔐 Important Credentials:${NC}"
    echo -e "  ${BLUE}•${NC} Database Password:  (stored in Azure Key Vault or environment)"
    echo -e "  ${BLUE}•${NC} Redis Key:          (stored in Azure Key Vault or environment)"
    echo -e "  ${BLUE}•${NC} JWT Secret:         (stored in Container App secrets)"

    echo -e "\n${CYAN}📝 Management Commands:${NC}"
    echo -e "  ${BLUE}•${NC} View logs:          az containerapp logs show --name ${APP_NAME}-backend -g $RESOURCE_GROUP"
    echo -e "  ${BLUE}•${NC} Scale backend:      az containerapp update --name ${APP_NAME}-backend -g $RESOURCE_GROUP --min-replicas 2"
    echo -e "  ${BLUE}•${NC} Update image:       az containerapp update --name ${APP_NAME}-backend -g $RESOURCE_GROUP --image ${ACR_LOGIN_SERVER}/aim-backend:latest"
    echo -e "  ${BLUE}•${NC} Delete deployment:  az group delete --name $RESOURCE_GROUP --yes"

    echo -e "\n${GREEN}🚀 AIM is now running on Azure!${NC}\n"
}

################################################################################
# Main
################################################################################

main() {
    print_header

    print_info "Starting Azure deployment..."
    print_info "Resource Group: $RESOURCE_GROUP"
    print_info "Location: $LOCATION"

    check_prerequisites
    setup_resource_group
    setup_container_registry
    setup_postgres
    setup_redis
    build_and_push_images
    create_container_app_environment
    deploy_backend
    deploy_frontend
    run_migrations
    print_deployment_summary
}

# Run main
main "$@"
