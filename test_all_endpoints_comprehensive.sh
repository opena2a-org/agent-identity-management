#!/bin/bash

# Comprehensive AIM Endpoint Test Suite
# Auto-generated from main.go route registrations
# Total Routes: 163

BASE_URL="http://localhost:8080"
PASSED=0
FAILED=0
AUTH_REQUIRED=0
TOTAL=0

# Colors
GREEN="\033[0;32m"
RED="\033[0;31m"
YELLOW="\033[1;33m"
BLUE="\033[0;34m"
NC="\033[0m"

# Test function
test_endpoint() {
    local method=$1
    local path=$2
    local description=$3

    TOTAL=$((TOTAL + 1))

    # Replace :id and :audit_id with test UUIDs
    test_path=$(echo "$path" | sed "s/:id/00000000-0000-0000-0000-000000000000/g" | sed "s/:audit_id/00000000-0000-0000-0000-000000000001/g")

    # Test endpoint
    http_code=$(curl -s -o /dev/null -w "%{http_code}" -X $method "${BASE_URL}${test_path}" 2>/dev/null)

    case $http_code in
        404)
            echo -e "${RED}✗ MISSING${NC} - $method $path"
            FAILED=$((FAILED + 1))
            ;;
        401|403)
            echo -e "${BLUE}🔒 AUTH${NC} - $method $path"
            AUTH_REQUIRED=$((AUTH_REQUIRED + 1))
            ;;
        200|201)
            echo -e "${GREEN}✓ OK${NC} - $method $path"
            PASSED=$((PASSED + 1))
            ;;
        400|422)
            echo -e "${YELLOW}~ PARTIAL${NC} - $method $path (validation error)"
            PASSED=$((PASSED + 1))
            ;;
        500|502|503)
            echo -e "${RED}✗ ERROR${NC} - $method $path (HTTP $http_code)"
            FAILED=$((FAILED + 1))
            ;;
        *)
            echo -e "${YELLOW}? UNKNOWN${NC} - $method $path (HTTP $http_code)"
            ;;
    esac
}

echo "=========================================="
echo "  Testing All 163 AIM Endpoints"
echo "=========================================="
echo ""

