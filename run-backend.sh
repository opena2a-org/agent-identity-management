#!/bin/bash
# ====================================================================================
# AIM Backend Quick Start Script
# ====================================================================================
# Quick script to start just the backend service
#
# Usage:
#   ./run-backend.sh
# ====================================================================================

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}Starting AIM Backend...${NC}"
echo ""

# Project root directory
PROJECT_ROOT="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Check if .env exists
if [ ! -f "$PROJECT_ROOT/.env" ]; then
    echo "Error: .env file not found!"
    echo "Run ./setup.sh first"
    exit 1
fi

# Navigate to backend directory
cd "$PROJECT_ROOT/apps/backend"

# Start backend
echo -e "${GREEN}Backend starting on http://localhost:8080${NC}"
echo ""
go run cmd/server/main.go
