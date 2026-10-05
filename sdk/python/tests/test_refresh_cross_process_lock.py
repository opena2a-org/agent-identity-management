"""
Processes that share ~/.aim/sdk_credentials.json refresh it once.

The AIM server's refresh tokens are single-use: a token another process already
rotated is refused, and presenting it is reuse that ends the sign-in for every
process holding it. So before refreshing, the SDK takes a cross-process lock on
the credentials file and re-reads the file under that lock. A process that finds
a pair another process already obtained uses that pair instead of presenting its
own stale copy, and a token the server refused is not presented again.

NOT marked `integration` -- `pytest.ini` deselects that marker by default.
"""

import json
import os
import subprocess
import sys
import textwrap
import threading
import time
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

import jwt
import pytest

import aim_sdk.credentials as credentials_module
import aim_sdk.oauth as oauth_module
from aim_sdk.credentials import (
    CredentialOrigin,
    SDKCredentialsLockTimeout,
    detect_credential_origin,
    save_sdk_credentials,
    sdk_credentials_lock,
)
from aim_sdk.oauth import OAuthTokenManager

SDK_ROOT = Path(__file__).resolve().parent.parent
KEY = "refresh-lock-test-signing-key-0123456789"
REFRESH_PATH = "/api/v1/auth/refresh"


def _token(typ, ttl=900):
    return jwt.encode(
        {"user_id": "u-1", "typ": typ, "iss": "agent-identity-management",
         "jti": str(uuid.uuid4()), "exp": int(time.time()) + ttl},
        KEY, algorithm="HS256",
    )


def _login_creds(url, refresh):
    """The shape `aim-sdk login` saves."""
    return {"aimUrl": url, "refreshToken": refresh, "accessToken": _token("access", -60),
            "userId": "u-1", "userEmail": "user@example.test", "organizationId": "o-1"}


def _download_creds(url, refresh):
    """The shape a dashboard SDK download carries."""
    return {"aimUrl": url, "refreshToken": refresh, "sdkTokenId": str(uuid.uuid4()),
            "userId": "u-1", "userEmail": "user@example.test"}


ORIGINS = [
    (CredentialOrigin.LOGIN, _login_creds),
    (CredentialOrigin.SDK_DOWNLOAD, _download_creds),
]


def _write(path, data):
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps({**data, "type": "sdk_oauth", "schemaVersion": "1.0"}))
    os.chmod(path, 0o600)


class StrictRefreshServer:
    """A refresh endpoint with single-use tokens, like the AIM server.

    The current refresh token is accepted once and rotated. Any other token is
    refused with 401 and, as on the server, ends the sign-in (no token is
    accepted afterwards). Every POST is recorded.
    """

    def __init__(self, refresh_token, delay=0.0):
        self.current = refresh_token
        self.delay = delay
        self.posts = []
        self._lock = threading.Lock()
        outer = self

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                length = int(self.headers.get("Content-Length") or 0)
                body = json.loads(self.rfile.read(length) or b"{}")
                status, payload = outer._answer(self.path, body)
                data = json.dumps(payload).encode()
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            def log_message(self, *args):
                pass

        self._httpd = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self._httpd.server_address[1]}"
        threading.Thread(target=self._httpd.serve_forever, daemon=True).start()

    def _answer(self, path, body):
        with self._lock:
            self.posts.append((path, body))
            accepted = (path == REFRESH_PATH and self.current is not None
                        and body.get("refreshToken") == self.current)
            if not accepted:
                if path == REFRESH_PATH:
                    self.current = None
                return 401, {"error": "Token has been revoked or is invalid"}
            new_refresh = _token("refresh", 86400)
            self.current = new_refresh
        time.sleep(self.delay)
        return 200, {"accessToken": _token("access"), "refreshToken": new_refresh}

    def refreshes(self):
        return [body.get("refreshToken") for path, body in self.posts if path == REFRESH_PATH]

    def other_posts(self):
        return [path for path, _ in self.posts if path != REFRESH_PATH]

    def close(self):
        self._httpd.shutdown()
        self._httpd.server_close()


@pytest.fixture
def server_factory():
    servers = []

    def make(refresh_token, delay=0.0):
        server = StrictRefreshServer(refresh_token, delay)
        servers.append(server)
        return server

    yield make
    for server in servers:
        server.close()


