"""
AIM-14 -- aim-sdk (Python): the five P2 findings from the 2.0.2 release test.

AC1  `aim-sdk login` fails fast and bounded on an unreachable server: a dead
     loopback port exits non-zero within a stated bound, names the URL, and
     never opens a browser; after a successful browser open the callback wait
     is bounded by a deadline and times out non-zero with a timeout message.
AC2  The README manual-mode one-liner and the code agree in the same tree:
     the manual-mode `secure(api_key=...)` example carries `aim_url=` and the
     prose states the requirement the code actually enforces.
AC3  The no-credentials ConfigurationError names the real fixes for a pip
     user: the `aim-sdk login` command, and that api_key mode requires aim_url.
AC4  The SOC-facing security log distinguishes "AIM was never asked" from
     "AIM said no": an unreachable server writes a CAPABILITY_CHECK whose
     result is not the policy-denial literal DENIED, and when the enforcement
     rule executes the function unverified a security event records that the
     action executed.
AC5  A 401 on verification writes a check result distinguishable from the
     DENIED literal that a server status "denied" produces.
AC6  A rejected capability registration (404, 401; 500 stays warning-only)
     prints no console line claiming the capability was registered.
AC7  The tree's own record is coherent: CHANGELOG documents the batch under a
     section newer than 2.0.2, the README passages match the delivered code,
     and no test in this file performs network I/O beyond loopback.

Every server below is a scripted loopback listener on 127.0.0.1; the "dead
URL" is a loopback port held bound but never listening, so connects are
refused. No test talks to anything beyond 127.0.0.1.
"""

import json
import logging
import re
import socket
import threading
import time
import types
import uuid
import warnings
from pathlib import Path

import pytest

from aim_sdk import cli
from aim_sdk import client as client_module
from aim_sdk import security_logging
from aim_sdk.client import AIMClient, register_agent
from aim_sdk.decision import get_mode_cache
from aim_sdk.exceptions import (
    ActionDeniedError,
    AuthenticationError,
    ConfigurationError,
)

SDK_ROOT = Path(__file__).resolve().parent.parent
README = SDK_ROOT / "README.md"
CHANGELOG = SDK_ROOT / "CHANGELOG.md"


# --------------------------------------------------------------------------- #
# Loopback fixtures: a scripted HTTP listener, a refused port, an event spy.
# --------------------------------------------------------------------------- #
class LocalServer:
    """Answers each request from handler(method, path) -> (status, headers, body)."""

    def __init__(self, handler):
        self._sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self._sock.bind(("127.0.0.1", 0))
        self._sock.listen(16)
        self.url = f"http://127.0.0.1:{self._sock.getsockname()[1]}"
        self.request_lines = []
        self._handler = handler
        self._closing = threading.Event()
        self._open = []
        threading.Thread(target=self._accept, daemon=True).start()

    def _accept(self):
        while not self._closing.is_set():
            try:
                conn, _ = self._sock.accept()
            except OSError:
                return
            self._open.append(conn)
            threading.Thread(target=self._serve, args=(conn,), daemon=True).start()

    def _serve(self, conn):
        try:
            conn.settimeout(30)
            data = b""
            while b"\r\n\r\n" not in data:
                chunk = conn.recv(65536)
                if not chunk:
                    return
                data += chunk
            head, _, body = data.partition(b"\r\n\r\n")
            lines = head.decode("latin-1").split("\r\n")
            self.request_lines.append(lines[0])
            method, path = lines[0].split(" ")[0:2]
            headers = {}
            for line in lines[1:]:
                if ":" in line:
                    key, value = line.split(":", 1)
                    headers[key.strip().lower()] = value.strip()
            need = int(headers.get("content-length", "0"))
            while len(body) < need:
                chunk = conn.recv(65536)
                if not chunk:
                    break
                body += chunk

            status, extra_headers, payload = self._handler(method, path)
            encoded = payload.encode("utf-8")
            response_head = (
                f"HTTP/1.1 {status} X\r\n"
                f"Content-Type: application/json\r\n"
                f"Content-Length: {len(encoded)}\r\n"
                f"Connection: close\r\n"
            )
            for key, value in extra_headers.items():
                response_head += f"{key}: {value}\r\n"
            conn.sendall(response_head.encode("latin-1") + b"\r\n" + encoded)
            conn.close()
        except OSError:
            pass

    def close(self):
        self._closing.set()
        try:
            self._sock.close()
        except OSError:
            pass
        for conn in self._open:
            try:
                conn.close()
            except OSError:
                pass


