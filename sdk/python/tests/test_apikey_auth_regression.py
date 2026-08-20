"""
Regression tests for API-key-only authentication bugs.

Bug 1: perform_action crashes with AttributeError when signing_key is None
Bug 2: _register_single_mcp has variable scoping error (json_body_str undefined
       in API-key-only mode)
Bug 3: SecurityLogger._create_event double-passes agent_id when set_agent_context
       has been called and log_authentication also receives agent_id
"""

import base64
import json
import pytest
import responses
from unittest.mock import patch, MagicMock
from nacl.signing import SigningKey
from nacl.encoding import Base64Encoder

from aim_sdk import AIMClient
from aim_sdk.exceptions import (
    ConfigurationError,
    AuthenticationError,
    VerificationError,
    ActionDeniedError,
)
from aim_sdk.security_logging import (
    SecurityLogger,
    EventCategory,
    EventSeverity,
    AuthnEventType,
    security_logger,
)


# ---------------------------------------------------------------------------
# Fixtures
# ---------------------------------------------------------------------------

@pytest.fixture
def api_key_client():
    """Create an AIMClient in API-key-only mode (no Ed25519 keys)."""
    return AIMClient(
        agent_id="550e8400-e29b-41d4-a716-446655440000",
        aim_url="https://aim.example.com",
        api_key="aim_test_key_12345",
        timeout=5,
        auto_retry=False,
    )


@pytest.fixture
def ed25519_client():
    """Create an AIMClient with Ed25519 keys for comparison tests."""
    signing_key = SigningKey.generate()
    public_key = signing_key.verify_key.encode(encoder=Base64Encoder).decode("utf-8")
    private_key = base64.b64encode(bytes(signing_key)).decode("utf-8")
    return AIMClient(
        agent_id="550e8400-e29b-41d4-a716-446655440000",
        public_key=public_key,
        private_key=private_key,
        aim_url="https://aim.example.com",
        timeout=5,
        auto_retry=False,
    )


# ---------------------------------------------------------------------------
# Bug 1: verify_capability must not crash in API-key-only mode
# ---------------------------------------------------------------------------

class TestApiKeyOnlyVerifyCapability:
    """Regression: verify_capability must work without Ed25519 signing keys."""

    @responses.activate
    def test_verify_capability_api_key_mode_sends_request(self, api_key_client):
        """verify_capability should send a request using API key auth, not crash."""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={
                "id": "ver-123",
                "status": "approved",
                "approvedBy": "auto",
            },
            status=200,
        )

        result = api_key_client.verify_capability(
            capability="db:read",
            resource="users_table",
        )

        assert result["verified"] is True
        assert result["verification_id"] == "ver-123"

        # Verify the request used API key header, not a signature
        request_headers = responses.calls[0].request.headers
        assert request_headers.get("X-API-Key") == "aim_test_key_12345"
        assert "X-Signature" not in request_headers

    @responses.activate
    def test_verify_capability_api_key_mode_no_signature_in_body(self, api_key_client):
        """In API-key-only mode, request body should not contain signature or publicKey."""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={
                "id": "ver-456",
                "status": "approved",
                "approvedBy": "auto",
            },
            status=200,
        )

        api_key_client.verify_capability(capability="email:send")

        body = json.loads(responses.calls[0].request.body)
        assert "signature" not in body
        assert "publicKey" not in body
        assert body["capability"] == "email:send"
        assert body["agentId"] == "550e8400-e29b-41d4-a716-446655440000"

    def test_verify_capability_no_auth_raises_config_error(self):
        """Client with neither signing_key nor api_key should raise ConfigurationError."""
        client = AIMClient(
            agent_id="test-agent",
            aim_url="https://aim.example.com",
            api_key="temp_key",
        )
        client.api_key = None  # Remove after init to simulate edge case
        client.signing_key = None

        with pytest.raises(ConfigurationError, match="Ed25519 keys required"):
            client.verify_capability(capability="db:write")

    @responses.activate
    def test_perform_action_decorator_api_key_mode(self, api_key_client):
        """The @perform_action decorator must not crash in API-key-only mode."""
        # Mock register_capability endpoint
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/agents/550e8400-e29b-41d4-a716-446655440000/capabilities/register",
            json={"status": "granted", "message": "OK"},
            status=200,
        )
        # Mock verify_capability endpoint
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={
                "id": "ver-789",
                "status": "approved",
                "approvedBy": "auto",
            },
            status=200,
        )
        # Mock log_capability_result endpoint
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications/ver-789/result",
            json={"success": True},
            status=200,
        )

        @api_key_client.perform_action(capability="test:action")
        def my_test_function():
            return "success"

        result = my_test_function()
        assert result == "success"


# ---------------------------------------------------------------------------
# Bug 2: _register_single_mcp variable scoping (json_body_str)
# ---------------------------------------------------------------------------

