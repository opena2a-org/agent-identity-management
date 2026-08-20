"""
require_approval() must never print "approved" unless AIM actually approved.

Found by the aim-sdk 2.0.0 release test. The AttributeError crash this release
fixes (console.show_jit_awaiting did not exist) was the ONLY thing standing
between an operator and this: `verdict.blocked is False` covers three cases, and
only one of them is a real approval.

    Outcome.ALLOW                                           -- a real approval
    Outcome.UNKNOWN + monitoring mode (leniency)            -- NOT approved, ran anyway
    Outcome.UNKNOWN + unresolved mode + transport failure   -- NOT approved, ran anyway

The second row used to read `Outcome.DENY + monitoring mode`. As of 2026-08-13 an
explicit denial blocks in every mode, so that case cannot reach the console at
all and a test asserting on its output would have asserted nothing. What remains
lenient under monitoring -- and therefore still able to print "approved" for an
action nobody approved -- is the UNKNOWN row.

Reproduced exactly as measured in the release test: backend unreachable, cold
mode cache, AIM_STRICT_MODE unset.

    ⚠ Warning: AIM could not be reached to verify 'transfer_money' ...
      ✓ transfer_money approved - executing...
    RESULT: MONEY MOVED

No AIM instance was contacted. No human saw anything. The last line an operator
reads said the opposite of the warning directly above it.

These tests drive the REAL client and the REAL decorator, stubbing only
`session.request`, and assert on captured console output -- the same standard
`test_enforcement_matrix.py` holds itself to and for the same reason: a mock's
call count can look right next to output that lies.

NOT marked `integration` (pytest.ini deselects those by default and this file
must actually run in CI).
"""

import base64
import io
from contextlib import redirect_stdout

import pytest
import requests
from nacl.encoding import Base64Encoder
from nacl.signing import SigningKey

import aim_sdk.client as client_module
from aim_sdk.client import AIMClient
from aim_sdk.decision import get_mode_cache
from aim_sdk.strict_mode import reset_warning_state

VERIFICATION_ID = "11111111-2222-3333-4444-555555555555"
AGENT_ID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"


@pytest.fixture(autouse=True)
def _isolate(monkeypatch):
    """Same isolation as test_enforcement_matrix.py, and for the same reason:
    the mode cache is process-scoped and keyed by agent id, so a prior test's
    ALLOW would otherwise warm this one and hide the exact case that matters."""
    get_mode_cache().clear()
    reset_warning_state()
    monkeypatch.delenv("AIM_STRICT_MODE", raising=False)
    monkeypatch.setattr(client_module, "RATE_LIMIT_RETRY_ATTEMPTS", 0)
    yield
    get_mode_cache().clear()


class _FakeResponse:
    def __init__(self, status_code, payload=None):
        self.status_code = status_code
        self._payload = payload or {}

    def json(self):
        return self._payload

    def raise_for_status(self):
        if self.status_code >= 400:
            raise requests.exceptions.HTTPError(f"HTTP {self.status_code}")


def _client(request_stub):
    sk = SigningKey.generate()
    client = AIMClient(
        agent_id=AGENT_ID,
        public_key=sk.verify_key.encode(encoder=Base64Encoder).decode(),
        private_key=base64.b64encode(bytes(sk)).decode(),
        aim_url="http://aim.invalid",
        sdk_token_id="test-token",
        telemetry={"enabled": False},
    )
    client.session.request = request_stub
    return client


def _run_decorated(client, capture_console):
    """Apply require_approval, call it, capture BOTH output backends the way
    test_console_agent_found.py does, and return (body_ran, output_text)."""
    ran = {"body": False}

    @client.require_approval(risk_level="high", action_name="transfer_money")
    def transfer_money():
        ran["body"] = True
        return "MONEY MOVED"

    buf = io.StringIO()
    if capture_console.console is not None:
        capture_console.console.file = buf
    with redirect_stdout(buf):
        try:
            transfer_money()
        except BaseException:
            pass  # blocked cases are covered by test_enforcement_matrix.py
    return ran["body"], buf.getvalue()


