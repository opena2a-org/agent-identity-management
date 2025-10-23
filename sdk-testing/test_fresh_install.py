#!/usr/bin/env python3
"""
Simulate a fresh SDK installation by clearing all credentials and testing auto-copy.
"""

import os
import shutil
from pathlib import Path

print("🧹 Simulating fresh SDK installation...")

# Clear all credential locations
home_creds = Path.home() / ".aim" / "credentials.json"
home_dir = Path.home() / ".aim"

# Delete home credentials directory
if home_dir.exists():
    shutil.rmtree(home_dir)
    print(f"✅ Cleared {home_dir}")
else:
    print(f"📍 Home .aim directory doesn't exist")

# Clear secure storage
try:
    from aim_sdk.secure_storage import SecureCredentialStorage
    storage = SecureCredentialStorage(str(home_creds))
    if storage.credentials_exist():
        storage.delete_credentials()
        print("✅ Cleared secure storage credentials")
    else:
        print("📍 No credentials in secure storage")
except Exception as e:
    print(f"⚠️  Could not clear secure storage: {e}")

print("\n🔍 Now importing OAuthTokenManager with secure storage DISABLED...")
print("   (This forces plaintext file usage, so we can see auto-copy)")

from aim_sdk.oauth import OAuthTokenManager

# Create manager WITHOUT secure storage
manager = OAuthTokenManager(use_secure_storage=False)

print(f"\n📁 Manager created:")
print(f"   - Credentials path: {manager.credentials_path}")
print(f"   - Has credentials: {manager.has_credentials()}")

# Check if file was created
if home_creds.exists():
    print(f"\n✅ SUCCESS! Credentials auto-copied to {home_creds}")

    # Check permissions
    import stat
    perms = oct(os.stat(home_creds).st_mode)[-3:]
    print(f"   - File permissions: {perms}")

    # Read content
    import json
    with open(home_creds) as f:
        creds = json.load(f)
        print(f"   - aim_url: {creds.get('aim_url', 'N/A')}")
        print(f"   - user_id: {creds.get('user_id', 'N/A')}")
        print(f"   - email: {creds.get('email', 'N/A')}")
        print(f"   - has refresh_token: {'refresh_token' in creds}")

    print("\n✅ AUTO-COPY WORKS! The 'Stripe moment' is REAL!")
else:
    print(f"\n❌ Auto-copy did not work")
    print(f"   Expected file at: {home_creds}")
