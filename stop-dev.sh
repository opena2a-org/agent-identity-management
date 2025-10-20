#!/bin/bash
# ====================================================================================
# AIM Development Environment Stop Script
# ====================================================================================
# This script stops all AIM development services
#
# Usage:
#   ./stop-dev.sh              # Stop all services
#   ./stop-dev.sh --keep-db    # Stop backend/frontend but keep database running
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

# Stop processes on specific ports
stop_port() {
    local port=$1
    local service=$2
    
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        print_message "$YELLOW" "Stopping $service on port $port..."
        lsof -ti:$port | xargs kill -9 2>/dev/null || true
        print_message "$GREEN" "✅ Stopped $service"
    else
        print_message "$BLUE" "ℹ️  $service is not running"
    fi
}

# Stop Docker services
stop_docker_services() {
    print_header "Stopping Docker Services"
    
    cd "$PROJECT_ROOT"
    
    if docker compose ps | grep -q "Up"; then
        print_message "$YELLOW" "Stopping Docker containers..."
        docker compose down
        print_message "$GREEN" "✅ Docker services stopped"
    else
        print_message "$BLUE" "ℹ️  No Docker services running"
    fi
}

# Stop processes by PID file
stop_by_pid() {
    local pid_file=$1
    local service=$2
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat "$pid_file")
        if kill -0 "$pid" 2>/dev/null; then
            print_message "$YELLOW" "Stopping $service (PID: $pid)..."
            kill -9 "$pid" 2>/dev/null || true
            print_message "$GREEN" "✅ Stopped $service"
        fi
        rm "$pid_file"
    fi
}

# Main stop logic
if [ "${1:-}" = "--keep-db" ]; then
    print_header "Stopping Backend & Frontend (Keeping Database)"
    
    # Stop backend
    stop_port 8080 "Backend"
    stop_by_pid "$PROJECT_ROOT/.backend.pid" "Backend"
    
    # Stop frontend
    stop_port 3000 "Frontend"
    stop_by_pid "$PROJECT_ROOT/.frontend.pid" "Frontend"
    
    print_message "$GREEN" ""
    print_message "$GREEN" "✅ Backend and Frontend stopped"
    print_message "$BLUE" "Database services are still running"
    
else
    print_header "Stopping All AIM Services"
    
    # Stop backend
    stop_port 8080 "Backend"
    stop_by_pid "$PROJECT_ROOT/.backend.pid" "Backend"
    
    # Stop frontend
    stop_port 3000 "Frontend"
    stop_by_pid "$PROJECT_ROOT/.frontend.pid" "Frontend"
    
    # Stop Docker services
    stop_docker_services
    
    # Clean up log files
    if [ -f "$PROJECT_ROOT/backend.log" ]; then
        rm "$PROJECT_ROOT/backend.log"
    fi
    if [ -f "$PROJECT_ROOT/frontend.log" ]; then
        rm "$PROJECT_ROOT/frontend.log"
    fi
    
    print_message "$GREEN" ""
    print_message "$GREEN" "✅ All services stopped"
fi
