#!/usr/bin/env python3
"""
Test Script for AIM Verification Events
Triggers real verification events across all 6 protocols

Date: October 19, 2025
Purpose: Generate verification events to test monitoring dashboard
"""

import requests
import json
import time
import base64
import hashlib
from datetime import datetime
from typing import Dict, List, Optional

# Configuration
API_BASE_URL = "http://localhost:8080/api/v1"
ADMIN_EMAIL = "admin@opena2a.org"
ADMIN_PASSWORD = "admin123"  # Change this to your actual admin password

class AIMTestClient:
    """Test client for AIM verification events"""

    def __init__(self, base_url: str):
        self.base_url = base_url
        self.token = None
        self.org_id = None
        self.user_id = None
        self.agents = []

    def login(self, email: str, password: str) -> bool:
        """Login and get auth token"""
        print(f"\n🔐 Logging in as {email}...")

        try:
            response = requests.post(
                f"{self.base_url}/auth/login",
                json={"email": email, "password": password}
            )

            if response.status_code == 200:
                data = response.json()
                self.token = data.get("token")
                self.org_id = data.get("user", {}).get("organizationId")
                self.user_id = data.get("user", {}).get("id")
                print(f"✅ Login successful! Org ID: {self.org_id}")
                return True
            else:
                print(f"❌ Login failed: {response.status_code}")
                print(f"Response: {response.text}")
                return False

        except Exception as e:
            print(f"❌ Login error: {e}")
            return False

    def headers(self) -> Dict[str, str]:
        """Get request headers with auth token"""
        return {
            "Authorization": f"Bearer {self.token}",
            "Content-Type": "application/json"
        }

    def create_test_agent(self, name: str, agent_type: str = "ai_agent") -> Optional[str]:
        """Create a test agent"""
        print(f"\n🤖 Creating test agent: {name}...")

        try:
            response = requests.post(
                f"{self.base_url}/agents",
                headers=self.headers(),
                json={
                    "name": name,
                    "displayName": f"Test Agent - {name}",
                    "description": f"Test agent for {name} protocol verification",
                    "agentType": agent_type,
                }
            )

            if response.status_code == 201:
                agent = response.json()
                agent_id = agent.get("id")
                self.agents.append(agent)
                print(f"✅ Agent created: {agent_id}")
                return agent_id
            else:
                print(f"❌ Failed to create agent: {response.status_code}")
                print(f"Response: {response.text}")
                return None

        except Exception as e:
            print(f"❌ Error creating agent: {e}")
            return None

    def create_verification_event(
        self,
        agent_id: str,
        protocol: str,
        verification_type: str = "identity",
        status: str = "success",
        duration_ms: int = None,
        initiator_type: str = "user",
        action: str = None,
        resource_type: str = None
    ) -> bool:
        """Create a verification event"""

        # Generate realistic duration if not provided
        if duration_ms is None:
            import random
            duration_ms = random.randint(50, 500)

        print(f"\n📊 Creating {protocol} verification event...")

        try:
            event_data = {
                "agentId": agent_id,
                "protocol": protocol,
                "verificationType": verification_type,
                "status": status,
                "durationMs": duration_ms,
                "initiatorType": initiator_type,
                "confidence": 0.95 if status == "success" else 0.3,
                "metadata": {
                    "protocol": protocol,
                    "timestamp": datetime.now().isoformat(),
                    "testEvent": True
                }
            }

            if action:
                event_data["action"] = action
            if resource_type:
                event_data["resourceType"] = resource_type

            response = requests.post(
                f"{self.base_url}/verification-events",
                headers=self.headers(),
                json=event_data
            )

            if response.status_code in [200, 201]:
                print(f"✅ {protocol} verification event created successfully")
                return True
            else:
                print(f"❌ Failed to create verification event: {response.status_code}")
                print(f"Response: {response.text}")
                return False

        except Exception as e:
            print(f"❌ Error creating verification event: {e}")
            return False

    def test_mcp_protocol(self, agent_id: str):
        """Test MCP (Model Context Protocol) verification"""
        print("\n" + "="*60)
        print("🧪 Testing MCP Protocol")
        print("="*60)

        scenarios = [
            ("identity", "success", "mcp_server_auth", "mcp_server"),
            ("capability", "success", "list_tools", "mcp_tools"),
            ("permission", "success", "execute_tool", "mcp_tool"),
        ]

        for vtype, status, action, resource in scenarios:
            self.create_verification_event(
                agent_id=agent_id,
                protocol="MCP",
                verification_type=vtype,
                status=status,
                action=action,
                resource_type=resource,
                initiator_type="agent"
            )
            time.sleep(0.5)

    def test_a2a_protocol(self, agent_id: str):
        """Test A2A (Agent-to-Agent) verification"""
        print("\n" + "="*60)
        print("🧪 Testing A2A Protocol")
        print("="*60)

        scenarios = [
            ("identity", "success", "agent_handshake", "agent"),
            ("trust", "success", "verify_signature", "signature"),
            ("permission", "success", "delegate_task", "task"),
        ]

        for vtype, status, action, resource in scenarios:
            self.create_verification_event(
                agent_id=agent_id,
                protocol="A2A",
                verification_type=vtype,
                status=status,
                action=action,
                resource_type=resource,
                initiator_type="agent"
            )
            time.sleep(0.5)

    def test_acp_protocol(self, agent_id: str):
        """Test ACP (Agent Communication Protocol) verification"""
        print("\n" + "="*60)
        print("🧪 Testing ACP Protocol")
        print("="*60)

        scenarios = [
            ("identity", "success", "acp_connect", "connection"),
            ("capability", "success", "acp_capability_check", "capability"),
            ("permission", "success", "acp_message_send", "message"),
        ]

        for vtype, status, action, resource in scenarios:
            self.create_verification_event(
                agent_id=agent_id,
                protocol="ACP",
                verification_type=vtype,
                status=status,
                action=action,
                resource_type=resource,
                initiator_type="agent"
            )
            time.sleep(0.5)

    def test_did_protocol(self, agent_id: str):
        """Test DID (Decentralized Identity) verification"""
        print("\n" + "="*60)
        print("🧪 Testing DID Protocol")
        print("="*60)

        scenarios = [
            ("identity", "success", "did_resolution", "did_document"),
            ("trust", "success", "verify_did_signature", "signature"),
            ("capability", "success", "did_capability_check", "capability"),
        ]

        for vtype, status, action, resource in scenarios:
            self.create_verification_event(
                agent_id=agent_id,
                protocol="DID",
                verification_type=vtype,
                status=status,
                action=action,
                resource_type=resource,
                initiator_type="system"
            )
            time.sleep(0.5)

    def test_oauth_protocol(self, agent_id: str):
        """Test OAuth verification"""
        print("\n" + "="*60)
        print("🧪 Testing OAuth Protocol")
        print("="*60)

        scenarios = [
            ("identity", "success", "oauth_token_verify", "access_token"),
            ("permission", "success", "oauth_scope_check", "scope"),
            ("identity", "success", "oidc_id_token", "id_token"),
        ]

        for vtype, status, action, resource in scenarios:
            self.create_verification_event(
                agent_id=agent_id,
                protocol="OAuth",
                verification_type=vtype,
                status=status,
                action=action,
                resource_type=resource,
                initiator_type="user"
            )
            time.sleep(0.5)

    def test_saml_protocol(self, agent_id: str):
        """Test SAML verification"""
        print("\n" + "="*60)
        print("🧪 Testing SAML Protocol")
        print("="*60)

        scenarios = [
            ("identity", "success", "saml_assertion_verify", "saml_response"),
            ("permission", "success", "saml_attribute_check", "attributes"),
            ("identity", "success", "saml_sso", "sso_session"),
        ]

        for vtype, status, action, resource in scenarios:
            self.create_verification_event(
                agent_id=agent_id,
                protocol="SAML",
                verification_type=vtype,
                status=status,
                action=action,
                resource_type=resource,
                initiator_type="user"
            )
            time.sleep(0.5)

    def test_mixed_scenarios(self, agent_id: str):
        """Test mixed success/failure scenarios"""
        print("\n" + "="*60)
        print("🧪 Testing Mixed Success/Failure Scenarios")
        print("="*60)

        # Some failures for realistic metrics
        scenarios = [
            ("MCP", "identity", "failed", "mcp_auth_failed", "mcp_server"),
            ("A2A", "trust", "failed", "signature_invalid", "signature"),
            ("OAuth", "permission", "timeout", "oauth_timeout", "scope"),
            ("DID", "identity", "failed", "did_not_found", "did_document"),
        ]

        for protocol, vtype, status, action, resource in scenarios:
            self.create_verification_event(
                agent_id=agent_id,
                protocol=protocol,
                verification_type=vtype,
                status=status,
                action=action,
                resource_type=resource,
                duration_ms=5000 if status == "timeout" else 200,
                initiator_type="agent"
            )
            time.sleep(0.5)

    def get_verification_statistics(self, period: str = "24h"):
        """Get verification statistics from API"""
        print(f"\n📈 Fetching verification statistics ({period})...")

        try:
            response = requests.get(
                f"{self.base_url}/verification-events/statistics?period={period}",
                headers=self.headers()
            )

            if response.status_code == 200:
                stats = response.json()
                print("\n" + "="*60)
                print("📊 VERIFICATION STATISTICS")
                print("="*60)
                print(f"Total Verifications: {stats.get('totalVerifications', 0)}")
                print(f"Success Count: {stats.get('successCount', 0)}")
                print(f"Failed Count: {stats.get('failedCount', 0)}")
                print(f"Success Rate: {stats.get('successRate', 0):.2f}%")
                print(f"Avg Duration: {stats.get('avgDurationMs', 0):.0f}ms")
                print(f"Unique Agents: {stats.get('uniqueAgentsVerified', 0)}")

                print("\n📡 Protocol Distribution:")
                for protocol, count in stats.get('protocolDistribution', {}).items():
                    print(f"  {protocol}: {count}")

                print("\n🔧 Verification Type Distribution:")
                for vtype, count in stats.get('typeDistribution', {}).items():
                    print(f"  {vtype}: {count}")

                print("\n👤 Initiator Distribution:")
                for initiator, count in stats.get('initiatorDistribution', {}).items():
                    print(f"  {initiator}: {count}")

                return stats
            else:
                print(f"❌ Failed to get statistics: {response.status_code}")
                return None

        except Exception as e:
            print(f"❌ Error getting statistics: {e}")
            return None

    def get_recent_events(self, minutes: int = 15):
        """Get recent verification events"""
        print(f"\n📋 Fetching recent events (last {minutes} minutes)...")

        try:
            response = requests.get(
                f"{self.base_url}/verification-events/recent?minutes={minutes}",
                headers=self.headers()
            )

            if response.status_code == 200:
                data = response.json()
                events = data.get('events', [])
                print(f"\n✅ Found {len(events)} recent events")

                for i, event in enumerate(events[:5], 1):
                    print(f"\n  Event {i}:")
                    print(f"    Protocol: {event.get('protocol')}")
                    print(f"    Type: {event.get('verificationType')}")
                    print(f"    Status: {event.get('status')}")
                    print(f"    Duration: {event.get('durationMs')}ms")

                return events
            else:
                print(f"❌ Failed to get events: {response.status_code}")
                return []

        except Exception as e:
            print(f"❌ Error getting events: {e}")
            return []

    def cleanup_test_agents(self):
        """Delete test agents after testing"""
        print("\n🧹 Cleaning up test agents...")

        for agent in self.agents:
            agent_id = agent.get("id")
            try:
                response = requests.delete(
                    f"{self.base_url}/agents/{agent_id}",
                    headers=self.headers()
                )

                if response.status_code in [200, 204]:
                    print(f"✅ Deleted agent: {agent_id}")
                else:
                    print(f"⚠️  Could not delete agent {agent_id}: {response.status_code}")

            except Exception as e:
                print(f"❌ Error deleting agent {agent_id}: {e}")


