"""
``get_user_feedback`` reads the summary of an agent's user ratings.

The SDK pages documented ``submit_feedback``, which ``AIMClient`` never had,
and the issue that reported it asked for a read method for an agent's user
feedback. ``get_user_feedback`` calls GET /api/v1/agents/{id}/feedback, which
returns the counts and the average of the ratings, serves a request the agent
signs for its own ID and answers another agent's ID with 403.
"""

import base64

import pytest
import responses
from nacl.encoding import Base64Encoder
from nacl.signing import SigningKey, VerifyKey

from aim_sdk import AIMClient
from aim_sdk.exceptions import AuthenticationError, VerificationError

AIM_URL = "https://aim.example.com"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
OTHER_ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
ROUTE = f"{AIM_URL}/api/v1/agents/{AGENT_ID}/feedback"


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


def test_get_user_feedback_reads_the_agent_route(signed_client):
    # Ratings of 5, 5, 5, 4, 4, 3, 2 and 1.
    body = {
        "agentId": AGENT_ID,
        "agentName": "a",
        "total": 8,
        "positiveCount": 5,
        "neutralCount": 1,
        "negativeCount": 2,
        "averageRating": 3.625,
    }
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, ROUTE, json=body, status=200)
        assert signed_client.get_user_feedback() == body
        request = rsps.calls[0].request
    assert request.body is None
    assert request.headers["X-Agent-ID"] == AGENT_ID
    assert _signed_path(request) == f"/api/v1/agents/{AGENT_ID}/feedback"


def test_get_user_feedback_returns_none_for_the_average_of_no_ratings(signed_client):
    body = {
        "agentId": AGENT_ID,
        "agentName": "a",
        "total": 0,
        "positiveCount": 0,
        "neutralCount": 0,
        "negativeCount": 0,
        "averageRating": None,
    }
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, ROUTE, json=body, status=200)
        summary = signed_client.get_user_feedback()
    assert summary["total"] == 0
    assert summary["averageRating"] is None


def test_get_user_feedback_takes_another_agent_id(signed_client):
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, f"{AIM_URL}/api/v1/agents/{OTHER_ID}/feedback", json={"total": 0}, status=200)
        assert signed_client.get_user_feedback(OTHER_ID) == {"total": 0}
        request = rsps.calls[0].request
    assert _signed_path(request) == f"/api/v1/agents/{OTHER_ID}/feedback"


@pytest.mark.parametrize("status, error", [(401, AuthenticationError), (403, AuthenticationError), (404, VerificationError)])
def test_get_user_feedback_raises_the_sdk_errors(signed_client, status, error):
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, f"{AIM_URL}/api/v1/agents/{OTHER_ID}/feedback", json={"error": "x"}, status=status)
        with pytest.raises(error):
            signed_client.get_user_feedback(OTHER_ID)
