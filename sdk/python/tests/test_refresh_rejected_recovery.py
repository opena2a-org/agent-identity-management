"""
A rejected SDK refresh token gets the recovery that fits where the credential came from.

`aim-sdk login` writes a credential that signing in again replaces. A dashboard SDK
download bundles one that a fresh download replaces. The refresh-rejected banner used
to give every user the download steps, led by `rm`, which is the wrong fix for a
login credential. It also pointed at "Settings -> SDK Downloads", a dashboard path
that no longer exists (the page is Developers, then SDK & docs).

A downloaded credential also ends at the server's maximum age for a download
(SDK_TOKEN_FAMILY_MAX_AGE), however often it is refreshed. The server answers that
refusal as it answers an expired token, so the download banner names the maximum age
among the causes. A refused token is presented to the refresh route only: the recovery
route mints only for a sign-in's access token, which the SDK does not hold.

NOT marked `integration` -- `pytest.ini` deselects that marker by default.
"""

import ast
import os
import re
import shlex
import sys

import pytest

import aim_sdk.cli as cli_module
from aim_sdk.credentials import (
    CredentialOrigin,
    _LOGIN_ONLY_KEYS,
    detect_credential_origin,
    login_recovery_command,
    print_token_expired_error,
    refresh_rejected_fix,
)
from aim_sdk.oauth import OAuthTokenManager
from aim_sdk.security_logging import AuthnEventType

SDK_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
REPO_ROOT = os.path.dirname(os.path.dirname(SDK_ROOT))
AIM_URL = "https://aim.example.test"

# What `aim-sdk login` saves (aim_sdk/cli.py login()), after save_sdk_credentials has
# stamped schemaVersion and type and one token rotation has written sdkTokenId.
LOGIN_CREDS = {
    "aimUrl": AIM_URL,
    "refreshToken": "refresh",
    "accessToken": "access",
    "userId": "user",
    "userEmail": "user@example.test",
    "organizationId": "org",
    "schemaVersion": "1.0",
    "type": "sdk_oauth",
    "sdkTokenId": "rotated-jti",
}

# The backend's SDKCredentials JSON (sdk_handler.go), as a dashboard download writes it.
DOWNLOAD_CREDS = {
    "schemaVersion": "1.0",
    "type": "sdk_oauth",
    "aimUrl": AIM_URL,
    "refreshToken": "refresh",
    "sdkTokenId": "token-id",
    "userId": "user",
    "userEmail": "user@example.test",
    "email": "user@example.test",
}

LEGACY_DOWNLOAD_CREDS = {"aim_url": AIM_URL, "refresh_token": "refresh", "sdk_token_id": "token-id"}


def _banner(capsys, credentials):
    print_token_expired_error(AIM_URL, credentials)
    return capsys.readouterr().out


def _cited_sdk_commands(text):
    return [m.strip() for m in re.findall(r"aim-sdk ([a-z][^\n`]*)", text)]


# --- classification ---------------------------------------------------------------


@pytest.mark.parametrize(
    "creds",
    [
        LOGIN_CREDS,
        {**LOGIN_CREDS, "organizationId": None},
        {"aimUrl": AIM_URL, "refreshToken": "refresh", "accessToken": "access"},
    ],
)
def test_login_credentials_are_detected_as_login(creds):
    assert detect_credential_origin(creds) == CredentialOrigin.LOGIN


@pytest.mark.parametrize("creds", [DOWNLOAD_CREDS, LEGACY_DOWNLOAD_CREDS, {}, None])
def test_everything_else_is_detected_as_a_download(creds):
    assert detect_credential_origin(creds) == CredentialOrigin.SDK_DOWNLOAD


def test_the_login_writer_still_writes_the_login_only_keys():
    """The classification reads keys only `aim-sdk login` writes; pin that it still does."""
    tree = ast.parse(open(os.path.join(SDK_ROOT, "aim_sdk", "cli.py"), encoding="utf-8").read())
    login_fn = next(n for n in ast.walk(tree) if isinstance(n, ast.FunctionDef) and n.name == "login")
    written = set()
    for node in ast.walk(login_fn):
        if (
            isinstance(node, ast.Assign)
            and any(isinstance(t, ast.Name) and t.id == "credentials" for t in node.targets)
            and isinstance(node.value, ast.Dict)
        ):
            written |= {k.value for k in node.value.keys if isinstance(k, ast.Constant)}
    assert written, "login() no longer builds a `credentials = {...}` dict"
    for key in _LOGIN_ONLY_KEYS:
        assert key in written, f"aim-sdk login no longer writes {key!r}"