class TestRegisterSingleMcpScoping:
    """Regression: _register_single_mcp must not crash when signing_key is None."""

    @responses.activate
    def test_register_single_mcp_api_key_mode(self, api_key_client):
        """MCP registration should work with API-key-only auth (no NameError)."""
        from aim_sdk.client import _register_single_mcp

        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/agents/550e8400-e29b-41d4-a716-446655440000/mcp-servers",
            json={"id": "mcp-001", "name": "test-mcp"},
            status=201,
        )

        mcp_def = {
            "name": "test-mcp",
            "description": "Test MCP server",
            "url": "stdio://test-server",
            "version": "1.0.0",
            "capabilities": ["read", "write"],
        }

        result = _register_single_mcp(
            client=api_key_client,
            aim_url="https://aim.example.com",
            headers={"Content-Type": "application/json"},
            mcp_def=mcp_def,
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            detected_capabilities={},
        )

        assert result["success"] is True, f"Expected success but got: {result}"
        assert result["status"] == "registered"

        # Verify API key was sent in headers
        request_headers = responses.calls[0].request.headers
        assert request_headers.get("X-API-Key") == "aim_test_key_12345"

    @responses.activate
    def test_register_single_mcp_ed25519_mode(self, ed25519_client):
        """MCP registration should still work with Ed25519 keys."""
        from aim_sdk.client import _register_single_mcp

        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/agents/550e8400-e29b-41d4-a716-446655440000/mcp-servers",
            json={"id": "mcp-002", "name": "test-mcp"},
            status=201,
        )

        mcp_def = {
            "name": "test-mcp",
            "description": "Test MCP server",
            "url": "stdio://test-server",
            "version": "1.0.0",
            "capabilities": [],
        }

        result = _register_single_mcp(
            client=ed25519_client,
            aim_url="https://aim.example.com",
            headers={"Content-Type": "application/json"},
            mcp_def=mcp_def,
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            detected_capabilities={},
        )

        assert result["success"] is True
        # Ed25519 mode should have signature headers
        request_headers = responses.calls[0].request.headers
        assert "X-Signature" in request_headers


# ---------------------------------------------------------------------------
# Bug 3: SecurityLogger._create_event double agent_id
# ---------------------------------------------------------------------------

class TestSecurityLoggerDuplicateKwargs:
    """Regression: _create_event must not crash when agent_id is in both
    _agent_context and kwargs."""

    def test_log_authentication_after_set_agent_context(self):
        """Calling log_authentication with agent_id after set_agent_context
        should not raise TypeError for duplicate keyword argument."""
        logger = SecurityLogger()
        logger.configure(enabled=True)

        # Set persistent context with agent_id
        logger.set_agent_context(agent_id="agent-from-context")

        # This should NOT raise TypeError: got multiple values for keyword argument 'agent_id'
        logger.log_authentication(
            event_type=AuthnEventType.TOKEN_REFRESH,
            success=True,
            agent_id="agent-from-call",
            message="Token refreshed successfully",
        )

        # Clean up
        logger.clear_agent_context()

    def test_per_call_agent_id_overrides_context(self):
        """Per-call agent_id should take precedence over set_agent_context value."""
        logger = SecurityLogger()
        logger.configure(enabled=True)

        logger.set_agent_context(agent_id="context-agent")

        # Capture the event by patching _log_event
        captured_events = []
        original_log = logger._log_event

        def capture_event(event, level):
            captured_events.append(event)
            original_log(event, level)

        logger._log_event = capture_event

        logger.log_authentication(
            event_type=AuthnEventType.CREDENTIAL_LOAD,
            success=True,
            agent_id="explicit-agent",
        )

        assert len(captured_events) == 1
        assert captured_events[0].agent_id == "explicit-agent"

        logger.clear_agent_context()

    def test_context_agent_id_used_when_no_explicit(self):
        """When no agent_id is passed to log_authentication, context value is used."""
        logger = SecurityLogger()
        logger.configure(enabled=True)

        logger.set_agent_context(agent_id="context-only-agent")

        captured_events = []
        original_log = logger._log_event

        def capture_event(event, level):
            captured_events.append(event)
            original_log(event, level)

        logger._log_event = capture_event

        logger.log_authentication(
            event_type=AuthnEventType.CREDENTIAL_LOAD,
            success=True,
            # No agent_id passed -- should use context
        )

        assert len(captured_events) == 1
        assert captured_events[0].agent_id == "context-only-agent"

        logger.clear_agent_context()

    def test_no_context_no_explicit_agent_id(self):
        """When neither context nor explicit agent_id is set, agent_id should be None."""
        logger = SecurityLogger()
        logger.configure(enabled=True)

        captured_events = []
        original_log = logger._log_event

        def capture_event(event, level):
            captured_events.append(event)
            original_log(event, level)

        logger._log_event = capture_event

        logger.log_authentication(
            event_type=AuthnEventType.CREDENTIAL_LOAD,
            success=True,
        )

        assert len(captured_events) == 1
        assert captured_events[0].agent_id is None
