#!/usr/bin/env python3
"""
Comprehensive AIM SDK Test Script
Tests: Capability Requests, Different Risk Levels, MCP Registration, Violations
"""

import os
import sys
import time
import json

# Add the SDK to the path
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

from aim_sdk import secure
import random
import string

# Generate unique suffix for this test run
TEST_SUFFIX = ''.join(random.choices(string.ascii_lowercase + string.digits, k=6))

def test_decorator_risk_levels():
    """Test perform_action decorator with different risk levels"""
    print("\n" + "="*60)
    print("TESTING: SDK Decorators with Different Risk Levels")
    print("="*60)

    # Initialize agent with unique name for this test run
    agent = secure(f"risk-tester-{TEST_SUFFIX}")
    print(f"✓ Agent created: {agent.agent_id}")

    # Test LOW risk action
    print("\n--- Testing LOW risk action ---")
    @agent.perform_action(capability="data:read", resource="public-data", risk_level="low")
    def read_public_data():
        return {"data": "public info", "risk": "low"}

    try:
        result = read_public_data()
        print(f"✓ LOW risk action result: {result}")
    except Exception as e:
        print(f"✗ LOW risk action blocked: {e}")

    # Test MEDIUM risk action
    print("\n--- Testing MEDIUM risk action ---")
    @agent.perform_action(capability="data:write", resource="internal-database", risk_level="medium")
    def write_internal_data():
        return {"written": True, "risk": "medium"}

    try:
        result = write_internal_data()
        print(f"✓ MEDIUM risk action result: {result}")
    except Exception as e:
        print(f"✗ MEDIUM risk action blocked: {e}")

    # Test HIGH risk action
    print("\n--- Testing HIGH risk action ---")
    @agent.perform_action(capability="admin:modify", resource="user-accounts", risk_level="high")
    def modify_user_accounts():
        return {"modified": True, "risk": "high"}

    try:
        result = modify_user_accounts()
        print(f"✓ HIGH risk action result: {result}")
    except Exception as e:
        print(f"✗ HIGH risk action blocked: {e}")

    # Test CRITICAL risk action (without JIT)
    print("\n--- Testing CRITICAL risk action (no JIT) ---")
    @agent.perform_action(capability="system:delete", resource="production-database", risk_level="critical")
    def delete_production_data():
        return {"deleted": True, "risk": "critical"}

    try:
        result = delete_production_data()
        print(f"✓ CRITICAL risk action result: {result}")
    except Exception as e:
        print(f"✗ CRITICAL risk action blocked: {e}")

    # Test CRITICAL risk action WITH JIT
    print("\n--- Testing CRITICAL risk action WITH JIT Access ---")
    @agent.perform_action(
        capability="system:execute",
        resource="production-server",
        risk_level="critical",
        jit_access=True,
        timeout_seconds=5
    )
    def execute_production_command():
        return {"executed": True, "risk": "critical", "jit": True}

    try:
        result = execute_production_command()
        print(f"✓ CRITICAL+JIT action result: {result}")
    except Exception as e:
        print(f"✗ CRITICAL+JIT action blocked/timeout: {e}")

    return agent

def test_capability_requests():
    """Test capability request flow"""
    print("\n" + "="*60)
    print("TESTING: Capability Requests")
    print("="*60)

    agent = secure(f"cap-requester-{TEST_SUFFIX}")
    print(f"✓ Agent created: {agent.agent_id}")

    # Request various capabilities
    capabilities_to_request = [
        ("api:call", "Need to call external APIs for data enrichment"),
        ("data:read", "Need to read customer data for analysis"),
        ("data:write", "Need to update records in the database"),
        ("admin:access", "Need admin access for maintenance tasks"),
    ]

    for cap_type, reason in capabilities_to_request:
        print(f"\n--- Requesting capability: {cap_type} ---")
        try:
            result = agent.request_capability(capability_type=cap_type, reason=reason)
            print(f"✓ Request result: {json.dumps(result, indent=2)}")
        except Exception as e:
            print(f"✗ Request failed: {e}")
            # Check if there's a response body with more details
            if hasattr(e, 'response'):
                try:
                    print(f"  Response: {e.response.text}")
                except:
                    pass

    return agent

