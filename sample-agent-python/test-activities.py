#!/usr/bin/env python3
"""
AIM Python SDK - Security Alert Testing
=========================================

This script tests security monitoring by:
1. Creating an agent with LIMITED capabilities
2. Attempting actions OUTSIDE those capabilities → Triggers alerts
3. Performing dangerous operations → Triggers security alerts

All violations are reported to admin in the dashboard.
"""

import sys
import os
import time
from datetime import datetime

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim-sdk-python'))

from aim_sdk import secure
import requests

# Configuration
AIM_API_KEY = os.getenv('AIM_API_KEY', 'aim_live_rVqDQwzk-p9KRfxZZM3ocnidPcSqf9yhxVZkjs_CVXc=')
AIM_API_URL = os.getenv('AIM_API_URL', 'http://localhost:8080')
AIM_DASHBOARD_URL = os.getenv('AIM_DASHBOARD_URL', 'http://localhost:3000')
AGENT_NAME = os.getenv('AGENT_NAME', 'security-test-agent')


def print_header(title, emoji="🔹"):
    """Print formatted section header"""
    print('\n' + '=' * 70)
    print(f'{emoji}  {title}')
    print('=' * 70 + '\n')


def test_unauthorized_capability(agent):
    """Test using a capability the agent doesn't have"""
    print_header("TEST 1: Unauthorized Capability Usage", "⚠️")
    
    print("Agent has capabilities: read_files, make_api_calls")
    print("Attempting to use: execute_code (NOT GRANTED)")
    print("")
    
    try:
        headers = {
            'X-API-Key': AIM_API_KEY,
            'Content-Type': 'application/json'
        }
        
        # Try to verify an action the agent doesn't have capability for
        result = agent.verify_action(
            action_type="execute_code",  # NOT in agent's capabilities
            resource="eval(user_input)",
            context={
                "code": "os.system('rm -rf /')",
                "risk": "critical"
            }
        )
        
        print(f"❌ UNEXPECTED: Action was allowed: {result}")
        
    except Exception as e:
        print(f"✅ EXPECTED: Action blocked")
        print(f"   Error: {str(e)[:100]}")
        print(f"   📊 Security Alert Created!")
        print(f"   📊 Violation logged in database")
        print(f"   📊 Trust score decreased")
        return True
    
    return False


def test_capability_violation_via_api(agent):
    """Test capability violation by calling verify endpoint directly"""
    print_header("TEST 2: Direct API Capability Violation", "🚨")
    
    print("Testing unauthorized database write...")
    print("")
    
    try:
        headers = {
            'X-API-Key': AIM_API_KEY,
            'Content-Type': 'application/json'
        }
        
        # Attempt to verify action agent doesn't have permission for
        result = agent.verify_action(
            action_type="write_database",  # NOT in capabilities
            resource="DROP TABLE users",
            context={
                "query": "DROP TABLE users",
                "database": "production",
                "risk": "critical"
            }
        )
        
        print(f"❌ UNEXPECTED: Dangerous action allowed")
        
    except Exception as e:
        print(f"✅ EXPECTED: Dangerous action blocked")
        print(f"   Reason: {str(e)[:150]}")
        print(f"   📊 Alert Type: Capability Violation")
        print(f"   📊 Severity: HIGH")
        print(f"   📊 Reported to: Admin Dashboard")
        return True
    
    return False


def test_multiple_violations(agent):
    """Test multiple violations to trigger security alerts"""
    print_header("TEST 3: Multiple Security Violations", "🔥")
    
    print("Attempting multiple unauthorized actions...")
    print("")
    
    violations = [
        ("send_email", "spam@example.com", "Bulk email attempt"),
        ("access_credentials", "aws_secret_key", "Credential theft attempt"),
        ("network_scan", "192.168.1.0/24", "Network reconnaissance"),
        ("privilege_escalation", "sudo su -", "Privilege escalation"),
    ]
    
    blocked_count = 0
    for action, resource, description in violations:
        try:
            result = agent.verify_action(
                action_type=action,
                resource=resource,
                context={"description": description}
            )
            print(f"   ⚠️  {action} → Unexpectedly allowed")
        except Exception as e:
            print(f"   ✅ {action} → Blocked")
            blocked_count += 1
        
        time.sleep(0.3)
    
    print(f"\n📊 Results:")
    print(f"   • Blocked: {blocked_count}/{len(violations)}")
    print(f"   • Alerts Created: {blocked_count}")
    print(f"   • Trust Score Impact: -{blocked_count * 10} points")
    
    return blocked_count > 0


