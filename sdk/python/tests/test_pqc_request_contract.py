"""
The PQC write calls send the request members the backend reads.

``register_pqc_key``, ``rotate_pqc_key`` and ``set_hybrid_mode`` sent
``pqcPublicKey``/``enableHybridMode``, ``newPqcPublicKey`` and ``enabled``,
while the backend's request types read ``publicKey``/``enableHybrid``,
``newPublicKey`` and ``enable``. The key arrived empty, so registering and
rotating always failed, and ``set_hybrid_mode(True)`` turned hybrid mode off.

Each route's body lives in ``tests/fixtures/pqc_requests/``. A Go handler test
reads the same files, so a member renamed on either side fails a test.
"""

import inspect
import json
import os

import pytest
import responses

from aim_sdk import AIMClient
from aim_sdk.exceptions import ConfigurationError

AIM_URL = "http://aim.test"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
FIXTURES = os.path.join(os.path.dirname(__file__), "fixtures", "pqc_requests")
PQC_WRITES = ("register_pqc_key", "rotate_pqc_key", "set_hybrid_mode")


def _fixture(name):
    with open(os.path.join(FIXTURES, f"{name}.json")) as f:
        return json.load(f)


@pytest.fixture
def client():
    return AIMClient(agent_id=AGENT_ID, aim_url=AIM_URL, api_key="aim_live_test_not_real")


def _send(client, name):
    """Call one PQC write the way the fixture's body describes; return the request."""
    fx = _fixture(name)
    body = fx["body"]
    url = AIM_URL + fx["path"].replace("{id}", AGENT_ID)
    with responses.RequestsMock() as rsps:
        rsps.add(fx["method"], url, json={}, status=200)
        if name == "register_pqc_key":
            client.register_pqc_key(
                pqc_public_key=body["publicKey"],
                algorithm=body["algorithm"],
                enable_hybrid_mode=body["enableHybrid"],
            )
        elif name == "rotate_pqc_key":
            client.rotate_pqc_key(
                new_pqc_public_key=body["newPublicKey"],
                algorithm=body["algorithm"],
            )
        else:
            client.set_hybrid_mode(enabled=body["enable"])
        assert len(rsps.calls) == 1
        return fx, rsps.calls[0].request


@pytest.mark.parametrize("name", PQC_WRITES)
def test_sends_exactly_the_members_the_server_reads(client, name):
    fx, request = _send(client, name)
    assert request.method == fx["method"]
    assert json.loads(request.body) == fx["body"]


def test_register_pqc_key_defaults_to_no_hybrid_mode(client):
    default = inspect.signature(AIMClient.register_pqc_key).parameters["enable_hybrid_mode"].default
    assert default is False

    body = _fixture("register_pqc_key")["body"]
    url = f"{AIM_URL}/api/v1/agents/{AGENT_ID}/pqc-key"
    with responses.RequestsMock() as rsps:
        rsps.add(responses.POST, url, json={}, status=200)
        client.register_pqc_key(pqc_public_key=body["publicKey"])
        assert json.loads(rsps.calls[0].request.body)["enableHybrid"] is False


@pytest.mark.parametrize(
    "call",
    [
        lambda c: c.register_pqc_key(
            pqc_public_key=_fixture("register_pqc_key")["body"]["publicKey"],
            enable_hybrid_mode=True,
        ),
        lambda c: c.set_hybrid_mode(True),
    ],
    ids=["register_pqc_key", "set_hybrid_mode"],
)
def test_turning_hybrid_mode_on_raises_before_any_request(client, call):
    # No route is registered: any request the call made would be recorded.
    with responses.RequestsMock(assert_all_requests_are_fired=False) as rsps:
        with pytest.raises(ConfigurationError) as excinfo:
            call(client)
        assert len(rsps.calls) == 0
    message = str(excinfo.value)
    assert "no AIM SDK signs requests in that form" in message
    assert "enable_hybrid_mode=False" in message


@pytest.mark.parametrize("name", PQC_WRITES)
def test_docstrings_do_not_recommend_hybrid_mode_and_name_the_credential(name):
    doc = inspect.getdoc(getattr(AIMClient, name))
    for line in doc.lower().splitlines():
        assert not ("hybrid" in line and "recommended" in line), line
    assert "access token with role member, manager or admin" in " ".join(doc.split())
