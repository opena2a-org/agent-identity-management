"""
`aim-sdk demo` signs this machine in on demand. With no SDK credentials
stored, a person present under the rule secure() applies (CI unset or empty,
standard input a terminal or an IPython kernel) and no --ci, the demo runs the
sign-in secure() runs, from the same function (cli.sign_in_inline), with the
lines on standard output, then registers the demo agent with the credentials
that sign-in stored.

Before this change the demo printed "Not authenticated. The demo registers an
agent in your own organization, so it needs a login first:" and exited 1 at a
terminal too, so `aim-sdk demo` needed `aim-sdk login` ahead of it.

The fake backend answers the device-code request and a scripted sequence of
poll answers in the shapes the backend's device_auth_service.go and
device_auth_handler.go define. secure() is replaced: these tests end at the
hand-off to registration and record what was stored when it came.
"""

import io
import json
import sys
import types
from unittest.mock import MagicMock

import pytest

from aim_sdk import cli, credentials
from aim_sdk import demo as demo_mod
from aim_sdk.client import AgentType

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
LEAD = "demo-agent needs an AIM sign-in on this machine. Sign in once to continue."
WAIT = ("Waiting for your approval (up to 15 min). Ctrl+C stops the wait. "
        "Nothing is stored until you approve.")
NOT_AUTHENTICATED = [
    "Not authenticated. The demo registers an agent in your own",
    "organization, so it needs a login first:",
    "",
    "    aim-sdk login",
    "    aim-sdk demo",
]


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
def person(monkeypatch):
    """A person at a terminal: CI unset (conftest sets it) and standard input a terminal."""
    monkeypatch.delenv("CI", raising=False)
    monkeypatch.setattr(sys, "stdin", _Terminal())


@pytest.fixture
def registered(monkeypatch):
    """secure(), replaced: each call records the name, the arguments and the stored SDK credentials."""
    calls = []
    agent = MagicMock()
    agent.agent_id = "agent-123"
    # perform_action returns the function unchanged so the scripted pass can call it.
    agent.perform_action.return_value = lambda fn: fn

    def fake_secure(name, **kwargs):
        calls.append((name, kwargs, credentials.load_sdk_credentials()))
        return agent

    monkeypatch.setattr("aim_sdk.secure", fake_secure)
    return calls


def test_at_a_terminal_with_no_sign_in_the_demo_signs_in_then_registers(serve, person, registered, capsys):
    backend = serve()

    rc = demo_mod.run(url=AIM)

    assert rc == 0
    # The device grant, then the GET the demo sends before it prints anything
    # about the server (demo._unreachable_reason).
    assert backend.calls == [f"{AIM}/api/v1/oauth/device/code",
                             f"{AIM}/api/v1/oauth/device/token",
                             f"{AIM}/api/v1/oauth/device/token",
                             AIM]
    assert len(registered) == 1
    name, kwargs, stored = registered[0]
    assert name == demo_mod.DEMO_AGENT_NAME
    assert kwargs["agent_type"] == AgentType.DEMO
    assert stored["aimUrl"] == AIM
    assert stored["accessToken"] == "FAKE-ACCESS-TOKEN"
    assert stored["refreshToken"] == "FAKE-REFRESH-TOKEN"


def test_the_sign_in_lines_go_to_standard_output_before_the_banner(serve, person, registered, capsys):
    backend = serve()

    demo_mod.run(url=AIM)

    out, err = capsys.readouterr()
    lines = out.splitlines()
    assert lines[:7] == [
        LEAD,
        f"  Server: {AIM}",
        "  Open:   http://127.0.0.1:3000/device",
        "  Code:   BCDF-GHJK",
        WAIT,
        f"Signed in as dev@example.com on {AIM}.",
        "Credentials: ~/.aim/sdk_credentials.json (owner read/write only)",
    ]
    assert out.index(LEAD) < out.index("AIM DEMO AGENT")
    assert NOT_AUTHENTICATED[0] not in out
    assert LEAD not in err
    # Standard output is captured here, not a terminal: no browser opens.
    assert backend.opened == []


def test_a_browser_opens_on_the_complete_uri_when_standard_output_is_a_terminal(serve, person, registered,
                                                                                 monkeypatch):
    backend = serve()
    monkeypatch.setattr(sys, "stdout", _Terminal())

    assert demo_mod.run(url=AIM) == 0

    assert backend.opened == [DEVICE["verificationUriComplete"]]


