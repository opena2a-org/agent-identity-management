"""
AIM Python SDK - One-line agent registration and automatic identity verification

Enterprise-grade identity and capability management for AI agents.

This SDK provides seamless identity verification for AI agents registered with AIM.
All cryptographic signing and verification is handled automatically.

Quick Start (ONE LINE):
    from aim_sdk import secure

    # ONE LINE - that's it! Agent is registered, verified, and ready to use
    agent = secure("my-agent")

    @agent.perform_action("read_database", resource="users_table")
    def get_user_data(user_id):
        return database.query("SELECT * FROM users WHERE id = ?", user_id)

Manual Registration:
    from aim_sdk import AIMClient

    client = AIMClient(
        agent_id="your-agent-id",
        public_key="base64-public-key",
        private_key="base64-private-key",
        aim_url="https://aim.example.com"
    )

    @client.perform_action("read_database", resource="users_table")
    def get_user_data(user_id):
        return database.query("SELECT * FROM users WHERE id = ?", user_id)
"""

from .client import AIMClient, register_agent

# Alias for enterprise security
secure = register_agent

from .exceptions import AIMError, AuthenticationError, VerificationError, ActionDeniedError
from .detection import MCPDetector, auto_detect_mcps, track_mcp_call
from .capability_detection import CapabilityDetector, auto_detect_capabilities
from .protocol_detection import ProtocolDetector, auto_detect_protocol

__version__ = "1.0.0"
__all__ = [
    "AIMClient",
    "register_agent",
    "secure",
    "AIMError",
    "AuthenticationError",
    "VerificationError",
    "ActionDeniedError",
    "MCPDetector",
    "auto_detect_mcps",
    "CapabilityDetector",
    "auto_detect_capabilities",
    "ProtocolDetector",
    "auto_detect_protocol"
]
