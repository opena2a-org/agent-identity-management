"""
AIM-21 -- the three P2 findings and the seven P3 findings from the aim-sdk
2.0.3 release test, re-measured at head. One test (or parametrized group) per
acceptance criterion; every leaf test name carries its criterion id as the
first token.

P2.1 (the `aim-sdk login` pre-flight and bounded callback wait) is not covered
here: it is AIM-14.AC1, the same defect against the same lines, and it ships in
this same [Unreleased] section.

Everything here runs offline. HTTP goes only to loopback fakes these tests
start themselves, or to a loopback port that was bound and closed so it refuses
connections, or to a loopback socket that accepts and answers nothing. The only
process any test spawns is the running interpreter on a script the test wrote
into its own tmp_path (test_AIM_21_AC13 asserts both properties over this
file's own source).
"""

import contextlib
import io
import json
import logging
import os
import re
import socket
import subprocess
import sys
import threading
import types
import warnings
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from unittest import mock

import pytest

import aim_sdk
import aim_sdk.capability_detection as capability_detection_module
import aim_sdk.cli as cli
import aim_sdk.client as client_module
import aim_sdk.console as console_module
import aim_sdk.credentials as credentials_module
import aim_sdk.detection as detection_module
import aim_sdk.protocol_detection as protocol_detection_module
from aim_sdk import AIMClient
from aim_sdk.decision import UnknownSource, get_mode_cache
from aim_sdk.detection import MCPDetector
from aim_sdk.exceptions import (
    ConfigurationError,
    VerificationError,
    VerificationUnavailableError,
)
from aim_sdk.protocol_detection import ProtocolDetector
from aim_sdk.strict_mode import reset_warning_state

# Imported defensively so that at the base commit each criterion fails inside
# its own test instead of erroring this module's collection; AC5 asserts the
# helper exists inside the test body that needs it.
try:
    from aim_sdk.integrations.mcp.discovery import describe_exception
except ImportError:  # pragma: no cover - base commit only
    describe_exception = None

SDK_DIR = Path(__file__).resolve().parent.parent          # .../sdk/python
REPO_ROOT = SDK_DIR.parent.parent                         # repo root
CHANGELOG = (SDK_DIR / "CHANGELOG.md").read_text(encoding="utf-8")
RELEASE_SMOKE = (REPO_ROOT / "docs" / "testing" / "release-smoke.md").read_text(
    encoding="utf-8"
)
THIS_FILE = Path(__file__).read_text(encoding="utf-8")

# The seven environment variables the release-test finding named, plus every
# other name in the detector's indicator table, so "no indicator set" is a fact
# rather than a hope about the runner's environment.
PROTOCOL_ENV_VARS = sorted(
    {
        indicator
        for indicators in ProtocolDetector()._protocol_indicators.values()
        for indicator in indicators
    }
)

# The requests/urllib3 chain text that must never reach a caller.
LIBRARY_CHAIN_TEXT = (
    "HTTPConnectionPool",
    "HTTPSConnectionPool",
    "Max retries exceeded",
    "NewConnectionError",
)

# What a real urllib3-backed `requests` raises for a refused connection. Used
# as a stub so these assertions hold whichever `requests` the runner installed.
URLLIB3_CHAIN_MESSAGE = (
    "HTTPConnectionPool(host='127.0.0.1', port=45199): Max retries exceeded "
    "with url: /api/v1/sdk-api/verifications (Caused by NewConnectionError("
    "'<urllib3.connection.HTTPConnection object at 0xdead>: Failed to "
    "establish a new connection: [Errno 111] Connection refused'))"
)

TASKGROUP_WRAPPER_TEXT = "unhandled errors in a TaskGroup"


# ---------------------------------------------------------------------------
# Loopback fixtures
# ---------------------------------------------------------------------------

class _FakeAIM:
    """A loopback AIM fake: canned JSON answers, captured request bodies."""

    def __init__(self, routes):
        self.routes = routes  # {(METHOD, path): (status, body_dict)}
        self.requests = []    # [(method, path, parsed_body_or_bytes)]
        fake = self

        class Handler(BaseHTTPRequestHandler):
            def _serve(self):
                length = int(self.headers.get("Content-Length") or 0)
                raw = self.rfile.read(length) if length else b""
                try:
                    body = json.loads(raw.decode("utf-8")) if raw else None
                except ValueError:
                    body = raw
                fake.requests.append((self.command, self.path, body))
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
def silent_port_url():
    """A loopback socket that accepts a connection and then says nothing."""
    listener = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    listener.bind(("127.0.0.1", 0))
    listener.listen(8)
    listener.settimeout(0.2)
    port = listener.getsockname()[1]
    held = []
    stop = threading.Event()

    def serve():
        while not stop.is_set():
            try:
                conn, _ = listener.accept()
            except socket.timeout:
                continue
            except OSError:
                break
            held.append(conn)  # held open, answered never

    thread = threading.Thread(target=serve, daemon=True)
    thread.start()
    yield f"http://127.0.0.1:{port}"
    stop.set()
    thread.join(timeout=2)
    for conn in held:
        try:
            conn.close()
        except OSError:
            pass
    listener.close()


@pytest.fixture
def isolated_home(monkeypatch, tmp_path):
    """A fresh HOME so no test reads or writes the real user profile."""
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
    monkeypatch.setattr(
        credentials_module, "_find_sdk_package_credentials", lambda: None
    )
    return tmp_path


@pytest.fixture
def clean_verification_state(monkeypatch):
    """The mode cache is process-scoped; a prior ALLOW would warm this one."""
    get_mode_cache().clear()
    reset_warning_state()
    monkeypatch.delenv("AIM_STRICT_MODE", raising=False)
    yield monkeypatch
    get_mode_cache().clear()
    reset_warning_state()


