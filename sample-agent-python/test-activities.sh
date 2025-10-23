#!/bin/bash

# AIM Python SDK - Activity Testing Script
# This script demonstrates activity patterns for monitoring

set -e

echo "🔒 AIM Activity Demo - Quick Start"
echo "===================================="
echo ""

# Check if SDK is installed
if [ ! -d "aim-sdk-python" ]; then
    echo "❌ Error: aim-sdk-python directory not found"
    echo "   Please ensure you're in the sample-agent-python directory"
    exit 1
fi

# Check if backend is running
if ! curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "⚠️  Warning: Backend not responding on localhost:8080"
    echo "   Please start the backend first:"
    echo "   cd apps/backend && go run cmd/server/main.go"
    exit 1
fi

echo "▶️  Running activity demo..."
echo ""
python3 test-activities.py

echo ""
echo "✅ Activity demo complete!"
echo ""
echo "📊 View results in dashboard:"
echo "   http://localhost:3000/dashboard/agents"
echo ""
echo "💡 Note: This demo shows activity patterns."
echo "   Actual activity logging happens through:"
echo "   • Agent registration"
echo "   • Capability requests"
echo "   • SDK operations"
echo ""

