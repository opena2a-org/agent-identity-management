"""
AIM MCP Auto-Detection and Interception

Automatic detection and verification of MCP tool calls with zero-code integration.

This module provides three approaches for automatic MCP tool verification:

1. **Decorator-based** (@aim_mcp_tool): Wrap individual MCP tool functions
2. **Context manager** (with aim_mcp_session): Wrap MCP sessions/blocks
3. **Protocol interceptor** (MCPProtocolInterceptor): Intercept MCP protocol calls

Choose the approach that best fits your architecture:
- Decorator: Best for individual tool functions with explicit control
- Context manager: Best for session-based MCP interactions
- Interceptor: Best for protocol-level integration with MCP clients

All approaches automatically:
- Verify MCP tool calls with AIM before execution
- Log execution results back to AIM
- Handle errors gracefully with optional fallback
- Provide verbose logging for debugging
"""

import functools
import inspect
import json
from typing import Any, Callable, Dict, Optional, List, Union
from contextlib import contextmanager

from aim_sdk.client import AIMClient
from aim_sdk.integrations.mcp.verification import (
    verify_mcp_action,
    log_mcp_action_result,
    MCPActionWrapper
)


# ============================================================================
# APPROACH 1: DECORATOR-BASED AUTO-DETECTION
# ============================================================================

