"""
Tests for AIMClient
"""

import base64
import json
import pytest
import responses
from nacl.signing import SigningKey
from nacl.encoding import Base64Encoder

from aim_sdk import AIMClient
from aim_sdk.exceptions import (
    ConfigurationError,
    AuthenticationError,
    VerificationError,
    ActionDeniedError
)


# Test fixtures
@pytest.fixture
def test_keys():
    """Generate test Ed25519 key pair"""
    signing_key = SigningKey.generate()
    public_key = signing_key.verify_key.encode(encoder=Base64Encoder).decode('utf-8')
    private_key = base64.b64encode(bytes(signing_key)).decode('utf-8')
    return {
        'public_key': public_key,
        'private_key': private_key,
        'signing_key': signing_key
    }


@pytest.fixture
def aim_client(test_keys):
    """Create AIMClient instance for testing"""
    return AIMClient(
        agent_id="550e8400-e29b-41d4-a716-446655440000",
        public_key=test_keys['public_key'],
        private_key=test_keys['private_key'],
        aim_url="https://aim.example.com",
        timeout=10,
        auto_retry=False
    )


class TestClientInitialization:
    """Test AIMClient initialization and configuration"""

    def test_init_success(self, test_keys):
        """Test successful client initialization"""
        client = AIMClient(
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            public_key=test_keys['public_key'],
            private_key=test_keys['private_key'],
            aim_url="https://aim.example.com"
        )
        assert client.agent_id == "550e8400-e29b-41d4-a716-446655440000"
        assert client.aim_url == "https://aim.example.com"
        assert client.public_key == test_keys['public_key']

    def test_init_strips_trailing_slash(self, test_keys):
        """Test that AIM URL trailing slash is removed"""
        client = AIMClient(
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            public_key=test_keys['public_key'],
            private_key=test_keys['private_key'],
            aim_url="https://aim.example.com/"
        )
        assert client.aim_url == "https://aim.example.com"

    def test_init_missing_agent_id(self, test_keys):
        """Test initialization fails without agent_id"""
        with pytest.raises(ConfigurationError, match="agent_id is required"):
            AIMClient(
                agent_id="",
                public_key=test_keys['public_key'],
                private_key=test_keys['private_key'],
                aim_url="https://aim.example.com"
            )

    def test_init_missing_public_key(self, test_keys):
        """Test initialization fails without public_key"""
        with pytest.raises(ConfigurationError, match="Either api_key OR.*public_key.*private_key.*is required"):
            AIMClient(
                agent_id="550e8400-e29b-41d4-a716-446655440000",
                public_key="",
                private_key=test_keys['private_key'],
                aim_url="https://aim.example.com"
            )

    def test_init_invalid_private_key(self, test_keys):
        """Test initialization fails with invalid private key"""
        with pytest.raises(ConfigurationError, match="Invalid private key format"):
            AIMClient(
                agent_id="550e8400-e29b-41d4-a716-446655440000",
                public_key=test_keys['public_key'],
                private_key="invalid-base64",
                aim_url="https://aim.example.com"
            )

    def test_init_mismatched_keys(self, test_keys):
        """Test initialization fails when public/private keys don't match"""
        # Generate a different key pair
        other_signing_key = SigningKey.generate()
        other_public_key = other_signing_key.verify_key.encode(encoder=Base64Encoder).decode('utf-8')

        with pytest.raises(ConfigurationError, match="Public key does not match private key"):
            AIMClient(
                agent_id="550e8400-e29b-41d4-a716-446655440000",
                public_key=other_public_key,  # Different public key
                private_key=test_keys['private_key'],
                aim_url="https://aim.example.com"
            )


class TestSigning:
    """Test Ed25519 message signing"""

    def test_sign_message(self, aim_client, test_keys):
        """Test message signing produces valid signature"""
        message = "test message"
        signature = aim_client._sign_message(message)

        # Verify signature is base64 encoded
        assert isinstance(signature, str)
        signature_bytes = base64.b64decode(signature)
        assert len(signature_bytes) == 64  # Ed25519 signature is 64 bytes

        # Verify signature is valid
        from nacl.signing import VerifyKey
        verify_key = VerifyKey(test_keys['signing_key'].verify_key.encode())
        verify_key.verify(message.encode('utf-8'), signature_bytes)


