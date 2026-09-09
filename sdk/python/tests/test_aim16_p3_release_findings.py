"""
AIM-16 -- the twelve P3 findings from the 2.0.2 fresh-user release test,
re-measured at head. One test (or parametrized group) per acceptance
criterion; every leaf test name carries its criterion id as the first token.

Everything here runs offline: HTTP goes only to loopback fakes started by the
tests themselves, or to a loopback port that was bound and closed so it
refuses connections. No test opens a socket to any address outside loopback
(test_AIM_16_AC13 asserts that over this file's own source).
"""

import io
import json
import logging
import re
import socket
import sys
import threading
import time
import warnings
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import pytest

import aim_sdk
import aim_sdk.client as client_module
import aim_sdk.security_logging as security_logging_module
from aim_sdk import AIMClient
from aim_sdk.console import console, set_quiet
from aim_sdk.exceptions import (
    AuthenticationError,
    ConfigurationError,
)

AgentEventType = security_logging_module.AgentEventType
security_logger = security_logging_module.security_logger
# getattr with the old spelling as fallback, so at the base commit each
# criterion fails its own test instead of erroring this module's collection;
# AC9 asserts the strict import inside its test body.
configure_security_logging = getattr(
    security_logging_module,
    "configure_security_logging",
    security_logging_module.configure_from_environment,
)

SDK_DIR = Path(__file__).resolve().parent.parent
README = (SDK_DIR / "README.md").read_text(encoding="utf-8")
CHANGELOG = (SDK_DIR / "CHANGELOG.md").read_text(encoding="utf-8")
SETUP_PY = (SDK_DIR / "setup.py").read_text(encoding="utf-8")

LOG_ENV_VARS = (
    "AIM_SECURITY_LOG_FILE",
    "AIM_SECURITY_LOG_LEVEL",
    "AIM_SECURITY_LOG_STDOUT",
    "AIM_SECURITY_LOGGING_ENABLED",
)


# ---------------------------------------------------------------------------
# Loopback fixtures
# ---------------------------------------------------------------------------

class _FakeAIM:
    """A loopback AIM fake: canned JSON answers, captured requests."""

    def __init__(self, routes):
        self.routes = routes  # {(METHOD, path): (status, body_dict)}
        self.requests = []    # [(method, path, headers_dict, body_bytes)]
        fake = self

        class Handler(BaseHTTPRequestHandler):
            def _serve(self):
                length = int(self.headers.get("Content-Length") or 0)
                body = self.rfile.read(length) if length else b""
                fake.requests.append(
                    (self.command, self.path, dict(self.headers.items()), body)
                )
                status, payload = fake.routes.get(
                    (self.command, self.path), (404, {"error": "not found"})
                )
                data = json.dumps(payload).encode("utf-8")
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            do_GET = do_POST = do_PUT = do_DELETE = _serve

            def log_message(self, *args):
                pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self.server.server_address[1]}"
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def header(self, index, name):
        headers = self.requests[index][2]
        for key, value in headers.items():
            if key.lower() == name.lower():
                return value
        return None

    def close(self):
        self.server.shutdown()
        self.server.server_close()


@pytest.fixture
def fake_aim_factory():
    servers = []

    def factory(routes):
        server = _FakeAIM(routes)
        servers.append(server)
        return server

    yield factory
    for server in servers:
        server.close()


@pytest.fixture
def closed_port_url():
    """A loopback URL nothing listens on: bound, learned, closed."""
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.bind(("127.0.0.1", 0))
    port = sock.getsockname()[1]
    sock.close()
    return f"http://127.0.0.1:{port}"


@pytest.fixture
def clean_logging_env(monkeypatch):
    """Clear the AIM logging env vars; restore the import-time config after."""
    for var in LOG_ENV_VARS:
        monkeypatch.delenv(var, raising=False)
    yield monkeypatch
    for var in LOG_ENV_VARS:
        monkeypatch.delenv(var, raising=False)
    configure_security_logging()


