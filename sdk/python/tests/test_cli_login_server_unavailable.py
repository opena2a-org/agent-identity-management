"""
`aim-sdk login` against an AIM server that is unavailable: the login stops at
the first such answer, exits 75, and says the server was unavailable.

Measured before this file, against a loopback stub: a 503 on the token poll
printed "Login failed: service_unavailable" (or "Login failed: HTTP 503 from
<url>" for an empty or HTML body) and exited 1; a 503 whose body named
`authorization_pending` kept the CLI polling until the code's lifetime ran out
and then reported an expired code; a 503 on the device-code request exited 1
after telling the user to verify the server URL.

The stub below is a real loopback HTTP server, so an empty or HTML body is
parsed (or not) by the HTTP library exactly as it would be against a gateway.
Every cell counts the requests the server saw and checks the exit status, the
last lines printed, and that nothing was stored.
"""
import base64
import importlib.util
import io
import json
import re
import socket
import sys
import threading
from contextlib import redirect_stdout
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest

from aim_sdk import cli

CODE_PATH = "/api/v1/oauth/device/code"
TOKEN_PATH = "/api/v1/oauth/device/token"
DEVICE = {
    "deviceCode": "d" * 80,
    "userCode": "BCDF-GHJK",
    "verificationUri": "http://localhost:3000/device",
    "verificationUriComplete": "http://localhost:3000/device?user_code=BCDF-GHJK",
    "expiresIn": 900,
    "interval": 5,
}
PENDING = (403, {"error": "authorization_pending"})
# The body the production rate limiter sends with its 429.
LIMITED = (429, {"error": "Rate limit exceeded. Please try again later."})
HTML = b"<html><head><title>503 Service Unavailable</title></head><body>upstream</body></html>"

# What an unavailable server must never be reported as: a failed or refused
# login, an expired or timed-out code, a wrong URL, or a body that would not parse.
NEVER_PRINTED = (
    "Login failed", "Authentication failed", "expired", "timed out", "time-out", "timeout",
    "Check the URL", "verify the server URL", "denied", "Expecting value", "JSONDecodeError",
    "Traceback",
)


def _jwt(claims):
    def seg(d):
        return base64.urlsafe_b64encode(json.dumps(d).encode()).decode().rstrip("=")
    return f"{seg({'alg': 'HS256', 'typ': 'JWT'})}.{seg(claims)}.c2ln"


ACCESS = _jwt({"user_id": "u-1", "organization_id": "o-1", "email": "dev@example.com",
               "typ": "access", "exp": 4102444800})
PAIR = {"accessToken": ACCESS, "refreshToken": _jwt({"user_id": "u-1", "typ": "refresh", "exp": 4102444800}),
        "tokenType": "Bearer", "expiresIn": 7200}


class _Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def _answer(self, status, body=b"", headers=None):
        if isinstance(body, bytes):
            raw, content_type = body, "text/html"
        else:
            raw, content_type = json.dumps(body).encode(), "application/json"
        self.send_response(status)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(raw)))
        for name, value in (headers or {}).items():
            self.send_header(name, value)
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        self.server.aim.hits.append(f"GET {self.path}")
        self._answer(200, {})

    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length") or 0))
        aim = self.server.aim
        aim.hits.append(f"POST {self.path}")
        if self.path == CODE_PATH:
            self._answer(*(aim.code or (200, DEVICE)))
        elif self.path == TOKEN_PATH:
            self._answer(*(aim.polls.pop(0) if aim.polls else (200, PAIR)))
        else:
            self._answer(404, {"error": "no such route"})


class Aim:
    """A loopback AIM server: one scripted device-code answer, then scripted
    poll answers (status, body[, headers]); an unscripted poll is approved."""

    def __init__(self):
        self.code, self.polls, self.hits = None, [], []
        self.sleeps, self.opened, self.saved, self.keychain = [], [], {}, []
        self._server = ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
        self._server.aim = self
        self.url = f"http://127.0.0.1:{self._server.server_address[1]}"
        threading.Thread(target=self._server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True).start()

    def stop(self):
        if self._server is not None:
            self._server.shutdown()
            self._server.server_close()
            self._server = None

    @property
    def token_requests(self):
        return self.hits.count(f"POST {TOKEN_PATH}")

    @property
    def code_requests(self):
        return self.hits.count(f"POST {CODE_PATH}")


