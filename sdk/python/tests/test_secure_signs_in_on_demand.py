"""
secure() signs this machine in on demand. With no SDK credentials (from
`aim-sdk login` or a dashboard download) and no api_key, and a person present
(CI unset or empty, standard input a terminal or an IPython kernel), the first
call runs the device grant `aim-sdk login` uses (RFC 8628), stores the
credentials `aim-sdk login` stores, and registers in OAuth mode. In a CI job,
or with standard input that is not a terminal outside an IPython kernel, it
raises the no-credentials ConfigurationError before any request.

Before this change, at a terminal with an empty home directory,
secure("my-first-agent", aim_url=..., capabilities=["db:read"]) raised
ConfigurationError "No authentication credentials found." and the README's
path needed `aim-sdk login` first.

The fake backend answers the device-code request and a scripted sequence of
poll answers in the shapes the backend's device_auth_service.go and
device_auth_handler.go define. Registration is replaced, so these tests end at
the hand-off to OAuth registration.
"""

import io
import json
import stat
import sys
import time
import types
from unittest.mock import MagicMock

import pytest

from aim_sdk import cli, client, credentials, secure
from aim_sdk.exceptions import AuthenticationError, ConfigurationError

AIM = "http://127.0.0.1:8080"
DEVICE = {
    "deviceCode": "fake-device-code",
    "userCode": "BCDF-GHJK",
    "verificationUri": "http://127.0.0.1:3000/device",
    "verificationUriComplete": "http://127.0.0.1:3000/device?user_code=BCDF-GHJK",
    "expiresIn": 900,
    "interval": 5,
}
# The tokens are opaque strings here: decode_jwt_claims is replaced with this table.
CLAIMS = {"FAKE-ACCESS-TOKEN": {"user_id": "u-1", "organization_id": "o-1", "email": "dev@example.com"}}
PAIR = {"accessToken": "FAKE-ACCESS-TOKEN", "refreshToken": "FAKE-REFRESH-TOKEN",
        "tokenType": "Bearer", "expiresIn": 7200}
PENDING = (403, {"error": "authorization_pending"})
LEAD = "my-first-agent needs an AIM sign-in on this machine. Sign in once to continue."
WAIT = ("Waiting for your approval (up to {} min). Ctrl+C stops the wait. "
        "Nothing is stored until you approve.")


def _resp(status, body):
    r = MagicMock()
    r.status_code = status
    r.content = json.dumps(body).encode()
    r.text = json.dumps(body)
    r.headers = {"content-type": "application/json"}
    r.json.return_value = body
    return r


class Backend:
    """One server's device-grant routes: the device-code answer, then scripted poll answers."""

    def __init__(self, base=AIM, code=(200, DEVICE), polls=(PENDING, (200, PAIR))):
        self.base, self.code, self.polls = base, code, list(polls)
        self.calls, self.opened = [], []

    def post(self, url, json=None, headers=None, timeout=None):
        self.calls.append(url)
        if url == f"{self.base}/api/v1/oauth/device/code":
            return _resp(*self.code)
        if url == f"{self.base}/api/v1/oauth/device/token":
            return _resp(*(self.polls.pop(0) if self.polls else (200, PAIR)))
        return _resp(404, {"error": "no such route"})

    def get(self, url, **kwargs):
        self.calls.append(url)
        return _resp(404, {"error": "no such route"})


class _Terminal(io.StringIO):
    """Captured text that reports a terminal."""

    def isatty(self):
        return True


@pytest.fixture
def serve(monkeypatch):
    """serve(**kwargs) makes a Backend the only server requests reach, with no sleeps."""
    monkeypatch.setattr("aim_sdk.oauth.decode_jwt_claims", lambda token: CLAIMS.get(token))

    def install(**kwargs):
        backend = Backend(**kwargs)
        monkeypatch.setattr(cli.requests, "post", backend.post)
        monkeypatch.setattr(cli.requests, "get", backend.get)
        monkeypatch.setattr(cli.time, "sleep", lambda seconds: None)
        monkeypatch.setattr(cli.webbrowser, "open", lambda url, *a, **k: backend.opened.append(url) or True)
        return backend

    return install


