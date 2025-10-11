#!/usr/bin/env python3
"""
Comprehensive SDK Usage Testing Script
Tests all three SDKs (Python, Go, JavaScript) for usage tracking.
"""

import json
import zipfile
import tempfile
import subprocess
from pathlib import Path
import requests
import time

def find_latest_sdk_zip(sdk_type):
    """Find the most recent SDK zip file in Downloads."""
    downloads = Path.home() / "Downloads"
    pattern = f"aim-sdk-{sdk_type}*.zip"

    files = list(downloads.glob(pattern))
    if not files:
        print(f"❌ No {sdk_type} SDK zip found in Downloads")
        return None

    # Get most recent file
    latest = max(files, key=lambda p: p.stat().st_mtime)
    print(f"✅ Found {sdk_type} SDK: {latest.name}")
    return latest

def extract_credentials(zip_path):
    """Extract .aim/credentials.json from SDK zip."""
    with tempfile.TemporaryDirectory() as temp_dir:
        with zipfile.ZipFile(zip_path, 'r') as zip_ref:
            zip_ref.extractall(temp_dir)

        # Find .aim/credentials.json
        config_files = list(Path(temp_dir).rglob('.aim/credentials.json'))
        if not config_files:
            print("❌ No .aim/credentials.json found in SDK zip")
            return None

        with open(config_files[0], 'r') as f:
            config = json.load(f)

        print(f"   Token ID: {config.get('sdk_token_id')}")
        return config

def test_sdk_api_calls(sdk_type, token, base_url):
    """Make test API calls using SDK token."""
    print(f"\n📡 Testing {sdk_type} SDK API calls...")

    endpoints = [
        "/api/v1/agents",
        "/api/v1/mcp-servers",
        "/api/v1/api-keys",
    ]

    headers = {
        "Authorization": f"Bearer {token}",
        "Content-Type": "application/json"
    }

    success_count = 0
    fail_count = 0

    for endpoint in endpoints:
        url = f"{base_url}{endpoint}"
        try:
            response = requests.get(url, headers=headers)
            if response.status_code == 200:
                print(f"   ✅ {endpoint} → {response.status_code}")
                success_count += 1
            else:
                print(f"   ❌ {endpoint} → {response.status_code}")
                fail_count += 1
        except Exception as e:
            print(f"   ❌ {endpoint} → Error: {e}")
            fail_count += 1

    print(f"\n   📊 Results: {success_count} successful, {fail_count} failed")
    return success_count, fail_count

def main():
    print("=" * 80)
    print("🧪 COMPREHENSIVE SDK USAGE TESTING")
    print("=" * 80)
    print()
    print("Testing all three SDKs (Python, Go, JavaScript) for usage tracking.")
    print()

    # Base URL
    base_url = "http://localhost:8080"

    # Test results
    results = {}

    # Test each SDK type
    for sdk_type in ["python", "go", "javascript"]:
        print(f"\n{'=' * 80}")
        print(f"🔍 TESTING {sdk_type.upper()} SDK")
        print(f"{'=' * 80}")

        # Find SDK zip
        zip_path = find_latest_sdk_zip(sdk_type)
        if not zip_path:
            results[sdk_type] = {"status": "SKIP", "reason": "SDK zip not found"}
            continue

        # Extract credentials
        config = extract_credentials(zip_path)
        if not config:
            results[sdk_type] = {"status": "SKIP", "reason": "Credentials not found"}
            continue

        token = config.get('refresh_token')
        token_id = config.get('sdk_token_id')

        if not token:
            print("❌ No refresh_token in credentials")
            results[sdk_type] = {"status": "SKIP", "reason": "No token"}
            continue

        # Make API calls
        success, fail = test_sdk_api_calls(sdk_type, token, base_url)

        results[sdk_type] = {
            "status": "TESTED",
            "token_id": token_id,
            "success": success,
            "fail": fail,
            "total": success + fail
        }

        # Wait between SDK tests
        if sdk_type != "javascript":
            print("\n⏳ Waiting 2 seconds before next SDK test...")
            time.sleep(2)

    # Print summary
    print("\n" + "=" * 80)
    print("📊 FINAL SUMMARY")
    print("=" * 80)

    for sdk_type, result in results.items():
        print(f"\n{sdk_type.upper()} SDK:")
        if result["status"] == "SKIP":
            print(f"   ⚠️  SKIPPED: {result['reason']}")
        else:
            print(f"   Token ID: {result['token_id']}")
            print(f"   ✅ Successful: {result['success']}/{result['total']}")
            print(f"   ❌ Failed: {result['fail']}/{result['total']}")

    print("\n" + "=" * 80)
    print("✅ Testing complete!")
    print("=" * 80)
    print("\n🔍 Next: Check SDK Tokens dashboard to verify usage metrics updated:")
    print("   http://localhost:3000/dashboard/sdk-tokens")
    print()
    print("Expected results:")
    print("   - Python SDK token: Usage Count = 3")
    print("   - Go SDK token: Usage Count = 3")
    print("   - JavaScript SDK token: Usage Count = 3")
    print("   - Total Usage: Should increase by 9")

if __name__ == "__main__":
    main()
