#!/usr/bin/env python3
"""
Local end-to-end smoke for the Python SDK causal-denial telemetry relay.

Mirrors the TS sdk's /tmp/relay-smoke.cjs: runs the REAL AIMClient with BOTH
opt-ins (capture + relay), drives a denied verification with an injection
detection, lets the joiner assemble + persist the full record, then has the
real relay POST the anonymized indicator to a LOCAL capture server. Asserts:

  - exactly one indicator lands, eventType=denied_injection_attempt
  - NO identifiers are present on the wire (agentId, resource, capability,
    correlationId, credentialRef, inputRef, denial reason text)
  - the registry-required count-only fields are present and well-formed

ZERO external side effects: the AIM verify call is mocked (no live backend), and
the relay targets http://127.0.0.1:<ephemeral> not the real Registry.

Run:  python tools/relay_smoke.py
Exit: 0 = pass, 1 = fail.
"""
import json
import sys
import tempfile
import threading
import time
from http.server import BaseHTTPRequestHandler, HTTPServer

# Import the installed/local SDK.
from aim_sdk import AIMClient
from aim_sdk.exceptions import ActionDeniedError
from aim_sdk.telemetry import DetectionInput, IntentInput, read_correlated_records

_received = []


class _Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length) if length else b"{}"
        try:
            _received.append(json.loads(raw.decode("utf-8")))
        except Exception:
            _received.append({"_unparseable": raw.decode("utf-8", "replace")})
        self.send_response(201)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(b'{"eventId":"smoke-1"}')

    def log_message(self, *args):  # silence default logging
        pass


class _FakeDeniedResponse:
    status_code = 200
    text = ""

    def json(self):
        return {"id": "verif-1", "status": "denied", "denial_reason": "injected exfil blocked"}

    def raise_for_status(self):
        return None


class _MockSession:
    """Stands in for the AIM verify backend; returns a deny verdict."""

    headers = {}
    last_headers = None

    def request(self, method, url, json=None, headers=None, data=None, timeout=None):
        self.last_headers = headers or {}
        return _FakeDeniedResponse()

    def close(self):
        pass


def _fail(msg):
    print(f"FAIL: {msg}")
    sys.exit(1)


def main():
    server = HTTPServer(("127.0.0.1", 0), _Handler)
    port = server.server_address[1]
    threading.Thread(target=server.serve_forever, daemon=True).start()

    data_dir = tempfile.mkdtemp(prefix="cde-py-smoke-")
    print(f"capture server on 127.0.0.1:{port}, dataDir={data_dir}")

    # BOTH opt-ins: capture (telemetry.enabled) AND relay sharing (relay.enabled).
    client = AIMClient(
        agent_id="agent-secret-id",
        api_key="k",
        aim_url="http://localhost:9",
        telemetry={
            "enabled": True,
            "relay": {
                "enabled": True,
                "registry_url": f"http://127.0.0.1:{port}",
                "data_dir": data_dir,
                "package_name": "smoke-pkg",
                "agent_category": "ci-smoke",
                # Fast flush so the managed timer fires quickly if we rely on it.
                "interval_ms": 200,
            },
        },
    )
    mock = _MockSession()
    client.session = mock

    intent = IntentInput(intent_class="exfiltration", confidence=0.7, blocked=True, source="nanomind-intent")
    detection = DetectionInput(
        injection_detected=True, confidence=0.84, detector="nanomind-guard",
        technique_source="interim-mapping", technique_id="T-2002", attack_class="indirect",
        input_ref="sha256:secrethash",
    )

    try:
        client.verify_capability(
            "net:connect", resource="https://evil.example/secret-path",
            telemetry={"intent": intent, "detection": detection},
        )
        _fail("expected ActionDeniedError on a denied verification")
    except ActionDeniedError:
        pass  # expected -- the deny path is the causal-denial case

    # The correlation header must have been attached to the verify request.
    from aim_sdk.telemetry import CORRELATION_HEADER, is_correlation_id
    if not is_correlation_id((mock.last_headers or {}).get(CORRELATION_HEADER)):
        _fail("correlation header missing/malformed on verify request")

    # Wait for the joiner's deferred write to land the full record on disk.
    for _ in range(100):
        if read_correlated_records(data_dir):
            break
        time.sleep(0.02)
    recs = read_correlated_records(data_dir)
    if len(recs) != 1:
        _fail(f"expected 1 local correlated record, got {len(recs)}")
    if recs[0].assembly.completeness != "full":
        _fail(f"expected a full record, got {recs[0].assembly.completeness}")

    # Drive the relay flush deterministically (the managed timer would also fire).
    relay = client._own_relay
    if relay is None:
        _fail("relay was not started despite relay.enabled=True")
    result = relay.flush_once()
    if result.uploaded != 1:
        _fail(f"expected 1 upload, got {result} ; received={_received}")

    if len(_received) != 1:
        _fail(f"capture server received {len(_received)} POSTs, expected 1")
    body = _received[0]

    # Registry count-only contract: required fields present + well-formed.
    required = ["schemaVersion", "sensorToken", "eventType", "packageName",
                "runtimeEnv", "triggeredAt", "daySinceInstall", "enforcementOutcome", "agentCategory"]
    for f in required:
        if f not in body:
            _fail(f"missing required wire field: {f}")
    if body["eventType"] != "denied_injection_attempt":
        _fail(f"eventType was {body['eventType']}")
    if body["packageName"] != "smoke-pkg":
        _fail(f"packageName was {body['packageName']}")
    if body["runtimeEnv"] != "python":
        _fail(f"runtimeEnv was {body['runtimeEnv']}")
    if body.get("techniqueId") != "T-2002":
        _fail(f"techniqueId was {body.get('techniqueId')}")

    # NO identifiers on the wire.
    blob = json.dumps(body)
    leaks = {
        "agentId": "agent-secret-id",
        "resource": "evil.example",
        "secret-path": "secret-path",
        "credentialRef": "ref:",
        "inputRef": "sha256:secrethash",
        "correlationId": "cde_",
        "denial reason": "injected exfil blocked",
    }
    for label, needle in leaks.items():
        if needle in blob:
            _fail(f"identifier leaked on wire ({label}): found '{needle}' in {blob}")

    # Second flush must not re-send (cursor advanced).
    if relay.flush_once().uploaded != 0:
        _fail("relay re-sent an already-uploaded indicator (cursor did not advance)")

    client.close()
    server.shutdown()

    print("PASS: 1 anonymized denied_injection_attempt indicator relayed, "
          "no identifiers on wire, correlation header attached, cursor advanced.")
    print("wire body:", json.dumps(body, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())
