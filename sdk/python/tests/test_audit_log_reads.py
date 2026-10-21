"""
``get_audit_logs`` reads the agent's audit log route.

The SDK pages documented ``get_audit_logs`` while ``AIMClient`` had no such
method. It calls GET /api/v1/agents/{id}/audit-logs, which serves a request
the agent signs for its own ID and answers another agent's ID with 403. The
server verifies the signature over ``c.OriginalURL()``, which carries the
query string, so ``limit`` and ``offset`` are signed along with the path.
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
ROUTE = f"{AIM_URL}/api/v1/agents/{AGENT_ID}/audit-logs"


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


def test_get_audit_logs_reads_the_agent_route(signed_client):
    body = {
        "agentId": AGENT_ID,
        "agentName": "a",
        "logs": [{"id": "r1", "action": "update", "resourceType": "agent", "actorType": "user"}],
        "total": 1,
        "limit": 50,
        "offset": 0,
    }
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, ROUTE, json=body, status=200)
        assert signed_client.get_audit_logs() == body
        request = rsps.calls[0].request
    assert request.body is None
    assert request.headers["X-Agent-ID"] == AGENT_ID
    assert _signed_path(request) == f"/api/v1/agents/{AGENT_ID}/audit-logs"


def test_get_audit_logs_signs_the_limit_and_offset_query(signed_client):
    with responses.RequestsMock() as rsps:
        rsps.add(
            responses.GET,
            ROUTE,
            match=[responses.matchers.query_param_matcher({"limit": "20", "offset": "40"})],
            json={"logs": [], "total": 0},
            status=200,
        )
        signed_client.get_audit_logs(limit=20, offset=40)
        request = rsps.calls[0].request
    assert _signed_path(request) == f"/api/v1/agents/{AGENT_ID}/audit-logs?limit=20&offset=40"


def test_get_audit_logs_sends_an_offset_of_zero(signed_client):
    with responses.RequestsMock() as rsps:
        rsps.add(
            responses.GET,
            ROUTE,
            match=[responses.matchers.query_param_matcher({"offset": "0"})],
            json={"logs": [], "total": 0},
            status=200,
        )
        signed_client.get_audit_logs(offset=0)
        request = rsps.calls[0].request
    assert _signed_path(request) == f"/api/v1/agents/{AGENT_ID}/audit-logs?offset=0"


def test_get_audit_logs_takes_another_agent_id(signed_client):
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, f"{AIM_URL}/api/v1/agents/{OTHER_ID}/audit-logs", json={"logs": []}, status=200)
        assert signed_client.get_audit_logs(OTHER_ID) == {"logs": []}


@pytest.mark.parametrize("limit", [0, -1, True, 2.5, "10"])
def test_get_audit_logs_refuses_a_limit_that_is_not_a_positive_integer(signed_client, limit):
    with responses.RequestsMock():
        with pytest.raises(ConfigurationError, match="limit must be a positive integer"):
            signed_client.get_audit_logs(limit=limit)


@pytest.mark.parametrize("offset", [-1, True, 1.5, "0"])
def test_get_audit_logs_refuses_an_offset_that_is_not_a_non_negative_integer(signed_client, offset):
    with responses.RequestsMock():
        with pytest.raises(ConfigurationError, match="offset must be a non-negative integer"):
            signed_client.get_audit_logs(offset=offset)


@pytest.mark.parametrize("status, error", [(401, AuthenticationError), (403, AuthenticationError), (404, VerificationError)])
def test_get_audit_logs_raises_the_sdk_errors(signed_client, status, error):
    with responses.RequestsMock() as rsps:
        rsps.add(responses.GET, f"{AIM_URL}/api/v1/agents/{OTHER_ID}/audit-logs", json={"error": "x"}, status=status)
        with pytest.raises(error):
            signed_client.get_audit_logs(OTHER_ID)