@pytest.fixture
def isolated_home(monkeypatch, tmp_path):
    """
    A fresh HOME so credential files never touch the real user profile.

    aim_sdk.credentials resolves its paths at import time, so patching the
    HOME environment variable alone is not enough: the module constants must
    be repointed too.
    """
    import aim_sdk.credentials as credentials_module

    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))
    aim_dir = tmp_path / ".aim"
    monkeypatch.setattr(credentials_module, "AIM_DIR", aim_dir)
    monkeypatch.setattr(
        credentials_module, "SDK_CREDENTIALS_FILE", aim_dir / "sdk_credentials.json"
    )
    monkeypatch.setattr(credentials_module, "AGENTS_DIR", aim_dir / "agents")
    monkeypatch.setattr(
        credentials_module, "LEGACY_CREDENTIALS_FILE", aim_dir / "credentials.json"
    )
    return tmp_path


def _keypair():
    """A consistent (public_b64, private_b64) pair from whichever nacl is loaded."""
    import base64
    from nacl.signing import SigningKey

    sk = SigningKey.generate()
    priv = base64.b64encode(bytes(sk)).decode()
    pub = base64.b64encode(bytes(sk.verify_key)).decode()
    return pub, priv


def _stdout_handlers():
    return [
        h for h in security_logger._logger.handlers
        if isinstance(h, logging.StreamHandler)
        and not isinstance(h, logging.FileHandler)
        and h.stream is sys.stdout
    ]


def _api_key_client(url, **kwargs):
    return AIMClient(
        agent_id="aim16-agent",
        api_key="aim_test_key",
        aim_url=url,
        telemetry={"enabled": False},
        **kwargs,
    )


# ---------------------------------------------------------------------------
# AC1 -- the two boolean env vars agree on their accepted spellings
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("spelling", ["true", "1", "yes", "on", "TRUE", " On "])
def test_AIM_16_AC1_true_spellings_install_the_stdout_handler(clean_logging_env, spelling):
    clean_logging_env.setenv("AIM_SECURITY_LOG_STDOUT", spelling)
    configure_security_logging()
    assert _stdout_handlers(), (
        f"AIM_SECURITY_LOG_STDOUT={spelling!r} must install the stdout handler, "
        f"the same spellings AIM_STRICT_MODE accepts"
    )


@pytest.mark.parametrize("spelling", ["false", "0", "no", "off", "OFF", " No "])
def test_AIM_16_AC1_false_spellings_install_no_stdout_handler(clean_logging_env, spelling):
    clean_logging_env.setenv("AIM_SECURITY_LOG_STDOUT", spelling)
    configure_security_logging()
    assert not _stdout_handlers()


def test_AIM_16_AC1_unset_installs_no_stdout_handler(clean_logging_env):
    configure_security_logging()
    assert not _stdout_handlers()


def test_AIM_16_AC1_spellings_match_strict_mode_sets():
    from aim_sdk.strict_mode import TRUE_VALUES, FALSE_VALUES

    assert TRUE_VALUES == frozenset({"true", "1", "yes", "on"})
    assert FALSE_VALUES == frozenset({"false", "0", "no", "off"})


def test_AIM_16_AC1_readme_passage_enumerates_the_true_spellings():
    passages = [
        line for line in README.splitlines() if "AIM_SECURITY_LOG_STDOUT" in line
    ]
    assert passages, "README must name AIM_SECURITY_LOG_STDOUT"
    passage = "\n".join(passages)
    for spelling in ("`true`", "`1`", "`yes`", "`on`"):
        assert spelling in passage, (
            f"the README passage naming AIM_SECURITY_LOG_STDOUT must enumerate "
            f"the accepted true spellings; missing {spelling}"
        )


# ---------------------------------------------------------------------------
# AC2 -- the README default-logging claim matches the installed handler
# ---------------------------------------------------------------------------

