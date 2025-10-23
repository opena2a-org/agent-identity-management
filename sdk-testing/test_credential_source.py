#!/usr/bin/env python3
"""
Debug where credentials are actually coming from.
"""

import os
from pathlib import Path

# Check all possible credential locations
home_creds = Path.home() / ".aim" / "credentials.json"
sdk_creds = Path("/Users/decimai/workspace/aim-sdk-python/.aim/credentials.json")
cwd_creds = Path.cwd() / ".aim" / "credentials.json"

print("📍 Checking credential locations:")
print(f"   - Home: {home_creds.exists()} ({home_creds})")
print(f"   - SDK Package: {sdk_creds.exists()} ({sdk_creds})")
print(f"   - CWD: {cwd_creds.exists()} ({cwd_creds})")

# Check if secure storage is available
try:
    from aim_sdk.secure_storage import SecureCredentialStorage
    print("\n✅ Secure storage module is available")

    # Try to load from secure storage
    storage = SecureCredentialStorage(str(home_creds))
    if storage.credentials_exist():
        print("✅ Credentials exist in secure storage!")
        creds = storage.load_credentials()
        if creds:
            print(f"   - aim_url: {creds.get('aim_url', 'N/A')}")
            print(f"   - user_id: {creds.get('user_id', 'N/A')}")
            print(f"   - email: {creds.get('email', 'N/A')}")
    else:
        print("❌ No credentials in secure storage")
except ImportError:
    print("\n❌ Secure storage module not available")
except Exception as e:
    print(f"\n⚠️  Error checking secure storage: {e}")

# Now test OAuthTokenManager
print("\n🔍 Testing OAuthTokenManager...")
from aim_sdk.oauth import OAuthTokenManager

manager = OAuthTokenManager(use_secure_storage=False)  # Disable secure storage
print(f"   - Manager path: {manager.credentials_path}")
print(f"   - Has credentials: {manager.has_credentials()}")
print(f"   - Use secure storage: {manager.use_secure_storage}")
print(f"   - Secure storage obj: {manager.secure_storage}")

if manager.has_credentials():
    print(f"   - Loaded from: {manager.credentials_path}")
    print(f"   - aim_url: {manager.credentials.get('aim_url', 'N/A')}")