@pytest.fixture
def registration(monkeypatch):
    """OAuth registration, replaced: the tests end at the hand-off."""
    register = MagicMock(return_value=MagicMock())
    monkeypatch.setattr(client, "_register_via_oauth", register)
    return register


@pytest.fixture
def person(monkeypatch):
    """A person at a terminal: CI unset (conftest sets it) and standard input a terminal."""
    monkeypatch.delenv("CI", raising=False)
    monkeypatch.setattr(sys, "stdin", _Terminal())


def test_at_a_terminal_with_no_credentials_the_readme_call_signs_in_then_registers(serve, registration, person):
    backend = serve()

    secure("my-first-agent", aim_url=AIM, capabilities=["db:read"])

    assert backend.calls == [f"{AIM}/api/v1/oauth/device/code",
                             f"{AIM}/api/v1/oauth/device/token",
                             f"{AIM}/api/v1/oauth/device/token"]
    assert registration.call_count == 1
    handed = registration.call_args.kwargs
    assert handed["aim_url"] == AIM
    assert handed["sdk_creds"]["accessToken"] == "FAKE-ACCESS-TOKEN"
    assert handed["registration_data"]["capabilities"] == ["db:read"]

    path = credentials.SDK_CREDENTIALS_FILE
    stored = json.loads(path.read_text(encoding="utf-8"))
    assert stored["aimUrl"] == AIM
    assert stored["refreshToken"] == "FAKE-REFRESH-TOKEN"
    assert stored["accessToken"] == "FAKE-ACCESS-TOKEN"
    assert stored["userEmail"] == "dev@example.com"
    assert stored["organizationId"] == "o-1"
    assert credentials.detect_credential_origin(stored) == credentials.CredentialOrigin.LOGIN
    assert stat.S_IMODE(path.stat().st_mode) == 0o600


def test_the_sign_in_lines_go_to_standard_error_before_registration(serve, registration, person, capsys):
    serve()

    secure("my-first-agent", aim_url=AIM, capabilities=["db:read"], auto_detect=False)

    out, err = capsys.readouterr()
    lines = err.splitlines()
    start = lines.index(LEAD)
    assert lines[start:start + 7] == [
        LEAD,
        f"  Server: {AIM}",
        "  Open:   http://127.0.0.1:3000/device",
        "  Code:   BCDF-GHJK",
        WAIT.format(15),
        f"Signed in as dev@example.com on {AIM}.",
        "Credentials: ~/.aim/sdk_credentials.json (owner read/write only)",
    ]
    assert LEAD not in out


def test_without_aim_url_the_call_signs_in_to_the_default_server(serve, registration, person):
    backend = serve(base=cli.DEFAULT_AIM_URL)

    secure("my-first-agent", capabilities=["db:read"], auto_detect=False)

    assert backend.calls[0] == f"{cli.DEFAULT_AIM_URL}/api/v1/oauth/device/code"
    assert registration.call_args.kwargs["aim_url"] == cli.DEFAULT_AIM_URL


def test_an_empty_ci_variable_counts_as_unset(serve, registration, monkeypatch):
    backend = serve()
    monkeypatch.setenv("CI", "")
    monkeypatch.setattr(sys, "stdin", _Terminal())

    secure("my-first-agent", aim_url=AIM, capabilities=["db:read"], auto_detect=False)

    assert backend.calls[0] == f"{AIM}/api/v1/oauth/device/code"
    assert registration.call_count == 1


def test_in_an_ipython_kernel_the_call_signs_in(serve, registration, monkeypatch):
    backend = serve()
    monkeypatch.delenv("CI", raising=False)
    monkeypatch.setattr(sys, "stdin", io.StringIO())
    kernel_shell = types.SimpleNamespace(kernel=object())
    monkeypatch.setitem(sys.modules, "IPython", types.SimpleNamespace(get_ipython=lambda: kernel_shell))

    secure("my-first-agent", aim_url=AIM, capabilities=["db:read"], auto_detect=False)

    assert backend.calls[0] == f"{AIM}/api/v1/oauth/device/code"
    assert registration.call_count == 1


