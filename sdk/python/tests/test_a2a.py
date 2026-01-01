"""
Comprehensive tests for A2A (Agent-to-Agent) Protocol Support.

Tests cover:
- Data classes and their fields
- A2AClient methods
- Request signing and verification
- Trust score operations
- Consent management
- Helper functions
"""

import pytest
from datetime import datetime, timezone, timedelta
from unittest.mock import Mock, patch, MagicMock
import json

from aim_sdk.a2a import (
    A2AAgentCard,
    A2ARequestSignature,
    A2ATrustScore,
    A2APeerTrust,
    A2AConsent,
    A2AClient,
    A2AError,
    _parse_datetime,
)


# =========================================================================
# Data Class Tests - A2AAgentCard
# =========================================================================

class TestA2AAgentCard:
    """Tests for A2AAgentCard data class."""

    def test_agent_card_fields(self):
        """Test A2AAgentCard has all expected fields."""
        now = datetime.now(timezone.utc)
        card = A2AAgentCard(
            card_id="card-123",
            agent_id="agent-456",
            card_url="https://example.com/.well-known/agent.json",
            name="Test Agent",
            description="A test agent for A2A protocol",
            version="1.0.0",
            skills=[{"id": "skill-1", "name": "Data Processing"}],
            attestation_expires=now + timedelta(hours=24),
            aim_extensions={"trustScore": 0.85},
        )

        assert card.card_id == "card-123"
        assert card.agent_id == "agent-456"
        assert card.card_url == "https://example.com/.well-known/agent.json"
        assert card.name == "Test Agent"
        assert card.version == "1.0.0"
        assert len(card.skills) == 1
        assert card.aim_extensions["trustScore"] == 0.85

    def test_agent_card_defaults(self):
        """Test A2AAgentCard default values."""
        card = A2AAgentCard(
            card_id="card-123",
            agent_id="agent-456",
            card_url="https://example.com/.well-known/agent.json",
        )

        assert card.name == ""
        assert card.description == ""
        assert card.version == "1.0.0"
        assert card.skills == []
        assert card.attestation_expires is None
        assert card.aim_extensions is None

    def test_agent_card_with_attestation(self):
        """Test A2AAgentCard with attestation expiry."""
        expires = datetime.now(timezone.utc) + timedelta(hours=24)
        card = A2AAgentCard(
            card_id="card-123",
            agent_id="agent-456",
            card_url="https://example.com/.well-known/agent.json",
            attestation_expires=expires,
        )

        assert card.attestation_expires is not None
        assert card.attestation_expires > datetime.now(timezone.utc)


# =========================================================================
# Data Class Tests - A2ARequestSignature
# =========================================================================

class TestA2ARequestSignature:
    """Tests for A2ARequestSignature data class."""

    def test_signature_fields(self):
        """Test A2ARequestSignature has all expected fields."""
        now = int(datetime.now(timezone.utc).timestamp())
        sig = A2ARequestSignature(
            agent_id="agent-123",
            timestamp=now,
            nonce="unique-nonce-12345",
            signature="base64-signature",
            public_key="base64-public-key",
        )

        assert sig.agent_id == "agent-123"
        assert sig.timestamp == now
        assert sig.nonce == "unique-nonce-12345"
        assert sig.signature == "base64-signature"
        assert sig.public_key == "base64-public-key"

    def test_signature_to_headers(self):
        """Test converting signature to HTTP headers."""
        now = int(datetime.now(timezone.utc).timestamp())
        sig = A2ARequestSignature(
            agent_id="agent-123",
            timestamp=now,
            nonce="unique-nonce",
            signature="base64-sig",
            public_key="base64-key",
        )

        headers = sig.to_headers()

        assert headers["X-A2A-Agent-ID"] == "agent-123"
        assert headers["X-A2A-Timestamp"] == str(now)
        assert headers["X-A2A-Nonce"] == "unique-nonce"
        assert headers["X-A2A-Signature"] == "base64-sig"
        assert headers["X-A2A-Public-Key"] == "base64-key"

    def test_signature_headers_without_public_key(self):
        """Test headers without optional public key."""
        sig = A2ARequestSignature(
            agent_id="agent-123",
            timestamp=1234567890,
            nonce="nonce",
            signature="sig",
        )

        headers = sig.to_headers()

        assert "X-A2A-Agent-ID" in headers
        assert "X-A2A-Public-Key" not in headers