@pytest.fixture
def server_factory():
    servers = []

    def start(handler):
        server = LocalServer(handler)
        servers.append(server)
        return server

    yield start
    for server in servers:
        server.close()


@pytest.fixture
def closed_port():
    """A loopback port that refuses connections: bound, never listening,
    and held for the duration of the test so nothing else can claim it."""
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.bind(("127.0.0.1", 0))
    yield sock.getsockname()[1]
    sock.close()


class _EventCollector(logging.Handler):
    def __init__(self):
        super().__init__(level=logging.DEBUG)
        self.events = []

    def emit(self, record):
        event = getattr(record, "security_event", None)
        if event is not None:
            self.events.append(event)


@pytest.fixture
def security_events():
    logger = security_logging.security_logger._logger
    collector = _EventCollector()
    previous_level = logger.level
    logger.addHandler(collector)
    logger.setLevel(logging.DEBUG)
    yield collector
    logger.removeHandler(collector)
    logger.setLevel(previous_level)


@pytest.fixture(autouse=True)
def clean_mode_cache():
    get_mode_cache().clear()
    yield
    get_mode_cache().clear()


def _api_key_client(url):
    return AIMClient(
        agent_id=str(uuid.uuid4()),
        api_key="test-api-key",
        aim_url=url,
        auto_retry=False,
    )


def _no_stored_credentials(monkeypatch):
    monkeypatch.setattr(client_module, "_load_credentials", lambda name: None)
    monkeypatch.setattr(client_module, "load_sdk_credentials", lambda *a, **k: None)


# --------------------------------------------------------------------------- #
# AC1 -- login bounded and failing fast.
# --------------------------------------------------------------------------- #
def _login_args(url):
    return types.SimpleNamespace(url=url, force=True)


def _run_login_bounded(args, seconds):
    """Run cli.login in a thread with a hard observation bound, so an
    unbounded wait shows up as a failed assertion instead of a hung run."""
    result = {}

    def target():
        result["rc"] = cli.login(args)

    thread = threading.Thread(target=target, daemon=True)
    thread.start()
    thread.join(seconds)
    return (not thread.is_alive()), result.get("rc")


def test_AIM_14_AC1_dead_url_exits_nonzero_fast_names_url_and_opens_no_browser(
    monkeypatch, capsys, closed_port
):
    url = f"http://127.0.0.1:{closed_port}"
    opened = []
    monkeypatch.setattr(cli.webbrowser, "open", lambda *a, **k: opened.append(a) or True)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *a, **k: None)

    start = time.monotonic()
    finished, rc = _run_login_bounded(_login_args(url), 15)
    elapsed = time.monotonic() - start
    out = capsys.readouterr().out

    assert finished, "login must exit within a stated bound on an unreachable server"
    assert rc != 0, "login must exit non-zero when the server is unreachable"
    assert elapsed < 15, f"login took {elapsed:.1f}s against a refused loopback port"
    assert url in out, "the failure output must name the unreachable URL"
    assert opened == [], "webbrowser.open must not be called when the pre-flight probe fails"


