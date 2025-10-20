#!/bin/bash
# ====================================================================================
# AIM Production Deployment Script
# ====================================================================================
# This script helps deploy AIM to production environments
#
# Supported platforms:
#   - Docker Compose (local/VPS)
#   - Azure Container Apps
#   - AWS ECS
#   - Kubernetes (K8s)
#
# Usage:
#   ./deploy-prod.sh --platform=docker
#   ./deploy-prod.sh --platform=azure
#   ./deploy-prod.sh --platform=aws
#   ./deploy-prod.sh --platform=k8s
# ====================================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Project root directory
PROJECT_ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Print colored message
print_message() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

# Print section header
print_header() {
    echo ""
    print_message "$BLUE" "=========================================="
    print_message "$BLUE" "$1"
    print_message "$BLUE" "=========================================="
}

# Parse command line arguments
PLATFORM=""
for arg in "$@"; do
    case $arg in
        --platform=*)
            PLATFORM="${arg#*=}"
            shift
            ;;
        *)
            print_message "$RED" "Unknown argument: $arg"
            exit 1
            ;;
    esac
done

# Validate platform
if [ -z "$PLATFORM" ]; then
    print_message "$RED" "Error: --platform is required"
    print_message "$YELLOW" ""
    print_message "$YELLOW" "Usage: ./deploy-prod.sh --platform=<platform>"
    print_message "$YELLOW" ""
    print_message "$YELLOW" "Supported platforms:"
    print_message "$YELLOW" "  - docker    : Docker Compose (local/VPS)"
    print_message "$YELLOW" "  - azure     : Azure Container Apps"
    print_message "$YELLOW" "  - aws       : AWS ECS/Fargate"
    print_message "$YELLOW" "  - k8s       : Kubernetes"
    exit 1
fi

print_header "AIM Production Deployment - $PLATFORM"

# Docker Compose deployment
deploy_docker() {
    print_header "Deploying to Docker Compose"
    
    cd "$PROJECT_ROOT"
    
    # Build images
    print_message "$YELLOW" "Building Docker images..."
    docker compose -f docker-compose.yml build
    
    # Run security scan
    print_message "$YELLOW" "Running security scan with Trivy..."
    if command -v trivy &> /dev/null; then
        trivy image aim-backend:latest
        trivy image aim-frontend:latest
    else
        print_message "$YELLOW" "⚠️  Trivy not installed, skipping security scan"
    fi
    
    # Start services
    print_message "$YELLOW" "Starting production services..."
    docker compose -f docker-compose.yml up -d
    
    # Wait for health checks
    print_message "$YELLOW" "Waiting for health checks..."
    sleep 10
    
    # Verify deployment
    if curl -f http://localhost:8080/health > /dev/null 2>&1; then
        print_message "$GREEN" "✅ Backend health check passed"
    else
        print_message "$RED" "❌ Backend health check failed"
        exit 1
    fi
    
    print_message "$GREEN" ""
    print_message "$GREEN" "✅ Deployment complete!"
    print_message "$BLUE" ""
    print_message "$BLUE" "Services:"
    print_message "$BLUE" "  - Frontend: http://localhost:3000"
    print_message "$BLUE" "  - Backend:  http://localhost:8080"
    print_message "$BLUE" ""
}

# Azure deployment
deploy_azure() {
    print_header "Deploying to Azure Container Apps"
    
    print_message "$YELLOW" ""
    print_message "$YELLOW" "Azure deployment requires:"
    print_message "$YELLOW" "  1. Azure CLI installed (az)"
    print_message "$YELLOW" "  2. Azure Container Registry created"
    print_message "$YELLOW" "  3. Resource group created"
    print_message "$YELLOW" ""
    
    # Check Azure CLI
    if ! command -v az &> /dev/null; then
        print_message "$RED" "❌ Azure CLI is not installed"
        print_message "$YELLOW" "Install from: https://docs.microsoft.com/cli/azure/install-azure-cli"
        exit 1
    fi
    
    # Login check
    if ! az account show > /dev/null 2>&1; then
        print_message "$YELLOW" "Please login to Azure..."
        az login
    fi
    
    print_message "$GREEN" "✅ Azure CLI authenticated"
    print_message "$BLUE" ""
    print_message "$BLUE" "For complete Azure deployment guide, see:"
    print_message "$BLUE" "  DEPLOYMENT_GUIDE.md - Azure Container Apps section"
    print_message "$BLUE" ""
}