# =========================================================================
# Data Class Tests - A2ATrustScore
# =========================================================================

class TestA2ATrustScore:
    """Tests for A2ATrustScore data class."""

    def test_trust_score_fields(self):
        """Test A2ATrustScore has all expected fields."""
        now = datetime.now(timezone.utc)
        score = A2ATrustScore(
            agent_id="agent-123",
            a2a_trust_score=0.85,
            peer_trust_average=0.80,
            unique_peers_count=10,
            tasks_completed=95,
            tasks_failed=5,
            computed_at=now,
        )

        assert score.agent_id == "agent-123"
        assert score.a2a_trust_score == 0.85
        assert score.peer_trust_average == 0.80
        assert score.unique_peers_count == 10
        assert score.tasks_completed == 95
        assert score.tasks_failed == 5

    def test_trust_score_defaults(self):
        """Test A2ATrustScore default values."""
        score = A2ATrustScore(
            agent_id="agent-123",
            a2a_trust_score=0.5,
        )

        assert score.peer_trust_average is None
        assert score.unique_peers_count == 0
        assert score.tasks_completed == 0
        assert score.tasks_failed == 0
        assert score.computed_at is None

    def test_trust_score_success_rate(self):
        """Test calculating success rate from trust score data."""
        score = A2ATrustScore(
            agent_id="agent-123",
            a2a_trust_score=0.9,
            tasks_completed=90,
            tasks_failed=10,
        )

        total = score.tasks_completed + score.tasks_failed
        success_rate = score.tasks_completed / total if total > 0 else 0
        assert success_rate == 0.9


# =========================================================================
# Data Class Tests - A2APeerTrust
# =========================================================================

class TestA2APeerTrust:
    """Tests for A2APeerTrust data class."""

    def test_peer_trust_fields(self):
        """Test A2APeerTrust has all expected fields."""
        now = datetime.now(timezone.utc)
        trust = A2APeerTrust(
            agent_id="agent-123",
            peer_agent_id="peer-456",
            peer_trust_score=0.88,
            success_rate=0.92,
            tasks_initiated=50,
            tasks_received=30,
            first_interaction_at=now - timedelta(days=30),
            last_interaction_at=now,
        )

        assert trust.agent_id == "agent-123"
        assert trust.peer_agent_id == "peer-456"
        assert trust.peer_trust_score == 0.88
        assert trust.success_rate == 0.92
        assert trust.tasks_initiated == 50
        assert trust.tasks_received == 30

    def test_peer_trust_defaults(self):
        """Test A2APeerTrust default values."""
        trust = A2APeerTrust(
            agent_id="agent-123",
            peer_agent_id="peer-456",
            peer_trust_score=0.5,
        )

        assert trust.success_rate is None
        assert trust.tasks_initiated == 0
        assert trust.tasks_received == 0
        assert trust.first_interaction_at is None
        assert trust.last_interaction_at is None


# =========================================================================
# Data Class Tests - A2AConsent
# =========================================================================