def test_the_download_bundle_carries_no_login_only_key():
    go_file = os.path.join(
        REPO_ROOT, "apps", "backend", "internal", "interfaces", "http", "handlers", "sdk_handler.go"
    )
    if not os.path.exists(go_file):
        pytest.skip("backend source not present (SDK tested outside the repository)")
    source = open(go_file, encoding="utf-8").read()
    struct = re.search(r"type SDKCredentials struct \{(.*?)\n\}", source, re.S)
    assert struct, "SDKCredentials struct not found in sdk_handler.go"
    tags = set(re.findall(r'json:"([^",]+)', struct.group(1)))
    assert "refreshToken" in tags
    for key in _LOGIN_ONLY_KEYS:
        assert key not in tags, f"the SDK download bundle now carries {key!r}"


# --- the banner -------------------------------------------------------------------


def test_login_banner_names_aim_sdk_login_and_no_download(capsys):
    out = _banner(capsys, LOGIN_CREDS)
    assert "SDK REFRESH TOKEN REJECTED" in out
    assert "created by `aim-sdk login`" in out
    assert login_recovery_command(AIM_URL) in out
    assert "maximum session age" in out
    assert "Download a fresh SDK" not in out
    assert "pip install -e" not in out
    assert "rm " not in out


def test_download_banner_keeps_the_download_steps_on_the_current_dashboard_path(capsys):
    out = _banner(capsys, DOWNLOAD_CREDS)
    assert "Remove the stale file" in out
    assert "Download a fresh SDK" in out
    assert "Developers, then SDK & docs" in out
    assert "Settings" not in out
    assert out.index("Download a fresh SDK") < out.index(login_recovery_command(AIM_URL))


def test_banner_without_credentials_prints_the_download_steps(capsys):
    print_token_expired_error(AIM_URL)
    out = capsys.readouterr().out
    assert "Download a fresh SDK" in out
    assert "created by `aim-sdk login`" not in out


@pytest.mark.parametrize("creds", [LOGIN_CREDS, DOWNLOAD_CREDS])
def test_every_cited_command_parses_and_reaches_login_with_the_server_url(capsys, monkeypatch, creds):
    text = _banner(capsys, creds) + "\n" + refresh_rejected_fix(creds, AIM_URL)
    cited = _cited_sdk_commands(text)
    recovery = login_recovery_command(AIM_URL)[len("aim-sdk "):]
    assert recovery in cited, "the banner does not cite the recovery command"
    for command in cited:
        seen = {}

        def fake_login(args):
            seen["url"], seen["force"] = args.url, args.force
            return 0

        monkeypatch.setattr(cli_module, "login", fake_login)
        monkeypatch.setattr(sys, "argv", ["aim-sdk"] + shlex.split(command))
        assert cli_module.main() == 0, command
        assert seen, f"`aim-sdk {command}` did not reach the login command"
        if command == recovery:
            assert seen == {"url": AIM_URL, "force": True}, command


def test_refresh_rejected_fix_per_origin():
    login_fix = refresh_rejected_fix(LOGIN_CREDS, AIM_URL)
    assert login_fix == f"Sign in again: {login_recovery_command(AIM_URL)}"
    download_fix = refresh_rejected_fix(DOWNLOAD_CREDS, AIM_URL)
    assert "Developers, then SDK & docs" in download_fix
    assert login_recovery_command(AIM_URL) in download_fix


# --- the refresh path hands the banner the credential -----------------------------


class _Response:
    def __init__(self, status_code, body):
        self.status_code = status_code
        self._body = body
        self.headers = {"content-type": "application/json"}
        self.text = str(body)

    def json(self):
        return self._body


