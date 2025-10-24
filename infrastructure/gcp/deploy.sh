#!/bin/bash

################################################################################
# AIM - Google Cloud Platform Deployment Script
################################################################################
# Description: Deploy AIM to GCP using Cloud Run
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
PROJECT_ID="${GCP_PROJECT_ID:-$(gcloud config get-value project)}"
REGION="${GCP_REGION:-us-central1}"
SERVICE_NAME="${AIM_SERVICE_NAME:-aim}"
SQL_INSTANCE="${AIM_SQL_INSTANCE:-aim-postgres-$(date +%s)}"
REDIS_INSTANCE="${AIM_REDIS_INSTANCE:-aim-redis}"
VPC_CONNECTOR="${AIM_VPC_CONNECTOR:-aim-vpc-connector}"

################################################################################
# Helper Functions
################################################################################

print_header() {
    echo -e "${PURPLE}"
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║                                                                ║"
    echo "║          🛡️  AIM - GCP Deployment Script 🛡️                   ║"
    echo "║                                                                ║"
    echo "║                  Deploying to Cloud Run                       ║"
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

    # Check gcloud CLI
    if ! command -v gcloud &> /dev/null; then
        print_error "gcloud CLI not found"
        print_info "Install: https://cloud.google.com/sdk/docs/install"
        exit 1
    fi
    print_success "gcloud CLI found: $(gcloud version | head -n 1)"

    # Check if logged in
    if ! gcloud auth list --filter=status:ACTIVE --format="value(account)" &> /dev/null; then
        print_error "Not logged in to GCP"
        print_info "Run: gcloud auth login"
        exit 1
    fi
    print_success "Logged in to GCP"

    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker not found"
        exit 1
    fi
    print_success "Docker found"

    # Verify project
    if [ -z "$PROJECT_ID" ]; then
        print_error "GCP_PROJECT_ID not set"
        print_info "Set with: export GCP_PROJECT_ID=your-project-id"
        exit 1
    fi
    print_success "Project ID: $PROJECT_ID"

    print_success "All prerequisites met!"
}

################################################################################
# Enable Required APIs
################################################################################

enable_apis() {
    print_step "Enabling required GCP APIs..."

    local apis=(
        "run.googleapis.com"
        "sqladmin.googleapis.com"
        "redis.googleapis.com"
        "vpcaccess.googleapis.com"
        "artifactregistry.googleapis.com"
        "cloudbuild.googleapis.com"
        "secretmanager.googleapis.com"
    )

    for api in "${apis[@]}"; do
        print_info "Enabling $api..."
        gcloud services enable "$api" --project="$PROJECT_ID" 2>/dev/null || true
    done

    print_success "APIs enabled"
}

################################################################################
# Setup Infrastructure
################################################################################

setup_artifact_registry() {
    print_step "Setting up Artifact Registry..."

    # Create repository
    if gcloud artifacts repositories describe aim-docker \
        --location="$REGION" \
        --project="$PROJECT_ID" &> /dev/null; then
        print_info "Artifact Registry repository already exists"
    else
        print_info "Creating Artifact Registry repository..."
        gcloud artifacts repositories create aim-docker \
            --repository-format=docker \
            --location="$REGION" \
            --project="$PROJECT_ID"

        print_success "Artifact Registry created"
    fi

    # Configure Docker authentication
    print_info "Configuring Docker authentication..."
    gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet

    REGISTRY_URL="${REGION}-docker.pkg.dev/${PROJECT_ID}/aim-docker"
}

setup_cloud_sql() {
    print_step "Setting up Cloud SQL (PostgreSQL)..."

    # Create SQL instance
    if gcloud sql instances describe "$SQL_INSTANCE" \
        --project="$PROJECT_ID" &> /dev/null; then
        print_info "Cloud SQL instance already exists"
    else
        print_info "Creating Cloud SQL instance: $SQL_INSTANCE"
        print_info "This may take 5-10 minutes..."

        # Generate root password
        SQL_PASSWORD=$(openssl rand -base64 32 | tr -d "=+/" | cut -c1-25)

        gcloud sql instances create "$SQL_INSTANCE" \
            --database-version=POSTGRES_16 \
            --tier=db-f1-micro \
            --region="$REGION" \
            --root-password="$SQL_PASSWORD" \
            --database-flags=cloudsql.iam_authentication=on \
            --project="$PROJECT_ID"

        print_success "Cloud SQL instance created"

        # Create database
        print_info "Creating database: identity"
        gcloud sql databases create identity \
            --instance="$SQL_INSTANCE" \
            --project="$PROJECT_ID"

        print_success "Database created"
    fi

    # Get connection name
    SQL_CONNECTION_NAME=$(gcloud sql instances describe "$SQL_INSTANCE" \
        --project="$PROJECT_ID" \
        --format="value(connectionName)")

    DATABASE_URL="postgresql://postgres:${SQL_PASSWORD}@localhost/identity?host=/cloudsql/${SQL_CONNECTION_NAME}"

    print_success "Cloud SQL configured"
}

