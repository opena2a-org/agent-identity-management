"""
`aim-sdk login` opens a browser only when its standard output is a terminal
and --no-browser is not passed, and the page, the code and the wait reach the
output before the device-code poll starts.

With standard output a pipe, the login printed nothing while it polled for an
approval that can take up to 900 seconds: a log or a CI step showed neither
the page to open nor the code to approve.

The fake backend answers the pre-flight probe and the device-code request in
the shapes device_auth_handler.go emits; the poll is replaced by one that
returns the local deadline outcome at once, so no test waits.
"""
import io
import json
import sys
from contextlib import redirect_stdout
from types import SimpleNamespace
from unittest.mock import MagicMock

import pytest

from aim_sdk import cli

AIM = "http://127.0.0.1:8080"
DEVICE = {
    "deviceCode": "d" * 80,
    "userCode": "BCDF-GHJK",
    "verificationUri": "http://127.0.0.1:3000/device",
    "verificationUriComplete": "http://127.0.0.1:3000/device?user_code=BCDF-GHJK",
    "expiresIn": 600,
    "interval": 5,
}
WAITING = "Waiting for your approval (up to 10 min)."
NO_BROWSER = "No browser was opened"


def _resp(status, body):
    r = MagicMock()
    r.status_code = status
    r.content = json.dumps(body).encode()
    r.text = json.dumps(body)
    r.headers = {"content-type": "application/json"}
    r.json.return_value = body
    return r


class Output(io.StringIO):
    """A captured standard output that is not a terminal (a pipe, a log
    file), recording what had been written at each flush."""

    def __init__(self):
        super().__init__()
        self.flushed = []

    def flush(self):
        self.flushed.append(self.getvalue())
        super().flush()


class Terminal(Output):
    def isatty(self):
        return True


@pytest.fixture
def aim(monkeypatch):
    a = SimpleNamespace(device=dict(DEVICE), at_poll=[], browser=MagicMock(return_value=True))

    def post(url, json=None, headers=None, timeout=None):
        if url == f"{AIM}/api/v1/oauth/device/code":
            return _resp(200, a.device)
        return _resp(404, {"error": "no such route"})

    def poll(aim_url, device_code, interval, deadline):
        out = sys.stdout
        a.at_poll.append((out.getvalue(), list(out.flushed)))
        return {"error": "timeout"}

    monkeypatch.setattr("requests.post", post)
    monkeypatch.setattr("requests.get", lambda *args, **kw: _resp(200, {}))
    monkeypatch.setattr(cli.webbrowser, "open", a.browser)
    monkeypatch.setattr(cli, "poll_device_token", poll)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *args, **kw: None)
    monkeypatch.setattr("aim_sdk.credentials.save_sdk_credentials",
                        lambda c: pytest.fail("nothing may be stored on a timed-out login"))
    return a


def _login(out, **flags):
    with redirect_stdout(out):
        rc = cli.login(SimpleNamespace(url=AIM, force=True, **flags))
    return rc, out.getvalue()


def _no_browser_line(out):
    lines = [line for line in out.splitlines() if line.startswith(NO_BROWSER)]
    assert len(lines) == 1, f"expected one line starting {NO_BROWSER!r}:\n{out}"
    return lines[0]


def test_output_that_is_not_a_terminal_opens_no_browser_and_says_what_to_do(aim):
    rc, out = _login(Output())
    aim.browser.assert_not_called()
    assert DEVICE["verificationUri"] in out
    assert DEVICE["userCode"] in out
    line = _no_browser_line(out)
    assert "standard output is not a terminal" in line
    assert "open the page above and approve the code" in line
    assert "waits up to 600 seconds" in line
    assert "exits 1" in line
    assert "—" not in line
    assert rc == 1, out
    assert "login timed out" in out


def test_a_terminal_opens_the_complete_uri_once(aim):
    rc, out = _login(Terminal())
    aim.browser.assert_called_once_with(DEVICE["verificationUriComplete"])
    assert NO_BROWSER not in out
    assert DEVICE["verificationUri"] in out
    assert DEVICE["userCode"] in out
    assert rc == 1, out


def test_no_browser_flag_at_a_terminal_opens_no_browser_and_names_the_flag(aim):
    rc, out = _login(Terminal(), no_browser=True)
    aim.browser.assert_not_called()
    assert DEVICE["verificationUri"] in out
    assert DEVICE["userCode"] in out
    line = _no_browser_line(out)
    assert "--no-browser" in line
    assert "waits up to 600 seconds" in line
    assert rc == 1, out


def test_the_wait_named_is_the_local_maximum_when_the_code_lives_longer(aim):
    aim.device["expiresIn"] = cli.LOGIN_MAX_WAIT_SECONDS * 4
    rc, out = _login(Output())
    assert f"waits up to {cli.LOGIN_MAX_WAIT_SECONDS} seconds" in _no_browser_line(out)
    assert rc == 1, out


@pytest.mark.parametrize("make_output", [Output, Terminal], ids=["not-a-terminal", "terminal"])
def test_everything_printed_before_the_poll_is_flushed_before_the_poll_starts(aim, make_output):
    rc, out = _login(make_output())
    assert len(aim.at_poll) == 1
    written, flushed = aim.at_poll[0]
    assert len(flushed) > 0, "no flush before the poll: a pipe shows nothing while the login waits"
    assert flushed[-1] == written, "text written after the last flush before the poll"
    assert WAITING in flushed[-1]
    assert DEVICE["verificationUri"] in flushed[-1]
    assert DEVICE["userCode"] in flushed[-1]


def test_the_parser_accepts_no_browser(monkeypatch):
    seen = []
    monkeypatch.setattr(cli, "login", lambda args: seen.append(args) or 0)
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--no-browser", "--url", "http://127.0.0.1:1"])
    assert cli.main() == 0
    assert seen[0].no_browser is True
    assert seen[0].url == "http://127.0.0.1:1"


def test_the_parser_leaves_no_browser_off_by_default(monkeypatch):
    seen = []
    monkeypatch.setattr(cli, "login", lambda args: seen.append(args) or 0)
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--url", "http://127.0.0.1:1"])
    assert cli.main() == 0
    assert seen[0].no_browser is False


def test_login_help_names_the_flag_the_terminal_rule_and_the_maximum_wait(monkeypatch, capsys):
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--help"])
    with pytest.raises(SystemExit) as stop:
        cli.main()
    assert stop.value.code == 0
    text = " ".join(capsys.readouterr().out.split())
    assert "--no-browser" in text
    assert "only when standard output is a terminal" in text
    assert f"the lesser of the code's lifetime the server states and {cli.LOGIN_MAX_WAIT_SECONDS} seconds" in text
