"""
Tests for secure() function alias

The secure() function is an alias for register_agent() that provides
a more intuitive API for enterprise users. These tests verify that
both functions behave identically.
"""

import pytest
from unittest.mock import patch, MagicMock

from aim_sdk import secure, register_agent
from aim_sdk.exceptions import ConfigurationError


class TestSecureAlias:
    """Test secure() function alias"""

    def test_secure_is_alias_for_register_agent(self):
        """Verify secure() is an alias for register_agent()"""
        assert secure == register_agent, "secure() should be an alias for register_agent()"

    def test_secure_sdk_mode_success(self):
        """Test successful registration using secure() in SDK mode (OAuth)"""
        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token",
            "sdk_token_id": "mock_sdk_token"
        }

        with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
            with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                mock_oauth.return_value = MagicMock()

                # Use secure() instead of register_agent()
                agent = secure("test-agent")

                # Verify OAuth mode was used
                mock_oauth.assert_called_once()

    def test_secure_api_key_mode_success(self):
        """Test successful registration using secure() in API key mode"""
        # Mock no SDK credentials
        with patch('aim_sdk.client.load_sdk_credentials', return_value=None):
            with patch('aim_sdk.client._register_via_api_key') as mock_api_key:
                mock_api_key.return_value = MagicMock()

                # Use secure() with API key
                agent = secure(
                    "test-agent",
                    aim_url="https://aim.example.com",
                    api_key="aim_abc123"
                )

                # Verify API key mode was used
                mock_api_key.assert_called_once()

    def test_secure_no_credentials_error(self):
        """Test error when no credentials provided to secure()"""
        # Mock no SDK credentials
        with patch('aim_sdk.client.load_sdk_credentials', return_value=None):
            with pytest.raises(ConfigurationError, match="No authentication credentials found"):
                secure("test-agent")

    def test_secure_with_capabilities(self):
        """Test secure() with manual capabilities"""
        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token"
        }

        manual_capabilities = ["read_files", "write_files", "execute_code"]

        with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
            with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                mock_oauth.return_value = MagicMock()

                agent = secure(
                    "test-agent",
                    capabilities=manual_capabilities
                )

                # Verify capabilities were passed
                call_args = mock_oauth.call_args
                registration_data = call_args.kwargs['registration_data']
                assert registration_data['capabilities'] == manual_capabilities

    def test_secure_with_mcps(self):
        """Test secure() with manual MCP servers"""
        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token"
        }

        manual_mcps = ["filesystem-mcp", "github-mcp", "database-mcp"]

        with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
            with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                mock_oauth.return_value = MagicMock()

                agent = secure(
                    "test-agent",
                    talks_to=manual_mcps
                )

                # Verify MCPs were passed
                call_args = mock_oauth.call_args
                assert call_args.kwargs['talks_to'] == manual_mcps

    def test_secure_auto_detect_capabilities(self):
        """Test secure() with auto-detection of capabilities"""
        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token"
        }

        with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
            with patch('aim_sdk.client.auto_detect_capabilities', return_value=["read_files", "write_files"]):
                with patch('aim_sdk.detection.auto_detect_mcps', return_value=[]):
                    with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                        mock_oauth.return_value = MagicMock()

                        agent = secure("test-agent", auto_detect=True)

                        # Verify capabilities were detected
                        call_args = mock_oauth.call_args
                        registration_data = call_args.kwargs['registration_data']
                        assert registration_data['capabilities'] == ["read_files", "write_files"]

    def test_secure_auto_detect_mcps(self):
        """Test secure() with auto-detection of MCP servers"""
        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token"
        }

        # Mock MCP detection
        mcp_detections = [
            {"mcpServer": "filesystem-mcp"},
            {"mcpServer": "github-mcp"}
        ]

        with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
            with patch('aim_sdk.client.auto_detect_capabilities', return_value=[]):
                with patch('aim_sdk.detection.auto_detect_mcps', return_value=mcp_detections):
                    with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                        mock_oauth.return_value = MagicMock()

                        agent = secure("test-agent", auto_detect=True)

                        # Verify MCP servers were detected
                        call_args = mock_oauth.call_args
                        talks_to = call_args.kwargs['talks_to']
                        assert talks_to == ["filesystem-mcp", "github-mcp"]

    def test_secure_existing_credentials(self):
        """Test secure() loading existing credentials"""
        existing_creds = {
            "agent_id": "existing-agent-id",
            "public_key": "existing-public-key",
            "private_key": "existing-private-key",
            "aim_url": "https://aim.example.com",
            "status": "verified",
            "trust_score": 80.0
        }

        with patch('aim_sdk.client._load_credentials', return_value=existing_creds):
            with patch('aim_sdk.client.AIMClient') as mock_client:
                agent = secure("existing-agent")

                # Should create client with existing credentials
                mock_client.assert_called_once_with(
                    agent_id=existing_creds["agent_id"],
                    public_key=existing_creds["public_key"],
                    private_key=existing_creds["private_key"],
                    aim_url=existing_creds["aim_url"],
                    oauth_token_manager=None
                )

    def test_secure_force_new_registration(self):
        """Test secure() forcing new registration"""
        existing_creds = {
            "agent_id": "existing-agent-id",
            "public_key": "existing-public-key",
            "private_key": "existing-private-key",
            "aim_url": "https://aim.example.com",
            "status": "verified",
            "trust_score": 80.0
        }

        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token"
        }

        with patch('aim_sdk.client._load_credentials', return_value=existing_creds):
            with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
                with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                    mock_oauth.return_value = MagicMock()

                    agent = secure("existing-agent", force_new=True)

                    # Should call registration, not use existing credentials
                    mock_oauth.assert_called_once()

    def test_both_functions_identical_behavior(self):
        """Verify secure() and register_agent() have identical behavior"""
        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token"
        }

        with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
            with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                mock_oauth.return_value = MagicMock()

                # Call both functions with same arguments
                agent1 = secure("test-agent", auto_detect=True)

                # Reset mock
                mock_oauth.reset_mock()

                agent2 = register_agent("test-agent", auto_detect=True)

                # Both should call OAuth registration
                assert mock_oauth.call_count == 1


if __name__ == '__main__':
    pytest.main([__file__, '-v'])
