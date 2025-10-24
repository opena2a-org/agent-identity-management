#!/usr/bin/env python3
"""
QA Verification Script - Confirms All Features Working

This script verifies that after fresh OAuth login, all AIM features work correctly:
- Agent authentication
- Verification event creation
- Activity logging
- Dashboard data population

Run this AFTER getting fresh OAuth credentials.
"""

import sys
import os
import json
import time
import requests
from pathlib import Path

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim-sdk-python'))

from aim_sdk import secure

API_URL = "http://localhost:8080"

def print_section(title):
    """Print formatted section header"""
    print(f"\n{'='*80}")
    print(f"{title}")
    print('='*80)

def print_check(message, status):
    """Print check with status"""
    icon = "✅" if status else "❌"
    print(f"{icon} {message}")

def verify_credentials_exist():
    """Verify fresh credentials are available"""
    print_section("STEP 1: Verifying Fresh Credentials")

    creds_path = Path.home() / ".aim" / "credentials.json"

    if not creds_path.exists():
        print_check("Credentials file exists", False)
        print("\n⚠️  No credentials found!")
        print("Please complete OAuth login and download fresh SDK first.")
        print("\nSee NEXT_STEPS.md for instructions.")
        return False

    print_check("Credentials file exists", True)

    # Check if credentials have OAuth tokens
    try:
        with open(creds_path, 'r') as f:
            creds = json.load(f)

        has_refresh = "refresh_token" in creds
        has_sdk_token = "sdk_token_id" in creds

        print_check(f"Has refresh_token: {has_refresh}", has_refresh)
        print_check(f"Has sdk_token_id: {has_sdk_token}", has_sdk_token)

        if has_refresh and has_sdk_token:
            print("\n✅ Fresh credentials detected!")
            return True
        else:
            print("\n⚠️  Credentials may be stale (missing OAuth tokens)")
            print("Please download fresh SDK from portal.")
            return False

    except Exception as e:
        print_check(f"Credentials readable: {str(e)}", False)
        return False

def verify_agent_registration():
    """Verify agent can register/authenticate"""
    print_section("STEP 2: Verifying Agent Registration")

    try:
        # Use secure() to register/authenticate
        client = secure("flight-search-agent")

        if not client:
            print_check("Agent registration", False)
            return None

        print_check("Agent registration", True)
        print(f"   Agent ID: {client.agent_id}")

        return client

    except Exception as e:
        print_check(f"Agent registration failed: {str(e)}", False)
        return None

def verify_action_verification(client):
    """Verify verification flow works"""
    print_section("STEP 3: Verifying Action Verification Flow")

    try:
        # Request verification for a flight search
        verification = client.verify_action(
            action_type="search_flights",
            resource="NYC",
            context={
                "destination": "NYC",
                "departure_date": "flexible",
                "return_date": "flexible",
                "risk_level": "low"
            }
        )

        if verification and verification.get('verification_id'):
            print_check("Verification request created", True)
            print(f"   Verification ID: {verification['verification_id']}")
            return verification['verification_id']
        else:
            print_check("Verification request created", False)
            print(f"   Response: {verification}")
            return None

    except Exception as e:
        print_check(f"Verification failed: {str(e)}", False)
        return None

def verify_action_logging(client, verification_id):
    """Verify activity logging works"""
    print_section("STEP 4: Verifying Activity Logging")

    try:
        # Log action result
        result = client.log_action_result(
            verification_id=verification_id,
            success=True,
            result_summary="QA test - Found 4 flights to NYC, prices from $179-$289"
        )

        print_check("Activity logging", True)
        print("   Result logged successfully")
        return True

    except Exception as e:
        print_check(f"Activity logging failed: {str(e)}", False)
        return False

def verify_dashboard_data(client):
    """Verify dashboard has data"""
    print_section("STEP 5: Verifying Dashboard Data Population")

    # Give backend a moment to process events
    time.sleep(2)

    # Check verification events endpoint
    try:
        # Get auth token
        creds_path = Path.home() / ".aim" / "credentials.json"
        with open(creds_path, 'r') as f:
            creds = json.load(f)

        access_token = creds.get('access_token')

        # Query verification events
        response = requests.get(
            f"{API_URL}/api/v1/agents/{client.agent_id}/verification-events",
            headers={"Authorization": f"Bearer {access_token}"},
            timeout=10
        )

        if response.status_code == 200:
            events = response.json()
            event_count = len(events) if isinstance(events, list) else events.get('total', 0)

            print_check(f"Verification events retrieved: {event_count} events", event_count > 0)

            if event_count > 0:
                print("   ✅ Recent Activity tab should have data")
                print("   ✅ Trust History tab should have data")
            else:
                print("   ⚠️  No events found - tabs may still be empty")

            return event_count > 0
        else:
            print_check(f"Dashboard API accessible: HTTP {response.status_code}", False)
            return False

    except Exception as e:
        print_check(f"Dashboard check failed: {str(e)}", False)
        return False

