#!/usr/bin/env python3
"""
AIM Python SDK - Verification Events Test Script
Downloads Python SDK, creates test agent, and triggers verification events

Date: October 19, 2025
Purpose: Test all 6 protocol verifications using actual Python SDK
"""

import requests
import json
import time
import os
import sys
import zipfile
import shutil
import subprocess
from pathlib import Path
from datetime import datetime

# Configuration
API_BASE_URL = "http://localhost:8080/api/v1"
ADMIN_EMAIL = "admin@opena2a.org"
ADMIN_PASSWORD = "admin123"  # Change if needed
SDK_DIR = Path("./aim-sdk-test")

class SDKTester:
    """Test class for AIM Python SDK verification events"""

    def __init__(self):
        self.token = None
        self.org_id = None
        self.agent_id = None
        self.agent_name = "sdk-verification-tester"
        self.sdk_client = None

    def login(self) -> bool:
        """Login to AIM and get auth token"""
        print(f"\n🔐 Logging in as {ADMIN_EMAIL}...")

        try:
            response = requests.post(
                f"{API_BASE_URL}/auth/login",
                json={"email": ADMIN_EMAIL, "password": ADMIN_PASSWORD}
            )

            if response.status_code == 200:
                data = response.json()
                self.token = data.get("token")
                self.org_id = data.get("user", {}).get("organizationId")
                print(f"✅ Login successful! Org ID: {self.org_id}")
                return True
            else:
                print(f"❌ Login failed: {response.status_code}")
                print(f"Response: {response.text}")
                return False

        except Exception as e:
            print(f"❌ Login error: {e}")
            return False

    def headers(self):
        """Get auth headers"""
        return {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json"
        }

    def create_test_agent(self) -> bool:
        """Create test agent for SDK"""
        print(f"\n🤖 Creating test agent: {self.agent_name}...")

        try:
            response = requests.post(
                f"{API_BASE_URL}/agents",
                headers=self.headers(),
                json={
                    "name": self.agent_name,
                    "displayName": f"SDK Verification Tester",
                    "description": "Test agent for Python SDK verification events",
                    "agentType": "ai_agent",
                }
            )

            if response.status_code == 201:
                agent = response.json()
                self.agent_id = agent.get("id")
                print(f"✅ Agent created: {self.agent_id}")
                return True
            else:
                print(f"❌ Failed to create agent: {response.status_code}")
                print(f"Response: {response.text}")
                return False

        except Exception as e:
            print(f"❌ Error creating agent: {e}")
            return False

    def download_sdk(self) -> bool:
        """Download Python SDK from backend"""
        print(f"\n📦 Downloading Python SDK...")

        try:
            # Clean up existing SDK directory
            if SDK_DIR.exists():
                print(f"🧹 Cleaning up existing SDK directory...")
                shutil.rmtree(SDK_DIR)

            # Download SDK
            response = requests.get(
                f"{API_BASE_URL}/agents/{self.agent_id}/sdk?lang=python",
                headers=self.headers()
            )

            if response.status_code == 200:
                # Save ZIP file
                zip_path = SDK_DIR.parent / "aim-sdk.zip"
                with open(zip_path, "wb") as f:
                    f.write(response.content)

                print(f"✅ SDK downloaded ({len(response.content)} bytes)")

                # Extract ZIP
                print(f"📂 Extracting SDK...")
                with zipfile.ZipFile(zip_path, 'r') as zip_ref:
                    zip_ref.extractall(SDK_DIR)

                # Remove ZIP file
                zip_path.unlink()

                print(f"✅ SDK extracted to {SDK_DIR}")
                return True
            else:
                print(f"❌ Failed to download SDK: {response.status_code}")
                print(f"Response: {response.text}")
                return False

        except Exception as e:
            print(f"❌ Error downloading SDK: {e}")
            return False

    def install_sdk(self) -> bool:
        """Install SDK dependencies"""
        print(f"\n📥 Installing SDK dependencies...")

        try:
            # Install in development mode
            result = subprocess.run(
                [sys.executable, "-m", "pip", "install", "-e", str(SDK_DIR)],
                capture_output=True,
                text=True,
                cwd=SDK_DIR
            )

            if result.returncode == 0:
                print(f"✅ SDK installed successfully")
                return True
            else:
                print(f"❌ SDK installation failed:")
                print(result.stderr)
                return False

        except Exception as e:
            print(f"❌ Error installing SDK: {e}")
            return False

    def initialize_sdk_client(self):
        """Import and initialize AIM SDK client"""
        print(f"\n🔧 Initializing SDK client...")

        try:
            # Add SDK to Python path
            sys.path.insert(0, str(SDK_DIR))

            # Import AIM SDK
            from aim_sdk import AIMClient
            from aim_sdk.config import AGENT_ID, PUBLIC_KEY, PRIVATE_KEY, AIM_URL

            print(f"✅ SDK modules imported successfully")
            print(f"   Agent ID: {AGENT_ID[:20]}...")
            print(f"   AIM URL: {AIM_URL}")

            # Initialize client
            self.sdk_client = AIMClient(
                agent_id=AGENT_ID,
                public_key=PUBLIC_KEY,
                private_key=PRIVATE_KEY,
                aim_url=AIM_URL
            )

            print(f"✅ SDK client initialized")
            return True

        except Exception as e:
            print(f"❌ Error initializing SDK: {e}")
            import traceback
            traceback.print_exc()
            return False

    def test_mcp_protocol_verification(self):
        """Test MCP protocol verification using SDK"""
        print("\n" + "="*60)
        print("🧪 Testing MCP Protocol Verification")
        print("="*60)

        try:
            # Test MCP server registration (triggers MCP protocol verification)
            result = self.sdk_client.register_mcp(
                mcp_server_id="test-mcp-server-123",
                detection_method="manual",
                confidence=0.95,
                metadata={
                    "server_name": "test-mcp-server",
                    "capabilities": ["tools/list", "tools/execute"],
                    "protocol": "MCP"
                }
            )

            if result:
                print(f"✅ MCP verification triggered via SDK")
                return True
            else:
                print(f"⚠️  MCP verification returned False")
                return False

        except Exception as e:
            print(f"❌ MCP verification error: {e}")
            return False

    def test_capability_verification(self):
        """Test capability verification (A2A protocol)"""
        print("\n" + "="*60)
        print("🧪 Testing Capability Verification (A2A)")
        print("="*60)

        try:
            # Report capabilities (triggers A2A verification)
            result = self.sdk_client.report_capabilities(
                capabilities=[
                    {"name": "file_access", "confidence": 0.9},
                    {"name": "database_query", "confidence": 0.85},
                    {"name": "api_call", "confidence": 0.95}
                ]
            )

            if result:
                print(f"✅ A2A capability verification triggered")
                return True
            else:
                print(f"⚠️  Capability verification returned False")
                return False

        except Exception as e:
            print(f"❌ Capability verification error: {e}")
            return False

    def test_action_verification(self):
        """Test action verification using decorator"""
        print("\n" + "="*60)
        print("🧪 Testing Action Verification")
        print("="*60)

        try:
            # Use SDK decorator for action verification
            @self.sdk_client.perform_action("database_query", resource="users_table")
            def query_database():
                """Simulated database query"""
                time.sleep(0.1)
                return {"users": [{"id": 1, "name": "Test User"}]}

            # Execute decorated function (triggers verification)
            result = query_database()

            if result:
                print(f"✅ Action verification triggered via decorator")
                print(f"   Result: {result}")
                return True
            else:
                print(f"⚠️  Action verification returned None")
                return False

        except Exception as e:
            print(f"❌ Action verification error: {e}")
            import traceback
            traceback.print_exc()
            return False

    def test_sdk_integration_reporting(self):
        """Test SDK integration reporting"""
        print("\n" + "="*60)
        print("🧪 Testing SDK Integration Reporting")
        print("="*60)

        try:
            # Report SDK integration (triggers verification event)
            result = self.sdk_client.report_sdk_integration(
                sdk_version="1.0.0",
                platform="python3.11",
                capabilities=["ed25519_signing", "oauth_integration", "mcp_detection"]
            )

            if result:
                print(f"✅ SDK integration reporting triggered")
                return True
            else:
                print(f"⚠️  SDK integration reporting returned False")
                return False

        except Exception as e:
            print(f"❌ SDK integration reporting error: {e}")
            return False

    def get_verification_statistics(self):
        """Get statistics to verify events were created"""
        print(f"\n📊 Fetching verification statistics...")

        try:
            response = requests.get(
                f"{API_BASE_URL}/verification-events/statistics?period=24h",
                headers=self.headers()
            )

            if response.status_code == 200:
                stats = response.json()
                print("\n" + "="*60)
                print("📈 VERIFICATION STATISTICS")
                print("="*60)
                print(f"Total Verifications: {stats.get('totalVerifications', 0)}")
                print(f"Success Count: {stats.get('successCount', 0)}")
                print(f"Success Rate: {stats.get('successRate', 0):.2f}%")

                print("\n📡 Protocol Distribution:")
                for protocol, count in stats.get('protocolDistribution', {}).items():
                    print(f"  {protocol}: {count}")

                print("\n🔧 Verification Type Distribution:")
                for vtype, count in stats.get('typeDistribution', {}).items():
                    print(f"  {vtype}: {count}")

                return stats
            else:
                print(f"❌ Failed to get statistics: {response.status_code}")
                return None

        except Exception as e:
            print(f"❌ Error getting statistics: {e}")
            return None

    def cleanup(self):
        """Clean up test agent and SDK directory"""
        print("\n🧹 Cleaning up...")

        # Delete agent
        if self.agent_id:
            try:
                response = requests.delete(
                    f"{API_BASE_URL}/agents/{self.agent_id}",
                    headers=self.headers()
                )
                if response.status_code in [200, 204]:
                    print(f"✅ Test agent deleted")
            except Exception as e:
                print(f"⚠️  Could not delete agent: {e}")

        # Remove SDK directory
        if SDK_DIR.exists():
            try:
                shutil.rmtree(SDK_DIR)
                print(f"✅ SDK directory removed")
            except Exception as e:
                print(f"⚠️  Could not remove SDK: {e}")


