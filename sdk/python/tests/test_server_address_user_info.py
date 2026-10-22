"""
#859: `aim-sdk login --url` with an address that holds a user name and
password before the host was accepted, the address was stored as `aimUrl`
with them, and later output repeated it. `requests` turns user info in a URL into an
`Authorization: Basic` header, which replaces the SDK's bearer token.

Acceptance:
- `aim-sdk login`, `aim-sdk init`, `aim-sdk demo`, `secure()` and
  `AIMClient(aim_url=...)` refuse an address with a user name or password, a
  query string or a fragment before any request, without repeating it.
- A stored `aimUrl` with a user name or password is not used for a request;
  the command says so in one line without printing it, names
  `aim-sdk login --force --url <server address>`, and leaves the file as it is.
- No output prints the user name or password.
"""
import contextlib
import io
import json
import os
from types import SimpleNamespace
from unittest import mock

import pytest

from aim_sdk import cli
from aim_sdk import credentials as credentials_module
from aim_sdk import oauth as oauth_module
from aim_sdk.exceptions import ConfigurationError

USER = "FAKEUSER"
PASSWORD = "FAKEPASS"
WITH_USER_INFO = "http://FAKEUSER:FAKEPASS@127.0.0.1:8080"
STORED_LINE = (
    "The server address stored with the sign-in has a user name or password, so it is "
    "not used for a request; sign in again with: "
    "aim-sdk login --force --url <server address>"
)

GIVEN = [
    (WITH_USER_INFO, "has a user name or password"),
    ("http://FAKEUSER@127.0.0.1:8080", "has a user name or password"),
    ("https://FAKEUSER:FAKEPASS@aim.example.com/api", "has a user name or password"),
    ("ftp://FAKEUSER:FAKEPASS@aim.example.com", "has a user name or password"),
    ("https://aim.example.com/?team=FAKEPASS", "has a query string"),
    ("https://aim.example.com/#FAKEPASS", "has a fragment"),
]


def _shows_no_user_info(text):
    return USER not in text and PASSWORD not in text


@pytest.fixture
def no_network(monkeypatch):
    calls = []

    def refuse(*args, **kwargs):
        calls.append(args)
        raise AssertionError("a request was sent to an address that should have been refused")

    import requests

    for name in ("get", "post", "put", "delete", "request"):
        monkeypatch.setattr(requests, name, refuse)
    monkeypatch.setattr(cli.requests, "get", refuse)
    monkeypatch.setattr(cli.requests, "post", refuse)
    monkeypatch.setattr(oauth_module.requests, "post", refuse)
    monkeypatch.setattr("webbrowser.open", refuse)
    return calls


@pytest.fixture
def home(monkeypatch, tmp_path):
    aim_dir = tmp_path / ".aim"
    (aim_dir / "agents").mkdir(parents=True)
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))
    monkeypatch.setattr(credentials_module, "AIM_DIR", aim_dir)
    monkeypatch.setattr(credentials_module, "SDK_CREDENTIALS_FILE", aim_dir / "sdk_credentials.json")
    monkeypatch.setattr(credentials_module, "AGENTS_DIR", aim_dir / "agents")
    monkeypatch.setattr(credentials_module, "LEGACY_CREDENTIALS_FILE", aim_dir / "credentials.json")
    monkeypatch.setattr(credentials_module, "_find_sdk_package_credentials", lambda: None)
    monkeypatch.setattr(oauth_module, "get_sdk_credentials_path", lambda: aim_dir / "sdk_credentials.json")
    return aim_dir


def _store(aim_dir, aim_url=WITH_USER_INFO):
    path = aim_dir / "sdk_credentials.json"
    path.write_text(json.dumps({
        "aimUrl": aim_url,
        "refreshToken": "not-a-real-token",
        "sdkTokenId": "not-a-real-id",
        "userEmail": "dev@example.com",
    }))
    os.chmod(path, 0o600)
    return path


def _run(fn, *args, **kwargs):
    out = io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(out):
        rc = fn(*args, **kwargs)
    return rc, out.getvalue()


# --- a given address ---------------------------------------------------------


@pytest.mark.parametrize("url, problem", GIVEN)
def test_login_refuses_the_address_before_any_request(no_network, monkeypatch, url, problem):
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *a, **k: None)
    rc, out = _run(cli.login, SimpleNamespace(url=url, force=True))
    assert rc == 1
    assert f"Error: --url {problem}." in out
    assert _shows_no_user_info(out)
    assert no_network == []


@pytest.mark.parametrize("url, problem", GIVEN)
def test_init_refuses_the_address_before_any_request(no_network, home, url, problem):
    token = cli.BOOTSTRAP_TOKEN_PREFIX + "not-a-real-token"
    with mock.patch.dict(os.environ, {cli.BOOTSTRAP_TOKEN_ENV: token}):
        rc, out = _run(cli.init, SimpleNamespace(token=None, url=url, name=cli.DEFAULT_INIT_AGENT_NAME))
    assert rc == 1
    assert f"Error: --url {problem}." in out
    assert _shows_no_user_info(out)
    assert no_network == []
    assert list((home / "agents").glob("*")) == []


