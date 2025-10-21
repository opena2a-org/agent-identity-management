#!/bin/bash
# Production Readiness - Layer 6: Performance Testing Script
# Run k6 load tests and benchmark critical endpoints

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEST_DIR="$SCRIPT_DIR/../tests/load"
REPORT_DIR="$SCRIPT_DIR/../reports"

echo "⚡ AIM Performance Tests - Layer 6"
echo "==================================="
echo ""

# Check if k6 is installed
if ! command -v k6 &> /dev/null; then
    echo "❌ k6 not installed!"
    echo "   Install: brew install k6"
    echo "   Or visit: https://k6.io/docs/getting-started/installation/"
    exit 1
fi

# Check if backend is running
BACKEND_URL="${BACKEND_URL:-http://localhost:8080}"
echo "Testing against: $BACKEND_URL"

if ! curl -f "$BACKEND_URL/health" &> /dev/null; then
    echo "❌ Backend not responding at $BACKEND_URL"
    echo "   Start backend: cd apps/backend && go run cmd/server/main.go"
    exit 1
fi

echo "✅ Backend is healthy"
echo ""

cd "$TEST_DIR"

echo "1️⃣  Running Normal Load Test (100 users, 5 min)..."
echo "===================================================="
k6 run --out json=normal-load-results.json normal-load.js

echo ""
echo "2️⃣  Running Peak Load Test (500 users, 10 min)..."
echo "=================================================="
k6 run --out json=peak-load-results.json peak-load.js

echo ""
echo "3️⃣  Running Stress Test (1000+ users, 10 min)..."
echo "================================================"
k6 run --out json=stress-test-results.json stress-test.js

echo ""
echo "4️⃣  Running Spike Test (sudden traffic surge)..."
echo "================================================"
k6 run --out json=spike-test-results.json spike-test.js

echo ""
echo "📊 Generating performance report..."
cat > "$REPORT_DIR/performance-report.md" << 'EOF'
# AIM Performance Benchmark Report

**Date**: $(date +"%B %d, %Y")
**Environment**: Local Development
**Backend**: Go 1.23 / Fiber v3
**Database**: PostgreSQL 16

## Test Results

| Scenario | Users | Duration | p50 | p95 | p99 | Throughput | Error Rate |
|----------|-------|----------|-----|-----|-----|------------|------------|
| Normal Load | 100 | 5m | TBD | TBD | TBD | TBD | TBD |
| Peak Load | 500 | 10m | TBD | TBD | TBD | TBD | TBD |
| Stress Test | 1000+ | 10m | TBD | TBD | TBD | TBD | TBD |
| Spike Test | 100→1000 | 5m | TBD | TBD | TBD | TBD | TBD |

## Analysis

### Performance Targets

- ✅/❌ p95 latency < 100ms: TBD
- ✅/❌ Throughput > 1000 req/s: TBD
- ✅/❌ Handles 1000+ concurrent users: TBD
- ✅/❌ Error rate < 1%: TBD

### Bottlenecks Identified

[To be filled after analyzing results]

### Recommendations

[To be filled after analysis]

EOF

echo ""
echo "✅ Performance testing complete!"
echo ""
echo "📋 Results:"
echo "  - Normal load:  $TEST_DIR/normal-load-results.json"
echo "  - Peak load:    $TEST_DIR/peak-load-results.json"
echo "  - Stress test:  $TEST_DIR/stress-test-results.json"
echo "  - Spike test:   $TEST_DIR/spike-test-results.json"
echo "  - Report:       $REPORT_DIR/performance-report.md"
echo ""
echo "📈 Next steps:"
echo "  1. Analyze JSON results with k6 Cloud or custom scripts"
echo "  2. Update performance report with actual metrics"
echo "  3. Identify and fix any bottlenecks"
echo "  4. Re-run tests to verify improvements"
