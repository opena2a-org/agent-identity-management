"""
``A2AClient.update_agent_card`` sends ``skills`` as the kind of value the
server binds.

``PUT /api/v1/a2a/cards/:id`` binds ``skills`` as a list of strings and
answers a card whose ``skills`` are objects with an ``id``.
``update_agent_card`` declared ``skills`` as a list of dictionaries and sent
the list as given. It sends each skill as its identifier: a string as it is,
and a dictionary as its ``id``.

The stand-in server below binds the request the way the handler does. The
members the handler binds, and their Go types, are read from the backend
source.
"""

import json
import re
from pathlib import Path
from unittest.mock import Mock
from urllib.parse import urlparse

import pytest
import responses

from aim_sdk.a2a import A2AClient


AIM_URL = "https://aim.example.com"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
CARD_ROUTE = f"/api/v1/a2a/cards/{AGENT_ID}"

BACKEND = Path(__file__).resolve().parents[3] / "apps/backend"
ROUTES_SOURCE = BACKEND / "cmd/server/main.go"
HANDLER_SOURCE = BACKEND / "internal/interfaces/http/handlers/a2a_handler.go"

_UPDATE_HANDLER = re.compile(
    r"func \(h \*A2AHandler\) UpdateAgentCard\(c fiber\.Ctx\) error \{(.*?)\n\}\n", re.S
)
_REQUEST_TYPE = re.compile(r"var req struct \{(.*?)\n\t\}", re.S)
_BOUND_MEMBER = re.compile(r'^\s*\w+\s+(\S+)\s+`json:"([^",`]+)', re.M)

# The card the stand-in has stored, in the form the handler answers it.
STORED_CARD = {
    "name": "Stored agent",
    "description": "As registered",
    "url": "https://agent.example.com",
    "version": "1.0.0",
    "skills": [
        {"id": "summarize", "name": "Summarize"},
        {"id": "translate", "name": "Translate", "tags": ["text"]},
    ],
}


def _read_bound_members():
    """The JSON members UpdateAgentCard binds, each with its Go type."""
    assert HANDLER_SOURCE.is_file(), (
        f"{HANDLER_SOURCE} was not found. This test reads the backend's "
        "card update handler and has to run inside the repository."
    )
    handler = _UPDATE_HANDLER.search(HANDLER_SOURCE.read_text(encoding="utf-8"))
    assert handler, f"UpdateAgentCard is not declared in {HANDLER_SOURCE}"
    request = _REQUEST_TYPE.search(handler.group(1))
    assert request, "UpdateAgentCard does not declare the request it binds"
    return {name: go_type for go_type, name in _BOUND_MEMBER.findall(request.group(1))}


BOUND_MEMBERS = _read_bound_members()


def _binds(value, go_type):
    """Whether encoding/json decodes a JSON value into a Go value of go_type."""
    if value is None:
        return True
    if go_type == "string":
        return isinstance(value, str)
    if go_type == "[]string":
        return isinstance(value, list) and all(item is None or isinstance(item, str) for item in value)
    raise AssertionError(f"the stand-in does not know the Go type {go_type}")


def test_the_route_binds_skills_as_a_list_of_strings():
    routes = ROUTES_SOURCE.read_text(encoding="utf-8")
    assert 'a2aBound.Put("/cards/:id", h.A2A.UpdateAgentCard)' in routes
    assert BOUND_MEMBERS.get("skills") == "[]string", BOUND_MEMBERS
    for member in ("name", "description", "version"):
        assert BOUND_MEMBERS.get(member) == "string", BOUND_MEMBERS


class FakeAIM:
    """Answers the card update route the way the backend's handler does."""

    def __init__(self):
        self.requests = []

    def __call__(self, request):
        url = urlparse(request.url)
        body = json.loads(request.body) if request.body else {}
        self.requests.append((request.method, url.path, body))
        if (request.method, url.path) != ("PUT", CARD_ROUTE):
            return self._json(404, {"error": f"Cannot {request.method} {url.path}"})
        # Bind().JSON refuses a member whose value is of another kind than
        # the Go type it is declared with.
        for member, go_type in BOUND_MEMBERS.items():
            if not _binds(body.get(member), go_type):
                return self._json(400, {"error": "Invalid request body"})
        return self._json(200, STORED_CARD)

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
    rsps.add_callback(responses.PUT, re.compile(re.escape(AIM_URL) + r"/.*"), callback=fake)
    return rsps


@pytest.mark.parametrize("skills, sent", [
    (["summarize", "translate"], ["summarize", "translate"]),
    ([{"id": "summarize", "name": "Summarize"}, {"id": "translate"}], ["summarize", "translate"]),
    (["summarize", {"id": "translate", "tags": ["text"]}], ["summarize", "translate"]),
    (("summarize",), ["summarize"]),
    ([], []),
])
def test_skills_are_sent_as_a_list_of_identifiers(client, skills, sent):
    fake = FakeAIM()
    with _serve(fake):
        card = client.update_agent_card(name="Renamed", skills=skills)

    assert fake.requests == [("PUT", CARD_ROUTE, {"name": "Renamed", "skills": sent})]
    assert card.skills == STORED_CARD["skills"]


def test_the_skills_a_card_is_answered_with_can_be_sent_back(client):
    fake = FakeAIM()
    with _serve(fake):
        card = client.update_agent_card(description="First call")
        client.update_agent_card(skills=card.skills)

    assert [body for _, _, body in fake.requests] == [
        {"description": "First call"},
        {"skills": ["summarize", "translate"]},
    ]


@pytest.mark.parametrize("skills", [
    [{"name": "Summarize"}],
    [{"id": ""}],
    [{"id": 7}],
    [7],
    [None],
    "summarize",
    {"id": "summarize"},
])
def test_what_is_not_a_list_of_skills_is_refused_before_any_request(client, skills):
    fake = FakeAIM()
    with _serve(fake):
        with pytest.raises(ValueError):
            client.update_agent_card(name="Renamed", skills=skills)
    assert fake.requests == [], "nothing is sent for skills the method refuses"