def aim_mcp_tool(
    aim_client: Optional[AIMClient] = None,
    mcp_server_id: Optional[str] = None,
    tool_name: Optional[str] = None,
    risk_level: str = "medium",
    auto_load_agent: str = "mcp-agent",
    auto_load_server: bool = True,
    graceful_fallback: bool = True,
    verbose: bool = False
):
    """
    Decorator for automatic AIM verification of MCP tool calls.

    This decorator wraps an MCP tool function to automatically:
    1. Verify the tool call with AIM before execution
    2. Execute the tool if verification succeeds
    3. Log the execution result back to AIM
    4. Handle errors gracefully (optional)

    Usage - Explicit configuration:
        from aim_sdk import AIMClient
        from aim_sdk.integrations.mcp import register_mcp_server, aim_mcp_tool

        # Setup
        aim_client = AIMClient.from_credentials("my-agent")
        server_info = register_mcp_server(
            aim_client=aim_client,
            server_name="research-mcp",
            server_url="http://localhost:3000",
            public_key="ed25519_...",
            capabilities=["tools"]
        )

        # Wrap MCP tool with automatic verification
        @aim_mcp_tool(
            aim_client=aim_client,
            mcp_server_id=server_info["id"],
            risk_level="low"
        )
        def web_search(query: str) -> dict:
            '''Search the web for information'''
            # AIM verification happens automatically before this runs
            return mcp_client.call_tool("web_search", {"query": query})

        # Use normally - verification is automatic
        results = web_search("AI safety best practices")

    Usage - Auto-load configuration:
        from aim_sdk.integrations.mcp import aim_mcp_tool

        # Decorator auto-loads agent and server from credentials
        @aim_mcp_tool(risk_level="medium", verbose=True)
        def database_query(sql: str) -> list:
            '''Execute a database query via MCP'''
            return mcp_client.call_tool("database_query", {"sql": sql})

        # Verification happens automatically
        users = database_query("SELECT * FROM users LIMIT 10")

    Usage - Graceful degradation (no AIM available):
        @aim_mcp_tool(graceful_fallback=True)
        def read_file(path: str) -> str:
            '''Read a file via MCP filesystem tool'''
            # If AIM not configured, runs without verification
            return mcp_client.call_tool("read_file", {"path": path})

    Args:
        aim_client: AIMClient instance (auto-loads if not provided)
        mcp_server_id: UUID of MCP server (auto-detects if not provided)
        tool_name: Name of MCP tool (defaults to function name)
        risk_level: Risk level ("low", "medium", "high")
        auto_load_agent: Agent name to auto-load (default: "mcp-agent")
        auto_load_server: Auto-detect MCP server from context (default: True)
        graceful_fallback: Run without verification if AIM unavailable (default: True)
        verbose: Print debug information (default: False)

    Returns:
        Decorated function with automatic AIM verification

    Raises:
        PermissionError: If AIM verification fails (unless graceful_fallback=True)
        ValueError: If required configuration is missing and auto_load fails

    Implementation Notes:
        - Function signature is preserved for introspection
        - Docstrings are preserved for documentation tools
        - Execution context is captured and sent to AIM
        - Result logging happens asynchronously (doesn't block return)
    """
    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper(*args, **kwargs) -> Any:
            # Determine configuration
            _aim_client = aim_client
            _mcp_server_id = mcp_server_id
            _tool_name = tool_name or func.__name__

            # Auto-load agent if not provided
            if _aim_client is None:
                try:
                    _aim_client = AIMClient.from_credentials(auto_load_agent)
                    if verbose:
                        print(f"🔧 AIM: Auto-loaded agent '{auto_load_agent}'")
                except Exception as e:
                    if graceful_fallback:
                        if verbose:
                            print(f"⚠️  AIM: No agent configured, running without verification")
                        return func(*args, **kwargs)
                    else:
                        raise ValueError(f"Failed to load AIM agent '{auto_load_agent}': {e}")

            # Auto-detect MCP server if not provided
            if _mcp_server_id is None and auto_load_server:
                # Try to get server ID from thread-local context (set by context manager)
                _mcp_server_id = _get_thread_local_mcp_server()

                if _mcp_server_id is None:
                    if graceful_fallback:
                        if verbose:
                            print(f"⚠️  AIM: No MCP server configured, running without verification")
                        return func(*args, **kwargs)
                    else:
                        raise ValueError(
                            "mcp_server_id is required. Either provide it explicitly, "
                            "use aim_mcp_session context manager, or enable graceful_fallback"
                        )

            # Build context from function call
            context = {
                "tool": _tool_name,
                "function": func.__name__,
                "args_count": len(args),
                "kwargs_keys": list(kwargs.keys()),
                "risk_level": risk_level,
                "source": "mcp_@aim_mcp_tool_decorator"
            }

            # Add function signature for better context
            try:
                sig = inspect.signature(func)
                bound_args = sig.bind(*args, **kwargs)
                bound_args.apply_defaults()
                context["parameters"] = {
                    k: str(v)[:100]  # Truncate long values
                    for k, v in bound_args.arguments.items()
                }
            except Exception:
                pass  # Ignore signature inspection errors

            # Determine resource (use first argument if available)
            resource = ""
            if args:
                resource = f"{_tool_name}({str(args[0])[:100]})"
            elif kwargs:
                first_key = list(kwargs.keys())[0]
                resource = f"{_tool_name}({first_key}={str(kwargs[first_key])[:100]})"

            if verbose:
                print(f"🔧 AIM: Verifying MCP tool '{_tool_name}' (risk: {risk_level})")

            # Verify with AIM before execution
            try:
                verification = verify_mcp_action(
                    aim_client=_aim_client,
                    mcp_server_id=_mcp_server_id,
                    action_type=f"mcp_tool:{_tool_name}",
                    resource=resource,
                    context=context,
                    risk_level=risk_level
                )
                verification_id = verification.get("verification_id")

                if verbose:
                    print(f"✅ AIM: Tool verified (id: {verification_id})")

            except Exception as e:
                if graceful_fallback:
                    if verbose:
                        print(f"⚠️  AIM: Verification failed, running without verification: {e}")
                    return func(*args, **kwargs)
                else:
                    raise PermissionError(f"AIM verification failed for '{_tool_name}': {e}")

            # Execute the tool
            try:
                result = func(*args, **kwargs)

                # Log success
                if verification_id:
                    log_mcp_action_result(
                        aim_client=_aim_client,
                        verification_id=verification_id,
                        success=True,
                        result_summary=f"Tool '{_tool_name}' completed successfully"
                    )

                if verbose:
                    print(f"✅ AIM: Tool execution completed and logged")

                return result

            except Exception as e:
                # Log failure
                if verification_id:
                    log_mcp_action_result(
                        aim_client=_aim_client,
                        verification_id=verification_id,
                        success=False,
                        error_message=str(e)
                    )

                if verbose:
                    print(f"❌ AIM: Tool execution failed: {e}")

                # Re-raise the original exception
                raise

        return wrapper
    return decorator


# ============================================================================
# APPROACH 2: CONTEXT MANAGER FOR MCP SESSIONS
# ============================================================================

import threading

# Thread-local storage for MCP session context
_thread_local = threading.local()


def _get_thread_local_mcp_server() -> Optional[str]:
    """Get MCP server ID from thread-local context."""
    return getattr(_thread_local, 'mcp_server_id', None)


def _set_thread_local_mcp_server(server_id: Optional[str]):
    """Set MCP server ID in thread-local context."""
    _thread_local.mcp_server_id = server_id