@contextlib.contextmanager
def _captured_output():
    """
    Capture everything the SDK could put in front of a user for one call.

    Three routes at once: ``print``, rich's Console (which holds its own file
    handle), and the SDK's own logging StreamHandlers, which bound
    ``sys.stderr`` at import time and would otherwise write straight past a
    redirect. ``logging.lastResort`` reads ``sys.stderr`` on each emit, so a
    record on a handler-less ``aim_sdk`` logger is caught by the redirect.

    The root logger is deliberately left alone: under pytest its handlers are
    pytest's own log capture, and retargeting them would report the runner's
    log-capture format as SDK console output.
    """
    buffer = io.StringIO()
    swapped = []
    for logger_name in ("aim.security", "aim_sdk"):
        for handler in logging.getLogger(logger_name).handlers:
            if isinstance(handler, logging.StreamHandler) and not isinstance(
                handler, logging.FileHandler
            ):
                swapped.append((handler, handler.stream))
                handler.stream = buffer

    rich_console = getattr(console_module.console, "console", None)
    previous_file = getattr(rich_console, "file", None) if rich_console else None
    if rich_console is not None:
        rich_console.file = buffer

    try:
        with contextlib.redirect_stdout(buffer), contextlib.redirect_stderr(buffer):
            yield buffer
    finally:
        if rich_console is not None:
            rich_console.file = previous_file
        for handler, stream in swapped:
            handler.stream = stream


def _api_key_client(url, **kwargs):
    kwargs.setdefault("telemetry", {"enabled": False})
    return AIMClient(
        agent_id="aim21-agent",
        api_key="aim_test_key",
        aim_url=url,
        **kwargs,
    )


def _signing_client(url, **kwargs):
    """An Ed25519-mode client, for the methods that refuse API-key mode."""
    import base64

    from nacl.encoding import Base64Encoder
    from nacl.signing import SigningKey

    signing_key = SigningKey.generate()
    kwargs.setdefault("telemetry", {"enabled": False})
    kwargs.setdefault("auto_retry", False)
    return AIMClient(
        agent_id="aim21-agent",
        public_key=signing_key.verify_key.encode(encoder=Base64Encoder).decode("utf-8"),
        private_key=base64.b64encode(bytes(signing_key)).decode("utf-8"),
        aim_url=url,
        **kwargs,
    )


class _NoProtocolModules:
    """Stands in for `sys` inside protocol_detection: an empty module table."""

    modules = {"builtins": None, "os": None}


@pytest.fixture
def no_protocol_indicators(monkeypatch):
    """No indicator environment variable set, and no module loaded."""
    for name in PROTOCOL_ENV_VARS:
        monkeypatch.delenv(name, raising=False)
    monkeypatch.setattr(protocol_detection_module, "sys", _NoProtocolModules)
    return monkeypatch


# ---------------------------------------------------------------------------
# AC1 -- protocol detection never scores what it did not find (P2.2)
# ---------------------------------------------------------------------------

def test_AIM_21_AC1_no_indicators_means_zero_confidence_and_no_indicators_found(
    no_protocol_indicators,
):
    detector = ProtocolDetector()
    protocol = detector.detect_protocol()
    details = detector.get_protocol_details(protocol)

    assert details["indicators_found"] == [], (
        "nothing was set and nothing was imported, so nothing was found"
    )
    assert details["confidence"] == 0.0, (
        f"a default is not a detection; get_protocol_details reported "
        f"{details['confidence']} for {protocol!r} with no indicators"
    )
    assert detector.get_detection_confidence(protocol) == 0.0


def test_AIM_21_AC1_default_protocol_is_still_mcp(no_protocol_indicators):
    """The committed test at tests/test_protocol_detection.py:100-122 pins this
    return value; only the score it was reported with changes."""
    assert ProtocolDetector().detect_protocol() == "mcp"
    assert protocol_detection_module.auto_detect_protocol() == "mcp"


@pytest.mark.parametrize("protocol", ["mcp", "a2a", "oauth", "saml", "did", "acp"])
def test_AIM_21_AC1_every_known_protocol_scores_zero_with_no_indicators(
    no_protocol_indicators, protocol
):
    detector = ProtocolDetector()
    assert detector.get_detection_confidence(protocol) == 0.0
    assert detector.get_protocol_details(protocol)["confidence"] == 0.0


# ---------------------------------------------------------------------------
# AC2 -- an unknown protocol name never scores (P2.2, negative)
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("unknown", ["bogus", "", "MCP ", "grpc", "not-a-protocol"])
def test_AIM_21_AC2_unknown_protocol_name_scores_zero(no_protocol_indicators, unknown):
    detector = ProtocolDetector()
    details = detector.get_protocol_details(unknown)

    assert details["indicators_found"] == []
    assert details["confidence"] == 0.0
    assert detector.get_detection_confidence(unknown) == 0.0


def test_AIM_21_AC2_env_indicator_cells_keep_their_scores(no_protocol_indicators):
    """The positive control: the committed assertions at
    tests/test_protocol_detection.py:47 and :75 still hold."""
    no_protocol_indicators.setenv("MCP_SERVER_MODE", "true")
    detector = ProtocolDetector()
    protocol = detector.detect_protocol()
    assert protocol == "mcp"
    assert detector.get_detection_confidence(protocol) >= 90
    assert detector.get_protocol_details(protocol)["indicators_found"]

    no_protocol_indicators.delenv("MCP_SERVER_MODE")
    no_protocol_indicators.setenv("A2A_AGENT_MODE", "client")
    detector = ProtocolDetector()
    protocol = detector.detect_protocol()
    assert protocol == "a2a"
    assert detector.get_detection_confidence(protocol) >= 90


def test_AIM_21_AC2_import_indicator_still_scores(no_protocol_indicators):
    """The import row, unchanged: a loaded protocol module scores 60+."""
    no_protocol_indicators.setattr(
        protocol_detection_module,
        "sys",
        types.SimpleNamespace(modules={"mcp_server": None}),
    )
    detector = ProtocolDetector()
    assert detector.get_detection_confidence("mcp") >= 60


# ---------------------------------------------------------------------------
# AC3 -- transport failures have one shape across methods (P2.3)
# ---------------------------------------------------------------------------

def _assert_transport_shape(message, host, url):
    """The three things every transport failure message must carry."""
    assert host in message, f"no host from aim_url in: {message}"
    assert url in message, f"no URL to check in: {message}"
    lowered = message.lower()
    assert any(
        marker in lowered
        for marker in ("connection refused", "timed out", "could not connect")
    ) or re.search(r"\((?:[A-Za-z]*Error|[A-Za-z]*Timeout|[A-Za-z]*Exception)\)", message), (
        f"no failure class in: {message}"
    )
    for chain in LIBRARY_CHAIN_TEXT:
        assert chain not in message, f"library chain text {chain!r} in: {message}"


