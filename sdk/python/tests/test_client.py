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
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={
                "id": "verification-123",
                "status": "approved",
                "approvedBy": "system",
                "expiresAt": "2025-10-07T13:00:00Z"
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
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={
                "id": "verification-123",
                "status": "denied",
                "denialReason": "Insufficient permissions"
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
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={
                "id": "verification-123",
                "status": "pending"
            },
            status=200
        )

        # Subsequent polls return approved
        responses.add(
            responses.GET,
            "https://aim.example.com/api/v1/sdk-api/verifications/verification-123",
            json={
                "id": "verification-123",
                "status": "approved",
                "approvedBy": "admin@example.com",
                "expiresAt": "2025-10-07T13:00:00Z"
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
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={"error": "Unauthorized"},
            status=401
        )

        with pytest.raises(AuthenticationError, match="Authentication failed"):
            aim_client.verify_action(
                action_type="read_database",
                resource="users_table"
            )

    @responses.activate
    def test_verify_action_poll_sends_signed_headers(self, aim_client, test_keys):
        """Defect #160: the verification poll must Ed25519-sign each GET.

        Asserts the three X-AIM-* headers are present, the timestamp is fresh,
        and the signature verifies against the canonical GET message using the
        agent's registered public key.
        """
        import time as _time
        from nacl.signing import VerifyKey
        from nacl.encoding import Base64Encoder

        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={"id": "verification-123", "status": "pending"},
            status=200,
        )
        responses.add(
            responses.GET,
            "https://aim.example.com/api/v1/sdk-api/verifications/verification-123",
            json={
                "id": "verification-123",
                "status": "approved",
                "approvedBy": "system",
            },
            status=200,
        )

        result = aim_client.verify_action(
            action_type="send_email",
            resource="user@example.com",
            timeout_seconds=10,
        )
        assert result["verified"] is True

        get_calls = [c for c in responses.calls if c.request.method == "GET"]
        assert get_calls, "expected at least one GET poll"
        poll = get_calls[0]
        assert "X-AIM-Agent-ID" in poll.request.headers
        assert "X-AIM-Timestamp" in poll.request.headers
        assert "X-AIM-Signature" in poll.request.headers

        agent_id_header = poll.request.headers["X-AIM-Agent-ID"]
        ts_header = poll.request.headers["X-AIM-Timestamp"]
        sig_header = poll.request.headers["X-AIM-Signature"]

        assert agent_id_header == aim_client.agent_id
        # Timestamp must be fresh: within +/- 5 minutes of now.
        assert abs(int(ts_header) - int(_time.time())) <= 300

        canonical = (
            f"GET\n/api/v1/sdk-api/verifications/verification-123\n"
            f"{agent_id_header}\n{ts_header}"
        )
        verify_key = VerifyKey(test_keys['signing_key'].verify_key.encode())
        # Will raise nacl.exceptions.BadSignatureError on mismatch.
        verify_key.verify(canonical.encode("utf-8"), base64.b64decode(sig_header))

    def test_verify_action_poll_fails_without_signing_key(self, aim_client):
        """Defect #160: clients without a signing_key cannot poll.

        Calls _wait_for_approval directly so we bypass the POST-step's existing
        Ed25519 requirement and isolate the new poll-step requirement.
        """
        aim_client.signing_key = None
        with pytest.raises(VerificationError, match="signing key"):
            aim_client._wait_for_approval("verification-123", timeout_seconds=5)

    @responses.activate
    def test_verify_action_poll_normalizes_uppercase_uuid(self, aim_client, test_keys):
        """Defect #160 (UUID-case drift): the SDK must canonicalize UUIDs to
        lowercase before signing so the canonical bytes match the backend's
        (Go uuid.String() always lowercase). If the caller's agent_id or the
        server-returned verification_id is uppercase, the request must still
        verify server-side.
        """
        import time as _time
        from nacl.signing import VerifyKey

        # Force uppercase on the client's stored agent_id.
        upper_agent_id = aim_client.agent_id.upper()
        aim_client.agent_id = upper_agent_id

        # Backend returns the verification_id in uppercase too.
        upper_vid = "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA"

        # The SDK should rewrite the URL to lowercase; register the lowercase form.
        lower_vid = upper_vid.lower()
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={"id": upper_vid, "status": "pending"},
            status=200,
        )
        responses.add(
            responses.GET,
            f"https://aim.example.com/api/v1/sdk-api/verifications/{lower_vid}",
            json={"id": upper_vid, "status": "approved", "approvedBy": "system"},
            status=200,
        )

        result = aim_client.verify_action(
            action_type="send_email",
            resource="user@example.com",
            timeout_seconds=10,
        )
        assert result["verified"] is True

        get_calls = [c for c in responses.calls if c.request.method == "GET"]
        assert get_calls, "expected GET poll"
        poll = get_calls[0]
        # URL must be lowercased.
        assert lower_vid in poll.request.url
        assert upper_vid not in poll.request.url
        # Header must be lowercased.
        assert poll.request.headers["X-AIM-Agent-ID"] == upper_agent_id.lower()

        ts_header = poll.request.headers["X-AIM-Timestamp"]
        sig_header = poll.request.headers["X-AIM-Signature"]
        # Signature must verify against the LOWERCASE canonical (what backend builds).
        canonical = (
            f"GET\n/api/v1/sdk-api/verifications/{lower_vid}\n"
            f"{upper_agent_id.lower()}\n{ts_header}"
        )
        verify_key = VerifyKey(test_keys['signing_key'].verify_key.encode())
        verify_key.verify(canonical.encode("utf-8"), base64.b64decode(sig_header))


