"""
Two SDK requests carry only members the backend route binds.

- Automatic MCP registration (``_register_single_mcp``) posts to
  ``POST /api/v1/sdk-api/agents/{id}/mcp-servers``, which binds
  ``CreateMCPServerRequest``. It also sent ``verificationMethod``, which that
  type does not declare.
- ``attest_isolation`` posts to
  ``POST /api/v1/sdk-api/agents/{id}/isolation-attestation``, whose handler
  (``SubmitIsolationAttestation``) binds ``sandbox``, ``network``,
  ``filesystem`` and ``process``. It also sent ``signature`` and
  ``signedMessage``, which nothing reads; the request is authenticated by the
  agent's signature headers.

The members each route binds are read from the backend source.
"""

import base64
import json
import re
from pathlib import Path

import pytest
import responses
from nacl.encoding import Base64Encoder
from nacl.signing import SigningKey

from aim_sdk import AIMClient
from aim_sdk.client import _register_single_mcp

AIM_URL = "https://aim.example.com"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"

BACKEND = Path(__file__).resolve().parents[3] / "apps/backend"
MCP_SERVICE = BACKEND / "internal/application/mcp_service.go"
TRUST_SCORE_HANDLER = BACKEND / "internal/interfaces/http/handlers/trust_score_handler.go"

_JSON_MEMBER = re.compile(r'`json:"([^",`]+)')


def _read(path):
    assert path.is_file(), (
        f"{path} was not found. This test reads the backend source and has to run "
        "inside the repository."
    )
    return path.read_text(encoding="utf-8")


def _members_of(source, declaration, where):
    found = re.search(declaration + r"\{(.*?)\n\s*\}", source, re.S)
    assert found, f"{declaration!r} is not in {where}"
    members = _JSON_MEMBER.findall(found.group(1))
    assert members, f"{declaration!r} declares no JSON members in {where}"
    return set(members)


CREATE_MCP_SERVER_MEMBERS = _members_of(
    _read(MCP_SERVICE), r"type CreateMCPServerRequest struct ", MCP_SERVICE
)


def _isolation_members():
    source = _read(TRUST_SCORE_HANDLER)
    start = source.index("func (h *TrustScoreHandler) SubmitIsolationAttestation(")
    return _members_of(source[start:], r"var body struct ", TRUST_SCORE_HANDLER)


ISOLATION_MEMBERS = _isolation_members()


def _signing_client():
    signing_key = SigningKey.generate()
    return AIMClient(
        agent_id=AGENT_ID,
        public_key=signing_key.verify_key.encode(encoder=Base64Encoder).decode("utf-8"),
        private_key=base64.b64encode(bytes(signing_key)).decode("utf-8"),
        aim_url=AIM_URL,
        timeout=5,
        auto_retry=False,
        telemetry={"enabled": False},
    )


def _api_key_client():
    return AIMClient(
        agent_id=AGENT_ID,
        aim_url=AIM_URL,
        api_key="aim_test_key_12345",
        timeout=5,
        auto_retry=False,
        telemetry={"enabled": False},
    )


def _body(call):
    return json.loads(call.request.body)


# ---------------------------------------------------------------------------
# Automatic MCP registration
# ---------------------------------------------------------------------------

@responses.activate
@pytest.mark.parametrize("make_client", [_signing_client, _api_key_client], ids=["signing-keys", "api-key"])
def test_automatic_mcp_registration_sends_members_the_create_route_binds(make_client):
    responses.add(
        responses.POST,
        f"{AIM_URL}/api/v1/sdk-api/agents/{AGENT_ID}/mcp-servers",
        json={"id": "mcp-001", "name": "filesystem"},
        status=201,
    )
    mcp_def = {
        "name": "filesystem",
        "description": "Filesystem MCP server",
        "url": "stdio://npx/-y/@modelcontextprotocol/server-filesystem",
        "version": "1.0.0",
        "capabilities": ["read_file"],
    }

    result = _register_single_mcp(
        client=make_client(),
        aim_url=AIM_URL,
        headers={},
        mcp_def=mcp_def,
        agent_id=AGENT_ID,
        detected_capabilities={},
    )

    assert result["status"] == "registered", result
    sent = _body(responses.calls[0])
    assert set(sent) <= CREATE_MCP_SERVER_MEMBERS, (
        f"sent {sorted(set(sent) - CREATE_MCP_SERVER_MEMBERS)}, which CreateMCPServerRequest "
        f"does not declare; it declares {sorted(CREATE_MCP_SERVER_MEMBERS)}"
    )
    assert sent["name"] == "filesystem"
    assert sent["url"] == mcp_def["url"]
    assert sent["registeredByAgent"] == AGENT_ID


# ---------------------------------------------------------------------------
# Isolation attestation
# ---------------------------------------------------------------------------

@responses.activate
def test_isolation_attestation_sends_the_members_the_handler_binds():
    responses.add(
        responses.POST,
        f"{AIM_URL}/api/v1/sdk-api/agents/{AGENT_ID}/isolation-attestation",
        json={"id": "att-1", "isolationScore": 0.8},
        status=201,
    )

    _signing_client().attest_isolation(
        sandbox="docker", network="namespace", filesystem="readonly", process="seccomp"
    )

    call = responses.calls[0]
    assert _body(call) == {
        "sandbox": "docker",
        "network": "namespace",
        "filesystem": "readonly",
        "process": "seccomp",
    }
    assert set(_body(call)) == ISOLATION_MEMBERS
    # The request is authenticated by the agent's signature headers.
    assert "X-Signature" in call.request.headers
