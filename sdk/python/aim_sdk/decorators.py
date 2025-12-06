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
                raise ValueError(
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

            # Environment variable override for strict mode (useful for testing)
            # If set to 'true', always use strict mode regardless of backend setting
            # If set to 'false', always use monitoring mode regardless of backend setting
            # If not set or any other value, use backend's enforcement mode
            env_strict_override = os.getenv("AIM_STRICT_MODE")
            verification_id = None

            # Perform verification
            try:
                verification = client.verify_capability(
                    capability=action_type,
                    resource=resource_name,
                    context=context,
                )

                # Capture verification ID for execution status reporting
                verification_id = verification.get("verificationId") or verification.get("id")

                # Determine enforcement mode from backend response (or env override)
                backend_enforcement_mode = verification.get("enforcementMode", "monitoring")

                if env_strict_override is not None:
                    # Environment override takes precedence
                    strict_mode = env_strict_override.lower() == "true"
                else:
                    # Use backend enforcement mode setting
                    strict_mode = backend_enforcement_mode == "strict"

                # Check if verification succeeded
                if not verification.get("verified", False):
                    denial_reason = verification.get('reason', verification.get('error', 'Unknown reason'))

                    if strict_mode:
                        # Strict mode: block execution and report it
                        client.report_execution_status(
                            verification_id=verification_id,
                            executed=False,
                            strict_mode=True,
                            execution_error=f"Blocked by strict mode: {denial_reason}"
                        )
                        raise PermissionError(
                            f"AIM verification failed for {func.__name__}: {denial_reason}"
                        )
                    else:
                        # Monitoring mode: warn but continue execution
                        print(f"⚠️  AIM verification warning: {denial_reason}")
                        try:
                            result = func(*args, **kwargs)
                            # Report that we executed despite denial (monitoring mode)
                            client.report_execution_status(
                                verification_id=verification_id,
                                executed=True,
                                strict_mode=False
                            )
                            return result
                        except Exception as exec_error:
                            # Report execution failure
                            client.report_execution_status(
                                verification_id=verification_id,
                                executed=True,
                                strict_mode=False,
                                execution_error=str(exec_error)
                            )
                            raise

                # Verification succeeded - execute the original function
                try:
                    result = func(*args, **kwargs)
                    # Report successful execution
                    client.report_execution_status(
                        verification_id=verification_id,
                        executed=True,
                        strict_mode=strict_mode
                    )
                    return result
                except Exception as exec_error:
                    # Report execution failure (function threw an error)
                    client.report_execution_status(
                        verification_id=verification_id,
                        executed=True,
                        strict_mode=strict_mode,
                        execution_error=str(exec_error)
                    )
                    raise

            except PermissionError:
                # Re-raise permission errors (already reported above)
                raise
            except Exception as e:
                # Verification itself failed (network error, etc.)
                # Determine strict mode for error case (use env override or default to monitoring for safety)
                strict_mode = env_strict_override.lower() == "true" if env_strict_override else False

                if strict_mode:
                    # In strict mode, verification failure blocks execution
                    client.report_execution_status(
                        verification_id=verification_id,
                        executed=False,
                        strict_mode=True,
                        execution_error=f"Verification error: {str(e)}"
                    )
                    raise
                else:
                    # Monitoring mode: warn but continue execution
                    print(f"⚠️  AIM verification warning: {e}")
                    try:
                        result = func(*args, **kwargs)
                        # Report that we executed despite verification error
                        client.report_execution_status(
                            verification_id=verification_id,
                            executed=True,
                            strict_mode=False
                        )
                        return result
                    except Exception as exec_error:
                        client.report_execution_status(
                            verification_id=verification_id,
                            executed=True,
                            strict_mode=False,
                            execution_error=str(exec_error)
                        )
                        raise

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