@pytest.mark.parametrize(
    "method_name,args",
    [
        ("report_detections", ([{"mcpServer": "x", "detectionMethod": "manual"}],)),
        ("report_sdk_integration", ()),
        ("register_mcp", ("mcp-server-id",)),
    ],
)
def test_AIM_21_AC3_connection_refused_has_one_shape(
    closed_port_url, method_name, args
):
    client = _api_key_client(closed_port_url, auto_retry=False)
    host = closed_port_url.split("//", 1)[1]

    with pytest.raises(VerificationError) as excinfo:
        getattr(client, method_name)(*args)

    message = str(excinfo.value)
    _assert_transport_shape(message, host, closed_port_url)
    assert "connection refused" in message.lower()


def test_AIM_21_AC3_timeout_has_the_same_shape(silent_port_url):
    client = _api_key_client(silent_port_url, timeout=0.75, auto_retry=False)
    host = silent_port_url.split("//", 1)[1]

    with pytest.raises(VerificationError) as excinfo:
        client.report_detections([{"mcpServer": "x", "detectionMethod": "manual"}])

    message = str(excinfo.value)
    _assert_transport_shape(message, host, silent_port_url)
    assert "timed out" in message.lower()


def test_AIM_21_AC3_verification_path_builds_the_same_shape(
    closed_port_url, clean_verification_state
):
    clean_verification_state.setenv("AIM_STRICT_MODE", "true")
    client = _api_key_client(closed_port_url, auto_retry=False)
    host = closed_port_url.split("//", 1)[1]

    decision = client._decide_capability("db:read", resource="users")
    assert decision.unknown_source is UnknownSource.TRANSPORT
    _assert_transport_shape(decision.reason, host, closed_port_url)

    with pytest.raises(VerificationUnavailableError) as excinfo:
        client.verify_capability("db:read", resource="users")
    _assert_transport_shape(str(excinfo.value), host, closed_port_url)


def test_AIM_21_AC3_reporting_and_verification_agree_word_for_word(
    closed_port_url, clean_verification_state
):
    """The point of "one shape": the same closed port produces the same
    sentence whichever method the caller happened to be in."""
    client = _api_key_client(closed_port_url, auto_retry=False)

    with pytest.raises(VerificationError) as excinfo:
        client.report_detections([{"mcpServer": "x", "detectionMethod": "manual"}])
    reporting_message = str(excinfo.value)

    verification_reason = client._decide_capability("db:read").reason
    assert reporting_message == verification_reason


def test_AIM_21_AC3_a_server_that_answered_is_not_called_unreachable(
    fake_aim_factory,
):
    """
    The boundary of "one shape". A 4xx is not a transport failure: the server
    was reached and answered, so the message must not send its operator to
    check that the server is running. Only the two branches AC3 names -- no
    answer at all -- get the "Could not reach AIM" sentence.
    """
    fake = fake_aim_factory({})  # every route 404s
    client = _api_key_client(fake.url, auto_retry=False)

    with pytest.raises(VerificationError) as excinfo:
        client.report_detections([{"mcpServer": "x", "detectionMethod": "manual"}])

    message = str(excinfo.value)
    assert "Could not reach AIM" not in message, (
        f"a server that answered 404 was reported as unreachable: {message}"
    )
    assert "404" in message, message
    for chain in LIBRARY_CHAIN_TEXT:
        assert chain not in message, message


# ---------------------------------------------------------------------------
# AC4 -- no raw library chain and no print beside a raise (P2.3, negative)
# ---------------------------------------------------------------------------

def _urllib3_chain_stub(*args, **kwargs):
    import requests

    raise requests.exceptions.ConnectionError(URLLIB3_CHAIN_MESSAGE)


@pytest.mark.parametrize("chain_text", LIBRARY_CHAIN_TEXT)
def test_AIM_21_AC4_raised_message_never_carries_the_library_chain(
    closed_port_url, chain_text
):
    """Driven with the exact text a urllib3-backed `requests` raises, so the
    property holds whichever `requests` is installed on the runner."""
    client = _api_key_client(closed_port_url, auto_retry=False)
    client.session.request = _urllib3_chain_stub

    with pytest.raises(VerificationError) as excinfo:
        client.report_detections([{"mcpServer": "x", "detectionMethod": "manual"}])
    assert chain_text not in str(excinfo.value)


@pytest.mark.parametrize("chain_text", LIBRARY_CHAIN_TEXT)
def test_AIM_21_AC4_returned_reason_never_carries_the_library_chain(
    closed_port_url, clean_verification_state, chain_text
):
    client = _api_key_client(closed_port_url, auto_retry=False)
    client.session.request = _urllib3_chain_stub

    decision = client._decide_capability("db:read")
    assert chain_text not in (decision.reason or "")

    with pytest.raises(VerificationUnavailableError) as excinfo:
        client.verify_capability("db:read")
    assert chain_text not in str(excinfo.value)


def _transport_messages_the_sdk_builds(url, monkeypatch=None):
    """
    Every message this SDK composes from a `requests` transport failure.

    Not just the ones `_make_request` raises: the methods that post with
    `self.session` directly, the ones that RETURN the failure in a result dict,
    and the CLI's own token exchange each compose their own sentence, and each
    of them used to compose it by interpolating `str(e)`.

    With ``monkeypatch``, every transport is replaced by a stub that raises the
    exact text a urllib3-backed `requests` raises. That matters: against a real
    closed port the exception text depends on which `requests` the runner has,
    so a "the chain is absent" assertion would pass for the wrong reason on a
    runner whose `requests` never produces it.
    """
    client = _api_key_client(url, auto_retry=False)
    # attest_mcp posts with `self.session` directly and refuses API-key mode,
    # so it needs a signing client of its own.
    signing = _signing_client(url)

    if monkeypatch is not None:
        import requests

        monkeypatch.setattr(requests, "post", _urllib3_chain_stub)
        monkeypatch.setattr(requests, "get", _urllib3_chain_stub)
        for each in (client, signing):
            monkeypatch.setattr(each.session, "request", _urllib3_chain_stub)
            monkeypatch.setattr(each.session, "post", _urllib3_chain_stub)

    messages = {}

    with pytest.raises(VerificationError) as excinfo:
        client.report_detections([{"mcpServer": "x", "detectionMethod": "manual"}])
    messages["_make_request"] = str(excinfo.value)

    with pytest.raises(VerificationError) as excinfo:
        signing.attest_mcp(mcp_server_id="11111111-2222-3333-4444-555555555555")
    messages["attest_mcp"] = str(excinfo.value)

    messages["_register_single_mcp"] = client_module._register_single_mcp(
        client=client,
        aim_url=url,
        headers={"Content-Type": "application/json"},
        mcp_def={"name": "srv", "url": "stdio://srv", "capabilities": []},
        agent_id="aim21-agent",
        detected_capabilities={},
    )["error"]

    messages["exchange_code_for_tokens"] = cli.exchange_code_for_tokens(
        url, "code", "verifier", "http://127.0.0.1:1/callback"
    )["error"]

    messages["_decide_capability"] = client._decide_capability("db:read").reason

    return messages


