#!/usr/bin/env python3
"""
Setup Agents via REST API

This script creates multiple agents and MCP servers directly via the API,
using a bearer token for authentication.

Usage:
    python setup_agents_via_api.py
"""

import os
import sys
import json
import time
import base64
import requests
from pathlib import Path
from datetime import datetime, timezone
from nacl.signing import SigningKey

# Configuration
AIM_URL = "http://localhost:8080"
AUTH_FILE = "/tmp/auth.json"


def load_auth_token():
    """Load the access token from auth.json."""
    try:
        with open(AUTH_FILE, 'r') as f:
            auth_data = json.load(f)
        return auth_data.get('accessToken')
    except Exception as e:
        print(f"Error loading auth token: {e}")
        return None


def make_request(method, endpoint, data=None, token=None):
    """Make an authenticated API request."""
    url = f"{AIM_URL}{endpoint}"
    headers = {
        'Content-Type': 'application/json',
    }
    if token:
        headers['Authorization'] = f'Bearer {token}'

    try:
        if method == 'GET':
            response = requests.get(url, headers=headers, timeout=30)
        elif method == 'POST':
            response = requests.post(url, headers=headers, json=data, timeout=30)
        elif method == 'DELETE':
            response = requests.delete(url, headers=headers, timeout=30)
        else:
            raise ValueError(f"Unsupported method: {method}")

        if response.status_code >= 400:
            print(f"  Error {response.status_code}: {response.text[:200]}")
            return None

        if response.status_code == 204:
            return {}

        return response.json()
    except Exception as e:
        print(f"  Request failed: {e}")
        return None


def generate_keypair():
    """Generate Ed25519 keypair for agent."""
    signing_key = SigningKey.generate()
    private_key_bytes = bytes(signing_key)
    public_key_bytes = signing_key.verify_key.encode()

    # For Go compatibility: 64-byte private key (seed + public key)
    private_key_full = private_key_bytes + public_key_bytes
    private_key_b64 = base64.b64encode(private_key_full).decode('utf-8')
    public_key_b64 = base64.b64encode(public_key_bytes).decode('utf-8')

    return public_key_b64, private_key_b64


def detect_mcps_from_config():
    """Detect MCPs from Claude Desktop config."""
    config_path = Path.home() / "Library" / "Application Support" / "Claude" / "claude_desktop_config.json"

    if not config_path.exists():
        print("Claude Desktop config not found")
        return []

    try:
        with open(config_path, 'r') as f:
            config = json.load(f)

        mcp_servers = config.get('mcpServers', {})
        detections = []

        for name, config_data in mcp_servers.items():
            command = config_data.get('command', '')
            args = config_data.get('args', [])
            mcp_url = f"{command} {' '.join(args)}" if args else command

            detections.append({
                'name': name,
                'command': command,
                'args': args,
                'url': mcp_url,
            })

        return detections
    except Exception as e:
        print(f"Error reading config: {e}")
        return []


def create_agent(token, agent_data):
    """Create a new agent via API."""
    public_key, private_key = generate_keypair()

    payload = {
        'name': agent_data['name'],
        'displayName': agent_data.get('display_name', agent_data['name']),
        'description': agent_data.get('description', f"Agent: {agent_data['name']}"),
        'agentType': agent_data.get('agent_type', 'ai_agent'),
        'version': agent_data.get('version', '1.0.0'),
        'publicKey': public_key,
        'capabilities': agent_data.get('capabilities', []),
        'talksTo': agent_data.get('mcp_servers', []),
    }

    if 'repository_url' in agent_data:
        payload['repositoryUrl'] = agent_data['repository_url']

    result = make_request('POST', '/api/v1/agents', payload, token)

    if result:
        result['private_key'] = private_key
        result['public_key'] = public_key

    return result


def create_mcp_server(token, mcp_data):
    """Create a new MCP server via API."""
    public_key, _ = generate_keypair()

    payload = {
        'name': mcp_data['name'],
        'description': mcp_data.get('description', f"MCP Server: {mcp_data['name']}"),
        'url': mcp_data.get('url', ''),
        'version': mcp_data.get('version', '1.0.0'),
        'publicKey': public_key,
        'capabilities': mcp_data.get('capabilities', ['tools']),
    }

    return make_request('POST', '/api/v1/mcp-servers', payload, token)


def create_agent_mcp_connection(token, agent_id, mcp_server_id, connection_type="attested"):
    """Create a connection between an agent and MCP server via SDK-API endpoint."""
    payload = {
        'mcp_server_id': mcp_server_id,
        'connection_type': connection_type,
        'tool_name': 'default',
        'mcp_url': '',
        'mcp_name': '',
    }

    # Use SDK-API endpoint
    return make_request('POST', f'/api/v1/sdk-api/agents/{agent_id}/mcp-connections', payload, token)


def create_security_event(token, agent_id, event_type, details):
    """Create a security event for an agent."""
    payload = {
        'agentId': agent_id,
        'eventType': event_type,
        'details': details,
        'timestamp': datetime.now(timezone.utc).isoformat(),
    }

    return make_request('POST', '/api/v1/security/events', payload, token)


