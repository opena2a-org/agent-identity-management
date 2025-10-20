#!/bin/bash
# ====================================================================================
# AIM Development Environment Startup Script
# ====================================================================================
# This script starts the complete AIM development environment
#
# Usage:
#   ./start-dev.sh              # Start all services
#   ./start-dev.sh --backend    # Start only backend
#   ./start-dev.sh --frontend   # Start only frontend
#   ./start-dev.sh --db         # Start only database services
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

# Check if .env file exists
check_env_file() {
    if [ ! -f "$PROJECT_ROOT/.env" ]; then
        print_message "$RED" "❌ Error: .env file not found!"
        print_message "$YELLOW" ""
        print_message "$YELLOW" "Please create .env file from template:"
        print_message "$YELLOW" "  cp .env.example .env"
        print_message "$YELLOW" "  # Edit .env with your actual credentials"
        exit 1
    fi
    print_message "$GREEN" "✅ Found .env file"
}

# Check if Docker is running
check_docker() {
    if ! docker info > /dev/null 2>&1; then
        print_message "$RED" "❌ Error: Docker is not running!"
        print_message "$YELLOW" "Please start Docker Desktop and try again"
        exit 1
    fi
    print_message "$GREEN" "✅ Docker is running"
}

# Check if required ports are available
check_ports() {
    local ports=(5432 6379 8080 3000)
    local ports_in_use=()
    
    for port in "${ports[@]}"; do
        if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
            ports_in_use+=($port)
        fi
    done
    
    if [ ${#ports_in_use[@]} -gt 0 ]; then
        print_message "$YELLOW" "⚠️  Warning: Some ports are already in use:"
        for port in "${ports_in_use[@]}"; do
            print_message "$YELLOW" "   - Port $port"
        done
        print_message "$YELLOW" ""
        read -p "Continue anyway? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    else
        print_message "$GREEN" "✅ All required ports are available"
    fi
}

# Start database services (PostgreSQL + Redis)
start_database_services() {
    print_header "Starting Database Services"
    
    cd "$PROJECT_ROOT"
    
    print_message "$YELLOW" "Starting PostgreSQL and Redis..."
    docker compose up -d postgres redis
    
    print_message "$YELLOW" "Waiting for services to be ready..."
    sleep 5
    
    # Check PostgreSQL
    if docker compose ps postgres | grep -q "Up"; then
        print_message "$GREEN" "✅ PostgreSQL is running"
    else
        print_message "$RED" "❌ PostgreSQL failed to start"
        docker compose logs postgres
        exit 1
    fi
    
    # Check Redis
    if docker compose ps redis | grep -q "Up"; then
        print_message "$GREEN" "✅ Redis is running"
    else
        print_message "$YELLOW" "⚠️  Redis is not running (optional - AIM will continue without caching)"
    fi
}

# Start backend service
start_backend() {
    print_header "Starting Backend (Go)"
    
    cd "$PROJECT_ROOT/apps/backend"
    
    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        print_message "$RED" "❌ Error: Go is not installed!"
        print_message "$YELLOW" "Install Go from: https://go.dev/dl/"
        exit 1
    fi
    
    print_message "$GREEN" "✅ Go $(go version | awk '{print $3}') found"
    
    # Run database migrations
    print_message "$YELLOW" "Running database migrations..."
    if [ -d "migrations" ]; then
        # TODO: Add migration command here
        print_message "$YELLOW" "Skipping migrations (not implemented yet)"
    fi
    
    # Start backend
    print_message "$YELLOW" "Starting backend server on port 8080..."
    print_message "$BLUE" "Backend logs will appear below..."
    echo ""
    
    go run cmd/server/main.go
}

# Start frontend service
start_frontend() {
    print_header "Starting Frontend (Next.js)"
    
    cd "$PROJECT_ROOT/apps/web"
    
    # Check if Node.js is installed
    if ! command -v node &> /dev/null; then
        print_message "$RED" "❌ Error: Node.js is not installed!"
        print_message "$YELLOW" "Install Node.js from: https://nodejs.org/"
        exit 1
    fi
    
    print_message "$GREEN" "✅ Node.js $(node --version) found"
    
    # Check if npm is installed
    if ! command -v npm &> /dev/null; then
        print_message "$RED" "❌ Error: npm is not installed!"
        exit 1
    fi
    
    # Install dependencies if needed
    if [ ! -d "node_modules" ]; then
        print_message "$YELLOW" "Installing frontend dependencies..."
        npm install
    else
        print_message "$GREEN" "✅ Dependencies already installed"
    fi
    
    # Start frontend
    print_message "$YELLOW" "Starting frontend server on port 3000..."
    print_message "$BLUE" "Frontend logs will appear below..."
    echo ""
    
    npm run dev
}

# Start all services
start_all() {
    print_header "Starting Complete AIM Development Environment"
    
    check_env_file
    check_docker
    check_ports
    
    # Start database services
    start_database_services
    
    # Open two new terminal windows for backend and frontend
    print_message "$YELLOW" ""
    print_message "$YELLOW" "Opening backend and frontend in new terminal windows..."
    
    # macOS specific - open new terminal tabs
    if [[ "$OSTYPE" == "darwin"* ]]; then
        # Backend
        osascript -e "tell application \"Terminal\" to do script \"cd $PROJECT_ROOT && ./start-dev.sh --backend\""
        
        # Frontend
        osascript -e "tell application \"Terminal\" to do script \"cd $PROJECT_ROOT && ./start-dev.sh --frontend\""
        
        print_message "$GREEN" ""
        print_message "$GREEN" "✅ Development environment started!"
        print_message "$BLUE" ""
        print_message "$BLUE" "Services:"
        print_message "$BLUE" "  - Frontend: http://localhost:3000"
        print_message "$BLUE" "  - Backend:  http://localhost:8080"
        print_message "$BLUE" "  - API Docs: http://localhost:8080/swagger"
        print_message "$BLUE" ""
        print_message "$YELLOW" "Press Ctrl+C in each terminal window to stop services"
    else
        # Linux/Other - run in background
        print_message "$YELLOW" "Starting backend in background..."
        cd "$PROJECT_ROOT/apps/backend"
        go run cmd/server/main.go > "$PROJECT_ROOT/backend.log" 2>&1 &
        BACKEND_PID=$!
        
        print_message "$YELLOW" "Starting frontend in background..."
        cd "$PROJECT_ROOT/apps/web"
        npm run dev > "$PROJECT_ROOT/frontend.log" 2>&1 &
        FRONTEND_PID=$!
        
        print_message "$GREEN" ""
        print_message "$GREEN" "✅ Development environment started!"
        print_message "$BLUE" ""
        print_message "$BLUE" "Services:"
        print_message "$BLUE" "  - Frontend: http://localhost:3000"
        print_message "$BLUE" "  - Backend:  http://localhost:8080"
        print_message "$BLUE" "  - API Docs: http://localhost:8080/swagger"
        print_message "$BLUE" ""
        print_message "$BLUE" "Logs:"
        print_message "$BLUE" "  - Backend:  tail -f $PROJECT_ROOT/backend.log"
        print_message "$BLUE" "  - Frontend: tail -f $PROJECT_ROOT/frontend.log"
        print_message "$BLUE" ""
        print_message "$YELLOW" "To stop: ./stop-dev.sh"
        
        # Save PIDs for stop script
        echo $BACKEND_PID > "$PROJECT_ROOT/.backend.pid"
        echo $FRONTEND_PID > "$PROJECT_ROOT/.frontend.pid"
    fi
}

# Main script logic
case "${1:-}" in
    --backend)
        check_env_file
        start_backend
        ;;
    --frontend)
        check_env_file
        start_frontend
        ;;
    --db)
        check_env_file
        check_docker
        start_database_services
        ;;
    *)
        start_all
        ;;
esac
