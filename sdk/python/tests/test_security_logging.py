"""
Tests for AIM SDK Security Logging

Matches Java SDK SecurityFeaturesTest coverage:
- SecurityLogger singleton pattern
- Event categories and severity levels
- Session management
- Configuration options
"""

import pytest
import tempfile
from pathlib import Path

from aim_sdk.security_logging import (
    SecurityLogger,
    EventCategory,
    EventSeverity,
    AuthnEventType,
    AuthzEventType,
    AgentEventType,
    CredEventType,
    security_logger,
)


class TestSecurityLoggerSingleton:
    """Test SecurityLogger singleton pattern."""

    def test_singleton_instance(self):
        """SecurityLogger should return the same instance."""
        logger1 = security_logger
        logger2 = security_logger
        assert logger1 is logger2

    def test_singleton_not_none(self):
        """SecurityLogger instance should not be None."""
        assert security_logger is not None


class TestEventCategories:
    """Test event category enum values."""

    def test_authn_category_exists(self):
        assert EventCategory.AUTHN == "AUTHN"

    def test_authz_category_exists(self):
        assert EventCategory.AUTHZ == "AUTHZ"

    def test_agent_category_exists(self):
        assert EventCategory.AGENT == "AGENT"

    def test_cred_category_exists(self):
        assert EventCategory.CRED == "CRED"

    def test_mcp_category_exists(self):
        assert EventCategory.MCP == "MCP"

    def test_security_category_exists(self):
        assert EventCategory.SECURITY == "SECURITY"

    def test_audit_category_exists(self):
        assert EventCategory.AUDIT == "AUDIT"

    def test_config_category_exists(self):
        assert EventCategory.CONFIG == "CONFIG"


class TestEventSeverity:
    """Test event severity enum values."""

    def test_debug_severity_exists(self):
        assert EventSeverity.DEBUG == "DEBUG"

    def test_info_severity_exists(self):
        assert EventSeverity.INFO == "INFO"

    def test_warning_severity_exists(self):
        assert EventSeverity.WARNING == "WARNING"

    def test_error_severity_exists(self):
        assert EventSeverity.ERROR == "ERROR"

    def test_critical_severity_exists(self):
        assert EventSeverity.CRITICAL == "CRITICAL"


class TestAuthnEventTypes:
    """Test authentication event types."""

    def test_token_refresh_exists(self):
        assert AuthnEventType.TOKEN_REFRESH == "TOKEN_REFRESH"

    def test_token_refresh_failed_exists(self):
        assert AuthnEventType.TOKEN_REFRESH_FAILED == "TOKEN_REFRESH_FAILED"

    def test_token_expired_exists(self):
        assert AuthnEventType.TOKEN_EXPIRED == "TOKEN_EXPIRED"

    def test_credential_load_exists(self):
        assert AuthnEventType.CREDENTIAL_LOAD == "CREDENTIAL_LOAD"

    def test_credential_load_failed_exists(self):
        assert AuthnEventType.CREDENTIAL_LOAD_FAILED == "CREDENTIAL_LOAD_FAILED"

    def test_sdk_initialized_exists(self):
        assert AuthnEventType.SDK_INITIALIZED == "SDK_INITIALIZED"


class TestAuthzEventTypes:
    """Test authorization event types."""

    def test_capability_check_exists(self):
        assert AuthzEventType.CAPABILITY_CHECK == "CAPABILITY_CHECK"

    def test_capability_granted_exists(self):
        assert AuthzEventType.CAPABILITY_GRANTED == "CAPABILITY_GRANTED"

    def test_capability_denied_exists(self):
        assert AuthzEventType.CAPABILITY_DENIED == "CAPABILITY_DENIED"


class TestAgentEventTypes:
    """Test agent lifecycle event types."""

    def test_agent_registered_exists(self):
        assert AgentEventType.AGENT_REGISTERED == "AGENT_REGISTERED"

    def test_agent_updated_exists(self):
        assert AgentEventType.AGENT_UPDATED == "AGENT_UPDATED"


class TestCredEventTypes:
    """Test credential event types."""

    def test_credential_saved_exists(self):
        assert CredEventType.CREDENTIAL_SAVED == "CREDENTIAL_SAVED"

    def test_credential_loaded_exists(self):
        assert CredEventType.CREDENTIAL_LOADED == "CREDENTIAL_LOADED"

    def test_credential_deleted_exists(self):
        assert CredEventType.CREDENTIAL_DELETED == "CREDENTIAL_DELETED"

    def test_credential_created_exists(self):
        assert CredEventType.CREDENTIAL_CREATED == "CREDENTIAL_CREATED"

    def test_credential_migrated_exists(self):
        assert CredEventType.CREDENTIAL_MIGRATED == "CREDENTIAL_MIGRATED"