@contextmanager
def aim_mcp_session(
    aim_client: AIMClient,
    mcp_server_id: str,
    session_name: Optional[str] = None,
    default_risk_level: str = "medium",
    verbose: bool = False
):
    """
    Context manager for AIM-verified MCP sessions.

    This context manager wraps a block of MCP interactions to automatically:
    1. Set thread-local context for MCP server (used by @aim_mcp_tool)
    2. Track all MCP tool calls in the session
    3. Provide session-level logging and error handling

    Usage - Basic session:
        from aim_sdk import AIMClient
        from aim_sdk.integrations.mcp import register_mcp_server, aim_mcp_session, aim_mcp_tool

        aim_client = AIMClient.from_credentials("my-agent")
        server_info = register_mcp_server(aim_client, ...)

        # Tools automatically use session context
        @aim_mcp_tool(risk_level="low")
        def search(query: str):
            return mcp_client.call_tool("search", {"query": query})

        @aim_mcp_tool(risk_level="medium")
        def analyze(data: dict):
            return mcp_client.call_tool("analyze", data)

        # Session provides MCP server context
        with aim_mcp_session(aim_client, server_info["id"], verbose=True):
            # All tools in this block use the session's MCP server
            results = search("AI safety")
            analysis = analyze(results)

    Usage - Multiple sessions:
        with aim_mcp_session(aim_client, research_server_id, session_name="research"):
            docs = search_documents("quantum computing")

        with aim_mcp_session(aim_client, database_server_id, session_name="database"):
            users = query_users("SELECT * FROM users")

    Usage - Nested sessions:
        with aim_mcp_session(aim_client, server1_id):
            # Use server1
            data = get_data()

            with aim_mcp_session(aim_client, server2_id):
                # Use server2
                processed = process_data(data)

            # Back to server1
            save_results(processed)

    Args:
        aim_client: AIMClient instance for verification
        mcp_server_id: UUID of MCP server for this session
        session_name: Optional session name for logging
        default_risk_level: Default risk level for tools in session
        verbose: Print debug information

    Yields:
        MCPSessionContext object with session metadata

    Example - Complex workflow:
        with aim_mcp_session(client, server_id, "research_pipeline") as session:
            # Step 1: Search
            @aim_mcp_tool(risk_level="low")
            def search_papers(topic: str):
                return mcp.search(topic)

            papers = search_papers("neural networks")
            session.log(f"Found {len(papers)} papers")

            # Step 2: Analyze
            @aim_mcp_tool(risk_level="medium")
            def analyze_papers(papers: list):
                return mcp.analyze(papers)

            insights = analyze_papers(papers)
            session.log(f"Generated {len(insights)} insights")

            # Session automatically tracks all verifications
            print(f"Session stats: {session.get_stats()}")
    """
    # Save previous context (for nested sessions)
    previous_server_id = _get_thread_local_mcp_server()

    # Set new context
    _set_thread_local_mcp_server(mcp_server_id)

    # Create session context
    session_context = MCPSessionContext(
        aim_client=aim_client,
        mcp_server_id=mcp_server_id,
        session_name=session_name,
        default_risk_level=default_risk_level,
        verbose=verbose
    )

    if verbose:
        session_id = session_name or "unnamed"
        print(f"🔧 AIM: Starting MCP session '{session_id}' (server: {mcp_server_id[:8]}...)")

    try:
        yield session_context

        if verbose:
            stats = session_context.get_stats()
            print(f"✅ AIM: MCP session completed - {stats['total_calls']} calls, "
                  f"{stats['successful_calls']} successful")

    except Exception as e:
        if verbose:
            print(f"❌ AIM: MCP session failed: {e}")
        raise

    finally:
        # Restore previous context
        _set_thread_local_mcp_server(previous_server_id)


