"""
Regression tests for camelCase-to-snake_case key mapping in API-key registration.

Bug: The server returns camelCase keys (agentId, publicKey, privateKey, aimUrl)
but _register_via_api_key() expected snake_case keys (agent_id, public_key,
private_key, aim_url), causing KeyError on registration.

The OAuth path already had this mapping. This test ensures the API-key path
handles both camelCase and snake_case server responses correctly.
"""

import json
import pytest
import responses
from unittest.mock import patch, MagicMock

from aim_sdk.client import _register_via_api_key
from aim_sdk import AIMClient


# ---------------------------------------------------------------------------
# Fixtures -- valid Ed25519 test keys (32-byte seed / 32-byte public)
# ---------------------------------------------------------------------------

_TEST_PRIVATE_KEY = "XPyQB140i2D1raqyqUor39eSfxWxh7IrcA2IwoGRd+k="
_TEST_PUBLIC_KEY = "9tMOAsBHPU1Nvw32IYOA1s+DXOK4zkmNdxbJSnJ9sGs="

CAMEL_CASE_RESPONSE = {
    "agentId": "550e8400-e29b-41d4-a716-446655440000",
    "publicKey": _TEST_PUBLIC_KEY,
    "privateKey": _TEST_PRIVATE_KEY,
    "aimUrl": "https://aim.example.com",
    "trustScore": 0.85,
    "displayName": "Test Agent",
}

SNAKE_CASE_RESPONSE = {
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "public_key": _TEST_PUBLIC_KEY,
    "private_key": _TEST_PRIVATE_KEY,
    "aim_url": "https://aim.example.com",
    "trust_score": 0.85,
    "display_name": "Test Agent",
}

MIXED_CASE_RESPONSE = {
    "agentId": "550e8400-e29b-41d4-a716-446655440000",
    "public_key": _TEST_PUBLIC_KEY,
    "privateKey": _TEST_PRIVATE_KEY,
    "aim_url": "https://aim.example.com",
    "trustScore": 0.85,
    "display_name": "Test Agent",
}

ID_ONLY_RESPONSE = {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "publicKey": _TEST_PUBLIC_KEY,
    "privateKey": _TEST_PRIVATE_KEY,
}

AIM_URL = "https://aim.example.com"
API_KEY = "aim_test_key_12345"
AGENT_NAME = "test-agent"
REGISTER_URL = f"{AIM_URL}/api/v1/public/agents/register"


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

