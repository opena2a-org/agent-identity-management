"""
AIM Python SDK - One-line agent registration and automatic identity verification

"AIM is Stripe for AI Agent Identity"

This SDK provides seamless identity verification for AI agents registered with AIM.
All cryptographic signing and verification is handled automatically.

Quick Start (ONE LINE):
    from aim_sdk import register_agent

    # ONE LINE - that's it! Agent is registered, verified, and ready to use
    agent = register_agent("my-agent", "https://aim.example.com")

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
from .exceptions import AIMError, AuthenticationError, VerificationError, ActionDeniedError
from .decorators import (
    aim_verify,
    aim_verify_api_call,
    aim_verify_database,
    aim_verify_file_access,
    aim_verify_external_service
)

# Alias for security-conscious developers
secure = register_agent

__version__ = "1.0.0"
__all__ = [
    # Core
    "AIMClient",
    "register_agent",
    "secure",
    # Exceptions
    "AIMError",
    "AuthenticationError",
    "VerificationError",
    "ActionDeniedError",
    # Decorators
    "aim_verify",
    "aim_verify_api_call",
    "aim_verify_database",
    "aim_verify_file_access",
    "aim_verify_external_service"
]