def test_transport_failure_with_unresolved_mode_does_not_claim_approval(monkeypatch):
    """The exact release-test repro: connection refused, cold cache, strict
    mode unset. Body runs (that is the ruled 2.0.0 warning-window behaviour,
    not a regression target here) -- but the output must not say "approved"."""
    from aim_sdk import console as console_module

    def stub(method, url, **kwargs):
        raise requests.exceptions.ConnectionError("connection refused")

    client = _client(stub)
    body_ran, out = _run_decorated(client, console_module.console)

    assert body_ran, "this case is supposed to run under the 2.0.0 warning window"
    assert "approved" not in out.lower(), (
        "AIM was never reached and nothing approved this action, but the "
        "console said it was approved. Full output:\n" + out
    )


def test_unanswered_verification_under_monitoring_does_not_claim_approval():
    """
    A verification AIM could not answer, run anyway because the organization is
    in monitoring mode, is leniency -- not approval. Requires a warm cache
    carrying mode=monitoring, matching how test_enforcement_matrix.py establishes
    the monitoring rows.

    This test was `test_deny_under_monitoring_mode_does_not_claim_approval` until
    2026-08-13. Its premise -- a denial that executes -- no longer exists, so it
    was repointed rather than deleted: the honesty defect it guards is not about
    denials specifically, it is about `verdict.blocked is False` being read as
    "AIM approved this", and the UNKNOWN+monitoring cell still reaches that line.
    A 500 is used because it is a server answer that carries no decision, which
    is precisely what monitoring mode is for.
    """
    from aim_sdk import console as console_module

    def warm_stub(method, url, **kwargs):
        if "/verifications" in url:
            return _FakeResponse(200, {"id": VERIFICATION_ID, "status": "approved",
                                        "enforcementMode": "monitoring"})
        return _FakeResponse(200, {"ok": True})

    warm_client = _client(warm_stub)
    warm_client.verify_capability(capability="warm:up", resource="warm")

    def unanswered_stub(method, url, **kwargs):
        if "/verifications" in url:
            return _FakeResponse(500, {"error": "internal server error"})
        return _FakeResponse(200, {"ok": True})

    client = _client(unanswered_stub)
    body_ran, out = _run_decorated(client, console_module.console)

    assert body_ran, "monitoring mode is leniency on an UNANSWERED verification"
    assert "approved" not in out.lower(), (
        "AIM never returned a decision; monitoring mode ran the action anyway, "
        "which is not the same thing as approval. Full output:\n" + out
    )


def test_a_denial_under_monitoring_mode_never_reaches_the_console_at_all():
    """
    The retired premise, asserted as a fact rather than deleted silently.

    If a future change re-introduces "monitoring runs a denial", this file's
    honesty guarantee quietly regains a case it no longer covers. Pinning the
    block here means that regression shows up in the file that used to depend on
    it.
    """
    from aim_sdk import console as console_module
    from aim_sdk.exceptions import ActionDeniedError

    def warm_stub(method, url, **kwargs):
        if "/verifications" in url:
            return _FakeResponse(200, {"id": VERIFICATION_ID, "status": "approved",
                                        "enforcementMode": "monitoring"})
        return _FakeResponse(200, {"ok": True})

    _client(warm_stub).verify_capability(capability="warm:up", resource="warm")

    def deny_stub(method, url, **kwargs):
        if "/verifications" in url:
            return _FakeResponse(403, {
                "id": VERIFICATION_ID, "status": "denied",
                "denialReason": "policy: blocked by test",
                "enforcementMode": "monitoring",
            })
        return _FakeResponse(200, {"ok": True})

    client = _client(deny_stub)
    with pytest.raises(ActionDeniedError):
        @client.require_approval(risk_level="high", action_name="transfer_money")
        def transfer_money():
            raise AssertionError("a denied action executed under monitoring mode")

        transfer_money()


def test_genuine_allow_still_says_approved():
    """Positive control. Without this, a fix that deletes jit_approved()
    entirely -- silence in every case -- would pass the two tests above."""
    from aim_sdk import console as console_module

    def allow_stub(method, url, **kwargs):
        if "/verifications" in url:
            return _FakeResponse(200, {"id": VERIFICATION_ID, "status": "approved"})
        return _FakeResponse(200, {"ok": True})

    client = _client(allow_stub)
    body_ran, out = _run_decorated(client, console_module.console)

    assert body_ran
    assert "approved" in out.lower(), (
        "a genuine ALLOW must still be announced as approved. Full output:\n" + out
    )
