#!/usr/bin/env python3
"""
Integration tests for AIM + LangChain

Tests all three integration patterns:
1. AIMCallbackHandler - Automatic logging
2. @aim_verify decorator - Explicit verification
3. AIMToolWrapper - Wrap existing tools

Each test asserts, through the backend's API (GET
/api/v1/verification-events/agent/{id}), that the backend recorded one
verification event, with the agent's id and the verified action, for each
framework call the test makes. No test calls a model provider: tools,
callbacks and wrappers are invoked through LangChain's own tool runtime.

Requires: the langchain extra (pip install -e ".[dev,mcp,langchain]") and a
live AIM backend at AIM_URL with the admin AIM_TEST_ADMIN_EMAIL /
AIM_TEST_ADMIN_PASSWORD. scripts/test-sdk-python-framework-integration.sh starts
one and runs these tests.
Run with: pytest -m integration tests/test_langchain_integration.py
"""

import os
import sys
import uuid

import pytest
import requests

# Skip entire module if langchain_core is not installed
langchain_core = pytest.importorskip("langchain_core", reason="langchain-core not installed")

from langchain_core.tools import tool

from aim_sdk import AIMClient
from aim_sdk.integrations.langchain import AIMCallbackHandler, aim_verify, wrap_tools_with_aim

AIM_URL = os.environ.get("AIM_URL", "http://localhost:8080")


def _safe_arith(expression: str):
    """AST-walk evaluator for the arithmetic test fixtures.

    Replaces the unsafe Python builtin (the one that takes a string and
    runs it as code) so the test file itself cannot trigger code execution
    if a future contributor pastes attacker-controlled strings into the
    invoke() calls below.
    """
    import ast
    import operator as _op
    _OPS = {
        ast.Add: _op.add, ast.Sub: _op.sub,
        ast.Mult: _op.mul, ast.Div: _op.truediv,
        ast.USub: _op.neg, ast.UAdd: _op.pos,
    }

    def _walk(node):
        if isinstance(node, ast.Expression):
            return _walk(node.body)
        if isinstance(node, ast.Constant) and isinstance(node.value, (int, float)):
            return node.value
        if isinstance(node, ast.BinOp) and type(node.op) in _OPS:
            return _OPS[type(node.op)](_walk(node.left), _walk(node.right))
        if isinstance(node, ast.UnaryOp) and type(node.op) in _OPS:
            return _OPS[type(node.op)](_walk(node.operand))
        raise ValueError("unsupported expression")

    return _walk(ast.parse(expression, mode='eval'))


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


def _register(prefix: str, admin: _Admin) -> AIMClient:
    """Register a new agent through the SDK's API-key mode.

    The key is the one the backend returns when the admin creates an agent.
    auto_hooks=False keeps the count to the adapter under test: the default
    hooks add an AIMCallbackHandler to every LangChain callback manager.
    """
    suffix = uuid.uuid4().hex[:8]
    response = requests.post(
        f"{AIM_URL}/api/v1/agents",
        headers=admin.headers,
        json={"name": f"{prefix}-key-{suffix}", "displayName": f"{prefix} key", "agentType": "ai_agent"},
        timeout=10,
    )
    assert response.status_code == 201, f"creating the key agent answered {response.status_code}"
    return AIMClient.auto_register_or_load(
        f"{prefix}-{suffix}",
        AIM_URL,
        api_key=response.json()["apiKey"]["key"],
        auto_hooks=False,
    )


def _recorded_actions(admin: _Admin, agent_id: str) -> list:
    """The action of every verification event the backend holds for the agent."""
    response = requests.get(
        f"{AIM_URL}/api/v1/verification-events/agent/{agent_id}",
        headers=admin.headers,
        params={"limit": 100},
        timeout=10,
    )
    assert response.status_code == 200, f"reading verification events answered {response.status_code}"
    events = response.json().get("events") or []
    assert [event.get("agentId") for event in events] == [agent_id] * len(events)
    return sorted(event.get("action") for event in events)


def _organization_event_total(admin: _Admin) -> int:
    response = requests.get(
        f"{AIM_URL}/api/v1/verification-events",
        headers=admin.headers,
        params={"limit": 1},
        timeout=10,
    )
    assert response.status_code == 200, f"reading verification events answered {response.status_code}"
    return response.json()["total"]


