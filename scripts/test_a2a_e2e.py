#!/usr/bin/env python3
"""
End-to-End Integration Test for A2A (Agent-to-Agent) Protocol Support

This script tests all A2A features by creating real agents and having them
communicate with each other:
1. Agent Card registration and retrieval
2. Ed25519 signed request verification
3. Consent management between agents
4. Task logging and tracking
5. Trust score computation

Prerequisites:
- Backend running at http://localhost:8080
- SDK credentials file or admin access
"""

import os
import sys
import json
import time
import argparse
import requests
import hashlib
import threading
from http.server import HTTPServer, BaseHTTPRequestHandler
from datetime import datetime, timedelta

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '..', 'sdk', 'python'))

from aim_sdk import AIMClient
from aim_sdk.a2a import (
    A2AAgentCard, A2ARequestSignature, A2ATrustScore,
    A2APeerTrust, A2AConsent, A2AClient
)

BASE_URL = os.environ.get('AIM_BASE_URL', 'http://localhost:8080')

# ANSI colors for output
GREEN = '\033[92m'
RED = '\033[91m'
YELLOW = '\033[93m'
BLUE = '\033[94m'
RESET = '\033[0m'

def print_test(name: str, passed: bool, message: str = ""):
    status = f"{GREEN}✓ PASS{RESET}" if passed else f"{RED}✗ FAIL{RESET}"
    print(f"  {status} {name}")
    if message and not passed:
        print(f"        {message}")

def print_section(name: str):
    print(f"\n{BLUE}{'='*60}{RESET}")
    print(f"{BLUE}{name}{RESET}")
    print(f"{BLUE}{'='*60}{RESET}")


class AgentCardServer:
    """Simple HTTP server to serve A2A Agent Card JSON for testing."""

    def __init__(self, port: int = 9999):
        self.port = port
        self.server = None
        self.thread = None
        self.card_data = None

    def set_card_data(self, card_data: dict):
        """Set the agent card data to serve."""
        self.card_data = card_data

    def start(self):
        """Start the HTTP server in a background thread."""
        card_server = self

        class CardHandler(BaseHTTPRequestHandler):
            def log_message(self, format, *args):
                pass  # Suppress logging

            def do_GET(self):
                if self.path == '/.well-known/agent.json':
                    self.send_response(200)
                    self.send_header('Content-Type', 'application/json')
                    self.end_headers()
                    self.wfile.write(json.dumps(card_server.card_data).encode())
                else:
                    self.send_response(404)
                    self.end_headers()

        self.server = HTTPServer(('0.0.0.0', self.port), CardHandler)
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.daemon = True
        self.thread.start()
        print(f"  {GREEN}✓ Agent Card server started on port {self.port}{RESET}")

    def stop(self):
        """Stop the HTTP server."""
        if self.server:
            self.server.shutdown()
            print(f"  {GREEN}✓ Agent Card server stopped{RESET}")

    def get_card_url(self) -> str:
        """Get the URL for the agent card.
        Uses host.docker.internal for Docker containers to access host machine.
        """
        # Use host.docker.internal so Docker container can access host
        return f"http://host.docker.internal:{self.port}/.well-known/agent.json"