@pytest.fixture
def aim(monkeypatch, tmp_path):
    a = Aim()
    monkeypatch.setattr(cli.time, "sleep", a.sleeps.append)
    monkeypatch.setattr(cli.webbrowser, "open", lambda url, *args, **kw: a.opened.append(url) or True)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *args, **kw: None)
    monkeypatch.setattr("aim_sdk.credentials.save_sdk_credentials", lambda c: a.saved.update(c) or True)
    # Anything that wrote the credentials file directly would land here.
    a.aim_dir = tmp_path / ".aim"
    monkeypatch.setattr("aim_sdk.credentials.AIM_DIR", a.aim_dir)
    monkeypatch.setattr("aim_sdk.credentials.SDK_CREDENTIALS_FILE", a.aim_dir / "sdk_credentials.json")
    if importlib.util.find_spec("keyring") is not None:
        for name in ("set_password", "get_password", "delete_password"):
            monkeypatch.setattr(f"keyring.{name}", lambda *args, _name=name, **kw: a.keychain.append(_name))
    yield a
    a.stop()


def _login(aim, monkeypatch, *extra, url=None):
    """Run `aim-sdk login --url <stub> --force` through the entry point."""
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--url", url or aim.url, "--force", *extra])
    buf = io.StringIO()
    with redirect_stdout(buf):
        rc = cli.main()
    return rc, buf.getvalue()


def _assert_unavailable(aim, rc, out, url=None):
    """Exit 75, the two closing lines after one blank line, nothing stored."""
    assert rc == 75, out
    lines = out.splitlines()
    assert lines[-3] == "" and lines[-4] != "", f"exactly one blank line before the closing lines:\n{out}"
    assert lines[-2].startswith(f"The AIM server at {url or aim.url} is unavailable: "), out
    assert lines[-1].startswith("No sign-in was completed and nothing was stored. Run the same command again "), out
    for text in NEVER_PRINTED:
        assert text.lower() not in out.lower(), f"{text!r} printed for an unavailable server:\n{out}"
    _assert_nothing_stored(aim)
    return lines[-2], lines[-1]


def _assert_nothing_stored(aim):
    assert aim.saved == {}
    assert not aim.aim_dir.exists()
    assert aim.keychain == []


@pytest.fixture
def closed_port_url():
    """A loopback URL nothing listens on: bound, learned, closed."""
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
    sock.close()
    return f"http://127.0.0.1:{port}"


# --- A 502, 503 or 504 on the token poll ------------------------------------

def test_a_503_with_a_reason_code_and_retry_after_ends_the_login_after_one_poll(aim, monkeypatch):
    aim.polls = [(503, {"error": "service_unavailable", "reasonCode": "store_unavailable"}, {"Retry-After": "7"})]
    rc, out = _login(aim, monkeypatch)
    what, next_step = _assert_unavailable(aim, rc, out)
    assert aim.token_requests == 1
    assert aim.sleeps == [5], "the wait is printed, never slept"
    assert what == f"The AIM server at {aim.url} is unavailable: it answered HTTP 503, reason code store_unavailable."
    assert next_step.endswith("Run the same command again in 7 seconds.")


@pytest.mark.parametrize("body", [
    b"",
    HTML,
    {"error": "authorization_pending"},
    {"error": "slow_down"},
    {"error": "expired_token"},
    {"error": "access_denied"},
    [1, 2],
    "maintenance",
    7,
], ids=["empty", "html", "names-pending", "names-slow-down", "names-expired", "names-denied",
        "json-array", "json-string", "json-number"])
def test_a_503_with_any_body_ends_the_login_after_one_poll(aim, monkeypatch, body):
    aim.polls = [(503, body)]
    rc, out = _login(aim, monkeypatch)
    what, next_step = _assert_unavailable(aim, rc, out)
    assert aim.token_requests == 1
    assert aim.sleeps == [5]
    assert what.endswith("is unavailable: it answered HTTP 503.")
    assert next_step.endswith("Run the same command again later.")


