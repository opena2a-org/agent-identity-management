"""
`_make_request`'s retry/backoff path must work for API-key-only clients.

Found by an adversarial re-verification pass while confirming two unrelated
2.0.0 fixes. Pre-existing on `origin/main`; traces to an old reorg commit,
long before the 2.0.0 branch. Unrelated to enforcement mode, denial handling,
or anything else in this release -- filed and fixed as its own unit.

The bug: `import time` and `import json` are re-imported LOCALLY inside
`_make_request`, nested under `if self.signing_key and self.public_key and
self.agent_id:` (Ed25519 signing branch). A local binding anywhere in a
function body makes Python treat that name as local to the WHOLE function --
not merely from that line down. The retry/backoff loop
(`time.sleep(2 ** retry_count)`) sits OUTSIDE that branch, unconditioned. So
any client that does not have all three of signing_key/public_key/agent_id set
-- an api_key-only ("Manual mode") client, one of the two documented auth
modes -- raises `UnboundLocalError: cannot access local variable 'time' where
it is not associated with a value` the moment a request needs to retry (a
5xx with auto_retry, or a Timeout). That masks the REAL error (the 500, or the
timeout) behind a confusing internal one, on `register_capability`,
`request_capability`, `list_agents`, and any other caller of `_make_request`
that reaches the retry path.

Both `time` and `json` are already imported at module level (client.py:9,:11).
The fix is deletion of the two redundant local imports, not a new import
anywhere -- there is no behaviour to preserve, only a shadow to remove.

NOT marked `integration` -- must actually run in CI.
"""

import requests

from aim_sdk.client import AIMClient


class _FakeResponse:
    def __init__(self, status_code, payload=None):
        self.status_code = status_code
        self._payload = payload or {}
        self.text = str(self._payload)

    def json(self):
        return self._payload

    def raise_for_status(self):
        if self.status_code >= 400:
            raise requests.exceptions.HTTPError(f"HTTP {self.status_code}")


def _api_key_only_client():
    """The exact shape that reaches the buggy branch: api_key set, no
    public_key/private_key, so self.signing_key stays None and the Ed25519
    branch containing the (now-removed) local imports never executes."""
    return AIMClient(
        agent_id="test-agent",
        api_key="test-key",
        aim_url="http://aim.invalid",
        telemetry={"enabled": False},
    )


def test_retry_after_5xx_does_not_raise_unboundlocalerror():
    """A 500 with auto_retry (default True) takes the retry branch on the
    FIRST call, before any signing-key-branch code has ever run in this
    client's lifetime. Pre-fix this raised UnboundLocalError instead of
    eventually succeeding."""
    client = _api_key_only_client()

    calls = {"n": 0}

    def stub(method, url, **kwargs):
        calls["n"] += 1
        if calls["n"] == 1:
            return _FakeResponse(500, {"error": "internal server error"})
        return _FakeResponse(200, {"agents": [], "total": 0})

    client.session.request = stub

    result = client.list_agents()

    assert calls["n"] == 2, "must have actually retried, not merely not-crashed"
    assert result == {"agents": [], "total": 0}


def test_retry_after_timeout_does_not_raise_unboundlocalerror():
    """Same defect, the other trigger: requests.exceptions.Timeout instead
    of a 5xx status code."""
    client = _api_key_only_client()

    calls = {"n": 0}

    def stub(method, url, **kwargs):
        calls["n"] += 1
        if calls["n"] == 1:
            raise requests.exceptions.Timeout("timed out")
        return _FakeResponse(200, {"agents": [], "total": 0})

    client.session.request = stub

    result = client.list_agents()

    assert calls["n"] == 2
    assert result == {"agents": [], "total": 0}


def test_the_real_error_surfaces_when_retries_are_exhausted():
    """The other half of the bug: even when the caller-visible symptom is
    "an exception was raised" either way, pre-fix the message was the
    confusing UnboundLocalError rather than the real cause. Assert the
    surfaced message names the real failure, not the shadowing artifact."""
    client = _api_key_only_client()
    client.max_retries = 1

    def always_500(method, url, **kwargs):
        return _FakeResponse(500, {"error": "internal server error"})

    client.session.request = always_500

    try:
        client.list_agents()
        assert False, "expected an exception once retries are exhausted"
    except Exception as e:  # noqa: BLE001 - asserting on the message, not the type
        msg = str(e)
        assert "local variable" not in msg, (
            f"the real error was masked by the shadowing bug: {msg}"
        )
        assert "500" in msg or "internal server error" in msg or "HTTP" in msg, (
            f"expected the real HTTP failure in the message, got: {msg}"
        )


def test_signing_key_branch_still_signs_when_configured():
    """Positive control: deleting the redundant local imports must not break
    the Ed25519 signing path they were (accidentally) sitting inside of."""
    from nacl.encoding import Base64Encoder
    from nacl.signing import SigningKey

    sk = SigningKey.generate()
    client = AIMClient(
        agent_id="test-agent",
        public_key=sk.verify_key.encode(encoder=Base64Encoder).decode(),
        private_key=__import__("base64").b64encode(bytes(sk)).decode(),
        aim_url="http://aim.invalid",
        telemetry={"enabled": False},
    )

    captured = {}

    def stub(method, url, **kwargs):
        captured["headers"] = kwargs.get("headers", {})
        return _FakeResponse(200, {"agents": [], "total": 0})

    client.session.request = stub
    client.list_agents()

    assert "X-Signature" in captured["headers"], (
        f"signing branch did not run or did not attach X-Signature: {captured['headers']}"
    )
    assert captured["headers"]["X-Agent-ID"] == "test-agent"
    assert captured["headers"]["X-Public-Key"] == client.public_key
