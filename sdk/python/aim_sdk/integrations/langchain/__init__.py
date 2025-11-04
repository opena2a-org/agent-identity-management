"""
AIM + LangChain Integration

Seamless integration between AIM (Agent Identity Management) and LangChain.

Components:
- AIMCallbackHandler: Automatically log all tool calls to AIM
- @aim_verify: Decorator to add AIM verification to tools
- AIMToolWrapper: Wrap existing tools with AIM verification
- wrap_tools_with_aim: Convenience function to wrap multiple tools

Quick Start:
    from aim_sdk import AIMClient
    from aim_sdk.integrations.langchain import AIMCallbackHandler, aim_verify
    from langchain_core.tools import tool

    # Register AIM agent
    aim_client = AIMClient.auto_register_or_load("langchain-agent", "https://aim.company.com")

    # Option 1: Automatic logging (simplest)
    from langchain.agents import create_react_agent

    aim_handler = AIMCallbackHandler(agent=aim_client)
    agent = create_react_agent(llm=llm, tools=tools, callbacks=[aim_handler])

    # Option 2: Explicit verification (most secure)
    @tool
    @aim_verify(agent=aim_client, risk_level="high")
    def delete_user(user_id: str) -> str:
        '''Delete a user'''
        return f"Deleted {user_id}"
"""

from aim_sdk.integrations.langchain.callback import AIMCallbackHandler
from aim_sdk.integrations.langchain.decorators import aim_verify
from aim_sdk.integrations.langchain.tools import AIMToolWrapper, wrap_tools_with_aim

__all__ = [
    "AIMCallbackHandler",
    "aim_verify",
    "AIMToolWrapper",
    "wrap_tools_with_aim",
]