@pytest.mark.parametrize("ci, stdin, ipython", [
    pytest.param("true", _Terminal(), None, id="CI-set-at-a-terminal"),
    pytest.param(None, io.StringIO(), None, id="stdin-not-a-terminal"),
    pytest.param(None, None, None, id="no-stdin"),
    pytest.param(None, io.StringIO(), types.SimpleNamespace(get_ipython=lambda: types.SimpleNamespace()),
                 id="ipython-shell-without-a-kernel"),
])
def test_with_no_person_present_the_call_raises_before_any_request(serve, registration, monkeypatch,
                                                                    ci, stdin, ipython):
    backend = serve()
    if ci is None:
        monkeypatch.delenv("CI", raising=False)
    else:
        monkeypatch.setenv("CI", ci)
    monkeypatch.setattr(sys, "stdin", stdin)
    if ipython is None:
        monkeypatch.delitem(sys.modules, "IPython", raising=False)
    else:
        monkeypatch.setitem(sys.modules, "IPython", ipython)

    started = time.monotonic()
    with pytest.raises(ConfigurationError, match="No authentication credentials found"):
        secure("my-first-agent", aim_url=AIM, capabilities=["db:read"])
    assert time.monotonic() - started < 1

    assert backend.calls == []
    assert backend.opened == []
    assert registration.call_count == 0
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_stored_credentials_the_server_refuses_never_start_a_sign_in(serve, monkeypatch, person):
    backend = serve()
    assert credentials.save_sdk_credentials(
        {"aimUrl": AIM, "refreshToken": "FAKE-REFRESH-TOKEN", "accessToken": "FAKE-ACCESS-TOKEN"})
    refused = ConfigurationError("Cannot register new agent 'my-first-agent': OAuth access token failed.")
    monkeypatch.setattr(client, "_register_via_oauth", MagicMock(side_effect=refused))

    with pytest.raises(ConfigurationError) as excinfo:
        secure("my-first-agent", aim_url=AIM, capabilities=["db:read"], auto_detect=False)

    assert excinfo.value is refused
    assert backend.calls == []
    assert backend.opened == []


def test_an_api_key_never_starts_a_sign_in(serve, monkeypatch, person):
    backend = serve()
    by_key = MagicMock(return_value=MagicMock())
    monkeypatch.setattr(client, "_register_via_api_key", by_key)

    secure("my-first-agent", aim_url=AIM, api_key="FAKE-KEY-1234", capabilities=["db:read"], auto_detect=False)

    assert by_key.call_count == 1
    assert backend.calls == []


@pytest.mark.parametrize("answer, sentence", [
    pytest.param({"error": "access_denied"}, "The login was denied in the dashboard. Nothing was stored.",
                 id="denied"),
    pytest.param({"error": "expired_token"},
                 "The code expired before it was approved (waited 900s; login timed out).", id="expired"),
])
def test_a_denied_or_expired_code_raises_and_stores_nothing(serve, registration, person, answer, sentence):
    serve(polls=[PENDING, (403, answer)])

    with pytest.raises(AuthenticationError) as excinfo:
        secure("my-first-agent", aim_url=AIM, capabilities=["db:read"], auto_detect=False)

    assert sentence in str(excinfo.value)
    assert registration.call_count == 0
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_ctrl_c_during_the_wait_stops_the_call_and_stores_nothing(serve, registration, person, monkeypatch):
    serve()

    def interrupted(seconds):
        raise KeyboardInterrupt

    monkeypatch.setattr(cli.time, "sleep", interrupted)

    with pytest.raises(KeyboardInterrupt):
        secure("my-first-agent", aim_url=AIM, capabilities=["db:read"], auto_detect=False)

    assert registration.call_count == 0
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