def test_without_url_the_demo_signs_in_to_the_default_server(serve, person, registered):
    backend = serve(base=cli.DEFAULT_AIM_URL)

    assert demo_mod.run() == 0

    assert backend.calls[0] == f"{cli.DEFAULT_AIM_URL}/api/v1/oauth/device/code"
    assert registered[0][2]["aimUrl"] == cli.DEFAULT_AIM_URL


def test_the_cli_command_signs_in(serve, person, registered, monkeypatch):
    backend = serve()
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "demo", "--url", AIM])

    assert cli.main() == 0

    assert backend.calls[0] == f"{AIM}/api/v1/oauth/device/code"
    assert len(registered) == 1


def test_the_interactive_menu_signs_in_first_too(serve, person, registered, monkeypatch):
    backend = serve()
    monkeypatch.setattr(demo_mod, "_interactive_loop", lambda ctx: 0)

    assert demo_mod.run(interactive=True, url=AIM) == 0

    assert backend.calls[0] == f"{AIM}/api/v1/oauth/device/code"
    assert len(registered) == 1


def test_an_empty_ci_variable_counts_as_unset(serve, registered, monkeypatch):
    backend = serve()
    monkeypatch.setenv("CI", "")
    monkeypatch.setattr(sys, "stdin", _Terminal())

    assert demo_mod.run(url=AIM) == 0

    assert backend.calls[0] == f"{AIM}/api/v1/oauth/device/code"


def test_in_an_ipython_kernel_the_demo_signs_in(serve, registered, monkeypatch):
    backend = serve()
    monkeypatch.delenv("CI", raising=False)
    monkeypatch.setattr(sys, "stdin", io.StringIO())
    kernel_shell = types.SimpleNamespace(kernel=object())
    monkeypatch.setitem(sys.modules, "IPython", types.SimpleNamespace(get_ipython=lambda: kernel_shell))

    assert demo_mod.run(url=AIM) == 0

    assert backend.calls[0] == f"{AIM}/api/v1/oauth/device/code"


@pytest.mark.parametrize("ci_option, ci_variable, stdin", [
    pytest.param(True, None, _Terminal(), id="--ci-at-a-terminal"),
    pytest.param(False, "true", _Terminal(), id="CI-set-at-a-terminal"),
    pytest.param(False, None, io.StringIO(), id="stdin-not-a-terminal"),
    pytest.param(False, None, None, id="no-stdin"),
])
def test_with_ci_or_no_person_present_the_demo_needs_a_login_first(serve, registered, monkeypatch, capsys,
                                                                   ci_option, ci_variable, stdin):
    backend = serve()
    if ci_variable is None:
        monkeypatch.delenv("CI", raising=False)
    else:
        monkeypatch.setenv("CI", ci_variable)
    monkeypatch.setattr(sys, "stdin", stdin)
    monkeypatch.delitem(sys.modules, "IPython", raising=False)

    rc = demo_mod.run(ci=ci_option, url=AIM)

    assert rc == 1
    assert capsys.readouterr().out.splitlines() == NOT_AUTHENTICATED
    assert backend.calls == []
    assert backend.opened == []
    assert registered == []
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_a_stored_sign_in_never_starts_another(serve, person, registered):
    backend = serve()
    assert credentials.save_sdk_credentials({
        "aimUrl": AIM,
        "refreshToken": "FAKE-REFRESH-TOKEN",
        "accessToken": "FAKE-ACCESS-TOKEN",
        "userId": "u-1",
        "userEmail": "dev@example.com",
        "organizationId": "o-1",
    })

    assert demo_mod.run(url=AIM) == 0

    # Only the GET the demo sends before it prints anything about the server:
    # no device-code request.
    assert backend.calls == [AIM]
    assert backend.opened == []
    assert len(registered) == 1


@pytest.mark.parametrize("answer, sentence", [
    pytest.param({"error": "access_denied"}, "The login was denied in the dashboard. Nothing was stored.",
                 id="denied"),
    pytest.param({"error": "expired_token"},
                 "The code expired before it was approved (waited 900s; login timed out). "
                 "Run aim-sdk login to get a new code.", id="expired"),
])
def test_a_denied_or_expired_code_ends_the_demo_with_exit_1(serve, person, registered, capsys,
                                                            answer, sentence):
    serve(polls=[PENDING, (403, answer)])

    rc = demo_mod.run(url=AIM)

    out = capsys.readouterr().out
    assert rc == 1
    assert out.splitlines()[4:] == [WAIT, sentence]
    assert "AIM DEMO AGENT" not in out
    assert registered == []
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