def test_AIM_16_AC2_error_records_reach_stderr_under_defaults(clean_logging_env, capsys):
    configure_security_logging()
    security_logger.log_agent_event(
        AgentEventType.AGENT_REGISTRATION_FAILED,
        agent_name="aim16",
        error="server said no",
    )
    captured = capsys.readouterr()
    assert "AGENT_REGISTRATION_FAILED" in captured.err, (
        "an ERROR-severity security record goes to stderr under the SDK's defaults"
    )
    assert captured.out == "", "nothing goes to stdout under the defaults"


def test_AIM_16_AC2_warning_records_are_not_written_under_defaults(clean_logging_env, capsys):
    configure_security_logging()
    security_logger.warning("routine warning, nobody's business")
    captured = capsys.readouterr()
    assert captured.err == ""
    assert captured.out == ""


def test_AIM_16_AC2_readme_states_the_default_stderr_behaviour():
    assert "under the SDK's own defaults nothing is written" not in README, (
        "the README claim contradicted the unconditional stderr handler"
    )
    assert re.search(r"ERROR-severity[^.]*stderr", README), (
        "the README default-behaviour passage must state that ERROR-severity "
        "records go to stderr"
    )
    assert re.search(r"WARNING-severity[^.]*not written", README), (
        "the README must state that WARNING-severity records are not written"
    )


# ---------------------------------------------------------------------------
# AC3 -- silenceable chatter; each unreachable verification says a thing once
# ---------------------------------------------------------------------------

def test_AIM_16_AC3_set_quiet_is_importable_and_documented():
    assert "set_quiet" in aim_sdk.__all__
    from aim_sdk import set_quiet as imported  # noqa: F401
    assert "set_quiet" in README, "the README must name the silencer"


def test_AIM_16_AC3_quiet_secure_api_key_mode_prints_nothing(
    fake_aim_factory, isolated_home, clean_logging_env, capsys
):
    pub, priv = _keypair()
    fake = fake_aim_factory({
        ("POST", "/api/v1/public/agents/register"): (201, {
            "id": "11111111-1111-1111-1111-111111111111",
            "agentId": "11111111-1111-1111-1111-111111111111",
            "publicKey": pub,
            "privateKey": priv,
            "aimUrl": "unused",
            "trustScore": 80,
        }),
    })
    set_quiet(True)
    try:
        agent = aim_sdk.secure(
            "aim16-quiet-agent", aim_url=fake.url, api_key="aim_test_key"
        )
    finally:
        set_quiet(False)
    assert isinstance(agent, AIMClient)
    captured = capsys.readouterr()
    assert captured.out == "", (
        f"quiet mode must silence secure()'s stdout chatter, got: {captured.out!r}"
    )


def test_AIM_16_AC3_quiet_auto_detection_pass_prints_nothing(capsys):
    set_quiet(True)
    try:
        console.info("API Key Mode: Using API key authentication")
        console.detection_result("Agent Type", "langchain", "langchain imported")
        console.detection_none("Agent Type", fallback="ai_agent")
        console.detection_none("Capabilities")
        console.warning("noise")
        console.success("done")
    finally:
        set_quiet(False)
    assert capsys.readouterr().out == ""


def test_AIM_16_AC3_unreachable_verification_emits_pending_change_on_one_stream(
    closed_port_url, clean_logging_env, capsys
):
    client = _api_key_client(closed_port_url, timeout=2)

    @client.perform_action(capability="util:noop")
    def action():
        return "ran"

    with warnings.catch_warnings(record=True) as caught:
        warnings.simplefilter("always")
        result = action()

    assert result == "ran"
    pending = [
        w for w in caught
        if issubclass(w.category, aim_sdk.PendingEnforcementChange)
    ]
    assert len(pending) == 1, (
        f"exactly one PendingEnforcementChange warning, got {len(pending)}"
    )
    sentence = str(pending[0].message)
    captured = capsys.readouterr()
    assert sentence not in captured.out, (
        "the enforcement-mode sentence must not be duplicated onto stdout; it "
        "is emitted once, through the typed warning"
    )