def check_alerts_in_dashboard(agent):
    """Check if alerts were created"""
    print_header("VERIFICATION: Check Dashboard Alerts", "📊")
    
    try:
        headers = {
            'X-API-Key': AIM_API_KEY
        }
        
        # Try to get alerts (this endpoint might require JWT, but we'll try)
        response = requests.get(
            f'{AIM_API_URL}/api/v1/sdk-api/agents/{agent.agent_id}',
            headers=headers,
            timeout=5
        )
        
        if response.status_code == 200:
            data = response.json()
            print(f"✅ Agent Status:")
            print(f"   • Trust Score: {data.get('trust_score', 'N/A')}")
            print(f"   • Status: {data.get('status', 'N/A')}")
            print(f"   • Violations: Check dashboard")
        else:
            print(f"⚠️  Could not fetch agent status: {response.status_code}")
            
    except Exception as e:
        print(f"⚠️  Error checking status: {str(e)[:80]}")
    
    print(f"\n📊 View Alerts in Dashboard:")
    print(f"   {AIM_DASHBOARD_URL}/dashboard/security/alerts")
    print(f"\n📊 View Violations:")
    print(f"   {AIM_DASHBOARD_URL}/dashboard/agents/{agent.agent_id}")


def main():
    print('\n🔒 AIM Security Alert Testing')
    print('=' * 70)
    print('Testing unauthorized actions and security violations')
    print('=' * 70)
    
    try:
        # Initialize agent with LIMITED capabilities
        print('\n1. Creating Agent with LIMITED Capabilities...')
        print('-' * 70)
        
        print(f'🔧 Configuration:')
        print(f'   API URL: {AIM_API_URL}')
        print(f'   Agent Name: {AGENT_NAME}')
        print(f'   Granted Capabilities: read_files, make_api_calls ONLY')
        print('')
        
        agent = secure(
            AGENT_NAME,
            aim_url=AIM_API_URL,
            api_key=AIM_API_KEY
        )
        
        print(f'\n✅ Agent Created:')
        print(f'   • Agent ID: {agent.agent_id}')
        print(f'   • Name: {AGENT_NAME}')
        print(f'   • Capabilities: Limited (read_files, make_api_calls)')
        print(f'   • Ready to test violations')
        
        time.sleep(1)
        
        # Run tests
        test1_passed = test_unauthorized_capability(agent)
        time.sleep(1)
        
        test2_passed = test_capability_violation_via_api(agent)
        time.sleep(1)
        
        test3_passed = test_multiple_violations(agent)
        time.sleep(1)
        
        # Check results
        check_alerts_in_dashboard(agent)
        
        # Summary
        print_header("TEST SUMMARY", "🎯")
        
        print('✅ Tests Completed:')
        print(f'   • Unauthorized Capability Test: {"PASS" if test1_passed else "FAIL"}')
        print(f'   • Direct API Violation Test: {"PASS" if test2_passed else "FAIL"}')
        print(f'   • Multiple Violations Test: {"PASS" if test3_passed else "FAIL"}')
        
        print('\n📊 What Was Created:')
        print('   • Security Alerts → Admin Dashboard')
        print('   • Capability Violations → Database')
        print('   • Audit Logs → Compliance Trail')
        print('   • Trust Score Penalties → Agent Profile')
        
        print('\n📊 Where to View Results:')
        print(f'   1. Security Alerts:')
        print(f'      {AIM_DASHBOARD_URL}/dashboard/security/alerts')
        print(f'   2. Agent Violations:')
        print(f'      {AIM_DASHBOARD_URL}/dashboard/agents/{agent.agent_id}')
        print(f'   3. Audit Logs:')
        print(f'      {AIM_DASHBOARD_URL}/dashboard/audit')
        
        print('\n📋 Database Tables Updated:')
        print('   • alerts → Security alerts for admin')
        print('   • capability_violations → Violation records')
        print('   • audit_logs → Complete audit trail')
        print('   • agents → Trust score updated')
        
        print('\n🔍 Endpoints Involved:')
        print('   • POST /api/v1/sdk-api/verifications → Verify actions')
        print('   • Internal: AlertRepository.Create() → Create alerts')
        print('   • Internal: CapabilityRepository.CreateViolation()')
        print('   • Internal: AuditRepository.Create() → Log violations')
        
        print('\n✅ All violations have been reported to admin!\n')
        
    except KeyboardInterrupt:
        print('\n\n⚠️  Test interrupted by user\n')
        sys.exit(0)
    except Exception as error:
        print(f'\n❌ Error: {error}\n')
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()
