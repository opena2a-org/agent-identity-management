"""
AIM Tool Wrappers for LangChain

Wrap existing LangChain tools with AIM verification.
"""

from typing import Any, Optional, List, Type
from pydantic import BaseModel, Field
from langchain_core.tools import BaseTool, StructuredTool
from aim_sdk import AIMClient


class AIMToolWrapper(BaseTool):
    """
    Wraps a LangChain tool with AIM verification.

    This wrapper:
    1. Intercepts all tool calls
    2. Verifies the action with AIM
    3. Executes the wrapped tool if verification succeeds
    4. Logs the result back to AIM

    Usage:
        from langchain_core.tools import tool
        from aim_sdk import AIMClient
        from aim_sdk.integrations.langchain import AIMToolWrapper

        # Original LangChain tool
        @tool
        def my_tool(input: str) -> str:
            '''My tool description'''
            return f"Processed: {input}"

        # Wrap with AIM verification
        aim_client = AIMClient.auto_register_or_load("langchain-agent", "https://aim.company.com")
        verified_tool = AIMToolWrapper(
            tool=my_tool,
            aim_agent=aim_client,
            risk_level="high"
        )

        # Use in LangChain as normal
        tools = [verified_tool]
        agent = create_react_agent(llm=llm, tools=tools)
    """

    name: str = Field(description="Tool name")
    description: str = Field(description="Tool description")
    aim_agent: Any = Field(description="AIM client for verification", exclude=True)
    wrapped_tool: Any = Field(description="Original LangChain tool", exclude=True)
    risk_level: str = Field(default="medium", description="Risk level for AIM verification")

    class Config:
        arbitrary_types_allowed = True

    def _run(self, *args, **kwargs) -> Any:
        """Execute tool with AIM verification (synchronous)"""
        # Determine resource from arguments
        resource = ""
        if args:
            resource = str(args[0])[:100]
        elif kwargs:
            # Use first kwarg value
            first_value = next(iter(kwargs.values()), "")
            resource = str(first_value)[:100]

        # Verify with AIM
        try:
            verification_result = self.aim_agent.verify_action(
                action_type=f"langchain_tool:{self.name}",
                resource=resource,
                context={
                    "tool": self.name,
                    "risk_level": self.risk_level,
                    "source": "AIMToolWrapper"
                },
                timeout_seconds=5
            )
        except Exception as e:
            raise PermissionError(f"AIM verification failed for tool '{self.name}': {e}")

        # Verification succeeded - execute wrapped tool
        verification_id = verification_result.get("verification_id")

        try:
            # Execute the wrapped tool
            result = self.wrapped_tool.invoke(*args, **kwargs)

            # Log success
            if verification_id:
                try:
                    self.aim_agent.log_action_result(
                        verification_id=verification_id,
                        success=True,
                        result_summary=f"Tool '{self.name}' completed successfully"
                    )
                except Exception:
                    pass  # Don't fail on logging errors

            return result

        except Exception as e:
            # Log failure
            if verification_id:
                try:
                    self.aim_agent.log_action_result(
                        verification_id=verification_id,
                        success=False,
                        error_message=str(e)
                    )
                except Exception:
                    pass  # Don't fail on logging errors

            # Re-raise the original exception
            raise

    async def _arun(self, *args, **kwargs) -> Any:
        """Execute tool with AIM verification (asynchronous)"""
        # Same verification logic as sync version
        resource = ""
        if args:
            resource = str(args[0])[:100]
        elif kwargs:
            first_value = next(iter(kwargs.values()), "")
            resource = str(first_value)[:100]

        # Verify with AIM (synchronous verification for now)
        try:
            verification_result = self.aim_agent.verify_action(
                action_type=f"langchain_tool:{self.name}",
                resource=resource,
                context={
                    "tool": self.name,
                    "risk_level": self.risk_level,
                    "source": "AIMToolWrapper_async"
                },
                timeout_seconds=5
            )
        except Exception as e:
            raise PermissionError(f"AIM verification failed for tool '{self.name}': {e}")

        verification_id = verification_result.get("verification_id")

        try:
            # Execute the wrapped tool asynchronously
            result = await self.wrapped_tool.ainvoke(*args, **kwargs)

            # Log success
            if verification_id:
                try:
                    self.aim_agent.log_action_result(
                        verification_id=verification_id,
                        success=True,
                        result_summary=f"Tool '{self.name}' completed successfully (async)"
                    )
                except Exception:
                    pass

            return result

        except Exception as e:
            # Log failure
            if verification_id:
                try:
                    self.aim_agent.log_action_result(
                        verification_id=verification_id,
                        success=False,
                        error_message=str(e)
                    )
                except Exception:
                    pass

            raise


def wrap_tools_with_aim(
    tools: List[BaseTool],
    aim_agent: AIMClient,
    default_risk_level: str = "medium"
) -> List[BaseTool]:
    """
    Convenience function to wrap multiple LangChain tools with AIM verification.

    This function takes a list of existing LangChain tools and wraps each one
    with AIM verification, preserving all tool metadata.

    Usage:
        from langchain_core.tools import tool
        from aim_sdk import AIMClient
        from aim_sdk.integrations.langchain import wrap_tools_with_aim

        # Define tools
        @tool
        def calculator(expression: str) -> str:
            '''Calculate mathematical expressions'''
            return str(eval(expression))

        @tool
        def search_web(query: str) -> str:
            '''Search the web'''
            return f"Results for {query}"

        # Register AIM agent
        aim_client = AIMClient.auto_register_or_load("langchain-agent", "https://aim.company.com")

        # Wrap ALL tools with AIM verification
        verified_tools = wrap_tools_with_aim(
            tools=[calculator, search_web],
            aim_agent=aim_client,
            default_risk_level="medium"
        )

        # Use in LangChain - all tools now AIM-verified!
        agent = create_react_agent(llm=llm, tools=verified_tools)

    Args:
        tools: List of LangChain BaseTool instances to wrap
        aim_agent: AIMClient instance for verification
        default_risk_level: Default risk level for all tools ("low", "medium", "high")

    Returns:
        List of AIM-wrapped tools with same interface as originals
    """
    wrapped = []

    for tool in tools:
        wrapped_tool = AIMToolWrapper(
            name=tool.name,
            description=tool.description,
            aim_agent=aim_agent,
            wrapped_tool=tool,
            risk_level=default_risk_level
        )
        wrapped.append(wrapped_tool)

    return wrapped
