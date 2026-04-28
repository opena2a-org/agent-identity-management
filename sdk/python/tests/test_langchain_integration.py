#!/usr/bin/env python3
"""
Integration tests for AIM + LangChain

Tests all three integration patterns:
1. AIMCallbackHandler - Automatic logging
2. @aim_verify decorator - Explicit verification
3. AIMToolWrapper - Wrap existing tools

Requires: langchain-core installed + live AIM backend at AIM_URL
Run with: pytest -m integration tests/test_langchain_integration.py
"""

import sys
import os
from pathlib import Path

import pytest

# Skip entire module if langchain_core is not installed
langchain_core = pytest.importorskip("langchain_core", reason="langchain-core not installed")

from langchain_core.tools import tool
from langchain_core.callbacks import BaseCallbackHandler

from aim_sdk import AIMClient
from aim_sdk.integrations.langchain import AIMCallbackHandler, aim_verify, wrap_tools_with_aim

AIM_URL = "http://localhost:8080"


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


@pytest.mark.integration
def test_callback_handler():
    """Test 1: AIMCallbackHandler - Automatic logging"""
    # Register AIM agent
    aim_client = AIMClient.auto_register_or_load(
        "langchain-test-callback",
        AIM_URL
    )

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
        try:
            result = _safe_arith(expression)
            return f"Result: {result}"
        except Exception as e:
            return f"Error: {e}"

    # Simulate tool execution with callback
    serialized = {"name": simple_calculator.name}
    input_str = "2 + 2"
    run_id = "test-run-001"

    aim_handler.on_tool_start(
        serialized=serialized,
        input_str=input_str,
        run_id=run_id
    )

    result = simple_calculator.invoke(input_str)

    aim_handler.on_tool_end(
        output=result,
        run_id=run_id
    )


@pytest.mark.integration
def test_aim_verify_decorator():
    """Test 2: @aim_verify decorator - Explicit verification"""
    # Register AIM agent
    aim_client = AIMClient.auto_register_or_load(
        "langchain-test-decorator",
        AIM_URL
    )

    # Define tool with @aim_verify decorator
    @tool
    @aim_verify(agent=aim_client, risk_level="medium")
    def database_query(query: str) -> str:
        '''Execute a database query'''
        return f"Query executed: {query}"

    # Execute tool (AIM verification happens automatically)
    try:
        result = database_query.invoke("SELECT * FROM users")
    except PermissionError:
        # Expected if AIM denies the action
        pass


@pytest.mark.integration
def test_tool_wrapper():
    """Test 3: AIMToolWrapper - Wrap existing tools"""
    # Register AIM agent
    aim_client = AIMClient.auto_register_or_load(
        "langchain-test-wrapper",
        AIM_URL
    )

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

    # Execute wrapped tools
    try:
        calc_result = verified_tools[0].invoke("10 * 5")
        reverse_result = verified_tools[1].invoke("Hello AIM!")
    except PermissionError:
        pass


@pytest.mark.integration
def test_graceful_degradation():
    """Test 4: Graceful degradation when AIM not configured"""
    # Define tool without AIM agent (should work with warning)
    @tool
    @aim_verify()  # No agent specified, will try to auto-load "langchain-agent"
    def simple_tool(input: str) -> str:
        '''A simple tool'''
        return f"Processed: {input}"

    # Execute (should run with warning if no agent found)
    try:
        result = simple_tool.invoke("test")
    except Exception:
        pass


if __name__ == "__main__":
    sys.exit(pytest.main([__file__, "-v", "-m", "integration"]))
