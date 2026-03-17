"""
AIM SDK Security Logging - SOC-ready structured logging for security monitoring.

This module provides comprehensive security event logging designed for:
- Security Operations Center (SOC) integration
- SIEM (Splunk, ELK, Datadog, etc.) compatibility
- Compliance auditing (SOC2, ISO27001)
- Incident investigation and forensics

Log Format: JSON Lines (JSONL) for easy parsing by security tools.

Event Categories:
- AUTHN: Authentication events (token refresh, credential load)
- AUTHZ: Authorization events (capability verification, action execution)
- AGENT: Agent lifecycle events (registration, update, deletion)
- CRED: Credential management events (save, load, migrate)
- MCP: MCP integration events (connection, attestation)
- SECURITY: Security-specific events (anomalies, policy violations)

Severity Levels (aligned with syslog/SIEM standards):
- DEBUG: Detailed diagnostic information
- INFO: Normal operations
- WARNING: Unexpected but handled situations
- ERROR: Operation failures
- CRITICAL: Security incidents requiring immediate attention

Usage:
    from aim_sdk.security_logging import security_logger, SecurityEvent

    # Log security events
    security_logger.log_authentication(
        event_type="TOKEN_REFRESH",
        success=True,
        agent_id="abc-123",
        details={"token_id": "xyz"}
    )

    # For SIEM integration, enable JSON file logging
    security_logger.configure(
        log_file="/var/log/aim/security.log",
        log_level="INFO",
        include_stdout=False
    )
"""

import json
import logging
import os
import sys
import socket
import uuid
from datetime import datetime, timezone
from enum import Enum
from pathlib import Path
from typing import Optional, Dict, Any, List, Union
from dataclasses import dataclass, field, asdict


# =============================================================================
# CONSTANTS AND ENUMS
# =============================================================================

class EventCategory(str, Enum):
    """Security event categories for classification."""
    AUTHN = "AUTHN"         # Authentication events
    AUTHZ = "AUTHZ"         # Authorization events
    AGENT = "AGENT"         # Agent lifecycle events
    CRED = "CRED"           # Credential management
    MCP = "MCP"             # MCP integration events
    SECURITY = "SECURITY"   # Security incidents
    AUDIT = "AUDIT"         # Audit trail events
    CONFIG = "CONFIG"       # Configuration changes


class EventSeverity(str, Enum):
    """Severity levels aligned with syslog standards."""
    DEBUG = "DEBUG"
    INFO = "INFO"
    WARNING = "WARNING"
    ERROR = "ERROR"
    CRITICAL = "CRITICAL"


class AuthnEventType(str, Enum):
    """Authentication event types."""
    TOKEN_REFRESH = "TOKEN_REFRESH"
    TOKEN_REFRESH_FAILED = "TOKEN_REFRESH_FAILED"
    TOKEN_EXPIRED = "TOKEN_EXPIRED"
    TOKEN_REVOKED = "TOKEN_REVOKED"
    TOKEN_RECOVERED = "TOKEN_RECOVERED"
    CREDENTIAL_LOAD = "CREDENTIAL_LOAD"
    CREDENTIAL_LOAD_FAILED = "CREDENTIAL_LOAD_FAILED"
    SDK_INITIALIZED = "SDK_INITIALIZED"


class AuthzEventType(str, Enum):
    """Authorization event types."""
    CAPABILITY_CHECK = "CAPABILITY_CHECK"
    CAPABILITY_GRANTED = "CAPABILITY_GRANTED"
    CAPABILITY_DENIED = "CAPABILITY_DENIED"
    CAPABILITY_ESCALATION = "CAPABILITY_ESCALATION"
    ACTION_EXECUTED = "ACTION_EXECUTED"
    ACTION_DENIED = "ACTION_DENIED"
    JIT_REQUEST = "JIT_REQUEST"
    JIT_APPROVED = "JIT_APPROVED"
    JIT_DENIED = "JIT_DENIED"
    JIT_TIMEOUT = "JIT_TIMEOUT"


