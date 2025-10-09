#!/bin/bash

################################################################################
# AIM (Agent Identity Management) - Automated Deployment Script
################################################################################
# Description: One-command deployment for development and production
# Author: AIM Team
# Version: 1.0.0
# License: Apache 2.0
################################################################################

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Deployment configuration
DEPLOYMENT_MODE="${1:-development}"  # development | production | testing
AIM_VERSION="1.0.0"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

################################################################################
# Helper Functions
################################################################################

print_header() {
    echo -e "${PURPLE}"
    echo "╔════════════════════════════════════════════════════════════════╗"
    echo "║                                                                ║"
    echo "║     🛡️  AIM - Agent Identity Management Deployment 🛡️         ║"
    echo "║                                                                ║"
    echo "║             Secure the Agent-to-Agent Future                  ║"
    echo "║                    Version ${AIM_VERSION}                              ║"
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

print_warning() {
    echo -e "${YELLOW}⚠ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ $1${NC}"
}

################################################################################
# Prerequisite Checks
################################################################################

check_prerequisites() {
    print_step "Checking prerequisites..."

    local missing_deps=()

    # Check Docker
    if ! command -v docker &> /dev/null; then
        missing_deps+=("docker")
    else
        print_success "Docker found: $(docker --version | head -n 1)"
    fi

    # Check Docker Compose
    if ! command -v docker &> /dev/null || ! docker compose version &> /dev/null; then
        missing_deps+=("docker-compose")
    else
        print_success "Docker Compose found: $(docker compose version)"
    fi

    # Check Go (for backend development)
    if [ "$DEPLOYMENT_MODE" = "development" ]; then
        if ! command -v go &> /dev/null; then
            print_warning "Go not found (optional for development)"
        else
            print_success "Go found: $(go version)"
        fi
    fi

    # Check Node.js (for frontend development)
    if [ "$DEPLOYMENT_MODE" = "development" ]; then
        if ! command -v node &> /dev/null; then
            print_warning "Node.js not found (optional for development)"
        else
            print_success "Node.js found: $(node --version)"
        fi
    fi

    # Check Python (for SDK)
    if ! command -v python3 &> /dev/null; then
        print_warning "Python3 not found (required for SDK)"
    else
        print_success "Python3 found: $(python3 --version)"
    fi

    # Exit if critical dependencies are missing
    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "Missing required dependencies: ${missing_deps[*]}"
        print_info "Please install missing dependencies and try again"
        print_info "Visit: https://docs.docker.com/get-docker/"
        exit 1
    fi

    print_success "All prerequisites met!"
}

################################################################################
# Environment Setup
################################################################################

setup_environment() {
    print_step "Setting up environment configuration..."

    # Create .env file if it doesn't exist
    if [ ! -f "$SCRIPT_DIR/.env" ]; then
        print_info "Creating .env file from template..."

        # Generate random secrets
        local jwt_secret=$(openssl rand -hex 32)
        local postgres_password=$(openssl rand -hex 16)

        cat > "$SCRIPT_DIR/.env" << EOF
# AIM Environment Configuration
# Generated: $(date)
# Mode: ${DEPLOYMENT_MODE}

################################################################################
# Application Settings
################################################################################

# Server configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8080
ENVIRONMENT=${DEPLOYMENT_MODE}

# JWT Configuration
JWT_SECRET=${jwt_secret}
JWT_EXPIRATION=24h

# CORS Configuration
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001

################################################################################
# Database Configuration
################################################################################

# PostgreSQL
DATABASE_URL=postgresql://postgres:${postgres_password}@postgres:5432/identity?sslmode=disable
POSTGRES_USER=postgres
POSTGRES_PASSWORD=${postgres_password}
POSTGRES_DB=identity

# Redis
REDIS_URL=redis://redis:6379/0

# Elasticsearch
ELASTICSEARCH_URL=http://elasticsearch:9200

################################################################################
# Object Storage (MinIO)
################################################################################

MINIO_ENDPOINT=minio:9000
MINIO_ACCESS_KEY=aim_minio_user
MINIO_SECRET_KEY=aim_minio_password_dev
MINIO_USE_SSL=false
MINIO_BUCKET=aim-storage

################################################################################
# Message Queue (NATS)
################################################################################

NATS_URL=nats://nats:4222

################################################################################
# OAuth Providers (Optional - Configure for SSO)
################################################################################

# Google OAuth
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URL=http://localhost:8080/auth/google/callback

# Microsoft OAuth
MICROSOFT_CLIENT_ID=
MICROSOFT_CLIENT_SECRET=
MICROSOFT_REDIRECT_URL=http://localhost:8080/auth/microsoft/callback
MICROSOFT_TENANT_ID=common

# Okta OAuth
OKTA_CLIENT_ID=
OKTA_CLIENT_SECRET=
OKTA_REDIRECT_URL=http://localhost:8080/auth/okta/callback
OKTA_DOMAIN=

################################################################################
# Monitoring & Observability
################################################################################

# Prometheus
PROMETHEUS_ENABLED=true

# Grafana
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=admin

# Loki
LOKI_ENABLED=true

################################################################################
# Security Settings
################################################################################

# API Rate Limiting
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS_PER_MINUTE=100

# Trust Score Thresholds
TRUST_SCORE_MIN_LOW=50.0
TRUST_SCORE_MIN_MEDIUM=70.0
TRUST_SCORE_MIN_HIGH=85.0

################################################################################
# Feature Flags
################################################################################

ENABLE_AUDIT_LOGGING=true
ENABLE_WEBHOOKS=true
ENABLE_ALERTS=true
ENABLE_COMPLIANCE_REPORTS=true

EOF

        print_success "Environment file created at .env"
        print_warning "Please review and update OAuth credentials in .env for SSO functionality"
    else
        print_success "Environment file already exists at .env"
    fi
}

################################################################################
# Infrastructure Deployment
################################################################################

deploy_infrastructure() {
    print_step "Deploying infrastructure services..."

    # Pull latest images
    print_info "Pulling Docker images..."
    docker compose pull

    # Start services based on mode
    if [ "$DEPLOYMENT_MODE" = "production" ]; then
        print_info "Starting production services..."
        docker compose up -d
    else
        print_info "Starting development services..."
        docker compose up -d postgres redis elasticsearch minio nats prometheus grafana loki promtail
    fi

    print_success "Infrastructure services started"
}

################################################################################
# Health Checks
################################################################################

wait_for_service() {
    local service_name=$1
    local health_check_cmd=$2
    local max_attempts=30
    local attempt=0

    print_info "Waiting for ${service_name} to be ready..."

    while [ $attempt -lt $max_attempts ]; do
        if eval "$health_check_cmd" &> /dev/null; then
            print_success "${service_name} is ready!"
            return 0
        fi
        attempt=$((attempt + 1))
        sleep 2
        echo -n "."
    done

    print_error "${service_name} failed to start after ${max_attempts} attempts"
    return 1
}

check_services_health() {
    print_step "Checking service health..."

    # PostgreSQL
    wait_for_service "PostgreSQL" \
        "docker compose exec -T postgres pg_isready -U postgres -d identity"

    # Redis
    wait_for_service "Redis" \
        "docker compose exec -T redis redis-cli ping"

    # Elasticsearch
    wait_for_service "Elasticsearch" \
        "curl -f http://localhost:9200/_cluster/health"

    # MinIO
    wait_for_service "MinIO" \
        "curl -f http://localhost:9000/minio/health/live"

    # NATS
    wait_for_service "NATS" \
        "curl -f http://localhost:8222/healthz"

    print_success "All services are healthy!"
}

################################################################################
# Database Setup
################################################################################

setup_database() {
    print_step "Setting up database..."

    # Wait for PostgreSQL
    print_info "Waiting for PostgreSQL to be ready..."
    sleep 5

    # Check if migrations tool exists
    if [ -f "$SCRIPT_DIR/apps/backend/cmd/migrate/main.go" ]; then
        print_info "Running database migrations..."

        # Build and run migrations
        cd "$SCRIPT_DIR/apps/backend"

        # Source .env for database connection
        export $(grep -v '^#' "$SCRIPT_DIR/.env" | xargs)

        go run cmd/migrate/main.go || {
            print_warning "Migration tool not found, attempting SQL migrations..."

            # Run SQL migrations directly if they exist
            if [ -d "$SCRIPT_DIR/apps/backend/migrations" ]; then
                for migration in "$SCRIPT_DIR/apps/backend/migrations"/*.sql; do
                    if [ -f "$migration" ]; then
                        print_info "Applying migration: $(basename $migration)"
                        docker compose exec -T postgres psql -U postgres -d identity < "$migration"
                    fi
                done
            fi
        }

        cd "$SCRIPT_DIR"
        print_success "Database migrations completed"
    else
        print_warning "No migration tool found, skipping migrations"
    fi
}

################################################################################
# Application Deployment
################################################################################

deploy_backend() {
    print_step "Deploying backend application..."

    if [ "$DEPLOYMENT_MODE" = "production" ]; then
        # Build and start backend container
        print_info "Building backend Docker image..."

        if [ -f "$SCRIPT_DIR/apps/backend/Dockerfile" ]; then
            docker build -t aim-backend:${AIM_VERSION} -f "$SCRIPT_DIR/apps/backend/Dockerfile" "$SCRIPT_DIR/apps/backend"

            # Start backend
            docker run -d \
                --name aim-backend \
                --network aim-network \
                -p 8080:8080 \
                --env-file "$SCRIPT_DIR/.env" \
                aim-backend:${AIM_VERSION}

            print_success "Backend deployed"
        else
            print_warning "Backend Dockerfile not found, skipping containerized deployment"
        fi
    else
        print_info "Development mode - run backend manually with:"
        print_info "  cd apps/backend && go run cmd/server/main.go"
    fi
}

deploy_frontend() {
    print_step "Deploying frontend application..."

    if [ "$DEPLOYMENT_MODE" = "production" ]; then
        # Build and start frontend container
        print_info "Building frontend Docker image..."

        if [ -f "$SCRIPT_DIR/apps/web/Dockerfile" ]; then
            docker build -t aim-frontend:${AIM_VERSION} -f "$SCRIPT_DIR/apps/web/Dockerfile" "$SCRIPT_DIR/apps/web"

            # Start frontend
            docker run -d \
                --name aim-frontend \
                --network aim-network \
                -p 3000:3000 \
                -e NEXT_PUBLIC_API_URL=http://localhost:8080 \
                aim-frontend:${AIM_VERSION}

            print_success "Frontend deployed"
        else
            print_warning "Frontend Dockerfile not found, skipping containerized deployment"
        fi
    else
        print_info "Development mode - run frontend manually with:"
        print_info "  cd apps/web && npm install && npm run dev"
    fi
}

################################################################################
# Post-Deployment Setup
################################################################################

create_admin_user() {
    print_step "Creating default admin user..."

    # Check if bootstrap script exists
    if [ -f "$SCRIPT_DIR/apps/backend/cmd/bootstrap/main.go" ]; then
        print_info "Running bootstrap script..."
        cd "$SCRIPT_DIR/apps/backend"
        export $(grep -v '^#' "$SCRIPT_DIR/.env" | xargs)
        go run cmd/bootstrap/main.go || print_warning "Bootstrap failed or already run"
        cd "$SCRIPT_DIR"
    else
        print_warning "Bootstrap script not found"
        print_info "You'll need to create an admin user manually"
    fi
}

################################################################################
# Verification & Summary
################################################################################

verify_deployment() {
    print_step "Verifying deployment..."

    local all_healthy=true

    # Check backend health
    if [ "$DEPLOYMENT_MODE" = "production" ]; then
        if curl -f http://localhost:8080/health &> /dev/null; then
            print_success "Backend API is responding"
        else
            print_error "Backend API is not responding"
            all_healthy=false
        fi
    fi

    # Check database connectivity
    if docker compose exec -T postgres psql -U postgres -d identity -c "SELECT 1" &> /dev/null; then
        print_success "Database is accessible"
    else
        print_error "Database is not accessible"
        all_healthy=false
    fi

    if [ "$all_healthy" = true ]; then
        print_success "Deployment verification passed!"
    else
        print_error "Some checks failed - please review the logs"
        return 1
    fi
}

print_deployment_summary() {
    print_step "Deployment Summary"

    echo -e "\n${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║  🎉 AIM Deployment Completed Successfully! 🎉                 ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}\n"

    echo -e "${CYAN}📊 Service Endpoints:${NC}"
    echo -e "  ${BLUE}•${NC} Backend API:        http://localhost:8080"
    echo -e "  ${BLUE}•${NC} Frontend UI:        http://localhost:3000"
    echo -e "  ${BLUE}•${NC} API Documentation:  http://localhost:8080/swagger"
    echo -e "  ${BLUE}•${NC} Grafana Dashboard:  http://localhost:3003 (admin/admin)"
    echo -e "  ${BLUE}•${NC} MinIO Console:      http://localhost:9001"

    echo -e "\n${CYAN}🔧 Infrastructure Services:${NC}"
    echo -e "  ${BLUE}•${NC} PostgreSQL:         localhost:5432"
    echo -e "  ${BLUE}•${NC} Redis:              localhost:6379"
    echo -e "  ${BLUE}•${NC} Elasticsearch:      localhost:9200"
    echo -e "  ${BLUE}•${NC} NATS:               localhost:4222"
    echo -e "  ${BLUE}•${NC} Prometheus:         localhost:9090"

    echo -e "\n${CYAN}📚 Quick Start Commands:${NC}"

    if [ "$DEPLOYMENT_MODE" = "development" ]; then
        echo -e "  ${BLUE}•${NC} Start backend:      cd apps/backend && go run cmd/server/main.go"
        echo -e "  ${BLUE}•${NC} Start frontend:     cd apps/web && npm run dev"
    fi

    echo -e "  ${BLUE}•${NC} View logs:          docker compose logs -f"
    echo -e "  ${BLUE}•${NC} Stop services:      docker compose down"
    echo -e "  ${BLUE}•${NC} Reset database:     docker compose down -v && ./deploy.sh"

    echo -e "\n${CYAN}🐍 Python SDK:${NC}"
    echo -e "  ${BLUE}•${NC} Install SDK:        cd sdks/python && pip install -r requirements.txt"
    echo -e "  ${BLUE}•${NC} Register agent:     python3 -c \"from aim_sdk import register_agent; register_agent('my-agent', 'http://localhost:8080')\""
    echo -e "  ${BLUE}•${NC} View examples:      cd sdks/python && python3 example.py"

    echo -e "\n${CYAN}📖 Documentation:${NC}"
    echo -e "  ${BLUE}•${NC} Main README:        README.md"
    echo -e "  ${BLUE}•${NC} API Docs:           docs/API.md"
    echo -e "  ${BLUE}•${NC} Deployment Guide:   docs/DEPLOYMENT.md"
    echo -e "  ${BLUE}•${NC} Integration Guides: sdks/python/*_INTEGRATION.md"

    echo -e "\n${CYAN}🔐 Default Credentials:${NC}"
    echo -e "  ${BLUE}•${NC} Grafana:            admin / admin"
    echo -e "  ${BLUE}•${NC} MinIO:              aim_minio_user / aim_minio_password_dev"
    echo -e "  ${BLUE}•${NC} PostgreSQL:         postgres / (check .env file)"

    echo -e "\n${YELLOW}⚠️  Important Notes:${NC}"
    echo -e "  ${BLUE}•${NC} Configure OAuth in .env for SSO functionality"
    echo -e "  ${BLUE}•${NC} Update default passwords in production!"
    echo -e "  ${BLUE}•${NC} Enable HTTPS/TLS for production deployments"
    echo -e "  ${BLUE}•${NC} Review security settings in .env"

    echo -e "\n${GREEN}Next Steps:${NC}"
    echo -e "  1. Open http://localhost:3000 to access the UI"
    echo -e "  2. Register your first agent using the Python SDK"
    echo -e "  3. Explore integration guides in sdks/python/"
    echo -e "  4. Configure OAuth providers for SSO"
    echo -e "  5. Set up monitoring in Grafana\n"
}

################################################################################
# Cleanup Function
################################################################################

cleanup() {
    print_step "Cleaning up previous deployment..."

    # Stop existing containers
    docker compose down

    # Remove orphaned containers
    docker compose rm -f

    print_success "Cleanup completed"
}

################################################################################
# Main Deployment Flow
################################################################################

main() {
    print_header

    # Parse arguments
    case "${1:-}" in
        production)
            DEPLOYMENT_MODE="production"
            ;;
        development|dev)
            DEPLOYMENT_MODE="development"
            ;;
        testing|test)
            DEPLOYMENT_MODE="testing"
            ;;
        clean)
            cleanup
            print_success "Cleanup completed successfully!"
            exit 0
            ;;
        --help|-h)
            echo "Usage: $0 [MODE]"
            echo ""
            echo "Modes:"
            echo "  development  - Development mode (default)"
            echo "  production   - Production mode with Docker containers"
            echo "  testing      - Testing mode"
            echo "  clean        - Clean up all containers and volumes"
            echo ""
            echo "Examples:"
            echo "  $0                    # Deploy in development mode"
            echo "  $0 production         # Deploy in production mode"
            echo "  $0 clean              # Clean up deployment"
            exit 0
            ;;
        *)
            if [ -n "${1:-}" ]; then
                print_error "Unknown mode: $1"
                print_info "Use '$0 --help' for usage information"
                exit 1
            fi
            ;;
    esac

    print_info "Deployment Mode: ${DEPLOYMENT_MODE}"

    # Execute deployment steps
    check_prerequisites
    setup_environment
    deploy_infrastructure
    check_services_health
    setup_database

    if [ "$DEPLOYMENT_MODE" = "production" ]; then
        deploy_backend
        deploy_frontend
    fi

    create_admin_user
    verify_deployment
    print_deployment_summary

    # Final message
    echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
    echo -e "${GREEN}║                                                                ║${NC}"
    echo -e "${GREEN}║  🚀 AIM is ready! Visit http://localhost:3000 to get started  ║${NC}"
    echo -e "${GREEN}║                                                                ║${NC}"
    echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}\n"
}

# Trap errors
trap 'print_error "Deployment failed! Check the logs above for details."' ERR

# Run main
main "$@"
