#!/bin/bash
# Quick QA Test - Opens browser for OAuth and guides through verification

set -e

echo "================================================================================"
echo "AIM PLATFORM - QUICK QA TEST"
echo "================================================================================"
echo ""
echo "This script will help you complete QA testing by:"
echo "  1. Opening browser for fresh OAuth login"
echo "  2. Guiding you to download fresh SDK"
echo "  3. Running verification tests"
echo ""
echo "Press ENTER to continue..."
read

# Step 1: Open portal login
echo ""
echo "================================================================================"
echo "STEP 1: OAuth Login"
echo "================================================================================"
echo ""
echo "Opening AIM portal login page..."
echo "Please log in with your Microsoft account."
echo ""
sleep 2
open "http://localhost:3000/auth/login" 2>/dev/null || \
  xdg-open "http://localhost:3000/auth/login" 2>/dev/null || \
  echo "Please manually open: http://localhost:3000/auth/login"

echo ""
echo "After logging in, press ENTER to continue..."
read

# Step 2: Download SDK
echo ""
echo "================================================================================"
echo "STEP 2: Download Fresh SDK"
echo "================================================================================"
echo ""
echo "Opening SDK download page..."
echo "Please click 'Download SDK' for Python."
echo ""
sleep 2
open "http://localhost:3000/dashboard/sdk" 2>/dev/null || \
  xdg-open "http://localhost:3000/dashboard/sdk" 2>/dev/null || \
  echo "Please manually open: http://localhost:3000/dashboard/sdk"

echo ""
echo "After downloading SDK, press ENTER to continue..."
read

# Step 3: Extract SDK
echo ""
echo "================================================================================"
echo "STEP 3: Extract Fresh SDK"
echo "================================================================================"
echo ""
echo "Please enter the path to the downloaded SDK ZIP file:"
echo "(It should be in your Downloads folder, named something like aim-sdk-python.zip)"
echo ""
read -p "SDK ZIP path: " SDK_ZIP

if [ -f "$SDK_ZIP" ]; then
    echo ""
    echo "Extracting SDK to ./fresh-sdk/..."
    rm -rf ./fresh-sdk
    unzip -q "$SDK_ZIP" -d ./fresh-sdk
    echo "✅ SDK extracted"
else
    echo ""
    echo "⚠️  File not found: $SDK_ZIP"
    echo "Please extract manually and copy credentials to ~/.aim/"
    echo ""
    echo "Press ENTER when ready to continue..."
    read
fi

# Step 4: Copy credentials
echo ""
echo "================================================================================"
echo "STEP 4: Copy Fresh Credentials"
echo "================================================================================"
echo ""

if [ -d "./fresh-sdk/aim-sdk-python/.aim" ]; then
    echo "Backing up existing credentials (if any)..."
    if [ -d "$HOME/.aim" ]; then
        cp -r "$HOME/.aim" "$HOME/.aim.backup.$(date +%Y%m%d_%H%M%S)"
        echo "✅ Existing credentials backed up"
    fi

    echo "Copying fresh credentials..."
    cp -r ./fresh-sdk/aim-sdk-python/.aim "$HOME/.aim"
    chmod 600 "$HOME/.aim/credentials.json"
    echo "✅ Fresh credentials installed"
else
    echo "⚠️  Could not find credentials in extracted SDK"
    echo "Please manually copy .aim directory to $HOME/.aim/"
    echo ""
    echo "Press ENTER when ready to continue..."
    read
fi

# Step 5: Run verification
echo ""
echo "================================================================================"
echo "STEP 5: Run Verification Tests"
echo "================================================================================"
echo ""
echo "Running automated QA verification..."
echo ""
sleep 2

python3 verify_qa_complete.py
RESULT=$?

# Step 6: Open dashboard
echo ""
echo "================================================================================"
echo "STEP 6: Verify Dashboard"
echo "================================================================================"
echo ""
echo "Opening agent detail page in browser..."
echo "Please verify these tabs now have data:"
echo "  ✓ Recent Activity"
echo "  ✓ Trust History"
echo "  ✓ Capabilities"
echo "  ✓ Graph View"
echo ""
sleep 2

# Get agent ID from credentials
AGENT_ID=$(python3 -c "
import json
from pathlib import Path
creds_path = Path.home() / '.aim' / 'credentials.json'
if creds_path.exists():
    with open(creds_path) as f:
        creds = json.load(f)
    for key, value in creds.items():
        if isinstance(value, dict) and 'agent_id' in value:
            print(value['agent_id'])
            break
" 2>/dev/null)

if [ -n "$AGENT_ID" ]; then
    open "http://localhost:3000/dashboard/agents/$AGENT_ID" 2>/dev/null || \
      xdg-open "http://localhost:3000/dashboard/agents/$AGENT_ID" 2>/dev/null || \
      echo "Please manually open: http://localhost:3000/dashboard/agents/$AGENT_ID"
else
    open "http://localhost:3000/dashboard" 2>/dev/null || \
      xdg-open "http://localhost:3000/dashboard" 2>/dev/null || \
      echo "Please manually open: http://localhost:3000/dashboard"
fi

# Final summary
echo ""
echo "================================================================================"
echo "QA TEST COMPLETE"
echo "================================================================================"
echo ""

if [ $RESULT -eq 0 ]; then
    echo "🎉 ALL VERIFICATION TESTS PASSED!"
    echo ""
    echo "✅ Platform is production-ready"
    echo "✅ All features working correctly"
    echo "✅ Dashboard tabs should now have data"
    echo ""
    echo "Next steps:"
    echo "  1. Verify all tabs in browser"
    echo "  2. Review production readiness report (PRODUCTION_READINESS_REPORT.md)"
    echo "  3. Sign off on production deployment"
else
    echo "⚠️  Some verification tests failed"
    echo ""
    echo "Please review the output above for details."
    echo "See NEXT_STEPS.md for troubleshooting."
fi

echo ""
echo "================================================================================"