class TestA2AConsent:
    """Tests for A2AConsent data class."""

    def test_consent_fields(self):
        """Test A2AConsent has all expected fields."""
        now = datetime.now(timezone.utc)
        expires = now + timedelta(days=365)

        consent = A2AConsent(
            consent_id="consent-123",
            user_id="user-456",
            grantor_agent_id="agent-grantor",
            recipient_agent_id="agent-recipient",
            scope=["read:profile", "write:preferences"],
            purpose="data-sharing",
            data_types=["profile", "preferences"],
            granted_at=now,
            expires_at=expires,
            revoked=False,
        )

        assert consent.consent_id == "consent-123"
        assert consent.user_id == "user-456"
        assert consent.grantor_agent_id == "agent-grantor"
        assert consent.recipient_agent_id == "agent-recipient"
        assert len(consent.scope) == 2
        assert consent.purpose == "data-sharing"
        assert not consent.revoked

    def test_consent_defaults(self):
        """Test A2AConsent default values."""
        now = datetime.now(timezone.utc)
        consent = A2AConsent(
            consent_id="consent-123",
            user_id="user-456",
            grantor_agent_id="agent-grantor",
            recipient_agent_id="agent-recipient",
            scope=["read"],
            purpose="data-sharing",
            data_types=["profile"],
            granted_at=now,
        )

        assert consent.expires_at is None
        assert consent.revoked is False

    def test_consent_is_active(self):
        """Test checking if consent is active."""
        now = datetime.now(timezone.utc)

        # Active consent (not expired, not revoked)
        active_consent = A2AConsent(
            consent_id="consent-1",
            user_id="user",
            grantor_agent_id="agent-1",
            recipient_agent_id="agent-2",
            scope=["read"],
            purpose="test",
            data_types=["data"],
            granted_at=now,
            expires_at=now + timedelta(hours=24),
            revoked=False,
        )
        is_active = not active_consent.revoked
        if active_consent.expires_at:
            is_active = is_active and active_consent.expires_at > now
        assert is_active is True

        # Expired consent
        expired_consent = A2AConsent(
            consent_id="consent-2",
            user_id="user",
            grantor_agent_id="agent-1",
            recipient_agent_id="agent-2",
            scope=["read"],
            purpose="test",
            data_types=["data"],
            granted_at=now - timedelta(days=2),
            expires_at=now - timedelta(hours=24),
            revoked=False,
        )
        is_active = not expired_consent.revoked
        if expired_consent.expires_at:
            is_active = is_active and expired_consent.expires_at > now
        assert is_active is False

        # Revoked consent
        revoked_consent = A2AConsent(
            consent_id="consent-3",
            user_id="user",
            grantor_agent_id="agent-1",
            recipient_agent_id="agent-2",
            scope=["read"],
            purpose="test",
            data_types=["data"],
            granted_at=now,
            expires_at=now + timedelta(hours=24),
            revoked=True,
        )
        assert revoked_consent.revoked is True


# =========================================================================
# A2AError Tests
# =========================================================================

class TestA2AError:
    """Tests for A2AError exception."""

    def test_error_with_message(self):
        """Test A2AError with message only."""
        error = A2AError("Something went wrong")
        assert str(error) == "Something went wrong"
        assert error.status_code is None

    def test_error_with_status_code(self):
        """Test A2AError with status code."""
        error = A2AError("Not found", status_code=404)
        assert str(error) == "Not found"
        assert error.status_code == 404

    def test_error_is_exception(self):
        """Test A2AError can be raised and caught."""
        with pytest.raises(A2AError) as exc_info:
            raise A2AError("Test error", status_code=500)

        assert "Test error" in str(exc_info.value)
        assert exc_info.value.status_code == 500


# =========================================================================
# Helper Function Tests
# =========================================================================

class TestParseDatetime:
    """Tests for _parse_datetime helper function."""

    def test_parse_iso_datetime(self):
        """Test parsing standard ISO datetime."""
        result = _parse_datetime("2024-01-15T10:30:00+00:00")
        assert result is not None
        assert result.year == 2024
        assert result.month == 1
        assert result.day == 15

    def test_parse_datetime_with_z(self):
        """Test parsing datetime with Z suffix."""
        result = _parse_datetime("2024-01-15T10:30:00Z")
        assert result is not None
        assert result.year == 2024

    def test_parse_none(self):
        """Test parsing None returns None."""
        assert _parse_datetime(None) is None

    def test_parse_empty_string(self):
        """Test parsing empty string returns None."""
        assert _parse_datetime("") is None

    def test_parse_invalid_format(self):
        """Test parsing invalid format returns None."""
        assert _parse_datetime("not-a-date") is None


# =========================================================================
# A2AClient Tests
# =========================================================================

