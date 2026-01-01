"""
Tests for AIM SDK Credential Management

Matches Java SDK CredentialManagerTest coverage:
- Loading credentials from files
- Saving SDK and agent credentials
- Credential type detection
- Path management
"""

import pytest
import json
import tempfile
from pathlib import Path
from unittest.mock import patch, MagicMock

from aim_sdk.credentials import (
    CredentialType,
    detect_credential_type,
    get_sdk_credentials_path,
    load_sdk_credentials,
    save_sdk_credentials,
    get_agent_credentials_path,
    load_agent_credentials,
    save_agent_credentials,
    list_agent_credentials,
    delete_agent_credentials,
    AIM_DIR,
    AGENTS_DIR,
)


class TestCredentialTypeDetection:
    """Test credential type detection."""

    def test_detect_sdk_oauth_with_refresh_token(self):
        """SDK OAuth credentials should be detected by refreshToken."""
        data = {
            "refreshToken": "test-refresh-token",
            "sdkTokenId": "sdk-123",
            "aimUrl": "http://localhost:8080"
        }
        assert detect_credential_type(data) == CredentialType.SDK_OAUTH

    def test_detect_sdk_oauth_with_snake_case(self):
        """SDK OAuth credentials with snake_case should be detected."""
        data = {
            "refresh_token": "test-refresh-token",
            "sdk_token_id": "sdk-123"
        }
        assert detect_credential_type(data) == CredentialType.SDK_OAUTH

    def test_detect_agent_credentials_with_agent_id(self):
        """Agent credentials should be detected by agentId."""
        data = {
            "agentId": "agent-123",
            "privateKey": "base64-private-key",
            "publicKey": "base64-public-key"
        }
        assert detect_credential_type(data) == CredentialType.AGENT

    def test_detect_agent_credentials_with_snake_case(self):
        """Agent credentials with snake_case should be detected."""
        data = {
            "agent_id": "agent-123",
            "private_key": "base64-private-key"
        }
        assert detect_credential_type(data) == CredentialType.AGENT

    def test_detect_unknown_credentials(self):
        """Unknown credential format should return UNKNOWN."""
        data = {
            "foo": "bar",
            "baz": 123
        }
        assert detect_credential_type(data) == CredentialType.UNKNOWN

    def test_detect_empty_credentials(self):
        """Empty credentials should return UNKNOWN."""
        data = {}
        assert detect_credential_type(data) == CredentialType.UNKNOWN


class TestSdkCredentialsPath:
    """Test SDK credentials path management."""

    def test_get_sdk_credentials_path_returns_path(self):
        """Should return a Path object."""
        path = get_sdk_credentials_path()
        assert isinstance(path, Path)

    def test_get_sdk_credentials_path_in_aim_dir(self):
        """Path should be in ~/.aim directory."""
        path = get_sdk_credentials_path()
        assert ".aim" in str(path)
        assert "sdk_credentials.json" in str(path)


class TestAgentCredentialsPath:
    """Test agent credentials path management."""

    def test_get_agent_credentials_path_returns_path(self):
        """Should return a Path object."""
        path = get_agent_credentials_path("test-agent")
        assert isinstance(path, Path)

    def test_get_agent_credentials_path_in_agents_dir(self):
        """Path should be in ~/.aim/agents directory."""
        path = get_agent_credentials_path("my-agent")
        assert ".aim" in str(path)
        assert "agents" in str(path)
        assert "my-agent.json" in str(path)


class TestSaveSdkCredentials:
    """Test saving SDK credentials."""

    def test_save_sdk_credentials_with_valid_data(self):
        """Saving valid credentials should return True."""
        with tempfile.TemporaryDirectory() as tmpdir:
            tmp_path = Path(tmpdir) / "sdk_credentials.json"

            credentials = {
                "refreshToken": "test-refresh-token",
                "sdkTokenId": "sdk-123",
                "aimUrl": "http://localhost:8080",
                "organizationId": "org-456"
            }

            # Mock the SDK_CREDENTIALS_FILE to use temp path
            with patch('aim_sdk.credentials.SDK_CREDENTIALS_FILE', tmp_path):
                with patch('aim_sdk.credentials.AIM_DIR', Path(tmpdir)):
                    result = save_sdk_credentials(credentials)
                    assert result is True
                    assert tmp_path.exists()


class TestSaveAgentCredentials:
    """Test saving agent credentials."""

    def test_save_agent_credentials_with_valid_data(self):
        """Saving valid agent credentials should return True."""
        with tempfile.TemporaryDirectory() as tmpdir:
            agents_dir = Path(tmpdir) / "agents"
            agents_dir.mkdir(parents=True)

            credentials = {
                "agentId": "agent-123",
                "privateKey": "base64-private-key",
                "publicKey": "base64-public-key",
                "organizationId": "org-456"
            }

            with patch('aim_sdk.credentials.AGENTS_DIR', agents_dir):
                result = save_agent_credentials("test-agent", credentials)
                assert result is True
                assert (agents_dir / "test-agent.json").exists()