@pytest.mark.parametrize("chain_text", LIBRARY_CHAIN_TEXT)
def test_AIM_21_AC4_no_composed_message_carries_the_library_chain(
    closed_port_url, clean_verification_state, chain_text
):
    messages = _transport_messages_the_sdk_builds(
        closed_port_url, monkeypatch=clean_verification_state
    )
    for origin, message in messages.items():
        assert message, f"{origin} reported a transport failure with no message"
        assert chain_text not in message, (
            f"{origin} put the library chain text {chain_text!r} in: {message}"
        )


def test_AIM_21_AC4_every_composed_message_carries_the_one_shape(
    closed_port_url, clean_verification_state
):
    """The positive half of the sweep: each of those messages is the shape
    AC3 describes, so "no chain" was not bought by saying nothing."""
    host = closed_port_url.split("//", 1)[1]
    for origin, message in _transport_messages_the_sdk_builds(closed_port_url).items():
        _assert_transport_shape(message, host, closed_port_url)
        assert "connection refused" in message.lower(), f"{origin}: {message}"


def test_AIM_21_AC4_a_stubbed_chain_is_scrubbed_from_every_shape(closed_port_url):
    """
    Driven with the exact text a urllib3-backed `requests` raises, on the branch
    that renders a failure which is NOT a transport failure -- the one place a
    chain could still be copied through verbatim.
    """
    import requests

    scrubbed = client_module._request_failure_message(
        requests.exceptions.RequestException(URLLIB3_CHAIN_MESSAGE),
        closed_port_url,
    )
    for chain in LIBRARY_CHAIN_TEXT:
        assert chain not in scrubbed, scrubbed
    assert "RequestException" in scrubbed, scrubbed


@pytest.mark.parametrize("entry_point", ["verify_capability", "verify_action"])
def test_AIM_21_AC4_nothing_is_printed_beside_the_raise(
    closed_port_url, clean_verification_state, entry_point
):
    clean_verification_state.setenv("AIM_STRICT_MODE", "true")
    client = _api_key_client(closed_port_url, auto_retry=False)

    with _captured_output() as buffer:
        with pytest.raises(VerificationUnavailableError):
            getattr(client, entry_point)("db:read")

    assert buffer.getvalue() == "", (
        f"{entry_point} raised AND wrote to the console: "
        f"{buffer.getvalue()!r}"
    )


def test_AIM_21_AC4_the_permissive_path_still_warns(
    fake_aim_factory, clean_verification_state
):
    """
    Positive control. The console warning did not disappear, it moved to the
    path that acts on the decision instead of raising it: monitoring mode runs
    the body and says so, exactly as before.
    """
    fake = fake_aim_factory({
        ("POST", "/api/v1/sdk-api/verifications"): (200, {
            "id": "11111111-2222-3333-4444-555555555555",
            "status": "approved",
            "enforcementMode": "monitoring",
        }),
    })
    client = _api_key_client(fake.url, auto_retry=False)
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        client.verify_capability("warm:up")  # warms the mode cache: monitoring

    client.session.request = _urllib3_chain_stub

    @client.perform_action("db:read", resource="users")
    def read_users():
        return "rows"

    with _captured_output() as buffer:
        with warnings.catch_warnings():
            warnings.simplefilter("ignore")
            assert read_users() == "rows"

    printed = buffer.getvalue()
    assert "enforcement mode is monitoring" in printed, (
        f"the permissive path printed no enforcement warning: {printed!r}"
    )
    assert "executed anyway" in printed, printed
    for chain in LIBRARY_CHAIN_TEXT:
        assert chain not in printed, f"library chain text {chain!r} printed: {printed}"


# ---------------------------------------------------------------------------
# AC5 -- tool discovery reports the real cause (P2.4)
# ---------------------------------------------------------------------------

MISSING_COMMAND = "aim21-no-such-mcp-server-on-path"


@pytest.fixture
def claude_config_with_broken_servers(tmp_path, monkeypatch):
    """Two servers that cannot be used, one config, a temporary HOME."""
    exit_script = tmp_path / "exits_without_speaking_mcp.py"
    exit_script.write_text("import sys\nsys.exit(3)\n", encoding="utf-8")

    config_dir = tmp_path / ".claude"
    config_dir.mkdir()
    (config_dir / "claude_desktop_config.json").write_text(
        json.dumps({
            "mcpServers": {
                "ghost": {"command": MISSING_COMMAND, "args": ["--serve"]},
                "silent": {"command": sys.executable, "args": [str(exit_script)]},
            }
        }),
        encoding="utf-8",
    )
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))
    return {"script": exit_script}


def _detections_by_server(detections):
    return {d["mcpServer"]: d for d in detections}


@pytest.fixture
def without_mcp_client_library(monkeypatch):
    """
    Force the branch that runs when the MCP client library is absent.

    Which branch a runner takes is an installation fact -- CI installs
    `.[dev,mcp]`, a bare checkout has neither. Both branches ask the command
    itself why it could not be used, so the assertions here hold on either;
    this fixture pins the one a runner with `mcp` installed would not take.
    """
    from aim_sdk.integrations.mcp import discovery as discovery_module

    monkeypatch.setattr(discovery_module, "MCP_SDK_AVAILABLE", False)
    return discovery_module


