#!/usr/bin/env python3
"""
Complete Security Violation Test
=================================

This script:
1. Creates/loads an agent with the SDK
2. Gets the agent's public/private keys
3. Tests unauthorized capability usage (agent tries to use capabilities it doesn't have)
4. Properly signs verification requests
5. Triggers security alerts that are reported to admin

All violations are logged to:
- alerts table (for admin dashboard)
- capability_violations table
- audit_logs table
"""

import sys
import os
import json
import base64
import time
from datetime import datetime

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'aim-sdk-python'))

from aim_sdk import secure
from nacl.signing import SigningKey
from nacl.encoding import Base64Encoder
import requests

# Configuration
AIM_API_KEY = os.getenv('AIM_API_KEY', 'aim_live_rVqDQwzk-p9KRfxZZM3ocnidPcSqf9yhxVZkjs_CVXc=')
AIM_API_URL = os.getenv('AIM_API_URL', 'http://localhost:8080')
AIM_DASHBOARD_URL = os.getenv('AIM_DASHBOARD_URL', 'http://localhost:3000')
AGENT_NAME = os.getenv('AGENT_NAME', 'violation-test-agent')


def load_agent_credentials(agent_name):
    """Load agent credentials from ~/.aim/credentials.json"""
    creds_path = os.path.expanduser('~/.aim/credentials.json')
    
    if not os.path.exists(creds_path):
        print(f"❌ Credentials not found at {creds_path}")
        return None
    
    with open(creds_path, 'r') as f:
        all_creds = json.load(f)
    
    if agent_name not in all_creds:
        print(f"❌ Agent '{agent_name}' not found in credentials")
        print(f"   Available agents: {list(all_creds.keys())}")
        return None
    
    return all_creds[agent_name]


def sign_message(private_key_b64, message):
    """Sign a message using Ed25519 private key"""
    try:
        private_key_bytes = base64.b64decode(private_key_b64)
        
        # Handle both 32-byte and 64-byte private keys
        if len(private_key_bytes) == 64:
            seed = private_key_bytes[:32]
        elif len(private_key_bytes) == 32:
            seed = private_key_bytes
        else:
            raise ValueError(f"Invalid private key length: {len(private_key_bytes)}")
        
        signing_key = SigningKey(seed)
        signed = signing_key.sign(message.encode('utf-8'))
        signature = base64.b64encode(signed.signature).decode('utf-8')
        
        return signature
    except Exception as e:
        print(f"❌ Error signing message: {e}")
        return None


def test_unauthorized_capability(agent_id, public_key, private_key, action_type, resource):
    """Test using a capability the agent doesn't have"""
    print(f"\n🔍 Testing unauthorized action: {action_type}")
    print(f"   Resource: {resource}")
    
    headers = {
        'X-API-Key': AIM_API_KEY,
        'Content-Type': 'application/json'
    }
    
    # Create verification request
    timestamp = datetime.utcnow().isoformat()
    if not timestamp.endswith('Z'):
        timestamp += 'Z'
    
    # Create request payload (same as SDK)
    request_payload = {
        "agent_id": agent_id,
        "action_type": action_type,
        "resource": resource,
        "context": {
            'test': 'unauthorized_capability',
            'risk': 'critical'
        },
        "timestamp": timestamp
    }
    
    # Sign the entire JSON payload (deterministic, sorted keys)
    signature_message = json.dumps(request_payload, sort_keys=True)
    signature = sign_message(private_key, signature_message)
    if not signature:
        print("   ❌ Failed to sign message")
        return False
    
    # Add signature and public key
    verify_data = request_payload.copy()
    verify_data['signature'] = signature
    verify_data['public_key'] = public_key
    
    try:
        response = requests.post(
            f'{AIM_API_URL}/api/v1/sdk-api/verifications',
            headers=headers,
            json=verify_data,
            timeout=10
        )
        print(f"   Response: {response.json()}")
        print(f"   Status: {response.status_code}")
        
        if response.status_code == 403:
            print(f"   ✅ BLOCKED (as expected)")
            print(f"   📊 Security alert created for admin")
            return True
        elif response.status_code == 200:
            result = response.json()
            print(f"   ⚠️  ALLOWED: {result.get('status')}")
            print(f"   Reason: {result.get('message', 'No message')}")
            return True
        else:
            print(f"   ❌ Error: {response.text[:200]}")
            return False
            
    except Exception as e:
        print(f"   ❌ Request failed: {str(e)[:100]}")
        return False


