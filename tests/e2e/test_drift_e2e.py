#!/usr/bin/env python3
"""
E2E check for the verification-events write route.

POST /verification-events no longer accepts caller-reported events: the server
records a verification event when it performs the verification. This script
creates a test agent, then checks that a caller-reported event carrying an
unregistered MCP server is refused, so it cannot seed a drift alert.

Set AIM_E2E_TOKEN to an admin JWT for the local backend before running.
"""

import os
import sys

import requests

# Configuration
BASE_URL = "http://localhost:8080/api/v1"
TOKEN = os.environ.get("AIM_E2E_TOKEN")
if not TOKEN:
    sys.exit("Set AIM_E2E_TOKEN to an admin JWT for the local backend.")

headers = {
    "Authorization": f"Bearer {TOKEN}",
    "Content-Type": "application/json"
}

print("=" * 80)
print("E2E Test: Drift Approval Workflow")
print("=" * 80)

# Step 1: Create test agent with specific talks_to configuration
print("\n1. Creating test agent with talks_to: ['filesystem-mcp', 'github-mcp']")
agent_data = {
    "name": "drift-test-agent",
    "display_name": "Drift Test Agent",
    "description": "Agent for testing configuration drift approval",
    "agent_type": "ai_agent",
    "version": "1.0.0",
    "capabilities": ["file_operations", "network_access"],
    "talks_to": ["filesystem-mcp", "github-mcp"],
    "rate_limit": 100,
    "status": "verified"
}

response = requests.post(f"{BASE_URL}/agents", headers=headers, json=agent_data)
if response.status_code == 201:
    agent = response.json()
    agent_id = agent["id"]
    print(f"✅ Agent created successfully: {agent_id}")
    print(f"   Name: {agent['name']}")
    print(f"   Talks To: {agent.get('talks_to', [])}")
else:
    print(f"❌ Failed to create agent: {response.status_code}")
    print(f"   Response: {response.text}")
    sys.exit(1)

# Step 2: a caller-reported verification event is refused
print("\n2. Posting a caller-reported verification event with an unregistered MCP server")
verification_data = {
    "agent_id": agent_id,
    "organization_id": "11111111-1111-1111-1111-111111111111",
    "protocol": "mcp",
    "verification_type": "identity",
    "status": "success",
    "confidence": 0.95,
    "current_mcp_servers": ["filesystem-mcp", "github-mcp", "external-api-mcp"],
    "current_capabilities": []
}

response = requests.post(f"{BASE_URL}/verification-events", headers=headers, json=verification_data)
body = response.json() if response.headers.get("Content-Type", "").startswith("application/json") else {}
if response.status_code == 403 and body.get("code") == "verificationEventWriteNotAccepted":
    print("   Refused with 403 verificationEventWriteNotAccepted, as expected.")
else:
    print(f"   Expected 403 verificationEventWriteNotAccepted, got {response.status_code}")
    print(f"   Response: {response.text}")
    sys.exit(1)

print("\n" + "=" * 80)
print("PASS: the write route refuses caller-reported events; no drift alert can be seeded through it.")
print("=" * 80)
