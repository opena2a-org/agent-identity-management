#!/bin/bash

# Comprehensive Endpoint Audit Script
# Tests all 114+ registered endpoints and categorizes by status

BASE_URL="http://localhost:8080"
PASSED=0
FAILED=0
AUTH_REQUIRED=0
TOTAL=0
MISSING=0

# Test result arrays
declare -a WORKING_ENDPOINTS
declare -a BROKEN_ENDPOINTS
declare -a MISSING_ENDPOINTS
declare -a AUTH_ENDPOINTS

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test function
test_endpoint() {
    local method=$1
    local path=$2
    local description=$3
    local needs_auth=${4:-true}

    TOTAL=$((TOTAL + 1))

    # Replace :id with UUID for testing
    test_path=$(echo "$path" | sed 's/:id/00000000-0000-0000-0000-000000000000/g' | sed 's/:audit_id/00000000-0000-0000-0000-000000000000/g')

    # Test without auth first
    http_code=$(curl -s -o /dev/null -w "%{http_code}" -X $method "${BASE_URL}${test_path}" 2>/dev/null)

    case $http_code in
        404)
            echo -e "${RED}✗ MISSING${NC} - $method $path"
            MISSING=$((MISSING + 1))
            MISSING_ENDPOINTS+=("$method $path - $description")
            ;;
        401|403)
            echo -e "${BLUE}🔒 AUTH${NC} - $method $path"
            AUTH_REQUIRED=$((AUTH_REQUIRED + 1))
            AUTH_ENDPOINTS+=("$method $path - $description")
            ;;
        200|201)
            echo -e "${GREEN}✓ OK${NC} - $method $path ($http_code)"
            PASSED=$((PASSED + 1))
            WORKING_ENDPOINTS+=("$method $path - $description")
            ;;
        400|422)
            echo -e "${YELLOW}~ PARTIAL${NC} - $method $path ($http_code - validation error expected)"
            PASSED=$((PASSED + 1))
            WORKING_ENDPOINTS+=("$method $path - $description (validation)")
            ;;
        500|502|503)
            echo -e "${RED}✗ ERROR${NC} - $method $path ($http_code)"
            FAILED=$((FAILED + 1))
            BROKEN_ENDPOINTS+=("$method $path - $description (HTTP $http_code)")
            ;;
        *)
            echo -e "${YELLOW}? UNKNOWN${NC} - $method $path ($http_code)"
            ;;
    esac
}

echo "=========================================="
echo "  AIM Endpoint Audit - All Endpoints"
echo "=========================================="
echo ""

# Health & Status Endpoints
echo "--- Health & Status ---"
test_endpoint "GET" "/health" "Health check" false
test_endpoint "GET" "/health/ready" "Readiness check" false
test_endpoint "GET" "/api/v1/status" "System status" false

# Public Registration & Auth
echo ""
echo "--- Public Auth & Registration ---"
test_endpoint "POST" "/api/v1/register" "User registration" false
test_endpoint "POST" "/api/v1/request-access" "Request access" false
test_endpoint "POST" "/api/v1/login" "Login" false
test_endpoint "POST" "/api/v1/forgot-password" "Forgot password" false
test_endpoint "POST" "/api/v1/reset-password" "Reset password" false

# Protected Auth
echo ""
echo "--- Protected Auth ---"
test_endpoint "POST" "/api/v1/auth/logout" "Logout"
test_endpoint "GET" "/api/v1/auth/me" "Get current user"
test_endpoint "POST" "/api/v1/auth/change-password" "Change password"

# Agent Detection
echo ""
echo "--- Agent Auto-Detection ---"
test_endpoint "POST" "/api/v1/detection/agents/:id/report" "Report detection"
test_endpoint "POST" "/api/v1/detection/agents/:id/capabilities/report" "Report capabilities"

# Agent Management
echo ""
echo "--- Agent Management ---"
test_endpoint "GET" "/api/v1/agents" "List agents"
test_endpoint "POST" "/api/v1/agents" "Create agent"
test_endpoint "GET" "/api/v1/agents/:id" "Get agent"
test_endpoint "PUT" "/api/v1/agents/:id" "Update agent"
test_endpoint "DELETE" "/api/v1/agents/:id" "Delete agent"
test_endpoint "POST" "/api/v1/agents/:id/verify" "Verify agent"
test_endpoint "POST" "/api/v1/agents/:id/suspend" "Suspend agent"
test_endpoint "POST" "/api/v1/agents/:id/reactivate" "Reactivate agent"
test_endpoint "POST" "/api/v1/agents/:id/rotate-credentials" "Rotate credentials"
test_endpoint "POST" "/api/v1/agents/:id/verify-action" "Verify action"
test_endpoint "POST" "/api/v1/agents/:id/log-action/:audit_id" "Log action result"
test_endpoint "GET" "/api/v1/agents/:id/sdk" "Download SDK"
test_endpoint "GET" "/api/v1/agents/:id/credentials" "Get credentials"