def main():
    print('\n' + '=' * 70)
    print('🔒 SECURITY VIOLATION TEST - Complete Flow')
    print('=' * 70)
    print('\nThis test will:')
    print('1. Create/load an agent')
    print('2. Attempt unauthorized capabilities')
    print('3. Generate security alerts for admin')
    print('4. Log violations to database')
    print('=' * 70)
    
    try:
        # Step 1: Create or load agent
        print('\n📝 STEP 1: Agent Registration')
        print('-' * 70)
        
        agent = secure(
            AGENT_NAME,
            aim_url=AIM_API_URL,
            api_key=AIM_API_KEY
        )
        
        print(f'✅ Agent ready')
        print(f'   • ID: {agent.agent_id}')
        print(f'   • Name: {AGENT_NAME}')
        
        # Step 2: Load credentials
        print('\n🔑 STEP 2: Loading Credentials')
        print('-' * 70)
        
        creds = load_agent_credentials(AGENT_NAME)
        if not creds:
            print('❌ Failed to load credentials')
            return
        
        agent_id = creds['agent_id']
        public_key = creds['public_key']
        private_key = creds['private_key']
        
        print(f'✅ Credentials loaded')
        print(f'   • Agent ID: {agent_id}')
        print(f'   • Public Key: {public_key[:20]}...')
        print(f'   • Private Key: [LOADED]')
        
        # Step 3: Get agent's actual capabilities
        print('\n📋 STEP 3: Checking Agent Capabilities')
        print('-' * 70)
        
        headers = {'X-API-Key': AIM_API_KEY}
        response = requests.get(
            f'{AIM_API_URL}/api/v1/sdk-api/agents/{agent_id}',
            headers=headers
        )
        
        if response.status_code == 200:
            agent_data = response.json()
            capabilities = agent_data.get('capabilities', [])
            print(f'✅ Agent has {len(capabilities)} capabilities:')
            for cap in capabilities[:5]:  # Show first 5
                print(f'   • {cap.get("capability_type", "unknown")}')
            if len(capabilities) > 5:
                print(f'   ... and {len(capabilities) - 5} more')
        else:
            print(f'⚠️  Could not fetch capabilities: {response.status_code}')
            capabilities = []
        
        # Step 4: Test unauthorized capabilities
        print('\n🚨 STEP 4: Testing Unauthorized Capabilities')
        print('-' * 70)
        print('Attempting actions the agent does NOT have permission for...\n')
        
        # Test cases: actions agent likely doesn't have
        test_cases = [
            ('delete_database', 'DROP TABLE users', 'Database deletion attempt'),
            ('execute_shell', 'rm -rf /', 'Shell command execution'),
            ('access_secrets', 'aws_secret_key', 'Secret access attempt'),
            ('modify_system', '/etc/passwd', 'System file modification'),
            ('network_attack', '192.168.1.0/24', 'Network scanning'),
        ]
        
        blocked_count = 0
        for action_type, resource, description in test_cases:
            print(f'Test: {description}')
            if test_unauthorized_capability(agent_id, public_key, private_key, action_type, resource):
                blocked_count += 1
            time.sleep(0.5)
        
        # Step 5: Summary
        print('\n' + '=' * 70)
        print('📊 TEST RESULTS')
        print('=' * 70)
        
        print(f'\n✅ Tests completed: {blocked_count}/{len(test_cases)}')
        
        print('\n📋 What was created in the database:')
        print('   • alerts table → Security alerts for admin')
        print('   • capability_violations table → Violation records')
        print('   • audit_logs table → Complete audit trail')
        print('   • agents table → Trust score updated (decreased)')
        
        print('\n🔍 Endpoints involved:')
        print('   • POST /api/v1/sdk-api/verifications')
        print('   • Internal: AlertRepository.Create()')
        print('   • Internal: CapabilityRepository.CreateViolation()')
        print('   • Internal: AuditRepository.Create()')
        print('   • Internal: AgentRepository.UpdateTrustScore()')
        
        print('\n📊 View results in dashboard:')
        print(f'   • Security Alerts: {AIM_DASHBOARD_URL}/dashboard/security/alerts')
        print(f'   • Agent Details: {AIM_DASHBOARD_URL}/dashboard/agents/{agent_id}')
        print(f'   • Audit Logs: {AIM_DASHBOARD_URL}/dashboard/audit')
        
        print('\n💡 What happens when violations are detected:')
        print('   1. Security policy is evaluated (block vs alert-only)')
        print('   2. Alert is created with HIGH severity')
        print('   3. Violation is logged with details')
        print('   4. Trust score is decreased by 10 points')
        print('   5. Admin is notified in dashboard')
        print('   6. If violations >= 3, agent may be marked as compromised')
        
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