setup_redis() {
    print_step "Setting up Memorystore (Redis)..."

    if gcloud redis instances describe "$REDIS_INSTANCE" \
        --region="$REGION" \
        --project="$PROJECT_ID" &> /dev/null; then
        print_info "Redis instance already exists"
    else
        print_info "Creating Redis instance: $REDIS_INSTANCE"
        print_info "This may take 5-10 minutes..."

        gcloud redis instances create "$REDIS_INSTANCE" \
            --size=1 \
            --region="$REGION" \
            --tier=basic \
            --redis-version=redis_7_0 \
            --project="$PROJECT_ID"

        print_success "Redis instance created"
    fi

    # Get Redis host and port
    REDIS_HOST=$(gcloud redis instances describe "$REDIS_INSTANCE" \
        --region="$REGION" \
        --project="$PROJECT_ID" \
        --format="value(host)")

    REDIS_PORT=$(gcloud redis instances describe "$REDIS_INSTANCE" \
        --region="$REGION" \
        --project="$PROJECT_ID" \
        --format="value(port)")

    REDIS_URL="redis://${REDIS_HOST}:${REDIS_PORT}/0"

    print_success "Redis configured"
}

setup_vpc_connector() {
    print_step "Setting up VPC Access Connector..."

    if gcloud compute networks vpc-access connectors describe "$VPC_CONNECTOR" \
        --region="$REGION" \
        --project="$PROJECT_ID" &> /dev/null; then
        print_info "VPC connector already exists"
    else
        print_info "Creating VPC connector: $VPC_CONNECTOR"

        gcloud compute networks vpc-access connectors create "$VPC_CONNECTOR" \
            --region="$REGION" \
            --range=10.8.0.0/28 \
            --project="$PROJECT_ID"

        print_success "VPC connector created"
    fi
}

################################################################################
# Build and Push Docker Images
################################################################################

build_and_push_images() {
    print_step "Building and pushing Docker images..."

    # Build backend image
    print_info "Building backend image..."
    cd apps/backend
    gcloud builds submit \
        --tag="${REGISTRY_URL}/aim-backend:latest" \
        --project="$PROJECT_ID" \
        .

    print_success "Backend image built and pushed"

    # Build frontend image
    cd ../web
    print_info "Building frontend image..."
    gcloud builds submit \
        --tag="${REGISTRY_URL}/aim-frontend:latest" \
        --project="$PROJECT_ID" \
        --substitutions="_NEXT_PUBLIC_API_URL=https://${SERVICE_NAME}-backend-${PROJECT_ID}.${REGION}.run.app" \
        .

    print_success "Frontend image built and pushed"

    cd ../..
}

################################################################################
# Deploy Cloud Run Services
################################################################################

deploy_backend() {
    print_step "Deploying backend to Cloud Run..."

    # Generate JWT secret
    JWT_SECRET=$(openssl rand -hex 32)

    # Deploy backend service
    print_info "Creating backend Cloud Run service..."
    gcloud run deploy "${SERVICE_NAME}-backend" \
        --image="${REGISTRY_URL}/aim-backend:latest" \
        --platform=managed \
        --region="$REGION" \
        --allow-unauthenticated \
        --port=8080 \
        --cpu=1 \
        --memory=2Gi \
        --min-instances=1 \
        --max-instances=10 \
        --vpc-connector="$VPC_CONNECTOR" \
        --add-cloudsql-instances="$SQL_CONNECTION_NAME" \
        --set-env-vars="SERVER_PORT=8080,ENVIRONMENT=production,JWT_SECRET=${JWT_SECRET},DATABASE_URL=${DATABASE_URL},REDIS_URL=${REDIS_URL},CORS_ALLOWED_ORIGINS=*" \
        --project="$PROJECT_ID"

    print_success "Backend deployed"

    # Get backend URL
    BACKEND_URL=$(gcloud run services describe "${SERVICE_NAME}-backend" \
        --region="$REGION" \
        --project="$PROJECT_ID" \
        --format="value(status.url)")

    print_success "Backend URL: $BACKEND_URL"
}

