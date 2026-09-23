"""
Regression tests for camelCase-to-snake_case key mapping in API-key registration.

Bug: The server returns camelCase keys (agentId, publicKey, aimUrl) but
_register_via_api_key() expected snake_case keys (agent_id, public_key,
aim_url), causing KeyError on registration.

The OAuth path already had this mapping. These tests ensure the API-key path
handles both camelCase and snake_case server responses correctly.

The API-key path registers through POST /api/v1/agents, which stores the
public key the SDK generates and sends, and never returns a private key. The
fixtures answer as that route does: any public key in the response template
is replaced by the one the request carried, and the saved private key is the
locally generated one, whatever the server says.
"""

import base64
import json
import pytest
import responses
from unittest.mock import patch

from aim_sdk.client import _register_via_api_key
from aim_sdk import AIMClient


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"

CAMEL_CASE_RESPONSE = {
    "agentId": AGENT_ID,
    "publicKey": "<echoed>",
    "aimUrl": "https://aim.example.com",
    "trustScore": 0.85,
    "displayName": "Test Agent",
}

SNAKE_CASE_RESPONSE = {
    "agent_id": AGENT_ID,
    "public_key": "<echoed>",
    "aim_url": "https://aim.example.com",
    "trust_score": 0.85,
    "display_name": "Test Agent",
}

MIXED_CASE_RESPONSE = {
    "agentId": AGENT_ID,
    "public_key": "<echoed>",
    "aim_url": "https://aim.example.com",
    "trustScore": 0.85,
    "display_name": "Test Agent",
}

ID_ONLY_RESPONSE = {
    "id": AGENT_ID,
    "publicKey": "<echoed>",
}

AIM_URL = "https://aim.example.com"
API_KEY = "aim_live_test_not_real"
AGENT_NAME = "test-agent"
REGISTER_URL = f"{AIM_URL}/api/v1/agents"

_ECHO_KEYS = ("publicKey", "public_key")


def _serve(template, status=201):
    """Register the agents route: the template, with every public-key field
    that reads "<echoed>" replaced by the public key the request sent."""
    def callback(request):
        sent = json.loads(request.body)["publicKey"]
        body = {k: (sent if k in _ECHO_KEYS and v == "<echoed>" else v) for k, v in template.items()}
        return status, {"Content-Type": "application/json"}, json.dumps(body)
    responses.add_callback(responses.POST, REGISTER_URL, callback=callback)


def _register():
    return _register_via_api_key(
        name=AGENT_NAME,
        aim_url=AIM_URL,
        api_key=API_KEY,
        registration_data={"name": AGENT_NAME, "capabilities": []},
        sdk_token_id=None,
        mcp_server_names=[],
        mcp_full_definitions=[],
    )


def _sent_public_key():
    return json.loads(responses.calls[0].request.body)["publicKey"]


def _assert_local_private_key(saved):
    assert len(saved["private_key"]) == 88
    assert len(base64.b64decode(saved["private_key"])) == 64
    assert saved["private_key"] not in responses.calls[0].response.text


# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

class TestApiKeyCamelCaseMapping:
    """Verify _register_via_api_key maps camelCase server responses to snake_case."""

    @responses.activate
    @patch("aim_sdk.client._save_credentials")
    @patch("aim_sdk.client._print_registration_success")
    @patch("aim_sdk.client.security_logger")
    def test_camel_case_response_maps_to_snake_case(self, mock_logger, mock_print, mock_save):
        """Server returns all camelCase keys -- SDK must map them before use."""
        _serve(CAMEL_CASE_RESPONSE)

        client = _register()

        assert isinstance(client, AIMClient)
        assert client.agent_id == AGENT_ID

        saved = mock_save.call_args[0][1]
        assert saved["agent_id"] == AGENT_ID
        assert saved["public_key"] == _sent_public_key()
        _assert_local_private_key(saved)
        assert saved["aim_url"] == AIM_URL
        assert saved["trust_score"] == 0.85
        assert saved["display_name"] == "Test Agent"

    @responses.activate
    @patch("aim_sdk.client._save_credentials")
    @patch("aim_sdk.client._print_registration_success")
    @patch("aim_sdk.client.security_logger")
    def test_snake_case_response_passes_through(self, mock_logger, mock_print, mock_save):
        """Server returns snake_case keys -- SDK must not break them."""
        _serve(SNAKE_CASE_RESPONSE)

        client = _register()

        assert isinstance(client, AIMClient)
        assert client.agent_id == AGENT_ID

        saved = mock_save.call_args[0][1]
        assert saved["agent_id"] == AGENT_ID
        assert saved["public_key"] == _sent_public_key()
        _assert_local_private_key(saved)
        assert saved["aim_url"] == AIM_URL

    @responses.activate
    @patch("aim_sdk.client._save_credentials")
    @patch("aim_sdk.client._print_registration_success")
    @patch("aim_sdk.client.security_logger")
    def test_mixed_case_response_maps_correctly(self, mock_logger, mock_print, mock_save):
        """Server returns a mix -- each key must resolve to its snake_case form."""
        _serve(MIXED_CASE_RESPONSE)

        client = _register()

        assert isinstance(client, AIMClient)
        saved = mock_save.call_args[0][1]
        assert saved["agent_id"] == AGENT_ID
        assert saved["public_key"] == _sent_public_key()
        _assert_local_private_key(saved)
        assert saved["aim_url"] == AIM_URL
        assert saved["trust_score"] == 0.85
        assert saved["display_name"] == "Test Agent"

    @responses.activate
    @patch("aim_sdk.client._save_credentials")
    @patch("aim_sdk.client._print_registration_success")
    @patch("aim_sdk.client.security_logger")
    def test_id_field_maps_to_agent_id(self, mock_logger, mock_print, mock_save):
        """The agents route answers with 'id' rather than 'agentId'; the SDK
        must map it, and fill the aim_url the caller registered against."""
        _serve(ID_ONLY_RESPONSE)

        client = _register()

        assert isinstance(client, AIMClient)
        assert client.agent_id == AGENT_ID

        saved = mock_save.call_args[0][1]
        assert saved["agent_id"] == AGENT_ID
        assert saved["public_key"] == _sent_public_key()
        _assert_local_private_key(saved)
        assert saved["aim_url"] == AIM_URL

    @responses.activate
    @patch("aim_sdk.client._save_credentials")
    @patch("aim_sdk.client._print_registration_success")
    @patch("aim_sdk.client.security_logger")
    def test_snake_case_not_overwritten_by_camel(self, mock_logger, mock_print, mock_save):
        """When both forms are present, the snake_case value is honoured, and a
        private key in the response never replaces the local one."""
        _serve({
            "agentId": "camel-id",
            "agent_id": AGENT_ID,
            "publicKey": "camel-pub",
            "public_key": "<echoed>",
            "privateKey": "camel-priv-not-a-key",
        })

        client = _register()

        assert client.agent_id == AGENT_ID
        saved = mock_save.call_args[0][1]
        assert saved["agent_id"] == AGENT_ID
        assert saved["public_key"] == _sent_public_key()
        assert saved["private_key"] != "camel-priv-not-a-key"
        _assert_local_private_key(saved)
