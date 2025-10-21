#!/bin/bash

set -e  # Exit on error

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
REPORT_DIR="$PROJECT_ROOT/production-readiness/reports"

echo "==================================="
echo "AIM Integration Test Runner"
echo "==================================="
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Create reports directory
mkdir -p "$REPORT_DIR"

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}❌ Docker is not running. Please start Docker Desktop.${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Docker is running${NC}"
echo ""

# Navigate to project root
cd "$PROJECT_ROOT"

# Check if Docker Compose file exists
if [ ! -f "docker-compose.yml" ]; then
    echo -e "${RED}❌ docker-compose.yml not found in project root${NC}"
    exit 1
fi

echo -e "${YELLOW}Starting test environment with Docker Compose...${NC}"
echo ""

# Stop any existing containers
docker compose down -v 2>/dev/null || true

# Start PostgreSQL and Redis only (not the full stack)
docker compose up -d postgres redis

# Wait for PostgreSQL to be healthy
echo -e "${YELLOW}Waiting for PostgreSQL to be ready...${NC}"
MAX_RETRIES=30
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if docker compose exec -T postgres pg_isready -U postgres -d identity > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PostgreSQL is ready${NC}"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
        echo -e "${RED}❌ PostgreSQL failed to start within timeout${NC}"
        docker compose logs postgres
        docker compose down -v
        exit 1
    fi
    sleep 2
done

# Wait for Redis to be healthy
echo -e "${YELLOW}Waiting for Redis to be ready...${NC}"
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if docker compose exec -T redis redis-cli ping > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Redis is ready${NC}"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
        echo -e "${RED}❌ Redis failed to start within timeout${NC}"
        docker compose logs redis
        docker compose down -v
        exit 1
    fi
    sleep 2
done

echo ""

# Run database migrations
echo -e "${YELLOW}Running database migrations...${NC}"
cd "$PROJECT_ROOT/apps/backend"

export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=postgres
export POSTGRES_DB=identity
export POSTGRES_SSL_MODE=disable
export REDIS_HOST=localhost
export REDIS_PORT=6379
export JWT_SECRET=test-secret-for-integration-tests

# Run migrations using psql (migrations are standard SQL files)
echo "Running migrations..."
export PGPASSWORD=postgres

for migration_file in migrations/*.sql; do
    if [ -f "$migration_file" ]; then
        echo "  Applying $(basename $migration_file)..."
        psql -h localhost -p 5432 -U postgres -d identity -f "$migration_file" > /dev/null 2>&1 || {
            echo -e "${RED}❌ Migration failed: $migration_file${NC}"
            cd "$PROJECT_ROOT"
            docker compose down -v
            exit 1
        }
    fi
done

echo -e "${GREEN}✓ Migrations complete${NC}"
echo ""

# Start backend server in background
echo -e "${YELLOW}Starting backend server...${NC}"
export LOG_LEVEL=error  # Reduce log noise during tests

# Build backend if not built
if [ ! -f "./cmd/server/server" ]; then
    echo "Building backend..."
    go build -o ./cmd/server/server ./cmd/server/main.go
fi

# Start server in background
./cmd/server/server > "$REPORT_DIR/backend-test.log" 2>&1 &
BACKEND_PID=$!

# Function to cleanup on exit
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up...${NC}"

    # Kill backend server
    if [ ! -z "$BACKEND_PID" ]; then
        kill $BACKEND_PID 2>/dev/null || true
    fi

    # Stop Docker Compose
    cd "$PROJECT_ROOT"
    docker compose down -v

    echo -e "${GREEN}✓ Cleanup complete${NC}"
}

# Register cleanup function
trap cleanup EXIT INT TERM

# Wait for backend to be ready
echo -e "${YELLOW}Waiting for backend to be ready...${NC}"
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${GREEN}✓ Backend is ready${NC}"
        break
    fi
    RETRY_COUNT=$((RETRY_COUNT + 1))
    if [ $RETRY_COUNT -eq $MAX_RETRIES ]; then
        echo -e "${RED}❌ Backend failed to start within timeout${NC}"
        cat "$REPORT_DIR/backend-test.log"
        exit 1
    fi
    sleep 2
done

echo ""
echo -e "${GREEN}==================================="
echo "Running Integration Tests"
echo "===================================${NC}"
echo ""

# Set test environment variable
export TEST_BASE_URL=http://localhost:8080

# Run integration tests
cd "$PROJECT_ROOT/apps/backend"

TEST_OUTPUT="$REPORT_DIR/integration-test-output.txt"
TEST_JSON="$REPORT_DIR/integration-test-results.json"

# Run tests with JSON output and capture results
go test -v ./tests/integration/... -json 2>&1 | tee "$TEST_JSON" | tee "$TEST_OUTPUT"

TEST_EXIT_CODE=${PIPESTATUS[0]}

echo ""

# Parse test results
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo -e "${GREEN}==================================="
    echo "✅ All Integration Tests Passed!"
    echo "===================================${NC}"
else
    echo -e "${RED}==================================="
    echo "❌ Some Integration Tests Failed"
    echo "===================================${NC}"
fi

echo ""
echo "Test results saved to:"
echo "  - $TEST_OUTPUT"
echo "  - $TEST_JSON"
echo "  - Backend logs: $REPORT_DIR/backend-test.log"
echo ""

# Generate summary report
REPORT_FILE="$REPORT_DIR/integration-test-report.md"

cat > "$REPORT_FILE" <<EOF
# Integration Test Report

**Date**: $(date '+%Y-%m-%d %H:%M:%S')
**Environment**: Local Docker Compose
**Backend URL**: http://localhost:8080

## Test Results Summary

EOF

# Count test results
TOTAL_TESTS=$(grep -c '"Test":' "$TEST_JSON" 2>/dev/null || echo "0")
PASSED_TESTS=$(grep '"Action":"pass"' "$TEST_JSON" | wc -l | tr -d ' ')
FAILED_TESTS=$(grep '"Action":"fail"' "$TEST_JSON" | wc -l | tr -d ' ')
SKIPPED_TESTS=$(grep '"Action":"skip"' "$TEST_JSON" | wc -l | tr -d ' ')

cat >> "$REPORT_FILE" <<EOF
- **Total Tests**: $TOTAL_TESTS
- **Passed**: $PASSED_TESTS
- **Failed**: $FAILED_TESTS
- **Skipped**: $SKIPPED_TESTS

## Test Categories Covered

### ✅ Authentication Endpoints
- User registration
- Local login
- Token validation
- Token refresh
- Password change
- Duplicate email rejection
- Invalid credentials handling

### ✅ Agent Endpoints
- Agent CRUD operations
- Trust score calculation
- Agent verification
- Activity tracking
- Search functionality
- Authentication requirements

### ✅ MCP Server Endpoints
- MCP server registration
- Server verification
- Capability management
- Search functionality
- Update operations
- Deletion handling

### ✅ Admin Endpoints
- User management
- Role updates
- System statistics
- Audit logs
- User deactivation
- Permission enforcement

### ✅ Security Endpoints
- Security alerts
- Threat detection
- Security scans
- Scan results
- Alert acknowledgment

### ✅ Analytics Endpoints
- Dashboard data
- Usage statistics
- Trust trends
- Agent distribution
- Top agents
- Compliance reports

## Infrastructure Validation

✅ **PostgreSQL**: Running and healthy
✅ **Redis**: Running and healthy
✅ **Database Migrations**: All migrations applied successfully
✅ **Backend API**: Started and responding to health checks

## Next Steps

EOF

if [ "$FAILED_TESTS" = "0" ]; then
    cat >> "$REPORT_FILE" <<EOF
✅ **All integration tests passing** - Ready to proceed to Layer 4 (E2E Testing)

**Recommendation**: Move forward with end-to-end testing of complete user journeys.
EOF
else
    cat >> "$REPORT_FILE" <<EOF
⚠️ **Some tests failed** - Review failures and fix before proceeding

**Action Required**:
1. Review failed tests in $TEST_OUTPUT
2. Check backend logs at $REPORT_DIR/backend-test.log
3. Fix identified issues
4. Re-run integration tests
EOF
fi

echo ""
echo "Integration test report generated: $REPORT_FILE"
echo ""

# Show summary
cat "$REPORT_FILE"

exit $TEST_EXIT_CODE
