#!/usr/bin/env python3
"""
Debug the auto-copy with explicit path checking.
"""

import os
import shutil
from pathlib import Path

# Manually test the discovery logic
print("🔍 Testing credential discovery logic manually...")

home_creds = Path.home() / ".aim" / "credentials.json"
print(f"\n1. Home directory check:")
print(f"   - Path: {home_creds}")
print(f"   - Exists: {home_creds.exists()}")

if not home_creds.exists():
    print(f"\n2. SDK package directory check:")

    import aim_sdk
    sdk_package_dir = Path(aim_sdk.__file__).parent
    sdk_creds = sdk_package_dir / ".aim" / "credentials.json"

    print(f"   - SDK package dir: {sdk_package_dir}")
    print(f"   - SDK creds path: {sdk_creds}")
    print(f"   - SDK creds exists: {sdk_creds.exists()}")

    if sdk_creds.exists():
        print(f"\n3. Attempting auto-copy...")
        try:
            # Create parent directory
            home_creds.parent.mkdir(parents=True, exist_ok=True)
            print(f"   ✅ Created directory: {home_creds.parent}")

            # Copy file
            shutil.copy(sdk_creds, home_creds)
            print(f"   ✅ Copied file to: {home_creds}")

            # Set permissions
            os.chmod(home_creds, 0o600)
            print(f"   ✅ Set permissions to 600")

            # Verify
            if home_creds.exists():
                print(f"\n✅ SUCCESS! File exists at {home_creds}")

                # Check content
                import json
                with open(home_creds) as f:
                    creds = json.load(f)
                    print(f"   - aim_url: {creds.get('aim_url', 'N/A')}")
                    print(f"   - email: {creds.get('email', 'N/A')}")
            else:
                print(f"\n❌ File still doesn't exist after copy!")

        except Exception as e:
            print(f"\n❌ Error during auto-copy: {e}")
            import traceback
            traceback.print_exc()
