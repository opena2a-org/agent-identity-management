"""
`aim-sdk login` prints the sign-in block secure() prints, from the same
function (cli.sign_in_block), with the lead line `Sign in to AIM.`: the
server, the page to open, the code, and the wait line. Once the credentials
are stored it prints the two lines secure() prints (cli.signed_in_lines), the
credentials file written with `~` for the home directory.

Before this change, the same login printed a box banner and a `Server:` line,
then "To sign in, open this page in your browser and approve the code:", the
page and the code on lines of their own, and "Waiting for approval... (the
code expires in 900s; Ctrl+C to cancel)". After the approval it printed
"Successfully authenticated." and the credentials file as an absolute path.
The user code was printed as the server sent it.

The fake backend answers the pre-flight probe, the device-code request and a
scripted sequence of poll answers with the fields login() reads. The tokens
are opaque strings; decode_jwt_claims is replaced with a table.
"""

import io
import json
import sys
from contextlib import redirect_stdout
from unittest.mock import MagicMock

import pytest

from aim_sdk import cli

AIM = "http://127.0.0.1:8080"
DEVICE = {
    "deviceCode": "fake-device-code",
    "userCode": "BCDF-GHJK",
    "verificationUri": "http://127.0.0.1:3000/device",
    "verificationUriComplete": "http://127.0.0.1:3000/device?user_code=BCDF-GHJK",
    "expiresIn": 900,
    "interval": 5,
}
CLAIMS = {"FAKE-ACCESS-TOKEN": {"user_id": "u-1", "organization_id": "o-1", "email": "dev@example.com"}}
PAIR = {"accessToken": "FAKE-ACCESS-TOKEN", "refreshToken": "FAKE-REFRESH-TOKEN",
        "tokenType": "Bearer", "expiresIn": 7200}
PENDING = (403, {"error": "authorization_pending"})
LEAD = "Sign in to AIM."
WAIT = ("Waiting for your approval (up to {} min). Ctrl+C stops the wait. "
        "Nothing is stored until you approve.")
BLOCK = [
    LEAD,
    f"  Server: {AIM}",
    "  Open:   http://127.0.0.1:3000/device",
    "  Code:   BCDF-GHJK",
    WAIT.format(15),
]
SIGNED_IN = [
    f"Signed in as dev@example.com on {AIM}.",
    "Credentials: ~/.aim/sdk_credentials.json (owner read/write only)",
]
BOX = set("╔╗╚╝║═")


def _resp(status, body):
    r = MagicMock()
    r.status_code = status
    r.content = json.dumps(body).encode()
    r.text = json.dumps(body)
    r.headers = {"content-type": "application/json"}
    r.json.return_value = body
    return r


class Backend:
    """The probe, the device-code answer, then scripted poll answers."""

    def __init__(self, code=(200, DEVICE), polls=(PENDING, (200, PAIR))):
        self.code, self.polls, self.opened = code, list(polls), []

    def get(self, url, **kwargs):
        return _resp(200, {})

    def post(self, url, json=None, headers=None, timeout=None):
        if url == f"{AIM}/api/v1/oauth/device/code":
            return _resp(*self.code)
        if url == f"{AIM}/api/v1/oauth/device/token":
            return _resp(*(self.polls.pop(0) if self.polls else (200, PAIR)))
        return _resp(404, {"error": "no such route"})


class Terminal(io.StringIO):
    """Captured standard output that reports a terminal."""

    def isatty(self):
        return True


@pytest.fixture
def serve(monkeypatch):
    """serve(**kwargs) makes a Backend the only server requests reach, with no sleeps."""
    monkeypatch.setattr("aim_sdk.oauth.decode_jwt_claims", lambda token: CLAIMS.get(token))

    def install(**kwargs):
        backend = Backend(**kwargs)
        monkeypatch.setattr(cli.requests, "get", backend.get)
        monkeypatch.setattr(cli.requests, "post", backend.post)
        monkeypatch.setattr(cli.time, "sleep", lambda seconds: None)
        monkeypatch.setattr(cli.webbrowser, "open", lambda url, *a, **k: backend.opened.append(url) or True)
        return backend

    return install


def _login(monkeypatch, out, *extra, url=AIM):
    """`aim-sdk login --url <url> --force <extra>` through the entry point, writing to `out`."""
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--url", url, "--force", *extra])
    with redirect_stdout(out):
        rc = cli.main()
    return rc, out.getvalue()


