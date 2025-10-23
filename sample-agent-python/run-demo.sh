#!/bin/bash

# AIM Python SDK Demo - Quick Start Script
# This script ensures clean execution of the demo

set -e

echo "🚀 AIM Python SDK Demo - Quick Start"
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
    echo "   Starting backend..."
    cd ../apps/backend && go run cmd/server/main.go > ../../backend.log 2>&1 &
    sleep 3
    cd ../../sample-agent-python
fi

# Option 1: Clean run (new agent every time)
if [ "$1" == "--clean" ]; then
    echo "🗑️  Cleaning old credentials..."
    rm -rf ~/.aim
    echo "✅ Running with fresh credentials"
    echo ""
fi

# Option 2: Force new agent with timestamp
if [ "$1" == "--new" ]; then
    export AGENT_NAME="demo-agent-$(date +%s)"
    echo "🆕 Creating new agent: $AGENT_NAME"
    echo ""
fi

# Run the demo
echo "▶️  Running demo..."
echo ""
python3 demo.py

echo ""
echo "✅ Demo complete!"
echo ""
echo "📊 View in dashboard:"
echo "   http://localhost:3000/dashboard/agents"

