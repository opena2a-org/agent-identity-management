#!/usr/bin/env python3
"""
Integration tests for AIM + MCP (Model Context Protocol)

Tests the MCP integration against a live backend:
1. register_mcp_server - an agent registers an MCP server
2. list_mcp_servers - the agent lists its MCP servers
3. verify_mcp_action - the agent verifies an MCP tool call
4. MCPActionWrapper - a tool call verified and executed in one step

The verification tests assert, through the backend's API (GET
/api/v1/verification-events/agent/{id}), that the backend recorded one
verification event, with the agent's id and the verified action, for each
call the test makes.

Requires: a live AIM backend at AIM_URL with the admin AIM_TEST_ADMIN_EMAIL /
AIM_TEST_ADMIN_PASSWORD. scripts/test-sdk-python-framework-integration.sh starts
one and runs these tests.
Run with: pytest -m integration tests/test_mcp_integration.py
"""

import base64
import os
import sys
import uuid
from collections import Counter

import pytest
import requests
from nacl.signing import SigningKey

from aim_sdk import AIMClient
from aim_sdk.integrations.mcp import (
    register_mcp_server,
    list_mcp_servers,
    verify_mcp_action,
)
from aim_sdk.integrations.mcp.verification import MCPActionWrapper

AIM_URL = os.environ.get("AIM_URL", "http://localhost:8080")


class _Admin:
    """The seeded admin's session. Its repr never shows the access token."""

    def __init__(self, access_token: str):
        self.headers = {"Authorization": f"Bearer {access_token}"}

    def __repr__(self) -> str:
        return "<admin session>"


@pytest.fixture(scope="module")
def admin():
    """The admin the backend was seeded with, signed in."""
    password = os.environ.get("AIM_TEST_ADMIN_PASSWORD")
    if not password:
        pytest.fail("AIM_TEST_ADMIN_PASSWORD is not set; scripts/test-sdk-python-framework-integration.sh sets it")
    response = requests.post(
        f"{AIM_URL}/api/v1/auth/login/local",
        json={"email": os.environ.get("AIM_TEST_ADMIN_EMAIL", "admin@opena2a.org"), "password": password},
        timeout=10,
    )
    assert response.status_code == 200, (
        f"admin sign-in answered {response.status_code}; DATABASE_URL must name a database this run bootstrapped"
    )
    return _Admin(response.json()["accessToken"])


@pytest.fixture(scope="module")
def aim_client(admin):
    """An agent registered through the SDK's API-key mode.

    The key is the one the backend returns when the admin creates an agent.
    """
    suffix = uuid.uuid4().hex[:8]
    response = requests.post(
        f"{AIM_URL}/api/v1/agents",
        headers=admin.headers,
        json={"name": f"mcp-test-key-{suffix}", "displayName": "mcp-test key", "agentType": "ai_agent"},
        timeout=10,
    )
    assert response.status_code == 201, f"creating the key agent answered {response.status_code}"
    return AIMClient.auto_register_or_load(
        f"mcp-test-registration-{suffix}",
        AIM_URL,
        api_key=response.json()["apiKey"]["key"],
        auto_hooks=False,
    )


@pytest.fixture(scope="module")
def mcp_server(aim_client):
    """The MCP server the agent registers; register_mcp_server's answer."""
    return register_mcp_server(
        aim_client=aim_client,
        server_name=f"test-mcp-server-{uuid.uuid4().hex[:8]}",
        server_url="http://localhost:3000",
        public_key=base64.b64encode(bytes(SigningKey.generate().verify_key)).decode(),
        capabilities=["tools", "resources", "prompts"],
        description="Test MCP server for integration testing",
        version="1.0.0"
    )


def _recorded_actions(admin: _Admin, agent_id: str) -> Counter:
    """How many verification events the backend holds for the agent, per action."""
    response = requests.get(
        f"{AIM_URL}/api/v1/verification-events/agent/{agent_id}",
        headers=admin.headers,
        params={"limit": 100},
        timeout=10,
    )
    assert response.status_code == 200, f"reading verification events answered {response.status_code}"
    events = response.json().get("events") or []
    assert [event.get("agentId") for event in events] == [agent_id] * len(events)
    return Counter(event.get("action") for event in events)


@pytest.mark.integration
def test_mcp_server_registration(admin, mcp_server):
    """Test 1: MCP Server Registration"""
    response = requests.get(
        f"{AIM_URL}/api/v1/mcp-servers/{mcp_server['id']}",
        headers=admin.headers,
        timeout=10,
    )

    assert response.status_code == 200, f"reading the registered server answered {response.status_code}"
    assert response.json()["name"] == mcp_server["name"]


@pytest.mark.integration
def test_mcp_server_listing(aim_client, mcp_server):
    """Test 2: MCP Server Listing"""
    servers = list_mcp_servers(aim_client, limit=10)

    assert mcp_server["id"] in [server["id"] for server in servers]


@pytest.mark.integration
def test_mcp_action_verification(admin, aim_client, mcp_server):
    """Test 3: MCP Action Verification"""
    before = _recorded_actions(admin, aim_client.agent_id)

    verify_mcp_action(
        aim_client=aim_client,
        mcp_server_id=mcp_server["id"],
        action_type="mcp_tool:web_search",
        resource="search query: AI safety best practices",
        context={
            "tool": "web_search",
            "params": {"q": "AI safety"},
            "framework": "mcp"
        },
        risk_level="low"
    )

    recorded = _recorded_actions(admin, aim_client.agent_id) - before
    assert recorded == Counter({"mcp_tool:web_search": 1})


@pytest.mark.integration
def test_mcp_action_wrapper(admin, aim_client, mcp_server):
    """Test 4: MCPActionWrapper"""
    before = _recorded_actions(admin, aim_client.agent_id)
    mcp_wrapper = MCPActionWrapper(
        aim_client=aim_client,
        mcp_server_id=mcp_server["id"],
        default_risk_level="low",
        verbose=True
    )

    def mock_web_search():
        """Mock MCP tool execution"""
        return {"results": ["result1", "result2", "result3"]}

    result = mcp_wrapper.execute_tool(
        tool_name="web_search",
        tool_function=mock_web_search,
        risk_level="low",
        context={"query": "test search"}
    )

    assert result == {"results": ["result1", "result2", "result3"]}
    recorded = _recorded_actions(admin, aim_client.agent_id) - before
    assert recorded == Counter({"mcp_tool:web_search": 1})


if __name__ == "__main__":
    sys.exit(pytest.main([__file__, "-v", "-m", "integration"]))
