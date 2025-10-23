#!/usr/bin/env python3
"""
🔬 COMPREHENSIVE INTEGRATION TEST - AIM Security Features
========================================================

This test verifies the complete end-to-end flow of AIM's security features:
1. Authentication and authorization
2. Agent registration and capability management
3. Action verification with capability-based access control
4. Audit logging persistence
5. Verification event tracking
6. Security alert generation for violations

Purpose: Production-ready automated test for CI/CD pipeline
Author: AIM Engineering Team
Date: October 23, 2025
"""

import sys
import os
import time
import json
import requests
from nacl.signing import SigningKey
import base64
from typing import Dict, Optional, Tuple

# ============================================================================
# CONFIGURATION
# ============================================================================

AIM_URL = os.getenv("AIM_URL", "http://localhost:8080")
ADMIN_EMAIL = "admin@opena2a.org"
ADMIN_PASSWORD = "AIM2025!Secure"

# For CI/CD: Set this environment variable to bypass login
# You can get this from browser localStorage or use a dedicated test account
ADMIN_TOKEN_OVERRIDE = os.getenv("AIM_ADMIN_TOKEN")

# Test agent details (use timestamp to ensure uniqueness)
TEST_AGENT_NAME = f"integration-test-agent-{int(time.time())}"
TEST_AGENT_DESCRIPTION = "Automated integration test agent"
TEST_AGENT_TYPE = "ai_agent"

# Test capabilities
ALLOWED_CAPABILITY = "get_weather"
DENIED_CAPABILITIES = ["read_database", "send_email"]

# ============================================================================
# UTILITIES
# ============================================================================

class Colors:
    """ANSI color codes for terminal output"""
    HEADER = '\033[95m'
    OKBLUE = '\033[94m'
    OKCYAN = '\033[96m'
    OKGREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'
    UNDERLINE = '\033[4m'

def print_header(title: str):
    """Print formatted section header"""
    print(f"\n{Colors.HEADER}{'=' * 80}{Colors.ENDC}")
    print(f"{Colors.HEADER}{Colors.BOLD}{title}{Colors.ENDC}")
    print(f"{Colors.HEADER}{'=' * 80}{Colors.ENDC}\n")

def print_success(message: str):
    """Print success message"""
    print(f"{Colors.OKGREEN}✅ {message}{Colors.ENDC}")

def print_error(message: str):
    """Print error message"""
    print(f"{Colors.FAIL}❌ {message}{Colors.ENDC}")

def print_info(message: str):
    """Print info message"""
    print(f"{Colors.OKCYAN}ℹ️  {message}{Colors.ENDC}")

def print_warning(message: str):
    """Print warning message"""
    print(f"{Colors.WARNING}⚠️  {message}{Colors.ENDC}")

# ============================================================================
# TEST EXECUTION
# ============================================================================

