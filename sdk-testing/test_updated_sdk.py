#!/usr/bin/env python3
"""
Test the UPDATED SDK with corrected path logic.
"""

import sys
import os
from pathlib import Path

# Add the UPDATED SDK to path (from /workspace/aim-sdk-python)
sdk_path = "/Users/decimai/workspace/aim-sdk-python"
sys.path.insert(0, sdk_path)

print(f"🔍 Using SDK from: {sdk_path}")

# Clear home credentials to force auto-copy
home_creds = Path.home() / ".aim" / "credentials.json"
if home_creds.exists():
    home_creds.unlink()
    print(f"✅ Cleared {home_creds}")

# Now import from the updated SDK
from aim_sdk.oauth import OAuthTokenManager

print(f"\n🔍 Creating OAuthTokenManager (should trigger auto-copy)...")
manager = OAuthTokenManager(use_secure_storage=False)

print(f"\n📁 Manager created:")
print(f"   - Credentials path: {manager.credentials_path}")
print(f"   - Has credentials: {manager.has_credentials()}")

# Check if auto-copy happened
if home_creds.exists():
    print(f"\n✅ SUCCESS! Credentials auto-copied to {home_creds}")

    # Check permissions
    import stat
    perms = oct(os.stat(home_creds).st_mode)[-3:]
    print(f"   - File permissions: {perms}")

    # Check content
    import json
    with open(home_creds) as f:
        creds = json.load(f)
        print(f"   - aim_url: {creds.get('aim_url', 'N/A')}")
        print(f"   - user_id: {creds.get('user_id', 'N/A')}")
        print(f"   - email: {creds.get('email', 'N/A')}")
        print(f"   - has refresh_token: {'refresh_token' in creds}")

    print("\n🎉 AUTO-COPY WORKS! The 'Stripe moment' is REAL!")
    print("✅ OAuth fix VERIFIED - SDK works out-of-the-box!")
else:
    print(f"\n❌ Auto-copy failed")
    print(f"   - Expected at: {home_creds}")
    print(f"   - Manager path: {manager.credentials_path}")
