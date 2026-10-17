"""
The bodies four ``client.py`` calls send carry only the members the server's
handler binds.

- ``report_execution_status`` sends ``executed``, ``executedAt`` and, when
  there is one, ``executionError``. ``strict_mode`` stays a parameter and is
  not sent: the server takes the enforcement mode from the organization.
- ``attest_isolation`` sends the four posture fields. With a signing key the
  request is signed in its headers over the method, path, timestamp and this
  body; no signature or signed message travels inside the body.
- ``register_agent`` posts no ``organizationDomain`` from either poster (SDK
  credentials, API key); the ``organization_domain`` parameter warns.
- the MCP attach sends no ``verificationMethod``: the server sets it from how
  the request authenticated.

Each call goes through the HTTP layer and the test reads the body it sent.
"""

import base64
import json
from datetime import datetime

import pytest
import responses
from nacl.encoding import Base64Encoder
from nacl.signing import SigningKey, VerifyKey

import aim_sdk.client as client_module
from aim_sdk import AIMClient
from aim_sdk.client import _register_single_mcp, register_agent
from aim_sdk.exceptions import AIMError


AIM_URL = "https://aim.example.com"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
VERIFICATION_ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
AGENT_NAME = "route-body-agent"


@pytest.fixture
def api_key_client():
    return AIMClient(
        agent_id=AGENT_ID,
        aim_url=AIM_URL,
        api_key="not-a-real-key",
        timeout=5,
        auto_retry=False,
    )


@pytest.fixture
def signing_client():
    signing_key = SigningKey.generate()
    return AIMClient(
        agent_id=AGENT_ID,
        public_key=signing_key.verify_key.encode(encoder=Base64Encoder).decode("utf-8"),
        private_key=base64.b64encode(bytes(signing_key)).decode("utf-8"),
        aim_url=AIM_URL,
        timeout=5,
        auto_retry=False,
    )


def _sent_body(call):
    return json.loads(call.request.body)


# ---------------------------------------------------------------------------
# report_execution_status
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("strict_mode", [True, False])
@pytest.mark.parametrize("executed, execution_error", [(True, None), (False, "blocked by policy")])
@responses.activate
def test_report_execution_status_sends_what_the_server_reads(api_key_client, strict_mode, executed, execution_error):
    route = f"{AIM_URL}/api/v1/sdk-api/verifications/{VERIFICATION_ID}/execution-status"
    responses.add(responses.POST, route, json={"success": True}, status=200)

    api_key_client.report_execution_status(
        VERIFICATION_ID, executed=executed, strict_mode=strict_mode, execution_error=execution_error
    )

    assert len(responses.calls) == 1
    assert responses.calls[0].request.url == route
    body = _sent_body(responses.calls[0])
    executed_at = body.pop("executedAt")
    assert datetime.fromisoformat(executed_at).tzinfo is not None
    expected = {"executed": executed}
    if execution_error:
        expected["executionError"] = execution_error
    assert body == expected


# ---------------------------------------------------------------------------
# attest_isolation
# ---------------------------------------------------------------------------

@responses.activate
def test_attest_isolation_sends_the_four_posture_fields_signed_in_the_headers(signing_client):
    endpoint = f"/api/v1/sdk-api/agents/{AGENT_ID}/isolation-attestation"
    responses.add(responses.POST, f"{AIM_URL}{endpoint}", json={"id": "att-1", "isolationScore": 0.6}, status=201)

    signing_client.attest_isolation(sandbox="docker", network="namespace", filesystem="readonly", process="seccomp")

    assert len(responses.calls) == 1
    request = responses.calls[0].request
    body = _sent_body(responses.calls[0])
    assert body == {"sandbox": "docker", "network": "namespace", "filesystem": "readonly", "process": "seccomp"}

    # The transport signature covers the body: method, path, timestamp and
    # the body, as _make_request signs every request.
    message = "\n".join(["POST", endpoint, request.headers["X-Timestamp"], json.dumps(body, sort_keys=True)])
    verify_key = VerifyKey(request.headers["X-Public-Key"].encode("utf-8"), encoder=Base64Encoder)
    verify_key.verify(message.encode("utf-8"), base64.b64decode(request.headers["X-Signature"]))
    assert request.headers["X-Agent-ID"] == AGENT_ID