@pytest.mark.integration
def test_callback_handler(admin):
    """Test 1: AIMCallbackHandler - Automatic logging"""
    aim_client = _register("langchain-test-callback", admin)

    # Create callback handler
    aim_handler = AIMCallbackHandler(
        agent=aim_client,
        log_inputs=True,
        log_outputs=True,
        verbose=True
    )

    # Define a simple tool
    @tool
    def simple_calculator(expression: str) -> str:
        '''Calculate a mathematical expression'''
        return f"Result: {_safe_arith(expression)}"

    # LangChain's tool runtime calls on_tool_start and on_tool_end.
    result = simple_calculator.invoke("2 + 2", config={"callbacks": [aim_handler]})

    assert result == "Result: 4"
    assert _recorded_actions(admin, aim_client.agent_id) == ["langchain_tool:simple_calculator"]


@pytest.mark.integration
def test_callback_handler_failed_tool(admin):
    """Test 1b: AIMCallbackHandler records a tool that raises"""
    aim_client = _register("langchain-test-callback-error", admin)
    aim_handler = AIMCallbackHandler(agent=aim_client)

    @tool
    def failing_lookup(key: str) -> str:
        '''Look up a key that does not exist'''
        raise KeyError(key)

    # LangChain's tool runtime calls on_tool_start and on_tool_error.
    with pytest.raises(KeyError):
        failing_lookup.invoke("missing", config={"callbacks": [aim_handler]})

    assert _recorded_actions(admin, aim_client.agent_id) == ["langchain_tool:failing_lookup"]


@pytest.mark.integration
def test_aim_verify_decorator(admin):
    """Test 2: @aim_verify decorator - Explicit verification"""
    aim_client = _register("langchain-test-decorator", admin)

    # Define tool with @aim_verify decorator
    @tool
    @aim_verify(agent=aim_client, risk_level="medium")
    def database_query(query: str) -> str:
        '''Execute a database query'''
        return f"Query executed: {query}"

    # A tool of two arguments, which LangChain calls with keyword arguments
    @tool
    @aim_verify(agent=aim_client, risk_level="medium")
    def table_lookup(table: str, key: str) -> str:
        '''Look up a key in a table'''
        return f"{table}[{key}]"

    # Execute tools (AIM verification happens automatically)
    result = database_query.invoke("SELECT * FROM users")

    assert result == "Query executed: SELECT * FROM users"
    assert table_lookup.invoke({"table": "users", "key": "42"}) == "users[42]"
    assert _recorded_actions(admin, aim_client.agent_id) == [
        "langchain_tool:database_query",
        "langchain_tool:table_lookup",
    ]


@pytest.mark.integration
def test_tool_wrapper(admin):
    """Test 3: AIMToolWrapper - Wrap existing tools"""
    aim_client = _register("langchain-test-wrapper", admin)

    # Define tools (without AIM verification)
    @tool
    def calculator(expression: str) -> str:
        '''Calculate mathematical expressions'''
        return f"Result: {_safe_arith(expression)}"

    @tool
    def string_reverser(text: str) -> str:
        '''Reverse a string'''
        return text[::-1]

    # Wrap ALL tools with AIM verification
    verified_tools = wrap_tools_with_aim(
        tools=[calculator, string_reverser],
        aim_agent=aim_client,
        default_risk_level="low"
    )

    assert len(verified_tools) == 2

    assert verified_tools[0].invoke("10 * 5") == "Result: 50"
    assert verified_tools[1].invoke("Hello AIM!") == "!MIA olleH"
    assert _recorded_actions(admin, aim_client.agent_id) == [
        "langchain_tool:calculator",
        "langchain_tool:string_reverser",
    ]


@pytest.mark.integration
def test_graceful_degradation(admin):
    """Test 4: Graceful degradation when AIM not configured"""
    before = _organization_event_total(admin)

    # Define tool without AIM agent (runs with a warning)
    @tool
    @aim_verify()  # No agent specified, will try to auto-load "langchain-agent"
    def simple_tool(input: str) -> str:
        '''A simple tool'''
        return f"Processed: {input}"

    # No stored "langchain-agent" exists, so the tool runs unverified and
    # the backend records nothing.
    assert simple_tool.invoke("test") == "Processed: test"
    assert _organization_event_total(admin) == before


if __name__ == "__main__":
    sys.exit(pytest.main([__file__, "-v", "-m", "integration"]))