def test_login_opens_with_the_sign_in_block_and_prints_no_banner(serve, monkeypatch):
    backend = serve()

    rc, out = _login(monkeypatch, Terminal())

    assert rc == 0, out
    assert out.splitlines()[:5] == BLOCK
    assert "AIM SDK Login" not in out
    assert not BOX & set(out)
    assert backend.opened == [DEVICE["verificationUriComplete"]]


def test_after_the_approval_login_prints_the_signed_in_lines_then_one_blank_line(serve, monkeypatch,
                                                                                   isolated_home):
    serve()

    rc, out = _login(monkeypatch, Terminal())

    assert rc == 0, out
    assert out.splitlines()[5:8] == SIGNED_IN + [""]
    assert str(isolated_home) not in out
    assert "Successfully authenticated." not in out


def test_login_and_secure_print_the_same_lines_but_the_lead(serve, monkeypatch):
    serve()
    rc, out = _login(monkeypatch, Terminal())
    assert rc == 0, out

    serve()
    inline = io.StringIO()
    cli.sign_in_inline(AIM, "my-first-agent", out=inline)

    from_login = out.splitlines()[:7]
    from_secure = inline.getvalue().splitlines()
    assert from_login[0] == LEAD
    assert from_secure[0] == cli.INLINE_SIGN_IN_LEAD.format(agent="my-first-agent")
    assert from_login[1:] == from_secure[1:] == BLOCK[1:] + SIGNED_IN


def test_login_renders_the_block_with_sign_in_block(serve, monkeypatch):
    serve()
    leads = []
    render = cli.sign_in_block

    def recorded(lead, *args):
        leads.append(lead)
        return render(lead, *args)

    monkeypatch.setattr(cli, "sign_in_block", recorded)

    rc, out = _login(monkeypatch, Terminal())

    assert rc == 0, out
    assert leads == [LEAD]


def test_the_block_is_the_same_at_a_terminal_in_a_pipe_and_with_no_color(serve, monkeypatch):
    blocks = []
    for make_output, no_color in ((Terminal, None), (io.StringIO, None), (io.StringIO, "1")):
        serve()
        if no_color:
            monkeypatch.setenv("NO_COLOR", no_color)
        rc, out = _login(monkeypatch, make_output())
        assert rc == 0, out
        blocks.append(out.splitlines(keepends=True)[:5])

    assert blocks[0] == blocks[1] == blocks[2]
    assert [line.rstrip("\n") for line in blocks[0]] == BLOCK


@pytest.mark.parametrize("expires_in, minutes", [(900, 15), (5000, 15), (600, 10), (90, 2), (30, 1)])
def test_the_wait_line_gives_the_code_lifetime_in_minutes_capped_at_15(serve, monkeypatch, expires_in, minutes):
    serve(code=(200, dict(DEVICE, expiresIn=expires_in)))

    rc, out = _login(monkeypatch, Terminal())

    assert rc == 0, out
    assert out.splitlines()[4] == WAIT.format(minutes)


def test_server_chosen_values_are_printed_as_one_terminal_line_each(serve, monkeypatch):
    serve(code=(200, dict(DEVICE, userCode="BCDF\x1b[2J\nGHJK")))

    rc, out = _login(monkeypatch, Terminal())

    assert rc == 0, out
    assert out.splitlines()[3] == "  Code:   BCDF�[2J�GHJK"
    assert "\x1b" not in out


def test_a_login_that_ends_before_a_code_is_shown_prints_one_line_and_none_of_the_block(serve, monkeypatch):
    serve(code=(500, {"error": "server_error", "errorDescription": "Failed to initiate device authorization"}))

    rc, out = _login(monkeypatch, Terminal())

    assert rc == 1, out
    assert out.splitlines() == ["Could not start the device login: Failed to initiate device authorization"]


def test_allow_http_prints_the_warning_then_one_blank_line_then_the_block(serve, monkeypatch):
    remote = "http://aim.example.test:8080"
    backend = serve()
    monkeypatch.setattr(cli.requests, "post", lambda url, **kw: backend.post(url.replace(remote, AIM), **kw))

    rc, out = _login(monkeypatch, Terminal(), "--allow-http", url=remote)

    assert rc == 0, out
    lines = out.splitlines()
    assert lines[0].startswith(f"Warning: {remote} is not this machine"), out
    assert lines[1] == ""
    assert lines[2:4] == [LEAD, f"  Server: {remote}"]