@pytest.mark.parametrize(
    "creds, expect, reject",
    [
        (LOGIN_CREDS, "created by `aim-sdk login`", "Download a fresh SDK"),
        (DOWNLOAD_CREDS, "Download a fresh SDK", "created by `aim-sdk login`"),
    ],
)
def test_refused_refresh_prints_the_banner_for_the_stored_credential(capsys, monkeypatch, tmp_path, creds, expect, reject):
    # A refresh re-reads and locks the credentials file; keep both out of the real profile.
    monkeypatch.setattr("aim_sdk.credentials.SDK_CREDENTIALS_FILE", tmp_path / "sdk_credentials.json")
    manager = OAuthTokenManager.__new__(OAuthTokenManager)
    manager.credentials = dict(creds)
    manager.access_token = None
    manager.access_token_expiry = None

    def fake_post(url, json=None, timeout=None, **kwargs):
        return _Response(401, {"error": "invalid or revoked refresh token"})

    monkeypatch.setattr("aim_sdk.oauth.requests.post", fake_post)
    assert manager._refresh_token() is None
    out = capsys.readouterr().out
    assert "SDK REFRESH TOKEN REJECTED" in out
    assert expect in out
    assert reject not in out


# --- a download past the server's maximum age -------------------------------------

BACKEND_ROOT = os.path.join(REPO_ROOT, "apps", "backend", "internal")


def _backend_source(*parts):
    path = os.path.join(BACKEND_ROOT, *parts)
    if not os.path.exists(path):
        pytest.skip("backend source not present (SDK tested outside the repository)")
    return open(path, encoding="utf-8").read()


def _flat(text):
    return " ".join(text.split())


def test_download_banner_names_the_maximum_age_and_the_server_default(capsys):
    out = _flat(_banner(capsys, DOWNLOAD_CREDS))
    assert "The SDK download reached the server's maximum age (SDK_TOKEN_FAMILY_MAX_AGE, 365 days by default)." in out
    assert "Refreshing does not extend it; a fresh download starts a new one." in out
    assert "no longer the latest" not in out
    jwt_go = _backend_source("infrastructure", "auth", "jwt.go")
    assert 'getEnv("SDK_TOKEN_FAMILY_MAX_AGE", "8760h")' in jwt_go
    assert "defaultSDKTokenFamilyMaxAge = 365 * 24 * time.Hour" in jwt_go


def test_login_banner_does_not_name_the_download_maximum_age(capsys):
    assert "SDK_TOKEN_FAMILY_MAX_AGE" not in _banner(capsys, LOGIN_CREDS)


def test_the_server_answers_the_maximum_age_refusal_as_an_expired_token():
    handler = _backend_source("interfaces", "http", "handlers", "auth_refresh_handler.go")
    refusal = re.search(
        r"if h\.familyPastMaxAge\([^)]*\) \{\s*return c\.Status\(fiber\.StatusUnauthorized\)\.JSON\(fiber\.Map\{\s*"
        r'"error": "([^"]+)"',
        handler,
    )
    assert refusal, "the maximum-age refusal's body was not found in auth_refresh_handler.go"
    assert refusal.group(1) == "Invalid or expired refresh token"


@pytest.mark.parametrize("body", ["Invalid or expired refresh token", "Token has been revoked or is invalid"])
def test_a_refused_refresh_token_is_presented_once_and_the_banner_is_all_that_is_printed(
    capsys, monkeypatch, tmp_path, body
):
    assert f'"error": "{body}"' in _backend_source("interfaces", "http", "handlers", "auth_refresh_handler.go")
    monkeypatch.setattr("aim_sdk.credentials.SDK_CREDENTIALS_FILE", tmp_path / "sdk_credentials.json")
    manager = OAuthTokenManager.__new__(OAuthTokenManager)
    manager.credentials = dict(DOWNLOAD_CREDS)
    manager.access_token = None
    manager.access_token_expiry = None
    posted = []

    def fake_post(url, json=None, timeout=None, **kwargs):
        posted.append((url, json))
        return _Response(401, {"error": body})

    logged = []

    def fake_log(event_type, success=False, error=None, **kwargs):
        logged.append((event_type, success, error))

    monkeypatch.setattr("aim_sdk.oauth.requests.post", fake_post)
    monkeypatch.setattr("aim_sdk.oauth.security_logger.log_authentication", fake_log)
    assert manager._refresh_token() is None
    assert posted == [(f"{AIM_URL}/api/v1/auth/refresh", {"refreshToken": "refresh"})]
    assert logged == [(AuthnEventType.TOKEN_EXPIRED, False, "Refresh token refused")]
    out = capsys.readouterr().out
    assert out.lstrip().startswith("=" * 72)
    assert out.rstrip().endswith("=" * 72)
    assert "SDK REFRESH TOKEN REJECTED" in out
    assert "SDK_TOKEN_FAMILY_MAX_AGE" in out
    assert "Token was revoked" not in out
    assert "recovery" not in out.lower()
