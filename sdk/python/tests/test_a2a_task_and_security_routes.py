"""
Four A2A methods request the routes the server registers.

The server registers ``GET /api/v1/a2a/tasks`` (filtered by the ``agentId``
query parameter), ``GET`` and ``PUT /api/v1/a2a/security/settings`` and
``GET /api/v1/a2a/security/violations``, with no identifier in the path: each
answers for the organization of the caller's credentials.
``get_task_history``, ``get_security_settings``, ``update_security_settings``
and ``get_security_violations`` put an agent or organization identifier in
the path, on routes the server does not register.

``PUT /api/v1/a2a/security/settings`` binds ``A2ASecuritySettings`` and writes
every setting of it: a setting the request does not carry is written as its
zero value. ``update_security_settings`` sent only the settings it was given,
three of them under names the type does not declare (``requireSigning``,
``allowedAgents`` and ``blockedAgents``).

The stand-in server below answers the four routes. The routes, and the JSON
members ``A2ASecuritySettings`` declares, are read from the backend source.
"""

import json
import re
from pathlib import Path
from unittest.mock import Mock
from urllib.parse import parse_qs, urlparse

import pytest
import responses

from aim_sdk.a2a import A2AClient, A2AError


AIM_URL = "https://aim.example.com"
ORG_ID = "6f1c2e4a-0000-4000-8000-000000000001"
OTHER_ORG_ID = "6f1c2e4a-0000-4000-8000-000000000002"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
PEER_ID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"
UNRELATED_ID = "9b2d1f00-0000-4000-8000-0000000000aa"

BACKEND = Path(__file__).resolve().parents[3] / "apps/backend"
ROUTES_SOURCE = BACKEND / "cmd/server/main.go"
SETTINGS_SOURCE = BACKEND / "internal/domain/a2a.go"

_SETTINGS_TYPE = re.compile(r"type A2ASecuritySettings struct \{(.*?)\n\}", re.S)
_JSON_MEMBER = re.compile(r'`json:"([^",`]+)')


def _read_settings_members():
    """The JSON members of the settings type the PUT route binds."""
    assert SETTINGS_SOURCE.is_file(), (
        f"{SETTINGS_SOURCE} was not found. This test reads the backend's "
        "settings type and has to run inside the repository."
    )
    declared = _SETTINGS_TYPE.search(SETTINGS_SOURCE.read_text(encoding="utf-8"))
    assert declared, f"A2ASecuritySettings is not declared in {SETTINGS_SOURCE}"
    return _JSON_MEMBER.findall(declared.group(1))


SETTINGS_MEMBERS = _read_settings_members()


def test_the_backend_registers_the_four_routes():
    routes = ROUTES_SOURCE.read_text(encoding="utf-8")
    assert 'a2a := v1.Group("/a2a")' in routes
    for route in (
        'a2a.Get("/tasks", h.A2A.ListTasks)',
        'a2a.Get("/security/settings", h.A2A.GetSecuritySettings)',
        'a2a.Put("/security/settings", middleware.ManagerMiddleware(), h.A2A.UpdateSecuritySettings)',
        'a2a.Get("/security/violations", h.A2A.GetSecurityViolations)',
    ):
        assert route in routes, f"main.go does not register {route}"
    for member in ("enforcementMode", "requireRequestSigning", "minTrustScore",
                   "requireVerifiedSkills", "allowedAgentIds", "blockedAgentIds"):
        assert member in SETTINGS_MEMBERS, f"A2ASecuritySettings does not declare {member}"


class FakeAIM:
    """Answers the four routes for one organization, the way the backend does."""

    def __init__(self, violations=None):
        self.requests = []
        # The backend's answer for an organization with no stored settings.
        self.settings = {
            "id": "00000000-0000-0000-0000-000000000000",
            "organizationId": ORG_ID,
            "enforcementMode": "monitor",
            "requireVerifiedSkills": False,
            "minTrustScore": 0,
            "minAttestationCount": 0,
            "requireRequestSigning": True,
            "requireNonceValidation": True,
            "nonceValiditySeconds": 300,
            "maxRequestsPerMinute": 60,
            "maxRequestsPerHour": 1000,
        }
        self.tasks = [
            {"id": "t-1", "externalTaskId": "ext-1", "clientAgentId": AGENT_ID, "remoteAgentId": PEER_ID, "state": "COMPLETED"},
            {"id": "t-2", "externalTaskId": "ext-2", "clientAgentId": AGENT_ID, "remoteAgentId": UNRELATED_ID, "state": "FAILED"},
        ]
        self.violations = violations

    def __call__(self, request):
        url = urlparse(request.url)
        query = parse_qs(url.query)
        body = json.loads(request.body) if request.body else None
        self.requests.append((request.method, url.path, query, body))
        route = (request.method, url.path)
        if route == ("GET", "/api/v1/a2a/tasks"):
            tasks = self.tasks
            # ListTasks ignores an agentId it cannot parse as a UUID.
            agent = query.get("agentId", [""])[0]
            if re.fullmatch(r"[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}", agent):
                tasks = [t for t in tasks if agent in (t["clientAgentId"], t["remoteAgentId"])]
            return self._json(200, {"tasks": tasks, "total": len(tasks), "limit": 100, "offset": 0})
        if route == ("GET", "/api/v1/a2a/security/settings"):
            return self._json(200, self.settings)
        if route == ("PUT", "/api/v1/a2a/security/settings"):
            # Bind().JSON into A2ASecuritySettings, then store the whole struct:
            # a member the body does not carry is stored as its zero value.
            stored = {member: body.get(member) for member in SETTINGS_MEMBERS if member in body}
            stored["organizationId"] = ORG_ID
            self.settings = stored
            return self._json(200, stored)
        if route == ("GET", "/api/v1/a2a/security/violations"):
            return self._json(200, self.violations)
        return self._json(404, {"error": f"Cannot {request.method} {url.path}"})

    @staticmethod
    def _json(status, payload):
        return (status, {"Content-Type": "application/json"}, json.dumps(payload))