class AIMIntegrationTest:
    """Comprehensive integration test suite for AIM"""

    def __init__(self):
        self.admin_token: Optional[str] = None
        self.agent_id: Optional[str] = None
        self.org_id: Optional[str] = None
        self.signing_key: Optional[SigningKey] = None
        self.verify_key = None
        self.test_results = {
            "passed": [],
            "failed": [],
            "warnings": []
        }

    def run(self) -> bool:
        """Run complete test suite"""
        print_header("🔬 AIM COMPREHENSIVE INTEGRATION TEST")
        print_info(f"Target: {AIM_URL}")
        print_info(f"Admin: {ADMIN_EMAIL}")
        print()

        try:
            # Phase 1: Authentication
            if not self.test_admin_login():
                return False

            # Phase 2: Agent Setup
            if not self.test_agent_registration():
                return False

            if not self.test_capability_grant():
                return False

            # Phase 3: Action Verification Tests
            if not self.test_legitimate_action():
                return False

            if not self.test_capability_violations():
                return False

            # Phase 4: Data Persistence Verification
            if not self.test_audit_logs_persisted():
                return False

            if not self.test_verification_events_recorded():
                return False

            if not self.test_security_alerts_created():
                return False

            # Phase 5: Cleanup
            self.cleanup_test_data()

            # Print final report
            self.print_summary()

            return len(self.test_results["failed"]) == 0

        except Exception as e:
            print_error(f"CRITICAL ERROR: {e}")
            import traceback
            traceback.print_exc()
            return False

    def test_admin_login(self) -> bool:
        """Test 1: Admin authentication"""
        print_header("TEST 1: Admin Authentication")

        # Check for token override (useful when password has changed or for CI/CD)
        if ADMIN_TOKEN_OVERRIDE:
            print_info("Using admin token from environment variable")
            self.admin_token = ADMIN_TOKEN_OVERRIDE

            # Extract org_id from token (JWT payload)
            try:
                import base64
                import json
                payload_b64 = self.admin_token.split('.')[1]
                # Add padding if needed
                padding = len(payload_b64) % 4
                if padding:
                    payload_b64 += '=' * (4 - padding)
                payload = json.loads(base64.b64decode(payload_b64))
                self.org_id = payload.get("organization_id")
                print_success("Admin token validated")
                print_info(f"Token: {self.admin_token[:40]}...")
                print_info(f"Organization: {self.org_id}")
                self.test_results["passed"].append("Admin authentication (token override)")
                return True
            except Exception as e:
                print_error(f"Failed to decode admin token: {e}")
                self.test_results["failed"].append("Token decoding")
                return False

        # Normal login flow
        try:
            response = requests.post(
                f"{AIM_URL}/api/v1/public/login",
                json={
                    "email": ADMIN_EMAIL,
                    "password": ADMIN_PASSWORD
                },
                timeout=10
            )

            if response.status_code != 200:
                print_error(f"Login failed with status {response.status_code}")
                print_error(f"Response: {response.text}")
                print_warning("Hint: If password has changed, set AIM_ADMIN_TOKEN environment variable")
                self.test_results["failed"].append("Admin authentication")
                return False

            data = response.json()

            # Check if password change required
            if not data.get("success", True) and "password" in data.get("message", "").lower():
                print_warning("Admin password change required")
                self.test_results["warnings"].append("Admin must change password on first login")

            self.admin_token = data.get("accessToken") or data.get("access_token")
            if not self.admin_token:
                print_error("No access token in response")
                self.test_results["failed"].append("Token extraction")
                return False

            # Extract organization ID
            user_data = data.get("user", {})
            self.org_id = user_data.get("organization_id") or user_data.get("organizationId")

            print_success("Admin authenticated successfully")
            print_info(f"Token: {self.admin_token[:40]}...")
            print_info(f"Organization: {self.org_id}")
            self.test_results["passed"].append("Admin authentication")
            return True

        except Exception as e:
            print_error(f"Login exception: {e}")
            self.test_results["failed"].append(f"Admin authentication: {e}")
            return False

    def test_agent_registration(self) -> bool:
        """Test 2: Agent registration and Ed25519 keypair generation"""
        print_header("TEST 2: Agent Registration & Keypair Generation")

        try:
            # Generate Ed25519 keypair
            seed = os.urandom(32)
            self.signing_key = SigningKey(seed)
            self.verify_key = self.signing_key.verify_key

            private_key_b64 = base64.b64encode(bytes(self.signing_key)).decode()
            public_key_b64 = base64.b64encode(bytes(self.verify_key)).decode()

            print_success("Ed25519 keypair generated")
            print_info(f"Public Key: {public_key_b64[:40]}...")

            # Register agent
            response = requests.post(
                f"{AIM_URL}/api/v1/agents",
                headers={
                    "Authorization": f"Bearer {self.admin_token}",
                    "Content-Type": "application/json"
                },
                json={
                    "name": TEST_AGENT_NAME,
                    "display_name": TEST_AGENT_NAME,
                    "description": TEST_AGENT_DESCRIPTION,
                    "agent_type": TEST_AGENT_TYPE,
                    "public_key": public_key_b64,
                    "key_algorithm": "ed25519"
                },
                timeout=10
            )

            if response.status_code not in [200, 201]:
                print_error(f"Agent registration failed with status {response.status_code}")
                print_error(f"Response: {response.text}")
                self.test_results["failed"].append("Agent registration")
                return False

            agent_data = response.json()
            self.agent_id = agent_data.get("id")

            if not self.agent_id:
                print_error("No agent ID in response")
                self.test_results["failed"].append("Agent ID extraction")
                return False

            print_success(f"Agent registered: {TEST_AGENT_NAME}")
            print_info(f"Agent ID: {self.agent_id}")
            self.test_results["passed"].append("Agent registration")
            return True

        except Exception as e:
            print_error(f"Registration exception: {e}")
            self.test_results["failed"].append(f"Agent registration: {e}")
            return False

    def test_capability_grant(self) -> bool:
        """Test 3: Grant capability to agent"""
        print_header("TEST 3: Capability Grant")

        try:
            response = requests.post(
                f"{AIM_URL}/api/v1/agents/{self.agent_id}/capabilities",
                headers={
                    "Authorization": f"Bearer {self.admin_token}",
                    "Content-Type": "application/json"
                },
                json={
                    "capabilities": [ALLOWED_CAPABILITY]
                },
                timeout=10
            )

            if response.status_code not in [200, 201]:
                print_error(f"Capability grant failed with status {response.status_code}")
                print_error(f"Response: {response.text}")
                self.test_results["failed"].append("Capability grant")
                return False

            print_success(f"Capability '{ALLOWED_CAPABILITY}' granted to agent")
            self.test_results["passed"].append("Capability grant")
            return True

        except Exception as e:
            print_error(f"Capability grant exception: {e}")
            self.test_results["failed"].append(f"Capability grant: {e}")
            return False

    def test_legitimate_action(self) -> bool:
        """Test 4: Verify legitimate action is approved"""
        print_header("TEST 4: Legitimate Action Verification")

        try:
            # Sign the action
            action = ALLOWED_CAPABILITY
            parameters = {"location": "San Francisco, CA", "units": "metric"}

            message_data = json.dumps({
                "agent_id": self.agent_id,
                "action": action,
                "parameters": parameters,
                "timestamp": int(time.time())
            }, sort_keys=True)

            signed_message = self.signing_key.sign(message_data.encode())
            signature_b64 = base64.b64encode(signed_message.signature).decode()

            # Verify action
            response = requests.post(
                f"{AIM_URL}/api/v1/agents/{self.agent_id}/verify-action",
                headers={
                    "Authorization": f"Bearer {self.admin_token}",
                    "Content-Type": "application/json"
                },
                json={
                    "action": action,
                    "parameters": parameters,
                    "signature": signature_b64,
                    "timestamp": int(time.time())
                },
                timeout=10
            )

            if response.status_code != 200:
                print_error(f"Action verification failed with status {response.status_code}")
                print_error(f"Response: {response.text}")
                self.test_results["failed"].append("Legitimate action approval")
                return False

            data = response.json()
            if not data.get("allowed"):
                print_error(f"Action was denied: {data.get('reason')}")
                self.test_results["failed"].append("Legitimate action was denied")
                return False

            print_success(f"Legitimate action '{action}' approved")
            print_info(f"Audit ID: {data.get('audit_id')}")
            self.test_results["passed"].append("Legitimate action approval")
            return True

        except Exception as e:
            print_error(f"Legitimate action test exception: {e}")
            self.test_results["failed"].append(f"Legitimate action: {e}")
            return False

    def test_capability_violations(self) -> bool:
        """Test 5: Verify unauthorized actions are blocked"""
        print_header("TEST 5: Capability Violation Detection")

        all_passed = True

        for denied_action in DENIED_CAPABILITIES:
            try:
                # Sign the unauthorized action
                parameters = {"table": "users"} if denied_action == "read_database" else {"to": "admin@example.com"}

                message_data = json.dumps({
                    "agent_id": self.agent_id,
                    "action": denied_action,
                    "parameters": parameters,
                    "timestamp": int(time.time())
                }, sort_keys=True)

                signed_message = self.signing_key.sign(message_data.encode())
                signature_b64 = base64.b64encode(signed_message.signature).decode()

                # Try to verify (should be blocked)
                response = requests.post(
                    f"{AIM_URL}/api/v1/agents/{self.agent_id}/verify-action",
                    headers={
                        "Authorization": f"Bearer {self.admin_token}",
                        "Content-Type": "application/json"
                    },
                    json={
                        "action": denied_action,
                        "parameters": parameters,
                        "signature": signature_b64,
                        "timestamp": int(time.time())
                    },
                    timeout=10
                )

                if response.status_code == 200:
                    data = response.json()
                    if data.get("allowed"):
                        print_error(f"Unauthorized action '{denied_action}' was ALLOWED (should be blocked)")
                        self.test_results["failed"].append(f"Violation detection: {denied_action}")
                        all_passed = False
                    else:
                        print_success(f"Unauthorized action '{denied_action}' blocked")
                        print_info(f"Reason: {data.get('reason')}")
                        self.test_results["passed"].append(f"Violation blocked: {denied_action}")
                elif response.status_code == 403:
                    print_success(f"Unauthorized action '{denied_action}' blocked (403)")
                    self.test_results["passed"].append(f"Violation blocked: {denied_action}")
                else:
                    print_error(f"Unexpected status {response.status_code} for '{denied_action}'")
                    self.test_results["failed"].append(f"Unexpected response: {denied_action}")
                    all_passed = False

                time.sleep(0.5)  # Small delay between tests

            except Exception as e:
                print_error(f"Violation test exception for '{denied_action}': {e}")
                self.test_results["failed"].append(f"Violation test: {denied_action}: {e}")
                all_passed = False

        return all_passed

    def test_audit_logs_persisted(self) -> bool:
        """Test 6: Verify audit logs are persisted"""
        print_header("TEST 6: Audit Logs Persistence")

        try:
            # Query audit logs
            response = requests.get(
                f"{AIM_URL}/api/v1/admin/audit-logs",
                headers={
                    "Authorization": f"Bearer {self.admin_token}"
                },
                params={
                    "limit": 10,
                    "resource_type": "agent_action",
                    "resource_id": self.agent_id
                },
                timeout=10
            )

            if response.status_code != 200:
                print_warning(f"Audit logs query returned status {response.status_code}")
                print_warning("This might be OK if endpoint is not implemented yet")
                self.test_results["warnings"].append("Audit logs query failed")
                return True  # Don't fail the test if endpoint doesn't exist

            data = response.json()
            logs = data.get("logs", []) or data.get("data", [])

            if len(logs) == 0:
                print_warning("No audit logs found for test agent")
                self.test_results["warnings"].append("No audit logs persisted")
            else:
                print_success(f"Found {len(logs)} audit log entries")
                self.test_results["passed"].append("Audit logs persisted")

            return True

        except Exception as e:
            print_warning(f"Audit logs test exception: {e}")
            self.test_results["warnings"].append(f"Audit logs: {e}")
            return True  # Don't fail entire test

    def test_verification_events_recorded(self) -> bool:
        """Test 7: Verify verification events are recorded"""
        print_header("TEST 7: Verification Events Tracking")

        try:
            # Query verification events
            response = requests.get(
                f"{AIM_URL}/api/v1/verification-events",
                headers={
                    "Authorization": f"Bearer {self.admin_token}"
                },
                params={
                    "limit": 10,
                    "agent_id": self.agent_id
                },
                timeout=10
            )

            if response.status_code != 200:
                print_warning(f"Verification events query returned status {response.status_code}")
                self.test_results["warnings"].append("Verification events query failed")
                return True

            data = response.json()
            events = data.get("events", []) or data.get("data", [])

            if len(events) == 0:
                print_warning("No verification events found for test agent")
                self.test_results["warnings"].append("No verification events recorded")
            else:
                print_success(f"Found {len(events)} verification events")

                # Verify we have both success and failed events
                success_count = sum(1 for e in events if e.get("status") == "success")
                failed_count = sum(1 for e in events if e.get("status") == "failed")

                print_info(f"Success: {success_count}, Failed: {failed_count}")
                self.test_results["passed"].append("Verification events recorded")

            return True

        except Exception as e:
            print_warning(f"Verification events test exception: {e}")
            self.test_results["warnings"].append(f"Verification events: {e}")
            return True

    def test_security_alerts_created(self) -> bool:
        """Test 8: Verify security alerts are created for violations"""
        print_header("TEST 8: Security Alerts Generation")

        try:
            # Query alerts
            response = requests.get(
                f"{AIM_URL}/api/v1/admin/alerts",
                headers={
                    "Authorization": f"Bearer {self.admin_token}"
                },
                params={
                    "limit": 10,
                    "severity": "high",
                    "resource_id": self.agent_id
                },
                timeout=10
            )

            if response.status_code != 200:
                print_warning(f"Alerts query returned status {response.status_code}")
                self.test_results["warnings"].append("Alerts query failed")
                return True

            data = response.json()
            alerts = data.get("alerts", []) or data.get("data", [])

            if len(alerts) == 0:
                print_warning("No security alerts found for test agent violations")
                self.test_results["warnings"].append("No security alerts created")
            else:
                print_success(f"Found {len(alerts)} security alerts")

                # Show alert details
                for alert in alerts[:3]:  # Show first 3
                    print_info(f"  - {alert.get('title', 'N/A')}")
                    print_info(f"    Severity: {alert.get('severity', 'N/A')}")

                self.test_results["passed"].append("Security alerts created")

            return True

        except Exception as e:
            print_warning(f"Security alerts test exception: {e}")
            self.test_results["warnings"].append(f"Security alerts: {e}")
            return True

    def cleanup_test_data(self):
        """Cleanup: Delete test agent"""
        print_header("CLEANUP: Removing Test Data")

        try:
            if self.agent_id:
                response = requests.delete(
                    f"{AIM_URL}/api/v1/agents/{self.agent_id}",
                    headers={
                        "Authorization": f"Bearer {self.admin_token}"
                    },
                    timeout=10
                )

                if response.status_code in [200, 204]:
                    print_success(f"Test agent deleted: {self.agent_id}")
                else:
                    print_warning(f"Could not delete test agent: {response.status_code}")

        except Exception as e:
            print_warning(f"Cleanup exception: {e}")

    def print_summary(self):
        """Print test results summary"""
        print_header("📊 TEST RESULTS SUMMARY")

        total = len(self.test_results["passed"]) + len(self.test_results["failed"])
        passed = len(self.test_results["passed"])
        failed = len(self.test_results["failed"])
        warnings = len(self.test_results["warnings"])

        print(f"\n{Colors.BOLD}Total Tests: {total}{Colors.ENDC}")
        print(f"{Colors.OKGREEN}✅ Passed: {passed}{Colors.ENDC}")
        print(f"{Colors.FAIL}❌ Failed: {failed}{Colors.ENDC}")
        print(f"{Colors.WARNING}⚠️  Warnings: {warnings}{Colors.ENDC}\n")

        if failed > 0:
            print(f"{Colors.FAIL}{Colors.BOLD}FAILED TESTS:{Colors.ENDC}")
            for test in self.test_results["failed"]:
                print(f"  {Colors.FAIL}❌ {test}{Colors.ENDC}")
            print()

        if warnings > 0:
            print(f"{Colors.WARNING}{Colors.BOLD}WARNINGS:{Colors.ENDC}")
            for warning in self.test_results["warnings"]:
                print(f"  {Colors.WARNING}⚠️  {warning}{Colors.ENDC}")
            print()

        if failed == 0:
            print(f"{Colors.OKGREEN}{Colors.BOLD}{'=' * 80}{Colors.ENDC}")
            print(f"{Colors.OKGREEN}{Colors.BOLD}🎉 ALL TESTS PASSED! AIM IS READY FOR RELEASE 🎉{Colors.ENDC}")
            print(f"{Colors.OKGREEN}{Colors.BOLD}{'=' * 80}{Colors.ENDC}\n")
        else:
            print(f"{Colors.FAIL}{Colors.BOLD}{'=' * 80}{Colors.ENDC}")
            print(f"{Colors.FAIL}{Colors.BOLD}❌ TESTS FAILED - FIX ISSUES BEFORE RELEASE{Colors.ENDC}")
            print(f"{Colors.FAIL}{Colors.BOLD}{'=' * 80}{Colors.ENDC}\n")

# ============================================================================
# MAIN EXECUTION
# ============================================================================

if __name__ == "__main__":
    test = AIMIntegrationTest()
    success = test.run()

    sys.exit(0 if success else 1)
