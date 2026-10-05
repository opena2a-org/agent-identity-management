"""
report_capabilities registers each capability through the route that respects
the organization's enforcement mode, and reads the outcome the server returns.

The stand-in server answers the way the backend's router does: a request is
served only when its method and path are in the SDK-API route table, read here
from the backend source, and gets a 404 otherwise. The registration route is
answered the way RegisterCapability answers it (201 granted in monitoring mode,
202 pending with a request id in strict mode, 409 already_exists or pending on
a repeat), and the legacy grant route the way GrantCapability answers it (201
with the granted capability, whatever the enforcement mode).
"""

import json
import re
import uuid
import warnings
from pathlib import Path

import pytest
import responses

from aim_sdk import AIMClient
from aim_sdk.exceptions import ConfigurationError, VerificationError


AIM_URL = "https://aim.example.com"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
REGISTER_PATH = f"/api/v1/sdk-api/agents/{AGENT_ID}/capabilities/register"

ROUTE_TABLE = Path(__file__).resolve().parents[3] / "apps/backend/cmd/server/sdk_api_routes.go"
_STRING_CONSTANT = re.compile(r'(\w+)\s*=\s*"([^"]*)"')
_ROUTE_ENTRY = re.compile(r'Method:\s*http\.Method(\w+),\s*Path:\s*(?:"([^"]+)"|(\w+))')


def _read_registered_routes():
    """Every (METHOD, full path) of the backend's SDK-API route table."""
    assert ROUTE_TABLE.is_file(), (
        f"{ROUTE_TABLE} was not found. This test reads the backend's route table "
        "and has to run inside the repository."
    )
    source = ROUTE_TABLE.read_text(encoding="utf-8")
    constants = dict(_STRING_CONSTANT.findall(source))
    base = constants.get("sdkAPIBasePath")
    assert base, f"sdkAPIBasePath is not declared in {ROUTE_TABLE}"
    routes = []
    for method, literal, constant in _ROUTE_ENTRY.findall(source):
        relative = literal or constants.get(constant)
        assert relative, f"route path {constant} is not a string constant in {ROUTE_TABLE}"
        segments = [s for s in (base + relative).split("/") if s]
        pattern = "".join(
            "/[^/]+" if s.startswith(":") else "/" + re.escape(s) for s in segments
        )
        routes.append((method.upper(), base + relative, re.compile(pattern + "$")))
    assert routes, f"no route entries found in {ROUTE_TABLE}"
    return routes


ROUTES = _read_registered_routes()


def _route_for(method, path):
    for route_method, route_path, matcher in ROUTES:
        if route_method == method and matcher.match(path):
            return route_path
    return None


class FakeAIM:
    """Serves the SDK-API routes with the backend's capability semantics."""

    def __init__(self, mode="monitoring", fail=None):
        self.mode = mode
        # capability type -> (status code, body) to answer instead of the handler
        self.fail = fail or {}
        self.granted = set()
        self.pending = set()
        self.requests = []

    def __call__(self, request):
        path = request.path_url.split("?", 1)[0]
        body = json.loads(request.body) if request.body else None
        self.requests.append((request.method, path, body))
        route = _route_for(request.method, path)
        if route is None:
            return self._json(404, {"error": f"Cannot {request.method} {path}"})
        if route.endswith("/agents/:id/capabilities/register"):
            return self._register(body)
        if route.endswith("/agents/:id/capabilities"):
            # GrantCapability: grants directly, never consults enforcement mode.
            cap = body.get("capabilityType")
            self.granted.add(cap)
            return self._json(201, {"id": str(uuid.uuid4()), "capabilityType": cap})
        return self._json(200, {})

    def _register(self, body):
        cap = (body or {}).get("capabilityType", "")
        if cap in self.fail:
            status, payload = self.fail[cap]
            return self._json(status, payload)
        if not cap:
            return self._json(400, {"error": "capabilityType is required"})
        if cap in self.granted:
            return self._json(409, {
                "success": True, "capabilityType": cap, "status": "already_exists",
                "message": "Capability already granted to this agent",
            })
        if self.mode == "monitoring":
            self.granted.add(cap)
            return self._json(201, {
                "success": True, "capabilityType": cap, "status": "granted",
                "message": "Capability auto-granted (monitoring mode)",
            })
        if cap in self.pending:
            return self._json(409, {
                "success": True, "capabilityType": cap, "status": "pending",
                "message": "A pending request for this capability already exists",
            })
        self.pending.add(cap)
        return self._json(202, {
            "success": True, "capabilityType": cap, "status": "pending",
            "message": "Capability request created - awaiting admin approval (strict mode)",
            "requestId": f"req-{cap}",
        })

    @staticmethod
    def _json(status, payload):
        return (status, {"Content-Type": "application/json"}, json.dumps(payload))


@pytest.fixture
def client():
    return AIMClient(
        agent_id=AGENT_ID,
        aim_url=AIM_URL,
        api_key="aim_test_key_12345",
        timeout=5,
        auto_retry=False,
    )