@pytest.fixture
def client():
    aim = Mock()
    aim.agent_id = AGENT_ID
    aim.aim_url = AIM_URL
    aim.api_key = "not-a-real-key"
    aim.signing_key = None
    aim.public_key = None
    aim.oauth_token_manager = None
    return A2AClient(aim)


def _serve(fake):
    rsps = responses.RequestsMock(assert_all_requests_are_fired=False)
    for method in (responses.GET, responses.PUT):
        rsps.add_callback(method, re.compile(re.escape(AIM_URL) + r"/.*"), callback=fake)
    return rsps


def test_task_history_lists_the_target_agents_tasks(client):
    fake = FakeAIM()
    with _serve(fake):
        history = client.get_task_history(PEER_ID, limit=10)

    assert [t["externalTaskId"] for t in history] == ["ext-1"]
    method, path, query, _ = fake.requests[-1]
    assert (method, path) == ("GET", "/api/v1/a2a/tasks")
    assert query == {"agentId": [PEER_ID], "limit": ["10"]}


def test_task_history_refuses_an_agent_id_the_server_would_ignore(client):
    fake = FakeAIM()
    with _serve(fake):
        with pytest.raises(ValueError):
            client.get_task_history("other-agent-id")
    assert fake.requests == [], "a request the server answers with every task must not be sent"


def test_security_settings_are_the_callers_organizations(client):
    fake = FakeAIM()
    with _serve(fake):
        assert client.get_security_settings()["enforcementMode"] == "monitor"
        assert client.get_security_settings(ORG_ID.upper())["organizationId"] == ORG_ID
        with pytest.raises(A2AError, match="caller's organization"):
            client.get_security_settings(OTHER_ORG_ID)
    assert {(m, p) for m, p, _, _ in fake.requests} == {("GET", "/api/v1/a2a/security/settings")}


def test_update_changes_the_given_settings_and_keeps_the_rest(client):
    fake = FakeAIM()
    with _serve(fake):
        updated = client.update_security_settings(
            ORG_ID,
            enforcement_mode="strict",
            require_signing=False,
            allowed_agents=[PEER_ID],
            blocked_agents=[UNRELATED_ID],
        )

    put = [r for r in fake.requests if r[0] == "PUT"]
    assert len(put) == 1
    _, path, _, body = put[0]
    assert path == "/api/v1/a2a/security/settings"
    unread = sorted(set(body) - set(SETTINGS_MEMBERS))
    assert unread == [], f"the server's settings type does not declare {unread}"
    assert updated["enforcementMode"] == "strict"
    assert updated["requireRequestSigning"] is False
    assert updated["allowedAgentIds"] == [PEER_ID]
    assert updated["blockedAgentIds"] == [UNRELATED_ID]
    # Settings the call did not name keep their values.
    for member in ("requireNonceValidation", "nonceValiditySeconds", "maxRequestsPerMinute", "maxRequestsPerHour"):
        assert fake.settings[member] == FakeAIM().settings[member], member


def test_update_for_another_organization_writes_nothing(client):
    fake = FakeAIM()
    with _serve(fake):
        with pytest.raises(A2AError, match="caller's organization"):
            client.update_security_settings(OTHER_ORG_ID, enforcement_mode="strict")
    assert [m for m, _, _, _ in fake.requests] == ["GET"]
    assert fake.settings["enforcementMode"] == "monitor"


def test_security_violations_are_the_list_the_server_answers(client):
    violation = {"id": "v-1", "organizationId": ORG_ID, "violationType": "LOW_TRUST", "actionTaken": "LOGGED"}
    fake = FakeAIM(violations=[violation])
    with _serve(fake):
        assert client.get_security_violations(limit=5, offset=10) == [violation]
        assert client.get_security_violations(ORG_ID) == [violation]
        with pytest.raises(A2AError, match="caller's organization"):
            client.get_security_violations(OTHER_ORG_ID)
    method, path, query, _ = fake.requests[0]
    assert (method, path) == ("GET", "/api/v1/a2a/security/violations")
    assert query == {"limit": ["5"], "offset": ["10"]}


def test_no_security_violations_is_an_empty_list(client):
    # A Go nil slice is encoded as null.
    with _serve(FakeAIM(violations=None)):
        assert client.get_security_violations() == []


@pytest.mark.parametrize("answer", [None, []])
def test_no_security_violations_for_another_organization_raises(client, answer):
    # An answer with no violations names no organization, so the settings
    # answer is what shows that organization_id is not the caller's.
    fake = FakeAIM(violations=answer)
    with _serve(fake):
        with pytest.raises(A2AError, match="caller's organization"):
            client.get_security_violations(OTHER_ORG_ID)
        assert [path for _, path, _, _ in fake.requests] == ["/api/v1/a2a/security/settings"], (
            "the violations of another organization must not be requested"
        )

        assert client.get_security_violations(ORG_ID) == []
        assert client.get_security_violations() == []
    assert [path for _, path, _, _ in fake.requests[1:]] == [
        "/api/v1/a2a/security/settings",
        "/api/v1/a2a/security/violations",
        "/api/v1/a2a/security/violations",
    ]
