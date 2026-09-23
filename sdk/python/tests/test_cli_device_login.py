"""
`aim-sdk login --url` completes through the OAuth 2.0 device grant (RFC 8628)
the backend serves: POST /api/v1/oauth/device/code, then POST
/api/v1/oauth/device/token until the user approves the code in the dashboard,
then the same user pair the login page issues is stored.

Measured 2026-09-22 against a self-hosted stack: the CLI opened
/auth/login?response_type=code&code_challenge=... (which the dashboard
ignored) and posted the code to /api/v1/auth/token (which no backend in this
repository registers), so no self-hosted login ever completed.

The fake backend below answers the device-code request, then a scripted
sequence of poll answers, exactly in the shapes device_auth_handler.go emits.
"""
import base64
import io
import json
from contextlib import redirect_stdout
from types import SimpleNamespace
from unittest.mock import MagicMock

import pytest

from aim_sdk import cli

AIM = "http://localhost:8080"
DEVICE = {
    "deviceCode": "d" * 80,
    "userCode": "BCDF-GHJK",
    "verificationUri": "http://localhost:3000/device",
    "verificationUriComplete": "http://localhost:3000/device?user_code=BCDF-GHJK",
    "expiresIn": 900,
    "interval": 5,
}
GRANT = "urn:ietf:params:oauth:grant-type:device_code"


def _jwt(claims):
    def seg(d):
        return base64.urlsafe_b64encode(json.dumps(d).encode()).decode().rstrip("=")
    return f"{seg({'alg': 'HS256', 'typ': 'JWT'})}.{seg(claims)}.c2ln"


ACCESS = _jwt({"user_id": "u-1", "organization_id": "o-1", "email": "dev@example.com",
               "role": "admin", "typ": "access", "iss": "agent-identity-management", "exp": 4102444800})
REFRESH = _jwt({"user_id": "u-1", "organization_id": "o-1", "typ": "refresh",
                "iss": "agent-identity-management", "exp": 4102444800, "jti": "j-1"})
PAIR = {"accessToken": ACCESS, "refreshToken": REFRESH, "tokenType": "Bearer", "expiresIn": 7200}


def _resp(status, body):
    r = MagicMock()
    r.status_code = status
    r.content = json.dumps(body).encode()
    r.text = json.dumps(body)
    r.headers = {"content-type": "application/json"}
    r.json.return_value = body
    return r


class Stack:
    """A fake backend: the device-code answer, then scripted poll answers."""

    def __init__(self, code=(200, DEVICE), polls=((403, {"error": "authorization_pending"}), (200, PAIR))):
        self.code, self.polls, self.calls, self.sleeps = code, list(polls), [], []
        self.opened, self.saved = [], {}

    def post(self, url, json=None, headers=None, timeout=None):
        self.calls.append((url, json))
        if url == f"{AIM}/api/v1/oauth/device/code":
            return _resp(*self.code)
        if url == f"{AIM}/api/v1/oauth/device/token":
            return _resp(*(self.polls.pop(0) if self.polls else (200, PAIR)))
        return _resp(404, {"error": "no such route"})


@pytest.fixture
def stack(monkeypatch):
    s = Stack()
    monkeypatch.setattr(cli.requests, "post", s.post)
    monkeypatch.setattr(cli.requests, "get", lambda *a, **k: _resp(200, {}))
    monkeypatch.setattr(cli.time, "sleep", lambda n: s.sleeps.append(n))
    # The pre-device-grant CLI waited on a local callback server; bound that wait
    # so a regression fails in seconds instead of minutes.
    monkeypatch.setattr(cli, "LOGIN_CALLBACK_TIMEOUT_SECONDS", 1, raising=False)
    monkeypatch.setattr(cli.webbrowser, "open", lambda url, *a, **k: s.opened.append(url) or True)
    monkeypatch.setattr("aim_sdk.credentials.save_sdk_credentials", lambda c: s.saved.update(c) or True)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *a, **k: None)
    return s


def _login(stack):
    buf = io.StringIO()
    with redirect_stdout(buf):
        rc = cli.login(SimpleNamespace(url=AIM, force=True))
    return rc, buf.getvalue()


def test_login_requests_a_device_code_polls_and_stores_the_user_pair(stack):
    rc, out = _login(stack)
    assert rc == 0, out
    assert stack.calls[0][0] == f"{AIM}/api/v1/oauth/device/code"
    assert stack.calls[0][1].get("clientId") == "aim-sdk"
    polls = [c for c in stack.calls if c[0].endswith("/oauth/device/token")]
    assert len(polls) == 2
    assert polls[0][1] == {"deviceCode": DEVICE["deviceCode"], "grantType": GRANT}
    assert stack.saved["aimUrl"] == AIM
    assert stack.saved["accessToken"] == ACCESS
    assert stack.saved["refreshToken"] == REFRESH
    assert stack.saved["userEmail"] == "dev@example.com"
    assert stack.saved["userId"] == "u-1"
    assert stack.saved["organizationId"] == "o-1"
    assert "Traceback" not in out


