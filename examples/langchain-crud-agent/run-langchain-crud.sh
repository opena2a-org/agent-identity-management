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

# How the demo reaches the local AIM stack (stack_env.py does the same for the
# agent). Each port comes from the environment first, then from the
# repository's .env (the file docker compose reads for docker-compose.yml),
# then from the default the compose files use:
#   AIM_BACKEND_PORT=8080   the backend's host port
#   AIM_FRONTEND_PORT=3000  the dashboard's host port
# An AIM_API_URL set in the environment or in this directory's .env wins over
# the backend port.
STACK_DOTENV="$(dirname "${BASH_SOURCE[0]}")/../../.env"

# stack_dotenv_value NAME prints the last value the repository's .env assigns
# to NAME, or nothing.
stack_dotenv_value() {
    [ -f "$STACK_DOTENV" ] || return 0
    sed -n "s/^$1=//p" "$STACK_DOTENV" | tail -n 1
}

AIM_BACKEND_PORT="${AIM_BACKEND_PORT:-$(stack_dotenv_value AIM_BACKEND_PORT)}"
AIM_BACKEND_PORT="${AIM_BACKEND_PORT:-8080}"
AIM_FRONTEND_PORT="${AIM_FRONTEND_PORT:-$(stack_dotenv_value AIM_FRONTEND_PORT)}"
AIM_FRONTEND_PORT="${AIM_FRONTEND_PORT:-3000}"
export AIM_API_URL="${AIM_API_URL:-http://localhost:$AIM_BACKEND_PORT}"
AIM_DASHBOARD_URL="http://localhost:$AIM_FRONTEND_PORT"

# Check if AIM_API_KEY is set
if [ -z "$AIM_API_KEY" ]; then
    echo ""
    echo "❌ Error: AIM_API_KEY is not set!"
    echo ""
    echo "Please set your AIM API key:"
    echo "  1. Get your API key from: $AIM_DASHBOARD_URL/dashboard/settings/api-keys"
    echo "  2. Create a .env file:"
    echo ""
    echo "     echo 'AIM_API_KEY=your-api-key-here' > .env"
    echo "     echo 'AIM_API_URL=$AIM_API_URL' >> .env"
    echo ""
    echo "  Or export it directly:"
    echo ""
    echo "     export AIM_API_KEY='your-api-key-here'"
    echo ""
    exit 1
fi

echo ""

# Check if AIM backend is running
echo "Checking AIM backend at $AIM_API_URL..."
if curl -sf "$AIM_API_URL/health" > /dev/null 2>&1; then
    echo "✅ AIM backend is running"
else
    echo "❌ AIM backend is not running!"
    echo "   No answer from: curl -sf $AIM_API_URL/health"
    echo "   Please start it first:"
    echo "   cd apps/backend && go run ./cmd/server"
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
echo "   $AIM_DASHBOARD_URL/dashboard/agents"
echo ""
