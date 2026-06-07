"""
Tests that AIMClient wires causal-denial telemetry correctly: a correlation
header is attached to the verify request, the enforcement outcome (allow AND
deny) is fed to the joiner, intent/detection seams flow through, and telemetry
is completely inert when not opted in.
"""
import pytest

from aim_sdk import AIMClient
from aim_sdk.exceptions import ActionDeniedError
from aim_sdk.telemetry import (
    CORRELATION_HEADER,
    CorrelationJoiner,
    DetectionInput,
    IntentInput,
    is_correlation_id,
)


class _FakeResponse:
    def __init__(self, status_code, payload):
        self.status_code = status_code
        self._payload = payload
        self.text = ""

    def json(self):
        return self._payload

    def raise_for_status(self):
        return None


class _SessionSpy:
    """Records the headers of the last request and returns a canned response."""

    def __init__(self, payload):
        self.payload = payload
        self.last_headers = None
        self.headers = {}

    def request(self, method, url, json=None, headers=None, data=None, timeout=None):
        self.last_headers = headers or {}
        return _FakeResponse(200, self.payload)

    def close(self):
        pass


def _client_with_capturing_joiner(payload, enabled=True):
    records = []
    joiner = CorrelationJoiner(now=lambda: 0, on_record=lambda r: records.append(r))
    telemetry = {"enabled": enabled, "joiner": joiner} if enabled else None
    c = AIMClient(agent_id="agent-1", api_key="k", aim_url="http://localhost:9",
                  telemetry=telemetry)
    c.session = _SessionSpy(payload)
    return c, records


def test_allow_feeds_enforcement_and_attaches_header():
    c, records = _client_with_capturing_joiner({"id": "v1", "status": "approved"})
    intent = IntentInput(intent_class="read", confidence=0.9, blocked=False, source="nm")
    detection = DetectionInput(injection_detected=False, confidence=0.1, detector="guard",
                               technique_source="interim-mapping")
    res = c.verify_capability("db:read", resource="users",
                              telemetry={"intent": intent, "detection": detection})
    assert res["verified"] is True
    # correlation header attached to the verify request
    cid = c.session.last_headers.get(CORRELATION_HEADER)
    assert is_correlation_id(cid)
    # full record assembled (enforcement + intent + detection)
    assert len(records) == 1
    rec = records[0]
    assert rec.enforcement.decision == "allow"
    assert rec.enforcement.observed is True
    assert rec.assembly.completeness == "full"
    c.close()


def test_deny_records_before_raise():
    c, records = _client_with_capturing_joiner({"id": "v1", "status": "denied",
                                                "denial_reason": "policy X"})
    with pytest.raises(ActionDeniedError):
        c.verify_capability("db:write", resource="secrets")
    assert len(records) == 0  # enforcement-only buffered; not yet flushed
    # but the enforcement fact was ingested (pending in the joiner)
    assert c._supplied_joiner.pending == 1
    c.close()


def test_telemetry_off_attaches_no_header_and_no_records():
    c, _ = _client_with_capturing_joiner({"id": "v1", "status": "approved"}, enabled=False)
    res = c.verify_capability("db:read", resource="users")
    assert res["verified"] is True
    assert CORRELATION_HEADER not in (c.session.last_headers or {})
    assert c._get_joiner() is None
    c.close()


def test_deny_then_window_flush_emits_partial():
    # Drive the deny path, then expire the window to confirm a partial emits.
    records = []
    clock = {"t": 0}
    joiner = CorrelationJoiner(window_ms=1000, now=lambda: clock["t"],
                              on_record=lambda r: records.append(r))
    c = AIMClient(agent_id="agent-1", api_key="k", aim_url="http://localhost:9",
                  telemetry={"enabled": True, "joiner": joiner})
    c.session = _SessionSpy({"id": "v1", "status": "denied", "denial_reason": "blocked"})
    with pytest.raises(ActionDeniedError):
        c.verify_capability("net:connect", resource="evil.example")
    clock["t"] = 2000
    joiner.flush_expired()
    assert len(records) == 1
    assert records[0].enforcement.decision == "deny"
    assert records[0].assembly.completeness == "partial-missing-both"
    c.close()
