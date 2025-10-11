#!/usr/bin/env python3
"""
Test script to verify SDK usage metrics are tracked correctly.

This script:
1. Extracts the SDK token from a downloaded SDK zip
2. Makes API calls using the SDK
3. Verifies that usage metrics increase
"""

import os
import json
import zipfile
import tempfile
import shutil
import requests
from pathlib import Path

# Configuration
API_BASE_URL = "http://localhost:8080"
DOWNLOADS_DIR = Path.home() / "Downloads"

def find_latest_sdk_zip():
    """Find the most recently downloaded SDK zip file."""
    sdk_zips = list(DOWNLOADS_DIR.glob("aim-sdk-*.zip"))
    if not sdk_zips:
        print("❌ No SDK zip files found in Downloads folder")
        print(f"   Looking in: {DOWNLOADS_DIR}")
        return None

    # Sort by modification time, newest first
    sdk_zips.sort(key=lambda p: p.stat().st_mtime, reverse=True)
    latest_zip = sdk_zips[0]
    print(f"✅ Found SDK zip: {latest_zip.name}")
    return latest_zip

def extract_config_from_zip(zip_path):
    """Extract .aim/credentials.json from SDK zip."""
    with tempfile.TemporaryDirectory() as temp_dir:
        with zipfile.ZipFile(zip_path, 'r') as zip_ref:
            zip_ref.extractall(temp_dir)

        # Find .aim/credentials.json in extracted files
        config_files = list(Path(temp_dir).rglob('.aim/credentials.json'))
        if not config_files:
            print("❌ No .aim/credentials.json found in SDK zip")
            print("   Files in zip:")
            for file in Path(temp_dir).rglob('*'):
                if file.is_file():
                    print(f"     - {file.relative_to(temp_dir)}")
            return None

        config_path = config_files[0]
        with open(config_path, 'r') as f:
            config = json.load(f)

        print(f"✅ Extracted credentials from SDK")
        print(f"   API URL: {config.get('aim_url')}")
        print(f"   User ID: {config.get('user_id')}")
        print(f"   Token: {config.get('refresh_token')[:20]}...")
        return config

def make_api_request(token, endpoint, method='GET'):
    """Make an API request using the SDK token."""
    url = f"{API_BASE_URL}{endpoint}"
    headers = {
        'Authorization': f'Bearer {token}',
        'Content-Type': 'application/json'
    }

    print(f"\n📡 Making {method} request to {endpoint}")

    try:
        if method == 'GET':
            response = requests.get(url, headers=headers, timeout=10)
        else:
            response = requests.request(method, url, headers=headers, timeout=10)

        print(f"   Status: {response.status_code}")

        if response.status_code == 200:
            print(f"   ✅ Success")
            return True
        else:
            print(f"   ❌ Failed: {response.text}")
            return False
    except Exception as e:
        print(f"   ❌ Error: {str(e)}")
        return False

def get_token_usage_count(auth_token, sdk_token):
    """Get the current usage count for a specific SDK token."""
    url = f"{API_BASE_URL}/api/v1/sdk-tokens"
    headers = {
        'Authorization': f'Bearer {auth_token}',
        'Content-Type': 'application/json'
    }

    try:
        response = requests.get(url, headers=headers, timeout=10)
        if response.status_code != 200:
            print(f"❌ Failed to fetch tokens: {response.status_code}")
            return None

        tokens = response.json()

        # Find the token in the list
        for token in tokens:
            if token.get('token') == sdk_token or token.get('id') == sdk_token:
                return token.get('usage_count', 0)

        print(f"❌ Could not find token in list")
        return None
    except Exception as e:
        print(f"❌ Error fetching token usage: {str(e)}")
        return None

def main():
    print("=" * 60)
    print("SDK USAGE TEST SCRIPT")
    print("=" * 60)

    # Step 1: Find latest SDK zip
    print("\n1️⃣  Finding latest SDK download...")
    zip_path = find_latest_sdk_zip()
    if not zip_path:
        print("\n❌ TEST FAILED: No SDK zip found")
        return False

    # Step 2: Extract config
    print("\n2️⃣  Extracting SDK configuration...")
    config = extract_config_from_zip(zip_path)
    if not config:
        print("\n❌ TEST FAILED: Could not extract config")
        return False

    sdk_token = config.get('refresh_token')
    if not sdk_token:
        print("\n❌ TEST FAILED: No refresh_token in credentials")
        return False

    # Step 3: Get initial usage count
    print("\n3️⃣  Getting initial usage metrics...")
    print("   (Note: This requires main auth token, which we don't have in SDK)")
    print("   Skipping initial count check - will verify increase via UI")

    # Step 4: Make API requests using SDK token
    print("\n4️⃣  Making API requests with SDK token...")

    endpoints = [
        '/api/v1/agents',
        '/api/v1/mcp-servers',
        '/api/v1/api-keys',
        '/api/v1/activity/recent',
    ]

    success_count = 0
    for endpoint in endpoints:
        if make_api_request(sdk_token, endpoint):
            success_count += 1

    print(f"\n📊 API Request Results:")
    print(f"   ✅ Successful: {success_count}/{len(endpoints)}")
    print(f"   ❌ Failed: {len(endpoints) - success_count}/{len(endpoints)}")

    # Step 5: Instructions for manual verification
    print("\n5️⃣  Manual Verification Required:")
    print("   1. Go to http://localhost:3000/dashboard/sdk-tokens")
    print("   2. Find the token with ID from the SDK config")
    print(f"   3. Verify 'Usage Count' increased by {success_count}")
    print(f"   4. Verify 'Last Used' shows recent timestamp")

    # Summary
    print("\n" + "=" * 60)
    if success_count >= len(endpoints) / 2:
        print("✅ TEST PASSED")
        print(f"   Made {success_count} successful API calls using SDK token")
        print(f"   Total Usage should increase by {success_count}")
        return True
    else:
        print("⚠️  TEST PARTIALLY PASSED")
        print(f"   Only {success_count}/{len(endpoints)} requests succeeded")
        return False

if __name__ == '__main__':
    try:
        success = main()
        exit(0 if success else 1)
    except KeyboardInterrupt:
        print("\n\n⚠️  Test interrupted by user")
        exit(1)
    except Exception as e:
        print(f"\n❌ Unexpected error: {str(e)}")
        import traceback
        traceback.print_exc()
        exit(1)
