"""
Tests for register_agent() function
"""

import pytest
from unittest.mock import patch, MagicMock, mock_open
import json

from aim_sdk import register_agent
from aim_sdk.exceptions import ConfigurationError


class TestRegisterAgent:
    """Test register_agent() function"""

    def test_register_agent_sdk_mode_success(self):
        """Test successful registration in SDK mode (OAuth)"""
        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token",
            "sdk_token_id": "mock_sdk_token"
        }

        # Mock registration response
        mock_response = {
            "agent_id": "123e4567-e89b-12d3-a456-426614174000",
            "name": "test-agent",
            "public_key": "mock_public_key",
            "private_key": "mock_private_key",
            "aim_url": "https://aim.example.com",
            "status": "verified",
            "trust_score": 75.0,
            "message": "Agent registered successfully"
        }

        with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
            with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                mock_oauth.return_value = MagicMock()

                # Should not raise error
                agent = register_agent("test-agent")

                # Verify OAuth mode was used
                mock_oauth.assert_called_once()

    def test_register_agent_api_key_mode_success(self):
        """Test successful registration in API key mode"""
        # Mock no SDK credentials
        with patch('aim_sdk.client.load_sdk_credentials', return_value=None):
            with patch('aim_sdk.client._register_via_api_key') as mock_api_key:
                mock_api_key.return_value = MagicMock()

                # Should not raise error
                agent = register_agent(
                    "test-agent",
                    aim_url="https://aim.example.com",
                    api_key="aim_abc123"
                )

                # Verify API key mode was used
                mock_api_key.assert_called_once()

    def test_register_agent_no_credentials_error(self):
        """Test error when no credentials provided"""
        # Mock no SDK credentials
        with patch('aim_sdk.client.load_sdk_credentials', return_value=None):
            with pytest.raises(ConfigurationError, match="No authentication credentials found"):
                register_agent("test-agent")

    def test_register_agent_api_key_without_url_error(self):
        """Test error when API key provided but no URL"""
        # Mock no SDK credentials
        with patch('aim_sdk.client.load_sdk_credentials', return_value=None):
            with pytest.raises(ConfigurationError, match="aim_url is required"):
                register_agent("test-agent", api_key="aim_abc123")

    def test_register_agent_auto_detect_capabilities(self):
        """Test auto-detection of capabilities"""
        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token"
        }

        # Mock capability detection
        with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
            with patch('aim_sdk.client.auto_detect_capabilities', return_value=["read_files", "write_files"]):
                with patch('aim_sdk.detection.auto_detect_mcps', return_value=[]):
                    with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                        mock_oauth.return_value = MagicMock()

                        agent = register_agent("test-agent", auto_detect=True)

                        # Verify capabilities were detected
                        # Check that registration_data included capabilities
                        call_args = mock_oauth.call_args
                        registration_data = call_args.kwargs['registration_data']
                        assert registration_data['capabilities'] == ["read_files", "write_files"]

    def test_register_agent_auto_detect_mcps(self):
        """Test auto-detection of MCP servers"""
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

                        agent = register_agent("test-agent", auto_detect=True)

                        # Verify MCP servers were detected
                        call_args = mock_oauth.call_args
                        talks_to = call_args.kwargs['talks_to']
                        assert talks_to == ["filesystem-mcp", "github-mcp"]

    def test_register_agent_disable_auto_detect(self):
        """Test disabling auto-detection"""
        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token"
        }

        with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
            with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                mock_oauth.return_value = MagicMock()

                agent = register_agent("test-agent", auto_detect=False)

                # Verify auto-detection was not called
                call_args = mock_oauth.call_args
                registration_data = call_args.kwargs['registration_data']
                assert 'capabilities' not in registration_data or registration_data['capabilities'] is None
                assert call_args.kwargs['talks_to'] is None

    def test_register_agent_manual_override(self):
        """Test manual capability and MCP override"""
        # Mock SDK credentials
        sdk_creds = {
            "aim_url": "https://aim.example.com",
            "refresh_token": "mock_refresh_token"
        }

        manual_capabilities = ["custom_capability"]
        manual_mcps = ["custom-mcp-server"]

        with patch('aim_sdk.client.load_sdk_credentials', return_value=sdk_creds):
            with patch('aim_sdk.client._register_via_oauth') as mock_oauth:
                mock_oauth.return_value = MagicMock()

                agent = register_agent(
                    "test-agent",
                    capabilities=manual_capabilities,
                    talks_to=manual_mcps
                )

                # Verify manual values were used
                call_args = mock_oauth.call_args
                registration_data = call_args.kwargs['registration_data']
                assert registration_data['capabilities'] == manual_capabilities
                assert call_args.kwargs['talks_to'] == manual_mcps

    def test_register_agent_existing_credentials(self):
        """Test loading existing credentials"""
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
                agent = register_agent("existing-agent")

                # Should create client with existing credentials
                mock_client.assert_called_once_with(
                    agent_id=existing_creds["agent_id"],
                    public_key=existing_creds["public_key"],
                    private_key=existing_creds["private_key"],
                    aim_url=existing_creds["aim_url"],
                    oauth_token_manager=None
                )

    def test_register_agent_force_new_registration(self):
        """Test forcing new registration even when credentials exist"""
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

                    agent = register_agent("existing-agent", force_new=True)

                    # Should call registration, not use existing credentials
                    mock_oauth.assert_called_once()


if __name__ == '__main__':
    pytest.main([__file__, '-v'])