def verify_capabilities(client):
    """Verify capabilities were auto-detected"""
    print_section("STEP 6: Verifying Auto-Detected Capabilities")

    try:
        # Get auth token
        creds_path = Path.home() / ".aim" / "credentials.json"
        with open(creds_path, 'r') as f:
            creds = json.load(f)

        access_token = creds.get('access_token')

        # Query agent capabilities
        response = requests.get(
            f"{API_URL}/api/v1/agents/{client.agent_id}",
            headers={"Authorization": f"Bearer {access_token}"},
            timeout=10
        )

        if response.status_code == 200:
            agent_data = response.json()
            capabilities = agent_data.get('capabilities', [])

            print_check(f"Capabilities detected: {len(capabilities)} capabilities", len(capabilities) > 0)

            if capabilities:
                print("   Detected capabilities:")
                for cap in capabilities:
                    print(f"      - {cap.get('name', 'unknown')}")
                print("   ✅ Capabilities tab should have data")

            return len(capabilities) > 0
        else:
            print_check(f"Capabilities API accessible: HTTP {response.status_code}", False)
            return False

    except Exception as e:
        print_check(f"Capabilities check failed: {str(e)}", False)
        return False

def print_final_summary(results):
    """Print final QA summary"""
    print_section("QA VERIFICATION SUMMARY")

    total = len(results)
    passed = sum(1 for r in results.values() if r)

    print(f"\nTests Passed: {passed}/{total}\n")

    for test, status in results.items():
        print_check(test, status)

    print("\n" + "="*80)

    if passed == total:
        print("🎉 ALL CHECKS PASSED - Platform Ready for Production!")
        print("="*80)
        print("\n✅ You can now verify these tabs have data:")
        print("   - Recent Activity (verification events)")
        print("   - Trust History (confidence timeline)")
        print("   - Capabilities (auto-detected)")
        print("   - Graph View (agent relationships)")
        print("\n💡 Navigate to: http://localhost:3000/dashboard/agents")
        return True
    else:
        print("⚠️  SOME CHECKS FAILED - Review Issues Above")
        print("="*80)
        print("\nTroubleshooting:")
        print("1. Ensure you completed fresh OAuth login")
        print("2. Downloaded fresh SDK from portal")
        print("3. Copied credentials to ~/.aim/")
        print("4. Backend is running (docker compose up)")
        print("\nSee NEXT_STEPS.md for detailed instructions.")
        return False

def main():
    """Run complete QA verification"""
    print("\n" + "="*80)
    print("AIM PLATFORM - QA VERIFICATION")
    print("="*80)
    print("\nThis script verifies all features work after fresh OAuth login.")
    print("Run this AFTER completing the steps in NEXT_STEPS.md\n")

    results = {}

    # Step 1: Check credentials
    if not verify_credentials_exist():
        print("\n❌ Cannot proceed without fresh credentials.")
        print("Please complete OAuth login first (see NEXT_STEPS.md)")
        sys.exit(1)

    results["Fresh credentials available"] = True

    # Step 2: Register agent
    client = verify_agent_registration()
    results["Agent registration successful"] = client is not None

    if not client:
        print("\n❌ Cannot proceed without agent registration.")
        sys.exit(1)

    # Step 3: Verify action
    verification_id = verify_action_verification(client)
    results["Verification flow working"] = verification_id is not None

    # Step 4: Log activity
    if verification_id:
        logged = verify_action_logging(client, verification_id)
        results["Activity logging working"] = logged
    else:
        results["Activity logging working"] = False

    # Step 5: Check dashboard data
    has_events = verify_dashboard_data(client)
    results["Dashboard data populated"] = has_events

    # Step 6: Check capabilities
    has_caps = verify_capabilities(client)
    results["Capabilities auto-detected"] = has_caps

    # Final summary
    success = print_final_summary(results)

    sys.exit(0 if success else 1)

if __name__ == "__main__":
    main()