def test_login_shows_the_code_and_the_verification_url_and_opens_the_complete_uri(stack):
    rc, out = _login(stack)
    assert rc == 0, out
    assert "BCDF-GHJK" in out
    assert "http://localhost:3000/device" in out
    assert stack.opened == [DEVICE["verificationUriComplete"]]


def test_login_never_polls_faster_than_the_interval_and_backs_off_on_slow_down_and_429(stack):
    stack.polls = [(403, {"error": "slow_down"}),
                   (429, {"error": "Rate limit exceeded. Please try again later."}),
                   (403, {"error": "authorization_pending"}),
                   (200, PAIR)]
    rc, out = _login(stack)
    assert rc == 0, out
    assert len(stack.sleeps) >= 4
    assert all(s >= 5 for s in stack.sleeps), stack.sleeps
    # After slow_down and after an HTTP 429 the wait grows by at least 5 s (RFC 8628 §3.5).
    assert stack.sleeps[1] >= 10 and stack.sleeps[2] >= 10, stack.sleeps


def test_login_fails_fast_and_names_the_cause_when_no_device_code_is_issued(stack):
    stack.code = (500, {"error": "server_error", "errorDescription": "Failed to initiate device authorization"})
    rc, out = _login(stack)
    assert rc == 1
    assert "device" in out.lower() and "Failed to initiate device authorization" in out
    assert out.count("Failed to initiate device authorization") == 1, "the reason is printed once, not prefixed twice"
    assert stack.opened == []
    assert stack.saved == {}


def test_login_fails_fast_on_a_device_code_body_without_the_fields(stack):
    stack.code = (200, {"unexpected": True})
    rc, out = _login(stack)
    assert rc == 1
    assert "device" in out.lower()
    assert stack.opened == []
    assert stack.saved == {}


@pytest.mark.parametrize("error,word", [("expired_token", "expired"), ("access_denied", "denied")])
def test_login_reports_expired_and_denied_as_distinct_failures(stack, error, word):
    stack.polls = [(403, {"error": error})]
    rc, out = _login(stack)
    assert rc == 1
    assert word in out.lower()
    assert stack.saved == {}


def test_login_never_prints_the_device_code(stack):
    """The device code is the secret half of the grant; only the user code is shown."""
    rc, out = _login(stack)
    assert rc == 0, out
    assert DEVICE["deviceCode"] not in out
    assert DEVICE["deviceCode"][:16] not in out


def test_login_gives_up_when_the_code_expires_before_approval(stack, monkeypatch):
    """A server that stays authorization_pending past expiresIn: the wait is
    bounded by the server's own lifetime, the exit is non-zero and named."""
    stack.code = (200, dict(DEVICE, expiresIn=2, interval=1))
    stack.polls = [(403, {"error": "authorization_pending"})] * 50
    clock = {"now": 1000.0}

    def monotonic():
        # Every read moves time on, so any wait loop that reads the clock
        # without sleeping still reaches its deadline.
        clock["now"] += 0.25
        return clock["now"]
    monkeypatch.setattr(cli.time, "monotonic", monotonic)

    def sleep(n):
        stack.sleeps.append(n)
        clock["now"] += n
    monkeypatch.setattr(cli.time, "sleep", sleep)

    rc, out = _login(stack)
    assert rc == 1
    assert "expired" in out.lower() or "timed out" in out.lower()
    assert 0 < 50 - len(stack.polls) < 10, "the loop polled a few times, then stopped at the code's lifetime"
    assert stack.saved == {}


# The verification URI the CLI prints and the one it hands to the browser are
# both server-chosen strings. Only http(s) may be opened; anything else exits
# before webbrowser.open and stores nothing.
@pytest.mark.parametrize("complete", ["javascript:alert(1)", "file:///etc/passwd"])
def test_login_refuses_to_open_a_non_http_complete_uri(stack, complete):
    stack.code = (200, dict(DEVICE, verificationUriComplete=complete))
    rc, out = _login(stack)
    assert rc == 1
    assert stack.opened == []
    assert stack.saved == {}
    assert "http" in out.lower()


def test_login_refuses_a_non_http_verification_uri(stack):
    stack.code = (200, dict(DEVICE, verificationUri="javascript:alert(1)", verificationUriComplete=None))
    rc, out = _login(stack)
    assert rc == 1
    assert stack.opened == []
    assert stack.saved == {}


def test_login_tolerates_a_non_numeric_interval(stack):
    """A server answer with a malformed interval is refused or defaulted, never a traceback."""
    stack.code = (200, dict(DEVICE, interval="abc"))
    rc, out = _login(stack)
    assert rc in (0, 1)
    assert "Traceback" not in out
    if rc == 0:
        assert all(s >= 5 for s in stack.sleeps), stack.sleeps