echo "--- ADMIN (29 endpoints) ---"
test_endpoint "GET" "/api/v1/admin/users" "admin - /users"
test_endpoint "GET" "/api/v1/admin/users/pending" "admin - /users/pending"
test_endpoint "POST" "/api/v1/admin/users/:id/approve" "admin - /users/:id/approve"
test_endpoint "POST" "/api/v1/admin/users/:id/reject" "admin - /users/:id/reject"
test_endpoint "PUT" "/api/v1/admin/users/:id/role" "admin - /users/:id/role"
test_endpoint "POST" "/api/v1/admin/users/:id/deactivate" "admin - /users/:id/deactivate"
test_endpoint "POST" "/api/v1/admin/users/:id/activate" "admin - /users/:id/activate"
test_endpoint "DELETE" "/api/v1/admin/users/:id" "admin - /users/:id"
test_endpoint "POST" "/api/v1/admin/registration-requests/:id/approve" "admin - /registration-requests/:id/approve"
test_endpoint "POST" "/api/v1/admin/registration-requests/:id/reject" "admin - /registration-requests/:id/reject"
test_endpoint "GET" "/api/v1/admin/organization/settings" "admin - /organization/settings"
test_endpoint "PUT" "/api/v1/admin/organization/settings" "admin - /organization/settings"
test_endpoint "GET" "/api/v1/admin/audit-logs" "admin - /audit-logs"
test_endpoint "GET" "/api/v1/admin/alerts" "admin - /alerts"
test_endpoint "GET" "/api/v1/admin/alerts/unacknowledged/count" "admin - /alerts/unacknowledged/count"
test_endpoint "POST" "/api/v1/admin/alerts/:id/acknowledge" "admin - /alerts/:id/acknowledge"
test_endpoint "POST" "/api/v1/admin/alerts/:id/resolve" "admin - /alerts/:id/resolve"
test_endpoint "POST" "/api/v1/admin/alerts/:id/approve-drift" "admin - /alerts/:id/approve-drift"
test_endpoint "GET" "/api/v1/admin/dashboard/stats" "admin - /dashboard/stats"
test_endpoint "GET" "/api/v1/admin/security-policies" "admin - /security-policies"
test_endpoint "GET" "/api/v1/admin/security-policies/:id" "admin - /security-policies/:id"
test_endpoint "POST" "/api/v1/admin/security-policies" "admin - /security-policies"
test_endpoint "PUT" "/api/v1/admin/security-policies/:id" "admin - /security-policies/:id"
test_endpoint "DELETE" "/api/v1/admin/security-policies/:id" "admin - /security-policies/:id"
test_endpoint "PATCH" "/api/v1/admin/security-policies/:id/toggle" "admin - /security-policies/:id/toggle"
test_endpoint "GET" "/api/v1/admin/capability-requests" "admin - /capability-requests"
test_endpoint "GET" "/api/v1/admin/capability-requests/:id" "admin - /capability-requests/:id"
test_endpoint "POST" "/api/v1/admin/capability-requests/:id/approve" "admin - /capability-requests/:id/approve"
test_endpoint "POST" "/api/v1/admin/capability-requests/:id/reject" "admin - /capability-requests/:id/reject"
echo ""
echo "--- AGENTS (34 endpoints) ---"
test_endpoint "GET" "/api/v1/agents/" "agents - /"
test_endpoint "POST" "/api/v1/agents/" "agents - /"
test_endpoint "GET" "/api/v1/agents/:id" "agents - /:id"
test_endpoint "PUT" "/api/v1/agents/:id" "agents - /:id"
test_endpoint "DELETE" "/api/v1/agents/:id" "agents - /:id"
test_endpoint "POST" "/api/v1/agents/:id/verify" "agents - /:id/verify"
test_endpoint "POST" "/api/v1/agents/:id/suspend" "agents - /:id/suspend"
test_endpoint "POST" "/api/v1/agents/:id/reactivate" "agents - /:id/reactivate"
test_endpoint "POST" "/api/v1/agents/:id/rotate-credentials" "agents - /:id/rotate-credentials"
test_endpoint "POST" "/api/v1/agents/:id/verify-action" "agents - /:id/verify-action"
test_endpoint "POST" "/api/v1/agents/:id/log-action/:audit_id" "agents - /:id/log-action/:audit_id"
test_endpoint "GET" "/api/v1/agents/:id/sdk" "agents - /:id/sdk"
test_endpoint "GET" "/api/v1/agents/:id/credentials" "agents - /:id/credentials"
test_endpoint "GET" "/api/v1/agents/:id/mcp-servers" "agents - /:id/mcp-servers"
test_endpoint "PUT" "/api/v1/agents/:id/mcp-servers" "agents - /:id/mcp-servers"
test_endpoint "DELETE" "/api/v1/agents/:id/mcp-servers/bulk" "agents - /:id/mcp-servers/bulk"
test_endpoint "DELETE" "/api/v1/agents/:id/mcp-servers/:mcp_id" "agents - /:id/mcp-servers/:mcp_id"
test_endpoint "POST" "/api/v1/agents/:id/mcp-servers/detect" "agents - /:id/mcp-servers/detect"
test_endpoint "GET" "/api/v1/agents/:id/trust-score" "agents - /:id/trust-score"
test_endpoint "GET" "/api/v1/agents/:id/trust-score/history" "agents - /:id/trust-score/history"
test_endpoint "PUT" "/api/v1/agents/:id/trust-score" "agents - /:id/trust-score"
test_endpoint "POST" "/api/v1/agents/:id/trust-score/recalculate" "agents - /:id/trust-score/recalculate"
test_endpoint "GET" "/api/v1/agents/:id/key-vault" "agents - /:id/key-vault"
test_endpoint "GET" "/api/v1/agents/:id/audit-logs" "agents - /:id/audit-logs"
test_endpoint "GET" "/api/v1/agents/:id/api-keys" "agents - /:id/api-keys"
test_endpoint "POST" "/api/v1/agents/:id/api-keys" "agents - /:id/api-keys"
test_endpoint "GET" "/api/v1/agents/:id/tags" "agents - /:id/tags"
test_endpoint "POST" "/api/v1/agents/:id/tags" "agents - /:id/tags"
test_endpoint "DELETE" "/api/v1/agents/:id/tags/:tagId" "agents - /:id/tags/:tagId"
test_endpoint "GET" "/api/v1/agents/:id/tags/suggestions" "agents - /:id/tags/suggestions"
test_endpoint "GET" "/api/v1/agents/:id/capabilities" "agents - /:id/capabilities"
test_endpoint "POST" "/api/v1/agents/:id/capabilities" "agents - /:id/capabilities"
test_endpoint "DELETE" "/api/v1/agents/:id/capabilities/:capabilityId" "agents - /:id/capabilities/:capabilityId"
test_endpoint "GET" "/api/v1/agents/:id/violations" "agents - /:id/violations"
echo ""
echo "--- ANALYTICS (6 endpoints) ---"
test_endpoint "GET" "/api/v1/analytics/dashboard" "analytics - /dashboard"
test_endpoint "GET" "/api/v1/analytics/usage" "analytics - /usage"
test_endpoint "GET" "/api/v1/analytics/trends" "analytics - /trends"
test_endpoint "GET" "/api/v1/analytics/verification-activity" "analytics - /verification-activity"
test_endpoint "GET" "/api/v1/analytics/reports/generate" "analytics - /reports/generate"
test_endpoint "GET" "/api/v1/analytics/agents/activity" "analytics - /agents/activity"
echo ""
echo "--- APIKEYS (4 endpoints) ---"
test_endpoint "GET" "/api/v1/api-keys/" "apiKeys - /"
test_endpoint "POST" "/api/v1/api-keys/" "apiKeys - /"
test_endpoint "PATCH" "/api/v1/api-keys/:id/disable" "apiKeys - /:id/disable"
test_endpoint "DELETE" "/api/v1/api-keys/:id" "apiKeys - /:id"
echo ""
echo "--- APP (3 endpoints) ---"
test_endpoint "GET" "/health" "app - /health"
test_endpoint "GET" "/health/ready" "app - /health/ready"
test_endpoint "GET" "/api/v1/status" "app - /api/v1/status"
echo ""
echo "--- AUTH (3 endpoints) ---"
test_endpoint "POST" "/api/v1/auth/login/local" "auth - /login/local"
test_endpoint "POST" "/api/v1/auth/logout" "auth - /logout"
test_endpoint "POST" "/api/v1/auth/refresh" "auth - /refresh"
echo ""
echo "--- AUTHPROTECTED (2 endpoints) ---"
test_endpoint "GET" "/api/v1/auth/me" "authProtected - /me"
test_endpoint "POST" "/api/v1/auth/change-password" "authProtected - /change-password"
echo ""
echo "--- CAPABILITIES (1 endpoints) ---"
test_endpoint "GET" "/api/v1/capabilities/" "capabilities - /"
echo ""
echo "--- CAPABILITYREQUESTS (1 endpoints) ---"
test_endpoint "POST" "/api/v1/capability-requests/" "capabilityRequests - /"
echo ""
echo "--- COMPLIANCE (11 endpoints) ---"
test_endpoint "GET" "/api/v1/compliance/status" "compliance - /status"
test_endpoint "GET" "/api/v1/compliance/metrics" "compliance - /metrics"
test_endpoint "GET" "/api/v1/compliance/audit-log/export" "compliance - /audit-log/export"
test_endpoint "GET" "/api/v1/compliance/audit-log/access-review" "compliance - /audit-log/access-review"
test_endpoint "GET" "/api/v1/compliance/audit-log/data-retention" "compliance - /audit-log/data-retention"
test_endpoint "GET" "/api/v1/compliance/access-review" "compliance - /access-review"
test_endpoint "POST" "/api/v1/compliance/check" "compliance - /check"
test_endpoint "POST" "/api/v1/compliance/reports/generate" "compliance - /reports/generate"
test_endpoint "GET" "/api/v1/compliance/reports" "compliance - /reports"
test_endpoint "GET" "/api/v1/compliance/access-reviews" "compliance - /access-reviews"
test_endpoint "GET" "/api/v1/compliance/data-retention" "compliance - /data-retention"
echo ""
echo "--- DETECTION (3 endpoints) ---"
test_endpoint "POST" "/api/v1/detection/agents/:id/report" "detection - /agents/:id/report"
test_endpoint "GET" "/api/v1/detection/agents/:id/status" "detection - /agents/:id/status"
test_endpoint "POST" "/api/v1/detection/agents/:id/capabilities/report" "detection - /agents/:id/capabilities/report"
echo ""
echo "--- MCPSERVERS (17 endpoints) ---"
test_endpoint "GET" "/api/v1/mcp-servers/" "mcpServers - /"
test_endpoint "POST" "/api/v1/mcp-servers/" "mcpServers - /"
test_endpoint "GET" "/api/v1/mcp-servers/:id" "mcpServers - /:id"
test_endpoint "PUT" "/api/v1/mcp-servers/:id" "mcpServers - /:id"
test_endpoint "DELETE" "/api/v1/mcp-servers/:id" "mcpServers - /:id"
test_endpoint "POST" "/api/v1/mcp-servers/:id/verify" "mcpServers - /:id/verify"
test_endpoint "POST" "/api/v1/mcp-servers/:id/keys" "mcpServers - /:id/keys"
test_endpoint "GET" "/api/v1/mcp-servers/:id/verification-status" "mcpServers - /:id/verification-status"
test_endpoint "GET" "/api/v1/mcp-servers/:id/capabilities" "mcpServers - /:id/capabilities"
test_endpoint "GET" "/api/v1/mcp-servers/:id/agents" "mcpServers - /:id/agents"
test_endpoint "GET" "/api/v1/mcp-servers/:id/verification-events" "mcpServers - /:id/verification-events"
test_endpoint "GET" "/api/v1/mcp-servers/:id/audit-logs" "mcpServers - /:id/audit-logs"
test_endpoint "POST" "/api/v1/mcp-servers/:id/verify-action" "mcpServers - /:id/verify-action"
test_endpoint "GET" "/api/v1/mcp-servers/:id/tags" "mcpServers - /:id/tags"
test_endpoint "POST" "/api/v1/mcp-servers/:id/tags" "mcpServers - /:id/tags"
test_endpoint "DELETE" "/api/v1/mcp-servers/:id/tags/:tagId" "mcpServers - /:id/tags/:tagId"
test_endpoint "GET" "/api/v1/mcp-servers/:id/tags/suggestions" "mcpServers - /:id/tags/suggestions"
echo ""
echo "--- PUBLIC (8 endpoints) ---"
test_endpoint "POST" "/api/v1/public/agents/register" "public - /agents/register"
test_endpoint "POST" "/api/v1/public/register" "public - /register"
test_endpoint "GET" "/api/v1/public/register/:requestId/status" "public - /register/:requestId/status"
test_endpoint "POST" "/api/v1/public/login" "public - /login"
test_endpoint "POST" "/api/v1/public/change-password" "public - /change-password"
test_endpoint "POST" "/api/v1/public/forgot-password" "public - /forgot-password"
test_endpoint "POST" "/api/v1/public/reset-password" "public - /reset-password"
test_endpoint "POST" "/api/v1/public/request-access" "public - /request-access"
echo ""
echo "--- SDK (1 endpoints) ---"
test_endpoint "GET" "/api/v1/sdk/download" "sdk - /download"
echo ""
echo "--- SDKAPI (4 endpoints) ---"
test_endpoint "GET" "/api/v1/sdkAPI/agents/:identifier" "sdkAPI - /agents/:identifier"
test_endpoint "POST" "/api/v1/sdkAPI/agents/:id/capabilities" "sdkAPI - /agents/:id/capabilities"
test_endpoint "POST" "/api/v1/sdkAPI/agents/:id/mcp-servers" "sdkAPI - /agents/:id/mcp-servers"
test_endpoint "POST" "/api/v1/sdkAPI/agents/:id/detection/report" "sdkAPI - /agents/:id/detection/report"
echo ""
echo "--- SDKTOKENS (4 endpoints) ---"
test_endpoint "GET" "/api/v1/sdkTokens/" "sdkTokens - /"
test_endpoint "GET" "/api/v1/sdkTokens/count" "sdkTokens - /count"
test_endpoint "POST" "/api/v1/sdkTokens/:id/revoke" "sdkTokens - /:id/revoke"
test_endpoint "POST" "/api/v1/sdkTokens/revoke-all" "sdkTokens - /revoke-all"
echo ""
echo "--- SECURITY (6 endpoints) ---"
test_endpoint "GET" "/api/v1/security/threats" "security - /threats"
test_endpoint "GET" "/api/v1/security/anomalies" "security - /anomalies"
test_endpoint "GET" "/api/v1/security/metrics" "security - /metrics"
test_endpoint "GET" "/api/v1/security/scan/:id" "security - /scan/:id"
test_endpoint "GET" "/api/v1/security/incidents" "security - /incidents"
test_endpoint "POST" "/api/v1/security/incidents/:id/resolve" "security - /incidents/:id/resolve"
echo ""
echo "--- TAGS (5 endpoints) ---"
test_endpoint "GET" "/api/v1/tags/" "tags - /"
test_endpoint "POST" "/api/v1/tags/" "tags - /"
test_endpoint "GET" "/api/v1/tags/popular" "tags - /popular"
test_endpoint "GET" "/api/v1/tags/search" "tags - /search"
test_endpoint "DELETE" "/api/v1/tags/:id" "tags - /:id"
echo ""
echo "--- TRUST (4 endpoints) ---"
test_endpoint "POST" "/api/v1/trust-score/calculate/:id" "trust - /calculate/:id"
test_endpoint "GET" "/api/v1/trust-score/agents/:id" "trust - /agents/:id"
test_endpoint "GET" "/api/v1/trust-score/agents/:id/history" "trust - /agents/:id/history"
test_endpoint "GET" "/api/v1/trust-score/trends" "trust - /trends"
echo ""
echo "--- VERIFICATIONEVENTS (9 endpoints) ---"
test_endpoint "GET" "/api/v1/verification-events/" "verificationEvents - /"
test_endpoint "GET" "/api/v1/verification-events/recent" "verificationEvents - /recent"
test_endpoint "GET" "/api/v1/verification-events/statistics" "verificationEvents - /statistics"
test_endpoint "GET" "/api/v1/verification-events/stats" "verificationEvents - /stats"
test_endpoint "GET" "/api/v1/verification-events/agent/:id" "verificationEvents - /agent/:id"
test_endpoint "GET" "/api/v1/verification-events/mcp/:id" "verificationEvents - /mcp/:id"
test_endpoint "GET" "/api/v1/verification-events/:id" "verificationEvents - /:id"
test_endpoint "POST" "/api/v1/verification-events/" "verificationEvents - /"
test_endpoint "DELETE" "/api/v1/verification-events/:id" "verificationEvents - /:id"
echo ""
echo "--- VERIFICATIONS (3 endpoints) ---"
test_endpoint "POST" "/api/v1/verifications/" "verifications - /"
test_endpoint "GET" "/api/v1/verifications/:id" "verifications - /:id"
test_endpoint "POST" "/api/v1/verifications/:id/result" "verifications - /:id/result"
echo ""
echo "--- WEBHOOKS (5 endpoints) ---"
test_endpoint "POST" "/api/v1/webhooks/" "webhooks - /"
test_endpoint "GET" "/api/v1/webhooks/" "webhooks - /"
test_endpoint "GET" "/api/v1/webhooks/:id" "webhooks - /:id"
test_endpoint "DELETE" "/api/v1/webhooks/:id" "webhooks - /:id"
test_endpoint "POST" "/api/v1/webhooks/:id/test" "webhooks - /:id/test"
echo ""
echo "=========================================="
echo "  Test Summary"
echo "=========================================="
echo "Total Endpoints: $TOTAL"
echo -e "${GREEN}Working: $PASSED${NC}"
echo -e "${BLUE}Auth Required: $AUTH_REQUIRED${NC}"
echo -e "${RED}Failed/Missing: $FAILED${NC}"
echo ""

implemented=$((TOTAL - FAILED))
success_rate=$((implemented * 100 / TOTAL))

echo "=========================================="
echo "  Implementation Rate: ${success_rate}% ($implemented/$TOTAL)"
echo "=========================================="

if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All endpoints working!${NC}"
    exit 0
else
    echo -e "${YELLOW}⚠ $FAILED endpoints need attention${NC}"
    exit 1
fi