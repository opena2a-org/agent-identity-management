"""
AIM-06 -- one enforcement deadline over the whole verification call (Python).

AC1  `enforcement_timeout` constructor option: resolution order is constructor
     argument, then AIM_ENFORCEMENT_TIMEOUT_MS (integer milliseconds), then the
     5.0s module default; an invalid environment value warns once per process
     and falls back to the default.
AC2  The deadline bounds the WHOLE enforcement call -- the verifications POST,
     the bounded 429 retry loop, and every approval-wait poll -- measured
     against local listeners that accept and never answer. Expiry raises
     VerificationUnavailableError (Outcome.UNKNOWN, UnknownSource.TRANSPORT)
     and never touches the cached enforcement mode.
AC4  Nothing else moved: `timeout` default 30, `timeout_seconds` default 300,
     the 300s mode TTL; no seconds-spelled environment variable exists, and an
     AIM_ENFORCEMENT_TIMEOUT_MS of 2000 is a 2s deadline, never 2000s.

Every wall time below is measured by time.monotonic() around the single call.
"""

import inspect
import socket
import subprocess
import threading
import time
import uuid
from pathlib import Path

import pytest
from nacl.encoding import Base64Encoder
from nacl.signing import SigningKey

from aim_sdk import client as client_module
from aim_sdk.client import (
    AIMClient,
    DEFAULT_ENFORCEMENT_TIMEOUT_SECONDS,
    ENFORCEMENT_TIMEOUT_ENV_VAR,
    reset_enforcement_timeout_warning_state,
)
from aim_sdk.decision import (
    DEFAULT_MODE_TTL_SECONDS,
    EnforcementMode,
    ModeSource,
    Outcome,
    UnknownSource,
    get_mode_cache,
    resolve_mode,
)
from aim_sdk.enforcement import evaluate
from aim_sdk.exceptions import VerificationUnavailableError


# --------------------------------------------------------------------------- #
# Fixtures: local listeners. The stalled listener is the contract's red/green
# fixture -- socket.listen on 127.0.0.1:0, an acceptor thread, no reply. The
# scripted server answers per-request from a handler; a handler returning None
# stalls that request (accepted, held open, never answered).
# --------------------------------------------------------------------------- #
class LocalServer:
    def __init__(self, handler=None):
        self._sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
        self._sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
        self._sock.bind(("127.0.0.1", 0))
        self._sock.listen(16)
        self.url = f"http://127.0.0.1:{self._sock.getsockname()[1]}"
        self.connections = 0
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
            self.connections += 1
            self._open.append(conn)
            if self._handler is not None:
                threading.Thread(target=self._serve, args=(conn,), daemon=True).start()
            # handler None: the pure stalled listener -- hold the socket open,
            # never read, never answer.

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

            reply = self._handler(method, path)
            if reply is None:
                # Stall: accepted, read, never answered.
                self._closing.wait()
                return
            status, extra_headers, payload = reply
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

    def start(handler=None):
        server = LocalServer(handler)
        servers.append(server)
        return server

    yield start
    for server in servers:
        server.close()


@pytest.fixture(autouse=True)
def clean_state(monkeypatch):
    monkeypatch.delenv(ENFORCEMENT_TIMEOUT_ENV_VAR, raising=False)
    reset_enforcement_timeout_warning_state()
    get_mode_cache().clear()
    yield
    get_mode_cache().clear()
    reset_enforcement_timeout_warning_state()


def fresh_agent_id():
    return str(uuid.uuid4())


def api_key_client(url, agent_id=None, **kwargs):
    return AIMClient(
        agent_id=agent_id or fresh_agent_id(),
        api_key="test-api-key",
        aim_url=url,
        **kwargs,
    )


def signing_client(url, **kwargs):
    signing_key = SigningKey.generate()
    return AIMClient(
        agent_id=fresh_agent_id(),
        public_key=signing_key.verify_key.encode(encoder=Base64Encoder).decode("ascii"),
        private_key=signing_key.encode(encoder=Base64Encoder).decode("ascii"),
        aim_url=url,
        sdk_token_id="test-sdk-token",
        **kwargs,
    )


# --------------------------------------------------------------------------- #
# AC1 -- option, environment variable, default; warn-once on an invalid value.
# --------------------------------------------------------------------------- #
def test_AIM_06_AC1_constructor_argument_wins_over_environment_and_default(monkeypatch):
    monkeypatch.setenv(ENFORCEMENT_TIMEOUT_ENV_VAR, "2000")
    client = api_key_client("http://127.0.0.1:9", enforcement_timeout=1.5)
    assert client.enforcement_timeout == 1.5


def test_AIM_06_AC1_environment_variable_wins_over_the_default(monkeypatch):
    monkeypatch.setenv(ENFORCEMENT_TIMEOUT_ENV_VAR, "2500")
    client = api_key_client("http://127.0.0.1:9")
    assert client.enforcement_timeout == 2.5


