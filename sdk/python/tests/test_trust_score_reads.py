"""
``get_trust_score`` and ``get_trust_score_history`` read the agent routes.

The SDK pages documented both methods while ``AIMClient`` had neither. They
call GET /api/v1/agents/{id}/trust-score and /trust-score/history, the routes
the agents group admits a signed agent to. The server verifies the signature
over ``c.OriginalURL()``, which carries the query string, so the history
call must sign ``?limit=N`` along with the path.
"""

import base64

import pytest
import responses
from nacl.encoding import Base64Encoder
from nacl.signing import SigningKey, VerifyKey

from aim_sdk import AIMClient
from aim_sdk.exceptions import AuthenticationError, ConfigurationError, VerificationError

AIM_URL = "https://aim.example.com"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
OTHER_ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"


@pytest.fixture
def signed_client():
    signing_key = SigningKey.generate()
    return AIMClient(
        agent_id=AGENT_ID,
        public_key=signing_key.verify_key.encode(encoder=Base64Encoder).decode("utf-8"),
        private_key=base64.b64encode(bytes(signing_key)).decode("utf-8"),
        aim_url=AIM_URL,
        timeout=5,
        auto_retry=False,
    )


def _signed_path(request):
    """Verify the request's signature and return the path it covers."""
    headers = request.headers
    path = request.path_url
    signed = "\n".join([request.method, path, headers["X-Timestamp"]]).encode("utf-8")
    VerifyKey(base64.b64decode(headers["X-Public-Key"])).verify(
        signed, base64.b64decode(headers["X-Signature"])
    )
    return path


def test_get_trust_score_reads_the_agent_route(signed_client):
    body = {"agentId": AGENT_ID, "agentName": "a", "score": 0.82, "factors": {}, "calculatedAt": "2026-10-11T00:00:00Z"}
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, f"{AIM_URL}/api/v1/agents/{AGENT_ID}/trust-score", json=body, status=200)
        assert signed_client.get_trust_score() == body
        request = rsps.calls[0].request
    assert request.body is None
    assert _signed_path(request) == f"/api/v1/agents/{AGENT_ID}/trust-score"


def test_get_trust_score_takes_another_agent_id(signed_client):
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, f"{AIM_URL}/api/v1/agents/{OTHER_ID}/trust-score", json={"score": 0.5}, status=200)
        assert signed_client.get_trust_score(OTHER_ID) == {"score": 0.5}


def test_get_trust_score_history_without_limit_sends_no_query(signed_client):
    body = {"agentId": AGENT_ID, "agentName": "a", "history": [], "total": 0}
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, f"{AIM_URL}/api/v1/agents/{AGENT_ID}/trust-score/history", json=body, status=200)
        assert signed_client.get_trust_score_history() == body
        request = rsps.calls[0].request
    assert _signed_path(request) == f"/api/v1/agents/{AGENT_ID}/trust-score/history"


def test_get_trust_score_history_signs_the_limit_query(signed_client):
    with responses.RequestsMock() as rsps:
        rsps.add(
            responses.GET,
            f"{AIM_URL}/api/v1/agents/{AGENT_ID}/trust-score/history",
            match=[responses.matchers.query_param_matcher({"limit": "10"})],
            json={"history": [], "total": 0},
            status=200,
        )
        signed_client.get_trust_score_history(limit=10)
        request = rsps.calls[0].request
    assert _signed_path(request) == f"/api/v1/agents/{AGENT_ID}/trust-score/history?limit=10"


@pytest.mark.parametrize("limit", [0, -1, True, 2.5, "10"])
def test_get_trust_score_history_refuses_a_limit_the_server_would_ignore(signed_client, limit):
    with responses.RequestsMock():
        with pytest.raises(ConfigurationError, match="limit must be a positive integer"):
            signed_client.get_trust_score_history(limit=limit)


@pytest.mark.parametrize("status, error", [(403, AuthenticationError), (404, VerificationError)])
def test_trust_score_reads_raise_the_sdk_errors(signed_client, status, error):
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, f"{AIM_URL}/api/v1/agents/{OTHER_ID}/trust-score", json={"error": "x"}, status=status)
        rsps.add(responses.GET, f"{AIM_URL}/api/v1/agents/{OTHER_ID}/trust-score/history", json={"error": "x"}, status=status)
        with pytest.raises(error):
            signed_client.get_trust_score(OTHER_ID)
        with pytest.raises(error):
            signed_client.get_trust_score_history(OTHER_ID)