@pytest.mark.parametrize("code, polls, sent, what", [
    pytest.param((502, {}), [], 1, "it answered HTTP 502", id="device-code-502"),
    pytest.param((503, {}), [], 1, "it answered HTTP 503", id="device-code-503"),
    pytest.param((504, {}), [], 1, "it answered HTTP 504", id="device-code-504"),
    pytest.param((200, DEVICE), [(503, {})], 2, "it answered HTTP 503", id="poll-503"),
])
def test_an_unavailable_server_ends_the_sign_in_and_stores_nothing(serve, registration, person,
                                                                   code, polls, sent, what):
    backend = serve(code=code, polls=polls)

    with pytest.raises(ConfigurationError) as excinfo:
        secure("my-first-agent", aim_url=AIM, capabilities=["db:read"], auto_detect=False)

    assert f"The AIM server at {AIM} is unavailable: {what}." in str(excinfo.value)
    assert len(backend.calls) == sent
    assert registration.call_count == 0
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_a_server_that_gives_no_http_answer_ends_the_sign_in_and_stores_nothing(serve, registration, person,
                                                                                monkeypatch):
    backend = serve()

    def refused(url, **kwargs):
        backend.calls.append(url)
        raise cli.requests.exceptions.ConnectionError("Connection refused")

    monkeypatch.setattr(cli.requests, "post", refused)

    with pytest.raises(ConfigurationError, match=r"is unavailable: it gave no HTTP answer"):
        secure("my-first-agent", aim_url=AIM, capabilities=["db:read"], auto_detect=False)

    assert backend.calls == [f"{AIM}/api/v1/oauth/device/code"]
    assert registration.call_count == 0
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_an_http_address_on_another_host_is_refused_before_any_request(serve, registration, person):
    backend = serve(base="http://aim.example.test:8080")

    with pytest.raises(ConfigurationError, match="only from this machine"):
        secure("my-first-agent", aim_url="http://aim.example.test:8080", capabilities=["db:read"],
               auto_detect=False)

    assert backend.calls == []
    assert registration.call_count == 0


def test_an_aim_url_that_is_not_an_http_url_is_refused_before_any_request(serve, registration, person):
    backend = serve()

    with pytest.raises(ConfigurationError, match=r"aim_url 'localhost:8080' is not an http\(s\) URL"):
        secure("my-first-agent", aim_url="localhost:8080", capabilities=["db:read"], auto_detect=False)

    assert backend.calls == []


@pytest.mark.parametrize("expires_in, minutes", [(900, 15), (5000, 15), (600, 10), (90, 2), (30, 1)])
def test_the_wait_line_gives_the_code_lifetime_in_minutes_capped_at_15(serve, expires_in, minutes):
    serve(code=(200, dict(DEVICE, expiresIn=expires_in)))
    out = io.StringIO()

    cli.sign_in_inline(AIM, "my-first-agent", out=out)

    assert WAIT.format(minutes) in out.getvalue().splitlines()


def test_the_lines_are_the_same_at_a_terminal_in_a_pipe_and_with_no_color(serve, monkeypatch):
    runs = []
    for out, no_color in ((_Terminal(), None), (io.StringIO(), None), (io.StringIO(), "1")):
        serve()
        if no_color:
            monkeypatch.setenv("NO_COLOR", no_color)
        cli.sign_in_inline(AIM, "my-first-agent", out=out)
        runs.append(out.getvalue())

    assert runs[0] == runs[1] == runs[2]
    assert runs[0].startswith(LEAD + "\n")


def test_a_browser_opens_on_the_complete_uri_only_when_standard_output_is_a_terminal(serve, monkeypatch):
    backend = serve()
    monkeypatch.setattr(sys, "stdout", _Terminal())
    cli.sign_in_inline(AIM, "my-first-agent", out=io.StringIO())
    assert backend.opened == [DEVICE["verificationUriComplete"]]

    backend = serve()
    monkeypatch.setattr(sys, "stdout", io.StringIO())
    cli.sign_in_inline(AIM, "my-first-agent", out=io.StringIO())
    assert backend.opened == []


def test_the_credentials_line_says_owner_only_only_when_the_file_mode_is_0600(serve, monkeypatch):
    serve()
    save = credentials.save_sdk_credentials

    def save_then_widen(stored):
        saved = save(stored)
        credentials.SDK_CREDENTIALS_FILE.chmod(0o644)
        return saved

    monkeypatch.setattr(credentials, "save_sdk_credentials", save_then_widen)
    out = io.StringIO()

    cli.sign_in_inline(AIM, "my-first-agent", out=out)

    assert "Credentials: ~/.aim/sdk_credentials.json" in out.getvalue().splitlines()


def test_server_chosen_values_are_printed_as_one_terminal_line_each(serve):
    serve(code=(200, dict(DEVICE, userCode="BCDF\x1b[2J\nGHJK")))
    out = io.StringIO()

    cli.sign_in_inline(AIM, "my-first-agent", out=out)

    assert "  Code:   BCDF�[2J�GHJK" in out.getvalue().splitlines()
    assert "\x1b" not in out.getvalue()