# AWS deployment
deploy_aws() {
    print_header "Deploying to AWS ECS/Fargate"
    
    print_message "$YELLOW" ""
    print_message "$YELLOW" "AWS deployment requires:"
    print_message "$YELLOW" "  1. AWS CLI installed (aws)"
    print_message "$YELLOW" "  2. ECR repository created"
    print_message "$YELLOW" "  3. ECS cluster created"
    print_message "$YELLOW" ""
    
    # Check AWS CLI
    if ! command -v aws &> /dev/null; then
        print_message "$RED" "❌ AWS CLI is not installed"
        print_message "$YELLOW" "Install from: https://aws.amazon.com/cli/"
        exit 1
    fi
    
    # Login check
    if ! aws sts get-caller-identity > /dev/null 2>&1; then
        print_message "$RED" "❌ AWS CLI is not configured"
        print_message "$YELLOW" "Run: aws configure"
        exit 1
    fi
    
    print_message "$GREEN" "✅ AWS CLI authenticated"
    print_message "$BLUE" ""
    print_message "$BLUE" "For complete AWS deployment guide, see:"
    print_message "$BLUE" "  DEPLOYMENT_GUIDE.md - AWS ECS section"
    print_message "$BLUE" ""
}

# Kubernetes deployment
deploy_k8s() {
    print_header "Deploying to Kubernetes"
    
    print_message "$YELLOW" ""
    print_message "$YELLOW" "Kubernetes deployment requires:"
    print_message "$YELLOW" "  1. kubectl installed"
    print_message "$YELLOW" "  2. Helm installed (optional)"
    print_message "$YELLOW" "  3. Cluster access configured"
    print_message "$YELLOW" ""
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        print_message "$RED" "❌ kubectl is not installed"
        print_message "$YELLOW" "Install from: https://kubernetes.io/docs/tasks/tools/"
        exit 1
    fi
    
    # Check cluster access
    if ! kubectl cluster-info > /dev/null 2>&1; then
        print_message "$RED" "❌ Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    print_message "$GREEN" "✅ kubectl connected to cluster"
    print_message "$BLUE" ""
    print_message "$BLUE" "Applying Kubernetes manifests..."
    
    cd "$PROJECT_ROOT/infrastructure/k8s"
    kubectl apply -f namespace.yaml
    kubectl apply -f secrets.yaml
    kubectl apply -f configmap.yaml
    kubectl apply -f postgres.yaml
    kubectl apply -f redis.yaml
    kubectl apply -f backend.yaml
    kubectl apply -f frontend.yaml
    kubectl apply -f ingress.yaml
    
    print_message "$GREEN" ""
    print_message "$GREEN" "✅ Deployment complete!"
    print_message "$BLUE" ""
    print_message "$BLUE" "Check status:"
    print_message "$BLUE" "  kubectl get pods -n aim"
    print_message "$BLUE" ""
}

# Execute deployment based on platform
case "$PLATFORM" in
    docker)
        deploy_docker
        ;;
    azure)
        deploy_azure
        ;;
    aws)
        deploy_aws
        ;;
    k8s)
        deploy_k8s
        ;;
    *)
        print_message "$RED" "Unsupported platform: $PLATFORM"
        exit 1
        ;;
esac

print_message "$BLUE" ""
print_message "$BLUE" "For detailed deployment instructions, see DEPLOYMENT_GUIDE.md"
print_message "$BLUE" ""