def test_AIM_06_AC1_default_is_the_five_second_module_constant():
    client = api_key_client("http://127.0.0.1:9")
    assert client.enforcement_timeout == DEFAULT_ENFORCEMENT_TIMEOUT_SECONDS
    assert DEFAULT_ENFORCEMENT_TIMEOUT_SECONDS == 5.0


@pytest.mark.parametrize("raw", ["not-a-number", "2.5", "-100", "0"])
def test_AIM_06_AC1_invalid_environment_value_falls_back_to_the_default(monkeypatch, raw):
    monkeypatch.setenv(ENFORCEMENT_TIMEOUT_ENV_VAR, raw)
    client = api_key_client("http://127.0.0.1:9")
    assert client.enforcement_timeout == 5.0


def test_AIM_06_AC1_invalid_environment_value_warns_exactly_once_across_two_constructions(monkeypatch):
    warnings_seen = []
    monkeypatch.setattr(
        client_module.console, "warning", lambda message: warnings_seen.append(message)
    )
    monkeypatch.setenv(ENFORCEMENT_TIMEOUT_ENV_VAR, "banana")

    first = api_key_client("http://127.0.0.1:9")
    second = api_key_client("http://127.0.0.1:9")

    assert first.enforcement_timeout == 5.0
    assert second.enforcement_timeout == 5.0
    matching = [message for message in warnings_seen if ENFORCEMENT_TIMEOUT_ENV_VAR in message]
    assert len(matching) == 1, matching


# --------------------------------------------------------------------------- #
# AC2 -- the deadline bounds the whole enforcement call.
# --------------------------------------------------------------------------- #
def test_AIM_06_AC2_default_deadline_bounds_a_stalled_verification_post(server_factory):
    # (a) Defaults, no enforcement option anywhere: the 5.0s module default
    # applies. Base reading at origin/main 5405500a (contract intake, same
    # fixture): raised after 30.01s wall time with 1 accepted connection.
    server = server_factory(handler=None)
    client = api_key_client(server.url)

    start = time.monotonic()
    with pytest.raises(VerificationUnavailableError) as excinfo:
        client.verify_capability("db:read")
    elapsed = time.monotonic() - start

    assert elapsed < 6.0, f"took {elapsed:.2f}s"
    assert elapsed >= 4.5, f"raised too early ({elapsed:.2f}s) to have been the 5.0s deadline"
    assert server.connections == 1
    decision = excinfo.value.decision
    assert decision.outcome is Outcome.UNKNOWN
    assert decision.unknown_source is UnknownSource.TRANSPORT


def test_AIM_06_AC2_environment_deadline_bounds_a_stalled_verification_post(server_factory, monkeypatch):
    # (b) AIM_ENFORCEMENT_TIMEOUT_MS=2000 in the environment, no constructor
    # argument, `timeout` left at 30. Base reading at origin/main 5405500a
    # (contract intake, same fixture): 30.01s, 1 accepted connection.
    monkeypatch.setenv(ENFORCEMENT_TIMEOUT_ENV_VAR, "2000")
    server = server_factory(handler=None)
    client = api_key_client(server.url)

    start = time.monotonic()
    with pytest.raises(VerificationUnavailableError) as excinfo:
        client.verify_capability("db:read")
    elapsed = time.monotonic() - start

    assert elapsed < 3.0, f"took {elapsed:.2f}s"
    assert elapsed >= 1.5, f"raised too early ({elapsed:.2f}s) to have been the 2000ms deadline"
    assert server.connections == 1
    assert excinfo.value.decision.unknown_source is UnknownSource.TRANSPORT


def test_AIM_06_AC2_deadline_bounds_the_approval_wait_polls(server_factory):
    # (c) The POST answers pending; the following GET is never answered. The
    # deadline (2.0s), not the approval wait's own timeout_seconds (6s), must
    # end the call. Base reading (origin/main 5405500a, host run): 7.02s --
    # the call ran to the approval wait's own timeout_seconds. Only the 3.0s
    # bound is asserted.
    def handler(method, path):
        if method == "POST":
            return 200, {}, '{"id": "9d2f66d1-8f6a-4f0e-9f30-3f4d0f6b6a01", "status": "pending"}'
        return None  # every poll GET stalls

    server = server_factory(handler)
    client = signing_client(server.url, timeout=1, enforcement_timeout=2.0)

    start = time.monotonic()
    with pytest.raises(VerificationUnavailableError) as excinfo:
        client.verify_capability("db:read", timeout_seconds=6)
    elapsed = time.monotonic() - start

    assert elapsed < 3.0, f"took {elapsed:.2f}s"
    decision = excinfo.value.decision
    assert decision.outcome is Outcome.UNKNOWN
    assert decision.unknown_source is UnknownSource.TRANSPORT


