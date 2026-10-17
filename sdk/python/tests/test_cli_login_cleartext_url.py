"""
`aim-sdk login` receives the access and refresh tokens over `http:` only from
this machine, unless --allow-http is passed.

An `http:` server address is accepted when its host is `localhost`,
`127.0.0.1` or `::1`, read from the URL text and never resolved: the rule
`aim-sdk init` applies, from the same helper. Any other `http:` address ends
the command with exit 1 before the reachability probe: no request is made,
nothing is saved, and the output names the https form of the command and
--allow-http. With --allow-http the command prints what the flag accepts,
flushed before its first request, and continues. An address with a
backslash, whitespace or a control character is refused with or without the
flag. `https:` addresses and loopback `http:` addresses are unchanged.

`requests.get` and `requests.post` are replaced in every test: `no_network`
refuses any request, and `stack` answers the probe, the device-code request
and the token poll and records each request with what standard output held
and had flushed at that moment.
"""
import base64
import io
import json
import sys
from contextlib import redirect_stdout
from types import SimpleNamespace
from unittest.mock import MagicMock

import pytest

from aim_sdk import cli

DEVICE = {
    "deviceCode": "d" * 80,
    "userCode": "BCDF-GHJK",
    "verificationUri": "http://127.0.0.1:3000/device",
    "verificationUriComplete": "http://127.0.0.1:3000/device?user_code=BCDF-GHJK",
    "expiresIn": 900,
    "interval": 5,
}
REFUSED = "is not this machine. Nothing was sent or saved."
WARNING = "Warning: "
UNENCRYPTED = "will cross the network unencrypted"


def _jwt(claims):
    def seg(d):
        return base64.urlsafe_b64encode(json.dumps(d).encode()).decode().rstrip("=")
    return f"{seg({'alg': 'HS256', 'typ': 'JWT'})}.{seg(claims)}.c2ln"


PAIR = {
    "accessToken": _jwt({"user_id": "u-1", "organization_id": "o-1", "email": "dev@example.com",
                         "typ": "access", "exp": 4102444800}),
    "refreshToken": _jwt({"user_id": "u-1", "typ": "refresh", "exp": 4102444800, "jti": "j-1"}),
    "tokenType": "Bearer",
    "expiresIn": 7200,
}


def _resp(status, body):
    r = MagicMock()
    r.status_code = status
    r.content = json.dumps(body).encode()
    r.text = json.dumps(body)
    r.headers = {"content-type": "application/json"}
    r.json.return_value = body
    return r


class Output(io.StringIO):
    """A captured standard output that is not a terminal, recording what had
    been written at each flush."""

    def __init__(self):
        super().__init__()
        self.flushed = []

    def flush(self):
        self.flushed.append(self.getvalue())
        super().flush()


@pytest.fixture
def no_network(monkeypatch):
    calls = []

    def refuse(*args, **kwargs):
        calls.append(args)
        raise AssertionError("login made a request for an address it must refuse")

    monkeypatch.setattr(cli.requests, "get", refuse)
    monkeypatch.setattr(cli.requests, "post", refuse)
    monkeypatch.setattr(cli.webbrowser, "open", refuse)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *a, **k: None)
    monkeypatch.setattr("aim_sdk.credentials.save_sdk_credentials",
                        lambda c: pytest.fail("nothing may be saved for a refused address"))
    return calls


