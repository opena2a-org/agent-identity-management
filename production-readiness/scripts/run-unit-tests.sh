#!/bin/bash
# Production Readiness - Layer 2: Unit Testing Script
# Run all unit tests with coverage reporting

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
BACKEND_DIR="$PROJECT_ROOT/apps/backend"
REPORT_DIR="$SCRIPT_DIR/../reports"

echo "🧪 AIM Unit Tests - Layer 2"
echo "============================"
echo ""

cd "$BACKEND_DIR"

echo "📦 Installing dependencies..."
go mod download

echo ""
echo "🔬 Running unit tests with coverage..."
go test ./... -v -cover -coverprofile="$REPORT_DIR/coverage.out" -timeout 30s

echo ""
echo "📊 Generating coverage report..."
go tool cover -func="$REPORT_DIR/coverage.out" > "$REPORT_DIR/coverage.txt"
go tool cover -html="$REPORT_DIR/coverage.out" -o "$REPORT_DIR/coverage.html"

echo ""
echo "📈 Coverage summary:"
echo "==================="
tail -1 "$REPORT_DIR/coverage.txt"

echo ""
COVERAGE=$(go tool cover -func="$REPORT_DIR/coverage.out" | grep total | awk '{print $3}')
COVERAGE_NUM=$(echo "$COVERAGE" | sed 's/%//')

if (( $(echo "$COVERAGE_NUM >= 90" | bc -l) )); then
    echo "✅ PASS: Coverage $COVERAGE >= 90% target"
else
    echo "❌ FAIL: Coverage $COVERAGE < 90% target"
    echo ""
    echo "Files with low coverage:"
    go tool cover -func="$REPORT_DIR/coverage.out" | awk '$3 != "100.0%" {print $1 " - " $3}' | head -10
    exit 1
fi

echo ""
echo "📄 Reports generated:"
echo "  - Text:  $REPORT_DIR/coverage.txt"
echo "  - HTML:  $REPORT_DIR/coverage.html"
echo "  - Raw:   $REPORT_DIR/coverage.out"
echo ""
echo "✅ Unit testing complete!"