class TestA2AClient:
    """Tests for A2AClient class."""

    @pytest.fixture
    def mock_aim_client(self):
        """Create a mock AIM client for testing."""
        aim = Mock()
        aim.agent_id = "test-agent-123"
        aim.aim_url = "http://localhost:8080"
        aim.api_key = "test-api-key"
        aim.signing_key = None
        aim.public_key = None
        return aim

    @pytest.fixture
    def a2a_client(self, mock_aim_client):
        """Create an A2A client with mock AIM client."""
        return A2AClient(mock_aim_client)

    def test_client_initialization(self, a2a_client, mock_aim_client):
        """Test A2AClient initialization."""
        assert a2a_client.agent_id == "test-agent-123"
        assert a2a_client.aim_url == "http://localhost:8080"
        assert a2a_client.timeout == 30

    def test_client_custom_timeout(self, mock_aim_client):
        """Test A2AClient with custom timeout."""
        client = A2AClient(mock_aim_client, timeout=60)
        assert client.timeout == 60

    @patch.object(A2AClient, '_make_request')
    def test_register_agent_card(self, mock_request, a2a_client):
        """Test registering an agent card."""
        mock_request.return_value = {
            "cardId": "card-123",
            "agentId": "test-agent-123",
            "skills": ["skill-1", "skill-2"],
            "attestationExpires": "2024-12-31T23:59:59Z",
        }

        card = a2a_client.register_agent_card("https://example.com/.well-known/agent.json")

        assert card.card_id == "card-123"
        assert card.agent_id == "test-agent-123"
        assert card.card_url == "https://example.com/.well-known/agent.json"
        assert len(card.skills) == 2
        mock_request.assert_called_once()

    @patch.object(A2AClient, '_make_request')
    def test_get_agent_card(self, mock_request, a2a_client):
        """Test getting an agent card."""
        mock_request.return_value = {
            "name": "Test Agent",
            "version": "1.0.0",
            "aim": {"trustScore": 0.85},
        }

        card = a2a_client.get_agent_card()

        assert card["name"] == "Test Agent"
        assert card["aim"]["trustScore"] == 0.85
        mock_request.assert_called_once_with("GET", "/api/v1/a2a/agents/test-agent-123/card")

    @patch.object(A2AClient, '_make_request')
    def test_sign_request(self, mock_request, a2a_client):
        """Test signing a request."""
        mock_request.return_value = {
            "agentId": "test-agent-123",
            "timestamp": 1234567890,
            "nonce": "unique-nonce",
            "signature": "base64-signature",
        }

        sig = a2a_client.sign_request("POST", "/tasks/send", {"message": "hello"})

        assert isinstance(sig, A2ARequestSignature)
        assert sig.agent_id == "test-agent-123"
        assert sig.nonce == "unique-nonce"
        assert sig.signature == "base64-signature"

    @patch.object(A2AClient, '_make_request')
    def test_verify_request(self, mock_request, a2a_client):
        """Test verifying a request."""
        mock_request.return_value = {
            "valid": True,
            "agentName": "Remote Agent",
            "trustScore": 0.9,
        }

        result = a2a_client.verify_request(
            agent_id="remote-agent",
            timestamp=1234567890,
            nonce="nonce",
            signature="sig",
            request_hash="hash",
        )

        assert result["valid"] is True
        assert result["agentName"] == "Remote Agent"

    @patch.object(A2AClient, '_make_request')
    def test_get_a2a_trust_score(self, mock_request, a2a_client):
        """Test getting A2A trust score."""
        mock_request.return_value = {
            "a2aTrustScore": 0.85,
            "peerTrustAverage": 0.80,
            "uniquePeersCount": 10,
            "tasksCompleted": 95,
            "tasksFailed": 5,
        }

        score = a2a_client.get_a2a_trust_score()

        assert isinstance(score, A2ATrustScore)
        assert score.a2a_trust_score == 0.85
        assert score.peer_trust_average == 0.80
        assert score.unique_peers_count == 10

    @patch.object(A2AClient, '_make_request')
    def test_get_peer_trust(self, mock_request, a2a_client):
        """Test getting peer trust."""
        mock_request.return_value = {
            "peerTrustScore": 0.88,
            "successRate": 0.92,
            "tasksInitiated": 50,
            "tasksReceived": 30,
        }

        trust = a2a_client.get_peer_trust("peer-agent-123")

        assert isinstance(trust, A2APeerTrust)
        assert trust.peer_trust_score == 0.88
        assert trust.success_rate == 0.92

    @patch.object(A2AClient, '_make_request')
    def test_get_skills(self, mock_request, a2a_client):
        """Test getting agent skills."""
        mock_request.return_value = {
            "skills": [
                {"id": "skill-1", "name": "Data Analysis"},
                {"id": "skill-2", "name": "Text Processing"},
            ]
        }

        skills = a2a_client.get_skills()

        assert len(skills) == 2
        assert skills[0]["name"] == "Data Analysis"

    @patch.object(A2AClient, '_make_request')
    def test_search_skills(self, mock_request, a2a_client):
        """Test searching skills."""
        mock_request.return_value = {
            "skills": [
                {"id": "skill-1", "name": "Data Analysis"},
            ]
        }

        skills = a2a_client.search_skills("data")

        assert len(skills) == 1
        mock_request.assert_called_with("GET", "/api/v1/a2a/skills/search?q=data&limit=20")

    @patch.object(A2AClient, '_make_request')
    def test_log_task(self, mock_request, a2a_client):
        """Test logging a task."""
        mock_request.return_value = {
            "id": "task-123",
            "state": "SUBMITTED",
        }

        result = a2a_client.log_task(
            external_task_id="ext-123",
            remote_agent_id="remote-agent",
            skill_id="skill-1",
        )

        assert result["id"] == "task-123"
        assert result["state"] == "SUBMITTED"

    @patch.object(A2AClient, '_make_request')
    def test_update_task_state(self, mock_request, a2a_client):
        """Test updating task state."""
        mock_request.return_value = {"success": True}

        result = a2a_client.update_task_state("task-123", "COMPLETED")

        assert result["success"] is True
        mock_request.assert_called_once()

    @patch.object(A2AClient, '_make_request')
    def test_update_task_state_with_error(self, mock_request, a2a_client):
        """Test updating task state with error details."""
        mock_request.return_value = {"success": True}

        result = a2a_client.update_task_state(
            "task-123", "FAILED",
            error_code="TIMEOUT",
            error_message="Request timed out"
        )

        assert result["success"] is True
        mock_request.assert_called_with(
            "PUT",
            "/api/v1/a2a/tasks/task-123/state",
            data={
                "state": "FAILED",
                "errorCode": "TIMEOUT",
                "errorMessage": "Request timed out"
            }
        )

    @patch.object(A2AClient, '_make_request')
    def test_get_task_history(self, mock_request, a2a_client):
        """Test getting task history."""
        mock_request.return_value = {
            "tasks": [
                {"taskId": "task-1", "status": "COMPLETED"},
                {"taskId": "task-2", "status": "FAILED"},
            ]
        }

        result = a2a_client.get_task_history("target-agent-123", limit=10)

        assert len(result) == 2
        assert result[0]["taskId"] == "task-1"
        mock_request.assert_called_with(
            "GET",
            "/api/v1/a2a/tasks/target-agent-123?limit=10"
        )

    @patch.object(A2AClient, '_make_request')
    def test_compute_a2a_trust_score(self, mock_request, a2a_client):
        """Test computing A2A trust score."""
        mock_request.return_value = {
            "a2aTrustScore": 0.92,
            "peerTrustAverage": 0.88,
            "uniquePeersCount": 15,
            "tasksCompleted": 120,
            "tasksFailed": 5,
            "computedAt": "2024-01-15T10:30:00Z",
        }

        score = a2a_client.compute_a2a_trust_score()

        assert isinstance(score, A2ATrustScore)
        assert score.a2a_trust_score == 0.92
        assert score.unique_peers_count == 15
        mock_request.assert_called_with(
            "POST",
            "/api/v1/a2a/agents/test-agent-123/trust-score/compute"
        )

    @patch.object(A2AClient, '_make_request')
    def test_record_consent(self, mock_request, a2a_client):
        """Test recording consent."""
        mock_request.return_value = {
            "id": "consent-123",
            "grantedAt": "2024-01-15T10:30:00Z",
            "expiresAt": "2024-01-16T10:30:00Z",
        }

        consent = a2a_client.record_consent(
            user_id="user-123",
            recipient_agent_id="recipient-agent",
            scope=["pii"],
            purpose="Process order",
            data_types=["email", "address"],
        )

        assert isinstance(consent, A2AConsent)
        assert consent.consent_id == "consent-123"
        assert consent.purpose == "Process order"

    @patch.object(A2AClient, '_make_request')
    def test_check_consent(self, mock_request, a2a_client):
        """Test checking consent."""
        mock_request.return_value = {"hasConsent": True}

        result = a2a_client.check_consent("user-123", "recipient-agent", "pii")

        assert result is True

    @patch.object(A2AClient, '_make_request')
    def test_revoke_consent(self, mock_request, a2a_client):
        """Test revoking consent."""
        mock_request.return_value = {"success": True}

        result = a2a_client.revoke_consent("consent-123", "User request")

        assert result["success"] is True
        mock_request.assert_called_with(
            "POST",
            "/api/v1/a2a/consent/consent-123/revoke",
            data={"reason": "User request"}
        )

    @patch.object(A2AClient, '_make_request')
    def test_list_user_consents(self, mock_request, a2a_client):
        """Test listing user consents."""
        mock_request.return_value = {
            "consents": [
                {
                    "id": "consent-1",
                    "grantorAgentId": "agent-1",
                    "recipientAgentId": "agent-2",
                    "scope": ["read"],
                    "purpose": "test",
                    "dataTypes": ["data"],
                    "grantedAt": "2024-01-15T10:30:00Z",
                    "revoked": False,
                }
            ]
        }

        consents = a2a_client.list_user_consents("user-123")

        assert len(consents) == 1
        assert consents[0].consent_id == "consent-1"

    @patch.object(A2AClient, '_make_request')
    def test_evaluate_policy(self, mock_request, a2a_client):
        """Test evaluating policy."""
        mock_request.return_value = {
            "decision": "ALLOW",
            "policiesEvaluated": ["trust-policy"],
        }

        result = a2a_client.evaluate_policy(
            event="task.create",
            remote_agent_id="remote-agent",
        )

        assert result["decision"] == "ALLOW"


