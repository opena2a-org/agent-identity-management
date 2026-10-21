"""
``register_mcp``, ``report_execution_status`` and ``_register_single_mcp`` send
the request members the backend's handlers bind.

Each route's body lives in ``tests/fixtures/sdk_requests/``. A Go handler test
reads the same files, decodes each body into the request type its handler
binds and refuses a member the type does not declare, so a member renamed on
either side fails a test.
"""

import json
import os
from datetime import datetime

import pytest
import responses

import aim_sdk.client as client_module
from aim_sdk import AIMClient
from aim_sdk.client import _register_single_mcp

AIM_URL = "http://aim.test"
FIXTURES = os.path.join(os.path.dirname(__file__), "fixtures", "sdk_requests")
VERIFICATION_ID = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"


def _fixture(name):
    with open(os.path.join(FIXTURES, f"{name}.json")) as f:
        return json.load(f)


# The agent ID both this test and the Go test put in the path.
AGENT_ID = _fixture("register_single_mcp")["body"]["registeredByAgent"]


@pytest.fixture
def client():
    return AIMClient(
        agent_id=AGENT_ID,
        aim_url=AIM_URL,
        api_key="not-a-real-key",
        timeout=5,
        auto_retry=False,
    )


def _route(fx, path_id):
    return AIM_URL + fx["path"].replace("{id}", path_id)


def _only_request(rsps):
    assert len(rsps.calls) == 1
    return rsps.calls[0].request


def test_register_mcp_sends_the_fixture_body(client):
    fx = _fixture("register_mcp")
    body = fx["body"]
    with responses.RequestsMock() as rsps:
        rsps.add(fx["method"], _route(fx, AGENT_ID), json={}, status=200)
        client.register_mcp(
            mcp_server_id=body["mcpServerIds"][0],
            detection_method=body["detectedMethod"],
            confidence=body["confidence"],
            metadata=body["metadata"],
        )
        request = _only_request(rsps)
    assert request.method == fx["method"]
    assert json.loads(request.body) == body


def test_report_execution_status_sends_the_fixture_body(client, monkeypatch):
    fx = _fixture("report_execution_status")
    body = fx["body"]
    executed_at = datetime.fromisoformat(body["executedAt"])

    class FixedClock(datetime):
        @classmethod
        def now(cls, tz=None):
            return executed_at.astimezone(tz)

    monkeypatch.setattr(client_module, "datetime", FixedClock)
    with responses.RequestsMock() as rsps:
        rsps.add(fx["method"], _route(fx, VERIFICATION_ID), json={"status": "execution_status_recorded"}, status=200)
        client.report_execution_status(
            VERIFICATION_ID,
            executed=body["executed"],
            strict_mode=True,
            execution_error=body["executionError"],
        )
        request = _only_request(rsps)
    assert request.method == fx["method"]
    assert json.loads(request.body) == body


def test_mcp_registration_sends_the_fixture_body(client):
    fx = _fixture("register_single_mcp")
    body = fx["body"]
    mcp_def = {member: body[member] for member in ("name", "description", "url", "version", "capabilities")}
    with responses.RequestsMock() as rsps:
        rsps.add(fx["method"], _route(fx, AGENT_ID), json={"id": "mcp-001", "name": body["name"]}, status=201)
        result = _register_single_mcp(
            client=client,
            aim_url=AIM_URL,
            headers={},
            mcp_def=mcp_def,
            agent_id=AGENT_ID,
            detected_capabilities={},
        )
        request = _only_request(rsps)
    assert result["status"] == "registered", result
    assert request.method == fx["method"]
    assert json.loads(request.body) == body