@pytest.fixture
def home(monkeypatch, tmp_path):
    """A fresh ~/.aim so no test reads or writes the real user profile."""
    aim_dir = tmp_path / ".aim"
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))
    monkeypatch.setattr(credentials_module, "AIM_DIR", aim_dir)
    monkeypatch.setattr(credentials_module, "SDK_CREDENTIALS_FILE", aim_dir / "sdk_credentials.json")
    monkeypatch.setattr(credentials_module, "AGENTS_DIR", aim_dir / "agents")
    monkeypatch.setattr(credentials_module, "LEGACY_CREDENTIALS_FILE", aim_dir / "credentials.json")
    monkeypatch.setattr(oauth_module, "get_sdk_credentials_path", lambda: aim_dir / "sdk_credentials.json")
    return aim_dir


# --- two real processes on one file ---------------------------------------------------

CHILD = textwrap.dedent(
    """
    import json, sys, time
    from pathlib import Path
    from aim_sdk.oauth import OAuthTokenManager

    go, ready = Path(sys.argv[1]), Path(sys.argv[2])
    manager = OAuthTokenManager()
    ready.write_text("ready")
    deadline = time.monotonic() + 30
    while not go.exists() and time.monotonic() < deadline:
        time.sleep(0.01)
    print("RESULT " + json.dumps({"token": manager.get_access_token()}), flush=True)
    """
)


def _child_env(home_dir):
    env = {k: v for k, v in os.environ.items() if "proxy" not in k.lower()}
    env.update(HOME=str(home_dir), USERPROFILE=str(home_dir), PYTHONPATH=str(SDK_ROOT),
               NO_PROXY="127.0.0.1,localhost")
    return env


@pytest.mark.parametrize("origin, make_creds", ORIGINS)
def test_two_processes_on_one_file_make_one_refresh_and_the_second_adopts_the_pair(
    tmp_path, server_factory, origin, make_creds
):
    r0 = _token("refresh", 86400)
    # The delay keeps the first refresh in flight while the second process asks,
    # so a re-read without a lock would still read and present r0.
    server = server_factory(r0, delay=1.0)
    home_dir = tmp_path / "home"
    creds_file = home_dir / ".aim" / "sdk_credentials.json"
    _write(creds_file, make_creds(server.url, r0))

    go = tmp_path / "go"
    procs = []
    for n in range(2):
        ready = tmp_path / f"ready-{n}"
        proc = subprocess.Popen(
            [sys.executable, "-c", CHILD, str(go), str(ready)],
            env=_child_env(home_dir), cwd=str(tmp_path),
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True,
        )
        procs.append((proc, ready))

    deadline = time.monotonic() + 30
    while not all(ready.exists() for _, ready in procs) and time.monotonic() < deadline:
        time.sleep(0.01)
    go.write_text("go")

    tokens = []
    for proc, _ in procs:
        out, err = proc.communicate(timeout=60)
        assert proc.returncode == 0, err
        line = next(l for l in out.splitlines() if l.startswith("RESULT "))
        tokens.append(json.loads(line[len("RESULT "):])["token"])

    assert server.refreshes() == [r0], server.posts
    assert server.other_posts() == []
    assert tokens[0] and tokens[0] == tokens[1]

    stored = json.loads(creds_file.read_text())
    assert stored["refreshToken"] == server.current
    # The shared access token must not change which recovery the banner gives.
    assert detect_credential_origin(stored) == origin


# --- re-read before refresh --------------------------------------------------------------


@pytest.mark.parametrize("origin, make_creds", ORIGINS)
def test_a_refresh_after_another_process_rotated_presents_the_new_token(
    home, server_factory, origin, make_creds
):
    r0, r1 = _token("refresh", 86400), _token("refresh", 86400)
    server = server_factory(r1)
    creds_file = home / "sdk_credentials.json"
    _write(creds_file, make_creds(server.url, r0))
    manager = OAuthTokenManager()

    # Another process (an older SDK, so no shared access token) rotated r0 to r1.
    _write(creds_file, make_creds(server.url, r1))

    assert manager.get_access_token(suppress_errors=True)
    assert server.refreshes() == [r1]
    assert server.other_posts() == []
    assert json.loads(creds_file.read_text())["refreshToken"] == server.current