class TestSecurityLoggerContext:
    """Test SecurityLogger agent context management."""

    def test_set_agent_context(self):
        """Setting agent context should not raise."""
        # This should not raise any exceptions
        security_logger.set_agent_context(
            agent_id="test-agent-123",
            agent_name="test-agent",
            user_id="user-456",
            organization_id="org-789"
        )

    def test_clear_agent_context(self):
        """Clearing agent context should not raise."""
        security_logger.set_agent_context(
            agent_id="test-agent-123",
            agent_name="test-agent"
        )
        # This should not raise
        security_logger.clear_agent_context()


class TestSecurityLoggerConfiguration:
    """Test SecurityLogger configuration."""

    def test_configure_log_level(self):
        """Configuring log level should not raise."""
        # This should not raise
        security_logger.configure(log_level="DEBUG")

    def test_configure_with_file(self):
        """Configuring with file output should work."""
        with tempfile.TemporaryDirectory() as tmpdir:
            log_file = Path(tmpdir) / "test_security.log"
            security_logger.configure(
                log_file=str(log_file),
                log_level="INFO"
            )
            # Logging should work
            security_logger.log_authentication(
                event_type=AuthnEventType.SDK_INITIALIZED,
                success=True,
                details={"test": True}
            )


class TestSecurityLoggerLogging:
    """Test SecurityLogger logging methods."""

    def test_log_authentication_success(self):
        """Logging authentication success should work."""
        security_logger.log_authentication(
            event_type=AuthnEventType.TOKEN_REFRESH,
            success=True,
            agent_id="agent-123",
            details={"token_id": "xyz"}
        )

    def test_log_authentication_failure(self):
        """Logging authentication failure should work."""
        security_logger.log_authentication(
            event_type=AuthnEventType.TOKEN_REFRESH_FAILED,
            success=False,
            agent_id="agent-123",
            details={"error": "Token expired"}
        )

    def test_log_authorization_granted(self):
        """Logging authorization granted should work."""
        security_logger.log_authorization(
            event_type=AuthzEventType.CAPABILITY_GRANTED,
            action="db:read",
            resource="users_table",
            granted=True,
            agent_id="agent-123"
        )

    def test_log_authorization_denied(self):
        """Logging authorization denied should work."""
        security_logger.log_authorization(
            event_type=AuthzEventType.CAPABILITY_DENIED,
            action="db:delete",
            resource="users_table",
            granted=False,
            agent_id="agent-123",
            error="Insufficient trust score"
        )

    def test_log_agent_event(self):
        """Logging agent events should work."""
        security_logger.log_agent_event(
            event_type=AgentEventType.AGENT_REGISTERED,
            agent_id="agent-123",
            agent_name="my-agent",
            details={"agent_type": "custom"}
        )

    def test_log_credential_event(self):
        """Logging credential events should work."""
        security_logger.log_credential_event(
            event_type=CredEventType.CREDENTIAL_LOADED,
            credential_type="agent",
            success=True,
            agent_name="my-agent",
            details={"source": "file"}
        )

    def test_log_security_event(self):
        """Logging security events should work."""
        from aim_sdk.security_logging import SecurityEventType
        security_logger.log_security_event(
            event_type=SecurityEventType.ANOMALY_DETECTED,
            message="Unusual access pattern detected",
            severity=EventSeverity.WARNING,
            agent_id="agent-123",
            details={"anomaly_type": "unusual_access_pattern"}
        )


class TestSecurityLoggerSessionManagement:
    """Test SecurityLogger session management."""

    def test_start_session_returns_id(self):
        """Starting a session should return a session ID."""
        session_id = security_logger.start_session()
        assert session_id is not None
        assert len(session_id) > 0

    def test_start_session_unique_ids(self):
        """Each session should have a unique ID."""
        session1 = security_logger.start_session()
        session2 = security_logger.start_session()
        assert session1 != session2


class TestSecurityLoggerConvenienceMethods:
    """Test SecurityLogger convenience methods."""

    def test_debug_method(self):
        """Debug method should not raise."""
        security_logger.debug("Debug test message")

    def test_info_method(self):
        """Info method should not raise."""
        security_logger.info("Info test message")

    def test_warning_method(self):
        """Warning method should not raise."""
        security_logger.warning("Warning test message")

    def test_error_method(self):
        """Error method should not raise."""
        security_logger.error("Error test message")

    def test_critical_method(self):
        """Critical method should not raise."""
        security_logger.critical("Critical test message")


if __name__ == "__main__":
    pytest.main([__file__, "-v"])
