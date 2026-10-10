"""
``AIMClient.register_mcp`` sends the request that the backend route for adding
MCP servers to an agent's list reads.

``PUT /api/v1/agents/{id}/mcp-servers`` (``AddMCPServersToAgent``) binds
``AddMCPServersRequest``: ``mcpServerIds``, ``detectedMethod``, ``confidence``
and ``metadata``. It adds each identifier to the agent's list and answers
``message``, ``talksTo``, ``added_servers`` and ``total_count``. The route sits
behind the member role gate: the token of a signed-in user with the member
role or higher passes it, and an API key or an agent signature is answered
401 ``Authentication required``.

The SDK posted those members, spelled in snake case, to
``POST /api/v1/sdk-api/agents/{id}/mcp-servers``, the route that creates an MCP
server. That route reads none of them, so the call created a server with an
empty name and URL, or was refused, and the given server was never added.

The stand-in server below answers both routes. The members it binds for the
first one, and the fact that the backend registers that route behind the
member role gate, are read from the backend source.
"""

import base64
import json
import re
import uuid
from pathlib import Path

import pytest
import responses
from nacl.encoding import Base64Encoder
from nacl.signing import SigningKey

from aim_sdk import AIMClient
from aim_sdk.exceptions import AuthenticationError, VerificationError


AIM_URL = "https://aim.example.com"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
ATTACH_PATH = f"/api/v1/agents/{AGENT_ID}/mcp-servers"
CREATE_PATH = f"/api/v1/sdk-api/agents/{AGENT_ID}/mcp-servers"

MCP_SERVER_ID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
MEMBER_TOKEN = "member-token-not-real"
VIEWER_TOKEN = "viewer-token-not-real"

BACKEND = Path(__file__).resolve().parents[3] / "apps/backend"
ROUTES_SOURCE = BACKEND / "cmd/server/main.go"
REQUEST_TYPE_SOURCE = BACKEND / "internal/application/agent_service.go"

_ATTACH_ROUTE = re.compile(
    r'\.Put\("/:id/mcp-servers",\s*middleware\.MemberMiddleware\(\),\s*h\.Agent\.AddMCPServersToAgent\)'
)
_REQUEST_TYPE = re.compile(r"type AddMCPServersRequest struct \{(.*?)\n\}", re.S)
_JSON_MEMBER = re.compile(r'`json:"([^",`]+)')


def _read_bound_members():
    """The JSON members of the request type the attach route binds."""
    assert REQUEST_TYPE_SOURCE.is_file(), (
        f"{REQUEST_TYPE_SOURCE} was not found. This test reads the backend's "
        "request type and has to run inside the repository."
    )
    declared = _REQUEST_TYPE.search(REQUEST_TYPE_SOURCE.read_text(encoding="utf-8"))
    assert declared, f"AddMCPServersRequest is not declared in {REQUEST_TYPE_SOURCE}"
    members = _JSON_MEMBER.findall(declared.group(1))
    assert members, f"AddMCPServersRequest has no JSON members in {REQUEST_TYPE_SOURCE}"
    return members


BOUND_MEMBERS = _read_bound_members()


