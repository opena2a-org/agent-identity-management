#!/usr/bin/env python3
"""
Test that auto-copy actually works when credentials don't exist in home directory.
"""

import os
import sys
from pathlib import Path

# Clear home directory credentials to force auto-copy
home_creds = Path.home() / ".aim" / "credentials.json"
if home_creds.exists():
    home_creds.unlink()
    print(f"✅ Cleared {home_creds}")
else:
    print(f"📍 Home credentials don't exist yet: {home_creds}")

# Now import and create manager - should trigger auto-copy
print("\n🔍 Importing OAuthTokenManager...")
from aim_sdk.oauth import OAuthTokenManager

print("🔍 Creating OAuthTokenManager (should auto-copy)...")
manager = OAuthTokenManager()

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
    if manager.has_credentials():
        print(f"   - aim_url: {manager.credentials.get('aim_url', 'N/A')}")
        print(f"   - user_id: {manager.credentials.get('user_id', 'N/A')}")
        print(f"   - email: {manager.credentials.get('email', 'N/A')}")
else:
    print(f"\n❌ FAILED! Credentials NOT auto-copied to {home_creds}")
    print(f"   - Manager path: {manager.credentials_path}")
    sys.exit(1)

print("\n✅ Auto-copy test PASSED!")
