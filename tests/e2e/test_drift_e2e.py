#!/usr/bin/env python3
"""
E2E test for drift approval workflow
Creates test agent and verification event with drift
"""

import requests
import json

# Configuration
BASE_URL = "http://localhost:8080/api/v1"
TOKEN = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJlbWFpbCI6ImFkbWluQGFpbS50ZXN0IiwiZXhwIjoxNzYwMDIyNzA5LCJpYXQiOjE3NTk5MzYzMDksIm9yZ19pZCI6IjExMTExMTExLTExMTEtMTExMS0xMTExLTExMTExMTExMTExMSIsInJvbGUiOiJhZG1pbiIsInN1YiI6IjIyMjIyMjIyLTIyMjItMjIyMi0yMjIyLTIyMjIyMjIyMjIyMiJ9.Cz6yxgcGjJP-GriMKW50y2h8_Njpp35tPMoOlt_mJb4"

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
    exit(1)

# Step 2: Create verification event with drift (includes unauthorized MCP server)
print(f"\n2. Creating verification event with drift")
print(f"   Current runtime servers: ['filesystem-mcp', 'github-mcp', 'external-api-mcp']")
print(f"   Registered servers: ['filesystem-mcp', 'github-mcp']")
print(f"   Drift detected: 'external-api-mcp' is not registered!")

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
if response.status_code == 201:
    event = response.json()
    print(f"✅ Verification event created: {event['id']}")
else:
    print(f"❌ Failed to create verification event: {response.status_code}")
    print(f"   Response: {response.text}")
    exit(1)

# Step 3: Check if drift alert was created
print(f"\n3. Checking for drift alert...")
response = requests.get(f"{BASE_URL}/admin/alerts?limit=10", headers=headers)
if response.status_code == 200:
    alerts = response.json()
    drift_alerts = [a for a in alerts if a.get("alert_type") == "configuration_drift" and a.get("resource_id") == agent_id]

    if drift_alerts:
        alert = drift_alerts[0]
        print(f"✅ Drift alert created!")
        print(f"   Alert ID: {alert['id']}")
        print(f"   Severity: {alert['severity']}")
        print(f"   Title: {alert['title']}")
        print(f"   Description preview: {alert['description'][:200]}...")
        print(f"\n" + "=" * 80)
        print("SUCCESS: Test data created successfully!")
        print("=" * 80)
        print(f"\nNext steps:")
        print(f"1. Navigate to http://localhost:3000/dashboard/admin/alerts")
        print(f"2. Find the drift alert for agent '{agent['name']}'")
        print(f"3. Click 'Approve Drift' button")
        print(f"4. Verify agent registration updated with 'external-api-mcp'")
    else:
        print(f"⚠️  No drift alert found yet (might still be processing)")
        print(f"   Total alerts: {len(alerts)}")
else:
    print(f"❌ Failed to fetch alerts: {response.status_code}")
    print(f"   Response: {response.text}")