@pytest.mark.parametrize("server", ["ghost", "silent"])
def test_AIM_21_AC5_discovery_error_names_the_command_and_never_the_wrapper(
    claude_config_with_broken_servers, server
):
    """Holds on whichever branch this runner takes."""
    detections = MCPDetector().detect_with_tools(timeout_per_server=5.0)
    detection = _detections_by_server(detections)[server]
    error = detection["details"]["discoveryError"]
    expected_command = (
        MISSING_COMMAND
        if server == "ghost"
        else str(claude_config_with_broken_servers["script"])
    )

    assert error, "a server that could not be queried must report why"
    assert expected_command in error, f"the command is not named in: {error}"
    assert server in error, f"the server is not named in: {error}"
    assert TASKGROUP_WRAPPER_TEXT not in error, f"anyio wrapper text in: {error}"
    assert "MCP SDK not installed" not in error, (
        f"an installation note replaced the real cause: {error}"
    )


def test_AIM_21_AC5_missing_command_reports_the_os_error(
    claude_config_with_broken_servers,
):
    detections = MCPDetector().detect_with_tools(timeout_per_server=5.0)
    error = _detections_by_server(detections)["ghost"]["details"]["discoveryError"]

    assert MISSING_COMMAND in error, error
    assert "No such file or directory" in error, (
        f"the underlying OSError text is not in: {error}"
    )


def test_AIM_21_AC5_command_that_exits_immediately_reports_the_transport_failure(
    claude_config_with_broken_servers,
):
    detections = MCPDetector().detect_with_tools(timeout_per_server=5.0)
    error = _detections_by_server(detections)["silent"]["details"]["discoveryError"]

    assert str(claude_config_with_broken_servers["script"]) in error, error
    assert "exited with status 3" in error, (
        f"the exit status of the server process is not in: {error}"
    )
    assert "initialize" in error, (
        f"the failure is not described as a protocol failure in: {error}"
    )


def test_AIM_21_AC5_a_working_server_is_not_reported_as_broken(
    tmp_path, monkeypatch, without_mcp_client_library
):
    """
    Negative control for the probe: a command that starts and answers is not
    accused of failing to start. Without this, "report the real cause" could be
    satisfied by calling every server broken.
    """
    script = tmp_path / "answers_initialize.py"
    script.write_text(
        "import sys\n"
        "sys.stdin.readline()\n"
        'sys.stdout.write("{\\"jsonrpc\\":\\"2.0\\",\\"id\\":0,'
        '\\"result\\":{}}\\n")\n'
        "sys.stdout.flush()\n"
        "sys.stdin.readline()\n",
        encoding="utf-8",
    )
    config_dir = tmp_path / ".claude"
    config_dir.mkdir()
    (config_dir / "claude_desktop_config.json").write_text(
        json.dumps({
            "mcpServers": {
                "healthy": {"command": sys.executable, "args": [str(script)]}
            }
        }),
        encoding="utf-8",
    )
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))

    detections = MCPDetector().detect_with_tools(timeout_per_server=10.0)
    error = _detections_by_server(detections)["healthy"]["details"]["discoveryError"]

    assert "could not be executed" not in error, error
    assert "exited with status" not in error, error
    assert "pip install mcp" in error, (
        f"a server that answered should report only the missing client "
        f"library: {error}"
    )


def test_AIM_21_AC5_discovery_writes_no_traceback_to_stderr(
    claude_config_with_broken_servers,
):
    with _captured_output() as buffer:
        detections = MCPDetector().detect_with_tools(timeout_per_server=5.0)

    assert len(detections) == 2
    assert "Traceback" not in buffer.getvalue(), (
        f"a third-party traceback reached the console: {buffer.getvalue()!r}"
    )


def test_AIM_21_AC5_taskgroup_wrapper_is_unwrapped_to_its_cause():
    """The MCP-client-installed path: a failed stdio session arrives as an
    exception group whose str() is the wrapper sentence."""
    assert describe_exception is not None, (
        "aim_sdk.integrations.mcp.discovery exposes no describe_exception, so "
        "the anyio wrapper text is what reaches details.discoveryError"
    )
    leaf = FileNotFoundError(2, "No such file or directory", MISSING_COMMAND)
    try:
        raise BaseExceptionGroup(TASKGROUP_WRAPPER_TEXT, [leaf])
    except BaseExceptionGroup as group:
        described = describe_exception(group)

    assert TASKGROUP_WRAPPER_TEXT not in described
    assert "FileNotFoundError" in described
    assert MISSING_COMMAND in described


@pytest.mark.parametrize("entry_point", ["detect_with_tools", "auto_detect_mcps"])
def test_AIM_21_AC5_docstrings_say_the_server_commands_are_executed(entry_point):
    documented = (
        MCPDetector.detect_with_tools.__doc__
        if entry_point == "detect_with_tools"
        else detection_module.auto_detect_mcps.__doc__
    )
    lowered = documented.lower()
    assert "execut" in lowered, documented
    assert "command" in lowered, documented
    assert "discover_tools" in lowered, documented


# ---------------------------------------------------------------------------
# AC6 -- nonsense arguments are refused at construction (P3.1, negative)
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("timeout", [0, -1, "abc", None, 0.0])
def test_AIM_21_AC6_bad_timeout_is_refused(timeout):
    with pytest.raises(ConfigurationError) as excinfo:
        AIMClient(
            agent_id="agent-123",
            api_key="k",
            aim_url="https://aim.example.com",
            timeout=timeout,
        )
    assert "timeout" in str(excinfo.value)


@pytest.mark.parametrize("max_retries", [-1, 1.5, "3"])
def test_AIM_21_AC6_bad_max_retries_is_refused(max_retries):
    with pytest.raises(ConfigurationError) as excinfo:
        AIMClient(
            agent_id="agent-123",
            api_key="k",
            aim_url="https://aim.example.com",
            max_retries=max_retries,
        )
    assert "max_retries" in str(excinfo.value)


@pytest.mark.parametrize(
    "aim_url", ["not a url", "aim.example.com", "ftp://aim.example.com", "http://"]
)
def test_AIM_21_AC6_bad_aim_url_is_refused(aim_url):
    with pytest.raises(ConfigurationError) as excinfo:
        AIMClient(agent_id="agent-123", api_key="k", aim_url=aim_url)
    assert "aim_url" in str(excinfo.value)


