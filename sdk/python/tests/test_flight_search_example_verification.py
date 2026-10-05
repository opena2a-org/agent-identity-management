"""
The flight-search example (examples/flight-search-agent/flight_agent.py) must
refuse a search AIM did not verify, and must report a verified search's
execution on a route the server answers.

Two defects this pins:

- A verification error printed "Proceeding without verification" and ran the
  search anyway, so a failed verification left no AIM record and the agent
  still acted.
- The result report read ``audit_id`` from verify_capability's return, a key
  it never carries, so the report never ran. The report also targeted
  ``/verifications/{id}/result``, which the server refuses for every caller;
  the live route is ``/verifications/{id}/execution-status``.

Everything runs offline against a loopback fake this file starts itself. The
fake does not verify signatures; it answers with the bodies the server's
CreateVerification and UpdateExecutionStatus handlers return.
"""

import base64
import importlib.util
import json
import sys
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

from aim_sdk.client import AIMClient
from aim_sdk.exceptions import ActionDeniedError, VerificationUnavailableError

EXAMPLE = (
    Path(__file__).resolve().parents[3]
    / "examples" / "flight-search-agent" / "flight_agent.py"
)

VERIFY_PATH = "/api/v1/sdk-api/verifications"
VID = "6f1c2b9e-0d4a-4c2e-9a51-3b7d8e2f1a00"
EXECUTION_PATH = f"{VERIFY_PATH}/{VID}/execution-status"
WITHDRAWN_RESULT_PATH = f"{VERIFY_PATH}/{VID}/result"


class _FakeAIM:
    """Loopback fake: canned answers per (method, path), every request recorded."""

    def __init__(self, routes):
        self.routes = routes
        self.requests = []  # [(method, path, body, status_served)]
        fake = self

        class Handler(BaseHTTPRequestHandler):
            def _serve(self):
                length = int(self.headers.get("Content-Length") or 0)
                raw = self.rfile.read(length) if length else b""
                body = json.loads(raw.decode("utf-8")) if raw else None
                status, payload = fake.routes.get(
                    (self.command, self.path), (404, {"error": "not found"})
                )
                fake.requests.append((self.command, self.path, body, status))
                data = json.dumps(payload).encode("utf-8")
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            do_GET = do_POST = _serve

            def log_message(self, *args):
                pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self.server.server_address[1]}"
        threading.Thread(target=self.server.serve_forever, daemon=True).start()

    def paths(self):
        return [path for _, path, _, _ in self.requests]

    def close(self):
        self.server.shutdown()
        self.server.server_close()


class _Untouchable(dict):
    """MOCK_FLIGHTS stand-in: reaching the flight lookup fails the test."""

    def get(self, *args, **kwargs):
        raise AssertionError("the search ran although AIM did not verify it")


@pytest.fixture(scope="module")
def flight_agent_module():
    saved_path = list(sys.path)
    spec = importlib.util.spec_from_file_location("_flight_agent_example", EXAMPLE)
    module = importlib.util.module_from_spec(spec)
    try:
        spec.loader.exec_module(module)
    finally:
        sys.path[:] = saved_path  # the example prepends SDK paths on import
    return module


@pytest.fixture
def fake_aim():
    servers = []

    def start(routes):
        server = _FakeAIM(routes)
        servers.append(server)
        return server

    yield start
    for server in servers:
        server.close()


def _agent(module, monkeypatch, url):
    """A FlightAgent whose registration returns a client pointed at the fake."""
    from nacl.encoding import Base64Encoder
    from nacl.signing import SigningKey

    signing_key = SigningKey.generate()
    client = AIMClient(
        agent_id="7a0e0e52-1111-4222-8333-444455556666",
        public_key=signing_key.verify_key.encode(encoder=Base64Encoder).decode("utf-8"),
        private_key=base64.b64encode(bytes(signing_key)).decode("utf-8"),
        aim_url=url,
        auto_retry=False,
        telemetry={"enabled": False},
    )
    monkeypatch.setattr(module, "secure", lambda *args, **kwargs: client)
    agent = module.FlightAgent()
    assert agent.client is client
    return agent


def test_verification_error_refuses_the_search(flight_agent_module, monkeypatch, fake_aim):
    aim = fake_aim({("POST", VERIFY_PATH): (500, {"error": "database unavailable"})})
    agent = _agent(flight_agent_module, monkeypatch, aim.url)
    monkeypatch.setattr(flight_agent_module, "MOCK_FLIGHTS", _Untouchable())

    with pytest.raises(VerificationUnavailableError):
        agent.search_flights("NYC")

    assert aim.paths() == [VERIFY_PATH]


def test_denial_refuses_the_search_and_records_that_it_did_not_run(
    flight_agent_module, monkeypatch, fake_aim
):
    aim = fake_aim({
        ("POST", VERIFY_PATH): (403, {
            "id": VID,
            "status": "denied",
            "denialReason": "agent has no grant for flights:search",
            "enforcementMode": "strict",
        }),
        ("POST", EXECUTION_PATH): (200, {"success": True}),
    })
    agent = _agent(flight_agent_module, monkeypatch, aim.url)
    monkeypatch.setattr(flight_agent_module, "MOCK_FLIGHTS", _Untouchable())

    with pytest.raises(ActionDeniedError):
        agent.search_flights("NYC")

    assert aim.paths() == [VERIFY_PATH, EXECUTION_PATH]
    _, _, body, status = aim.requests[1]
    assert body["executed"] is False
    assert 200 <= status < 300


@pytest.mark.parametrize("destination", ["NYC", "ZZZ"])
def test_verified_search_reports_its_execution_and_is_answered_2xx(
    flight_agent_module, monkeypatch, fake_aim, destination
):
    aim = fake_aim({
        ("POST", VERIFY_PATH): (200, {
            "id": VID,
            "status": "approved",
            "enforcementMode": "strict",
        }),
        ("POST", EXECUTION_PATH): (200, {"success": True}),
    })
    agent = _agent(flight_agent_module, monkeypatch, aim.url)

    flights = agent.search_flights(destination)

    expected = flight_agent_module.MOCK_FLIGHTS.get(destination, [])
    assert len(flights) == len(expected)
    assert aim.paths() == [VERIFY_PATH, EXECUTION_PATH]
    assert WITHDRAWN_RESULT_PATH not in aim.paths()
    _, _, body, status = aim.requests[1]
    assert body["executed"] is True
    assert 200 <= status < 300


def test_interactive_search_prints_a_refusal_once(
    flight_agent_module, monkeypatch, fake_aim, capsys
):
    aim = fake_aim({("POST", VERIFY_PATH): (500, {"error": "database unavailable"})})
    agent = _agent(flight_agent_module, monkeypatch, aim.url)
    monkeypatch.setattr(flight_agent_module, "MOCK_FLIGHTS", _Untouchable())
    commands = iter(["search NYC", "quit"])
    monkeypatch.setattr("builtins.input", lambda prompt="": next(commands))
    capsys.readouterr()

    agent.interactive_mode()

    out = capsys.readouterr().out
    assert out.count("Search refused") == 1
    assert "Error:" not in out
    assert "Proceeding without verification" not in out
