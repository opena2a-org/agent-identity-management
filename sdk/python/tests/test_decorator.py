#!/usr/bin/env python3
"""
Test the @aim_verify universal decorator

Demonstrates how developers can use @aim_verify on ANY Python function
to automatically verify actions with the AIM backend.

Requires: live AIM backend at AIM_URL
Run with: pytest -m integration tests/test_decorator.py
"""

import sys
import os
import time
from pathlib import Path

import pytest

from aim_sdk import AIMClient, aim_verify, aim_verify_database, aim_verify_api_call

AIM_URL = "http://localhost:8080"


@pytest.mark.integration
def test_decorator_with_explicit_client():
    """Test 1: Using decorator with explicit AIM client"""
    # Register/load agent
    aim_client = AIMClient.auto_register_or_load("decorator-test-agent", AIM_URL)

    # Define a function with AIM verification
    @aim_verify(aim_client, action_type="database_query", risk_level="high")
    def delete_user(user_id: str):
        """Simulates deleting a user from database"""
        return {"deleted": True, "user_id": user_id}

    # Call the function - verification happens automatically
    result = delete_user("user123")
    assert result["deleted"] is True
    assert result["user_id"] == "user123"


@pytest.mark.integration
def test_decorator_with_auto_init():
    """Test 2: Using decorator with auto-initialization from environment"""
    # Set environment variables
    os.environ["AIM_AGENT_NAME"] = "decorator-test-agent"
    os.environ["AIM_URL"] = AIM_URL
    os.environ["AIM_AUTO_REGISTER"] = "true"

    # Define function with auto-init (no explicit client needed)
    @aim_verify(auto_init=True, action_type="api_call", risk_level="medium")
    def call_external_api(endpoint: str):
        """Simulates calling an external API"""
        return {"status": "success", "endpoint": endpoint}

    # Call the function - client auto-initializes and verifies
    result = call_external_api("/users/profile")
    assert result["status"] == "success"


@pytest.mark.integration
def test_convenience_decorators():
    """Test 3: Using convenience decorators"""
    aim_client = AIMClient.auto_register_or_load("decorator-test-agent", AIM_URL)

    # Test database decorator
    @aim_verify_database(aim_client)
    def query_users():
        return [{"id": "1", "name": "Alice"}, {"id": "2", "name": "Bob"}]

    # Test API call decorator
    @aim_verify_api_call(aim_client)
    def fetch_weather(city: str):
        return {"city": city, "temp": 72, "condition": "sunny"}

    users = query_users()
    assert len(users) == 2

    weather = fetch_weather("San Francisco")
    assert weather["city"] == "San Francisco"


@pytest.mark.integration
def test_decorator_preserves_metadata():
    """Test 4: Verify decorator preserves function metadata"""
    aim_client = AIMClient.auto_register_or_load("decorator-test-agent", AIM_URL)

    @aim_verify(aim_client)
    def example_function(x: int, y: int) -> int:
        """Adds two numbers together"""
        return x + y

    # Check metadata
    assert example_function.__name__ == "example_function", "Function name not preserved"
    assert "Adds two numbers" in example_function.__doc__, "Docstring not preserved"

    # Test execution
    result = example_function(5, 3)
    assert result == 8


if __name__ == "__main__":
    sys.exit(pytest.main([__file__, "-v", "-m", "integration"]))