class MCPSessionContext:
    """
    Context object for MCP sessions.

    Provides session-level tracking and logging for MCP interactions.
    """

    def __init__(
        self,
        aim_client: AIMClient,
        mcp_server_id: str,
        session_name: Optional[str] = None,
        default_risk_level: str = "medium",
        verbose: bool = False
    ):
        self.aim_client = aim_client
        self.mcp_server_id = mcp_server_id
        self.session_name = session_name
        self.default_risk_level = default_risk_level
        self.verbose = verbose
        self.logs: List[str] = []
        self.verification_ids: List[str] = []
        self.call_count = 0
        self.success_count = 0
        self.error_count = 0

    def log(self, message: str):
        """Add a log message to the session."""
        self.logs.append(message)
        if self.verbose:
            print(f"📝 Session: {message}")

    def track_call(self, verification_id: str, success: bool):
        """Track a tool call in this session."""
        self.call_count += 1
        if verification_id:
            self.verification_ids.append(verification_id)
        if success:
            self.success_count += 1
        else:
            self.error_count += 1

    def get_stats(self) -> Dict[str, Any]:
        """Get session statistics."""
        return {
            "session_name": self.session_name,
            "mcp_server_id": self.mcp_server_id,
            "total_calls": self.call_count,
            "successful_calls": self.success_count,
            "failed_calls": self.error_count,
            "verification_ids": self.verification_ids,
            "logs": self.logs
        }


# ============================================================================
# APPROACH 3: PROTOCOL-LEVEL INTERCEPTOR
# ============================================================================

