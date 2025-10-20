#!/bin/bash
# ====================================================================================
# AIM First-Time Setup Script
# ====================================================================================
# This script sets up the AIM development environment for the first time
#
# Usage:
#   ./setup.sh
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

print_header "AIM (Agent Identity Management) - First-Time Setup"

print_message "$BLUE" ""
print_message "$BLUE" "This script will help you set up the AIM development environment."
print_message "$BLUE" ""

# Check prerequisites
print_header "Step 1: Checking Prerequisites"

# Check Docker
if ! command -v docker &> /dev/null; then
    print_message "$RED" "❌ Docker is not installed"
    print_message "$YELLOW" "Install Docker Desktop from: https://www.docker.com/products/docker-desktop"
    exit 1
else
    print_message "$GREEN" "✅ Docker $(docker --version | awk '{print $3}' | sed 's/,//') found"
fi

# Check Docker Compose
if ! command -v docker compose &> /dev/null; then
    print_message "$RED" "❌ Docker Compose is not installed"
    print_message "$YELLOW" "Install Docker Desktop (includes Compose) from: https://www.docker.com/products/docker-desktop"
    exit 1
else
    print_message "$GREEN" "✅ Docker Compose found"
fi

# Check Go
if ! command -v go &> /dev/null; then
    print_message "$RED" "❌ Go is not installed"
    print_message "$YELLOW" "Install Go from: https://go.dev/dl/"
    exit 1
else
    print_message "$GREEN" "✅ Go $(go version | awk '{print $3}') found"
fi

# Check Node.js
if ! command -v node &> /dev/null; then
    print_message "$RED" "❌ Node.js is not installed"
    print_message "$YELLOW" "Install Node.js from: https://nodejs.org/"
    exit 1
else
    print_message "$GREEN" "✅ Node.js $(node --version) found"
fi

# Check npm
if ! command -v npm &> /dev/null; then
    print_message "$RED" "❌ npm is not installed"
    exit 1
else
    print_message "$GREEN" "✅ npm $(npm --version) found"
fi

# Setup environment file
print_header "Step 2: Setting Up Environment Variables"

if [ -f "$PROJECT_ROOT/.env" ]; then
    print_message "$YELLOW" "⚠️  .env file already exists"
    read -p "Overwrite with template? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        cp "$PROJECT_ROOT/.env.example" "$PROJECT_ROOT/.env"
        print_message "$GREEN" "✅ Created .env from template"
    else
        print_message "$BLUE" "ℹ️  Keeping existing .env file"
    fi
else
    cp "$PROJECT_ROOT/.env.example" "$PROJECT_ROOT/.env"
    print_message "$GREEN" "✅ Created .env from template"
fi

# Generate JWT secret
print_message "$YELLOW" ""
print_message "$YELLOW" "Generating JWT secret..."
JWT_SECRET=$(openssl rand -hex 32)
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s/your_jwt_secret_here_replace_with_random_64_char_hex/$JWT_SECRET/" "$PROJECT_ROOT/.env"
else
    sed -i "s/your_jwt_secret_here_replace_with_random_64_char_hex/$JWT_SECRET/" "$PROJECT_ROOT/.env"
fi
print_message "$GREEN" "✅ Generated JWT secret"

# Generate KeyVault master key
print_message "$YELLOW" "Generating KeyVault master key..."
KEYVAULT_KEY=$(openssl rand -base64 32)
if [[ "$OSTYPE" == "darwin"* ]]; then
    sed -i '' "s/your_keyvault_master_key_here_replace_with_base64/$KEYVAULT_KEY/" "$PROJECT_ROOT/.env"
else
    sed -i "s/your_keyvault_master_key_here_replace_with_base64/$KEYVAULT_KEY/" "$PROJECT_ROOT/.env"
fi
print_message "$GREEN" "✅ Generated KeyVault master key"

# Install backend dependencies
print_header "Step 3: Installing Backend Dependencies"
cd "$PROJECT_ROOT/apps/backend"
print_message "$YELLOW" "Downloading Go modules..."
go mod download
print_message "$GREEN" "✅ Backend dependencies installed"

# Install frontend dependencies
print_header "Step 4: Installing Frontend Dependencies"
cd "$PROJECT_ROOT/apps/web"
print_message "$YELLOW" "Installing npm packages (this may take a few minutes)..."
npm install
print_message "$GREEN" "✅ Frontend dependencies installed"

# Start database services
print_header "Step 5: Starting Database Services"
cd "$PROJECT_ROOT"
print_message "$YELLOW" "Starting PostgreSQL and Redis..."
docker compose up -d postgres redis

print_message "$YELLOW" "Waiting for database to be ready..."
sleep 5

if docker compose ps postgres | grep -q "Up"; then
    print_message "$GREEN" "✅ PostgreSQL is running"
else
    print_message "$RED" "❌ PostgreSQL failed to start"
    docker compose logs postgres
    exit 1
fi

# Run database migrations
print_header "Step 6: Running Database Migrations"
cd "$PROJECT_ROOT/apps/backend"
print_message "$YELLOW" "Setting up database schema..."
# TODO: Add migration command when available
print_message "$YELLOW" "Skipping migrations (not implemented yet)"

# Final message
print_header "Setup Complete! 🎉"

print_message "$GREEN" ""
print_message "$GREEN" "✅ AIM development environment is ready!"
print_message "$BLUE" ""
print_message "$BLUE" "Next steps:"
print_message "$BLUE" "  1. Review .env file and update any credentials if needed"
print_message "$BLUE" "  2. Start the development environment:"
print_message "$BLUE" "     ./start-dev.sh"
print_message "$BLUE" ""
print_message "$BLUE" "The application will be available at:"
print_message "$BLUE" "  - Frontend: http://localhost:3000"
print_message "$BLUE" "  - Backend:  http://localhost:8080"
print_message "$BLUE" "  - API Docs: http://localhost:8080/swagger"
print_message "$BLUE" ""
print_message "$YELLOW" "To stop services: ./stop-dev.sh"
print_message "$BLUE" ""