@pytest.mark.parametrize("status", [502, 504])
def test_a_502_or_504_on_the_poll_ends_the_login_the_same_way(aim, monkeypatch, status):
    aim.polls = [PENDING, (status, HTML)]
    rc, out = _login(aim, monkeypatch)
    what, _ = _assert_unavailable(aim, rc, out)
    assert aim.token_requests == 2
    assert aim.sleeps == [5, 5]
    assert what.endswith(f"it answered HTTP {status}.")


@pytest.mark.parametrize("header,phrase", [
    ("7", "again in 7 seconds."),
    ("1", "again in 1 second."),
    ("0", "again now."),
    (" 30 ", "again in 30 seconds."),
    ("-3", "again later."),
    ("1.5", "again later."),
    ("soon", "again later."),
    ("Wed, 21 Oct 2026 07:28:00 GMT", "again later."),
    ("9" * 40, "again later."),
])
def test_the_wait_is_retry_after_as_whole_seconds_or_later_and_never_a_default(aim, monkeypatch, header, phrase):
    aim.polls = [(503, b"", {"Retry-After": header})]
    rc, out = _login(aim, monkeypatch)
    _, next_step = _assert_unavailable(aim, rc, out)
    assert next_step.endswith("Run the same command " + phrase)
    assert aim.sleeps == [5]


@pytest.mark.parametrize("body,shown", [
    ({"reasonCode": "store_unavailable", "code": "other"}, "store_unavailable"),
    ({"reasonCode": 5, "code": "maintenance"}, "maintenance"),
    ({"code": "maintenance"}, "maintenance"),
    ({"reasonCode": None, "code": ["x"]}, None),
    ({"reasonCode": "\x1b[31mnot a code"}, None),
])
def test_the_reason_code_is_the_body_reason_code_then_code_and_only_when_it_is_a_plain_token(
        aim, monkeypatch, body, shown):
    aim.polls = [(503, body)]
    rc, out = _login(aim, monkeypatch)
    what, _ = _assert_unavailable(aim, rc, out)
    if shown is None:
        assert what.endswith("it answered HTTP 503.")
    else:
        assert what.endswith(f"it answered HTTP 503, reason code {shown}.")
    assert "\x1b" not in out


# --- A 502, 503 or 504 on the device-code request ---------------------------

@pytest.mark.parametrize("status", [502, 503, 504])
def test_an_unavailable_answer_to_the_device_code_request_never_polls_or_opens_a_browser(
        aim, monkeypatch, status):
    aim.code = (status, {"error": "service_unavailable"}, {"Retry-After": "7"})
    rc, out = _login(aim, monkeypatch)
    what, next_step = _assert_unavailable(aim, rc, out)
    assert aim.code_requests == 1
    assert aim.token_requests == 0
    assert aim.sleeps == []
    assert aim.opened == []
    assert what.endswith(f"it answered HTTP {status}.")
    assert next_step.endswith("Run the same command again in 7 seconds.")


# --- The status decides before the body --------------------------------------

def test_a_429_from_the_rate_limiter_slows_every_later_poll_and_the_login_completes(aim, monkeypatch):
    aim.polls = [PENDING, LIMITED, PENDING, (200, PAIR)]
    rc, out = _login(aim, monkeypatch)
    assert rc == 0, out
    assert aim.token_requests == 4
    assert aim.sleeps == [5, 5, 10, 10], "every poll after the 429 waits 5 s more than the interval"
    assert aim.saved["accessToken"] == ACCESS


@pytest.mark.parametrize("state", ["authorization_pending", "slow_down", "expired_token", "access_denied"])
def test_a_429_is_a_slow_down_whatever_its_body_names(aim, monkeypatch, state):
    aim.polls = [(429, {"error": state}), (200, PAIR)]
    rc, out = _login(aim, monkeypatch)
    assert rc == 0, out
    assert aim.token_requests == 2
    assert aim.sleeps == [5, 10]


