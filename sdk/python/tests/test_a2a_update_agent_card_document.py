"""
``A2AClient.update_agent_card`` replaces the agent's card document.

``PUT /api/v1/a2a/cards/:id`` reads ``cardUrl`` or ``cardData`` and runs the
card registration: the server fetches or takes the document, recomputes its
hash, issues a new attestation and stores the card in place of the previous
one. It does not edit a card field by field, and it answers a body with
neither member 400, naming the field-level members the body carried.
``update_agent_card`` sent ``name``, ``description``, ``version`` and
``skills``, which the route read and did not store. It sends ``card_url`` as
``cardUrl`` and ``card_data`` as ``cardData``, and refuses the four field-level
arguments without sending a request.

The stand-in server below answers the way the handler does. The members the
handler binds, their Go types and its refusal are read from the backend
source.
"""

import json
import re
from datetime import datetime, timezone
from pathlib import Path
from unittest.mock import Mock
from urllib.parse import urlparse

import pytest
import responses

from aim_sdk.a2a import A2AClient


AIM_URL = "https://aim.example.com"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
CARD_ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
CARD_ROUTE = f"/api/v1/a2a/cards/{AGENT_ID}"
CARD_URL = "https://agent.example.com/.well-known/agent.json"

BACKEND = Path(__file__).resolve().parents[3] / "apps/backend"
ROUTES_SOURCE = BACKEND / "cmd/server/main.go"
HANDLER_SOURCE = BACKEND / "internal/interfaces/http/handlers/a2a_handler.go"

_UPDATE_HANDLER = re.compile(
    r"func \(h \*A2AHandler\) UpdateAgentCard\(c fiber\.Ctx\) error \{(.*?)\n\}\n", re.S
)
_REQUEST_TYPE = re.compile(r"var req struct \{(.*?)\n\t\}", re.S)
_BOUND_MEMBER = re.compile(r'^\s*\w+\s+(\S+)\s+`json:"([^",`]+)', re.M)
_REFUSAL = re.compile(r'const cardDocumentRequiredMessage = "([^"]+)"')

DOCUMENT = {
    "name": "Report agent",
    "description": "Summarizes and translates reports",
    "url": "https://agent.example.com",
    "version": "1.1.0",
    "skills": [
        {"id": "summarize", "name": "Summarize"},
        {"id": "translate", "name": "Translate", "tags": ["text"]},
    ],
}


def _handler_source():
    assert HANDLER_SOURCE.is_file(), (
        f"{HANDLER_SOURCE} was not found. This test reads the backend's "
        "card update handler and has to run inside the repository."
    )
    return HANDLER_SOURCE.read_text(encoding="utf-8")


def _read_bound_members():
    """The JSON members UpdateAgentCard binds, each with its Go type."""
    handler = _UPDATE_HANDLER.search(_handler_source())
    assert handler, f"UpdateAgentCard is not declared in {HANDLER_SOURCE}"
    request = _REQUEST_TYPE.search(handler.group(1))
    assert request, "UpdateAgentCard does not declare the request it binds"
    return {name: go_type for go_type, name in _BOUND_MEMBER.findall(request.group(1))}


def _read_refusal():
    """The handler's answer to a body with neither cardUrl nor cardData."""
    refusal = _REFUSAL.search(_handler_source())
    assert refusal, f"cardDocumentRequiredMessage is not declared in {HANDLER_SOURCE}"
    return refusal.group(1)


BOUND_MEMBERS = _read_bound_members()
REFUSAL = _read_refusal()
FIELD_MEMBERS = ["name", "description", "version", "endpoint", "skills"]


def test_the_route_reads_the_card_document_and_names_the_field_members():
    routes = ROUTES_SOURCE.read_text(encoding="utf-8")
    assert 'a2aBound.Put("/cards/:id", h.A2A.UpdateAgentCard)' in routes
    assert BOUND_MEMBERS.get("cardUrl") == "string", BOUND_MEMBERS
    assert BOUND_MEMBERS.get("cardData") == "json.RawMessage", BOUND_MEMBERS
    assert sorted(BOUND_MEMBERS) == sorted(["cardUrl", "cardData", *FIELD_MEMBERS]), BOUND_MEMBERS