@pytest.mark.parametrize("url, problem", GIVEN)
@pytest.mark.parametrize("signed_in", [False, True])
def test_demo_refuses_the_address_before_any_request(no_network, home, url, problem, signed_in):
    from aim_sdk import demo

    if signed_in:
        _store(home, "http://127.0.0.1:8080")
    rc, out = _run(demo.run, ci=True, url=url)
    assert rc == 1
    assert f"Error: --url {problem}." in out
    assert _shows_no_user_info(out)
    assert no_network == []


@pytest.mark.parametrize("url, problem", GIVEN)
def test_secure_refuses_the_address_before_any_request(no_network, home, url, problem):
    from aim_sdk.client import register_agent

    with pytest.raises(ConfigurationError) as raised:
        register_agent("issue-859-agent", aim_url=url)
    assert f"aim_url {problem}" in str(raised.value)
    assert _shows_no_user_info(str(raised.value))
    assert no_network == []


@pytest.mark.parametrize("url, problem", GIVEN)
def test_aim_client_refuses_the_address(url, problem):
    from aim_sdk.client import AIMClient

    with pytest.raises(ConfigurationError) as raised:
        AIMClient(agent_id="issue-859", api_key="not-a-real-key", aim_url=url)
    assert f"aim_url {problem}" in str(raised.value)
    assert _shows_no_user_info(str(raised.value))


@pytest.mark.parametrize("url", [
    "http:/FAKEUSER:FAKEPASS@",
    "http://FAKEUSER:FAKEPASS@",
])
def test_an_unusable_address_with_user_info_is_not_quoted(url):
    reason = cli.invalid_server_url_reason(url)
    assert reason is not None
    assert _shows_no_user_info(reason)


@pytest.mark.parametrize("url", ["http://localhost:8080", "https://aim.example.com/aim", " https://aim.example.com/ "])
def test_an_address_without_those_parts_is_accepted(url):
    assert cli.invalid_server_url_reason(url) is None
    assert credentials_module.server_address_part_refusal(url) is None


def test_a_transport_failure_message_leaves_the_user_info_out():
    import requests

    from aim_sdk.client import _transport_failure_message

    message = _transport_failure_message(requests.exceptions.ConnectionError("refused"), WITH_USER_INFO)
    assert "http://127.0.0.1:8080" in message
    assert _shows_no_user_info(message)


# --- a stored address --------------------------------------------------------


def test_status_names_the_stored_address_problem_without_printing_it(home):
    path = _store(home)
    before = path.read_bytes()
    rc, out = _run(cli.status, SimpleNamespace(json=False))
    assert f"   Server: {STORED_LINE}" in out.splitlines()
    assert "aim-sdk login --force --url <server address>" in out
    assert _shows_no_user_info(out)
    assert path.read_bytes() == before


def test_status_json_reports_no_server_for_the_stored_address(home):
    path = _store(home)
    before = path.read_bytes()
    rc, out = _run(cli.status, SimpleNamespace(json=True))
    assert json.loads(out)["server"] is None
    assert _shows_no_user_info(out)
    assert path.read_bytes() == before


def test_login_without_force_names_the_stored_address_problem_without_printing_it(no_network, home, monkeypatch):
    path = _store(home)
    before = path.read_bytes()
    monkeypatch.setattr("builtins.input", lambda prompt="": "n")
    rc, out = _run(cli.login, SimpleNamespace(url="http://127.0.0.1:8080", force=False))
    assert rc == 0
    assert STORED_LINE in out.splitlines()
    assert _shows_no_user_info(out)
    assert no_network == []
    assert path.read_bytes() == before


def test_token_refresh_sends_nothing_to_the_stored_address(no_network, home):
    path = _store(home)
    before = path.read_bytes()
    manager = oauth_module.OAuthTokenManager()
    token, out = _run(manager.get_access_token, suppress_errors=False)
    assert token is None
    assert STORED_LINE in out.splitlines()
    assert _shows_no_user_info(out)
    assert no_network == []
    assert path.read_bytes() == before


def test_secure_sends_nothing_to_the_stored_address(no_network, home):
    from aim_sdk.client import register_agent

    path = _store(home)
    before = path.read_bytes()
    with pytest.raises(ConfigurationError) as raised:
        register_agent("issue-859-agent")
    assert str(raised.value) == STORED_LINE
    assert no_network == []
    assert path.read_bytes() == before


@pytest.mark.parametrize("cleanup", [False, True])
def test_demo_sends_nothing_to_the_stored_address(no_network, home, cleanup):
    from aim_sdk import demo

    path = _store(home)
    before = path.read_bytes()
    rc, out = _run(demo.run, ci=True, cleanup=cleanup)
    assert rc == 1
    assert STORED_LINE in out.splitlines()
    assert _shows_no_user_info(out)
    assert no_network == []
    assert path.read_bytes() == before


def test_logout_sends_no_revocation_to_the_stored_address(no_network, home, monkeypatch, capsys):
    import sys

    path = _store(home)
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "logout"])
    rc = cli.main() or 0
    out = capsys.readouterr().out
    assert rc == 1
    assert ("Revocation not confirmed: the server address stored with the sign-in has a user name "
            "or password, so no request was sent.") in out
    assert _shows_no_user_info(out)
    assert no_network == []
    assert not path.exists()


def test_the_recovery_command_names_a_placeholder_for_an_address_with_user_info():
    command = credentials_module.login_recovery_command(WITH_USER_INFO)
    assert command == "aim-sdk login --url <server address> --force"