def test_AIM_21_AC6_good_arguments_are_accepted():
    """Negative control, and the reason agent_id is not UUID-validated: 13
    committed tests construct clients with ids of exactly this shape."""
    client = AIMClient(
        agent_id="agent-123",
        api_key="k",
        aim_url="http://localhost:8080/",
        timeout=1,
        max_retries=0,
        telemetry={"enabled": False},
    )
    assert client.agent_id == "agent-123"
    assert client.aim_url == "http://localhost:8080"


def test_AIM_21_AC6_mcp_detector_sdk_version_is_validated():
    with pytest.raises(TypeError):
        MCPDetector(sdk_version=123)
    with pytest.raises(ValueError):
        MCPDetector(sdk_version="")
    assert MCPDetector(sdk_version=None).sdk_version == (
        f"aim-sdk-python@{aim_sdk.__version__}"
    )
    assert MCPDetector(sdk_version="explicit@1.2.3").sdk_version == "explicit@1.2.3"


# ---------------------------------------------------------------------------
# AC7 -- a corrupt Claude config is reported, not swallowed (P3.2)
# ---------------------------------------------------------------------------

@pytest.fixture
def corrupt_claude_config(tmp_path, monkeypatch):
    config_dir = tmp_path / ".claude"
    config_dir.mkdir()
    config_path = config_dir / "claude_desktop_config.json"
    config_path.write_text('{"mcpServers": {,}', encoding="utf-8")
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))
    # `detect_all` also scans loaded modules and installed distributions. Under
    # the full suite `tests.test_mcp_integration` is loaded and the substring
    # rule in `_is_mcp_module` turns it into an sdk_import row -- a property of
    # the runner, not of the config being read. Both scans are emptied so this
    # test measures only what the corrupt config contributes.
    monkeypatch.setattr(
        detection_module,
        "sys",
        types.SimpleNamespace(modules={}, platform=sys.platform),
    )
    monkeypatch.setattr(detection_module, "distributions", None)
    return config_path


@pytest.mark.parametrize(
    "call",
    [
        lambda d: d.detect_from_claude_config(),
        lambda d: d.detect_all(),
        lambda d: d.detect_with_tools(),
    ],
    ids=["detect_from_claude_config", "detect_all", "detect_with_tools"],
)
def test_AIM_21_AC7_corrupt_config_returns_empty_and_warns_once(
    corrupt_claude_config, call
):
    detector = MCPDetector()

    with warnings.catch_warnings(record=True) as caught:
        warnings.simplefilter("always")
        result = call(detector)

    assert result == [], "a config that cannot be parsed yields no detections"
    assert len(caught) == 1, (
        f"expected exactly one warning, got {[str(w.message) for w in caught]}"
    )
    message = str(caught[0].message)
    assert str(corrupt_claude_config) in message, message
    assert "JSON" in message, message
    assert "line" in message or "char" in message, (
        f"the decode error itself is not in: {message}"
    )


def test_AIM_21_AC7_corrupt_config_never_raises(corrupt_claude_config):
    with warnings.catch_warnings():
        warnings.simplefilter("ignore")
        detector = MCPDetector()
        assert detector.detect_from_claude_config() == []
        assert detector.detect_with_tools() == []
        assert detection_module.get_mcp_server_config("anything") is None


# ---------------------------------------------------------------------------
# AC8 -- the config-search docstring and the search agree (P3.3)
# ---------------------------------------------------------------------------

DOCUMENTED_SEARCH_PATHS = [
    "~/Library/Application Support/Claude/claude_desktop_config.json",
    "%APPDATA%\\Claude\\claude_desktop_config.json",
    "~/.config/Claude/claude_desktop_config.json",
    "~/.claude/claude_desktop_config.json",
]

NOT_SEARCHED_PATHS = ["~/.cursor/mcp.json", "./.cursor/mcp.json", "./mcp.json"]


def test_AIM_21_AC8_docstring_lists_exactly_the_searched_paths_in_order():
    doc = MCPDetector.detect_from_claude_config.__doc__
    positions = []
    for path in DOCUMENTED_SEARCH_PATHS:
        index = doc.find(path)
        assert index != -1, f"{path} is searched but not documented"
        positions.append(index)

    assert positions == sorted(positions), (
        "the docstring lists the searched paths out of search order"
    )
    assert doc.count("claude_desktop_config.json") == len(DOCUMENTED_SEARCH_PATHS), (
        "the docstring names a config path that is not searched"
    )


def test_AIM_21_AC8_search_matches_the_documented_list_on_this_platform(
    tmp_path, monkeypatch
):
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))

    searched = [str(p) for p in MCPDetector._claude_config_search_paths()]
    expected = [
        str(tmp_path / ".config" / "Claude" / "claude_desktop_config.json"),
        str(tmp_path / ".claude" / "claude_desktop_config.json"),
    ]
    if sys.platform == "darwin":
        expected.insert(
            0,
            str(
                tmp_path / "Library" / "Application Support" / "Claude"
                / "claude_desktop_config.json"
            ),
        )
    if os.name == "nt" and os.getenv("APPDATA"):
        expected.insert(
            0 if sys.platform != "darwin" else 1,
            str(Path(os.environ["APPDATA"]) / "Claude" / "claude_desktop_config.json"),
        )

    assert searched == expected


def test_AIM_21_AC8_xdg_config_location_is_actually_searched(tmp_path, monkeypatch):
    """The functional half: a config only Claude Desktop's Linux location holds
    is found, where before the search never looked there."""
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))
    config_dir = tmp_path / ".config" / "Claude"
    config_dir.mkdir(parents=True)
    (config_dir / "claude_desktop_config.json").write_text(
        json.dumps({"mcpServers": {"xdg-server": {"command": "true", "args": []}}}),
        encoding="utf-8",
    )

    detections = MCPDetector().detect_from_claude_config()
    assert [d["mcpServer"] for d in detections] == ["xdg-server"]
    assert detections[0]["details"]["configPath"] == str(
        config_dir / "claude_desktop_config.json"
    )


