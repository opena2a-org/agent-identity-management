"""
AIM Python SDK - One-line agent registration and automatic identity verification

Production-ready identity and capability management for AI agents.

This SDK provides seamless identity verification for AI agents registered with AIM.
All cryptographic signing and verification is handled automatically.

Quick Start (ONE LINE):
    from aim_sdk import secure

    # ONE LINE - that's it! Agent type is auto-detected from your imports
    agent = secure("my-agent")

    @agent.perform_action("db:read", resource="users_table")
    def get_user_data(user_id):
        return database.query("SELECT * FROM users WHERE id = ?", user_id)

Auto-Detection:
    The SDK automatically detects your agent type based on imported packages:
    - LangChain imports -> agent_type="langchain"
    - CrewAI imports -> agent_type="crewai"
    - Anthropic imports -> agent_type="claude"
    - OpenAI imports -> agent_type="gpt"
    - And more...

    You can also specify explicitly:
        from aim_sdk import secure, AgentType
        agent = secure("my-agent", agent_type=AgentType.LANGCHAIN)

Manual Registration:
    from aim_sdk import AIMClient

    client = AIMClient(
        agent_id="your-agent-id",
        public_key="base64-public-key",
        private_key="base64-private-key",
        aim_url="https://aim.example.com"
    )

    @client.perform_action("db:read", resource="users_table")
    def get_user_data(user_id):
        return database.query("SELECT * FROM users WHERE id = ?", user_id)

Agent Management (Generic SDK Pattern):
    # Create agents programmatically through an authenticated client
    from aim_sdk import AIMClient, AgentType

    client = AIMClient(agent_id="admin-agent", aim_url="https://aim.example.com", ...)

    # Create a new agent with explicit type
    new_agent = client.create_new_agent(
        name="my-langchain-agent",
        display_name="My LangChain Agent",
        description="An agent for processing data",
        agent_type=AgentType.LANGCHAIN,
        capabilities=["db:read", "file:write"]
    )
    print(f"Created: {new_agent['id']}")

    # List all agents
    agents = client.list_agents()
    for agent in agents["agents"]:
        print(f"{agent['name']}: {agent['status']}")

    # Get/update/delete agents
    details = client.get_agent_details(agent_id)
    updated = client.update_agent(display_name="New Name")
    client.delete_agent(agent_id)
"""

from .client import AIMClient, register_agent, AgentType
from .decorators import aim_verify, aim_verify_database, aim_verify_api_call

# Alias for enterprise security
secure = register_agent

from .exceptions import AIMError, AuthenticationError, VerificationError, ActionDeniedError, ConfigurationError, StaleCredentialsError
from .secrets import SecretsClient, SecretsError
from .detection import MCPDetector, auto_detect_mcps, track_mcp_call
from .capability_detection import CapabilityDetector, auto_detect_capabilities, auto_detect_agent_type
from .protocol_detection import ProtocolDetector, auto_detect_protocol
from .attestation_cache import AttestationCache
from .isolation import (
    SandboxType,
    NetworkIsolation,
    FilesystemIsolation,
    ProcessIsolation,
    score_isolation,
    auto_detect_isolation,
)
from .auto_hooks import PolicyCache, activate_hooks

# Credential management utilities
from .credentials import (
    load_sdk_credentials,
    save_sdk_credentials,
    load_agent_credentials,
    save_agent_credentials,
    list_agent_credentials,
    delete_agent_credentials,
    CredentialType,
)

# Security logging for SOC/SIEM integration
from .security_logging import (
    SecurityLogger,
    SecurityEvent,
    security_logger,
    configure_from_environment as configure_security_logging,
    # Event types for custom logging
    EventCategory,
    EventSeverity,
    AuthnEventType,
    AuthzEventType,
    AgentEventType,
    CredEventType,
    MCPEventType,
    SecurityEventType,
)

# A2A (Agent-to-Agent) protocol support
from .a2a import (
    A2AClient,
    A2AAgentCard,
    A2ARequestSignature,
    A2ATrustScore,
    A2APeerTrust,
    A2AConsent,
    A2AError,
    create_a2a_client_from_env,
)

# Read version from VERSION file (single source of truth)
# Supports both development (file in parent dir) and installed package scenarios
import os as _os

def _get_version():
    """Get version from VERSION file or fallback to importlib.metadata."""
    # Try VERSION file in parent directory (development mode)
    version_file = _os.path.join(_os.path.dirname(__file__), "..", "VERSION")
    if _os.path.exists(version_file):
        with open(version_file, "r", encoding="utf-8") as vf:
            return vf.read().strip()

    # Try importlib.metadata (installed package)
    try:
        from importlib.metadata import version
        return version("aim-sdk")
    except Exception:
        pass

    return "0.0.0"  # Ultimate fallback

__version__ = _get_version()
__all__ = [
    "AIMClient",
    "register_agent",
    "secure",
    "AgentType",
    "AIMError",
    "AuthenticationError",
    "VerificationError",
    "ActionDeniedError",
    "ConfigurationError",
    "StaleCredentialsError",
    "MCPDetector",
    "auto_detect_mcps",
    "CapabilityDetector",
    "auto_detect_capabilities",
    "auto_detect_agent_type",
    "ProtocolDetector",
    "auto_detect_protocol",
    "AttestationCache",
    # Auto-hook activation
    "PolicyCache",
    "activate_hooks",
    # Credential management
    "load_sdk_credentials",
    "save_sdk_credentials",
    "load_agent_credentials",
    "save_agent_credentials",
    "list_agent_credentials",
    "delete_agent_credentials",
    "CredentialType",
    # Security logging (SOC/SIEM integration)
    "SecurityLogger",
    "SecurityEvent",
    "security_logger",
    "configure_security_logging",
    "EventCategory",
    "EventSeverity",
    "AuthnEventType",
    "AuthzEventType",
    "AgentEventType",
    "CredEventType",
    "MCPEventType",
    "SecurityEventType",
    # A2A (Agent-to-Agent) protocol support
    "A2AClient",
    "A2AAgentCard",
    "A2ARequestSignature",
    "A2ATrustScore",
    "A2APeerTrust",
    "A2AConsent",
    "A2AError",
    "create_a2a_client_from_env",
    # Isolation attestation types
    "SandboxType",
    "NetworkIsolation",
    "FilesystemIsolation",
    "ProcessIsolation",
    "score_isolation",
    "auto_detect_isolation",
    # Decorators
    "aim_verify",
    "aim_verify_database",
    "aim_verify_api_call",
]
