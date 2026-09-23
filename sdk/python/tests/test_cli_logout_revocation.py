"""
`aim-sdk logout` revokes the stored pair through the backend's logout route and
claims a revocation only when the server confirmed it.

Before this, logout posted `/api/v1/auth/revoke`, a route no backend in this
repository registers, deleted the local file, and printed `[OK] Token revoked`
whatever the server answered, so the refresh token outlived the logout.
"""
import inspect
import json
import os
import sys
import time
import uuid
from types import SimpleNamespace

import jwt
import pytest
import requests

import aim_sdk.oauth as oauth_module
from aim_sdk import cli
from aim_sdk import credentials as credentials_module

AIM_URL = "http://127.0.0.1:9"
KEY = "test-only-signing-key-not-a-real-secret"


def _token(typ: str, exp_offset: int = 3600) -> str:
    return jwt.encode(
        {"user_id": "u-1", "organization_id": "o-1", "typ": typ, "iss": "agent-identity-management",
         "jti": str(uuid.uuid4()), "exp": int(time.time()) + exp_offset},
        KEY, algorithm="HS256",
    )


@pytest.fixture
def home(monkeypatch, tmp_path):
    """A fresh HOME so no test reads or writes the real user profile."""
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))
    aim_dir = tmp_path / ".aim"
    monkeypatch.setattr(credentials_module, "AIM_DIR", aim_dir)
    monkeypatch.setattr(credentials_module, "SDK_CREDENTIALS_FILE", aim_dir / "sdk_credentials.json")
    monkeypatch.setattr(credentials_module, "AGENTS_DIR", aim_dir / "agents")
    monkeypatch.setattr(credentials_module, "LEGACY_CREDENTIALS_FILE", aim_dir / "credentials.json")
    monkeypatch.setattr(oauth_module, "get_sdk_credentials_path", lambda: aim_dir / "sdk_credentials.json")
    return aim_dir


def _write_credentials(aim_dir, access, refresh):
    aim_dir.mkdir(parents=True, exist_ok=True)
    path = aim_dir / "sdk_credentials.json"
    data = {"aimUrl": AIM_URL, "refreshToken": refresh, "type": "sdk_oauth", "schemaVersion": "1.0"}
    if access is not None:
        data["accessToken"] = access
    path.write_text(json.dumps(data))
    os.chmod(path, 0o600)
    return path


class Posts:
    """Records every requests.post and answers with a scripted response or raises."""

    def __init__(self, status=200, body=None, raise_exc=None):
        self.status, self.body, self.raise_exc, self.calls = status, body, raise_exc, []

    def __call__(self, url, **kwargs):
        self.calls.append((url, kwargs))
        if self.raise_exc:
            raise self.raise_exc
        r = SimpleNamespace(status_code=self.status, text=json.dumps(self.body if self.body is not None else {}))
        r.json = lambda: (self.body if self.body is not None else {})
        r.content = r.text.encode()
        return r


def _run_logout(monkeypatch, capsys, posts):
    monkeypatch.setattr(oauth_module.requests, "post", posts)
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "logout"])
    rc = cli.main()
    return (rc or 0), capsys.readouterr().out


CONFIRMED = {"message": "Logged out successfully", "revoked": {"accessToken": True, "refreshToken": True}}


def test_logout_posts_the_logout_route_once_with_the_bearer_and_the_refresh_token(home, monkeypatch, capsys):
    access, refresh = _token("access"), _token("refresh", 7 * 86400)
    _write_credentials(home, access, refresh)
    posts = Posts(200, CONFIRMED)
    _run_logout(monkeypatch, capsys, posts)
    assert len(posts.calls) == 1
    url, kwargs = posts.calls[0]
    assert url == AIM_URL + "/api/v1/auth/logout"
    assert kwargs["headers"]["Authorization"] == "Bearer " + access
    assert kwargs["json"] == {"refreshToken": refresh}
    assert kwargs.get("timeout")
    assert "/api/v1/auth/revoke" not in inspect.getsource(oauth_module)


def test_logout_confirmed_by_the_server_exits_zero_and_deletes_the_file(home, monkeypatch, capsys):
    path = _write_credentials(home, _token("access"), _token("refresh", 7 * 86400))
    rc, out = _run_logout(monkeypatch, capsys, Posts(200, CONFIRMED))
    assert rc == 0
    assert "revoked" in out.lower()
    assert not path.exists()


@pytest.mark.parametrize("status,body", [
    (404, {"error": "Cannot POST /api/v1/auth/logout"}),
    (429, {"error": "Rate limit exceeded. Please try again later."}),
    (500, {"error": "server_error"}),
    (200, {"message": "Logged out successfully"}),
    (200, {"message": "Logged out successfully", "revoked": {"accessToken": True, "refreshToken": False}}),
])
def test_logout_unconfirmed_names_the_reason_the_expiry_and_the_deletion_and_exits_one(home, monkeypatch, capsys, status, body):
    path = _write_credentials(home, _token("access"), _token("refresh", 7 * 86400))
    rc, out = _run_logout(monkeypatch, capsys, Posts(status, body))
    assert rc == 1
    assert "[OK] Token revoked" not in out
    assert (str(status) in out) or ("did not confirm" in out.lower())
    assert "utc" in out.lower(), "the refresh token's expiry is named"
    assert "deleted" in out.lower()
    assert not path.exists()


def test_logout_unreachable_server_is_named_and_exits_one(home, monkeypatch, capsys):
    path = _write_credentials(home, _token("access"), _token("refresh", 7 * 86400))
    rc, out = _run_logout(monkeypatch, capsys, Posts(raise_exc=requests.ConnectionError("refused")))
    assert rc == 1
    assert "unreachable" in out.lower() or "could not reach" in out.lower()
    assert not path.exists()


@pytest.mark.parametrize("posts", [
    Posts(200, CONFIRMED),
    Posts(500, {"error": "server_error"}),
    Posts(raise_exc=requests.ConnectionError("refused")),
])
def test_logout_never_prints_a_token(home, monkeypatch, capsys, posts):
    access, refresh = _token("access"), _token("refresh", 7 * 86400)
    _write_credentials(home, access, refresh)
    _, out = _run_logout(monkeypatch, capsys, posts)
    assert access not in out and refresh not in out
    assert access[-20:] not in out and refresh[-20:] not in out


def test_logout_without_credentials_posts_nothing_and_exits_zero(home, monkeypatch, capsys):
    posts = Posts(200, CONFIRMED)
    rc, out = _run_logout(monkeypatch, capsys, posts)
    assert rc == 0
    assert posts.calls == []
    assert "no credentials" in out.lower()


def test_logout_with_only_a_refresh_token_posts_without_a_bearer(home, monkeypatch, capsys):
    refresh = _token("refresh", 7 * 86400)
    _write_credentials(home, None, refresh)
    posts = Posts(200, CONFIRMED)
    _run_logout(monkeypatch, capsys, posts)
    assert len(posts.calls) == 1
    _, kwargs = posts.calls[0]
    assert "Authorization" not in kwargs.get("headers", {})
    assert kwargs["json"] == {"refreshToken": refresh}