class AgentEventType(str, Enum):
    """Agent lifecycle event types."""
    AGENT_REGISTERED = "AGENT_REGISTERED"
    AGENT_REGISTRATION_FAILED = "AGENT_REGISTRATION_FAILED"
    AGENT_LOADED = "AGENT_LOADED"
    AGENT_UPDATED = "AGENT_UPDATED"
    AGENT_DELETED = "AGENT_DELETED"
    AGENT_SUSPENDED = "AGENT_SUSPENDED"
    AGENT_REACTIVATED = "AGENT_REACTIVATED"
    AGENT_STALE_CREDENTIALS = "AGENT_STALE_CREDENTIALS"  # Cached credentials no longer valid
    TRUST_SCORE_CHANGED = "TRUST_SCORE_CHANGED"


class CredEventType(str, Enum):
    """Credential management event types."""
    CREDENTIAL_CREATED = "CREDENTIAL_CREATED"
    CREDENTIAL_LOADED = "CREDENTIAL_LOADED"
    CREDENTIAL_SAVED = "CREDENTIAL_SAVED"
    CREDENTIAL_MIGRATED = "CREDENTIAL_MIGRATED"
    CREDENTIAL_DELETED = "CREDENTIAL_DELETED"
    CREDENTIAL_ROTATED = "CREDENTIAL_ROTATED"
    CREDENTIAL_TYPE_MISMATCH = "CREDENTIAL_TYPE_MISMATCH"
    SECURE_STORAGE_ENABLED = "SECURE_STORAGE_ENABLED"
    SECURE_STORAGE_FAILED = "SECURE_STORAGE_FAILED"


class MCPEventType(str, Enum):
    """MCP integration event types."""
    MCP_SERVER_CONNECTED = "MCP_SERVER_CONNECTED"
    MCP_SERVER_DISCONNECTED = "MCP_SERVER_DISCONNECTED"
    MCP_TOOL_USED = "MCP_TOOL_USED"
    MCP_ATTESTATION_CREATED = "MCP_ATTESTATION_CREATED"
    MCP_ATTESTATION_FAILED = "MCP_ATTESTATION_FAILED"
    MCP_CAPABILITY_DRIFT = "MCP_CAPABILITY_DRIFT"
    MCP_CONNECTION_RECORDED = "MCP_CONNECTION_RECORDED"


class SecurityEventType(str, Enum):
    """Security-specific event types."""
    ANOMALY_DETECTED = "ANOMALY_DETECTED"
    POLICY_VIOLATION = "POLICY_VIOLATION"
    SIGNATURE_VERIFICATION_FAILED = "SIGNATURE_VERIFICATION_FAILED"
    UNAUTHORIZED_ACCESS_ATTEMPT = "UNAUTHORIZED_ACCESS_ATTEMPT"
    RATE_LIMIT_EXCEEDED = "RATE_LIMIT_EXCEEDED"
    SUSPICIOUS_PATTERN = "SUSPICIOUS_PATTERN"
    ENCRYPTION_FAILURE = "ENCRYPTION_FAILURE"


# =============================================================================
# SECURITY EVENT DATA STRUCTURE
# =============================================================================