def test_AIM_14_AC1_callback_wait_is_bounded_and_times_out_nonzero(
    monkeypatch, capsys, server_factory
):
    server = server_factory(lambda method, path: (200, {}, "{}"))
    opened = []
    monkeypatch.setattr(cli.webbrowser, "open", lambda *a, **k: opened.append(a) or True)
    monkeypatch.setattr("aim_sdk.credentials.load_sdk_credentials", lambda *a, **k: None)
    # Shrink the deadline so the green run is quick; at base the attribute does
    # not exist and the wait re-enters handle_request forever.
    monkeypatch.setattr(cli, "LOGIN_CALLBACK_TIMEOUT_SECONDS", 2, raising=False)

    finished, rc = _run_login_bounded(_login_args(server.url), 15)
    out = capsys.readouterr().out

    assert finished, "the callback wait must be bounded by a deadline"
    assert opened, "control: with a reachable server the browser open is reached"
    assert rc != 0, "an expired callback wait must exit non-zero"
    assert "timed out" in out.lower(), f"expected a timeout message, got: {out!r}"


# --------------------------------------------------------------------------- #
# AC2 -- README manual mode and the code agree in the same tree.
# --------------------------------------------------------------------------- #
def _manual_mode_section():
    text = README.read_text(encoding="utf-8")
    match = re.search(r"^## Manual mode.*?(?=^## )", text, re.M | re.S)
    assert match, "README must keep its manual-mode section"
    return match.group(0)


def _prose_of(section):
    """The section text outside fenced code blocks."""
    parts = section.split("```")
    return "\n".join(parts[0::2])


def test_AIM_14_AC2_readme_manual_mode_example_carries_aim_url_and_states_it():
    section = _manual_mode_section()
    example_lines = [
        line for line in section.splitlines()
        if "secure(" in line and "api_key=" in line
    ]
    assert example_lines, "the manual-mode section must still show secure(api_key=...)"
    for line in example_lines:
        assert "aim_url=" in line, (
            f"the manual-mode secure() example must carry aim_url= beside "
            f"api_key= (the call raises without it): {line.strip()!r}"
        )
    assert "aim_url" in _prose_of(section), (
        "the manual-mode prose must state that aim_url is required with api_key"
    )


def test_AIM_14_AC2_secure_with_api_key_and_no_aim_url_raises_the_documented_requirement(
    monkeypatch,
):
    _no_stored_credentials(monkeypatch)
    with pytest.raises(ConfigurationError) as excinfo:
        register_agent("aim14-ac2-agent", api_key="aim_test_key")
    assert "aim_url" in str(excinfo.value)


# --------------------------------------------------------------------------- #
# AC3 -- the no-credentials error names the real fixes.
# --------------------------------------------------------------------------- #
def test_AIM_14_AC3_no_credentials_error_names_login_command_and_aim_url_requirement(
    monkeypatch,
):
    _no_stored_credentials(monkeypatch)
    with pytest.raises(ConfigurationError) as excinfo:
        register_agent("aim14-ac3-agent")
    message = str(excinfo.value)
    assert "aim-sdk login" in message, (
        f"the no-credentials error must name the aim-sdk login command "
        f"(the README's own step for a pip install): {message!r}"
    )
    assert "api_key" in message and "aim_url" in message, (
        f"the error must state that api_key mode also requires aim_url: {message!r}"
    )