@pytest.mark.parametrize("state", ["authorization_pending", "slow_down", "expired_token", "access_denied", "timeout"])
def test_a_500_that_names_a_grant_state_is_a_failed_request_not_that_state(aim, monkeypatch, state):
    aim.polls = [(500, {"error": state})]
    rc, out = _login(aim, monkeypatch)
    assert rc == 1, out
    assert aim.token_requests == 1
    assert aim.sleeps == [5]
    assert f"Login failed: HTTP 500 from {aim.url}{TOKEN_PATH}" in out
    assert "expired" not in out.lower() and "denied" not in out.lower()
    _assert_nothing_stored(aim)


@pytest.mark.parametrize("status", [400, 403])
def test_the_grant_states_still_count_on_a_400_or_403(aim, monkeypatch, status):
    aim.polls = [(status, {"error": "authorization_pending"}), (status, {"error": "slow_down"}),
                 (status, {"error": "access_denied"})]
    rc, out = _login(aim, monkeypatch)
    assert rc == 1, out
    assert aim.token_requests == 3
    assert aim.sleeps == [5, 5, 10]
    assert "denied" in out.lower()
    _assert_nothing_stored(aim)


# --- No HTTP answer -----------------------------------------------------------

def test_a_refused_connection_mid_poll_ends_the_login_as_unavailable(aim, monkeypatch):
    aim.polls = [PENDING]

    def sleep(seconds):
        aim.sleeps.append(seconds)
        if len(aim.sleeps) == 2:
            aim.stop()  # the server goes away between the first poll and the second
    monkeypatch.setattr(cli.time, "sleep", sleep)

    rc, out = _login(aim, monkeypatch)
    what, next_step = _assert_unavailable(aim, rc, out)
    assert aim.token_requests == 1
    assert aim.sleeps == [5, 5]
    assert what.endswith("is unavailable: it gave no HTTP answer (connection refused).")
    assert next_step.endswith("Run the same command again later.")


def test_an_unreachable_server_at_the_pre_flight_probe_is_unavailable(aim, monkeypatch, closed_port_url):
    rc, out = _login(aim, monkeypatch, url=closed_port_url)
    what, _ = _assert_unavailable(aim, rc, out, url=closed_port_url)
    assert aim.hits == []
    assert aim.opened == []
    assert what.endswith("is unavailable: it gave no HTTP answer (connection refused).")


def test_a_probe_that_gets_no_answer_in_time_is_unavailable_and_not_called_a_timeout(aim, monkeypatch):
    def unanswered(*args, **kw):
        raise cli.requests.exceptions.Timeout("HTTPConnectionPool: Read timed out. (read timeout=5)")
    monkeypatch.setattr(cli.requests, "get", unanswered)
    rc, out = _login(aim, monkeypatch)
    what, _ = _assert_unavailable(aim, rc, out)
    assert aim.hits == []
    assert what.endswith("is unavailable: it gave no HTTP answer in time.")


def test_no_answer_to_the_device_code_request_is_unavailable(aim, monkeypatch):
    def refused(*args, **kw):
        raise cli.requests.exceptions.ConnectionError("[Errno 61] Connection refused")
    monkeypatch.setattr(cli.requests, "post", refused)
    rc, out = _login(aim, monkeypatch)
    _assert_unavailable(aim, rc, out)
    assert aim.hits == ["GET /"]
    assert aim.opened == []


# A failure that is not an outage keeps exit 1 and its own words: running the
# same command again later would not help.

def test_a_url_that_cannot_be_requested_is_not_an_outage(aim, monkeypatch):
    def invalid(*args, **kw):
        raise cli.requests.exceptions.RequestException("Invalid URL")
    monkeypatch.setattr(cli.requests, "get", invalid)
    rc, out = _login(aim, monkeypatch)
    assert rc == 1, out
    assert "Check the URL" in out
    assert "is unavailable" not in out
    assert aim.hits == []