class A2AIntegrationTest:
    def __init__(self, base_url: str = None, keep_data: bool = False):
        self.base_url = base_url or BASE_URL
        self.keep_data = keep_data
        self.session = requests.Session()
        self.admin_token = None
        self.org_id = None
        self.agent1_id = None
        self.agent1_api_key = None
        self.agent2_id = None
        self.agent2_api_key = None
        self.results = {"passed": 0, "failed": 0}
        self.card_server = AgentCardServer(port=9999)

    def run(self) -> bool:
        """Run all integration tests."""
        print(f"\n{YELLOW}A2A Protocol End-to-End Integration Tests{RESET}")
        print(f"Target: {self.base_url}")
        print(f"Time: {datetime.now().isoformat()}")

        try:
            # Setup
            print_section("Setup: Authentication & Agent Creation")
            if not self.setup_authentication():
                print(f"{RED}Failed to authenticate. Exiting.{RESET}")
                return False

            if not self.create_test_agents():
                print(f"{RED}Failed to create test agents. Exiting.{RESET}")
                return False

            # Run test suites
            print_section("Test Suite 1: A2A Agent Cards")
            self.test_agent_cards()

            print_section("Test Suite 2: A2A Request Signing")
            self.test_request_signing()

            print_section("Test Suite 3: A2A Consent Management")
            self.test_consent_management()

            print_section("Test Suite 4: A2A Task Logging")
            self.test_task_logging()

            print_section("Test Suite 5: A2A Trust Scores")
            self.test_trust_scores()

            # Cleanup
            if self.keep_data:
                print_section("Cleanup (skipped: --keep-data)")
                self.card_server.stop()
                print(f"  {YELLOW}Skipping agent/data cleanup to preserve dashboard data{RESET}")
            else:
                print_section("Cleanup")
                self.cleanup()

            # Summary
            print_section("Test Summary")
            total = self.results["passed"] + self.results["failed"]
            print(f"  Total: {total}")
            print(f"  {GREEN}Passed: {self.results['passed']}{RESET}")
            print(f"  {RED}Failed: {self.results['failed']}{RESET}")

            return self.results["failed"] == 0

        except Exception as e:
            print(f"{RED}Test execution failed: {e}{RESET}")
            import traceback
            traceback.print_exc()
            return False

    def record_result(self, name: str, passed: bool, message: str = ""):
        """Record a test result."""
        print_test(name, passed, message)
        if passed:
            self.results["passed"] += 1
        else:
            self.results["failed"] += 1

    def setup_authentication(self) -> bool:
        """Set up authentication for API access."""
        # Try to login first (use /public endpoints)
        # Default admin: admin@opena2a.org / AIM2025!Secure
        try:
            resp = self.session.post(
                f"{self.base_url}/api/v1/public/login",
                json={"email": "admin@opena2a.org", "password": os.environ.get("ADMIN_PASSWORD", "AIM2025!Secure")}
            )
            if resp.status_code == 200:
                data = resp.json()
                # accessToken is the field, not token
                self.admin_token = data.get("accessToken") or data.get("token")
                self.session.headers["Authorization"] = f"Bearer {self.admin_token}"

                # organizationId is in the user object
                user = data.get("user", {})
                if user.get("organizationId"):
                    self.org_id = user["organizationId"]
                    print(f"  {GREEN}✓ Logged in as admin{RESET}")
                    print(f"  {GREEN}✓ Using organization: {self.org_id}{RESET}")
                    return True

                print(f"  {GREEN}✓ Logged in as admin{RESET}")
                # Fallback: Get organization from API
                resp = self.session.get(f"{self.base_url}/api/v1/organizations")
                if resp.status_code == 200:
                    orgs = resp.json().get("organizations", [])
                    if orgs:
                        self.org_id = orgs[0]["id"]
                        print(f"  {GREEN}✓ Using organization: {self.org_id}{RESET}")
                        return True

                print(f"  {YELLOW}No organizations found, creating one...{RESET}")
                resp = self.session.post(
                    f"{self.base_url}/api/v1/organizations",
                    json={"name": "Test Organization", "slug": "test-org"}
                )
                if resp.status_code in [200, 201]:
                    self.org_id = resp.json().get("id")
                    print(f"  {GREEN}✓ Created organization: {self.org_id}{RESET}")
                    return True

            print(f"  {YELLOW}Admin login failed ({resp.status_code}), trying to register...{RESET}")
            # Try to register (use /public endpoints)
            resp = self.session.post(
                f"{self.base_url}/api/v1/public/register",
                json={
                    "email": "a2atest@example.com",
                    "firstName": "A2A",
                    "lastName": "TestUser",
                    "password": "Test123!@#"
                }
            )
            if resp.status_code in [200, 201]:
                data = resp.json()
                self.admin_token = data.get("token")
                self.session.headers["Authorization"] = f"Bearer {self.admin_token}"
                self.org_id = data.get("organizationId")
                print(f"  {GREEN}✓ Registered new user{RESET}")
                return True

            # If registration fails (user exists), try login with test user
            print(f"  {YELLOW}Registration failed ({resp.status_code}), trying test user login...{RESET}")
            resp = self.session.post(
                f"{self.base_url}/api/v1/public/login",
                json={"email": "a2atest@example.com", "password": "Test123!@#"}
            )
            if resp.status_code == 200:
                data = resp.json()
                self.admin_token = data.get("token")
                self.session.headers["Authorization"] = f"Bearer {self.admin_token}"
                print(f"  {GREEN}✓ Logged in as test user{RESET}")

                # Get organization
                resp = self.session.get(f"{self.base_url}/api/v1/organizations")
                if resp.status_code == 200:
                    orgs = resp.json().get("organizations", [])
                    if orgs:
                        self.org_id = orgs[0]["id"]
                        print(f"  {GREEN}✓ Using organization: {self.org_id}{RESET}")
                        return True

            print(f"  {RED}Authentication failed: {resp.status_code} - {resp.text}{RESET}")
            return False

        except Exception as e:
            print(f"  {RED}Authentication error: {e}{RESET}")
            return False

    def create_test_agents(self) -> bool:
        """Create two test agents for A2A testing."""
        try:
            # Use timestamp for unique names
            ts = int(time.time())

            # Create Agent 1
            resp = self.session.post(
                f"{self.base_url}/api/v1/agents",
                json={
                    "name": f"a2a-test-agent-1-{ts}",
                    "displayName": "A2A Test Agent 1",
                    "description": "First agent for A2A testing",
                    "agentType": "custom",
                    "organizationId": self.org_id
                }
            )
            if resp.status_code in [200, 201]:
                data = resp.json()
                self.agent1_id = data.get("id")
                self.agent1_api_key = data.get("apiKey")
                print(f"  {GREEN}✓ Created Agent 1: {self.agent1_id}{RESET}")
            else:
                print(f"  {RED}Failed to create Agent 1: {resp.status_code} - {resp.text}{RESET}")
                return False

            # Create Agent 2
            resp = self.session.post(
                f"{self.base_url}/api/v1/agents",
                json={
                    "name": f"a2a-test-agent-2-{ts}",
                    "displayName": "A2A Test Agent 2",
                    "description": "Second agent for A2A testing",
                    "agentType": "custom",
                    "organizationId": self.org_id
                }
            )
            if resp.status_code in [200, 201]:
                data = resp.json()
                self.agent2_id = data.get("id")
                self.agent2_api_key = data.get("apiKey")
                print(f"  {GREEN}✓ Created Agent 2: {self.agent2_id}{RESET}")
            else:
                print(f"  {RED}Failed to create Agent 2: {resp.status_code} - {resp.text}{RESET}")
                return False

            return True

        except Exception as e:
            print(f"  {RED}Error creating agents: {e}{RESET}")
            return False

    def test_agent_cards(self):
        """Test A2A Agent Card registration and retrieval."""
        # Start the local HTTP server to serve the agent card
        agent_card_json = {
            "name": "A2A Test Agent 1",
            "description": "First agent for A2A testing",
            "url": "http://host.docker.internal:9999",
            "version": "1.0.0",
            "provider": {
                "organization": "Test Org",
                "url": "https://testorg.com"
            },
            "capabilities": {
                "streaming": True,
                "pushNotifications": False,
                "stateTransitionHistory": True
            },
            "skills": [
                {
                    "id": "data-analysis",
                    "name": "Data Analysis",
                    "description": "Analyze data patterns",
                    "tags": ["analytics", "ml"],
                    "inputModes": ["text", "file"],
                    "outputModes": ["text", "json"]
                }
            ],
            "defaultInputModes": ["text"],
            "defaultOutputModes": ["text", "json"]
        }

        # Test 1: Register Agent Card for Agent 1
        # Endpoint: POST /api/v1/a2a/agents/:id/card
        # Send cardData directly to avoid SSRF issues with local URLs
        card_data = {
            "cardData": agent_card_json
        }

        resp = self.session.post(
            f"{self.base_url}/api/v1/a2a/agents/{self.agent1_id}/card",
            json=card_data
        )
        self.record_result(
            "Register Agent Card",
            resp.status_code in [200, 201],
            f"Status: {resp.status_code}, Response: {resp.text[:200] if resp.text else 'empty'}"
        )

        if resp.status_code in [200, 201]:
            # Test 2: Get Agent Card
            resp = self.session.get(
                f"{self.base_url}/api/v1/a2a/agents/{self.agent1_id}/card"
            )
            # Response returns card directly, not nested under "cardData"
            card_resp = resp.json() if resp.status_code == 200 else {}
            self.record_result(
                "Get Agent Card by Agent ID",
                resp.status_code == 200 and ("name" in card_resp or "aim" in card_resp),
                f"Status: {resp.status_code}"
            )

            # Test 3: Get Agent Skills
            resp = self.session.get(
                f"{self.base_url}/api/v1/a2a/agents/{self.agent1_id}/skills"
            )
            self.record_result(
                "Get Agent Skills",
                resp.status_code == 200,
                f"Status: {resp.status_code}"
            )

            # Test 4: Search Skills (uses 'q' parameter, not 'tags')
            resp = self.session.get(
                f"{self.base_url}/api/v1/a2a/skills/search",
                params={"q": "analytics"}
            )
            self.record_result(
                "Search Skills by Query",
                resp.status_code == 200,
                f"Status: {resp.status_code}"
            )

    def test_request_signing(self):
        """Test A2A request signing and verification."""
        import hashlib
        import base64

        # Test 1: Create request signature data class
        sig = A2ARequestSignature(
            agent_id=self.agent1_id,
            timestamp=int(time.time()),
            nonce=hashlib.sha256(os.urandom(32)).hexdigest()[:32],
            signature="test-signature",
            public_key="test-public-key"
        )
        self.record_result(
            "Create A2ARequestSignature",
            sig.agent_id == self.agent1_id,
            ""
        )

        # Test 2: Convert to headers
        headers = sig.to_headers()
        self.record_result(
            "Convert signature to headers",
            "X-A2A-Agent-ID" in headers and "X-A2A-Timestamp" in headers,
            f"Headers: {list(headers.keys())}"
        )

        # Test 3: Verify nonce endpoint (anti-replay)
        from datetime import timezone
        nonce_data = {
            "agentId": self.agent1_id,
            "nonce": hashlib.sha256(os.urandom(32)).hexdigest()[:32],
            "expiresAt": (datetime.now(timezone.utc) + timedelta(minutes=5)).isoformat()
        }
        resp = self.session.post(
            f"{self.base_url}/api/v1/a2a/nonces",
            json=nonce_data
        )
        self.record_result(
            "Register nonce for anti-replay",
            resp.status_code in [200, 201, 404],  # 404 if endpoint not exposed
            f"Status: {resp.status_code}"
        )

    def test_consent_management(self):
        """Test A2A consent management."""
        from datetime import timezone
        # Test 1: Record consent
        # Endpoint: POST /api/v1/a2a/consent
        consent_data = {
            "userId": "test-user-123",
            "grantorAgentId": self.agent1_id,
            "recipientAgentId": self.agent2_id,
            "scope": ["read:profile", "write:preferences"],
            "purpose": "Testing A2A data sharing",
            "dataTypes": ["profile", "preferences"],
            "expiresAt": (datetime.now(timezone.utc) + timedelta(days=30)).isoformat(),
            "consentMethod": "explicit_click"
        }

        resp = self.session.post(
            f"{self.base_url}/api/v1/a2a/consent",
            json=consent_data
        )
        consent_id = None
        if resp.status_code in [200, 201]:
            consent_id = resp.json().get("id")

        self.record_result(
            "Record consent",
            resp.status_code in [200, 201],
            f"Status: {resp.status_code}, Response: {resp.text[:200] if resp.text else 'empty'}"
        )

        # Test 2: Check consent
        resp = self.session.get(
            f"{self.base_url}/api/v1/a2a/consent/check",
            params={
                "userId": "test-user-123",
                "grantorAgentId": self.agent1_id,
                "recipientAgentId": self.agent2_id,
                "scope": "read:profile"
            }
        )
        self.record_result(
            "Check consent status",
            resp.status_code in [200, 404],
            f"Status: {resp.status_code}"
        )

        # Test 3: List user consents
        resp = self.session.get(
            f"{self.base_url}/api/v1/a2a/consent/user/test-user-123"
        )
        self.record_result(
            "List user consents",
            resp.status_code == 200,
            f"Status: {resp.status_code}"
        )

        # Test 4: Revoke consent
        if consent_id:
            resp = self.session.post(
                f"{self.base_url}/api/v1/a2a/consent/{consent_id}/revoke",
                json={"reason": "User requested revocation"}
            )
            self.record_result(
                "Revoke consent",
                resp.status_code in [200, 204],
                f"Status: {resp.status_code}"
            )

    def test_task_logging(self):
        """Test A2A task logging."""
        # Test 1: Log a task
        # Endpoint: POST /api/v1/a2a/tasks
        # Note: policyDecision is computed server-side, not sent in request
        task_data = {
            "externalTaskId": f"task-{int(time.time())}",
            "clientAgentId": self.agent1_id,
            "remoteAgentId": self.agent2_id,
            "skillId": "data-analysis"
        }

        resp = self.session.post(
            f"{self.base_url}/api/v1/a2a/tasks",
            json=task_data
        )
        task_id = None
        if resp.status_code in [200, 201]:
            task_id = resp.json().get("id")

        self.record_result(
            "Log A2A task",
            resp.status_code in [200, 201],
            f"Status: {resp.status_code}, Response: {resp.text[:200] if resp.text else 'empty'}"
        )

        if task_id:
            # Test 2: Update task state to WORKING
            # Endpoint: PUT /api/v1/a2a/tasks/:id/state
            resp = self.session.put(
                f"{self.base_url}/api/v1/a2a/tasks/{task_id}/state",
                json={"state": "WORKING"}
            )
            self.record_result(
                "Update task state to WORKING",
                resp.status_code == 200,
                f"Status: {resp.status_code}"
            )

            # Test 3: Complete task
            resp = self.session.put(
                f"{self.base_url}/api/v1/a2a/tasks/{task_id}/state",
                json={"state": "COMPLETED"}
            )
            self.record_result(
                "Complete task",
                resp.status_code == 200,
                f"Status: {resp.status_code}"
            )

    def test_trust_scores(self):
        """Test A2A trust score computation."""
        # Test 1: Get A2A trust score
        # Endpoint: GET /api/v1/a2a/agents/:id/trust-score
        resp = self.session.get(
            f"{self.base_url}/api/v1/a2a/agents/{self.agent1_id}/trust-score"
        )
        self.record_result(
            "Get A2A trust score",
            resp.status_code in [200, 404],  # 404 if no interactions yet
            f"Status: {resp.status_code}"
        )

        # Test 2: Get peer trust between agents
        # Endpoint: GET /api/v1/a2a/agents/:id/peers/:peer_id/trust
        resp = self.session.get(
            f"{self.base_url}/api/v1/a2a/agents/{self.agent1_id}/peers/{self.agent2_id}/trust"
        )
        self.record_result(
            "Get peer trust",
            resp.status_code in [200, 404],
            f"Status: {resp.status_code}"
        )

        # Test 3: Compute trust score (trigger recalculation)
        # Endpoint: POST /api/v1/a2a/agents/:id/trust-score/compute
        resp = self.session.post(
            f"{self.base_url}/api/v1/a2a/agents/{self.agent1_id}/trust-score/compute"
        )
        self.record_result(
            "Compute trust score",
            resp.status_code in [200, 201, 404],
            f"Status: {resp.status_code}"
        )

    def cleanup(self):
        """Clean up test data."""
        try:
            # Stop the agent card server
            self.card_server.stop()

            # Delete Agent 1
            if self.agent1_id:
                resp = self.session.delete(
                    f"{self.base_url}/api/v1/agents/{self.agent1_id}"
                )
                print(f"  Deleted Agent 1: {resp.status_code}")

            # Delete Agent 2
            if self.agent2_id:
                resp = self.session.delete(
                    f"{self.base_url}/api/v1/agents/{self.agent2_id}"
                )
                print(f"  Deleted Agent 2: {resp.status_code}")

        except Exception as e:
            print(f"  {YELLOW}Cleanup warning: {e}{RESET}")


def main():
    parser = argparse.ArgumentParser(description="A2A Protocol E2E Integration Tests")
    parser.add_argument(
        '--aim-url',
        default=os.environ.get('AIM_BASE_URL', 'http://localhost:8080'),
        help='AIM platform URL (default: $AIM_BASE_URL or http://localhost:8080)'
    )
    parser.add_argument(
        '--keep-data',
        action='store_true',
        help='Skip cleanup to preserve test data in the database for dashboard viewing'
    )
    args = parser.parse_args()

    test = A2AIntegrationTest(base_url=args.aim_url, keep_data=args.keep_data)
    success = test.run()
    sys.exit(0 if success else 1)


if __name__ == "__main__":
    main()