# ---------------------------------------------------------------------------
# AC4 -- typed actionable registration error; one class per HTTP condition
# ---------------------------------------------------------------------------

def test_AIM_16_AC4_unreachable_registration_raises_actionable_untangled_error(
    closed_port_url, isolated_home, clean_logging_env, capsys
):
    with pytest.raises(ConfigurationError) as excinfo:
        aim_sdk.secure(
            "aim16-noserver-agent", aim_url=closed_port_url, api_key="aim_test_key"
        )
    message = str(excinfo.value)
    assert closed_port_url in message, "the message names the URL to correct"
    assert "docker compose up" in message or "aim-sdk login" in message, (
        "the message names a command that starts or points at a server"
    )
    assert "Max retries exceeded" not in message
    assert excinfo.value.__suppress_context__ is True, (
        "raise ... from None: the requests/urllib3 chain must not be re-printed "
        "as implicit context"
    )
    assert excinfo.value.__cause__ is None


def test_AIM_16_AC4_registration_401_and_verification_401_raise_the_same_class(
    fake_aim_factory, isolated_home, clean_logging_env, capsys
):
    fake = fake_aim_factory({
        ("POST", "/api/v1/public/agents/register"): (401, {"error": "bad api key"}),
        ("POST", "/api/v1/sdk-api/some-route"): (401, {"error": "bad creds"}),
    })

    with pytest.raises(AuthenticationError) as reg_excinfo:
        aim_sdk.secure(
            "aim16-badkey-agent", aim_url=fake.url, api_key="aim_wrong_key"
        )

    client = _api_key_client(fake.url, auto_retry=False)
    with pytest.raises(AuthenticationError) as verify_excinfo:
        client._make_request("POST", "/api/v1/sdk-api/some-route", {"x": 1})

    assert type(reg_excinfo.value) is type(verify_excinfo.value), (
        "a 401 answered to registration and a 401 answered to verification "
        "must raise the same exception class"
    )


# ---------------------------------------------------------------------------
# AC5 -- no absent identifier in a URL; one case convention per body
# ---------------------------------------------------------------------------

def _recording_session(client):
    calls = []

    def record(method=None, url=None, **kwargs):
        calls.append({"method": method, "url": url, **kwargs})
        raise AssertionError("no response wired; test only records the attempt")

    client.session.request = record
    return calls


@pytest.mark.parametrize("absent", [None, ""])
def test_AIM_16_AC5_absent_verification_id_issues_no_request(closed_port_url, absent):
    client = _api_key_client(closed_port_url)
    calls = _recording_session(client)
    client.log_capability_result(absent, True, result_summary="fine")
    assert calls == [], (
        "log_capability_result must return without any HTTP request when "
        "verification_id is absent -- never POST to /verifications/None/result"
    )


def test_AIM_16_AC5_result_body_uses_the_camel_case_convention(closed_port_url):
    client = _api_key_client(closed_port_url)
    calls = []

    class _Ok:
        status_code = 200

        def raise_for_status(self):
            pass

        def json(self):
            return {}

    def record(method=None, url=None, **kwargs):
        calls.append({"method": method, "url": url, **kwargs})
        return _Ok()

    client.session.request = record
    client.log_capability_result(
        "vid-123", False, result_summary="s", error_message="boom"
    )
    assert len(calls) == 1
    assert "/verifications/vid-123/result" in calls[0]["url"]
    assert "/None/" not in calls[0]["url"]
    body = calls[0]["json"]
    assert "resultSummary" in body and "errorMessage" in body, (
        "the result body uses the same camelCase convention as strictMode/"
        "executedAt and displayName/agentType"
    )
    assert "result_summary" not in body and "error_message" not in body


# ---------------------------------------------------------------------------
# AC6 -- an SDK User-Agent on every request to an AIM URL
# ---------------------------------------------------------------------------

EXPECTED_UA = f"AIM-Python-SDK/{aim_sdk.__version__}"


