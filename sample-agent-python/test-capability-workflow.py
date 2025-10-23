#!/usr/bin/env python3
"""
Capability Request Workflow Test
=================================

This script demonstrates the complete capability request workflow:
1. Agent registers with initial capabilities
2. Agent tries to perform an action without the required capability (triggers alert)
3. Agent requests the missing capability via SDK
4. Admin approves the request in the dashboard
5. Agent can now perform the action

This demonstrates how AIM prevents capability violations and provides a
secure approval workflow for capability expansion.
"""

import sys
import os
import requests
from datetime import datetime

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim-sdk-python'))

from aim_sdk import secure, AIMClient

# Configuration
AIM_API_KEY = os.getenv('AIM_API_KEY', 'aim_live_rVqDQwzk-p9KRfxZZM3ocnidPcSqf9yhxVZkjs_CVXc=')
AIM_API_URL = os.getenv('AIM_API_URL', 'http://localhost:8080')
AGENT_NAME = f'capability-workflow-test-{datetime.now().strftime("%Y%m%d%H%M%S")}'


def main():
    print('\n🔐 Capability Request Workflow Demonstration')
    print('=' * 70)
    
    try:
        # Step 1: Register agent with limited capabilities
        print('\n📝 Step 1: Registering agent with initial capabilities')
        print('-' * 70)
        
        agent = secure(
            AGENT_NAME,
            aim_url=AIM_API_URL,
            api_key=AIM_API_KEY
        )
        
        print(f'✅ Agent registered: {agent.agent_id}')
        print(f'   Name: {AGENT_NAME}')
        print(f'   Initial capabilities: read_files, send_email, make_api_calls, execute_code, write_files')
        
        # Step 2: Try to perform an action without the required capability
        print('\n🚫 Step 2: Attempting unauthorized action (write_database)')
        print('-' * 70)
        print('   This should trigger a security alert...')
        
        headers = {
            'X-API-Key': AIM_API_KEY,
            'Content-Type': 'application/json'
        }
        
        # Try to verify an action that requires write_database capability
        verify_payload = {
            'agent_id': agent.agent_id,
            'action_type': 'write_database',
            'resource': 'users_table',
            'context': {'operation': 'UPDATE'},
            'timestamp': datetime.utcnow().isoformat() + 'Z'
        }
        
        response = requests.post(
            f'{AIM_API_URL}/api/v1/sdk-api/verifications',
            headers=headers,
            json=verify_payload
        )
        
        if response.status_code == 403:
            print('   ✅ Action BLOCKED (as expected - no capability)')
            print('   🚨 Security alert should be created in dashboard')
        elif response.status_code == 201:
            result = response.json()
            print(f'   ⚠️  Action ALLOWED: {result.get("status")}')
            print('   (This might be due to security policy in alert-only mode)')
        else:
            print(f'   ⚠️  Unexpected response: {response.status_code}')
            print(f'   {response.text[:200]}')
        
        # Step 3: Request the missing capability via SDK
        print('\n📋 Step 3: Requesting missing capability via SDK')
        print('-' * 70)
        
        # Create AIM client for SDK methods
        client = AIMClient(
            agent_id=agent.agent_id,
            aim_url=AIM_API_URL,
            api_key=AIM_API_KEY
        )
        
        # Request the capability
        capability_request = client.request_capability(
            capability_type='write_database',
            reason='Need to update user records in the database for analytics and reporting'
        )
        
        print(f'   ✅ Capability request created!')
        print(f'   Request ID: {capability_request["id"]}')
        print(f'   Status: {capability_request["status"]}')
        print(f'   Capability: {capability_request["capability_type"]}')
        
        # Step 4: Instructions for admin
        print('\n👤 Step 4: Admin Approval Required')
        print('-' * 70)
        print('   📊 View and approve the request in the dashboard:')
        print(f'   http://localhost:3000/dashboard/admin/capability-requests')
        print('')
        print('   After approval, the agent will be able to perform write_database actions.')
        print('')
        print('   🔍 You can also check the security alerts:')
        print('   http://localhost:3000/dashboard/admin/alerts')
        print('')
        
        # Step 5: Summary
        print('\n📊 Workflow Summary')
        print('=' * 70)
        print(f'✅ Agent ID: {agent.agent_id}')
        print(f'✅ Agent Name: {AGENT_NAME}')
        print(f'✅ Capability Request ID: {capability_request["id"]}')
        print(f'✅ Requested Capability: write_database')
        print(f'✅ Status: {capability_request["status"]}')
        print('')
        print('🔐 Security Features Demonstrated:')
        print('   ✓ Capability-based access control (CBAC)')
        print('   ✓ Automatic security alert on violation')
        print('   ✓ SDK-based capability request workflow')
        print('   ✓ Admin approval required for capability expansion')
        print('   ✓ Full audit trail of all actions')
        print('')
        
    except Exception as error:
        print(f'\n❌ Error: {error}\n')
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()