def test_AIM_21_AC8_unsearched_locations_are_named_as_unsearched(tmp_path, monkeypatch):
    doc = MCPDetector.detect_from_claude_config.__doc__
    for path in NOT_SEARCHED_PATHS:
        assert path in doc, f"{path} is neither searched nor documented as unsearched"
    assert "NOT searched" in doc

    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))
    searched = [str(p) for p in MCPDetector._claude_config_search_paths()]
    assert not any("cursor" in p for p in searched)
    assert not any(p.endswith("mcp.json") for p in searched)


# ---------------------------------------------------------------------------
# AC9 -- the detection modules declare their public surface (P3.4)
# ---------------------------------------------------------------------------

DETECTION_MODULES = [
    detection_module,
    protocol_detection_module,
    capability_detection_module,
]


@pytest.mark.parametrize(
    "module", DETECTION_MODULES, ids=lambda m: m.__name__.rsplit(".", 1)[-1]
)
def test_AIM_21_AC9_module_defines_all_and_every_name_resolves(module):
    exported = getattr(module, "__all__", None)
    assert exported, f"{module.__name__} defines no __all__"
    for name in exported:
        assert hasattr(module, name), f"{module.__name__}.__all__ names {name!r}"
        assert not isinstance(getattr(module, name), types.ModuleType), (
            f"{module.__name__}.__all__ exports the module {name!r}"
        )


@pytest.mark.parametrize(
    "module_path",
    ["aim_sdk.detection", "aim_sdk.protocol_detection", "aim_sdk.capability_detection"],
)
def test_AIM_21_AC9_star_import_binds_no_stdlib_or_typing_name(module_path):
    namespace = {}
    exec(f"from {module_path} import *", namespace)  # noqa: S102 - the property under test
    bound = set(namespace) - {"__builtins__"}

    leaked = bound & {
        "json", "os", "sys", "pathlib", "ast", "inspect", "logging", "warnings",
        "importlib", "Optional", "List", "Dict", "Any", "Tuple", "Set",
        "datetime", "timezone", "dataclass", "distributions",
    }
    assert leaked == set(), f"{module_path} star-import leaks {sorted(leaked)}"
    assert bound == set(sys.modules[module_path].__all__)


def test_AIM_21_AC9_package_all_resolves_to_non_module_attributes():
    for name in aim_sdk.__all__:
        assert hasattr(aim_sdk, name), f"aim_sdk.__all__ names {name!r}, absent"
        assert not isinstance(getattr(aim_sdk, name), types.ModuleType), (
            f"aim_sdk.__all__ exports the module {name!r}"
        )


# ---------------------------------------------------------------------------
# AC10 -- the CLI answers in JSON and knows the word help (P3.5)
# ---------------------------------------------------------------------------

def _run_cli(argv):
    with mock.patch.object(sys, "argv", ["aim-sdk"] + argv):
        return cli.main()


def test_AIM_21_AC10_status_json_unauthenticated_is_one_object_and_exit_1(
    isolated_home, capsys
):
    code = _run_cli(["status", "--json"])
    out, err = capsys.readouterr()

    assert code == 1, "the base exit code for 'not authenticated' is kept"
    payload = json.loads(out)
    assert set(payload) == {
        "authenticated", "server", "user", "credentialsPath", "tokenState"
    }
    assert payload["authenticated"] is False
    assert payload["tokenState"] == "absent"
    assert out.strip().count("\n") == 0, f"more than one line on stdout: {out!r}"
    assert err == ""


def test_AIM_21_AC10_status_json_authenticated_reports_the_credentials(
    isolated_home, capsys
):
    aim_dir = isolated_home / ".aim"
    aim_dir.mkdir(parents=True, exist_ok=True)
    (aim_dir / "sdk_credentials.json").write_text(
        json.dumps({
            "aimUrl": "https://aim.example.com",
            "userEmail": "someone@example.com",
            "refreshToken": "refresh-token",
            "accessToken": "not.a.jwt",
        }),
        encoding="utf-8",
    )

    code = _run_cli(["status", "--json"])
    out, _ = capsys.readouterr()
    payload = json.loads(out)

    assert code == 0
    assert payload["authenticated"] is True
    assert payload["server"] == "https://aim.example.com"
    assert payload["user"] == "someone@example.com"
    assert payload["credentialsPath"] == str(aim_dir / "sdk_credentials.json")
    assert payload["tokenState"] == "unknown"


def test_AIM_21_AC10_version_json_reports_the_installed_version(capsys):
    code = _run_cli(["version", "--json"])
    out, _ = capsys.readouterr()

    assert code == 0
    assert json.loads(out) == {"version": aim_sdk.__version__}


def test_AIM_21_AC10_help_verb_and_bare_invocation_both_exit_zero(capsys):
    help_code = _run_cli(["help"])
    help_out, _ = capsys.readouterr()

    bare_code = _run_cli([])
    bare_out, _ = capsys.readouterr()

    assert help_code == 0, "`aim-sdk help` is a successful invocation"
    assert bare_code == 0, "`aim-sdk` with no arguments is a successful invocation"
    assert help_out == bare_out, "the two spellings print different help"
    assert "usage: aim-sdk" in help_out
    for command in ("login", "logout", "status", "version"):
        assert command in help_out


def test_AIM_21_AC10_human_output_is_unchanged_without_the_flag(isolated_home, capsys):
    """Negative control: --json adds a rendering, it does not replace one."""
    code = _run_cli(["status"])
    out, _ = capsys.readouterr()

    assert code == 1
    assert "Not authenticated." in out
    assert "aim-sdk login" in out


# ---------------------------------------------------------------------------
# AC11 -- the release-smoke matrix carries a Python SDK row (P3.6)
# ---------------------------------------------------------------------------

SMOKE_RUN_COMMAND = (
    "python -c \"from aim_sdk import detection; assert "
    "detection.MCPDetector().sdk_version == 'aim-sdk-python@' + "
    "__import__('aim_sdk').__version__\""
)


def _smoke_matrix_rows():
    lines = RELEASE_SMOKE.splitlines()
    start = lines.index("## Component smoke matrix")
    rows = []
    for line in lines[start:]:
        if line.startswith("|") and not re.fullmatch(r"\|[-|]+\|", line.strip()):
            cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
            rows.append(cells)
    return rows