@pytest.mark.skipif(not hasattr(cli.requests.exceptions, "SSLError"), reason="needs the requests package")
@pytest.mark.parametrize("stage", ["probe", "device-code"])
def test_a_rejected_tls_certificate_is_not_an_outage(aim, monkeypatch, stage):
    def rejected(*args, **kw):
        raise cli.requests.exceptions.SSLError("certificate verify failed: self-signed certificate")
    monkeypatch.setattr(cli.requests, "get" if stage == "probe" else "post", rejected)
    rc, out = _login(aim, monkeypatch)
    assert rc == 1, out
    assert "is unavailable" not in out
    assert aim.opened == []
    _assert_nothing_stored(aim)


# --- The device flow has its own requests ------------------------------------

@pytest.mark.parametrize("script", ["approved", "poll-503", "code-503"])
def test_the_device_flow_requests_never_go_through_the_client_request_path(aim, monkeypatch, script):
    """`AIMClient._make_request` waits out a `Retry-After` and retries; the
    device-code request and the token poll must never inherit that."""
    from aim_sdk.client import AIMClient

    calls = []
    monkeypatch.setattr(AIMClient, "_make_request", lambda self, *args, **kw: calls.append(args) or {})
    if script == "poll-503":
        aim.polls = [(503, b"", {"Retry-After": "7"})]
    elif script == "code-503":
        aim.code = (503, b"", {"Retry-After": "7"})
    else:
        aim.polls = [PENDING, LIMITED]
    rc, out = _login(aim, monkeypatch)
    assert rc == (0 if script == "approved" else 75), out
    assert calls == []
    assert aim.code_requests == 1
    assert aim.token_requests == {"approved": 3, "poll-503": 1, "code-503": 0}[script]


# --- `login --help` lists the exit codes, and each line is true ---------------

def _login_help(monkeypatch, capsys):
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--help"])
    with pytest.raises(SystemExit) as excinfo:
        cli.main()
    assert excinfo.value.code == 0
    return capsys.readouterr().out


def test_login_help_lists_the_exit_codes(monkeypatch, capsys):
    out = _login_help(monkeypatch, capsys)
    listed = out.split("exit codes:", 1)[1]
    assert re.findall(r"^  (\d+) ", listed, re.M) == ["0", "1", "2", "75"]
    assert re.search(r"^  75 +the AIM server was unavailable", listed, re.M)
    assert "HTTP 502, 503 or 504" in listed and "no HTTP answer" in listed


def test_exit_0_is_a_completed_sign_in(aim, monkeypatch):
    rc, out = _login(aim, monkeypatch)
    assert rc == 0, out
    assert aim.saved["accessToken"] == ACCESS


def test_exit_0_is_also_an_existing_sign_in_that_was_kept(aim, monkeypatch):
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials",
                        lambda *args, **kw: {"aimUrl": aim.url, "userEmail": "dev@example.com"})
    monkeypatch.setattr("builtins.input", lambda prompt="": "n")
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--url", aim.url])
    with redirect_stdout(io.StringIO()):
        assert cli.main() == 0
    assert aim.hits == []
    _assert_nothing_stored(aim)


@pytest.mark.parametrize("answer", [(403, {"error": "access_denied"}), (403, {"error": "expired_token"}),
                                    (400, {"error": "invalid_request", "errorDescription": "Unknown device code"})],
                         ids=["denied", "expired", "refused"])
def test_exit_1_is_a_login_that_was_denied_expired_or_refused(aim, monkeypatch, answer):
    aim.polls = [answer]
    rc, out = _login(aim, monkeypatch)
    assert rc == 1, out
    assert "is unavailable" not in out
    _assert_nothing_stored(aim)


def test_exit_2_is_a_command_line_that_was_not_valid(aim, monkeypatch, capsys):
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--no-such-option"])
    with pytest.raises(SystemExit) as excinfo:
        cli.main()
    assert excinfo.value.code == 2
    assert aim.hits == []


@pytest.mark.parametrize("unavailable", ["answered-503", "no-answer"])
def test_exit_75_is_an_unavailable_server(aim, monkeypatch, closed_port_url, unavailable):
    if unavailable == "answered-503":
        aim.polls = [(503, b"")]
        rc, out = _login(aim, monkeypatch)
    else:
        rc, out = _login(aim, monkeypatch, url=closed_port_url)
    assert rc == 75, out
    _assert_nothing_stored(aim)
