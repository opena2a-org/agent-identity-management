#!/usr/bin/env python3
"""
Simple Python SDK Test
Uses existing agent ID and downloads SDK to verify it works
"""

import requests
import zipfile
import io
import os
import sys
from pathlib import Path

# Configuration
API_BASE_URL = "http://localhost:8080/api/v1"
SDK_DIR = Path("./aim-sdk-simple-test")

# Get token from user
print("=" * 60)
print("🚀 SIMPLE PYTHON SDK TEST")
print("=" * 60)
print()
print("To get your auth token:")
print("1. Open http://localhost:3000 in your browser")
print("2. Open browser console (F12)")
print("3. Run: localStorage.getItem('auth_token')")
print("4. Copy the token")
print()

TOKEN = input("Enter your auth token: ").strip()

if not TOKEN:
    print("❌ No token provided. Exiting.")
    sys.exit(1)

headers = {
    "Authorization": f"Bearer {TOKEN}",
    "Content-Type": "application/json"
}

# List agents to get an agent ID
print("\n📋 Fetching your agents...")
try:
    response = requests.get(f"{API_BASE_URL}/agents", headers=headers)
    if response.status_code == 200:
        agents = response.json()
        if agents:
            agent = agents[0]
            agent_id = agent.get('id')
            agent_name = agent.get('name', 'Unknown')
            print(f"✅ Found agent: {agent_name} ({agent_id})")
        else:
            print("❌ No agents found. Please create an agent first.")
            sys.exit(1)
    else:
        print(f"❌ Failed to fetch agents: {response.status_code}")
        print(f"Response: {response.text}")
        sys.exit(1)
except Exception as e:
    print(f"❌ Error fetching agents: {e}")
    sys.exit(1)

# Download SDK
print(f"\n📦 Downloading Python SDK for agent {agent_id}...")
try:
    response = requests.get(
        f"{API_BASE_URL}/agents/{agent_id}/sdk?lang=python",
        headers=headers
    )

    if response.status_code == 200:
        print(f"✅ SDK downloaded ({len(response.content)} bytes)")

        # Clean up existing SDK directory
        if SDK_DIR.exists():
            print(f"🧹 Cleaning up existing SDK directory...")
            import shutil
            shutil.rmtree(SDK_DIR)

        # Extract ZIP
        print(f"📂 Extracting SDK...")
        with zipfile.ZipFile(io.BytesIO(response.content)) as zip_ref:
            zip_ref.extractall(SDK_DIR)

        print(f"✅ SDK extracted to {SDK_DIR}")

        # List SDK contents
        print(f"\n📁 SDK Contents:")
        for root, dirs, files in os.walk(SDK_DIR):
            level = root.replace(str(SDK_DIR), '').count(os.sep)
            indent = ' ' * 2 * level
            print(f'{indent}{os.path.basename(root)}/')
            subindent = ' ' * 2 * (level + 1)
            for file in files:
                print(f'{subindent}{file}')

        # Check for key files
        print(f"\n🔍 Verifying SDK structure...")
        required_files = [
            "aim_sdk/__init__.py",
            "aim_sdk/client.py",
            "aim_sdk/config.py",
            "setup.py",
            "README.md"
        ]

        all_present = True
        for file_path in required_files:
            full_path = SDK_DIR / file_path
            if full_path.exists():
                print(f"  ✅ {file_path}")
            else:
                print(f"  ❌ {file_path} - MISSING!")
                all_present = False

        if all_present:
            print(f"\n🎉 SUCCESS! Python SDK is complete and ready to use!")
            print(f"\nNext steps:")
            print(f"  cd {SDK_DIR}")
            print(f"  pip install -e .")
            print(f"  python example.py")
        else:
            print(f"\n⚠️  SDK is incomplete - some files are missing")

    else:
        print(f"❌ Failed to download SDK: {response.status_code}")
        print(f"Response: {response.text}")
        sys.exit(1)

except Exception as e:
    print(f"❌ Error downloading SDK: {e}")
    import traceback
    traceback.print_exc()
    sys.exit(1)

print(f"\n{'=' * 60}")
print("Test complete!")
print(f"{'=' * 60}")