# =========================================================================
# Edge Cases and Error Handling
# =========================================================================

class TestEdgeCases:
    """Tests for edge cases and error handling."""

    @pytest.fixture
    def mock_aim_client(self):
        """Create a mock AIM client."""
        aim = Mock()
        aim.agent_id = "test-agent"
        aim.aim_url = "http://localhost:8080"
        aim.api_key = "test-key"
        aim.signing_key = None
        aim.public_key = None
        return aim

    def test_empty_skills_list(self):
        """Test agent card with empty skills list."""
        card = A2AAgentCard(
            card_id="card-1",
            agent_id="agent-1",
            card_url="https://example.com/agent.json",
        )
        assert card.skills == []

    def test_zero_trust_score(self):
        """Test trust score of zero."""
        score = A2ATrustScore(
            agent_id="agent-1",
            a2a_trust_score=0.0,
        )
        assert score.a2a_trust_score == 0.0

    def test_max_trust_score(self):
        """Test maximum trust score."""
        score = A2ATrustScore(
            agent_id="agent-1",
            a2a_trust_score=1.0,
        )
        assert score.a2a_trust_score == 1.0

    def test_consent_with_no_expiry(self):
        """Test consent without expiry (perpetual)."""
        consent = A2AConsent(
            consent_id="consent-1",
            user_id="user-1",
            grantor_agent_id="agent-1",
            recipient_agent_id="agent-2",
            scope=["read"],
            purpose="test",
            data_types=["data"],
            granted_at=datetime.now(timezone.utc),
        )
        assert consent.expires_at is None

    def test_signature_with_empty_nonce(self):
        """Test signature with empty nonce is invalid."""
        sig = A2ARequestSignature(
            agent_id="agent-1",
            timestamp=1234567890,
            nonce="",
            signature="sig",
        )
        assert sig.nonce == ""

    @patch('requests.Session.request')
    def test_request_error_handling(self, mock_request, mock_aim_client):
        """Test A2A error handling on failed request."""
        mock_response = Mock()
        mock_response.ok = False
        mock_response.status_code = 500
        mock_response.text = "Internal Server Error"
        mock_response.json.side_effect = ValueError("No JSON")
        mock_request.return_value = mock_response

        client = A2AClient(mock_aim_client)

        with pytest.raises(A2AError) as exc_info:
            client._make_request("GET", "/api/v1/test")

        assert exc_info.value.status_code == 500

    def test_peer_trust_with_no_interactions(self):
        """Test peer trust with no interaction history."""
        trust = A2APeerTrust(
            agent_id="agent-1",
            peer_agent_id="agent-2",
            peer_trust_score=0.5,
            tasks_initiated=0,
            tasks_received=0,
        )
        assert trust.tasks_initiated == 0
        assert trust.tasks_received == 0


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