def test_AIM_16_AC6_api_key_registration_carries_the_sdk_user_agent(
    fake_aim_factory, isolated_home, clean_logging_env, capsys
):
    pub, priv = _keypair()
    fake = fake_aim_factory({
        ("POST", "/api/v1/public/agents/register"): (201, {
            "id": "22222222-2222-2222-2222-222222222222",
            "publicKey": pub,
            "privateKey": priv,
            "trustScore": 80,
        }),
    })
    aim_sdk.secure("aim16-ua-agent", aim_url=fake.url, api_key="aim_test_key")
    assert fake.requests, "the registration request must have been issued"
    assert fake.header(0, "User-Agent") == EXPECTED_UA


def test_AIM_16_AC6_oauth_check_and_registration_carry_the_sdk_user_agent(
    fake_aim_factory, isolated_home, clean_logging_env, monkeypatch, capsys
):
    name = "aim16-oauth-agent"
    fake = fake_aim_factory({
        ("GET", f"/api/v1/sdk-api/agents/{name}"): (404, {"error": "no such agent"}),
        ("POST", "/api/v1/agents"): (201, {
            "id": "33333333-3333-3333-3333-333333333333",
            "trustScore": 80,
        }),
    })

    class _StubTokenManager:
        credentials = {}

        def __init__(self, *args, **kwargs):
            pass

        def get_access_token(self, **kwargs):
            return "stub-access-token"

    monkeypatch.setattr(client_module, "OAuthTokenManager", _StubTokenManager)
    client_module._register_via_oauth(
        name=name,
        aim_url=fake.url,
        sdk_creds={},
        registration_data={"name": name, "displayName": name, "agentType": "ai_agent"},
        sdk_token_id=None,
        mcp_server_names=[],
        mcp_full_definitions=[],
    )
    assert len(fake.requests) >= 2, "agent-exists check plus registration"
    for index, request in enumerate(fake.requests):
        assert fake.header(index, "User-Agent") == EXPECTED_UA, (
            f"request {request[0]} {request[1]} reached the server without the "
            f"SDK User-Agent"
        )


def test_AIM_16_AC6_mcp_registration_carries_the_sdk_user_agent(
    fake_aim_factory, clean_logging_env, capsys
):
    agent_id = "44444444-4444-4444-4444-444444444444"
    fake = fake_aim_factory({
        ("POST", f"/api/v1/sdk-api/agents/{agent_id}/mcp-servers"): (
            201, {"id": "mcp-1"},
        ),
    })
    client = _api_key_client(fake.url)
    result = client_module._register_single_mcp(
        client,
        fake.url,
        {},
        {"name": "aim16-mcp", "description": "d"},
        agent_id,
        {},
    )
    assert result["success"] is True
    assert fake.requests
    assert fake.header(0, "User-Agent") == EXPECTED_UA


def test_AIM_16_AC6_credential_validation_probe_carries_the_sdk_user_agent(
    fake_aim_factory, isolated_home, clean_logging_env, capsys
):
    agent_id = "55555555-5555-5555-5555-555555555555"
    fake = fake_aim_factory({
        ("GET", f"/api/v1/agents/{agent_id}"): (200, {"id": agent_id}),
    })
    ok, error = client_module._validate_cached_credentials(
        agent_id, fake.url, "aim16-agent", api_key="aim_test_key"
    )
    assert ok is True and error is None
    assert fake.header(0, "User-Agent") == EXPECTED_UA


# ---------------------------------------------------------------------------
# AC7 -- a PyPI-resolvable README; classifiers cover python_requires
# ---------------------------------------------------------------------------

PACKAGED_ROOT_FILES = {"README.md", "CHANGELOG.md", "LICENSE", "VERSION"}


