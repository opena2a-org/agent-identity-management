#!/bin/bash
# Production Readiness - Layer 5: Security Scanning Script
# Run comprehensive security scans (OWASP, dependencies, containers)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
REPORT_DIR="$SCRIPT_DIR/../reports"

echo "🔒 AIM Security Scan - Layer 5"
echo "==============================="
echo ""

# Create report file
SECURITY_REPORT="$REPORT_DIR/security-report.md"
cat > "$SECURITY_REPORT" << EOF
# AIM Security Audit Report

**Date**: $(date +"%B %d, %Y")
**Status**: IN PROGRESS

## Summary
- Critical Vulnerabilities: TBD
- High Vulnerabilities: TBD
- Medium Vulnerabilities: TBD
- Low Vulnerabilities: TBD

## Scans Performed

EOF

echo "1️⃣  Scanning Go dependencies..."
echo "================================"
cd "$PROJECT_ROOT/apps/backend"

# Check if nancy is installed
if ! command -v nancy &> /dev/null; then
    echo "📥 Installing nancy..."
    go install github.com/sonatype-nexus-community/nancy@latest
fi

go list -json -m all | nancy sleuth --output=text > "$REPORT_DIR/backend-dependencies-scan.txt" || true

echo ""
echo "2️⃣  Scanning npm dependencies..."
echo "================================="
cd "$PROJECT_ROOT/apps/web"
npm audit --json > "$REPORT_DIR/frontend-dependencies-scan.json" || true
npm audit > "$REPORT_DIR/frontend-dependencies-scan.txt" || true

echo ""
echo "3️⃣  Scanning Docker images..."
echo "=============================="

# Check if trivy is installed
if ! command -v trivy &> /dev/null; then
    echo "⚠️  Trivy not installed. Skipping container scanning."
    echo "   Install: brew install aquasecurity/trivy/trivy"
else
    echo "Scanning backend image..."
    docker pull aimprodacr1760993976.azurecr.io/aim-backend:latest || echo "⚠️  Backend image not found"
    trivy image --severity HIGH,CRITICAL \
        aimprodacr1760993976.azurecr.io/aim-backend:latest \
        > "$REPORT_DIR/backend-container-scan.txt" || true

    echo "Scanning frontend image..."
    docker pull aimprodacr1760993976.azurecr.io/aim-frontend:latest || echo "⚠️  Frontend image not found"
    trivy image --severity HIGH,CRITICAL \
        aimprodacr1760993976.azurecr.io/aim-frontend:latest \
        > "$REPORT_DIR/frontend-container-scan.txt" || true
fi

echo ""
echo "4️⃣  Analyzing scan results..."
echo "=============================="

# Count vulnerabilities
BACKEND_DEPS_VULNS=$(grep -c "CVE" "$REPORT_DIR/backend-dependencies-scan.txt" || echo "0")
FRONTEND_CRITICAL=$(jq '.metadata.vulnerabilities.critical // 0' "$REPORT_DIR/frontend-dependencies-scan.json")
FRONTEND_HIGH=$(jq '.metadata.vulnerabilities.high // 0' "$REPORT_DIR/frontend-dependencies-scan.json")

echo "Backend dependencies: $BACKEND_DEPS_VULNS vulnerabilities"
echo "Frontend dependencies: $FRONTEND_CRITICAL critical, $FRONTEND_HIGH high"

# Update report
cat >> "$SECURITY_REPORT" << EOF

### 1. Dependency Scanning

**Backend (Go modules)**:
- Vulnerabilities found: $BACKEND_DEPS_VULNS
- Details: See \`backend-dependencies-scan.txt\`

**Frontend (npm)**:
- Critical: $FRONTEND_CRITICAL
- High: $FRONTEND_HIGH
- Details: See \`frontend-dependencies-scan.json\`

### 2. Container Scanning

**Status**: See \`*-container-scan.txt\` files

### 3. OWASP Top 10 Compliance

**Status**: Manual testing required
- See \`docs/layer-5-security.md\` for test procedures

### 4. Penetration Testing

**Status**: Manual testing required
- Automated: Use OWASP ZAP
- Manual: See security testing checklist

EOF

echo ""
echo "✅ Security scans complete!"
echo ""
echo "📋 Reports generated:"
echo "  - Summary:            $SECURITY_REPORT"
echo "  - Backend deps:       $REPORT_DIR/backend-dependencies-scan.txt"
echo "  - Frontend deps:      $REPORT_DIR/frontend-dependencies-scan.json"
echo "  - Container scans:    $REPORT_DIR/*-container-scan.txt"
echo ""
echo "⚠️  Next steps:"
echo "  1. Review all vulnerability reports"
echo "  2. Fix critical and high-severity issues"
echo "  3. Run OWASP Top 10 compliance tests (manual)"
echo "  4. Perform penetration testing"
echo "  5. Update security report with final results"
