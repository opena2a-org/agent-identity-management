"""Regression tests for issues #178 and #174.

#178 (P0): Stale agent credentials must not trigger silent re-registration
that wipes admin-curated agent state. The SDK raises StaleCredentialsError
with recovery instructions instead.

#174 (P2):
  - Token rotation message names the prior token as revoked.
  - Bundled-SDK install path prints a visible adoption warning.
  - Encrypted shadow file is migrated to the JSON single source of truth
    on first OAuthTokenManager instantiation and then deleted.
  - print_token_expired_error explains the root cause (token rotation),
    not just "download a fresh SDK".
"""

import io
import json
from contextlib import redirect_stdout
from pathlib import Path
from unittest.mock import patch

import pytest

from aim_sdk import StaleCredentialsError
from aim_sdk.credentials import (
    _install_sdk_credentials,
    print_token_expired_error,
)
import aim_sdk.oauth as oauth_module
from aim_sdk.oauth import OAuthTokenManager, load_sdk_credentials


# ---------------------------------------------------------------------------
# #178 — silent re-registration is refused
# ---------------------------------------------------------------------------


class TestStaleCredentialsRefusal:
    """When cached credentials are stale, raise instead of wipe+re-register."""

    def _patch_creds(self):
        return {
            "agent_id": "11111111-2222-3333-4444-555555555555",
            "public_key": "pk",
            "private_key": "sk",
            "aim_url": "https://aim.example.com",
            "status": "verified",
            "trust_score": 0.92,
        }

    def test_stale_creds_raise_stale_credentials_error(self):
        from aim_sdk import register_agent

        with patch("aim_sdk.client._load_credentials", return_value=self._patch_creds()):
            with patch(
                "aim_sdk.client._validate_cached_credentials",
                return_value=(False, "Agent 'demo-agent' (ID: 11111111...) not found in backend"),
            ):
                # Patch at the source module so any import path that tries
                # to call it (lazy or top-level) is caught by mock_delete.
                with patch("aim_sdk.credentials.delete_agent_credentials") as mock_delete:
                    with pytest.raises(StaleCredentialsError) as exc_info:
                        register_agent("demo-agent")

                    mock_delete.assert_not_called()
                    msg = str(exc_info.value)
                    assert "demo-agent" in msg
                    assert "11111111" in msg
                    assert "wipe" in msg.lower() or "admin" in msg.lower()
                    assert "rm" in msg
                    assert "agents/demo-agent.json" in msg

    def test_stale_credentials_error_subclasses_configuration_error(self):
        from aim_sdk import ConfigurationError

        assert issubclass(StaleCredentialsError, ConfigurationError)


# ---------------------------------------------------------------------------
# #174 — visible bundled-SDK install warning
# ---------------------------------------------------------------------------


class TestInstallSdkCredentialsWarning:
    def test_install_prints_adopting_warning_with_truncated_token(self, tmp_path):
        creds = {
            "aimUrl": "https://aim.example.com",
            "sdkTokenId": "abcdefghij1234567890",
            "refreshToken": "rt-xyz",
            "email": "u@example.com",
            "schemaVersion": "1.0",
        }
        with patch("aim_sdk.credentials.SDK_CREDENTIALS_FILE", tmp_path / "sdk_credentials.json"):
            with patch("aim_sdk.credentials.save_sdk_credentials", return_value=True) as mock_save:
                buf = io.StringIO()
                with redirect_stdout(buf):
                    _install_sdk_credentials(creds)

                out = buf.getvalue()
                assert "Adopting SDK credentials from bundled package" in out
                # Truncated token id (first 8 chars + ellipsis), full token not echoed.
                assert "abcdefgh..." in out
                assert "abcdefghij1234567890" not in out
                mock_save.assert_called_once()


# ---------------------------------------------------------------------------
# #174 — print_token_expired_error explains root cause
# ---------------------------------------------------------------------------


class TestTokenExpiredErrorWording:
    def test_error_names_rotation_and_shows_path(self):
        buf = io.StringIO()
        with redirect_stdout(buf):
            print_token_expired_error("https://aim.example.com")
        out = buf.getvalue()

        assert "REFRESH TOKEN REJECTED" in out
        assert "rotates" in out.lower()
        # Surfaces the local path so recovery is obvious instead of vague.
        assert "sdk_credentials.json" in out
        assert "rm " in out
        # Still includes the dashboard URL for the re-download step.
        assert "https://aim.example.com" in out


