#!/usr/bin/env python3
"""
Get fresh SDK credentials with proper OAuth flow
This simulates what happens when a user downloads SDK from portal
"""

import requests
import json
import zipfile
import io
import os
from pathlib import Path

API_URL = "http://localhost:8080"

print("="*80)
print("GETTING FRESH SDK WITH PROPER OAUTH")
print("="*80 + "\n")

# Step 1: For now, we'll manually trigger the Microsoft OAuth flow
# In production, this would be done via browser
print("Step 1: OAuth Login Required")
print("-" * 80)
print("To get fresh credentials, we need to log in via OAuth.")
print("Please navigate to: http://localhost:3000/auth/login")
print("And log in with Microsoft.")
print()
print("After logging in, the portal will have fresh tokens.")
print("Then we can download the SDK from the portal UI.")
print()

# Alternative: Check if we can use the existing backend session
print("Step 2: Checking for existing session...")
print("-" * 80)

# Try to get user info to see if session is valid
try:
    # Load existing tokens
    creds_path = Path.home() / ".aim" / "credentials.json"
    if creds_path.exists():
        with open(creds_path, 'r') as f:
            creds = json.load(f)

        # Try to use refresh token to get new access token
        print("Found existing credentials, attempting token refresh...")

        # Call refresh endpoint
        response = requests.post(
            f"{API_URL}/api/v1/auth/refresh",
            json={"refresh_token": creds.get("refresh_token")},
            timeout=10
        )

        if response.status_code == 200:
            data = response.json()
            access_token = data['access_token']
            new_refresh_token = data['refresh_token']

            print(f"✅ Got fresh access token!")
            print(f"✅ Got new refresh token (token rotation)")

            # Update credentials with new tokens
            creds['refresh_token'] = new_refresh_token
            with open(creds_path, 'w') as f:
                json.dump(creds, f, indent=2)
            os.chmod(creds_path, 0o600)

            print(f"✅ Saved new refresh token")
            print()

            # Now download fresh SDK
            print("Step 3: Downloading Fresh SDK...")
            print("-" * 80)

            response = requests.post(
                f"{API_URL}/api/v1/sdk/download/python",
                headers={"Authorization": f"Bearer {access_token}"},
                json={"device_name": "Flight Agent - Fresh Download"},
                timeout=30
            )

            if response.status_code == 200:
                data = response.json()
                download_url = data.get('download_url')

                if download_url:
                    # Download the SDK zip
                    print(f"✅ SDK download URL obtained")
                    print(f"   Downloading SDK...")

                    sdk_response = requests.get(download_url, timeout=60)
                    if sdk_response.status_code == 200:
                        # Extract to current directory
                        with zipfile.ZipFile(io.BytesIO(sdk_response.content)) as zip_file:
                            zip_file.extractall("./fresh-sdk")

                        print(f"✅ SDK downloaded and extracted to ./fresh-sdk/")
                        print()
                        print("="*80)
                        print("SUCCESS!")
                        print("="*80)
                        print()
                        print("Fresh SDK with new OAuth credentials ready at:")
                        print("  ./fresh-sdk/aim-sdk-python/")
                        print()
                        print("The new credentials include:")
                        print("  - Fresh refresh_token")
                        print("  - New SDK token ID")
                        print("  - Valid for verification flow")
                        print()
                        print("Next: Update flight agent to use fresh credentials")
                    else:
                        print(f"❌ Failed to download SDK: {sdk_response.status_code}")
                else:
                    print(f"❌ No download URL in response")
                    print(f"   Response: {data}")
            else:
                print(f"❌ Failed to request SDK download: {response.status_code}")
                print(f"   Error: {response.text}")
        else:
            print(f"❌ Token refresh failed: {response.status_code}")
            print(f"   This means the refresh token has been revoked (security working!)")
            print()
            print("="*80)
            print("MANUAL OAUTH LOGIN REQUIRED")
            print("="*80)
            print()
            print("1. Open browser: http://localhost:3000/auth/login")
            print("2. Log in with Microsoft")
            print("3. Navigate to: http://localhost:3000/dashboard/sdk")
            print("4. Click 'Download SDK' for Python")
            print("5. Extract downloaded ZIP to ./fresh-sdk/")
            print()
            print("This will give you fresh OAuth credentials for testing.")
    else:
        print("❌ No existing credentials found")
        print("   Please log in via portal first")

except Exception as e:
    print(f"❌ Error: {e}")
    import traceback
    traceback.print_exc()