@responses.activate
def test_attest_isolation_with_an_api_key_sends_the_same_body(api_key_client):
    endpoint = f"/api/v1/sdk-api/agents/{AGENT_ID}/isolation-attestation"
    responses.add(responses.POST, f"{AIM_URL}{endpoint}", json={"id": "att-1"}, status=201)

    api_key_client.attest_isolation()

    assert _sent_body(responses.calls[0]) == {
        "sandbox": "none", "network": "none", "filesystem": "none", "process": "none",
    }


# ---------------------------------------------------------------------------
# register_agent, through both posters
# ---------------------------------------------------------------------------

REGISTRATION_BODY = {
    "name": AGENT_NAME,
    "displayName": AGENT_NAME,
    "description": "Route body test",
    "agentType": "ai_agent",
    "version": "1.0.0",
    "repositoryUrl": "https://example.com/repo",
}


def _registration_body(call):
    assert call.request.method == "POST"
    assert call.request.url == f"{AIM_URL}/api/v1/agents"
    body = _sent_body(call)
    public_key = body.pop("publicKey")
    assert len(base64.b64decode(public_key)) == 32
    return body


def _register(**kwargs):
    # The stand-in answers the registration with 400 once it has the body,
    # so nothing after the POST runs.
    with pytest.warns(DeprecationWarning, match="organization_domain"):
        with pytest.raises(AIMError, match="recorded"):
            register_agent(
                AGENT_NAME,
                description="Route body test",
                repository_url="https://example.com/repo",
                organization_domain="example.com",
                auto_detect=False,
                force_new=True,
                auto_hooks=False,
                **kwargs,
            )


@responses.activate
def test_register_agent_with_an_api_key_posts_no_organization_domain(monkeypatch):
    monkeypatch.setattr(client_module, "load_sdk_credentials", lambda: None)
    responses.add(responses.POST, f"{AIM_URL}/api/v1/agents", json={"error": "recorded"}, status=400)

    _register(aim_url=AIM_URL, api_key="not-a-real-key")

    assert len(responses.calls) == 1
    assert responses.calls[0].request.headers["X-API-Key"] == "not-a-real-key"
    assert _registration_body(responses.calls[0]) == REGISTRATION_BODY


class _TokenManager:
    credentials = {}

    def __init__(self, *args, **kwargs):
        pass

    def get_access_token(self, *args, **kwargs):
        return "not-a-real-token"


@responses.activate
def test_register_agent_with_sdk_credentials_posts_no_organization_domain(monkeypatch):
    monkeypatch.setattr(client_module, "load_sdk_credentials", lambda: {"aimUrl": AIM_URL})
    monkeypatch.setattr(client_module, "OAuthTokenManager", _TokenManager)
    responses.add(responses.GET, f"{AIM_URL}/api/v1/sdk-api/agents/{AGENT_NAME}", json={"error": "not found"}, status=404)
    responses.add(responses.POST, f"{AIM_URL}/api/v1/agents", json={"error": "recorded"}, status=400)

    _register()

    assert [call.request.method for call in responses.calls] == ["GET", "POST"]
    assert _registration_body(responses.calls[1]) == REGISTRATION_BODY


# ---------------------------------------------------------------------------
# The MCP attach
# ---------------------------------------------------------------------------

@responses.activate
def test_the_mcp_attach_sends_no_verification_method(api_key_client):
    route = f"{AIM_URL}/api/v1/sdk-api/agents/{AGENT_ID}/mcp-servers"
    responses.add(responses.POST, route, json={"id": "mcp-001", "name": "github"}, status=201)

    result = _register_single_mcp(
        client=api_key_client,
        aim_url=AIM_URL,
        headers={},
        mcp_def={
            "name": "github",
            "description": "GitHub API",
            "url": "stdio://github",
            "version": "1.2.0",
            "capabilities": ["repo:read"],
        },
        agent_id=AGENT_ID,
        detected_capabilities={},
    )

    assert result["status"] == "registered", result
    assert responses.calls[0].request.url == route
    assert _sent_body(responses.calls[0]) == {
        "name": "github",
        "description": "GitHub API",
        "url": "stdio://github",
        "version": "1.2.0",
        "capabilities": ["repo:read"],
        "registeredByAgent": AGENT_ID,
    }
