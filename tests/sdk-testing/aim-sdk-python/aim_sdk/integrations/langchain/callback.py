"""
AIM Callback Handler for LangChain

Automatically logs all LangChain tool invocations to AIM for audit and compliance.
"""

from typing import Any, Dict, List, Optional
from langchain_core.callbacks import BaseCallbackHandler
from aim_sdk import AIMClient


class AIMCallbackHandler(BaseCallbackHandler):
    """
    LangChain callback handler that logs all tool calls to AIM.

    This handler automatically captures:
    - Tool invocations (start, end, errors)
    - Input and output data
    - Execution metadata
    - Errors and exceptions

    Usage:
        from aim_sdk import AIMClient
        from aim_sdk.integrations.langchain import AIMCallbackHandler
        from langchain.agents import create_react_agent
        from langchain_openai import ChatOpenAI

        # Register AIM agent
        aim_client = AIMClient.auto_register_or_load(
            "langchain-agent",
            "https://aim.company.com"
        )

        # Create callback handler
        aim_handler = AIMCallbackHandler(
            agent=aim_client,
            log_inputs=True,
            log_outputs=True
        )

        # Attach to LangChain agent
        agent = create_react_agent(
            llm=ChatOpenAI(),
            tools=tools,
            callbacks=[aim_handler]  # ← All tool calls logged to AIM!
        )

        # Run agent - all actions automatically logged
        agent.invoke({"input": "What's the weather?"})

    Args:
        agent: AIMClient instance for logging
        log_inputs: Whether to log tool inputs (default: True)
        log_outputs: Whether to log tool outputs (default: True)
        log_errors: Whether to log errors (default: True)
        verbose: Print debug information (default: False)
    """

    def __init__(
        self,
        agent: AIMClient,
        log_inputs: bool = True,
        log_outputs: bool = True,
        log_errors: bool = True,
        verbose: bool = False
    ):
        super().__init__()
        self.agent = agent
        self.log_inputs = log_inputs
        self.log_outputs = log_outputs
        self.log_errors = log_errors
        self.verbose = verbose
        self._active_tools: Dict[str, Dict[str, Any]] = {}

    def on_tool_start(
        self,
        serialized: Dict[str, Any],
        input_str: str,
        *,
        run_id: str,
        parent_run_id: Optional[str] = None,
        tags: Optional[List[str]] = None,
        metadata: Optional[Dict[str, Any]] = None,
        **kwargs: Any
    ) -> Any:
        """Called when a tool starts executing"""
        tool_name = serialized.get("name", "unknown_tool")

        if self.verbose:
            print(f"🔧 AIM: Tool started - {tool_name}")

        # Store tool invocation details for later logging
        self._active_tools[run_id] = {
            "tool_name": tool_name,
            "input": input_str if self.log_inputs else "[hidden]",
            "tags": tags or [],
            "metadata": metadata or {},
            "run_id": run_id,
            "parent_run_id": parent_run_id
        }

    def on_tool_end(
        self,
        output: str,
        *,
        run_id: str,
        **kwargs: Any
    ) -> Any:
        """Called when a tool finishes successfully"""
        if run_id not in self._active_tools:
            if self.verbose:
                print(f"⚠️  AIM: Tool end event for unknown run_id: {run_id}")
            return

        tool_data = self._active_tools.pop(run_id)
        tool_name = tool_data["tool_name"]

        if self.verbose:
            print(f"✅ AIM: Tool completed - {tool_name}")

        # Log successful tool execution to AIM
        try:
            # Use verify_action for logging (doesn't actually block)
            verification_result = self.agent.verify_action(
                action_type=f"langchain_tool:{tool_name}",
                resource=tool_data.get("input", "")[:100],  # First 100 chars
                context={
                    "tool_output": output[:500] if self.log_outputs else "[hidden]",
                    "tags": tool_data.get("tags", []),
                    "run_id": run_id,
                    "status": "success",
                    **tool_data.get("metadata", {})
                },
                timeout_seconds=1  # Non-blocking
            )

            # Log completion
            if verification_result.get("verification_id"):
                self.agent.log_action_result(
                    verification_id=verification_result["verification_id"],
                    success=True,
                    result_summary=f"Tool '{tool_name}' completed successfully"
                )

        except Exception as e:
            if self.log_errors and self.verbose:
                print(f"⚠️  AIM logging error: {e}")

    def on_tool_error(
        self,
        error: BaseException,
        *,
        run_id: str,
        **kwargs: Any
    ) -> Any:
        """Called when a tool execution fails"""
        if run_id not in self._active_tools:
            return

        tool_data = self._active_tools.pop(run_id)
        tool_name = tool_data["tool_name"]

        if self.verbose:
            print(f"❌ AIM: Tool failed - {tool_name}: {str(error)[:100]}")

        # Log error to AIM
        if self.log_errors:
            try:
                verification_result = self.agent.verify_action(
                    action_type=f"langchain_tool:{tool_name}",
                    resource=tool_data.get("input", "")[:100],
                    context={
                        "error": str(error)[:500],
                        "error_type": type(error).__name__,
                        "status": "failed",
                        "run_id": run_id,
                        **tool_data.get("metadata", {})
                    },
                    timeout_seconds=1  # Non-blocking
                )

                # Log failure
                if verification_result.get("verification_id"):
                    self.agent.log_action_result(
                        verification_id=verification_result["verification_id"],
                        success=False,
                        error_message=str(error)
                    )

            except Exception as e:
                if self.verbose:
                    print(f"⚠️  AIM logging error: {e}")

    def on_chain_start(
        self,
        serialized: Dict[str, Any],
        inputs: Dict[str, Any],
        *,
        run_id: str,
        **kwargs: Any
    ) -> Any:
        """Called when a chain starts (optional - for chain-level logging)"""
        if self.verbose:
            chain_name = serialized.get("name", "unknown_chain")
            print(f"🔗 AIM: Chain started - {chain_name}")

    def on_chain_end(
        self,
        outputs: Dict[str, Any],
        *,
        run_id: str,
        **kwargs: Any
    ) -> Any:
        """Called when a chain ends (optional - for chain-level logging)"""
        if self.verbose:
            print(f"✅ AIM: Chain completed")

    def on_chain_error(
        self,
        error: BaseException,
        *,
        run_id: str,
        **kwargs: Any
    ) -> Any:
        """Called when a chain fails (optional - for chain-level logging)"""
        if self.verbose:
            print(f"❌ AIM: Chain failed - {str(error)[:100]}")