class TestLogActionResult:
    """Test action result logging"""

    @responses.activate
    def test_log_success(self, aim_client):
        """Test logging successful action result"""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications/verification-123/result",
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
        assert request_body["result"] == "success"
        assert request_body["result_summary"] == "Operation completed successfully"

    @responses.activate
    def test_log_failure(self, aim_client):
        """Test logging failed action result"""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications/verification-123/result",
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
        assert request_body["result"] == "failure"
        assert request_body["error_message"] == "Database connection failed"

    @responses.activate
    def test_log_ignores_errors(self, aim_client):
        """Test that logging errors don't raise exceptions"""
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications/verification-123/result",
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
        # Mock capability auto-registration (happens first on decorated function call)
        responses.add(
            responses.POST,
            f"https://aim.example.com/api/v1/sdk-api/agents/{aim_client.agent_id}/capabilities/register",
            json={"status": "granted", "message": "Capability registered"},
            status=200
        )

        # Mock verification approval
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={
                "id": "verification-123",
                "status": "approved",
                "approvedBy": "system",
                "expiresAt": "2025-10-07T13:00:00Z"
            },
            status=200
        )

        # Mock result logging
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications/verification-123/result",
            json={"status": "logged"},
            status=200
        )

        @aim_client.perform_action("db:read", resource="users_table")
        def get_users():
            return {"users": [{"id": 1, "name": "Alice"}]}

        result = get_users()

        assert result == {"users": [{"id": 1, "name": "Alice"}]}
        assert len(responses.calls) == 3  # Auto-registration + Verification + logging

    @responses.activate
    def test_decorator_action_denied(self, aim_client):
        """Test decorator when action is denied"""
        # Mock capability auto-registration
        responses.add(
            responses.POST,
            f"https://aim.example.com/api/v1/sdk-api/agents/{aim_client.agent_id}/capabilities/register",
            json={"status": "granted", "message": "Capability registered"},
            status=200
        )

        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={
                "id": "verification-123",
                "status": "denied",
                "denialReason": "Policy violation"
            },
            status=200
        )

        @aim_client.perform_action("db:delete", resource="production")
        def dangerous_action():
            return "should not execute"

        with pytest.raises(ActionDeniedError, match="Policy violation"):
            dangerous_action()

    @responses.activate
    def test_decorator_logs_execution_error(self, aim_client):
        """Test decorator logs errors when function fails"""
        # Mock capability auto-registration
        responses.add(
            responses.POST,
            f"https://aim.example.com/api/v1/sdk-api/agents/{aim_client.agent_id}/capabilities/register",
            json={"status": "granted", "message": "Capability registered"},
            status=200
        )

        # Mock verification approval
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications",
            json={
                "id": "verification-123",
                "status": "approved",
                "approvedBy": "system",
                "expiresAt": "2025-10-07T13:00:00Z"
            },
            status=200
        )

        # Mock result logging
        responses.add(
            responses.POST,
            "https://aim.example.com/api/v1/sdk-api/verifications/verification-123/result",
            json={"status": "logged"},
            status=200
        )

        @aim_client.perform_action("db:read", resource="users_table")
        def failing_function():
            raise ValueError("Database connection failed")

        with pytest.raises(ValueError, match="Database connection failed"):
            failing_function()

        # Verify error was logged
        assert len(responses.calls) == 3  # Auto-registration + Verification + logging
        log_request = json.loads(responses.calls[2].request.body)
        assert log_request["result"] == "failure"
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