def main():
    print("=" * 60)
    print("  AIM Setup via REST API")
    print("=" * 60)
    print()

    # Load auth token
    token = load_auth_token()
    if not token:
        print("Error: Could not load auth token from /tmp/auth.json")
        sys.exit(1)

    print(f"✓ Loaded auth token")
    print()

    # Detect MCPs from Claude config
    print("Detecting MCPs from Claude Desktop config...")
    mcp_detections = detect_mcps_from_config()
    print(f"✓ Found {len(mcp_detections)} MCP servers")
    print()

    # Define agents to create
    agents_to_create = [
        {
            'name': 'claude-code-agent',
            'display_name': 'Claude Code Agent',
            'description': 'Claude Code CLI instance for development',
            'agent_type': 'claude',
            'version': '1.0.0',
            'capabilities': ['file:read', 'file:write', 'code:execute', 'git:commit'],
            'mcp_servers': [d['name'] for d in mcp_detections],
        },
        {
            'name': 'langchain-data-processor',
            'display_name': 'LangChain Data Processor',
            'description': 'LangChain agent for processing and analyzing data',
            'agent_type': 'langchain',
            'version': '2.1.0',
            'capabilities': ['data:read', 'data:write', 'api:call', 'db:query'],
            'mcp_servers': ['github', 'filesystem', 'memory'],
        },
        {
            'name': 'crewai-research-team',
            'display_name': 'CrewAI Research Team',
            'description': 'Multi-agent crew for autonomous research tasks',
            'agent_type': 'crewai',
            'version': '1.5.0',
            'capabilities': ['web:scrape', 'file:read', 'api:call', 'data:analyze'],
            'mcp_servers': ['browserbase', 'github', 'aws-docs'],
        },
        {
            'name': 'github-copilot-assistant',
            'display_name': 'GitHub Copilot Assistant',
            'description': 'Code completion and suggestion assistant',
            'agent_type': 'copilot',
            'version': '1.0.0',
            'capabilities': ['code:suggest', 'code:complete', 'code:explain'],
            'mcp_servers': ['github', 'filesystem'],
        },
        {
            'name': 'autogpt-task-runner',
            'display_name': 'AutoGPT Task Runner',
            'description': 'Autonomous agent for complex multi-step tasks',
            'agent_type': 'autogpt',
            'version': '0.5.0',
            'capabilities': ['file:read', 'file:write', 'web:browse', 'code:execute', 'api:call'],
            'mcp_servers': ['filesystem', 'browserbase', 'docker-mcp'],
        },
    ]

    # Create agents
    print("Creating agents...")
    created_agents = []
    for agent_data in agents_to_create:
        print(f"  Creating {agent_data['display_name']}...")
        result = create_agent(token, agent_data)
        if result:
            print(f"    ✓ Created: {result.get('id', 'unknown')}")
            created_agents.append({
                'id': result.get('id'),
                'name': agent_data['name'],
                'display_name': agent_data['display_name'],
                'agent_type': agent_data['agent_type'],
                'public_key': result.get('public_key'),
                'private_key': result.get('private_key'),
                'mcp_servers': agent_data.get('mcp_servers', []),
            })
        else:
            print(f"    ✗ Failed")
    print()

    # Create MCP servers
    print("Creating MCP servers...")
    created_mcps = {}
    for mcp in mcp_detections[:10]:  # Limit to first 10 to avoid too many
        print(f"  Creating {mcp['name']}...")
        mcp_data = {
            'name': mcp['name'],
            'description': f"Auto-detected from Claude Desktop: {mcp['command']}",
            'url': mcp['url'],
            'version': '1.0.0',
            'capabilities': ['tools'],
        }
        result = create_mcp_server(token, mcp_data)
        if result:
            print(f"    ✓ Created: {result.get('id', 'unknown')}")
            created_mcps[mcp['name']] = result.get('id')
        else:
            print(f"    ✗ Failed")
    print()

    # Create agent-MCP connections
    print("Creating agent-MCP connections...")
    connection_count = 0
    for agent in created_agents:
        agent_id = agent['id']
        if not agent_id:
            continue

        for mcp_name in agent.get('mcp_servers', []):
            mcp_id = created_mcps.get(mcp_name)
            if not mcp_id:
                continue

            result = create_agent_mcp_connection(token, agent_id, mcp_id)
            if result:
                connection_count += 1

    print(f"✓ Created {connection_count} connections")
    print()

    # Summary
    print("=" * 60)
    print("  Setup Complete!")
    print("=" * 60)
    print()
    print(f"Created {len(created_agents)} agents:")
    for agent in created_agents:
        print(f"  • {agent['display_name']} ({agent['agent_type']})")
        print(f"    ID: {agent.get('id', 'N/A')}")

    print()
    print(f"Created {len(created_mcps)} MCP servers")
    print(f"Created {connection_count} agent-MCP connections")
    print()
    print(f"View agents: {AIM_URL}/dashboard/agents")
    print(f"View MCPs: {AIM_URL}/dashboard/mcp")
    print(f"View supply chain: {AIM_URL}/dashboard/mcp/supply-chain")


if __name__ == "__main__":
    main()
