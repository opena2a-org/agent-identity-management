"""
Tests for the SDK -> Registry relay. Key guarantees: inert when disabled;
uploads only denied_injection_attempt indicators; ships NO identifiers; the
cursor advances so records are not re-sent; failures block the cursor (retry)
while 4xx rejections advance it; nothing here ever raises.
"""
import json
import os
import stat
import tempfile

import pytest

from aim_sdk.telemetry import (
    CorrelatedRelay,
    DetectionInput,
    EnforcementInput,
    IntentInput,
    SENTINEL_PACKAGE_NAME,
    build_correlated_record,
    derive_sensor_token,
    write_correlated_record,
)
from aim_sdk.telemetry.relay import open_a2a_home


def _make_record(created_at, injection, decision=None):
    decision = decision or ("deny" if injection else "allow")
    rec = build_correlated_record(
        f"cde_000000000_{os.urandom(6).hex()}",
        "agent-secret-id",
        EnforcementInput(
            decision=decision,
            outcome="DENY_INTENT" if decision == "deny" else "ALLOW",
            capability="net:connect",
            resource="https://evil.example/secret-path",
            source="aim-pdp",
            denied_reason="blocked because injected exfil instruction",
            credential_ref="ref:db-prod",
        ),
        intent=IntentInput(intent_class="exfiltration", confidence=0.7, blocked=True,
                           source="nanomind-intent") if injection else None,
        detection=DetectionInput(injection_detected=True, confidence=0.84,
                                 detector="nanomind-guard", technique_source="interim-mapping",
                                 technique_id="T-2002", attack_class="indirect",
                                 input_ref="sha256:abcd") if injection else None,
    )
    rec.created_at = created_at
    return rec


class _Capture:
    """Injectable transport capturing POST bodies and returning a fixed status."""

    def __init__(self, status=200):
        self.status = status
        self.bodies = []

    def __call__(self, url, body, timeout):
        self.bodies.append(body)
        if isinstance(self.status, Exception):
            raise self.status
        return self.status


def test_inert_when_disabled():
    with tempfile.TemporaryDirectory() as d:
        write_correlated_record(_make_record("2026-06-06T00:00:00.000Z", injection=True), d)
        cap = _Capture()
        relay = CorrelatedRelay(enabled=False, data_dir=d, transport=cap)
        res = relay.flush_once()
        assert res.considered == 0 and res.uploaded == 0
        assert cap.bodies == []


def test_uploads_only_denied_injection_and_strips_identifiers():
    with tempfile.TemporaryDirectory() as d:
        write_correlated_record(_make_record("2026-06-06T00:00:00.000Z", injection=True), d)
        write_correlated_record(_make_record("2026-06-06T00:00:01.000Z", injection=False), d)
        cap = _Capture(200)
        relay = CorrelatedRelay(enabled=True, data_dir=d, transport=cap, package_name="mypkg")
        res = relay.flush_once()
        assert res.uploaded == 1  # only the injection-deny record
        body = cap.bodies[0]
        assert body["eventType"] == "denied_injection_attempt"
        assert body["packageName"] == "mypkg"
        assert body["runtimeEnv"] == "python"
        assert body["techniqueId"] == "T-2002"
        blob = json.dumps(body)
        for leaked in ("agent-secret-id", "evil.example", "secret-path", "ref:db-prod",
                       "sha256:abcd", "blocked because", "correlationId"):
            assert leaked not in blob


def test_default_package_name_is_sentinel():
    with tempfile.TemporaryDirectory() as d:
        write_correlated_record(_make_record("2026-06-06T00:00:00.000Z", injection=True), d)
        cap = _Capture(200)
        CorrelatedRelay(enabled=True, data_dir=d, transport=cap).flush_once()
        assert cap.bodies[0]["packageName"] == SENTINEL_PACKAGE_NAME


def test_cursor_advances_no_resend():
    with tempfile.TemporaryDirectory() as d:
        write_correlated_record(_make_record("2026-06-06T00:00:00.000Z", injection=True), d)
        cap = _Capture(200)
        relay = CorrelatedRelay(enabled=True, data_dir=d, transport=cap)
        assert relay.flush_once().uploaded == 1
        # second flush sees nothing new
        res2 = relay.flush_once()
        assert res2.considered == 0 and res2.uploaded == 0
        assert len(cap.bodies) == 1