def test_AIM_16_AC7_no_readme_link_targets_a_path_outside_the_packaged_tree():
    offenders = []
    for target in re.findall(r"\]\(([^)\s]+)\)", README):
        if target.startswith(("http://", "https://", "#", "mailto:")):
            continue
        path = target.split("#")[0]
        if not path:
            continue
        if path.startswith("../") or "/../" in path:
            offenders.append(target)
        elif not (path in PACKAGED_ROOT_FILES or path.startswith("aim_sdk/")):
            offenders.append(target)
    assert offenders == [], (
        f"README links must resolve for a PyPI reader; these target paths "
        f"outside the packaged tree: {offenders}"
    )


def test_AIM_16_AC7_version_pointer_resolves_for_a_wheel_only_install():
    assert "see `VERSION` file" not in README, (
        "no VERSION file exists after a wheel install"
    )
    assert "aim_sdk.__version__" in README


def test_AIM_16_AC7_classifiers_cover_every_minor_python_requires_admits():
    classifier_minors = {
        int(m) for m in re.findall(
            r"Programming Language :: Python :: 3\.(\d+)", SETUP_PY
        )
    }
    match = re.search(r"python_requires\s*=\s*[\"']([^\"']+)[\"']", SETUP_PY)
    assert match, "setup.py must declare python_requires"
    spec = match.group(1).replace(" ", "")
    lower = re.search(r">=3\.(\d+)", spec)
    upper = re.search(r"<3\.(\d+)", spec)
    assert lower, f"python_requires has no lower bound: {spec}"
    assert upper, (
        f"python_requires must carry an upper bound matching the classifier "
        f"list (or the classifiers cannot cover it): {spec}"
    )
    admitted = set(range(int(lower.group(1)), int(upper.group(1))))
    missing = admitted - classifier_minors
    assert missing == set(), (
        f"python_requires {spec} admits 3.{sorted(missing)} with no matching "
        f"'Programming Language :: Python :: 3.x' classifier"
    )


# ---------------------------------------------------------------------------
# AC8 -- the login banner is rectangular
# ---------------------------------------------------------------------------

def test_AIM_16_AC8_banner_lines_all_share_one_display_width(capsys):
    from aim_sdk.cli import print_banner

    print_banner()
    lines = [line for line in capsys.readouterr().out.splitlines() if line.strip()]
    assert lines, "the banner prints something"
    widths = {len(line) for line in lines}
    assert len(widths) == 1, (
        f"every banner line must be the same width so the right border is a "
        f"single column; got widths {sorted(widths)} over {lines!r}"
    )


# ---------------------------------------------------------------------------
# AC9 -- one importable spelling of configure_security_logging
# ---------------------------------------------------------------------------

def test_AIM_16_AC9_the_public_spelling_is_importable_from_both_paths():
    from aim_sdk import configure_security_logging as from_package
    from aim_sdk.security_logging import configure_security_logging as from_module

    assert from_package is from_module


def test_AIM_16_AC9_module_docstring_usage_block_shows_the_public_spelling():
    import aim_sdk.security_logging as security_logging_module

    docstring = security_logging_module.__doc__ or ""
    assert "Usage:" in docstring
    usage_block = docstring.split("Usage:", 1)[1]
    assert "configure_security_logging" in usage_block


# ---------------------------------------------------------------------------
# AC10 -- a bounded, stated unreachable-AIM cost
# ---------------------------------------------------------------------------

def test_AIM_16_AC10_decorated_call_returns_fast_when_aim_refuses_connections(
    closed_port_url, clean_logging_env, capsys
):
    client = _api_key_client(closed_port_url, timeout=5)

    @client.perform_action(capability="util:noop")
    def action():
        return 42

    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        start = time.monotonic()
        result = action()
        elapsed = time.monotonic() - start

    assert result == 42
    assert elapsed < 3.0, (
        f"a decorated call against a closed loopback port took {elapsed:.2f}s; "
        f"the README's stated budget is well under a second (was ~7s of "
        f"exponential backoff at base)"
    )


