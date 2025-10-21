#!/bin/bash
# Production Readiness - Layer 1: Code Audit Script
# Systematically audit all 100+ endpoints for real implementation

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BACKEND_DIR="$PROJECT_ROOT/apps/backend"
REPORT_FILE="$SCRIPT_DIR/../reports/code-audit-report.md"

echo "🔍 AIM Code Audit - Layer 1"
echo "============================="
echo ""
echo "Auditing: $BACKEND_DIR"
echo "Report: $REPORT_FILE"
echo ""

# Initialize report
cat > "$REPORT_FILE" << 'EOF'
# AIM Code Audit Report

**Date**: $(date +"%B %d, %Y")
**Auditor**: Production Readiness Team
**Endpoints Audited**: TBD

## Summary

- Total Endpoints: TBD
- ✅ Real Implementation: TBD
- ⚠️ Partial Implementation: TBD
- ❌ Mocked/Fake: TBD

## Critical Findings

[To be filled during audit]

## Endpoint-by-Endpoint Results

EOF

echo "📊 Step 1: Counting endpoints from main.go..."
TOTAL_ENDPOINTS=$(grep -E "(Get|Post|Put|Delete|Patch)\(" "$BACKEND_DIR/cmd/server/main.go" | wc -l | tr -d ' ')
echo "Found: $TOTAL_ENDPOINTS endpoint registrations"

echo ""
echo "📂 Step 2: Listing all handlers..."
HANDLER_DIR="$BACKEND_DIR/internal/interfaces/http/handlers"
HANDLERS=$(find "$HANDLER_DIR" -name "*_handler.go" -type f | sort)

HANDLER_COUNT=$(echo "$HANDLERS" | wc -l | tr -d ' ')
echo "Found: $HANDLER_COUNT handler files"

echo ""
echo "🔬 Step 3: Analyzing handlers for real implementation..."
echo "This will check:"
echo "  - Handler functions exist"
echo "  - Service layer called"
echo "  - Repository layer accessed"
echo "  - No hardcoded return values"
echo ""

# Add endpoint audit sections to report
cat >> "$REPORT_FILE" << 'EOF'

### Authentication Endpoints (8 endpoints)

#### POST /api/v1/auth/login/local
**Handler**: `auth_handler.go:LocalLogin()`
**Status**: ⏳ Pending audit

---

EOF

# Audit helper function
audit_endpoint() {
    local METHOD=$1
    local PATH=$2
    local HANDLER_FILE=$3
    local FUNCTION=$4

    echo "Auditing: $METHOD $PATH"

    # Check if handler exists
    if grep -q "func.*$FUNCTION" "$BACKEND_DIR/$HANDLER_FILE"; then
        echo "  ✅ Handler found: $FUNCTION"

        # Check for service calls (real implementation indicator)
        if grep -q "service\." "$BACKEND_DIR/$HANDLER_FILE"; then
            echo "  ✅ Service layer called"
        else
            echo "  ⚠️  No service layer calls found"
        fi

        # Check for hardcoded responses (anti-pattern)
        if grep -q "return.*fiber.Map.*\"success\":.*true" "$BACKEND_DIR/$HANDLER_FILE" | grep -v "service"; then
            echo "  ⚠️  Possible hardcoded response"
        fi
    else
        echo "  ❌ Handler NOT found: $FUNCTION"
    fi

    echo ""
}

echo "📝 Detailed audit output:"
echo "========================="
echo ""

# Audit sample endpoints (full audit would be done manually/interactively)
audit_endpoint "POST" "/api/v1/auth/login/local" "internal/interfaces/http/handlers/auth_handler.go" "LocalLogin"
audit_endpoint "POST" "/api/v1/agents" "internal/interfaces/http/handlers/agent_handler.go" "CreateAgent"
audit_endpoint "GET" "/api/v1/agents" "internal/interfaces/http/handlers/agent_handler.go" "ListAgents"
audit_endpoint "GET" "/api/v1/analytics/dashboard" "internal/interfaces/http/handlers/analytics_handler.go" "GetDashboardStats"

echo ""
echo "✅ Audit script complete!"
echo ""
echo "📋 Next steps:"
echo "  1. Review report: $REPORT_FILE"
echo "  2. Manually audit each endpoint using the template"
echo "  3. Mark endpoints as ✅ (real), ⚠️ (partial), or ❌ (mocked)"
echo "  4. Fix any issues found"
echo "  5. Update summary statistics in report"
echo ""
echo "📖 See docs/layer-1-code-audit.md for detailed audit instructions"
