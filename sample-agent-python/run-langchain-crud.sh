#!/bin/bash
# Run LangChain CRUD Agent with AIM SDK
# This script sets up and runs the LangChain CRUD agent demo

set -e

echo "🚀 LangChain CRUD Agent with AIM SDK"
echo "======================================"
echo ""

# Load .env file if it exists
if [ -f .env ]; then
    echo "Loading environment variables from .env file..."
    export $(cat .env | grep -v '^#' | xargs)
    echo "✅ Environment variables loaded"
else
    echo "⚠️  No .env file found"
fi

# Check if AIM_API_KEY is set
if [ -z "$AIM_API_KEY" ]; then
    echo ""
    echo "❌ Error: AIM_API_KEY is not set!"
    echo ""
    echo "Please set your AIM API key:"
    echo "  1. Get your API key from: http://localhost:3000/dashboard/settings/api-keys"
    echo "  2. Create a .env file:"
    echo ""
    echo "     echo 'AIM_API_KEY=your-api-key-here' > .env"
    echo "     echo 'AIM_API_URL=http://localhost:8080' >> .env"
    echo ""
    echo "  Or export it directly:"
    echo ""
    echo "     export AIM_API_KEY='your-api-key-here'"
    echo ""
    exit 1
fi

echo ""

# Check if AIM backend is running
echo "Checking AIM backend..."
if curl -s http://localhost:8080/health > /dev/null 2>&1; then
    echo "✅ AIM backend is running"
else
    echo "❌ AIM backend is not running!"
    echo "   Please start it first:"
    echo "   cd apps/backend && ./server"
    exit 1
fi

# Check if virtual environment exists
if [ ! -d "venv" ]; then
    echo ""
    echo "Creating virtual environment..."
    python3 -m venv venv
fi

# Activate virtual environment
echo "Activating virtual environment..."
source venv/bin/activate

# Install dependencies
echo ""
echo "Installing dependencies..."
pip install -q langchain langchain-google-genai langchain-core requests

# Set default values for optional variables
export AIM_API_URL="${AIM_API_URL:-http://localhost:8080}"

echo ""
echo "Configuration:"
echo "  AIM_API_KEY: ${AIM_API_KEY:0:20}..."
echo "  AIM_API_URL: $AIM_API_URL"
echo ""

# Run the agent
echo "Running LangChain CRUD Agent..."
echo ""
python3 langchain_crud_agent.py

echo ""
echo "✅ Demo complete!"
echo ""
echo "📊 View results in AIM Dashboard:"
echo "   http://localhost:3000/dashboard/agents"
echo ""