@pytest.mark.parametrize("code, polls, lines_before, what", [
    pytest.param((502, {}), [], 0, "it answered HTTP 502", id="device-code-502"),
    pytest.param((503, {}), [], 0, "it answered HTTP 503", id="device-code-503"),
    pytest.param((504, {}), [], 0, "it answered HTTP 504", id="device-code-504"),
    pytest.param((200, DEVICE), [(503, {})], 5, "it answered HTTP 503", id="poll-503"),
])
def test_an_unavailable_server_ends_the_demo_with_exit_1(serve, person, registered, capsys,
                                                         code, polls, lines_before, what):
    serve(code=code, polls=polls)

    rc = demo_mod.run(url=AIM)

    out = capsys.readouterr().out
    assert rc == 1
    assert out.splitlines()[lines_before:] == [
        f"The AIM server at {AIM} is unavailable: {what}.",
        "No sign-in was completed and nothing was stored. Run the same command again later.",
    ]
    assert registered == []
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_a_server_that_gives_no_http_answer_ends_the_demo_with_exit_1(serve, person, registered, capsys,
                                                                      monkeypatch):
    serve()

    def refused(url, **kwargs):
        raise cli.requests.exceptions.ConnectionError("connection refused")

    monkeypatch.setattr(cli.requests, "post", refused)

    rc = demo_mod.run(url=AIM)

    lines = capsys.readouterr().out.splitlines()
    assert rc == 1
    assert lines[0].startswith(f"The AIM server at {AIM} is unavailable: it gave no HTTP answer")
    assert lines[1:] == ["No sign-in was completed and nothing was stored. Run the same command again later."]
    assert registered == []
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_ctrl_c_during_the_wait_ends_the_demo_with_exit_1(serve, person, registered, capsys, monkeypatch):
    serve()

    def interrupted(seconds):
        raise KeyboardInterrupt

    monkeypatch.setattr(cli.time, "sleep", interrupted)

    rc = demo_mod.run(url=AIM)

    out = capsys.readouterr().out
    assert rc == 1
    assert out.splitlines()[4:] == [WAIT, "", "", "Login cancelled."]
    assert registered == []
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_an_http_address_on_another_host_is_refused_before_any_request(serve, person, registered, capsys):
    backend = serve(base="http://aim.example.test:8080")

    rc = demo_mod.run(url="http://aim.example.test:8080")

    out = capsys.readouterr().out
    assert rc == 1
    assert out.startswith("Error: aim-sdk login receives your access and refresh tokens over http: "
                          "only from this machine")
    assert backend.calls == []
    assert registered == []
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_a_url_that_is_not_an_http_url_is_refused_naming_the_demo_command(serve, person, registered, capsys):
    backend = serve()

    rc = demo_mod.run(url="aim.example.test")

    assert rc == 1
    assert capsys.readouterr().out.splitlines() == [
        "Error: --url 'aim.example.test' is not an http(s) URL.",
        "Pass the server's http(s) address, for example:",
        "  aim-sdk demo --url https://aim.example.com",
        f"or omit --url to use {cli.DEFAULT_AIM_URL}.",
    ]
    assert backend.calls == []
    assert registered == []
    assert not credentials.SDK_CREDENTIALS_FILE.exists()


def test_cleanup_with_no_sign_in_still_needs_a_login_first(serve, person, capsys):
    backend = serve()

    rc = demo_mod.run(cleanup=True, url=AIM)

    assert rc == 1
    assert capsys.readouterr().out == "Not authenticated. Run: aim-sdk login\n"
    assert backend.calls == []


@pytest.mark.parametrize("show_help", [
    pytest.param(lambda: cli.main(), id="aim-sdk-demo"),
    pytest.param(lambda: demo_mod.main(["--help"]), id="python-m-aim_sdk.demo"),
])
def test_the_url_help_names_the_server_a_sign_in_goes_to(monkeypatch, capsys, show_help):
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "demo", "--help"])

    with pytest.raises(SystemExit) as exc:
        show_help()

    assert exc.value.code == 0
    assert (f"or {cli.DEFAULT_AIM_URL} when the demo signs this machine in"
            in " ".join(capsys.readouterr().out.split()))