def test_AIM_06_AC2_a_429_delay_that_would_cross_the_deadline_is_not_slept(server_factory):
    # (d) Every POST answers 429 with Retry-After: 4. Base reading
    # (origin/main 5405500a, host run): 8.02s -- two slept retries of 4s each.
    # Only the 3.0s bound is asserted.
    def handler(method, path):
        return 429, {"Retry-After": "4"}, "{}"

    server = server_factory(handler)
    client = api_key_client(server.url, enforcement_timeout=2.0)

    start = time.monotonic()
    with pytest.raises(VerificationUnavailableError):
        client.verify_capability("db:read")
    elapsed = time.monotonic() - start

    assert elapsed < 3.0, f"took {elapsed:.2f}s"
    # One request, no slept retry: at base this is three connections and ~8s.
    assert server.connections == 1


def test_AIM_06_AC2_a_timed_out_call_neither_refreshes_nor_evicts_the_cached_mode(server_factory):
    # The 300s mode TTL lives at sdk/python/aim_sdk/decision.py:49
    # (DEFAULT_MODE_TTL_SECONDS = 300, read by the module-level ModeCache).
    agent_id = fresh_agent_id()

    def approve(method, path):
        return 200, {}, (
            '{"id": "5cbe0e3e-32b1-45f8-9f10-5a3d1a2b3c4d", "status": "approved",'
            ' "enforcementMode": "strict", "approvedBy": "tester"}'
        )

    priming_server = server_factory(approve)
    priming_client = api_key_client(priming_server.url, agent_id=agent_id)
    result = priming_client.verify_capability("db:read")
    assert result["mode"] == "strict"
    assert get_mode_cache().get(agent_id) is EnforcementMode.STRICT

    stalled = server_factory(handler=None)
    timed_out_client = api_key_client(stalled.url, agent_id=agent_id, enforcement_timeout=0.5)
    with pytest.raises(VerificationUnavailableError) as excinfo:
        timed_out_client.verify_capability("db:read")

    # The timed-out call itself read the cached mode and did not disturb it.
    assert excinfo.value.decision.mode is EnforcementMode.STRICT
    assert excinfo.value.decision.mode_source is ModeSource.CACHE
    mode, source = resolve_mode(EnforcementMode.UNKNOWN, agent_id)
    assert mode is EnforcementMode.STRICT
    assert source is ModeSource.CACHE

    # The following evaluate reads the same cached mode and source.
    following_client = api_key_client(stalled.url, agent_id=agent_id, enforcement_timeout=0.5)
    with pytest.raises(VerificationUnavailableError) as following:
        following_client.verify_capability("db:read")
    verdict = evaluate(following.value.decision)
    assert verdict.effective_mode is EnforcementMode.STRICT
    assert verdict.mode_source is ModeSource.CACHE


# --------------------------------------------------------------------------- #
# AC4 -- what must not have moved.
# --------------------------------------------------------------------------- #
def test_AIM_06_AC4_general_timeout_default_is_still_30():
    assert inspect.signature(AIMClient.__init__).parameters["timeout"].default == 30


def test_AIM_06_AC4_approval_wait_timeout_seconds_default_is_still_300():
    assert (
        inspect.signature(AIMClient._decide_capability).parameters["timeout_seconds"].default
        == 300
    )
    assert (
        inspect.signature(AIMClient.verify_capability).parameters["timeout_seconds"].default
        == 300
    )


def test_AIM_06_AC4_mode_ttl_is_still_300_seconds():
    assert DEFAULT_MODE_TTL_SECONDS == 300
    assert get_mode_cache().ttl_seconds == 300


def test_AIM_06_AC4_no_seconds_spelled_environment_variable_exists_in_either_sdk():
    # The contract's negative grep: only the milliseconds-suffixed environment
    # variable may exist under sdk/ — no seconds-suffixed variant (short or
    # long form) and no bare assignment of the un-suffixed name. The patterns
    # are built by concatenation so this file cannot match its own scan; the
    # short seconds suffix is a prefix of the long one, so one pattern covers
    # both. Scans git-tracked files so local node_modules or build output
    # cannot pollute the result.
    prefix = "AIM_ENFORCEMENT_" + "TIMEOUT"
    forbidden = [(prefix + "_S").encode(), (prefix + "=").encode()]

    sdk_dir = Path(__file__).resolve().parents[2]
    assert sdk_dir.name == "sdk"
    tracked = subprocess.run(
        ["git", "ls-files", "-z", "--", "sdk"],
        cwd=sdk_dir.parent,
        capture_output=True,
        check=True,
    ).stdout.split(b"\0")
    offenders = []
    for name in tracked:
        if not name:
            continue
        content = (sdk_dir.parent / name.decode()).read_bytes()
        for pattern in forbidden:
            if pattern in content:
                offenders.append((name.decode(), pattern.decode()))
    assert offenders == []


def test_AIM_06_AC4_env_value_2000_is_a_two_second_deadline_not_2000_seconds(monkeypatch):
    monkeypatch.setenv(ENFORCEMENT_TIMEOUT_ENV_VAR, "2000")
    client = api_key_client("http://127.0.0.1:9")
    assert client.enforcement_timeout == 2.0
