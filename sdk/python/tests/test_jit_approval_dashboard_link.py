"""
The JIT approval link points at the dashboard's JIT requests page.

``@perform_action(jit_access=True)`` and ``@require_approval`` print where an
administrator approves the pending verification. The link was built on
``aim_url`` as ``{aim_url}/dashboard/admin/verifications``: the server
registers no /dashboard route, and the dashboard has no
/dashboard/admin/verifications page. Its "JIT requests" tab,
/dashboard/admin/jit-requests, lists pending verifications and approves them.
The link is now built on the dashboard URL, the way ``aim-sdk demo`` builds
its links.
"""

import base64
from pathlib import Path

import pytest
import requests
from nacl.encoding import Base64Encoder
from nacl.signing import SigningKey

import aim_sdk.client as client_module
from aim_sdk._dashboard import JIT_APPROVALS_PAGE, dashboard_url_for
from aim_sdk.client import AIMClient
from aim_sdk.decision import get_mode_cache
from aim_sdk.strict_mode import reset_warning_state

REPO_ROOT = Path(__file__).resolve().parents[3]
AGENT_ID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"


@pytest.mark.parametrize("aim_url, dashboard", [
    ("http://localhost:8080", "http://localhost:3000"),
    ("http://localhost:8080/", "http://localhost:3000"),
    ("https://api.aim.opena2a.org", "https://aim.opena2a.org"),
    ("https://api.community.opena2a.org/", "https://community.opena2a.org"),
    ("https://aim.example.com/api", "https://aim.example.com"),
    ("https://aim.example.com", "https://aim.example.com"),
    # A host that starts with "api" keeps it.
    ("https://api.example.com", "https://api.example.com"),
])
def test_dashboard_url_for(aim_url, dashboard):
    assert dashboard_url_for(aim_url) == dashboard


def test_the_jit_approvals_page_is_a_dashboard_page():
    page = REPO_ROOT / "apps/web/app" / JIT_APPROVALS_PAGE.strip("/") / "page.tsx"
    assert page.is_file(), f"the dashboard serves no {JIT_APPROVALS_PAGE} page ({page})"
    tabs = (REPO_ROOT / "apps/web/lib/hub-tabs.ts").read_text(encoding="utf-8")
    assert f'href: "{JIT_APPROVALS_PAGE}"' in tabs, "the dashboard links the page from its tabs"


@pytest.fixture(autouse=True)
def _isolate(monkeypatch):
    get_mode_cache().clear()
    reset_warning_state()
    monkeypatch.delenv("AIM_STRICT_MODE", raising=False)
    monkeypatch.setattr(client_module, "RATE_LIMIT_RETRY_ATTEMPTS", 0)
    yield
    get_mode_cache().clear()


@pytest.fixture
def links(monkeypatch):
    """The dashboard_url every jit_waiting call receives."""
    received = []

    def jit_waiting(capability, risk_level, timeout, dashboard_url=None):
        received.append(dashboard_url)

    monkeypatch.setattr(client_module.console, "jit_waiting", jit_waiting)
    return received


def _client(aim_url):
    sk = SigningKey.generate()
    client = AIMClient(
        agent_id=AGENT_ID,
        public_key=sk.verify_key.encode(encoder=Base64Encoder).decode(),
        private_key=base64.b64encode(bytes(sk)).decode(),
        aim_url=aim_url,
        sdk_token_id="test-token",
        telemetry={"enabled": False},
    )

    def unreachable(method, url, **kwargs):
        raise requests.exceptions.ConnectionError("connection refused")

    client.session.request = unreachable
    return client


def _call(decorated):
    try:
        decorated()
    except BaseException:
        pass  # what an unreachable server does to the call is covered elsewhere


@pytest.mark.parametrize("aim_url, link", [
    ("http://localhost:8080", "http://localhost:3000/dashboard/admin/jit-requests"),
    ("https://api.aim.opena2a.org", "https://aim.opena2a.org/dashboard/admin/jit-requests"),
])
def test_perform_action_with_jit_access_links_the_dashboard(links, aim_url, link):
    client = _client(aim_url)

    @client.perform_action(capability="db:delete", jit_access=True, risk_level="high", auto_register=False)
    def drop_table():
        return "dropped"

    _call(drop_table)
    assert links == [link]


@pytest.mark.parametrize("aim_url, link", [
    ("http://localhost:8080", "http://localhost:3000/dashboard/admin/jit-requests"),
    ("https://api.aim.opena2a.org", "https://aim.opena2a.org/dashboard/admin/jit-requests"),
])
def test_require_approval_links_the_dashboard(links, aim_url, link):
    client = _client(aim_url)

    @client.require_approval(risk_level="high", action_name="transfer_money")
    def transfer_money():
        return "moved"

    _call(transfer_money)
    assert links == [link]