def _serve(fake):
    responses.add_callback(
        responses.POST, re.compile(re.escape(AIM_URL) + r"/.*"), callback=fake
    )


def test_route_table_is_read_from_backend_source():
    # Guards the reader: a pattern that stopped matching the table would turn
    # every request into a 404 for the wrong reason.
    assert _route_for("POST", REGISTER_PATH) is not None
    assert _route_for("POST", f"/api/v1/sdk-api/agents/{AGENT_ID}/not-a-route") is None


@responses.activate
def test_monitoring_mode_grants_each_distinct_capability_once_in_order(client):
    fake = FakeAIM(mode="monitoring")
    _serve(fake)

    result = client.report_capabilities(["db:read", "api:call", "db:read"])

    assert fake.requests == [
        ("POST", REGISTER_PATH, {"capabilityType": "db:read"}),
        ("POST", REGISTER_PATH, {"capabilityType": "api:call"}),
    ]
    assert result["granted"] == 2
    assert result["pending"] == 0
    assert result["total"] == 2
    assert [(r["capability_type"], r["status"]) for r in result["results"]] == [
        ("db:read", "granted"),
        ("api:call", "granted"),
    ]


@responses.activate
def test_strict_mode_reports_pending_with_request_id_and_grants_nothing(client):
    fake = FakeAIM(mode="strict")
    _serve(fake)

    result = client.report_capabilities(["db:write"])

    assert fake.granted == set(), "a strict-mode report must not grant a capability"
    assert result["granted"] == 0
    assert result["pending"] == 1
    assert result["total"] == 1
    assert result["results"] == [{
        "capability_type": "db:write",
        "status": "pending",
        "message": "Capability request created - awaiting admin approval (strict mode)",
        "request_id": "req-db:write",
    }]


@responses.activate
def test_a_second_call_reads_both_409_outcomes(client):
    fake = FakeAIM(mode="strict")
    fake.granted.add("file:read")  # granted earlier, e.g. at registration
    _serve(fake)

    client.report_capabilities(["db:write"])
    result = client.report_capabilities(["file:read", "db:write"])

    assert [(r["capability_type"], r["status"]) for r in result["results"]] == [
        ("file:read", "already_exists"),
        ("db:write", "pending"),
    ]
    assert "request_id" not in result["results"][1]
    assert result["granted"] == 1
    assert result["pending"] == 1
    assert result["total"] == 2


@responses.activate
def test_a_503_raises(client):
    fake = FakeAIM(fail={"db:read": (503, {"error": "Service unavailable"})})
    _serve(fake)

    with pytest.raises(VerificationError, match="503"):
        client.report_capabilities(["db:read"])


@responses.activate
def test_a_500_raises_and_is_not_counted_as_a_grant(client):
    fake = FakeAIM(fail={"api:call": (500, {"error": "duplicate key value violates unique constraint"})})
    _serve(fake)

    with pytest.raises(VerificationError, match="500"):
        client.report_capabilities(["db:read", "api:call", "file:read"])

    # Stops at the failure: the capability after it is not sent.
    assert [body["capabilityType"] for _, _, body in fake.requests] == ["db:read", "api:call"]


@responses.activate
def test_a_refusal_raises(client):
    fake = FakeAIM(fail={"db:read": (404, {"error": "Agent not found"})})
    _serve(fake)

    with pytest.raises(VerificationError, match="404"):
        client.report_capabilities(["db:read"])


@responses.activate
def test_an_answer_without_a_known_status_raises(client):
    fake = FakeAIM(fail={"db:read": (200, {"success": True})})
    _serve(fake)

    with pytest.raises(VerificationError, match="db:read"):
        client.report_capabilities(["db:read"])


@responses.activate
def test_scope_is_accepted_with_a_deprecation_warning_and_not_sent(client):
    fake = FakeAIM(mode="monitoring")
    _serve(fake)

    with pytest.warns(DeprecationWarning, match="scope"):
        result = client.report_capabilities(["db:read"], scope={"source": "test"})

    assert fake.requests == [("POST", REGISTER_PATH, {"capabilityType": "db:read"})]
    assert result["granted"] == 1


@responses.activate
def test_no_scope_no_warning(client):
    _serve(FakeAIM(mode="monitoring"))

    with warnings.catch_warnings():
        warnings.simplefilter("error", DeprecationWarning)
        client.report_capabilities(["db:read"])


@responses.activate
def test_an_empty_capability_is_refused_before_anything_is_sent(client):
    fake = FakeAIM(mode="monitoring")
    _serve(fake)

    with pytest.raises(ConfigurationError):
        client.report_capabilities(["db:read", ""])

    assert fake.requests == []


@responses.activate
def test_an_empty_list_sends_nothing(client):
    fake = FakeAIM(mode="monitoring")
    _serve(fake)

    assert client.report_capabilities([]) == {
        "granted": 0, "pending": 0, "total": 0, "results": [],
    }
    assert fake.requests == []