def test_the_client_refuses_with_the_servers_answer():
    assert A2AClient.CARD_DOCUMENT_REQUIRED == REFUSAL


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
        if not body.get("cardUrl") and body.get("cardData") is None:
            carried = [member for member in FIELD_MEMBERS if member in body]
            if carried:
                return self._json(400, {
                    "error": f"{REFUSAL}; it does not read {', '.join(carried)}",
                    "unreadMembers": carried,
                })
            return self._json(400, {"error": REFUSAL})
        document = body.get("cardData") or DOCUMENT
        return self._json(200, {
            "cardId": CARD_ID,
            "agentId": AGENT_ID,
            "cardUrl": body.get("cardUrl") or f"inline://{AGENT_ID}",
            "attestationExpires": "2026-11-15T12:00:00Z",
            "skills": [skill["id"] for skill in document.get("skills", [])],
        })

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


def test_card_data_is_sent_as_the_document(client):
    fake = FakeAIM()
    with _serve(fake):
        card = client.update_agent_card(card_data=DOCUMENT)

    assert fake.requests == [("PUT", CARD_ROUTE, {"cardData": DOCUMENT})]
    assert card.card_id == CARD_ID
    assert card.agent_id == AGENT_ID
    assert card.card_url == f"inline://{AGENT_ID}"
    assert card.skills == [{"id": "summarize"}, {"id": "translate"}]
    assert card.attestation_expires == datetime(2026, 11, 15, 12, 0, tzinfo=timezone.utc)


def test_card_url_is_sent_for_the_server_to_fetch(client):
    fake = FakeAIM()
    with _serve(fake):
        card = client.update_agent_card(card_url=CARD_URL)

    assert fake.requests == [("PUT", CARD_ROUTE, {"cardUrl": CARD_URL})]
    assert card.card_url == CARD_URL
    assert card.card_id == CARD_ID


@pytest.mark.parametrize("arguments, unread", [
    ({"name": "Renamed"}, "name"),
    ({"skills": ["summarize"]}, "skills"),
    ({"name": "Renamed", "description": "d", "version": "2.0.0", "skills": []}, "name, description, version, skills"),
])
def test_field_arguments_without_a_document_raise_the_servers_answer_before_any_request(client, arguments, unread):
    fake = FakeAIM()
    with _serve(fake):
        with pytest.raises(ValueError) as raised:
            client.update_agent_card(**arguments)

    assert str(raised.value) == f"{REFUSAL}; it does not read {unread}"
    assert fake.requests == [], "nothing is sent for a card update the server refuses"


def test_no_document_raises_the_servers_answer_before_any_request(client):
    fake = FakeAIM()
    with _serve(fake):
        with pytest.raises(ValueError) as raised:
            client.update_agent_card()

    assert str(raised.value) == REFUSAL
    assert fake.requests == []


@pytest.mark.parametrize("arguments", [
    {"name": "Renamed", "card_data": DOCUMENT},
    {"skills": ["summarize"], "card_url": CARD_URL},
])
def test_field_arguments_beside_a_document_raise_before_any_request(client, arguments):
    fake = FakeAIM()
    with _serve(fake):
        with pytest.raises(ValueError, match="does not edit fields"):
            client.update_agent_card(**arguments)

    assert fake.requests == [], "a field argument the route does not read is never dropped silently"


@pytest.mark.parametrize("card_data", [json.dumps(DOCUMENT), [DOCUMENT]])
def test_card_data_that_is_not_a_document_raises_before_any_request(client, card_data):
    fake = FakeAIM()
    with _serve(fake):
        with pytest.raises(ValueError, match="card_data must be the card document"):
            client.update_agent_card(card_data=card_data)

    assert fake.requests == []
