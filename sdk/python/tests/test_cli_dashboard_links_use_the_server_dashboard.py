"""
`aim-sdk init` and `aim-sdk demo` print their dashboard links on the dashboard
address the server states. A self-hosted stack serves the API and the
dashboard on different origins: the API on the address --url names, the
dashboard on the server's FRONTEND_URL (http://localhost:3000 by default).

The server states that address twice, both from FRONTEND_URL: as dashboardUrl
in the bootstrap token exchange answer (bootstrap_token_handler.go), which
`aim-sdk init` reads, and in the device grant's verificationUri, built as
FRONTEND_URL + "/device" (device_auth_service.go). `aim-sdk login`, and the
sign-in that secure() and `aim-sdk demo` run (cli.sign_in_inline), store that
URI without its "/device" as the sign-in's dashboardUrl, and `aim-sdk demo`
prints its links on it when it runs against the server of that sign-in. Both
commands build the link with _dashboard.dashboard_origin, which falls back to
the address dashboard_url_for derives from the server address when the server
states none, or one no link is built on.

Before this change the sign-in stored no dashboard address and the demo
derived every link from the server address: for a sign-in stored for
http://127.0.0.1:18080 its banner printed http://127.0.0.1:18080/dashboard.
"""
import io
import sys
from unittest import mock

import pytest

from aim_sdk import cli
from aim_sdk import demo as demo_mod
from aim_sdk._dashboard import dashboard_url_for

from .test_cli_device_login import AIM, DEVICE, Stack, _login, _resp, _Terminal
from .test_cli_init_bootstrap_token import AGENT_ID, TOKEN, FakeAIM, registered, run_init


@pytest.fixture(autouse=True)
def answering_server(monkeypatch):
    """The demo asks the server once before it prints anything about it; these
    tests stand in for a server that answered."""
    monkeypatch.setattr(demo_mod, "_unreachable_reason", lambda aim_url: None)


@pytest.fixture
def fake_server():
    """Starts loopback FakeAIM servers (tests/test_cli_init_bootstrap_token.py) and closes them after the test."""
    servers = []

    def start(answer):
        server = FakeAIM(answer)
        servers.append(server)
        return server

    yield start
    for server in servers:
        server.close()


@pytest.fixture
def stack(monkeypatch):
    """The scripted device grant of tests/test_cli_device_login.py (Stack) as the only server, with no
    sleeps, no browser and no sign-in stored; .saved holds what the sign-in saved."""
    s = Stack()
    monkeypatch.setattr(cli.requests, "post", s.post)
    monkeypatch.setattr(cli.requests, "get", lambda *a, **k: _resp(200, {}))
    monkeypatch.setattr(cli.time, "sleep", lambda n: s.sleeps.append(n))
    monkeypatch.setattr(cli.webbrowser, "open", lambda url, *a, **k: s.opened.append(url) or True)
    monkeypatch.setattr("aim_sdk.credentials.save_sdk_credentials", lambda c: s.saved.update(c) or True)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *a, **k: None)
    return s


# --- aim-sdk init: the exchange answer's dashboardUrl -----------------------------

def stating(dashboard_url):
    """The exchange answer of registered(), with dashboardUrl added."""
    def answer(request):
        status, body, extra = registered(request)
        body["dashboardUrl"] = dashboard_url
        return status, body, extra
    return answer


def test_init_links_to_the_dashboard_address_the_exchange_answer_states(fake_server, capsys):
    server = fake_server(stating("http://localhost:3000"))

    assert run_init(server.url, token=TOKEN) == 0

    out = capsys.readouterr().out
    assert f"   Dashboard:   http://localhost:3000/dashboard/agents/{AGENT_ID}" in out.splitlines()
    assert f"{server.url}/dashboard" not in out


@pytest.mark.parametrize("stated", ["http://u:pw@localhost:3000", "javascript:alert(1)"],
                         ids=["user-and-password", "javascript"])
def test_init_links_to_the_server_address_when_the_stated_one_is_not_a_dashboard_address(
        fake_server, capsys, stated):
    server = fake_server(stating(stated))

    assert run_init(server.url, token=TOKEN) == 0

    out = capsys.readouterr().out
    assert f"   Dashboard:   {server.url}/dashboard/agents/{AGENT_ID}" in out.splitlines()
    assert "u:pw@" not in out and "javascript:" not in out


# --- the sign-in stores the dashboard address its device grant names ---------------

def test_login_stores_the_dashboard_address_the_verification_uri_names(stack):
    rc, out = _login(stack)

    assert rc == 0, out
    assert DEVICE["verificationUri"] == "http://localhost:3000/device"
    assert stack.saved.get("dashboardUrl") == "http://localhost:3000"


def test_login_keeps_the_path_the_dashboard_is_served_under(stack):
    stack.code = (200, dict(DEVICE, verificationUri="https://example.com/aim/device",
                            verificationUriComplete="https://example.com/aim/device?user_code=BCDF-GHJK"))

    rc, out = _login(stack)

    assert rc == 0, out
    assert stack.saved.get("dashboardUrl") == "https://example.com/aim"


@pytest.mark.parametrize("verification_uri", ["http://localhost:3000/activate",
                                              "http://localhost:3000/devices",
                                              "http://localhost:3000/device?step=1"])