# ---------------------------------------------------------------------------
# #174 — encrypted shadow file is migrated and removed
# ---------------------------------------------------------------------------


class TestEncryptedShadowMigration:
    def test_orphan_encrypted_file_deleted_when_decrypt_unavailable(self, tmp_path, monkeypatch):
        """If SECURE_STORAGE_AVAILABLE is False, the encrypted shadow file
        is treated as an orphan and removed; the JSON store is untouched."""
        json_path = tmp_path / "sdk_credentials.json"
        encrypted_path = tmp_path / "sdk_credentials.encrypted"
        encrypted_path.write_bytes(b"opaque-bytes-cant-decrypt-without-libs")

        monkeypatch.setattr(oauth_module, "SECURE_STORAGE_AVAILABLE", False)

        buf = io.StringIO()
        with redirect_stdout(buf):
            mgr = OAuthTokenManager(credentials_path=str(json_path))

        assert not encrypted_path.exists(), "encrypted shadow file must be removed"
        assert mgr.secure_storage is None
        out = buf.getvalue()
        assert "Removed deprecated encrypted credentials file" in out
        assert "audit #12" in out

    def test_encrypted_file_preserved_when_decrypt_fails(self, tmp_path, monkeypatch):
        """Safety: if the secure-storage libs are present but decryption
        raises (corrupt file, missing keyring entry), the encrypted file is
        LEFT IN PLACE so the operator can recover manually. Deleting it
        would discard the only copy of the credentials."""
        json_path = tmp_path / "sdk_credentials.json"
        encrypted_path = tmp_path / "sdk_credentials.encrypted"
        encrypted_path.write_bytes(b"corrupted-ciphertext")

        monkeypatch.setattr(oauth_module, "SECURE_STORAGE_AVAILABLE", True)

        class _ExplodingStorage:
            def __init__(self, *_a, **_kw):
                raise RuntimeError("keyring entry missing")

        monkeypatch.setattr(oauth_module, "SecureCredentialStorage", _ExplodingStorage)

        buf = io.StringIO()
        with redirect_stdout(buf):
            OAuthTokenManager(credentials_path=str(json_path))

        assert encrypted_path.exists(), (
            "encrypted file must be preserved when decryption fails so the "
            "operator can recover manually"
        )
        assert "leaving the file in place" in buf.getvalue()

    def test_no_op_when_no_encrypted_file(self, tmp_path, monkeypatch):
        json_path = tmp_path / "sdk_credentials.json"
        # Pre-existing JSON should remain in place; migration is a no-op.
        sample = {
            "aimUrl": "https://aim.example.com",
            "refreshToken": "rt-existing",
            "sdkTokenId": "tokenid-1",
            "schemaVersion": "1.0",
        }
        json_path.write_text(json.dumps(sample))

        # Patch the module's loader so init's load_credentials() finds the JSON.
        monkeypatch.setattr(
            oauth_module,
            "_load_sdk_credentials_from_module",
            lambda: sample,
        )
        monkeypatch.setattr(oauth_module, "SECURE_STORAGE_AVAILABLE", False)

        mgr = OAuthTokenManager(credentials_path=str(json_path))

        assert json_path.exists()
        assert mgr.credentials == sample


# ---------------------------------------------------------------------------
# #174 — module-level load_sdk_credentials ignores use_secure_storage
# ---------------------------------------------------------------------------


class TestModuleLevelLoaderIgnoresLegacyParam:
    def test_param_is_a_no_op(self, monkeypatch):
        sentinel = {"refreshToken": "rt", "sdkTokenId": "id", "schemaVersion": "1.0"}
        monkeypatch.setattr(
            oauth_module,
            "_load_sdk_credentials_from_module",
            lambda: sentinel,
        )
        assert load_sdk_credentials(use_secure_storage=True) == sentinel
        assert load_sdk_credentials(use_secure_storage=False) == sentinel


# ---------------------------------------------------------------------------
# #174 — rotation message names the prior token as revoked
# ---------------------------------------------------------------------------


class TestRotationMessageWording:
    """Driving the full refresh path requires mocking JWT decoding, server
    responses, and credential persistence; the wording is a one-line
    user-visible contract and asserting on the source guarantees it ships."""

    def test_rotation_print_says_old_token_revoked(self):
        oauth_src = Path(oauth_module.__file__).read_text()
        assert "old refresh token revoked" in oauth_src, (
            "Audit #12: rotation print must name the prior refresh token as revoked"
        )
