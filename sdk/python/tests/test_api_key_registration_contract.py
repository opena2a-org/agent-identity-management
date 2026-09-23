"""
The api-key registration contract, pinned from the SDK side.

Measured 2026-09-22 against a self-hosted stack: ``secure(name, aim_url=...,
api_key=...)`` posted to ``/api/v1/public/agents/register`` with the header
``X-AIM-API-Key``. That route reads only a JWT and no backend file reads that
header, so every registration failed with 400. The backend's API-key
registration route is ``POST /api/v1/agents`` (``X-API-Key``), which the
TypeScript SDK already uses; it never returns a private key, so the SDK
generates the agent's Ed25519 keypair locally and sends the public half.

These tests never read Go sources, so nothing here skips outside the monorepo;
the backend side of the pair is pinned by a Go test in ``apps/backend/cmd/server``.
"""

import base64
import glob
import json
import os
from unittest.mock import patch

import pytest
import responses

from aim_sdk import AIMClient
from aim_sdk.client import API_KEY_HEADER, _register_via_api_key
from aim_sdk.exceptions import AuthenticationError, ConfigurationError

AIM_URL = "http://aim.test"
API_KEY = "aim_live_test_not_real"
AGENT_NAME = "my-first-agent"
AGENTS_URL = f"{AIM_URL}/api/v1/agents"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"


def _echo_public_key(request):
    """Answer as the backend does: the stored public key is the one sent."""
    body = json.loads(request.body)
    return (
        201,
        {"Content-Type": "application/json"},
        json.dumps({
            "id": AGENT_ID,
            "name": body["name"],
            "publicKey": body["publicKey"],
            "status": "pending",
        }),
    )


def _register(**overrides):
    kwargs = dict(
        name=AGENT_NAME,
        aim_url=AIM_URL,
        api_key=API_KEY,
        registration_data={"name": AGENT_NAME, "capabilities": ["db:read"]},
        sdk_token_id=None,
        mcp_server_names=[],
        mcp_full_definitions=[],
    )
    kwargs.update(overrides)
    return _register_via_api_key(**kwargs)


@responses.activate
@patch("aim_sdk.client._save_credentials")
@patch("aim_sdk.client._print_registration_success")
@patch("aim_sdk.client.security_logger")
def test_register_via_api_key_posts_to_agents_route_with_x_api_key(mock_logger, mock_print, mock_save):
    responses.add_callback(responses.POST, AGENTS_URL, callback=_echo_public_key)

    client = _register()

    assert len(responses.calls) == 1
    request = responses.calls[0].request
    assert request.url == AGENTS_URL
    assert request.headers["X-API-Key"] == API_KEY
    assert "X-AIM-API-Key" not in request.headers

    body = json.loads(request.body)
    assert len(body["publicKey"]) == 44, "a base64 Ed25519 public key is sent"
    assert body["capabilities"] == ["db:read"], "capabilities declared at secure() reach the server"

    saved = mock_save.call_args[0][1]
    assert saved["agent_id"] == AGENT_ID
    assert saved["public_key"] == body["publicKey"]
    assert len(saved["private_key"]) == 88, "the private key is the locally generated one"
    assert saved["private_key"] not in responses.calls[0].response.text
    assert len(base64.b64decode(saved["private_key"])) == 64
    assert isinstance(client, AIMClient)
    assert client.agent_id == AGENT_ID


@responses.activate
@patch("aim_sdk.client._save_credentials")
@patch("aim_sdk.client._print_registration_success")
@patch("aim_sdk.client.security_logger")
def test_backend_public_key_mismatch_is_refused(mock_logger, mock_print, mock_save):
    """A stored public key that is not the one this process generated cannot
    sign for this agent; the client refuses to come up rather than fail on
    the first verification."""
    other = base64.b64encode(bytes(range(32))).decode("ascii")
    responses.add(responses.POST, AGENTS_URL, status=201, json={
        "id": AGENT_ID, "name": AGENT_NAME, "publicKey": other, "status": "pending",
    })

    with pytest.raises(ConfigurationError) as excinfo:
        _register()
    assert "Public key does not match private key" in str(excinfo.value)


@responses.activate
@patch("aim_sdk.client.security_logger")
def test_401_in_api_key_mode_names_the_key(mock_logger):
    """The backend answers a revoked, expired or foreign key with a generic
    401; the SDK names the credential and where a valid one comes from."""
    responses.add(responses.POST, AGENTS_URL, status=401, json={"error": "No authentication token provided"})

    with pytest.raises(AuthenticationError) as excinfo:
        _register()
    assert "API key was not accepted" in str(excinfo.value)


def test_api_key_header_constant():
    assert API_KEY_HEADER == "X-API-Key"


def test_no_dead_header_in_package():
    package_dir = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "aim_sdk")
    offenders = []
    for path in glob.glob(os.path.join(package_dir, "**", "*.py"), recursive=True):
        with open(path, encoding="utf-8") as handle:
            if "X-AIM-API-Key" in handle.read():
                offenders.append(os.path.relpath(path, package_dir))
    assert offenders == [], "a header no backend reads is still sent from: %s" % offenders