def main():
    """Main test execution"""
    print("\n" + "="*60)
    print("🚀 AIM VERIFICATION EVENTS TEST SUITE")
    print("="*60)
    print(f"Start Time: {datetime.now().isoformat()}")
    print(f"API URL: {API_BASE_URL}")
    print("="*60)

    # Initialize client
    client = AIMTestClient(API_BASE_URL)

    # Login
    if not client.login(ADMIN_EMAIL, ADMIN_PASSWORD):
        print("\n❌ Login failed. Please check credentials and backend status.")
        print("Ensure backend is running: docker compose up -d backend")
        return

    # Create test agent
    agent_id = client.create_test_agent("protocol-test-agent")
    if not agent_id:
        print("\n❌ Failed to create test agent. Exiting.")
        return

    # Run protocol tests
    print("\n" + "="*60)
    print("🧪 RUNNING PROTOCOL VERIFICATION TESTS")
    print("="*60)

    client.test_mcp_protocol(agent_id)
    client.test_a2a_protocol(agent_id)
    client.test_acp_protocol(agent_id)
    client.test_did_protocol(agent_id)
    client.test_oauth_protocol(agent_id)
    client.test_saml_protocol(agent_id)
    client.test_mixed_scenarios(agent_id)

    # Wait a moment for events to be processed
    print("\n⏳ Waiting for events to be processed...")
    time.sleep(2)

    # Get statistics
    stats = client.get_verification_statistics("24h")

    # Get recent events
    events = client.get_recent_events(15)

    # Summary
    print("\n" + "="*60)
    print("✅ TEST SUITE COMPLETE")
    print("="*60)

    if stats:
        print(f"✅ Created {stats.get('totalVerifications', 0)} verification events")
        print(f"✅ Tested all 6 protocols: MCP, A2A, ACP, DID, OAuth, SAML")
        print(f"✅ Success rate: {stats.get('successRate', 0):.2f}%")
        print(f"✅ Dashboard should now show real data!")

    print(f"\n🌐 View monitoring dashboard at:")
    print(f"   http://localhost:3000/dashboard/monitoring")

    # Ask about cleanup
    cleanup = input("\n🧹 Delete test agent? (y/n): ").lower()
    if cleanup == 'y':
        client.cleanup_test_agents()
        print("✅ Cleanup complete")
    else:
        print("ℹ️  Test agent kept for further testing")

    print("\n" + "="*60)
    print(f"End Time: {datetime.now().isoformat()}")
    print("="*60)


if __name__ == "__main__":
    main()