deploy_frontend() {
    print_step "Deploying frontend to Cloud Run..."

    print_info "Creating frontend Cloud Run service..."
    gcloud run deploy "${SERVICE_NAME}-frontend" \
        --image="${REGISTRY_URL}/aim-frontend:latest" \
        --platform=managed \
        --region="$REGION" \
        --allow-unauthenticated \
        --port=3000 \
        --cpu=1 \
        --memory=1Gi \
        --min-instances=1 \
        --max-instances=5 \
        --set-env-vars="NEXT_PUBLIC_API_URL=${BACKEND_URL}" \
        --project="$PROJECT_ID"

    print_success "Frontend deployed"

    # Get frontend URL
    FRONTEND_URL=$(gcloud run services describe "${SERVICE_NAME}-frontend" \
        --region="$REGION" \
        --project="$PROJECT_ID" \
        --format="value(status.url)")

    print_success "Frontend URL: $FRONTEND_URL"
}

################################################################################
# Post-Deployment
################################################################################

run_migrations() {
    print_step "Running database migrations..."

    print_info "Running migrations via Cloud Run Jobs..."

    # Create a one-off job to run migrations
    gcloud run jobs create "${SERVICE_NAME}-migrate" \
        --image="${REGISTRY_URL}/aim-backend:latest" \
        --region="$REGION" \
        --vpc-connector="$VPC_CONNECTOR" \
        --add-cloudsql-instances="$SQL_CONNECTION_NAME" \
        --set-env-vars="DATABASE_URL=${DATABASE_URL}" \
        --command="/app/migrate" \
        --project="$PROJECT_ID" 2>/dev/null || true

    # Execute migration job
    gcloud run jobs execute "${SERVICE_NAME}-migrate" \
        --region="$REGION" \
        --project="$PROJECT_ID" \
        --wait || true

    print_success "Migrations completed"
}

print_deployment_summary() {
    print_step "Deployment Summary"

    echo -e "\n${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  🎉 AIM Deployed to Google Cloud Successfully! 🎉             ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}\n"

    echo -e "${CYAN}📊 Deployment Information:${NC}"
    echo -e "  ${BLUE}•${NC} Project ID:         $PROJECT_ID"
    echo -e "  ${BLUE}•${NC} Region:             $REGION"
    echo -e "  ${BLUE}•${NC} Frontend URL:       $FRONTEND_URL"
    echo -e "  ${BLUE}•${NC} Backend URL:        $BACKEND_URL"

    echo -e "\n${CYAN}🔧 GCP Resources:${NC}"
    echo -e "  ${BLUE}•${NC} Cloud SQL Instance: $SQL_INSTANCE"
    echo -e "  ${BLUE}•${NC} Redis Instance:     $REDIS_INSTANCE"
    echo -e "  ${BLUE}•${NC} VPC Connector:      $VPC_CONNECTOR"
    echo -e "  ${BLUE}•${NC} Artifact Registry:  ${REGION}-docker.pkg.dev/${PROJECT_ID}/aim-docker"

    echo -e "\n${CYAN}📚 Next Steps:${NC}"
    echo -e "  1. Open $FRONTEND_URL to access AIM"
    echo -e "  2. Configure OAuth in GCP Console"
    echo -e "  3. Set up custom domain (optional)"
    echo -e "  4. Configure Cloud Monitoring"
    echo -e "  5. Review IAM permissions"

    echo -e "\n${CYAN}📝 Management Commands:${NC}"
    echo -e "  ${BLUE}•${NC} View logs:          gcloud run services logs read ${SERVICE_NAME}-backend --region $REGION"
    echo -e "  ${BLUE}•${NC} Scale backend:      gcloud run services update ${SERVICE_NAME}-backend --min-instances=2 --region $REGION"
    echo -e "  ${BLUE}•${NC} Update backend:     gcloud run deploy ${SERVICE_NAME}-backend --image ${REGISTRY_URL}/aim-backend:latest --region $REGION"
    echo -e "  ${BLUE}•${NC} Delete services:    gcloud run services delete ${SERVICE_NAME}-backend --region $REGION"

    echo -e "\n${GREEN}🚀 AIM is now running on Google Cloud!${NC}\n"
}

################################################################################
# Main
################################################################################

main() {
    print_header

    print_info "Starting GCP deployment..."
    print_info "Project: $PROJECT_ID"
    print_info "Region: $REGION"

    check_prerequisites
    enable_apis
    setup_artifact_registry
    setup_cloud_sql
    setup_redis
    setup_vpc_connector
    build_and_push_images
    deploy_backend
    deploy_frontend
    run_migrations
    print_deployment_summary
}

# Run main
main "$@"