def main():
    """Main test execution"""
    print("\n" + "="*60)
    print("🚀 AIM PYTHON SDK VERIFICATION EVENTS TEST")
    print("="*60)
    print(f"Start Time: {datetime.now().isoformat()}")
    print(f"API URL: {API_BASE_URL}")
    print("="*60)

    tester = SDKTester()

    # Step 1: Login
    if not tester.login():
        print("\n❌ Test failed at login step")
        return

    # Step 2: Create agent
    if not tester.create_test_agent():
        print("\n❌ Test failed at agent creation step")
        return

    # Step 3: Download SDK
    if not tester.download_sdk():
        print("\n❌ Test failed at SDK download step")
        tester.cleanup()
        return

    # Step 4: Install SDK
    if not tester.install_sdk():
        print("\n❌ Test failed at SDK installation step")
        tester.cleanup()
        return

    # Step 5: Initialize SDK client
    if not tester.initialize_sdk_client():
        print("\n❌ Test failed at SDK initialization step")
        tester.cleanup()
        return

    # Step 6: Run verification tests
    print("\n" + "="*60)
    print("🧪 RUNNING SDK VERIFICATION TESTS")
    print("="*60)

    tests_passed = 0
    tests_total = 4

    if tester.test_mcp_protocol_verification():
        tests_passed += 1
    time.sleep(1)

    if tester.test_capability_verification():
        tests_passed += 1
    time.sleep(1)

    if tester.test_action_verification():
        tests_passed += 1
    time.sleep(1)

    if tester.test_sdk_integration_reporting():
        tests_passed += 1
    time.sleep(1)

    # Step 7: Get statistics
    stats = tester.get_verification_statistics()

    # Summary
    print("\n" + "="*60)
    print("✅ TEST SUITE COMPLETE")
    print("="*60)
    print(f"Tests Passed: {tests_passed}/{tests_total}")

    if stats:
        print(f"Verification Events Created: {stats.get('totalVerifications', 0)}")
        print(f"Success Rate: {stats.get('successRate', 0):.2f}%")

    print(f"\n🌐 View monitoring dashboard at:")
    print(f"   http://localhost:3000/dashboard/monitoring")

    # Ask about cleanup
    cleanup = input("\n🧹 Clean up test agent and SDK? (y/n): ").lower()
    if cleanup == 'y':
        tester.cleanup()
        print("✅ Cleanup complete")
    else:
        print(f"ℹ️  Test files kept at: {SDK_DIR}")

    print("\n" + "="*60)
    print(f"End Time: {datetime.now().isoformat()}")
    print("="*60)


if __name__ == "__main__":
    main()