class TestApiKeyCamelCaseMapping:
    """Verify _register_via_api_key maps camelCase server responses to snake_case."""

    @responses.activate
    @patch("aim_sdk.client._save_credentials")
    @patch("aim_sdk.client._print_registration_success")
    @patch("aim_sdk.client.security_logger")
    def test_camel_case_response_maps_to_snake_case(
        self, mock_logger, mock_print, mock_save
    ):
        """Server returns all camelCase keys -- SDK must map them before use."""
        responses.add(
            responses.POST,
            REGISTER_URL,
            json=CAMEL_CASE_RESPONSE,
            status=201,
        )

        client = _register_via_api_key(
            name=AGENT_NAME,
            aim_url=AIM_URL,
            api_key=API_KEY,
            registration_data={"name": AGENT_NAME, "capabilities": []},
            sdk_token_id=None,
            mcp_server_names=[],
            mcp_full_definitions=[],
        )

        assert isinstance(client, AIMClient)
        assert client.agent_id == CAMEL_CASE_RESPONSE["agentId"]

        # Verify credentials saved with snake_case keys
        saved = mock_save.call_args[0][1]
        assert saved["agent_id"] == CAMEL_CASE_RESPONSE["agentId"]
        assert saved["public_key"] == CAMEL_CASE_RESPONSE["publicKey"]
        assert saved["private_key"] == CAMEL_CASE_RESPONSE["privateKey"]
        assert saved["aim_url"] == AIM_URL
        assert saved["trust_score"] == 0.85
        assert saved["display_name"] == "Test Agent"

    @responses.activate
    @patch("aim_sdk.client._save_credentials")
    @patch("aim_sdk.client._print_registration_success")
    @patch("aim_sdk.client.security_logger")
    def test_snake_case_response_passes_through(
        self, mock_logger, mock_print, mock_save
    ):
        """Server returns snake_case keys -- no mapping needed, must still work."""
        responses.add(
            responses.POST,
            REGISTER_URL,
            json=SNAKE_CASE_RESPONSE,
            status=201,
        )

        client = _register_via_api_key(
            name=AGENT_NAME,
            aim_url=AIM_URL,
            api_key=API_KEY,
            registration_data={"name": AGENT_NAME, "capabilities": []},
            sdk_token_id=None,
            mcp_server_names=[],
            mcp_full_definitions=[],
        )

        assert isinstance(client, AIMClient)
        assert client.agent_id == SNAKE_CASE_RESPONSE["agent_id"]

    @responses.activate
    @patch("aim_sdk.client._save_credentials")
    @patch("aim_sdk.client._print_registration_success")
    @patch("aim_sdk.client.security_logger")
    def test_mixed_case_response_maps_correctly(
        self, mock_logger, mock_print, mock_save
    ):
        """Server returns a mix of camelCase and snake_case -- both must resolve."""
        responses.add(
            responses.POST,
            REGISTER_URL,
            json=MIXED_CASE_RESPONSE,
            status=201,
        )

        client = _register_via_api_key(
            name=AGENT_NAME,
            aim_url=AIM_URL,
            api_key=API_KEY,
            registration_data={"name": AGENT_NAME, "capabilities": []},
            sdk_token_id=None,
            mcp_server_names=[],
            mcp_full_definitions=[],
        )

        assert isinstance(client, AIMClient)

        saved = mock_save.call_args[0][1]
        assert saved["agent_id"] == MIXED_CASE_RESPONSE["agentId"]
        assert saved["public_key"] == MIXED_CASE_RESPONSE["public_key"]
        assert saved["private_key"] == MIXED_CASE_RESPONSE["privateKey"]
        assert saved["aim_url"] == MIXED_CASE_RESPONSE["aim_url"]

    @responses.activate
    @patch("aim_sdk.client._save_credentials")
    @patch("aim_sdk.client._print_registration_success")
    @patch("aim_sdk.client.security_logger")
    def test_id_field_maps_to_agent_id(
        self, mock_logger, mock_print, mock_save
    ):
        """Server returns 'id' instead of 'agentId' -- must map to agent_id."""
        responses.add(
            responses.POST,
            REGISTER_URL,
            json=ID_ONLY_RESPONSE,
            status=201,
        )

        client = _register_via_api_key(
            name=AGENT_NAME,
            aim_url=AIM_URL,
            api_key=API_KEY,
            registration_data={"name": AGENT_NAME, "capabilities": []},
            sdk_token_id=None,
            mcp_server_names=[],
            mcp_full_definitions=[],
        )

        assert isinstance(client, AIMClient)
        assert client.agent_id == ID_ONLY_RESPONSE["id"]

        saved = mock_save.call_args[0][1]
        assert saved["agent_id"] == ID_ONLY_RESPONSE["id"]
        # aim_url should be injected from parameter since server didn't return it
        assert saved["aim_url"] == AIM_URL

    @responses.activate
    @patch("aim_sdk.client._save_credentials")
    @patch("aim_sdk.client._print_registration_success")
    @patch("aim_sdk.client.security_logger")
    def test_snake_case_not_overwritten_by_camel(
        self, mock_logger, mock_print, mock_save
    ):
        """If both agentId and agent_id exist, snake_case value is preserved."""
        both_keys = {
            "agentId": "camel-id",
            "agent_id": "snake-id",
            "publicKey": "camel-pub",
            "public_key": _TEST_PUBLIC_KEY,
            "privateKey": "camel-priv",
            "private_key": _TEST_PRIVATE_KEY,
            "aimUrl": "https://camel.example.com",
            "aim_url": "https://snake.example.com",
        }
        responses.add(
            responses.POST,
            REGISTER_URL,
            json=both_keys,
            status=201,
        )

        client = _register_via_api_key(
            name=AGENT_NAME,
            aim_url=AIM_URL,
            api_key=API_KEY,
            registration_data={"name": AGENT_NAME, "capabilities": []},
            sdk_token_id=None,
            mcp_server_names=[],
            mcp_full_definitions=[],
        )

        # snake_case values should be preserved, not overwritten by camelCase
        saved = mock_save.call_args[0][1]
        assert saved["agent_id"] == "snake-id"
        assert saved["public_key"] == _TEST_PUBLIC_KEY
        assert saved["private_key"] == _TEST_PRIVATE_KEY
        assert saved["aim_url"] == "https://snake.example.com"