def test_AIM_16_AC10_readme_states_the_unreachable_cost_and_the_lever():
    assert "Cost of an unreachable AIM" in README
    section = README.split("Cost of an unreachable AIM", 1)[1].split("##", 1)[0]
    assert "single connection attempt" in section
    for lever in ("timeout", "max_retries", "auto_retry"):
        assert lever in section, f"the README must say how to change the cost ({lever})"


# ---------------------------------------------------------------------------
# AC11 -- a malformed 200 is named as malformed; paths agree on success
# ---------------------------------------------------------------------------

def test_AIM_16_AC11_a_200_with_an_empty_body_is_reported_as_malformed(
    fake_aim_factory, isolated_home, clean_logging_env, capsys
):
    fake = fake_aim_factory({
        ("POST", "/api/v1/public/agents/register"): (200, {}),
    })
    with pytest.raises(ConfigurationError) as excinfo:
        aim_sdk.secure(
            "aim16-empty200-agent", aim_url=fake.url, api_key="aim_test_key"
        )
    message = str(excinfo.value)
    assert "Unknown error" not in message
    assert "malformed" in message or "unexpected" in message
    assert "200" in message and "201" in message, (
        "the error names the statuses the SDK requires"
    )


def test_AIM_16_AC11_both_registration_paths_accept_200_and_201(
    fake_aim_factory, isolated_home, clean_logging_env, capsys
):
    # The OAuth path accepted 200 at base; the API-key path required exactly
    # 201. A well-formed 200 must now register on the API-key path too.
    pub, priv = _keypair()
    fake = fake_aim_factory({
        ("POST", "/api/v1/public/agents/register"): (200, {
            "id": "66666666-6666-6666-6666-666666666666",
            "publicKey": pub,
            "privateKey": priv,
            "trustScore": 80,
        }),
    })
    agent = aim_sdk.secure(
        "aim16-status200-agent", aim_url=fake.url, api_key="aim_test_key"
    )
    assert isinstance(agent, AIMClient)
    assert client_module.REGISTRATION_SUCCESS_STATUSES == (200, 201)


# ---------------------------------------------------------------------------
# AC12 -- PolicyCache's public status matches its documented reachability
# ---------------------------------------------------------------------------

def test_AIM_16_AC12_policycache_is_absent_from_the_public_namespace():
    assert "PolicyCache" not in aim_sdk.__all__, (
        "no released AIM server serves the route PolicyCache fetches; it must "
        "not be offered as public API"
    )
    assert not hasattr(aim_sdk, "PolicyCache")


def test_AIM_16_AC12_policycache_stays_importable_from_its_module():
    from aim_sdk.auto_hooks import PolicyCache  # noqa: F401


# ---------------------------------------------------------------------------
# AC13 -- the tree's own record of the batch is coherent
# ---------------------------------------------------------------------------

def test_AIM_16_AC13_changelog_carries_a_release_newer_than_2_0_2():
    headings = re.findall(r"^## \[(\d+)\.(\d+)\.(\d+)\]", CHANGELOG, re.MULTILINE)
    assert headings, "the changelog has release headings"
    newest = tuple(int(part) for part in headings[0])
    assert newest > (2, 0, 2), (
        f"the newest changelog section must be for a release after 2.0.2, "
        f"got {newest}"
    )


def test_AIM_16_AC13_the_new_section_names_all_twelve_dispositions():
    newest_section = re.sub(r"\s+", " ", CHANGELOG.split("## [", 2)[1])
    for item in range(1, 13):
        assert re.search(rf"\(item {item}[,)]", newest_section), (
            f"the release section must name the disposition of item {item} "
            f"of the twelve"
        )


def test_AIM_16_AC13_these_tests_touch_only_loopback_addresses():
    source = Path(__file__).read_text(encoding="utf-8")
    for host in re.findall(r"https?://([^/\s\"')]+)", source):
        hostname = host.split(":")[0]
        assert hostname in ("127.0.0.1", "localhost"), (
            f"AIM-16 tests must not reference any non-loopback address: {host}"
        )
