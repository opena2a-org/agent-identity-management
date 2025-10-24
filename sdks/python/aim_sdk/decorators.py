"""
AIM SDK - Universal Decorators

Provides decorators for seamless integration of AIM verification into any Python function.

Usage:
    from aim_sdk import AIMClient
    from aim_sdk.decorators import aim_verify

    # Initialize AIM client (auto-loads credentials)
    aim_client = AIMClient.from_credentials("my-agent")

    @aim_verify(aim_client, action_type="api_call", risk_level="medium")
    def fetch_user_data(user_id: str):
        # Your function code here
        return {"user_id": user_id, "name": "John Doe"}

    # Function automatically verifies with AIM before execution
    result = fetch_user_data("user123")
"""

import functools
import time
import os
from typing import Any, Callable, Optional, Dict
from aim_sdk.client import AIMClient


def aim_verify(
    aim_client: Optional[AIMClient] = None,
    action_type: str = "function_call",
    risk_level: str = "low",
    resource: Optional[str] = None,
    auto_init: bool = True,
    agent_name: Optional[str] = None,
    aim_url: Optional[str] = None,
):
    """
    Universal decorator for verifying function calls with AIM.

    This decorator can be applied to ANY Python function to automatically verify
    execution with the AIM backend before the function runs.

    Args:
        aim_client: AIMClient instance (if None, will auto-initialize from env vars)
        action_type: Type of action being performed (e.g., "api_call", "database_query")
        risk_level: Risk level of the action ("low", "medium", "high", "critical")
        resource: Resource being accessed (defaults to function name)
        auto_init: If True, automatically initialize AIM client from environment variables
        agent_name: Agent name for auto-initialization (uses AIM_AGENT_NAME env var if not provided)
        aim_url: AIM backend URL for auto-initialization (uses AIM_URL env var if not provided)

    Environment Variables (used when auto_init=True):
        AIM_AGENT_NAME: Agent name for auto-registration
        AIM_URL: AIM backend URL (default: http://localhost:8080)
        AIM_AUTO_REGISTER: Whether to auto-register if credentials not found (default: true)

    Example:
        >>> from aim_sdk.decorators import aim_verify
        >>>
        >>> # Option 1: Explicit client
        >>> aim_client = AIMClient.auto_register_or_load("my-agent", "http://localhost:8080")
        >>> @aim_verify(aim_client, action_type="database_query", risk_level="high")
        >>> def delete_user(user_id: str):
        >>>     db.execute("DELETE FROM users WHERE id = ?", user_id)
        >>>
        >>> # Option 2: Auto-initialization from environment
        >>> os.environ["AIM_AGENT_NAME"] = "my-agent"
        >>> os.environ["AIM_URL"] = "http://localhost:8080"
        >>> @aim_verify(auto_init=True)
        >>> def send_email(to: str, subject: str):
        >>>     email_service.send(to, subject)

    Returns:
        Decorated function that performs AIM verification before execution
    """

    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper(*args, **kwargs) -> Any:
            # Get or initialize AIM client
            client = aim_client
            if client is None and auto_init:
                client = _get_or_create_client(agent_name, aim_url)

            if client is None:
                raise ValueError(
                    "AIM client not provided and auto_init failed. "
                    "Either pass aim_client parameter or set AIM_AGENT_NAME environment variable."
                )

            # Determine resource name
            resource_name = resource or f"{func.__module__}.{func.__name__}"

            # Build context with function metadata
            context = {
                "function": func.__name__,
                "module": func.__module__,
                "args_count": len(args),
                "kwargs_keys": list(kwargs.keys()),
                "timestamp": int(time.time()),
            }

            # Perform verification
            try:
                verification = client.verify_action(
                    action_type=action_type,
                    resource=resource_name,
                    context=context,
                    risk_level=risk_level,
                )

                # Check if verification succeeded
                if not verification.get("allowed", False):
                    raise PermissionError(
                        f"AIM verification failed for {func.__name__}: "
                        f"{verification.get('reason', 'Unknown reason')}"
                    )

                # Execute the original function
                return func(*args, **kwargs)

            except Exception as e:
                # Log verification failure but don't block execution in development
                if os.getenv("AIM_STRICT_MODE", "false").lower() == "true":
                    raise  # Strict mode: block execution on verification failure
                else:
                    print(f"⚠️  AIM verification warning: {e}")
                    return func(*args, **kwargs)  # Continue execution

        return wrapper

    return decorator


def _get_or_create_client(agent_name: Optional[str] = None, aim_url: Optional[str] = None) -> Optional[AIMClient]:
    """
    Get or create AIM client from environment variables.

    Environment Variables:
        AIM_AGENT_NAME: Agent name for registration
        AIM_URL: AIM backend URL (default: http://localhost:8080)
        AIM_AUTO_REGISTER: Auto-register if credentials not found (default: true)

    Args:
        agent_name: Override agent name (uses AIM_AGENT_NAME if not provided)
        aim_url: Override AIM URL (uses AIM_URL if not provided)

    Returns:
        AIMClient instance or None if initialization failed
    """
    try:
        # Get configuration from environment
        name = agent_name or os.getenv("AIM_AGENT_NAME")
        url = aim_url or os.getenv("AIM_URL", "http://localhost:8080")
        auto_register = os.getenv("AIM_AUTO_REGISTER", "true").lower() == "true"

        if not name:
            return None

        # Try to load existing credentials first
        try:
            return AIMClient.from_credentials(name)
        except FileNotFoundError:
            # No credentials found
            if auto_register:
                # Auto-register new agent
                return AIMClient.auto_register_or_load(name, url)
            else:
                return None

    except Exception as e:
        print(f"⚠️  Failed to initialize AIM client: {e}")
        return None


# Convenience decorator with common presets
def aim_verify_api_call(
    aim_client: Optional[AIMClient] = None,
    risk_level: str = "medium",
    **kwargs
):
    """Convenience decorator for API calls."""
    return aim_verify(aim_client, action_type="api_call", risk_level=risk_level, **kwargs)


def aim_verify_database(
    aim_client: Optional[AIMClient] = None,
    risk_level: str = "high",
    **kwargs
):
    """Convenience decorator for database operations."""
    return aim_verify(aim_client, action_type="database_query", risk_level=risk_level, **kwargs)


def aim_verify_file_access(
    aim_client: Optional[AIMClient] = None,
    risk_level: str = "medium",
    **kwargs
):
    """Convenience decorator for file operations."""
    return aim_verify(aim_client, action_type="file_access", risk_level=risk_level, **kwargs)


def aim_verify_external_service(
    aim_client: Optional[AIMClient] = None,
    risk_level: str = "high",
    **kwargs
):
    """Convenience decorator for external service calls."""
    return aim_verify(aim_client, action_type="external_service", risk_level=risk_level, **kwargs)