def test_login_stores_no_dashboard_address_for_a_verification_uri_that_does_not_end_in_device(
        stack, verification_uri):
    stack.code = (200, dict(DEVICE, verificationUri=verification_uri, verificationUriComplete=None))

    rc, out = _login(stack)

    assert rc == 0, out
    assert stack.saved.get("dashboardUrl") is None


def test_sign_in_inline_stores_the_dashboard_address_and_the_keys_login_stores(stack):
    rc, out = _login(stack)
    assert rc == 0, out
    stored_by_login = dict(stack.saved)
    stack.saved.clear()

    stored = cli.sign_in_inline(AIM, agent_name=demo_mod.DEMO_AGENT_NAME, out=io.StringIO())

    assert stored.get("dashboardUrl") == "http://localhost:3000"
    assert sorted(stored) == sorted(stored_by_login)


# --- aim-sdk demo: the stored sign-in's dashboardUrl -------------------------------

SELF_HOSTED_API = "http://127.0.0.1:18080"
SIGN_IN = {"aimUrl": SELF_HOSTED_API, "dashboardUrl": "http://localhost:3000",
           "refreshToken": "rt", "accessToken": "at"}


def demo_agent():
    agent = mock.MagicMock()
    agent.agent_id = "agent-123"
    # perform_action returns the function unchanged so the scripted pass can call it.
    agent.perform_action.return_value = lambda fn: fn
    return agent


def set_up_demo(creds, url_override=None):
    with mock.patch("aim_sdk.credentials.load_sdk_credentials", return_value=creds), \
         mock.patch("aim_sdk.secure", return_value=demo_agent()):
        return demo_mod._setup(url_override, interactive=False, delay=0, ci=True)


def banner_dashboard_lines(out):
    return [line for line in out.splitlines() if line.startswith("  Dashboard:")]


def test_demo_links_to_the_dashboard_address_the_stored_sign_in_names(capsys):
    ctx = set_up_demo(SIGN_IN)

    assert ctx is not None
    assert banner_dashboard_lines(capsys.readouterr().out) == ["  Dashboard:   http://localhost:3000/dashboard"]
    assert ctx.dashboard_url == "http://localhost:3000"


def test_demo_with_url_naming_the_signed_in_server_links_to_that_dashboard(capsys):
    ctx = set_up_demo(SIGN_IN, url_override=SELF_HOSTED_API + "/")

    assert ctx is not None
    assert banner_dashboard_lines(capsys.readouterr().out) == ["  Dashboard:   http://localhost:3000/dashboard"]


def test_the_scripted_pass_links_the_agent_and_its_activity_on_that_dashboard(capsys):
    with mock.patch("aim_sdk.credentials.load_sdk_credentials", return_value=SIGN_IN), \
         mock.patch("aim_sdk.secure", return_value=demo_agent()):
        rc = demo_mod.run(ci=True)

    out = capsys.readouterr().out
    assert rc == 0, out
    assert "  Agent:    http://localhost:3000/dashboard/agents/agent-123" in out.splitlines()
    assert "  Activity: http://localhost:3000/dashboard/agents" in out.splitlines()
    assert f"{SELF_HOSTED_API}/dashboard" not in out


@pytest.mark.parametrize("creds, url_override, server", [
    (SIGN_IN, "http://127.0.0.1:9", "http://127.0.0.1:9"),
    ({"aimUrl": SELF_HOSTED_API, "refreshToken": "rt", "accessToken": "at"}, None, SELF_HOSTED_API),
    (dict(SIGN_IN, dashboardUrl="http://u:pw@localhost:3000"), None, SELF_HOSTED_API),
    (dict(SIGN_IN, dashboardUrl="javascript:alert(1)"), None, SELF_HOSTED_API),
    (dict(SIGN_IN, aimUrl="https://aim-api.example.com"), None, "https://aim-api.example.com"),
], ids=["url-names-another-server", "sign-in-stored-without-the-key", "user-and-password", "javascript",
        "loopback-address-for-a-server-on-another-host"])
def test_demo_links_to_the_address_derived_from_the_server_otherwise(capsys, creds, url_override, server):
    ctx = set_up_demo(creds, url_override)

    assert ctx is not None
    assert banner_dashboard_lines(capsys.readouterr().out) == [f"  Dashboard:   {dashboard_url_for(server)}/dashboard"]


def test_a_demo_that_signs_this_machine_in_links_to_the_dashboard_that_sign_in_stored(stack, monkeypatch, capsys):
    # A person at a terminal (conftest sets CI) and no sign-in stored (stack): the demo signs in first.
    monkeypatch.delenv("CI", raising=False)
    monkeypatch.setattr(sys, "stdin", _Terminal())
    stack.code = (200, dict(DEVICE, verificationUri="http://127.0.0.1:3001/device",
                            verificationUriComplete="http://127.0.0.1:3001/device?user_code=BCDF-GHJK"))

    with mock.patch("aim_sdk.secure", return_value=demo_agent()):
        ctx = demo_mod._setup(AIM, interactive=False, delay=0)

    assert ctx is not None
    assert stack.saved.get("dashboardUrl") == "http://127.0.0.1:3001"
    assert banner_dashboard_lines(capsys.readouterr().out) == ["  Dashboard:   http://127.0.0.1:3001/dashboard"]
