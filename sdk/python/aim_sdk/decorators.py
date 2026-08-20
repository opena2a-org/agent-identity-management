"""
AIM SDK - Universal Decorators

Provides decorators for seamless integration of AIM verification into any Python function.

Usage:
    from aim_sdk import AIMClient
    from aim_sdk.decorators import aim_verify

    # Initialize AIM client (auto-loads credentials)
    aim_client = AIMClient.from_credentials("my-agent")

    # Risk level is auto-detected from capability name!
    @aim_verify(aim_client, action_type="weather:fetch")  # auto: low
    def get_weather(city: str):
        return {"city": city, "temp": 72}

    @aim_verify(aim_client, action_type="db:delete")  # auto: high
    def delete_record(id: str):
        pass

    @aim_verify(aim_client, action_type="payment:process")  # auto: critical
    def charge_customer(amount: float):
        pass

    # You can still override when needed
    @aim_verify(aim_client, action_type="api:internal", risk_level="critical")
    def call_secret_api():
        pass

    # Function automatically verifies with AIM before execution
    result = get_weather("NYC")
"""

import functools
import time
import os
from typing import Any, Callable, Optional, Dict
from aim_sdk.client import AIMClient
from aim_sdk.decision import EnforcementMode
from aim_sdk.exceptions import ConfigurationError
from aim_sdk.risk_detector import detect_risk_level


def aim_verify(
    aim_client: Optional[AIMClient] = None,
    action_type: str = "function_call",
    risk_level: Optional[str] = None,
    resource: Optional[str] = None,
    auto_init: bool = True,
    agent_name: Optional[str] = None,
    aim_url: Optional[str] = None,
):
    """
    Universal decorator for verifying function calls with AIM.

    This decorator can be applied to ANY Python function to automatically verify
    execution with the AIM backend before the function runs.

    Risk Level Auto-Detection:
        If risk_level is not specified, it's automatically detected from the action_type
        based on well-established patterns:
        - ":read", ":fetch", ":list" -> "low"
        - ":write", ":update", ":create" -> "medium"
        - ":delete", ":send", ":execute" -> "high"
        - "payment:", "admin:", "system:" -> "critical"

    Args:
        aim_client: AIMClient instance (if None, will auto-initialize from env vars)
        action_type: Capability being performed in namespace:action format
                    (e.g., "weather:fetch", "db:read", "payment:process")
        risk_level: Risk level override ("low", "medium", "high", "critical")
                   If None, auto-detected from action_type
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
        >>> # Risk level auto-detected from capability name
        >>> @aim_verify(aim_client, action_type="weather:fetch")  # auto: low
        >>> def get_weather(city: str):
        >>>     return weather_api.fetch(city)
        >>>
        >>> @aim_verify(aim_client, action_type="db:delete")  # auto: high
        >>> def delete_user(user_id: str):
        >>>     db.execute("DELETE FROM users WHERE id = ?", user_id)
        >>>
        >>> # Override when you know better
        >>> @aim_verify(aim_client, action_type="api:internal", risk_level="critical")
        >>> def call_secret_api():
        >>>     return internal_api.call()

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
                # ConfigurationError, not a bare ValueError: this is the fifth
                # verification entry point and the SDK documents "catch
                # AIMError to handle every verification failure in one place".
                # A plain ValueError escapes that handler silently -- measured
                # by execution: `except AIMError: ...` around this call does
                # not catch it. This is a setup failure like any other
                # ConfigurationError (missing credentials, bad URL), not a
                # decision about the wrapped action, so it keeps that category
                # rather than joining the verification-outcome exceptions.
                raise ConfigurationError(
                    "AIM client not provided and auto_init failed. "
                    "Either pass aim_client parameter or set AIM_AGENT_NAME environment variable."
                )

            # Determine resource name
            resource_name = resource or f"{func.__module__}.{func.__name__}"

            # Auto-detect risk level from action_type if not explicitly provided
            # This uses the capability name to determine appropriate risk level
            effective_risk_level = detect_risk_level(action_type, override=risk_level)

            # Build context with function metadata and risk level
            context = {
                "function": func.__name__,
                "module": func.__module__,
                "args_count": len(args),
                "kwargs_keys": list(kwargs.keys()),
                "timestamp": int(time.time()),
                "risk_level": effective_risk_level,
                "risk_auto_detected": risk_level is None,  # Track if auto-detected
            }

            # Verify, then apply the one enforcement rule shared by every entry
            # point in this SDK. Three defects lived in what this block used to be:
            #
            #  (a) it read the enforcement mode from `verification["enforcementMode"]`,
            #      a key no return site in client.py ever emitted, so it fell back
            #      to "monitoring" on every call and the organization's dashboard
            #      setting had no effect on the Python SDK at all;
            #  (b) a denial raised ActionDeniedError, which was not a
            #      PermissionError, so it fell past `except PermissionError` into
            #      the blanket `except Exception` fail-open branch and the wrapped
            #      function ran -- no outage, no attacker, just the normal denial
            #      path;
            #  (c) it read the id as `verificationId`/`id` while the client emits
            #      `verification_id`, so every execution report returned early on a
            #      falsy id and AIM never received one.
            #
            # The blanket `except Exception` is GONE rather than narrowed. A
            # denial is routed by the mode check below, never by which handler
            # happens to catch it first -- ordering handlers is how (b) happened.
            verdict, decision = client._verify_and_enforce(
                capability=action_type,
                resource=resource_name,
                context=context,
                what=f"'{action_type}' in {func.__name__}",
            )
            verification_id = decision.verification_id
            strict = verdict.effective_mode is EnforcementMode.STRICT

            if verdict.blocked:
                client.report_execution_status(
                    verification_id=verification_id,
                    executed=False,
                    strict_mode=True,
                    execution_error=f"Blocked by AIM: {verdict.error}",
                )
                raise verdict.error

            try:
                result = func(*args, **kwargs)
            except Exception as exec_error:
                if verdict.reportable:
                    client.report_execution_status(
                        verification_id=verification_id,
                        executed=True,
                        strict_mode=strict,
                        execution_error=str(exec_error),
                    )
                raise
            if verdict.reportable:
                client.report_execution_status(
                    verification_id=verification_id,
                    executed=True,
                    strict_mode=strict,
                )
            return result

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
        from aim_sdk.console import console
        console.warning(f"Failed to initialize AIM client: {e}")
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
