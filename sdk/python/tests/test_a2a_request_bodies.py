"""
The bodies ``A2AClient`` sends to the trust and skill routes carry only the
members the server's handler binds, and a call the server would refuse is
refused before any request, in the server's words.

- ``update_trust_score`` sends nothing: the server answers ``PUT
  /api/v1/a2a/trust/:id`` with 405 because the A2A trust score has no write.
- ``register_skill`` sends ``inputModes`` and ``outputModes``; the
  ``input_schema`` and ``output_schema`` arguments warn and are not sent.

``update_agent_card`` and the card route are tested in
test_a2a_update_agent_card_document.py.

The members each handler binds, and the server's messages, are read from the
backend source, so a change on either side fails here.
"""

import json
import re
from pathlib import Path
from unittest.mock import Mock
from urllib.parse import urlparse

import pytest
import responses

from aim_sdk.a2a import (
    A2A_TRUST_SCORE_NOT_ASSERTED,
    A2AClient,
    A2AError,
    A2ATrustScoreWriteRefused,
)


AIM_URL = "https://aim.example.com"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
PEER_ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
SKILL_ROUTE = "/api/v1/a2a/skills"

HANDLERS_DIR = Path(__file__).resolve().parents[3] / "apps/backend/internal/interfaces/http/handlers"
HANDLER_SOURCE = HANDLERS_DIR / "a2a_handler.go"
# The status and reason of every A2A failure the handlers answer.
FAILURES_SOURCE = HANDLERS_DIR / "a2a_failures.go"

_REQUEST_TYPE = re.compile(r"var req struct \{(.*?)\n\t\}", re.S)
_BOUND_MEMBER = re.compile(r'^\s*\w+\s+(\S+)\s+`json:"([^",`]+)', re.M)


def _backend_source(path):
    assert path.is_file(), (
        f"{path} was not found. This test reads the backend's A2A "
        "handlers and has to run inside the repository."
    )
    return path.read_text(encoding="utf-8")


def _handler_body(source, handler):
    match = re.search(
        r"func \(h \*A2AHandler\) " + handler + r"\(c fiber\.Ctx\) error \{(.*?)\n\}\n", source, re.S
    )
    assert match, f"{handler} is not declared in {HANDLER_SOURCE}"
    return match.group(1)


def _bound_members(source, handler):
    """The JSON members the handler binds, each with its Go type."""
    request = _REQUEST_TYPE.search(_handler_body(source, handler))
    assert request, f"{handler} does not declare the request it binds"
    return {name: go_type for go_type, name in _BOUND_MEMBER.findall(request.group(1))}


def _go_string_const(source, name, path=HANDLER_SOURCE):
    match = re.search(r'^const ' + name + r' = "([^"]*)"$', source, re.M)
    assert match, f"const {name} is not declared in {path}"
    return match.group(1)


def _failure_answer(source, kind):
    """The answer entry a2a_failures.go holds for one failure kind."""
    match = re.search(r"^\t" + kind + r": \{(.*?)^\t\},$", source, re.S | re.M)
    assert match, f"{kind} has no multi-line answer in {FAILURES_SOURCE}"
    return match.group(1)


SOURCE = _backend_source(HANDLER_SOURCE)
FAILURES = _backend_source(FAILURES_SOURCE)
SKILL_MEMBERS = _bound_members(SOURCE, "RegisterSkill")


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


class FakeAIM:
    """Records each request and answers the skill route the way its handler
    does."""

    def __init__(self):
        self.requests = []

    def __call__(self, request):
        url = urlparse(request.url)
        body = json.loads(request.body) if request.body else {}
        self.requests.append((request.method, url.path, body))
        if (request.method, url.path) == ("POST", SKILL_ROUTE):
            return self._json(201, {"agentId": AGENT_ID, **body})
        return self._json(404, {"error": f"Cannot {request.method} {url.path}"})

    @staticmethod
    def _json(status, payload):
        return (status, {"Content-Type": "application/json"}, json.dumps(payload))


@pytest.fixture
def fake():
    fake = FakeAIM()
    with responses.RequestsMock(assert_all_requests_are_fired=False) as rsps:
        for method in (responses.GET, responses.POST, responses.PUT, responses.DELETE):
            rsps.add_callback(method, re.compile(re.escape(AIM_URL) + r"/.*"), callback=fake)
        yield fake


# ---------------------------------------------------------------------------
# update_trust_score
# ---------------------------------------------------------------------------

def test_update_trust_score_raises_the_server_refusal_without_a_request(client, fake):
    with pytest.raises(A2ATrustScoreWriteRefused) as raised:
        client.update_trust_score(PEER_ID, score=0.9, confidence=0.8, factors={"uptime": 1.0})

    assert fake.requests == []
    error = raised.value
    assert isinstance(error, A2AError)
    assert error.status_code == 405
    assert error.target_agent_id == PEER_ID
    assert error.reason == _go_string_const(SOURCE, "A2ATrustScoreNotAsserted") == A2A_TRUST_SCORE_NOT_ASSERTED
    assert "a2aTrustScoreNotWritable" in _handler_body(SOURCE, "RefuseTrustScoreWrite")
    refusal = _failure_answer(FAILURES, "a2aTrustScoreNotWritable")
    assert "reason: A2ATrustScoreNotAsserted," in refusal
    assert f'"read":   "{error.read_route}"' in refusal
    assert f'"record": "{error.record_route}"' in refusal
    for part in (A2A_TRUST_SCORE_NOT_ASSERTED, error.read_route, error.record_route, PEER_ID):
        assert part in str(error)


# ---------------------------------------------------------------------------
# register_skill
# ---------------------------------------------------------------------------

def test_register_skill_sends_the_members_the_skill_route_binds(client, fake):
    client.register_skill(
        name="analyze_code",
        description="Analyzes code for quality issues",
        tags=["code"],
        input_modes=["text/plain"],
        output_modes=["application/json"],
        examples=["Review this function"],
    )

    sent = {
        "agentId": AGENT_ID,
        "name": "analyze_code",
        "description": "Analyzes code for quality issues",
        "tags": ["code"],
        "inputModes": ["text/plain"],
        "outputModes": ["application/json"],
        "examples": ["Review this function"],
    }
    assert fake.requests == [("POST", SKILL_ROUTE, sent)]
    for member in sent:
        assert member in SKILL_MEMBERS, f"RegisterSkill does not bind {member}: {SKILL_MEMBERS}"


def test_register_skill_schemas_warn_and_are_not_sent(client, fake):
    with pytest.warns(DeprecationWarning, match="input_schema and output_schema"):
        client.register_skill(
            "analyze_code",
            "Analyzes code for quality issues",
            {"type": "object"},
            {"type": "object"},
            ["code"],
        )

    assert fake.requests == [("POST", SKILL_ROUTE, {
        "agentId": AGENT_ID,
        "name": "analyze_code",
        "description": "Analyzes code for quality issues",
        "tags": ["code"],
    })]