def test_AIM_21_AC11_matrix_has_a_python_sdk_row_with_the_verbatim_run_command():
    rows = [r for r in _smoke_matrix_rows() if SMOKE_RUN_COMMAND in r[-1]]
    assert len(rows) == 1, (
        "expected exactly one component smoke row whose run command is the "
        "documented Python SDK one-liner"
    )
    row = rows[0]
    assert len(row) == 5, row
    assert "aim_sdk" in row[0] or "Python SDK" in row[0], row[0]

    verifies = row[3]
    assert "sdkVersion" in verifies, verifies
    assert "installed" in verifies, verifies


def test_AIM_21_AC11_the_asserted_expression_is_true_of_this_tree():
    from aim_sdk import detection

    assert detection.MCPDetector().sdk_version == (
        'aim-sdk-python@' + __import__('aim_sdk').__version__
    )


# ---------------------------------------------------------------------------
# AC12 -- report_sdk_integration defaults, and the docstring names the
#         replacement API (P3.7)
# ---------------------------------------------------------------------------

REPORT_ROUTE = "/api/v1/sdk-api/agents/aim21-agent/detection/report"


def _report_fake(fake_aim_factory):
    return fake_aim_factory({
        ("POST", REPORT_ROUTE): (200, {"success": True, "detectionsProcessed": 1}),
    })


def _sent_sdk_version(fake):
    method, path, body = fake.requests[-1]
    return body["detections"][0]["sdkVersion"]


def test_AIM_21_AC12_omitted_sdk_version_sends_the_installed_version(
    fake_aim_factory,
):
    fake = _report_fake(fake_aim_factory)
    client = _api_key_client(fake.url)

    client.report_sdk_integration()

    assert _sent_sdk_version(fake) == f"aim-sdk-python@{aim_sdk.__version__}"


def test_AIM_21_AC12_explicit_none_sends_the_installed_version(fake_aim_factory):
    fake = _report_fake(fake_aim_factory)
    client = _api_key_client(fake.url)

    client.report_sdk_integration(sdk_version=None, platform="python")

    assert _sent_sdk_version(fake) == f"aim-sdk-python@{aim_sdk.__version__}"


@pytest.mark.parametrize("style", ["positional", "keyword"])
def test_AIM_21_AC12_explicit_version_is_sent_unchanged(fake_aim_factory, style):
    fake = _report_fake(fake_aim_factory)
    client = _api_key_client(fake.url)
    explicit = "aim-sdk-python@0.0.1-explicit"

    if style == "positional":
        client.report_sdk_integration(explicit)
    else:
        client.report_sdk_integration(sdk_version=explicit)

    assert _sent_sdk_version(fake) == explicit


def test_AIM_21_AC12_signature_default_is_none():
    import inspect

    default = inspect.signature(
        AIMClient.report_sdk_integration
    ).parameters["sdk_version"].default
    assert default is None


def test_AIM_21_AC12_client_docstring_names_the_replacement_api():
    doc = AIMClient.__doc__
    assert "verify_capability" in doc, doc
    assert "aim_sdk.decision.VerificationDecision" in doc, doc


# ---------------------------------------------------------------------------
# AC13 -- the tree's own record of the batch is coherent
# ---------------------------------------------------------------------------

CHANGELOG_MARKERS = [
    "Protocol detection never scores what it did not find",
    "Transport failures have one shape across every method",
    "No raw library chain, and no print beside a raise",
    "Tool discovery reports the real cause",
    "Nonsense arguments are refused at construction",
    "A corrupt Claude Desktop config is reported, not swallowed",
    "The config-search docstring and the search agree",
    "The detection modules declare their public surface",
    "The CLI answers in JSON and knows the word `help`",
    "carries a Python SDK row",
    "`report_sdk_integration` defaults to the installed version",
]


def _section_above_2_0_3():
    """The section the ten fixes are documented under: newer than [2.0.3]."""
    index = CHANGELOG.index("## [2.0.3]")
    return CHANGELOG[:index]


@pytest.mark.parametrize("marker", CHANGELOG_MARKERS)
def test_AIM_21_AC13_changelog_documents_each_fix_above_the_2_0_3_heading(marker):
    assert marker in _section_above_2_0_3(), (
        f"no CHANGELOG entry above the [2.0.3] heading documents: {marker}"
    )


def test_AIM_21_AC13_this_file_touches_only_loopback_and_its_own_fixture_script():
    hosts = set(re.findall(r"https?://([A-Za-z0-9.\-]+)", THIS_FILE))
    allowed = {"127.0.0.1", "localhost", "aim.example.com"}
    assert hosts <= allowed, f"non-loopback host in this test file: {hosts - allowed}"

    # Only one thing here is ever executed as a process: the interpreter
    # running the fixture script written into tmp_path.
    spawn_calls = re.findall(r"subprocess\.\w+\(|os\.system\(|os\.exec\w*\(", THIS_FILE)
    assert spawn_calls == [], f"this file spawns processes directly: {spawn_calls}"
    assert "sys.executable" in THIS_FILE


CHANGED_STRINGS = [
    "Connection failed",
    "Request timeout",
    "Network error during verification",
    "discoveryError",
    "MCP SDK not installed",
    "print_help",
]


@pytest.mark.parametrize("needle", CHANGED_STRINGS)
def test_AIM_21_AC13_no_pre_existing_test_asserts_on_a_string_this_batch_changes(
    needle,
):
    offenders = []
    for path in sorted((SDK_DIR / "tests").glob("test_*.py")):
        if path.name == Path(__file__).name:
            continue
        for lineno, line in enumerate(
            path.read_text(encoding="utf-8").splitlines(), start=1
        ):
            if needle in line and "assert" in line:
                offenders.append(f"{path.name}:{lineno}")
    assert offenders == [], (
        f"a committed test asserts on {needle!r}, which this batch changes: "
        f"{offenders}"
    )


def test_AIM_21_AC13_the_committed_protocol_assertions_are_unchanged():
    lines = (SDK_DIR / "tests" / "test_protocol_detection.py").read_text(
        encoding="utf-8"
    ).splitlines()
    assert lines[46].strip() == (
        'assert confidence >= 90, f"Expected confidence >= 90%, got {confidence}%"'
    )
    assert lines[74].strip() == (
        'assert confidence >= 90, f"Expected confidence >= 90%, got {confidence}%"'
    )
    assert lines[119].strip() == (
        "assert protocol == \"mcp\", f\"Expected default 'mcp', got '{protocol}'\""
    )
