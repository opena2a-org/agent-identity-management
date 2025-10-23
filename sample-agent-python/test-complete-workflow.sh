#!/bin/bash

# AIM Complete Security Workflow Test
# ====================================
# This script runs comprehensive tests for:
# - Activity logging
# - Security alerts
# - Capability violations
# - Capability requests

set -e

echo ""
echo "╔════════════════════════════════════════════════════════════════════════════╗"
echo "║                 AIM COMPLETE SECURITY WORKFLOW TEST                        ║"
echo "╚════════════════════════════════════════════════════════════════════════════╝"
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Check if we're in the correct directory
if [ ! -d "aim-sdk-python" ]; then
    echo -e "${RED}❌ Error: aim-sdk-python directory not found${NC}"
    echo "   Please ensure you're in the sample-agent-python directory"
    echo ""
    exit 1
fi

# Check if backend is running
echo -e "${BLUE}🔍 Checking backend status...${NC}"
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo -e "${RED}❌ Error: Backend not responding on localhost:8080${NC}"
    echo ""
    echo "Please start the backend first:"
    echo -e "${YELLOW}   cd apps/backend && go run cmd/server/main.go${NC}"
    echo ""
    exit 1
fi
echo -e "${GREEN}✅ Backend is running${NC}"
echo ""

# Check if frontend is running (optional)
echo -e "${BLUE}🔍 Checking frontend status...${NC}"
if curl -s http://localhost:3000 > /dev/null 2>&1; then
    echo -e "${GREEN}✅ Frontend is running${NC}"
else
    echo -e "${YELLOW}⚠️  Frontend not detected (optional)${NC}"
    echo "   Start frontend to view results in dashboard:"
    echo -e "${YELLOW}   cd apps/web && npm run dev${NC}"
fi
echo ""

# Check for required environment variables
if [ -z "$AIM_API_KEY" ]; then
    echo -e "${YELLOW}⚠️  AIM_API_KEY not set, using default test key${NC}"
    echo ""
fi

# Display configuration
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${BLUE}📋 Configuration:${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "   API URL: ${AIM_API_URL:-http://localhost:8080}"
echo "   Dashboard: ${AIM_DASHBOARD_URL:-http://localhost:3000}"
echo "   Agent Name: ${AGENT_NAME:-complete-workflow-test-<timestamp>}"
echo ""

# Run the test
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo -e "${GREEN}▶️  Running Complete Workflow Test...${NC}"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Make the Python script executable
chmod +x test-complete-workflow.py

# Run the test
python3 test-complete-workflow.py

# Check exit status
if [ $? -eq 0 ]; then
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${GREEN}✅ WORKFLOW TEST COMPLETED SUCCESSFULLY!${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo -e "${BLUE}📊 Quick Links:${NC}"
    echo "   • Security Alerts:      http://localhost:3000/dashboard/security/alerts"
    echo "   • All Agents:           http://localhost:3000/dashboard/agents"
    echo "   • Capability Requests:  http://localhost:3000/dashboard/admin/capability-requests"
    echo "   • Audit Logs:           http://localhost:3000/dashboard/audit"
    echo ""
    echo -e "${BLUE}💡 What to do next:${NC}"
    echo "   1. Open the dashboard and review the security alerts"
    echo "   2. Check the agent's trust score and violation history"
    echo "   3. Approve or reject the capability requests in the admin panel"
    echo "   4. Review the complete audit trail"
    echo ""
    echo -e "${GREEN}🎉 All tests passed! Your AIM system is working correctly.${NC}"
    echo ""
else
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo -e "${RED}❌ WORKFLOW TEST FAILED${NC}"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo -e "${YELLOW}Please check the error messages above and ensure:${NC}"
    echo "   • Backend is running (localhost:8080)"
    echo "   • Database is accessible"
    echo "   • API key is valid"
    echo "   • SDK is properly installed"
    echo ""
    exit 1
fi

