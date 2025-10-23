#!/usr/bin/env python3
"""
Create Capability Request Test
================================

This script creates capability requests for testing the admin approval workflow.
"""

import sys
import os
import requests
from datetime import datetime

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim-sdk-python'))

from aim_sdk import secure

# Configuration
AIM_API_KEY = os.getenv('AIM_API_KEY', 'aim_live_rVqDQwzk-p9KRfxZZM3ocnidPcSqf9yhxVZkjs_CVXc=')
AIM_API_URL = os.getenv('AIM_API_URL', 'http://localhost:8080')
AGENT_NAME = os.getenv('AGENT_NAME', f'test-capability-request-{datetime.now().strftime("%Y%m%d%H%M%S")}')


def main():
    print('\n🔧 Creating Capability Request Test')
    print('=' * 70)
    
    try:
        # Step 1: Create/load agent
        print('\n1. Creating agent...')
        agent = secure(
            AGENT_NAME,
            aim_url=AIM_API_URL,
            api_key=AIM_API_KEY
        )
        
        print(f'✅ Agent created: {agent.agent_id}')
        
        # Step 2: Create capability requests
        print('\n2. Creating capability requests...')
        
        headers = {
            'X-API-Key': AIM_API_KEY,
            'Content-Type': 'application/json'
        }
        
        # Request 1: Database access
        request1 = {
            'capability_type': 'write_database',
            'reason': 'Need to update user records in the database for analytics'
        }
        
        response = requests.post(
            f'{AIM_API_URL}/api/v1/sdk-api/agents/{agent.agent_id}/capability-requests',
            headers=headers,
            json=request1
        )
        
        if response.status_code in [200, 201]:
            result = response.json()
            print(f'   ✅ Request 1 created: write_database (ID: {result.get("id", "N/A")})')
        else:
            print(f'   ⚠️  Request 1 failed: {response.status_code} - {response.text[:200]}')
        
        # Request 2: Email sending
        request2 = {
            'capability_type': 'send_bulk_email',
            'reason': 'Need to send marketing emails to customers and partners'
        }
        
        response = requests.post(
            f'{AIM_API_URL}/api/v1/sdk-api/agents/{agent.agent_id}/capability-requests',
            headers=headers,
            json=request2
        )
        
        if response.status_code in [200, 201]:
            result = response.json()
            print(f'   ✅ Request 2 created: send_bulk_email (ID: {result.get("id", "N/A")})')
        else:
            print(f'   ⚠️  Request 2 failed: {response.status_code} - {response.text[:200]}')
        
        # Request 3: System access
        request3 = {
            'capability_type': 'execute_system_commands',
            'reason': 'Need to run system maintenance scripts for server health'
        }
        
        response = requests.post(
            f'{AIM_API_URL}/api/v1/sdk-api/agents/{agent.agent_id}/capability-requests',
            headers=headers,
            json=request3
        )
        
        if response.status_code in [200, 201]:
            result = response.json()
            print(f'   ✅ Request 3 created: execute_system_commands (ID: {result.get("id", "N/A")})')
        else:
            print(f'   ⚠️  Request 3 failed: {response.status_code} - {response.text[:200]}')
        
        print('\n✅ Capability requests created!')
        print('\n📊 View in dashboard:')
        print('   http://localhost:3000/dashboard/admin/capability-requests')
        print('\n💡 You can now approve or reject these requests in the admin panel.')
        print('')
        
    except Exception as error:
        print(f'\n❌ Error: {error}\n')
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == '__main__':
    main()

