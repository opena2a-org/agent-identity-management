"""
AIM Python SDK - Automatic Identity Verification for AI Agents

This SDK provides seamless identity verification for AI agents registered with AIM.
All cryptographic signing and verification is handled automatically.

Example:
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

from .client import AIMClient
from .exceptions import AIMError, AuthenticationError, VerificationError, ActionDeniedError

__version__ = "1.0.0"
__all__ = ["AIMClient", "AIMError", "AuthenticationError", "VerificationError", "ActionDeniedError"]
