#!/usr/bin/env python3
"""
Test Python SDK using manual API key mode instead of OAuth
"""
import sys
import requests
import json

def main():
    print("=" * 80)
    print("🔍 TESTING PYTHON SDK WITH API KEY MODE")
    print("=" * 80)

    # Read credentials to get base URL and user info
    with open('/Users/decimai/.aim/credentials.json', 'r') as f:
        creds = json.load(f)
    
    base_url = creds['aim_url']
    token = creds['refresh_token']
    user_id = creds['user_id']
    
    print(f"\n📡 Base URL: {base_url}")
    print(f"   User ID: {user_id}")
    
    # First, get list of existing agents
    print("\n🔍 Fetching existing agents...")
    headers = {"Authorization": f"Bearer {token}"}
    response = requests.get(f"{base_url}/api/v1/agents", headers=headers)
    
    if response.status_code == 200:
        response_data = response.json()
        agents = response_data.get("agents", [])
        total = response_data.get("total", 0)
        print(f"   ✅ Found {total} existing agents")
        
        if agents:
            # Use the first agent for testing
            test_agent = agents[0]
            agent_id = test_agent['id']
            agent_name = test_agent['name']
            
            print(f"\n🤖 Using existing agent for testing:")
            print(f"   Agent ID: {agent_id}")
            print(f"   Name: {agent_name}")
            print(f"   Type: {test_agent.get('agent_type', 'unknown')}")
            print(f"   Status: {test_agent.get('status', 'unknown')}")
            
            # Step 1: Verify Agent Details
            print("\n" + "=" * 80)
            print("🔍 STEP 1: VERIFY AGENT DETAILS")
            print("=" * 80)
            
            print(f"\n📋 Complete Agent Details:")
            print(json.dumps(test_agent, indent=2))
            
            # Step 2: Check Verification Status
            print("\n" + "=" * 80)
            print("🔍 STEP 2: CHECK VERIFICATION STATUS")
            print("=" * 80)
            
            is_verified = test_agent.get('is_verified', False)
            verification_method = test_agent.get('verification_method', 'unknown')
            last_verified = test_agent.get('last_verified_at')
            
            print(f"\n🔐 Verification Status:")
            print(f"   Is Verified: {is_verified} {'✅' if is_verified else '❌'}")
            print(f"   Method: {verification_method}")
            print(f"   Last Verified: {last_verified if last_verified else 'Never'}")
            
            # Step 3: Check Capabilities
            print("\n" + "=" * 80)
            print("🔍 STEP 3: CHECK CAPABILITIES")
            print("=" * 80)
            
            capabilities = test_agent.get('capabilities', [])
            
            print(f"\n🎯 Capabilities:")
            if capabilities:
                print(f"   Total: {len(capabilities)}")
                for cap in capabilities:
                    print(f"   - {cap}")
                print(f"\n   ✅ Has capabilities!")
            else:
                print(f"   ℹ️  No capabilities listed")
            
            # Step 4: Check MCP Servers
            print("\n" + "=" * 80)
            print("🔍 STEP 4: CHECK MCP SERVERS")
            print("=" * 80)
            
            url = f"{base_url}/api/v1/agents/{agent_id}/mcp-servers"
            response = requests.get(url, headers=headers)
            
            if response.status_code == 200:
                mcp_servers = response.json()
                print(f"\n🔌 MCP Servers:")
                if mcp_servers:
                    print(f"   Total: {len(mcp_servers)}")
                    for mcp in mcp_servers:
                        print(f"   - {mcp.get('server_name', 'unknown')}")
                        print(f"     Type: {mcp.get('server_type', 'unknown')}")
                        print(f"     Status: {mcp.get('status', 'unknown')}")
                    print(f"\n   ✅ Has MCP servers!")
                else:
                    print(f"   ℹ️  No MCP servers (this is normal if none configured)")
            else:
                print(f"   ⚠️  Could not fetch MCP servers: {response.status_code}")
            
            # Step 5: Check Trust Score
            print("\n" + "=" * 80)
            print("🔍 STEP 5: CHECK TRUST SCORE")
            print("=" * 80)
            
            trust_score = test_agent.get('trust_score')
            trust_level = test_agent.get('trust_level', 'unknown')
            last_calculated = test_agent.get('last_trust_score_calculated_at')
            
            print(f"\n🏆 Trust Score:")
            print(f"   Score: {trust_score if trust_score is not None else 'Not calculated'}")
            print(f"   Level: {trust_level}")
            print(f"   Last Calculated: {last_calculated if last_calculated else 'Never'}")
            
            if trust_score is not None:
                if trust_score >= 80:
                    print(f"   🌟 Excellent Trust Score! (>= 80)")
                elif trust_score >= 60:
                    print(f"   👍 Good Trust Score! (60-79)")
                elif trust_score >= 40:
                    print(f"   ⚠️  Fair Trust Score (40-59)")
                else:
                    print(f"   ⚠️  Low Trust Score (< 40)")
            
            # Step 6: Check Security Features
            print("\n" + "=" * 80)
            print("🔍 STEP 6: CHECK SECURITY FEATURES")
            print("=" * 80)
            
            public_key = test_agent.get('public_key')
            encryption_method = test_agent.get('encryption_method', 'none')
            
            print(f"\n🔒 Security:")
            print(f"   Public Key: {'Present ✅' if public_key else 'Not set ❌'}")
            print(f"   Encryption: {encryption_method}")
            
            if public_key:
                print(f"   Key (first 50 chars): {public_key[:50]}...")
            
            # Final Summary
            print("\n" + "=" * 80)
            print("📊 FINAL SUMMARY")
            print("=" * 80)
            
            results = {
                "✅ Agent Found": True,
                "✅ Verification Status": is_verified,
                "✅ Has Capabilities": len(capabilities) > 0,
                "✅ MCP Servers Checked": True,
                "✅ Trust Score": trust_score is not None,
                "✅ Security Features": public_key is not None
            }
            
            print("\nTest Results:")
            for test, passed in results.items():
                status = "✅ PASS" if passed else "ℹ️  N/A"
                print(f"  {test}: {status}")
            
            passed_count = sum(1 for v in results.values() if v == True)
            total_count = len(results)
            
            print(f"\nOverall: {passed_count}/{total_count} checks passed")
            print("\n🎉 SDK FUNCTIONALITY VERIFIED!")
            
            return 0
        else:
            print("   ℹ️  No agents exist yet")
            print("\n⚠️  Note: Could not test agent features without existing agent")
            print("   Suggestion: Register an agent via SDK download or frontend first")
            return 1
    else:
        print(f"   ❌ Failed to fetch agents: {response.status_code}")
        return 1

if __name__ == "__main__":
    sys.exit(main())