class TestLoadSdkCredentials:
    """Test loading SDK credentials."""

    def test_load_sdk_credentials_returns_none_for_missing_file(self):
        """Loading from non-existent file should return None."""
        with tempfile.TemporaryDirectory() as tmpdir:
            with patch('aim_sdk.credentials.SDK_CREDENTIALS_FILE', Path(tmpdir) / "nonexistent.json"):
                with patch('aim_sdk.credentials.LEGACY_CREDENTIALS_FILE', Path(tmpdir) / "legacy.json"):
                    result = load_sdk_credentials()
                    assert result is None

    def test_load_sdk_credentials_from_file(self):
        """Should load credentials from file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            cred_path = Path(tmpdir) / "sdk_credentials.json"
            credentials = {
                "refreshToken": "loaded-token",
                "sdkTokenId": "loaded-sdk-id",
                "aimUrl": "http://loaded:8080",
                "organizationId": "org-123"
            }
            cred_path.write_text(json.dumps(credentials))

            with patch('aim_sdk.credentials.SDK_CREDENTIALS_FILE', cred_path):
                result = load_sdk_credentials()
                assert result is not None
                # Check for either camelCase or snake_case (depends on implementation)
                assert "refreshToken" in result or "refresh_token" in result


class TestLoadAgentCredentials:
    """Test loading agent credentials."""

    def test_load_agent_credentials_returns_none_for_missing_file(self):
        """Loading from non-existent agent should return None."""
        with tempfile.TemporaryDirectory() as tmpdir:
            agents_dir = Path(tmpdir) / "agents"
            agents_dir.mkdir(parents=True)

            with patch('aim_sdk.credentials.AGENTS_DIR', agents_dir):
                result = load_agent_credentials("nonexistent-agent")
                assert result is None

    def test_load_agent_credentials_from_file(self):
        """Should load agent credentials from file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            agents_dir = Path(tmpdir) / "agents"
            agents_dir.mkdir(parents=True)
            cred_path = agents_dir / "test-agent.json"

            credentials = {
                "agentId": "agent-loaded",
                "privateKey": "loaded-private-key",
                "publicKey": "loaded-public-key"
            }
            cred_path.write_text(json.dumps(credentials))

            with patch('aim_sdk.credentials.AGENTS_DIR', agents_dir):
                result = load_agent_credentials("test-agent")
                assert result is not None
                assert result["agentId"] == "agent-loaded"


class TestListAgentCredentials:
    """Test listing agent credentials."""

    def test_list_agent_credentials_empty_dir(self):
        """Should return empty list for empty directory."""
        with tempfile.TemporaryDirectory() as tmpdir:
            agents_dir = Path(tmpdir) / "agents"
            agents_dir.mkdir(parents=True)

            with patch('aim_sdk.credentials.AGENTS_DIR', agents_dir):
                result = list_agent_credentials()
                assert result == []

    def test_list_agent_credentials_with_agents(self):
        """Should list all agent credential files."""
        with tempfile.TemporaryDirectory() as tmpdir:
            agents_dir = Path(tmpdir) / "agents"
            agents_dir.mkdir(parents=True)

            # Create some agent credential files
            (agents_dir / "agent-1.json").write_text('{"agentId": "1"}')
            (agents_dir / "agent-2.json").write_text('{"agentId": "2"}')
            (agents_dir / "agent-3.json").write_text('{"agentId": "3"}')

            with patch('aim_sdk.credentials.AGENTS_DIR', agents_dir):
                result = list_agent_credentials()
                assert len(result) == 3
                # Result is list of dicts with 'name' key
                names = [item['name'] for item in result]
                assert "agent-1" in names
                assert "agent-2" in names
                assert "agent-3" in names


class TestDeleteAgentCredentials:
    """Test deleting agent credentials."""

    def test_delete_agent_credentials_removes_file(self):
        """Deleting should remove the credential file."""
        with tempfile.TemporaryDirectory() as tmpdir:
            agents_dir = Path(tmpdir) / "agents"
            agents_dir.mkdir(parents=True)
            cred_path = agents_dir / "delete-me.json"
            cred_path.write_text('{"agentId": "delete-me"}')

            assert cred_path.exists()

            with patch('aim_sdk.credentials.AGENTS_DIR', agents_dir):
                delete_agent_credentials("delete-me")

            assert not cred_path.exists()

    def test_delete_agent_credentials_nonexistent(self):
        """Deleting non-existent agent should not raise."""
        with tempfile.TemporaryDirectory() as tmpdir:
            agents_dir = Path(tmpdir) / "agents"
            agents_dir.mkdir(parents=True)

            with patch('aim_sdk.credentials.AGENTS_DIR', agents_dir):
                # Should not raise - returns True even for non-existent
                result = delete_agent_credentials("nonexistent")
                assert result is True  # Silently succeeds even if not found


class TestCredentialsImmutability:
    """Test that credentials are handled safely."""

    def test_loaded_credentials_are_dict(self):
        """Loaded credentials should be a dictionary."""
        with tempfile.TemporaryDirectory() as tmpdir:
            agents_dir = Path(tmpdir) / "agents"
            agents_dir.mkdir(parents=True)
            cred_path = agents_dir / "safe-agent.json"
            cred_path.write_text('{"agentId": "safe", "privateKey": "key"}')

            with patch('aim_sdk.credentials.AGENTS_DIR', agents_dir):
                result = load_agent_credentials("safe-agent")
                assert isinstance(result, dict)


class TestCredentialTypeConstants:
    """Test CredentialType constants."""

    def test_sdk_oauth_constant(self):
        """SDK_OAUTH constant should exist."""
        assert CredentialType.SDK_OAUTH == "sdk_oauth"

    def test_agent_constant(self):
        """AGENT constant should exist."""
        assert CredentialType.AGENT == "agent"

    def test_unknown_constant(self):
        """UNKNOWN constant should exist."""
        assert CredentialType.UNKNOWN == "unknown"


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