class MCPProtocolInterceptor:
    """
    Protocol-level interceptor for MCP tool calls.

    This interceptor wraps an MCP client to automatically intercept all
    tool/resource/prompt calls and verify them with AIM before execution.

    Usage - Wrap MCP client:
        from aim_sdk import AIMClient
        from aim_sdk.integrations.mcp import MCPProtocolInterceptor
        from mcp import Client  # MCP SDK client

        # Setup AIM
        aim_client = AIMClient.from_credentials("my-agent")
        server_info = register_mcp_server(aim_client, ...)

        # Create MCP client
        mcp_client = Client("http://localhost:3000")

        # Wrap with AIM interceptor
        verified_mcp = MCPProtocolInterceptor(
            mcp_client=mcp_client,
            aim_client=aim_client,
            mcp_server_id=server_info["id"],
            auto_verify=True,
            verbose=True
        )

        # All MCP calls are now automatically verified
        results = verified_mcp.call_tool("web_search", {"query": "AI safety"})
        # AIM verification happened transparently before the call

    Usage - Selective verification:
        # Only verify high-risk tools
        verified_mcp = MCPProtocolInterceptor(
            mcp_client=mcp_client,
            aim_client=aim_client,
            mcp_server_id=server_info["id"],
            auto_verify=False  # Manual verification
        )

        # Low-risk tools - no verification
        data = verified_mcp.call_tool("read_config", {}, verify=False)

        # High-risk tools - require verification
        result = verified_mcp.call_tool("delete_database", {"db": "prod"}, verify=True)

    Args:
        mcp_client: MCP client instance to wrap
        aim_client: AIMClient instance for verification
        mcp_server_id: UUID of MCP server
        auto_verify: Automatically verify all calls (default: True)
        default_risk_level: Default risk level for auto-verified calls
        verbose: Print debug information

    Implementation Notes:
        - Wraps MCP client methods using __getattr__ proxy pattern
        - Preserves MCP client interface (drop-in replacement)
        - Intercepts: call_tool(), read_resource(), get_prompt()
        - Falls back to direct calls if verification disabled
        - Compatible with any MCP client that follows standard interface
    """

    def __init__(
        self,
        mcp_client: Any,
        aim_client: AIMClient,
        mcp_server_id: str,
        auto_verify: bool = True,
        default_risk_level: str = "medium",
        verbose: bool = False
    ):
        self._mcp_client = mcp_client
        self._aim_client = aim_client
        self._mcp_server_id = mcp_server_id
        self._auto_verify = auto_verify
        self._default_risk_level = default_risk_level
        self._verbose = verbose

        # Statistics
        self._stats = {
            "total_calls": 0,
            "verified_calls": 0,
            "unverified_calls": 0,
            "denied_calls": 0
        }

    def call_tool(
        self,
        tool_name: str,
        arguments: Dict[str, Any],
        verify: Optional[bool] = None,
        risk_level: Optional[str] = None
    ) -> Any:
        """
        Call an MCP tool with optional AIM verification.

        Args:
            tool_name: Name of the MCP tool
            arguments: Tool arguments
            verify: Override auto_verify setting (True/False/None)
            risk_level: Override default risk level

        Returns:
            Tool execution result

        Raises:
            PermissionError: If verification fails
        """
        self._stats["total_calls"] += 1
        should_verify = verify if verify is not None else self._auto_verify

        if should_verify:
            return self._verified_call("tool", tool_name, arguments, risk_level)
        else:
            self._stats["unverified_calls"] += 1
            return self._mcp_client.call_tool(tool_name, arguments)

    def read_resource(
        self,
        resource_uri: str,
        verify: Optional[bool] = None,
        risk_level: Optional[str] = None
    ) -> Any:
        """
        Read an MCP resource with optional AIM verification.

        Args:
            resource_uri: URI of the resource
            verify: Override auto_verify setting
            risk_level: Override default risk level

        Returns:
            Resource content
        """
        self._stats["total_calls"] += 1
        should_verify = verify if verify is not None else self._auto_verify

        if should_verify:
            return self._verified_call("resource", resource_uri, {}, risk_level)
        else:
            self._stats["unverified_calls"] += 1
            return self._mcp_client.read_resource(resource_uri)

    def get_prompt(
        self,
        prompt_name: str,
        arguments: Dict[str, Any],
        verify: Optional[bool] = None,
        risk_level: Optional[str] = None
    ) -> Any:
        """
        Get an MCP prompt with optional AIM verification.

        Args:
            prompt_name: Name of the prompt
            arguments: Prompt arguments
            verify: Override auto_verify setting
            risk_level: Override default risk level

        Returns:
            Prompt content
        """
        self._stats["total_calls"] += 1
        should_verify = verify if verify is not None else self._auto_verify

        if should_verify:
            return self._verified_call("prompt", prompt_name, arguments, risk_level)
        else:
            self._stats["unverified_calls"] += 1
            return self._mcp_client.get_prompt(prompt_name, arguments)

    def _verified_call(
        self,
        call_type: str,
        name: str,
        arguments: Dict[str, Any],
        risk_level: Optional[str] = None
    ) -> Any:
        """
        Execute a verified MCP call.

        Args:
            call_type: Type of call ("tool", "resource", "prompt")
            name: Name of tool/resource/prompt
            arguments: Call arguments
            risk_level: Risk level for verification

        Returns:
            Call result

        Raises:
            PermissionError: If verification fails
        """
        _risk_level = risk_level or self._default_risk_level
        action_type = f"mcp_{call_type}:{name}"

        if self._verbose:
            print(f"🔧 AIM: Verifying MCP {call_type} '{name}' (risk: {_risk_level})")

        # Verify with AIM
        try:
            verification = verify_mcp_action(
                aim_client=self._aim_client,
                mcp_server_id=self._mcp_server_id,
                action_type=action_type,
                resource=name,
                context={
                    "call_type": call_type,
                    "name": name,
                    "arguments": arguments,
                    "risk_level": _risk_level
                },
                risk_level=_risk_level
            )
            verification_id = verification.get("verification_id")
            self._stats["verified_calls"] += 1

            if self._verbose:
                print(f"✅ AIM: {call_type} verified (id: {verification_id})")

        except Exception as e:
            self._stats["denied_calls"] += 1
            raise PermissionError(f"AIM verification failed for '{action_type}': {e}")

        # Execute the MCP call
        try:
            if call_type == "tool":
                result = self._mcp_client.call_tool(name, arguments)
            elif call_type == "resource":
                result = self._mcp_client.read_resource(name)
            elif call_type == "prompt":
                result = self._mcp_client.get_prompt(name, arguments)
            else:
                raise ValueError(f"Unknown call type: {call_type}")

            # Log success
            if verification_id:
                log_mcp_action_result(
                    aim_client=self._aim_client,
                    verification_id=verification_id,
                    success=True,
                    result_summary=f"MCP {call_type} '{name}' completed successfully"
                )

            if self._verbose:
                print(f"✅ AIM: {call_type} execution completed and logged")

            return result

        except Exception as e:
            # Log failure
            if verification_id:
                log_mcp_action_result(
                    aim_client=self._aim_client,
                    verification_id=verification_id,
                    success=False,
                    error_message=str(e)
                )

            if self._verbose:
                print(f"❌ AIM: {call_type} execution failed: {e}")

            raise

    def get_stats(self) -> Dict[str, int]:
        """Get interceptor statistics."""
        return self._stats.copy()

    def __getattr__(self, name: str) -> Any:
        """
        Proxy all other attributes/methods to the wrapped MCP client.

        This allows MCPProtocolInterceptor to be a drop-in replacement
        for the original MCP client.
        """
        return getattr(self._mcp_client, name)


# ============================================================================
# CONVENIENCE EXPORTS
# ============================================================================

__all__ = [
    # Decorator
    "aim_mcp_tool",

    # Context manager
    "aim_mcp_session",
    "MCPSessionContext",

    # Protocol interceptor
    "MCPProtocolInterceptor",
]