def test_5xx_blocks_cursor_and_retries():
    with tempfile.TemporaryDirectory() as d:
        write_correlated_record(_make_record("2026-06-06T00:00:00.000Z", injection=True), d)
        cap = _Capture(503)
        relay = CorrelatedRelay(enabled=True, data_dir=d, transport=cap)
        res = relay.flush_once()
        assert res.stopped_on_error is True and res.uploaded == 0
        # cursor did NOT advance: record reconsidered next flush
        cap.status = 200
        res2 = relay.flush_once()
        assert res2.uploaded == 1


def test_4xx_advances_cursor_without_counting():
    with tempfile.TemporaryDirectory() as d:
        write_correlated_record(_make_record("2026-06-06T00:00:00.000Z", injection=True), d)
        cap = _Capture(400)
        relay = CorrelatedRelay(enabled=True, data_dir=d, transport=cap)
        res = relay.flush_once()
        assert res.stopped_on_error is False and res.uploaded == 0
        # cursor advanced: not reconsidered
        cap.status = 200
        res2 = relay.flush_once()
        assert res2.considered == 0


def test_network_exception_is_treated_as_failed_and_swallowed():
    with tempfile.TemporaryDirectory() as d:
        write_correlated_record(_make_record("2026-06-06T00:00:00.000Z", injection=True), d)
        cap = _Capture(RuntimeError("connection refused"))
        relay = CorrelatedRelay(enabled=True, data_dir=d, transport=cap)
        res = relay.flush_once()  # must not raise
        assert res.stopped_on_error is True and res.uploaded == 0


def test_egress_validation_omits_tampered_technique_fields():
    with tempfile.TemporaryDirectory() as d:
        rec = _make_record("2026-06-06T00:00:00.000Z", injection=True)
        # tamper the on-disk record's technique fields with free-text smuggling
        rec.detection.technique_id = "T-BAD; DROP TABLE"
        rec.detection.technique_source = "evil-source"
        rec.detection.confidence = 5.0  # out of [0,1]
        write_correlated_record(rec, d)
        cap = _Capture(200)
        CorrelatedRelay(enabled=True, data_dir=d, transport=cap).flush_once()
        body = cap.bodies[0]
        assert "techniqueId" not in body
        assert "techniqueSource" not in body
        assert "detectionConfidence" not in body


def test_sensor_token_is_stable_and_anonymous():
    with tempfile.TemporaryDirectory() as d:
        t1 = derive_sensor_token(d)
        t2 = derive_sensor_token(d)
        assert t1 == t2  # persisted salt -> stable
        assert len(t1) == 64  # sha256 hex


@pytest.mark.skipif(not hasattr(os, "getuid"), reason="POSIX perm semantics only")
def test_salt_file_is_owner_only():
    with tempfile.TemporaryDirectory() as d:
        derive_sensor_token(d)
        salt_path = os.path.join(d, "gtin-sensor-salt")
        mode = stat.S_IMODE(os.stat(salt_path).st_mode)
        assert mode & 0o077 == 0  # no group/other bits


@pytest.mark.skipif(not hasattr(os, "getuid"), reason="POSIX perm semantics only")
def test_untrusted_salt_is_regenerated():
    with tempfile.TemporaryDirectory() as d:
        salt_path = os.path.join(d, "gtin-sensor-salt")
        with open(salt_path, "w", encoding="utf-8") as fh:
            fh.write("attacker-planted-salt")
        os.chmod(salt_path, 0o644)  # world-readable -> untrusted
        token = derive_sensor_token(d)
        # salt was replaced; token does not derive from the planted salt
        import hashlib
        import socket
        import getpass
        planted = hashlib.sha256(
            f"{socket.gethostname()}|{getpass.getuser()}|attacker-planted-salt".encode()
        ).hexdigest()
        assert token != planted
        assert stat.S_IMODE(os.stat(salt_path).st_mode) & 0o077 == 0


def test_open_a2a_home_ignores_relative_env(monkeypatch):
    monkeypatch.setenv("OPENA2A_HOME", "relative/path")
    assert open_a2a_home().endswith(os.path.join(".opena2a"))
    monkeypatch.setenv("OPENA2A_HOME", "/abs/../abs2")
    assert open_a2a_home() == os.path.normpath("/abs/../abs2")