@pytest.fixture
def stack(monkeypatch):
    s = SimpleNamespace(calls=[], saved={}, opened=[])

    def record(method, url):
        out = sys.stdout
        s.calls.append(SimpleNamespace(method=method, url=url, written=out.getvalue(),
                                       flushed=list(getattr(out, "flushed", []))))

    def get(url, *args, **kwargs):
        record("GET", url)
        return _resp(200, {})

    def post(url, json=None, headers=None, timeout=None):
        record("POST", url)
        if url.endswith("/api/v1/oauth/device/code"):
            return _resp(200, DEVICE)
        if url.endswith("/api/v1/oauth/device/token"):
            return _resp(200, PAIR)
        return _resp(404, {"error": "no such route"})

    monkeypatch.setattr(cli.requests, "get", get)
    monkeypatch.setattr(cli.requests, "post", post)
    monkeypatch.setattr(cli.time, "sleep", lambda n: None)
    monkeypatch.setattr(cli.webbrowser, "open", lambda url, *a, **k: s.opened.append(url) or True)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *a, **k: None)
    monkeypatch.setattr("aim_sdk.credentials.save_sdk_credentials", lambda c: s.saved.update(c) or True)
    return s


def _login(url, **flags):
    out = Output()
    with redirect_stdout(out):
        rc = cli.login(SimpleNamespace(url=url, force=True, **flags))
    return rc, out.getvalue()


def _warning_line(out):
    lines = [line for line in out.splitlines() if line.startswith(WARNING)]
    assert len(lines) == 1, f"expected one line starting {WARNING!r}:\n{out}"
    return lines[0]


# (address, host as shown, host and port as the example commands show them)
NOT_THIS_MACHINE = [
    ("http://aim.example.test:8080", "aim.example.test", "aim.example.test:8080"),
    ("http://10.0.0.5:8080", "10.0.0.5", "10.0.0.5:8080"),
    ("http://[2001:db8::1]:8080", "[2001:db8::1]", "[2001:db8::1]:8080"),
    ("http://localhost.example.test", "localhost.example.test", "localhost.example.test"),
    ("http://127.0.0.1.example.test", "127.0.0.1.example.test", "127.0.0.1.example.test"),
]


# --- an http: address whose host is not this machine is refused --------------

@pytest.mark.parametrize("url, host, host_port", NOT_THIS_MACHINE)
def test_http_to_another_host_exits_1_before_any_request(no_network, url, host, host_port):
    rc, out = _login(url)

    assert rc == 1, out
    assert no_network == []
    assert ("Error: aim-sdk login receives your access and refresh tokens over http: only from this machine "
            f"(localhost, 127.0.0.1 or [::1]), and {host} {REFUSED}") in out
    assert f"  aim-sdk login --url https://{host_port}\n" in out
    assert f"  aim-sdk login --url http://{host_port} --allow-http\n" in out
    assert "TLS-terminating proxy" in out
    assert "AIM SDK Login" not in out
    assert UNENCRYPTED not in out


def test_the_refusal_names_the_host_without_the_address_user_part(no_network):
    rc, out = _login("http://FAKEUSER:FAKEPASS@aim.example.test")

    assert rc == 1, out
    assert no_network == []
    assert f"aim.example.test {REFUSED}" in out
    assert "FAKEUSER" not in out
    assert "FAKEPASS" not in out


# --- --allow-http accepts it, and says so before the first request -----------

@pytest.mark.parametrize("url, host, host_port", NOT_THIS_MACHINE)
def test_allow_http_prints_the_warning_before_the_first_request_then_signs_in(stack, url, host, host_port):
    rc, out = _login(url, allow_http=True)

    assert rc == 0, out
    assert [(c.method, c.url) for c in stack.calls[:2]] == [
        ("GET", url), ("POST", f"{url}/api/v1/oauth/device/code")]
    line = _warning_line(out)
    assert line == (f"Warning: http://{host_port} is not this machine and the address is http:. "
                    "The access and refresh tokens this sign-in receives will cross the network "
                    "unencrypted, and anyone on the path can read and reuse them. --allow-http accepted that.")
    first = stack.calls[0]
    assert first.flushed and line in first.flushed[-1], "the warning was not flushed before the probe"
    assert line in stack.calls[1].written
    assert REFUSED not in out
    assert out.count(UNENCRYPTED) == 1
    assert stack.saved["aimUrl"] == url
    assert stack.saved["refreshToken"] == PAIR["refreshToken"]