class FakeAIM:
    """Answers an agent's two MCP server routes the way the backend does."""

    def __init__(self, agent_exists=True):
        self.agent_exists = agent_exists
        self.requests = []
        self.talks_to = []
        self.created = []

    def __call__(self, request):
        path = request.path_url.split("?", 1)[0]
        body = json.loads(request.body) if request.body else {}
        self.requests.append((request.method, path, body, dict(request.headers)))
        if (request.method, path) == ("PUT", ATTACH_PATH):
            return self._attach(request, body)
        if (request.method, path) == ("POST", CREATE_PATH):
            return self._create(body)
        return self._json(404, {"error": f"Cannot {request.method} {path}"})

    def _attach(self, request, body):
        # The member role gate: only a user token carries a role.
        token = request.headers.get("Authorization")
        if token == f"Bearer {VIEWER_TOKEN}":
            return self._json(403, {
                "error": "Member access required (viewers cannot perform this action)"
            })
        if token != f"Bearer {MEMBER_TOKEN}":
            return self._json(401, {"error": "Authentication required"})
        # Members the request type does not declare are not read.
        bound = {member: body.get(member) for member in BOUND_MEMBERS}
        identifiers = bound["mcpServerIds"] or []
        if not identifiers:
            return self._json(400, {"error": "mcp_server_ids is required and must not be empty"})
        if not self.agent_exists:
            return self._json(404, {"error": "not found"})
        added = [i for i in identifiers if i not in self.talks_to]
        self.talks_to.extend(added)
        return self._json(200, {
            "message": f"Successfully added {len(added)} MCP server(s)",
            "talksTo": list(self.talks_to),
            "added_servers": added,
            "total_count": len(self.talks_to),
        })

    def _create(self, body):
        # CreateMCPServer reads name and url, and none of the attach members.
        server = {"id": str(uuid.uuid4()), "name": body.get("name", ""), "url": body.get("url", "")}
        self.created.append(server)
        return self._json(201, server)

    @staticmethod
    def _json(status, payload):
        return (status, {"Content-Type": "application/json"}, json.dumps(payload))


class _SignIn:
    """Stands in for the SDK sign-in: hands out a user's access token."""

    def __init__(self, token):
        self.token = token

    def get_access_token(self):
        if isinstance(self.token, Exception):
            raise self.token
        return self.token


