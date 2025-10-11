#!/usr/bin/env python3
"""Debug version of Python SDK test to find hanging issue"""

import os
import sys

# Add SDK to path
sdk_path = os.path.join(os.path.dirname(__file__), 'sdks', 'python')
sys.path.insert(0, sdk_path)

from aim_sdk import AIMClient

print("Creating client...")
client = AIMClient(
    agent_id="e237d89d-d366-43e5-808e-32c2ab64de6b",
    api_key="aim_live_dw4shT8Ng6fyM7OTO9XLVA71NP09KVeBqmJhlQe_cJw=",
    aim_url="http://localhost:8080",
    sdk_token_id=None,
    timeout=5  # Short timeout for debugging
)
print("✅ Client created\n")

print("Testing single capability grant...")
try:
    print("  Making HTTP request...")
    result = client._make_request(
        method="POST",
        endpoint=f"/api/v1/sdk-api/agents/{client.agent_id}/capabilities",
        data={
            "capabilityType": "network_access",
            "scope": {"test": True}
        }
    )
    print(f"  ✅ Response: {result}")
except Exception as e:
    print(f"  ❌ Error: {e}")
    import traceback
    traceback.print_exc()

print("\n✅ Debug test complete!")
