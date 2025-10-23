#!/bin/bash

echo "🚀 Setting up AIM Python Sample Agent"
echo "====================================="
echo ""

# Check if Python is installed
if ! command -v python3 &> /dev/null; then
    echo "❌ Python 3 is not installed. Please install Python 3.8 or higher."
    exit 1
fi

echo "✅ Python 3 found: $(python3 --version)"
echo ""

# Install SDK dependencies
echo "📦 Installing SDK dependencies..."
cd aim-sdk-python || exit 1
pip3 install -e . || {
    echo "❌ Failed to install SDK"
    exit 1
}
cd ..

# Install agent dependencies
echo ""
echo "📦 Installing agent dependencies..."
pip3 install -r requirements.txt || {
    echo "❌ Failed to install dependencies"
    exit 1
}

echo ""
echo "✅ Setup complete!"
echo ""
echo "🎯 Next steps:"
echo "   1. Make sure AIM backend is running (http://localhost:8080)"
echo "   2. Run the sample agent:"
echo "      python3 agent.py"
echo ""
echo "   3. Or run tests:"
echo "      python3 test_safe_execution.py"
echo "      python3 test_dangerous_execution.py"
echo ""