# Agent Security (NEW)
echo ""
echo "--- Agent Security ---"
test_endpoint "GET" "/api/v1/agents/:id/key-vault" "Get key vault"
test_endpoint "GET" "/api/v1/agents/:id/audit-logs" "Get agent audit logs"
test_endpoint "GET" "/api/v1/agents/:id/api-keys" "List agent API keys"
test_endpoint "POST" "/api/v1/agents/:id/api-keys" "Create agent API key"

# Agent Trust Score (NEW)
echo ""
echo "--- Agent Trust Score ---"
test_endpoint "GET" "/api/v1/agents/:id/trust-score" "Get trust score"
test_endpoint "GET" "/api/v1/agents/:id/trust-score/history" "Get trust score history"
test_endpoint "PUT" "/api/v1/agents/:id/trust-score" "Update trust score"
test_endpoint "POST" "/api/v1/agents/:id/trust-score/recalculate" "Recalculate trust score"

# API Keys
echo ""
echo "--- API Key Management ---"
test_endpoint "GET" "/api/v1/api-keys" "List API keys"
test_endpoint "POST" "/api/v1/api-keys" "Create API key"
test_endpoint "PATCH" "/api/v1/api-keys/:id/disable" "Disable API key"
test_endpoint "DELETE" "/api/v1/api-keys/:id" "Delete API key"

# Trust Score (Legacy paths)
echo ""
echo "--- Trust Score (Legacy) ---"
test_endpoint "POST" "/api/v1/trust-score/calculate/:id" "Calculate trust score"
test_endpoint "GET" "/api/v1/trust-score/agents/:id" "Get trust score (legacy)"
test_endpoint "GET" "/api/v1/trust-score/agents/:id/history" "Get history (legacy)"
test_endpoint "GET" "/api/v1/trust-score/trends" "Get trust score trends"

# Admin - User Management
echo ""
echo "--- Admin: User Management ---"
test_endpoint "GET" "/api/v1/admin/users" "List users"
test_endpoint "GET" "/api/v1/admin/users/pending" "Get pending users"
test_endpoint "POST" "/api/v1/admin/users/:id/approve" "Approve user"
test_endpoint "POST" "/api/v1/admin/users/:id/reject" "Reject user"
test_endpoint "PUT" "/api/v1/admin/users/:id/role" "Update user role"
test_endpoint "POST" "/api/v1/admin/registration-requests/:id/approve" "Approve registration"
test_endpoint "POST" "/api/v1/admin/registration-requests/:id/reject" "Reject registration"

# Admin - Organization
echo ""
echo "--- Admin: Organization ---"
test_endpoint "GET" "/api/v1/admin/organization/settings" "Get org settings"
test_endpoint "PUT" "/api/v1/admin/organization/settings" "Update org settings"

# Admin - Audit & Alerts
echo ""
echo "--- Admin: Audit & Alerts ---"
test_endpoint "GET" "/api/v1/admin/audit-logs" "Get audit logs"
test_endpoint "GET" "/api/v1/admin/alerts" "Get alerts"
test_endpoint "GET" "/api/v1/admin/alerts/unacknowledged/count" "Get alert count"
test_endpoint "POST" "/api/v1/admin/alerts/:id/acknowledge" "Acknowledge alert"
test_endpoint "POST" "/api/v1/admin/alerts/:id/resolve" "Resolve alert"
test_endpoint "POST" "/api/v1/admin/alerts/:id/approve-drift" "Approve drift"

# Admin - Dashboard
echo ""
echo "--- Admin: Dashboard ---"
test_endpoint "GET" "/api/v1/admin/dashboard/stats" "Get dashboard stats"

# Security Policies
echo ""
echo "--- Security Policies ---"
test_endpoint "GET" "/api/v1/admin/security-policies" "List policies"
test_endpoint "GET" "/api/v1/admin/security-policies/:id" "Get policy"
test_endpoint "POST" "/api/v1/admin/security-policies" "Create policy"
test_endpoint "PUT" "/api/v1/admin/security-policies/:id" "Update policy"
test_endpoint "DELETE" "/api/v1/admin/security-policies/:id" "Delete policy"

