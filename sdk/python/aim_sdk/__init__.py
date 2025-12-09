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

# Alias for enterprise security
secure = register_agent

from .exceptions import AIMError, AuthenticationError, VerificationError, ActionDeniedError
from .detection import MCPDetector, auto_detect_mcps, track_mcp_call
from .capability_detection import CapabilityDetector, auto_detect_capabilities, auto_detect_agent_type
from .protocol_detection import ProtocolDetector, auto_detect_protocol

__version__ = "1.6.0"
__all__ = [
    "AIMClient",
    "register_agent",
    "secure",
    "AgentType",
    "AIMError",
    "AuthenticationError",
    "VerificationError",
    "ActionDeniedError",
    "MCPDetector",
    "auto_detect_mcps",
    "CapabilityDetector",
    "auto_detect_capabilities",
    "auto_detect_agent_type",
    "ProtocolDetector",
    "auto_detect_protocol"
]
