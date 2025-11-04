"""
AIM Decorators for LangChain

Decorators to add AIM verification to LangChain tools.
"""

from functools import wraps
from typing import Callable, Optional, Any
from aim_sdk import AIMClient


def aim_verify(
    agent: Optional[AIMClient] = None,
    action_name: Optional[str] = None,
    risk_level: str = "medium",
    resource: Optional[str] = None,
    auto_load_agent: str = "langchain-agent"
):
    """
    Decorator to add AIM verification to LangChain tools.

    This decorator wraps a tool function to:
    1. Verify the action with AIM before execution
    2. Execute the tool if verification succeeds
    3. Log the result back to AIM
    4. Raise PermissionError if verification fails

    Usage with explicit agent:
        from aim_sdk import AIMClient
        from aim_sdk.integrations.langchain import aim_verify
        from langchain_core.tools import tool

        aim_client = AIMClient.auto_register_or_load("my-agent", "https://aim.company.com")

        @tool
        @aim_verify(agent=aim_client, risk_level="high")
        def delete_user(user_id: str) -> str:
            '''Delete a user from the database'''
            # AIM verification happens automatically before this code runs
            return f"Deleted user {user_id}"

    Usage with auto-loaded agent:
        @tool
        @aim_verify(risk_level="medium")
        def query_database(query: str) -> str:
            '''Execute a database query'''
            # Agent auto-loaded from ~/.aim/credentials.json
            return execute_query(query)

    Usage without AIM (graceful degradation):
        # If no agent configured, tool runs without verification
        @tool
        @aim_verify()
        def read_file(filename: str) -> str:
            '''Read a file'''
            return open(filename).read()

    Args:
        agent: AIMClient instance (optional - will auto-load if not provided)
        action_name: Custom action name (default: "langchain_tool:<function_name>")
        risk_level: Risk level for the action ("low", "medium", "high")
        resource: Resource being accessed (default: first argument of function)
        auto_load_agent: Agent name to auto-load if agent not provided (default: "langchain-agent")

    Returns:
        Decorated function with AIM verification

    Raises:
        PermissionError: If AIM verification fails
    """
    def decorator(func: Callable) -> Callable:
        @wraps(func)
        def wrapper(*args, **kwargs) -> Any:
            # Determine which agent to use
            _agent = agent
            if _agent is None:
                # Try to auto-load agent
                try:
                    _agent = AIMClient.from_credentials(auto_load_agent)
                except FileNotFoundError:
                    # No AIM agent configured - run without verification (graceful degradation)
                    print(f"⚠️  Warning: No AIM agent configured for {func.__name__}, running without verification")
                    return func(*args, **kwargs)

            # Determine action name
            _action_name = action_name or f"langchain_tool:{func.__name__}"

            # Determine resource (use first argument if not specified)
            _resource = resource
            if _resource is None and args:
                _resource = str(args[0])[:100]  # First 100 chars of first arg

            # Verify with AIM before execution
            try:
                verification_result = _agent.verify_action(
                    action_type=_action_name,
                    resource=_resource or "",
                    context={
                        "function": func.__name__,
                        "args_count": len(args),
                        "kwargs_keys": list(kwargs.keys()),
                        "risk_level": risk_level,
                        "source": "langchain_@aim_verify_decorator"
                    },
                    timeout_seconds=5  # Quick verification
                )
            except Exception as e:
                # Verification failed - raise permission error
                raise PermissionError(f"AIM verification failed for '{_action_name}': {e}")

            # Verification succeeded - execute the function
            verification_id = verification_result.get("verification_id")

            try:
                result = func(*args, **kwargs)

                # Log successful completion
                if verification_id:
                    try:
                        _agent.log_action_result(
                            verification_id=verification_id,
                            success=True,
                            result_summary=f"Tool '{func.__name__}' completed successfully"
                        )
                    except Exception:
                        pass  # Don't fail on logging errors

                return result

            except Exception as e:
                # Log failure
                if verification_id:
                    try:
                        _agent.log_action_result(
                            verification_id=verification_id,
                            success=False,
                            error_message=str(e)
                        )
                    except Exception:
                        pass  # Don't fail on logging errors

                # Re-raise the original exception
                raise

        return wrapper
    return decorator