def test_the_warning_names_the_host_without_the_address_user_part(stack):
    rc, out = _login("http://FAKEUSER:FAKEPASS@aim.example.test", allow_http=True)

    assert rc == 0, out
    line = _warning_line(out)
    assert line.startswith("Warning: http://aim.example.test is not this machine")
    assert "FAKEUSER" not in line
    assert "FAKEPASS" not in line


# --- an address urlsplit and requests could read different hosts from --------

@pytest.mark.parametrize("flags", [{}, {"allow_http": True}], ids=["no-flag", "allow-http"])
@pytest.mark.parametrize("url, reason", [
    ("http://aim.example.test\\@127.0.0.1", "Error: --url has a backslash."),
    ("http://local\thost:8080", "Error: --url has whitespace or a control character."),
])
def test_a_backslash_or_whitespace_is_refused_with_or_without_the_flag(no_network, url, reason, flags):
    rc, out = _login(url, **flags)

    assert rc == 1, out
    assert no_network == []
    assert reason in out
    assert "aim.example.test" not in out
    assert UNENCRYPTED not in out


# --- http: on a loopback host and https: on any host are unchanged ------------

@pytest.mark.parametrize("flags", [{}, {"allow_http": True}], ids=["no-flag", "allow-http"])
@pytest.mark.parametrize("url", [
    "http://localhost:8080",
    "http://127.0.0.1:8080",
    "http://[::1]:8080",
    "http://LOCALHOST:8080",
    "https://aim.example.test",
])
def test_a_loopback_http_or_an_https_address_signs_in_with_no_warning(stack, url, flags):
    rc, out = _login(url, **flags)

    assert rc == 0, out
    assert ("POST", f"{url}/api/v1/oauth/device/code") in [(c.method, c.url) for c in stack.calls]
    assert stack.saved["aimUrl"] == url
    assert WARNING not in out
    assert UNENCRYPTED not in out
    assert "is not this machine" not in out


# --- init shares the rule: its example now keeps the port ---------------------

def test_init_refusal_example_keeps_the_port_the_address_names(monkeypatch, capsys):
    monkeypatch.setattr(cli.requests, "post",
                        lambda *a, **k: pytest.fail("init made a request for an address it must refuse"))
    token = "".join([cli.BOOTSTRAP_TOKEN_PREFIX, "P0rtKept", "q" * 30])

    assert cli.init(SimpleNamespace(token=token, url="http://aim.example.test:8080", name="agent")) == 1

    out = capsys.readouterr().out
    assert "aim-sdk init sends the bootstrap token over http: only to this machine" in out
    assert "  aim-sdk init --url https://aim.example.test:8080\n" in out
    assert "--allow-http" not in out


# --- the parser and the help ----------------------------------------------------

def test_the_parser_accepts_allow_http(monkeypatch):
    seen = []
    monkeypatch.setattr(cli, "login", lambda args: seen.append(args) or 0)
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--allow-http", "--url", "http://127.0.0.1:1"])
    assert cli.main() == 0
    assert seen[0].allow_http is True
    assert seen[0].url == "http://127.0.0.1:1"


def test_the_parser_leaves_allow_http_off_by_default(monkeypatch):
    seen = []
    monkeypatch.setattr(cli, "login", lambda args: seen.append(args) or 0)
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--url", "http://127.0.0.1:1"])
    assert cli.main() == 0
    assert seen[0].allow_http is False


def test_login_help_names_the_http_rule_and_allow_http(monkeypatch, capsys):
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "login", "--help"])
    with pytest.raises(SystemExit) as stop:
        cli.main()
    assert stop.value.code == 0
    text = " ".join(capsys.readouterr().out.split())
    assert "--allow-http" in text
    assert "or the address was refused before any request" in text
    assert "An http: address is accepted only on this machine (localhost, 127.0.0.1 or [::1])" in text
    assert "--no-browser" in text