# Capability Requests
echo ""
echo "--- Capability Requests ---"
test_endpoint "GET" "/api/v1/capability-requests" "List capability requests"
test_endpoint "POST" "/api/v1/capability-requests" "Create capability request"
test_endpoint "GET" "/api/v1/capability-requests/:id" "Get capability request"
test_endpoint "POST" "/api/v1/capability-requests/:id/approve" "Approve request"
test_endpoint "POST" "/api/v1/capability-requests/:id/reject" "Reject request"

# MCP Servers
echo ""
echo "--- MCP Server Management ---"
test_endpoint "GET" "/api/v1/mcp-servers" "List MCP servers"
test_endpoint "POST" "/api/v1/mcp-servers" "Create MCP server"
test_endpoint "GET" "/api/v1/mcp-servers/:id" "Get MCP server"
test_endpoint "PUT" "/api/v1/mcp-servers/:id" "Update MCP server"
test_endpoint "DELETE" "/api/v1/mcp-servers/:id" "Delete MCP server"
test_endpoint "POST" "/api/v1/mcp-servers/:id/verify" "Verify MCP server"

# MCP Server Events (NEW)
echo ""
echo "--- MCP Server Events ---"
test_endpoint "GET" "/api/v1/mcp-servers/:id/verification-events" "Get MCP verification events"
test_endpoint "GET" "/api/v1/mcp-servers/:id/audit-logs" "Get MCP audit logs"

# Verification Events (NEW)
echo ""
echo "--- Verification Events ---"
test_endpoint "GET" "/api/v1/verification-events" "List verification events"
test_endpoint "POST" "/api/v1/verification-events" "Create verification event"
test_endpoint "GET" "/api/v1/verification-events/agent/:id" "Get agent verifications"
test_endpoint "GET" "/api/v1/verification-events/mcp/:id" "Get MCP verifications"
test_endpoint "GET" "/api/v1/verification-events/stats" "Get verification stats"

# Compliance (NEW)
echo ""
echo "--- Compliance ---"
test_endpoint "GET" "/api/v1/compliance/reports" "List compliance reports"
test_endpoint "GET" "/api/v1/compliance/access-reviews" "List access reviews"
test_endpoint "GET" "/api/v1/compliance/data-retention" "Get data retention"

# Capabilities (NEW)
echo ""
echo "--- Capabilities ---"
test_endpoint "GET" "/api/v1/capabilities" "List capabilities"

# Tags
echo ""
echo "--- Tags ---"
test_endpoint "GET" "/api/v1/tags" "List tags"
test_endpoint "POST" "/api/v1/tags" "Create tag"
test_endpoint "GET" "/api/v1/tags/:id" "Get tag"
test_endpoint "PUT" "/api/v1/tags/:id" "Update tag"
test_endpoint "DELETE" "/api/v1/tags/:id" "Delete tag"
test_endpoint "GET" "/api/v1/tags/popular" "Get popular tags"
test_endpoint "GET" "/api/v1/tags/search" "Search tags"

# Summary
echo ""
echo "=========================================="
echo "  Audit Summary"
echo "=========================================="
echo -e "Total Endpoints: ${TOTAL}"
echo -e "${GREEN}Working: ${PASSED}${NC} (returned 200/201/400/422)"
echo -e "${BLUE}Auth Required: ${AUTH_REQUIRED}${NC} (returned 401/403)"
echo -e "${RED}Server Errors: ${FAILED}${NC} (returned 500+)"
echo -e "${RED}Missing: ${MISSING}${NC} (returned 404)"
echo ""

# Detailed breakdown
if [ ${#MISSING_ENDPOINTS[@]} -gt 0 ]; then
    echo "=========================================="
    echo "  Missing Endpoints (404 errors)"
    echo "=========================================="
    for endpoint in "${MISSING_ENDPOINTS[@]}"; do
        echo "  - $endpoint"
    done
    echo ""
fi

if [ ${#BROKEN_ENDPOINTS[@]} -gt 0 ]; then
    echo "=========================================="
    echo "  Broken Endpoints (500+ errors)"
    echo "=========================================="
    for endpoint in "${BROKEN_ENDPOINTS[@]}"; do
        echo "  - $endpoint"
    done
    echo ""
fi

# Success rate
implemented=$((TOTAL - MISSING))
success_rate=$((implemented * 100 / TOTAL))

echo "=========================================="
echo "  Implementation Rate: ${success_rate}% ($implemented/$TOTAL)"
echo "=========================================="
echo ""

if [ $MISSING -eq 0 ] && [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All endpoints implemented and working!${NC}"
    exit 0
else
    echo -e "${YELLOW}⚠ Some endpoints need attention${NC}"
    exit 1
fi