@dataclass
class SecurityEvent:
    """
    Structured security event for SOC/SIEM integration.

    Fields are designed for easy parsing and correlation in security tools.
    All timestamps are ISO 8601 UTC format for cross-system compatibility.
    """
    # Required fields
    timestamp: str                          # ISO 8601 UTC
    event_id: str                           # Unique event identifier (UUID)
    category: str                           # EventCategory value
    event_type: str                         # Specific event type
    severity: str                           # EventSeverity value
    message: str                            # Human-readable description

    # Context fields
    sdk_version: str = ""                   # AIM SDK version
    hostname: str = ""                      # Machine hostname
    process_id: int = 0                     # Process ID
    thread_id: str = ""                     # Thread identifier

    # Identity fields
    agent_id: Optional[str] = None          # Agent UUID
    agent_name: Optional[str] = None        # Agent name
    user_id: Optional[str] = None           # User UUID (for SDK auth)
    organization_id: Optional[str] = None   # Organization UUID

    # Action fields
    action: Optional[str] = None            # Action being performed
    resource: Optional[str] = None          # Resource being accessed
    result: Optional[str] = None            # SUCCESS, FAILURE, DENIED

    # Network fields
    source_ip: Optional[str] = None         # Client IP address
    destination_url: Optional[str] = None   # Target URL/endpoint
    http_method: Optional[str] = None       # HTTP method if applicable
    http_status: Optional[int] = None       # HTTP status code

    # Additional context
    details: Dict[str, Any] = field(default_factory=dict)  # Extra data
    error: Optional[str] = None             # Error message if failed
    stack_trace: Optional[str] = None       # Stack trace for errors

    # Correlation fields
    correlation_id: Optional[str] = None    # Request correlation ID
    parent_event_id: Optional[str] = None   # Parent event for tracing
    session_id: Optional[str] = None        # Session identifier

    def to_json(self) -> str:
        """Convert to JSON string for logging."""
        data = {k: v for k, v in asdict(self).items() if v is not None and v != "" and v != {} and v != 0}
        return json.dumps(data, default=str, separators=(',', ':'))

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary, excluding empty fields."""
        return {k: v for k, v in asdict(self).items() if v is not None and v != "" and v != {} and v != 0}


# =============================================================================
# JSON FORMATTER FOR STRUCTURED LOGGING
# =============================================================================

class JSONSecurityFormatter(logging.Formatter):
    """
    JSON formatter for security logs - SIEM compatible.

    Output format: Single-line JSON (JSON Lines format)
    Compatible with: Splunk, ELK, Datadog, Sumo Logic, etc.
    """

    def format(self, record: logging.LogRecord) -> str:
        """Format log record as JSON."""
        # If the record has a security_event attribute, use it
        if hasattr(record, 'security_event') and isinstance(record.security_event, SecurityEvent):
            return record.security_event.to_json()

        # Otherwise, create a basic event from the log record
        event = SecurityEvent(
            timestamp=datetime.now(timezone.utc).isoformat(),
            event_id=str(uuid.uuid4()),
            category=EventCategory.AUDIT.value,
            event_type="LOG_MESSAGE",
            severity=record.levelname,
            message=record.getMessage(),
            sdk_version=_get_sdk_version(),
            hostname=socket.gethostname(),
            process_id=os.getpid(),
            details={
                "logger": record.name,
                "module": record.module,
                "function": record.funcName,
                "line": record.lineno,
            }
        )

        if record.exc_info:
            import traceback
            event.stack_trace = ''.join(traceback.format_exception(*record.exc_info))

        return event.to_json()


# =============================================================================
# SECURITY LOGGER CLASS
# =============================================================================

class SecurityLogger:
    """
    Security-focused logger for AIM SDK.

    Provides structured logging methods for all security-relevant events.
    Designed for SOC integration and SIEM compatibility.
    """

    def __init__(self):
        self._logger = logging.getLogger("aim.security")
        self._configured = False
        self._log_level = logging.INFO
        self._session_id: Optional[str] = None
        self._agent_context: Dict[str, str] = {}
        self._enabled = True

        # Default configuration
        self._configure_default()

    def _configure_default(self):
        """Configure default logging to stderr for immediate visibility."""
        if self._configured:
            return

        # Only add handler if none exist
        if not self._logger.handlers:
            handler = logging.StreamHandler(sys.stderr)
            handler.setFormatter(JSONSecurityFormatter())
            self._logger.addHandler(handler)
            self._logger.setLevel(logging.WARNING)  # Default: only warnings and above

        self._configured = True

    def configure(
        self,
        log_file: Optional[str] = None,
        log_level: str = "INFO",
        include_stdout: bool = False,
        max_bytes: int = 10 * 1024 * 1024,  # 10MB
        backup_count: int = 5,
        enabled: bool = True
    ):
        """
        Configure security logging for SOC integration.

        Args:
            log_file: Path to security log file (e.g., /var/log/aim/security.log)
            log_level: Minimum log level (DEBUG, INFO, WARNING, ERROR, CRITICAL)
            include_stdout: Also log to stdout (useful for containerized environments)
            max_bytes: Maximum log file size before rotation
            backup_count: Number of backup files to keep
            enabled: Enable/disable security logging entirely
        """
        self._enabled = enabled
        if not enabled:
            return

        self._log_level = getattr(logging, log_level.upper(), logging.INFO)
        self._logger.setLevel(self._log_level)

        # Clear existing handlers
        self._logger.handlers.clear()

        # Add file handler if specified
        if log_file:
            log_path = Path(log_file)
            log_path.parent.mkdir(parents=True, exist_ok=True)

            from logging.handlers import RotatingFileHandler
            file_handler = RotatingFileHandler(
                log_file,
                maxBytes=max_bytes,
                backupCount=backup_count
            )
            file_handler.setFormatter(JSONSecurityFormatter())
            file_handler.setLevel(self._log_level)
            self._logger.addHandler(file_handler)

        # Add stdout handler if requested
        if include_stdout:
            stdout_handler = logging.StreamHandler(sys.stdout)
            stdout_handler.setFormatter(JSONSecurityFormatter())
            stdout_handler.setLevel(self._log_level)
            self._logger.addHandler(stdout_handler)

        # Always have stderr for errors
        stderr_handler = logging.StreamHandler(sys.stderr)
        stderr_handler.setFormatter(JSONSecurityFormatter())
        stderr_handler.setLevel(logging.ERROR)
        self._logger.addHandler(stderr_handler)

        self._configured = True

    def set_agent_context(
        self,
        agent_id: Optional[str] = None,
        agent_name: Optional[str] = None,
        user_id: Optional[str] = None,
        organization_id: Optional[str] = None
    ):
        """Set persistent agent context for all subsequent log events."""
        if agent_id:
            self._agent_context["agent_id"] = agent_id
        if agent_name:
            self._agent_context["agent_name"] = agent_name
        if user_id:
            self._agent_context["user_id"] = user_id
        if organization_id:
            self._agent_context["organization_id"] = organization_id

    def clear_agent_context(self):
        """Clear agent context."""
        self._agent_context.clear()

    def start_session(self) -> str:
        """Start a new logging session and return session ID."""
        self._session_id = str(uuid.uuid4())
        return self._session_id

    def _create_event(
        self,
        category: EventCategory,
        event_type: Union[str, Enum],
        severity: EventSeverity,
        message: str,
        **kwargs
    ) -> SecurityEvent:
        """Create a security event with common fields populated.

        Merges persistent agent context with per-call kwargs. When the same
        field (e.g. agent_id) appears in both _agent_context and kwargs,
        the per-call kwarg takes precedence, preventing duplicate keyword
        argument errors in the SecurityEvent constructor.
        """
        event_type_str = event_type.value if isinstance(event_type, Enum) else event_type

        # Merge agent context with kwargs. Explicit non-None kwargs take
        # precedence over context values, preventing both duplicate keyword
        # errors and accidental overwrite of context with default None values.
        merged_context = {**self._agent_context}
        for key, value in kwargs.items():
            if value is not None or key not in merged_context:
                merged_context[key] = value

        event = SecurityEvent(
            timestamp=datetime.now(timezone.utc).isoformat(),
            event_id=str(uuid.uuid4()),
            category=category.value,
            event_type=event_type_str,
            severity=severity.value,
            message=message,
            sdk_version=_get_sdk_version(),
            hostname=socket.gethostname(),
            process_id=os.getpid(),
            session_id=self._session_id,
            **merged_context
        )
        return event

    def _log_event(self, event: SecurityEvent, level: int):
        """Log a security event at the specified level."""
        if not self._enabled:
            return

        record = self._logger.makeRecord(
            self._logger.name,
            level,
            __file__,
            0,
            event.message,
            (),
            None
        )
        record.security_event = event
        self._logger.handle(record)

    # =========================================================================
    # AUTHENTICATION EVENTS
    # =========================================================================

    def log_authentication(
        self,
        event_type: AuthnEventType,
        success: bool,
        message: Optional[str] = None,
        agent_id: Optional[str] = None,
        user_id: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
        error: Optional[str] = None
    ):
        """Log authentication events (token refresh, credential load, etc.)."""
        severity = EventSeverity.INFO if success else EventSeverity.WARNING
        if event_type in [AuthnEventType.TOKEN_REVOKED, AuthnEventType.TOKEN_EXPIRED]:
            severity = EventSeverity.WARNING if success else EventSeverity.ERROR

        default_message = f"Authentication event: {event_type.value}"
        event = self._create_event(
            category=EventCategory.AUTHN,
            event_type=event_type,
            severity=severity,
            message=message or default_message,
            agent_id=agent_id,
            user_id=user_id,
            result="SUCCESS" if success else "FAILURE",
            details=details or {},
            error=error
        )

        level = logging.INFO if success else logging.WARNING
        self._log_event(event, level)

    # =========================================================================
    # AUTHORIZATION EVENTS
    # =========================================================================

    def log_authorization(
        self,
        event_type: AuthzEventType,
        action: str,
        resource: Optional[str] = None,
        granted: bool = True,
        agent_id: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
        error: Optional[str] = None
    ):
        """Log authorization events (capability checks, action execution)."""
        severity = EventSeverity.INFO if granted else EventSeverity.WARNING
        if event_type == AuthzEventType.CAPABILITY_ESCALATION:
            severity = EventSeverity.WARNING
        elif event_type in [AuthzEventType.CAPABILITY_DENIED, AuthzEventType.ACTION_DENIED]:
            severity = EventSeverity.WARNING

        message = f"Authorization {event_type.value}: {action}"
        if resource:
            message += f" on {resource}"
        message += f" - {'GRANTED' if granted else 'DENIED'}"

        event = self._create_event(
            category=EventCategory.AUTHZ,
            event_type=event_type,
            severity=severity,
            message=message,
            agent_id=agent_id,
            action=action,
            resource=resource,
            result="GRANTED" if granted else "DENIED",
            details=details or {},
            error=error
        )

        level = logging.INFO if granted else logging.WARNING
        self._log_event(event, level)

    # =========================================================================
    # AGENT LIFECYCLE EVENTS
    # =========================================================================

    def log_agent_event(
        self,
        event_type: AgentEventType,
        agent_id: Optional[str] = None,
        agent_name: Optional[str] = None,
        message: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
        error: Optional[str] = None
    ):
        """Log agent lifecycle events (registration, updates, etc.)."""
        severity = EventSeverity.INFO
        if event_type == AgentEventType.AGENT_REGISTRATION_FAILED:
            severity = EventSeverity.ERROR
        elif event_type in [AgentEventType.AGENT_SUSPENDED, AgentEventType.AGENT_DELETED]:
            severity = EventSeverity.WARNING

        default_message = f"Agent event: {event_type.value}"
        if agent_name:
            default_message += f" for {agent_name}"

        event = self._create_event(
            category=EventCategory.AGENT,
            event_type=event_type,
            severity=severity,
            message=message or default_message,
            agent_id=agent_id,
            agent_name=agent_name,
            details=details or {},
            error=error
        )

        level = logging.ERROR if error else logging.INFO
        self._log_event(event, level)

    # =========================================================================
    # CREDENTIAL EVENTS
    # =========================================================================

    def log_credential_event(
        self,
        event_type: CredEventType,
        credential_type: str,  # "sdk_oauth" or "agent"
        success: bool = True,
        agent_name: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
        error: Optional[str] = None
    ):
        """Log credential management events."""
        severity = EventSeverity.INFO if success else EventSeverity.WARNING
        if event_type == CredEventType.CREDENTIAL_TYPE_MISMATCH:
            severity = EventSeverity.WARNING
        elif event_type == CredEventType.SECURE_STORAGE_FAILED:
            severity = EventSeverity.WARNING

        message = f"Credential {event_type.value}: {credential_type}"
        if agent_name:
            message += f" for {agent_name}"

        event = self._create_event(
            category=EventCategory.CRED,
            event_type=event_type,
            severity=severity,
            message=message,
            agent_name=agent_name,
            result="SUCCESS" if success else "FAILURE",
            details={"credential_type": credential_type, **(details or {})},
            error=error
        )

        level = logging.INFO if success else logging.WARNING
        self._log_event(event, level)

    # =========================================================================
    # MCP EVENTS
    # =========================================================================

    def log_mcp_event(
        self,
        event_type: MCPEventType,
        server_id: Optional[str] = None,
        server_name: Optional[str] = None,
        tool_name: Optional[str] = None,
        agent_id: Optional[str] = None,
        success: bool = True,
        details: Optional[Dict[str, Any]] = None,
        error: Optional[str] = None
    ):
        """Log MCP integration events."""
        severity = EventSeverity.INFO if success else EventSeverity.WARNING
        if event_type == MCPEventType.MCP_CAPABILITY_DRIFT:
            severity = EventSeverity.WARNING
        elif event_type == MCPEventType.MCP_ATTESTATION_FAILED:
            severity = EventSeverity.ERROR

        message = f"MCP {event_type.value}"
        if server_name:
            message += f": {server_name}"
        if tool_name:
            message += f" - tool: {tool_name}"

        mcp_details = details or {}
        if server_id:
            mcp_details["server_id"] = server_id
        if server_name:
            mcp_details["server_name"] = server_name
        if tool_name:
            mcp_details["tool_name"] = tool_name

        event = self._create_event(
            category=EventCategory.MCP,
            event_type=event_type,
            severity=severity,
            message=message,
            agent_id=agent_id,
            result="SUCCESS" if success else "FAILURE",
            details=mcp_details,
            error=error
        )

        level = logging.INFO if success else logging.WARNING
        self._log_event(event, level)

    # =========================================================================
    # SECURITY EVENTS (HIGH PRIORITY)
    # =========================================================================

    def log_security_event(
        self,
        event_type: SecurityEventType,
        message: str,
        severity: EventSeverity = EventSeverity.WARNING,
        agent_id: Optional[str] = None,
        source_ip: Optional[str] = None,
        details: Optional[Dict[str, Any]] = None,
        error: Optional[str] = None
    ):
        """Log security-specific events (anomalies, policy violations, etc.)."""
        event = self._create_event(
            category=EventCategory.SECURITY,
            event_type=event_type,
            severity=severity,
            message=message,
            agent_id=agent_id,
            source_ip=source_ip,
            details=details or {},
            error=error
        )

        # Security events always log at WARNING or higher
        level = max(logging.WARNING, getattr(logging, severity.value, logging.WARNING))
        self._log_event(event, level)

    # =========================================================================
    # CONVENIENCE METHODS
    # =========================================================================

    def debug(self, message: str, **kwargs):
        """Log debug message."""
        if not self._enabled:
            return
        event = self._create_event(
            EventCategory.AUDIT, "DEBUG", EventSeverity.DEBUG, message, **kwargs
        )
        self._log_event(event, logging.DEBUG)

    def info(self, message: str, **kwargs):
        """Log info message."""
        if not self._enabled:
            return
        event = self._create_event(
            EventCategory.AUDIT, "INFO", EventSeverity.INFO, message, **kwargs
        )
        self._log_event(event, logging.INFO)

    def warning(self, message: str, **kwargs):
        """Log warning message."""
        if not self._enabled:
            return
        event = self._create_event(
            EventCategory.AUDIT, "WARNING", EventSeverity.WARNING, message, **kwargs
        )
        self._log_event(event, logging.WARNING)

    def error(self, message: str, **kwargs):
        """Log error message."""
        if not self._enabled:
            return
        event = self._create_event(
            EventCategory.AUDIT, "ERROR", EventSeverity.ERROR, message, **kwargs
        )
        self._log_event(event, logging.ERROR)

    def critical(self, message: str, **kwargs):
        """Log critical message."""
        if not self._enabled:
            return
        event = self._create_event(
            EventCategory.AUDIT, "CRITICAL", EventSeverity.CRITICAL, message, **kwargs
        )
        self._log_event(event, logging.CRITICAL)


# =============================================================================
# HELPER FUNCTIONS
# =============================================================================

def _get_sdk_version() -> str:
    """Get the SDK version string."""
    try:
        # Read version from VERSION file directly to avoid circular imports
        version_file = Path(__file__).parent.parent / "VERSION"
        if version_file.exists():
            return f"aim-sdk-python@{version_file.read_text().strip()}"
        # Fallback to package metadata
        from aim_sdk import __version__
        return f"aim-sdk-python@{__version__}"
    except Exception:
        return "aim-sdk-python@unknown"


# =============================================================================
# GLOBAL LOGGER INSTANCE
# =============================================================================

# Global security logger instance
security_logger = SecurityLogger()


# =============================================================================
# ENVIRONMENT-BASED CONFIGURATION
# =============================================================================

def configure_from_environment():
    """
    Configure security logging from environment variables.

    Environment variables:
    - AIM_SECURITY_LOG_FILE: Path to security log file
    - AIM_SECURITY_LOG_LEVEL: Log level (DEBUG, INFO, WARNING, ERROR, CRITICAL)
    - AIM_SECURITY_LOG_STDOUT: Include stdout logging (true/false)
    - AIM_SECURITY_LOGGING_ENABLED: Enable/disable security logging (true/false)
    """
    log_file = os.environ.get("AIM_SECURITY_LOG_FILE")
    log_level = os.environ.get("AIM_SECURITY_LOG_LEVEL", "INFO")
    include_stdout = os.environ.get("AIM_SECURITY_LOG_STDOUT", "false").lower() == "true"
    enabled = os.environ.get("AIM_SECURITY_LOGGING_ENABLED", "true").lower() != "false"

    security_logger.configure(
        log_file=log_file,
        log_level=log_level,
        include_stdout=include_stdout,
        enabled=enabled
    )


# Auto-configure from environment on import
configure_from_environment()
