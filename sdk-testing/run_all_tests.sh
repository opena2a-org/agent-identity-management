#!/bin/bash

# AIM SDK Comprehensive Test Suite Runner
# Tests all SDK features and claims from documentation

set -e  # Exit on error

echo "================================================================================"
echo "🧪 AIM SDK COMPREHENSIVE TEST SUITE"
echo "================================================================================"
echo ""
echo "This test suite verifies ALL claims made in the AIM SDK documentation:"
echo "  ✅ ONE LINE secure() function"
echo "  ✅ Automatic capability detection"
echo "  ✅ Automatic MCP server detection"
echo "  ✅ @perform_action decorator"
echo "  ✅ Cryptographic signing (Ed25519)"
echo "  ✅ Credential storage"
echo "  ✅ Trust score tracking"
echo "  ✅ Audit trail logging"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test results tracking
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Function to run a test
run_test() {
    local test_name=$1
    local test_file=$2

    echo ""
    echo "================================================================================"
    echo "🧪 Running: $test_name"
    echo "================================================================================"

    TESTS_TOTAL=$((TESTS_TOTAL + 1))

    if python3 "$test_file"; then
        echo -e "${GREEN}✅ PASSED: $test_name${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        echo -e "${RED}❌ FAILED: $test_name${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Ensure we're in the right directory
cd "$(dirname "$0")"

# Check if .env exists
if [ ! -f .env ]; then
    echo -e "${YELLOW}⚠️  Warning: .env file not found${NC}"
    echo "Creating .env from template..."
    cat > .env << 'EOF'
# AIM SDK Testing Environment Variables
AIM_URL=https://aim-prod-backend.graypebble-c7e67ab8.canadacentral.azurecontainerapps.io
MOCK_MODE=true
LOG_LEVEL=DEBUG
EOF
    echo "✅ Created .env file"
fi

# Install dependencies
echo "📦 Installing dependencies..."
pip install -q -r requirements.txt
pip install -q -e /Users/decimai/workspace/aim-sdk-python

echo ""
echo "Starting tests..."
echo ""

# Run all tests
run_test "Test 1: secure() Function" "test_01_secure_function.py"
run_test "Test 2: Capability Detection" "test_02_capability_detection.py"
run_test "Test 3: MCP Detection" "test_03_mcp_detection.py"
run_test "Test 4: @perform_action Decorator" "test_04_perform_action_decorator.py"
run_test "Weather Agent SDK Demo" "weather_agent_sdk_demo.py"

# Print final summary
echo ""
echo "================================================================================"
echo "📊 TEST SUITE SUMMARY"
echo "================================================================================"
echo ""
echo "Total Tests:  $TESTS_TOTAL"
echo -e "${GREEN}Passed:       $TESTS_PASSED${NC}"

if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Failed:       $TESTS_FAILED${NC}"
else
    echo "Failed:       $TESTS_FAILED"
fi

echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}================================================================================"
    echo "✅ ALL TESTS PASSED!"
    echo "================================================================================${NC}"
    echo ""
    echo "SDK Claims Verified:"
    echo "  ✅ ONE LINE secure() registration works"
    echo "  ✅ Ed25519 cryptographic keys generated automatically"
    echo "  ✅ Credentials stored securely in ~/.aim/credentials.json"
    echo "  ✅ Automatic capability detection working"
    echo "  ✅ Automatic MCP server detection working"
    echo "  ✅ @perform_action decorator functioning correctly"
    echo "  ✅ Action verification and audit trail working"
    echo ""
    exit 0
else
    echo -e "${RED}================================================================================"
    echo "❌ SOME TESTS FAILED"
    echo "================================================================================${NC}"
    echo ""
    echo "Please review the test output above for details."
    echo ""
    exit 1
fi