def test_a_second_manager_uses_the_pair_the_first_obtained_without_a_refresh(home, server_factory):
    r0 = _token("refresh", 86400)
    server = server_factory(r0)
    _write(home / "sdk_credentials.json", _download_creds(server.url, r0))
    first, second = OAuthTokenManager(), OAuthTokenManager()

    token = first.get_access_token()
    assert token
    assert second.get_access_token() == token
    assert server.refreshes() == [r0]


def test_a_shared_access_token_issued_with_another_refresh_token_is_not_used(home, server_factory):
    r0 = _token("refresh", 86400)
    server = server_factory(r0)
    creds_file = home / "sdk_credentials.json"
    _write(creds_file, _download_creds(server.url, r0))
    assert OAuthTokenManager().get_access_token()
    stored = json.loads(creds_file.read_text())

    # A writer that copies the whole file but swaps in another refresh token.
    r1 = server.current = _token("refresh", 86400)
    stored["refreshToken"] = r1
    _write(creds_file, stored)

    assert OAuthTokenManager().get_access_token()
    assert server.refreshes() == [r0, r1]


# --- the lock is mandatory -----------------------------------------------------------------


def test_no_refresh_is_sent_while_another_holder_has_the_lock(home, server_factory, monkeypatch):
    r0 = _token("refresh", 86400)
    server = server_factory(r0)
    _write(home / "sdk_credentials.json", _download_creds(server.url, r0))
    manager = OAuthTokenManager()
    monkeypatch.setattr(credentials_module, "SDK_CREDENTIALS_LOCK_TIMEOUT", 0.2)

    held, release = threading.Event(), threading.Event()

    def holder():
        with sdk_credentials_lock():
            held.set()
            release.wait(10)

    thread = threading.Thread(target=holder)
    thread.start()
    try:
        assert held.wait(10)
        assert manager.get_access_token(suppress_errors=True) is None
        assert server.posts == []
    finally:
        release.set()
        thread.join(10)

    assert manager.get_access_token(suppress_errors=True)
    assert server.refreshes() == [r0]


def test_the_lock_times_out_rather_than_waiting_forever(home):
    held, release = threading.Event(), threading.Event()

    def holder():
        with sdk_credentials_lock():
            held.set()
            release.wait(10)

    thread = threading.Thread(target=holder)
    thread.start()
    try:
        assert held.wait(10)
        with pytest.raises(SDKCredentialsLockTimeout):
            with sdk_credentials_lock(timeout=0.2):
                pass
    finally:
        release.set()
        thread.join(10)


def test_the_lock_is_reentrant_so_a_save_inside_a_refresh_does_not_deadlock(home):
    with sdk_credentials_lock(timeout=1):
        with sdk_credentials_lock(timeout=1):
            assert save_sdk_credentials({"aimUrl": "https://aim.example.test", "refreshToken": "r"})
    assert json.loads((home / "sdk_credentials.json").read_text())["refreshToken"] == "r"


def test_saved_credentials_and_the_lock_file_are_private_and_no_temp_file_is_left(home):
    assert save_sdk_credentials({"aimUrl": "https://aim.example.test", "refreshToken": "r"})
    assert sorted(p.name for p in home.iterdir()) == ["sdk_credentials.json", "sdk_credentials.json.lock"]
    if os.name == "posix":
        for name in ("sdk_credentials.json", "sdk_credentials.json.lock"):
            assert (home / name).stat().st_mode & 0o777 == 0o600


# --- a refused token is not presented again --------------------------------------------------


@pytest.mark.parametrize("origin, make_creds", ORIGINS)
def test_a_refused_refresh_token_is_not_presented_again_until_the_file_changes(
    home, server_factory, origin, make_creds
):
    refused = _token("refresh", 86400)
    server = server_factory(None)
    creds_file = home / "sdk_credentials.json"
    _write(creds_file, make_creds(server.url, refused))
    manager = OAuthTokenManager()

    assert manager.get_access_token(suppress_errors=True) is None
    assert server.refreshes() == [refused]
    posts_after_refusal = len(server.posts)

    assert manager.get_access_token(suppress_errors=True) is None
    assert len(server.posts) == posts_after_refusal

    # A new sign-in written to the file is picked up and presented.
    fresh = server.current = _token("refresh", 86400)
    _write(creds_file, make_creds(server.url, fresh))
    assert manager.get_access_token(suppress_errors=True)
    assert server.refreshes()[-1] == fresh
