"""
#409: `aim-sdk status` on a home directory holding only a legacy
`~/.aim/credentials.json` (snake_case keys, a refresh token and no access
token) printed Server/User/Credentials and then nothing -- no verdict, exit 0,
`User: Unknown` -- and when adopting the file into the new location failed it
still printed "[OK] ... migrated" and cited a credentials file that did not
exist.

Acceptance: an explicit verdict always; never a nonexistent credentials path;
exit code matches the verdict; `--json` stays one parseable object.
"""
import base64
import contextlib
import io
import json
import time
from types import SimpleNamespace

import pytest

from aim_sdk import cli
from aim_sdk import credentials as credentials_module

VERDICTS = ("Authenticated", "Not authenticated")


@pytest.fixture
def home(monkeypatch, tmp_path):
    monkeypatch.setenv("HOME", str(tmp_path))
    aim_dir = tmp_path / ".aim"
    aim_dir.mkdir()
    (aim_dir / "secure").mkdir()
    (aim_dir / "agents").mkdir()
    monkeypatch.setattr(credentials_module, "AIM_DIR", aim_dir)
    monkeypatch.setattr(credentials_module, "SDK_CREDENTIALS_FILE", aim_dir / "sdk_credentials.json")
    monkeypatch.setattr(credentials_module, "LEGACY_CREDENTIALS_FILE", aim_dir / "credentials.json")
    monkeypatch.setattr(credentials_module, "AGENTS_DIR", aim_dir / "agents")
    monkeypatch.setattr(credentials_module, "_find_sdk_package_credentials", lambda: None)
    return aim_dir


def _jwt(exp):
    def seg(d):
        return base64.urlsafe_b64encode(json.dumps(d).encode()).decode().rstrip("=")
    return f"{seg({'alg': 'HS256'})}.{seg({'exp': exp})}.c2ln"


def _status(json_output=False):
    out, err = io.StringIO(), io.StringIO()
    with contextlib.redirect_stdout(out), contextlib.redirect_stderr(err):
        rc = cli.status(SimpleNamespace(json=json_output))
    return rc, out.getvalue(), err.getvalue()


def _write_legacy(aim_dir):
    (aim_dir / "credentials.json").write_text(json.dumps({
        "aim_url": "http://localhost:8080",
        "refresh_token": "legacy-opaque",
        "user_email": "dev@example.com",
    }))


def test_legacy_refresh_token_only_prints_a_verdict_and_reads_snake_case(home):
    _write_legacy(home)
    rc, out, _ = _status()
    assert rc == 0
    assert "Authenticated with a stored refresh token" in out
    assert "User: dev@example.com" in out
    assert "Server: http://localhost:8080" in out


def test_failed_adoption_cites_the_file_actually_read_and_no_false_ok(home, monkeypatch):
    _write_legacy(home)
    monkeypatch.setattr(credentials_module, "save_sdk_credentials", lambda c: False)
    rc, out, err = _status()
    assert rc == 0
    assert not (home / "sdk_credentials.json").exists()
    assert f"Credentials: {home / 'credentials.json'}" in out
    assert "sdk_credentials.json" not in out
    assert "[OK]" not in out + err
    assert "were not migrated" in err


@pytest.mark.parametrize("creds, expected_rc, phrase", [
    ({"refreshToken": "r", "accessToken": _jwt(time.time() + 3600)}, 0, "The access token is valid"),
    ({"refreshToken": "r", "accessToken": _jwt(time.time() - 3600)}, 0, "has expired"),
    ({"refreshToken": "r", "accessToken": "not-a-jwt"}, 0, "expiry could not be read"),
    ({"refreshToken": "r"}, 0, "stored refresh token"),
])
def test_every_token_state_prints_a_verdict_and_a_matching_exit_code(home, creds, expected_rc, phrase):
    (home / "sdk_credentials.json").write_text(json.dumps({**creds, "aimUrl": "https://aim.example.com"}))
    rc, out, _ = _status()
    assert rc == expected_rc
    assert phrase in out
    assert any(v in out for v in VERDICTS)


def test_no_usable_token_is_not_authenticated_with_exit_1():
    creds = {"accessToken": _jwt(time.time() - 3600)}
    authenticated, sentence = cli._status_verdict(creds, cli._token_state(creds["accessToken"]))
    assert authenticated is False
    assert sentence.startswith("Not authenticated")


def test_missing_server_names_logins_default(home):
    (home / "sdk_credentials.json").write_text(json.dumps({"refreshToken": "r"}))
    _, out, _ = _status()
    assert f"login uses {cli.DEFAULT_AIM_URL} by default" in out


def test_json_is_one_object_and_agrees_with_the_verdict(home):
    _write_legacy(home)
    rc, out, _ = _status(json_output=True)
    payload = json.loads(out)  # raises if a notice leaked onto stdout
    assert payload["authenticated"] is True and rc == 0
    assert payload["user"] == "dev@example.com"
    path = payload["credentialsPath"]
    assert path is not None and (home / "sdk_credentials.json").exists() and path.endswith("sdk_credentials.json")


def test_json_with_no_stored_credentials_cites_no_file(home):
    rc, out, _ = _status(json_output=True)
    payload = json.loads(out)
    assert payload["authenticated"] is False and rc == 1
    assert payload["tokenState"] == "absent"
    assert not (home / "sdk_credentials.json").exists()
    assert payload["credentialsPath"] is None
