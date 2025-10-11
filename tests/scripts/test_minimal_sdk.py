#!/usr/bin/env python3
"""Minimal test to isolate SDK initialization issue"""

import sys
import os

# Add SDK to path
sdk_path = os.path.join(os.path.dirname(__file__), 'sdks', 'python')
sys.path.insert(0, sdk_path)

print("Step 1: Importing AIMClient...")
try:
    from aim_sdk import AIMClient
    print("✅ Import successful")
except Exception as e:
    print(f"❌ Import failed: {e}")
    import traceback
    traceback.print_exc()
    sys.exit(1)

print("\nStep 2: Creating client...")
try:
    client = AIMClient(
        agent_id="test-agent-id",
        api_key="test-api-key",
        aim_url="http://localhost:8080",
        sdk_token_id=None
    )
    print("✅ Client created")
except Exception as e:
    print(f"❌ Client creation failed: {e}")
    import traceback
    traceback.print_exc()
    sys.exit(1)

print("\n✅ Test complete!")