# --------------------------------------------------------------------------- #
# AC4 -- unreached is not DENIED, and an unverified run is recorded.
# --------------------------------------------------------------------------- #
def test_AIM_14_AC4_unreachable_check_is_not_denied_and_the_run_is_recorded(
    closed_port, security_events
):
    client = _api_key_client(f"http://127.0.0.1:{closed_port}")
    ran = []

    @client.perform_action(capability="db:read", auto_register=False)
    def guarded():
        ran.append(True)
        return "done"

    with warnings.catch_warnings():
        # The transport-plus-unknown-mode cell emits PendingEnforcementChange.
        warnings.simplefilter("ignore")
        assert guarded() == "done"
    assert ran == [True], "control: the unverified action ran (monitoring-window cell)"

    checks = [e for e in security_events.events if e.event_type == "CAPABILITY_CHECK"]
    assert checks, "a CAPABILITY_CHECK authorization event must be written"
    for event in checks:
        assert event.result != "DENIED", (
            "an unreachable AIM (connection refused) must not be recorded with "
            "the policy-denial literal DENIED"
        )
    assert any(e.result == "UNAVAILABLE" for e in checks), (
        f"the unreached check must carry an unreached/unverified result value, "
        f"got: {[e.result for e in checks]}"
    )

    executed = [e for e in security_events.events if e.event_type == "ACTION_EXECUTED"]
    assert executed, (
        "when the enforcement rule executes the function unverified, a security "
        "event must record that the action executed"
    )
    assert executed[0].action == "db:read"


def test_AIM_14_AC4_unverified_require_approval_run_is_recorded(
    closed_port, security_events, capsys
):
    client = _api_key_client(f"http://127.0.0.1:{closed_port}")

    @client.require_approval(risk_level="high")
    def sensitive():
        return "ran"

    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        assert sensitive() == "ran"

    executed = [e for e in security_events.events if e.event_type == "ACTION_EXECUTED"]
    assert executed, (
        "the console.jit_unverified path must write a security event recording "
        "that the action executed"
    )
    assert executed[0].action == "sensitive"


# --------------------------------------------------------------------------- #
# AC5 -- a 401 on verification is not the policy-denial literal.
# --------------------------------------------------------------------------- #
def test_AIM_14_AC5_authentication_failure_result_is_distinguishable_from_denied(
    server_factory, security_events
):
    server = server_factory(lambda method, path: (401, {}, '{"error":"bad key"}'))
    client = _api_key_client(server.url)

    with pytest.raises(AuthenticationError):
        client.verify_capability("db:read")

    checks = [e for e in security_events.events if e.event_type == "CAPABILITY_CHECK"]
    assert checks, "the 401 must still write a CAPABILITY_CHECK event"
    for event in checks:
        assert event.result != "DENIED", (
            "a 401 (AIM never verified anything) must be distinguishable in the "
            "result field from an administrator's deny"
        )


def test_AIM_14_AC5_a_real_policy_denial_still_renders_denied(
    server_factory, security_events
):
    body = json.dumps(
        {"id": "ver-1", "status": "denied", "denialReason": "nope",
         "enforcementMode": "strict"}
    )
    server = server_factory(lambda method, path: (200, {}, body))
    client = _api_key_client(server.url)

    with pytest.raises(ActionDeniedError):
        client.verify_capability("db:read")

    denials = [e for e in security_events.events if e.event_type == "CAPABILITY_DENIED"]
    assert denials, "control: a server denial still writes CAPABILITY_DENIED"
    assert all(e.result == "DENIED" for e in denials), (
        "control: the policy-denial literal stays DENIED for a real denial"
    )


# --------------------------------------------------------------------------- #
# AC6 -- no success line on a rejected capability registration.
# --------------------------------------------------------------------------- #
def _registration_scenario(register_status, register_body):
    approved = json.dumps(
        {"id": "ver-ac6", "status": "approved", "enforcementMode": "monitoring"}
    )

    def handler(method, path):
        if path.endswith("/capabilities/register"):
            return (register_status, {}, register_body)
        if path.endswith("/sdk-api/verifications"):
            return (200, {}, approved)
        return (200, {}, "{}")

    return handler


@pytest.mark.parametrize(
    "register_status,register_body",
    [(404, '{"error":"not found"}'), (401, '{"error":"bad key"}')],
    ids=["404", "401"],
)
def test_AIM_14_AC6_rejected_registration_prints_no_registered_line(
    server_factory, capsys, register_status, register_body
):
    server = server_factory(_registration_scenario(register_status, register_body))
    client = _api_key_client(server.url)

    @client.perform_action(capability="net:call")
    def act():
        return "ok"

    assert act() == "ok"
    out = capsys.readouterr().out
    assert "Registered" not in out, (
        f"a registration answered {register_status} must not print a console "
        f"line claiming the capability was registered; got: {out!r}"
    )