def test_mcp_registration():
    """Test MCP server registration"""
    print("\n" + "="*60)
    print("TESTING: MCP Server Registration")
    print("="*60)

    agent = secure(f"mcp-registrar-{TEST_SUFFIX}")
    print(f"✓ Agent created: {agent.agent_id}")

    # Try to register an MCP server connection
    print("\n--- Registering MCP Server Connection ---")

    # register_mcp expects: mcp_server_id, detection_method, confidence, metadata
    # It registers that this agent "talks to" an existing MCP server
    if hasattr(agent, 'register_mcp'):
        try:
            # First, we need an existing MCP server ID. Let's use a placeholder for testing.
            # In real usage, this would be an existing MCP server in the system.
            result = agent.register_mcp(
                mcp_server_id="test-mcp-server",  # Would need to be a valid MCP ID
                detection_method="manual",
                confidence=100.0,
                metadata={"test": True, "purpose": "SDK validation"}
            )
            print(f"✓ MCP Registration result: {json.dumps(result, indent=2)}")
        except Exception as e:
            print(f"✗ MCP Registration failed: {e}")
            # This is expected if the MCP server doesn't exist in the system
    else:
        print("✗ register_mcp method not found on agent")
        print(f"  Available methods: {[m for m in dir(agent) if not m.startswith('_')]}")

    return agent

def test_action_verification():
    """Test action verification without decorator"""
    print("\n" + "="*60)
    print("TESTING: Direct Action Verification (verify_capability)")
    print("="*60)

    agent = secure(f"action-verifier-{TEST_SUFFIX}")
    print(f"✓ Agent created: {agent.agent_id}")

    # Use verify_capability (verify_action is deprecated)
    # verify_capability(capability, resource, risk_level, context, timeout_seconds)
    if hasattr(agent, 'verify_capability'):
        actions_to_verify = [
            {"capability": "api:call", "resource": "external-api", "risk_level": "low"},
            {"capability": "data:read", "resource": "user-database", "risk_level": "medium"},
            {"capability": "system:execute", "resource": "production", "risk_level": "critical"},
        ]

        for action in actions_to_verify:
            print(f"\n--- Verifying capability: {action['capability']} on {action['resource']} ---")
            try:
                result = agent.verify_capability(**action)
                print(f"✓ Verification result: {json.dumps(result, indent=2)}")
            except Exception as e:
                print(f"✗ Verification failed: {e}")
    else:
        print("✗ verify_capability method not found on agent")
        print(f"  Available methods: {[m for m in dir(agent) if not m.startswith('_')]}")

    return agent

def test_trust_score():
    """Test trust score retrieval"""
    print("\n" + "="*60)
    print("TESTING: Trust Score")
    print("="*60)

    agent = secure(f"trust-checker-{TEST_SUFFIX}")
    print(f"✓ Agent created: {agent.agent_id}")

    # Check trust score
    if hasattr(agent, 'get_trust_score'):
        try:
            score = agent.get_trust_score()
            print(f"✓ Trust Score: {score}")
        except Exception as e:
            print(f"✗ Trust Score retrieval failed: {e}")
    elif hasattr(agent, 'trust_score'):
        print(f"✓ Trust Score (property): {agent.trust_score}")
    else:
        print("✗ No trust score method/property found")
        print(f"  Available: {[m for m in dir(agent) if not m.startswith('_')]}")

    return agent

def main():
    """Run all SDK tests"""
    print("="*60)
    print("AIM SDK COMPREHENSIVE TEST SUITE")
    print("="*60)
    print(f"Time: {time.strftime('%Y-%m-%d %H:%M:%S')}")

    # Run all tests
    tests = [
        ("Decorator Risk Levels", test_decorator_risk_levels),
        ("Capability Requests", test_capability_requests),
        ("MCP Registration", test_mcp_registration),
        ("Action Verification", test_action_verification),
        ("Trust Score", test_trust_score),
    ]

    results = []
    for test_name, test_func in tests:
        print(f"\n\n{'#'*60}")
        print(f"# Running: {test_name}")
        print(f"{'#'*60}")
        try:
            test_func()
            results.append((test_name, "COMPLETED"))
        except Exception as e:
            print(f"\n✗ Test failed with error: {e}")
            results.append((test_name, f"FAILED: {e}"))

    # Print summary
    print("\n\n" + "="*60)
    print("TEST SUMMARY")
    print("="*60)
    for test_name, status in results:
        print(f"  {test_name}: {status}")
    print("="*60)

if __name__ == "__main__":
    main()
