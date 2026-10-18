"""
``A2AClient.list_skills`` requests the route the server registers for an
agent's skills.

The ``/api/v1/a2a`` group in main.go registers ``GET /agents/:id/skills``,
served by ``GetAgentSkills``, and no ``GET /skills``, the request
``list_skills`` made. ``list_skills`` requests
``GET /api/v1/a2a/agents/<id>/skills`` with the client's own agent ID, the
request ``get_skills()`` makes when it is given no agent ID.

The stand-in server below answers that route with the members
``GetAgentSkills`` answers, and any other request with 404. The route and the
members are checked against the backend source.
"""

import json
import re
from pathlib import Path
from unittest.mock import Mock
from urllib.parse import urlparse

import pytest
import responses

from aim_sdk.a2a import A2AClient, A2AError


AIM_URL = "https://aim.example.com"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
PEER_ID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"

BACKEND = Path(__file__).resolve().parents[3] / "apps/backend"
ROUTES_SOURCE = BACKEND / "cmd/server/main.go"
HANDLER_SOURCE = BACKEND / "internal/interfaces/http/handlers/a2a_handler.go"

# A route registered on the /api/v1/a2a group, or on the bound and held
# routers main.go wraps it in: its method, its path and its A2A handler.
_A2A_ROUTE = re.compile(
    r'^\s*(?:a2a|a2aBound|a2aHeld)\.(Get|Post|Put|Patch|Delete)\("([^"]+)",[^\n]*?\bh\.A2A\.(\w+)\)',
    re.M,
)
_GET_AGENT_SKILLS = re.compile(
    r"^func \(h \*A2AHandler\) GetAgentSkills\(c fiber\.Ctx\) error \{\n(.*?)^\}", re.M | re.S
)
_JSON_ANSWER = re.compile(r"return c\.JSON\(fiber\.Map\{(.*?)\}\)", re.S)
_MAP_KEY = re.compile(r'"(\w+)":')


def _read_source(path):
    assert path.is_file(), (
        f"{path} was not found. This test reads the backend's routes and "
        "handlers and has to run inside the repository."
    )
    return path.read_text(encoding="utf-8")


def _read_a2a_routes():
    """The (method, path) routes registered under /api/v1/a2a, with their handlers."""
    source = _read_source(ROUTES_SOURCE)
    for line in (
        'v1 := app.Group("/api/v1")',
        'a2a := v1.Group("/a2a")',
        "a2aBound := bindAgentRoutes(a2a)",
        "a2aHeld := holdAgentRoutes(a2a)",
    ):
        assert line in source, f"main.go no longer has {line!r}"
    return {(method.upper(), path): handler for method, path, handler in _A2A_ROUTE.findall(source)}


def _read_skills_answer_members():
    """The members of the answer GetAgentSkills gives for an agent it finds."""
    handler = _GET_AGENT_SKILLS.search(_read_source(HANDLER_SOURCE))
    assert handler, f"GetAgentSkills is not declared in {HANDLER_SOURCE}"
    answers = _JSON_ANSWER.findall(handler.group(1))
    assert len(answers) == 1, f"GetAgentSkills answers JSON in {len(answers)} places; this test reads one"
    return _MAP_KEY.findall(answers[0])


A2A_ROUTES = _read_a2a_routes()
SKILLS_ANSWER_MEMBERS = _read_skills_answer_members()


def test_the_backend_registers_the_agent_skills_route_and_no_skill_list_route():
    assert len(A2A_ROUTES) > 30, "the route reader found too few A2A routes; the reader is broken"
    assert A2A_ROUTES.get(("GET", "/agents/:id/skills")) == "GetAgentSkills"
    assert ("GET", "/skills") not in A2A_ROUTES, "main.go registers GET /api/v1/a2a/skills"
    assert SKILLS_ANSWER_MEMBERS == ["agentId", "skills", "count"]


def _skill(skill_uuid, agent_id, skill_id, name):
    """A skill with the members of A2ASkill that its JSON carries when they are empty."""
    return {
        "id": skill_uuid,
        "agentId": agent_id,
        "skillId": skill_id,
        "name": name,
        "invocationCount": 0,
        "successCount": 0,
        "failureCount": 0,
        "isVerified": False,
        "attestationCount": 0,
        "createdAt": "2026-10-17T00:00:00Z",
        "updatedAt": "2026-10-17T00:00:00Z",
    }


OWN_SKILLS = [
    _skill("3f0c8a52-0000-4000-8000-000000000001", AGENT_ID, "analyze_code", "Analyze code"),
    _skill("3f0c8a52-0000-4000-8000-000000000002", AGENT_ID, "summarize", "Summarize"),
]
PEER_SKILLS = [
    _skill("3f0c8a52-0000-4000-8000-000000000003", PEER_ID, "translate", "Translate"),
]

_AGENT_SKILLS_PATH = re.compile(r"/api/v1/a2a/agents/([^/]+)/skills")


class FakeAIM:
    """Answers GET /api/v1/a2a/agents/:id/skills with the skills of that agent."""

    def __init__(self, skills):
        self.skills = skills
        self.requests = []

    def __call__(self, request):
        path = urlparse(request.url).path
        self.requests.append((request.method, path))
        agent = _AGENT_SKILLS_PATH.fullmatch(path)
        if request.method == "GET" and agent:
            skills = self.skills.get(agent.group(1), [])
            return self._json(200, {"agentId": agent.group(1), "skills": skills, "count": len(skills)})
        return self._json(404, {"error": f"Cannot {request.method} {path}"})

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
    for method in (responses.GET, responses.POST, responses.PUT, responses.DELETE):
        rsps.add_callback(method, re.compile(re.escape(AIM_URL) + r"/.*"), callback=fake)
    return rsps


def test_list_skills_returns_the_skills_of_the_clients_own_agent(client):
    fake = FakeAIM({AGENT_ID: OWN_SKILLS, PEER_ID: PEER_SKILLS})
    with _serve(fake):
        skills = client.list_skills()

    assert skills == OWN_SKILLS
    assert fake.requests == [("GET", f"/api/v1/a2a/agents/{AGENT_ID}/skills")]


def test_list_skills_makes_the_request_get_skills_makes(client):
    fake = FakeAIM({AGENT_ID: OWN_SKILLS, PEER_ID: PEER_SKILLS})
    with _serve(fake):
        listed = client.list_skills()
        got = client.get_skills()

    assert listed == got
    assert len(fake.requests) == 2
    assert fake.requests[0] == fake.requests[1]


def test_list_skills_of_an_agent_with_no_skills_is_an_empty_list(client):
    fake = FakeAIM({AGENT_ID: [], PEER_ID: PEER_SKILLS})
    with _serve(fake):
        assert client.list_skills() == []

    assert fake.requests == [("GET", f"/api/v1/a2a/agents/{AGENT_ID}/skills")]


def test_list_skills_raises_a2a_error_with_the_status_of_a_refusal(client):
    rsps = responses.RequestsMock(assert_all_requests_are_fired=False)
    rsps.add(
        responses.GET,
        f"{AIM_URL}/api/v1/a2a/agents/{AGENT_ID}/skills",
        json={"error": "refused by the stand-in"},
        status=401,
    )
    with rsps:
        with pytest.raises(A2AError) as raised:
            client.list_skills()

    assert raised.value.status_code == 401
    assert "refused by the stand-in" in str(raised.value)