def test_AIM_14_AC6_500_registration_path_still_prints_only_a_warning(
    server_factory, capsys
):
    server = server_factory(_registration_scenario(500, '{"error":"boom"}'))
    client = _api_key_client(server.url)

    @client.perform_action(capability="net:call")
    def act():
        return "ok"

    assert act() == "ok"
    out = capsys.readouterr().out
    assert "Registered" not in out, (
        f"the 500 path must keep printing only a warning, no success line: {out!r}"
    )


def test_AIM_14_AC6_granted_registration_announces_exactly_once(
    server_factory, capsys
):
    granted = json.dumps({"status": "granted", "message": "ok"})
    server = server_factory(_registration_scenario(200, granted))
    client = _api_key_client(server.url)

    @client.perform_action(capability="net:call")
    def act():
        return "ok"

    assert act() == "ok"
    out = capsys.readouterr().out
    announcements = [
        line for line in out.splitlines()
        if "Registered" in line and "net:call" in line
    ]
    assert len(announcements) == 1, (
        f"a granted registration must be announced exactly once, got "
        f"{len(announcements)}: {out!r}"
    )


# --------------------------------------------------------------------------- #
# AC7 -- the tree's own record of the batch is coherent.
# --------------------------------------------------------------------------- #
def test_AIM_14_AC7_changelog_documents_the_batch_under_a_post_2_0_2_section():
    text = CHANGELOG.read_text(encoding="utf-8")
    headings = re.findall(r"^## \[(\d+)\.(\d+)\.(\d+)\]", text, re.M)
    assert headings, "CHANGELOG must keep its versioned sections"
    newest = tuple(int(part) for part in headings[0])
    assert newest > (2, 0, 2), (
        f"the batch must be documented under a section newer than 2.0.2; the "
        f"newest section is {'.'.join(map(str, headings[0]))}"
    )

    assert "## [2.0.2] - 2026-09-08" in text, (
        "adding the new section must not displace the 2.0.2 heading"
    )

    sections = re.split(r"^## \[", text, flags=re.M)
    newest_section = sections[1]
    for needle in (
        "login",            # item 1: bounded, fail-fast login
        "aim_url",          # items 2 and 3: the requirement, stated
        "aim-sdk login",    # item 3: the error names the pip user's fix
        "UNAVAILABLE",      # item 4: unreached is not DENIED
        "ACTION_EXECUTED",  # item 4: the unverified run is recorded
        "regist",           # item 5: no success line on a rejected registration
    ):
        assert needle in newest_section, (
            f"the post-2.0.2 CHANGELOG section must document the batch; "
            f"missing {needle!r}"
        )


def test_AIM_14_AC7_readme_credentials_guidance_matches_the_delivered_error(
    monkeypatch,
):
    assert "aim-sdk login" in README.read_text(encoding="utf-8"), (
        "the README must keep documenting aim-sdk login as the pip user's step"
    )
    _no_stored_credentials(monkeypatch)
    with pytest.raises(ConfigurationError) as excinfo:
        register_agent("aim14-ac7-agent")
    assert "aim-sdk login" in str(excinfo.value), (
        "the delivered error must name the same command the README documents"
    )


def test_AIM_14_AC7_this_file_touches_only_loopback():
    source = Path(__file__).read_text(encoding="utf-8")
    for host in re.findall(r"http://([\w\.\-:]+)", source):
        assert host.startswith("127.0.0.1") or host.startswith("localhost"), (
            f"non-loopback URL in this test file: {host}"
        )
    assert ("https" + "://") not in source, (
        "no test added for this task may perform network I/O beyond loopback"
    )