class TestVerifyAction:
    """Test action verification flow"""

    @responses.activate
    def test_verify_action_auto_approved(self, aim_client):
        """Test action verification with auto-approval"""
        # Mock successful verification response
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications",
            json={
                "id": "verification-123",
                "status": "approved",
                "approved_by": "system",
                "expires_at": "2025-10-07T13:00:00Z"
            },
            status=200
        )

        result = aim_client.verify_action(
            action_type="read_database",
            resource="users_table",
            context={"query": "SELECT * FROM users"}
        )

        assert result["verified"] is True
        assert result["verification_id"] == "verification-123"
        assert result["approved_by"] == "system"

    @responses.activate
    def test_verify_action_denied(self, aim_client):
        """Test action verification with denial"""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications",
            json={
                "id": "verification-123",
                "status": "denied",
                "denial_reason": "Insufficient permissions"
            },
            status=200
        )

        with pytest.raises(ActionDeniedError, match="Insufficient permissions"):
            aim_client.verify_action(
                action_type="delete_database",
                resource="production_db"
            )

    @responses.activate
    def test_verify_action_pending_then_approved(self, aim_client):
        """Test action verification with pending status that gets approved"""
        # First request returns pending
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications",
            json={
                "id": "verification-123",
                "status": "pending"
            },
            status=200
        )

        # Subsequent polls return approved
        responses.add(
            responses.GET,
            "https://aim.example.com/api/v1/verifications/verification-123",
            json={
                "id": "verification-123",
                "status": "approved",
                "approved_by": "admin@example.com",
                "expires_at": "2025-10-07T13:00:00Z"
            },
            status=200
        )

        result = aim_client.verify_action(
            action_type="send_email",
            resource="admin@example.com",
            timeout_seconds=10
        )

        assert result["verified"] is True
        assert result["approved_by"] == "admin@example.com"

    @responses.activate
    def test_verify_action_authentication_error(self, aim_client):
        """Test action verification with authentication failure"""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications",
            json={"error": "Unauthorized"},
            status=401
        )

        with pytest.raises(AuthenticationError, match="Authentication failed"):
            aim_client.verify_action(
                action_type="read_database",
                resource="users_table"
            )


class TestLogActionResult:
    """Test action result logging"""

    @responses.activate
    def test_log_success(self, aim_client):
        """Test logging successful action result"""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications/verification-123/result",
            json={"status": "logged"},
            status=200
        )

        # Should not raise exception
        aim_client.log_action_result(
            verification_id="verification-123",
            success=True,
            result_summary="Operation completed successfully"
        )

        assert len(responses.calls) == 1
        request_body = json.loads(responses.calls[0].request.body)
        assert request_body["success"] is True
        assert request_body["result_summary"] == "Operation completed successfully"

    @responses.activate
    def test_log_failure(self, aim_client):
        """Test logging failed action result"""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications/verification-123/result",
            json={"status": "logged"},
            status=200
        )

        aim_client.log_action_result(
            verification_id="verification-123",
            success=False,
            error_message="Database connection failed"
        )

        assert len(responses.calls) == 1
        request_body = json.loads(responses.calls[0].request.body)
        assert request_body["success"] is False
        assert request_body["error_message"] == "Database connection failed"

    @responses.activate
    def test_log_ignores_errors(self, aim_client):
        """Test that logging errors don't raise exceptions"""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications/verification-123/result",
            json={"error": "Internal server error"},
            status=500
        )

        # Should not raise exception even on failure
        aim_client.log_action_result(
            verification_id="verification-123",
            success=True
        )


class TestPerformActionDecorator:
    """Test @perform_action decorator"""

    @responses.activate
    def test_decorator_success(self, aim_client):
        """Test decorator with successful verification and execution"""
        # Mock verification approval
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications",
            json={
                "id": "verification-123",
                "status": "approved",
                "approved_by": "system",
                "expires_at": "2025-10-07T13:00:00Z"
            },
            status=200
        )

        # Mock result logging
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications/verification-123/result",
            json={"status": "logged"},
            status=200
        )

        @aim_client.perform_action("read_database", resource="users_table")
        def get_users():
            return {"users": [{"id": 1, "name": "Alice"}]}

        result = get_users()

        assert result == {"users": [{"id": 1, "name": "Alice"}]}
        assert len(responses.calls) == 2  # Verification + logging

    @responses.activate
    def test_decorator_action_denied(self, aim_client):
        """Test decorator when action is denied"""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications",
            json={
                "id": "verification-123",
                "status": "denied",
                "denial_reason": "Policy violation"
            },
            status=200
        )

        @aim_client.perform_action("delete_database", resource="production")
        def dangerous_action():
            return "should not execute"

        with pytest.raises(ActionDeniedError, match="Policy violation"):
            dangerous_action()

    @responses.activate
    def test_decorator_logs_execution_error(self, aim_client):
        """Test decorator logs errors when function fails"""
        # Mock verification approval
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications",
            json={
                "id": "verification-123",
                "status": "approved",
                "approved_by": "system",
                "expires_at": "2025-10-07T13:00:00Z"
            },
            status=200
        )

        # Mock result logging
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/verifications/verification-123/result",
            json={"status": "logged"},
            status=200
        )

        @aim_client.perform_action("read_database", resource="users_table")
        def failing_function():
            raise ValueError("Database connection failed")

        with pytest.raises(ValueError, match="Database connection failed"):
            failing_function()

        # Verify error was logged
        assert len(responses.calls) == 2
        log_request = json.loads(responses.calls[1].request.body)
        assert log_request["success"] is False
        assert "Database connection failed" in log_request["error_message"]


class TestContextManager:
    """Test context manager support"""

    def test_context_manager(self, test_keys):
        """Test client works as context manager"""
        with AIMClient(
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            public_key=test_keys['public_key'],
            private_key=test_keys['private_key'],
            aim_url="https://aim.example.com"
        ) as client:
            assert client.agent_id == "550e8400-e29b-41d4-a716-446655440000"

        # Session should be closed after context
        assert client.session is not None