def _signing_client(sign_in=None):
    signing_key = SigningKey.generate()
    return AIMClient(
        agent_id=AGENT_ID,
        public_key=signing_key.verify_key.encode(encoder=Base64Encoder).decode("utf-8"),
        private_key=base64.b64encode(bytes(signing_key)).decode("utf-8"),
        aim_url=AIM_URL,
        timeout=5,
        auto_retry=False,
        oauth_token_manager=sign_in,
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


def _serve(fake):
    for method in (responses.PUT, responses.POST, responses.GET):
        responses.add_callback(method, re.compile(re.escape(AIM_URL) + r"/.*"), callback=fake)


def _only_request(fake):
    """The single request the stand-in received: (method, path, body, headers)."""
    assert len(fake.requests) == 1, [(method, path) for method, path, _, _ in fake.requests]
    return fake.requests[0]


def test_backend_registers_the_route_behind_the_member_role_gate():
    # Guards the stand-in: it answers this path with this method, and refuses
    # every caller but a member, because the backend registers the route so.
    assert ROUTES_SOURCE.is_file(), (
        f"{ROUTES_SOURCE} was not found. This test reads the backend's routes "
        "and has to run inside the repository."
    )
    assert _ATTACH_ROUTE.search(ROUTES_SOURCE.read_text(encoding="utf-8")), (
        "PUT /:id/mcp-servers is no longer registered for AddMCPServersToAgent "
        f"behind MemberMiddleware in {ROUTES_SOURCE}"
    )


@responses.activate
def test_the_members_sent_are_the_ones_the_backend_binds():
    fake = FakeAIM()
    _serve(fake)

    _signing_client(_SignIn(MEMBER_TOKEN)).register_mcp(MCP_SERVER_ID)

    _, _, body, _ = _only_request(fake)
    assert sorted(body) == sorted(BOUND_MEMBERS)


@responses.activate
def test_a_signed_in_member_adds_the_given_server_to_the_agents_list():
    fake = FakeAIM()
    _serve(fake)

    result = _signing_client(_SignIn(MEMBER_TOKEN)).register_mcp(MCP_SERVER_ID)

    method, path, body, _ = _only_request(fake)
    assert (method, path) == ("PUT", ATTACH_PATH)
    assert body == {
        "mcpServerIds": [MCP_SERVER_ID],
        "detectedMethod": "manual",
        "confidence": 100.0,
        "metadata": {},
    }
    assert fake.talks_to == [MCP_SERVER_ID]
    assert fake.created == []
    assert result == {
        "message": "Successfully added 1 MCP server(s)",
        "talksTo": [MCP_SERVER_ID],
        "added_servers": [MCP_SERVER_ID],
        "total_count": 1,
    }


@responses.activate
def test_the_users_token_goes_with_the_request():
    fake = FakeAIM()
    _serve(fake)

    _signing_client(_SignIn(MEMBER_TOKEN)).register_mcp(MCP_SERVER_ID)

    _, _, _, headers = _only_request(fake)
    assert headers["Authorization"] == f"Bearer {MEMBER_TOKEN}"


@responses.activate
def test_the_given_method_confidence_and_metadata_are_sent():
    fake = FakeAIM()
    _serve(fake)

    _signing_client(_SignIn(MEMBER_TOKEN)).register_mcp(
        "filesystem",
        detection_method="auto_config",
        confidence=85.5,
        metadata={"source": "claude_desktop_config"},
    )

    method, path, body, _ = _only_request(fake)
    assert (method, path) == ("PUT", ATTACH_PATH)
    assert body == {
        "mcpServerIds": ["filesystem"],
        "detectedMethod": "auto_config",
        "confidence": 85.5,
        "metadata": {"source": "claude_desktop_config"},
    }
    assert fake.talks_to == ["filesystem"]


@responses.activate
def test_a_server_already_on_the_list_is_answered_with_nothing_added():
    fake = FakeAIM()
    _serve(fake)
    client = _signing_client(_SignIn(MEMBER_TOKEN))

    client.register_mcp(MCP_SERVER_ID)
    again = client.register_mcp(MCP_SERVER_ID)

    assert again["added_servers"] == []
    assert again["total_count"] == 1
    assert fake.talks_to == [MCP_SERVER_ID]
    assert fake.created == []


@responses.activate
@pytest.mark.parametrize(
    "make_client", [_signing_client, _api_key_client], ids=["signing-keys", "api-key"]
)
def test_without_a_sign_in_the_refusal_says_what_the_route_admits(make_client):
    fake = FakeAIM()
    _serve(fake)

    with pytest.raises(AuthenticationError) as excinfo:
        make_client().register_mcp(MCP_SERVER_ID)

    # One request, to the attach route, with no user token. Nothing is created.
    method, path, _, headers = _only_request(fake)
    assert (method, path) == ("PUT", ATTACH_PATH)
    assert "Authorization" not in headers
    assert fake.talks_to == []
    assert fake.created == []

    message = str(excinfo.value)
    assert "Authentication failed" in message
    assert "signed-in user with the member role or higher" in message
    assert "sent no user token" in message
    assert "aim-sdk login" in message


@responses.activate
@pytest.mark.parametrize(
    "token", [None, "", RuntimeError("refresh failed")], ids=["none", "empty", "raises"]
)
def test_a_sign_in_that_yields_no_token_is_treated_as_no_sign_in(token):
    fake = FakeAIM()
    _serve(fake)

    with pytest.raises(AuthenticationError) as excinfo:
        _signing_client(_SignIn(token)).register_mcp(MCP_SERVER_ID)

    method, path, _, headers = _only_request(fake)
    assert (method, path) == ("PUT", ATTACH_PATH)
    assert "Authorization" not in headers
    assert "sent no user token" in str(excinfo.value)
    assert fake.created == []


@responses.activate
def test_a_refused_user_token_is_not_reported_as_a_missing_sign_in():
    fake = FakeAIM()
    _serve(fake)

    with pytest.raises(AuthenticationError) as excinfo:
        _signing_client(_SignIn(VIEWER_TOKEN)).register_mcp(MCP_SERVER_ID)

    message = str(excinfo.value)
    assert "Forbidden" in message
    assert "signed-in user with the member role or higher" in message
    assert "sent no user token" not in message
    assert "aim-sdk login" not in message
    assert fake.talks_to == []
    assert fake.created == []


@responses.activate
def test_an_agent_the_organization_does_not_have_is_a_verification_error():
    fake = FakeAIM(agent_exists=False)
    _serve(fake)

    with pytest.raises(VerificationError) as excinfo:
        _signing_client(_SignIn(MEMBER_TOKEN)).register_mcp(MCP_SERVER_ID)

    method, path, _, _ = _only_request(fake)
    assert (method, path) == ("PUT", ATTACH_PATH)
    assert "404" in str(excinfo.value)
    assert fake.created == []
