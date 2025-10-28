"""
AIM CrewAI Task Decorators

Decorators for adding AIM verification to CrewAI tasks.
"""

from typing import Any, Callable, Optional
from functools import wraps
import os

from aim_sdk.client import AIMClient


def aim_verified_task(
    agent: Optional[AIMClient] = None,
    action_name: Optional[str] = None,
    risk_level: str = "medium",
    auto_load_agent: str = "crewai-agent"
) -> Callable:
    """
    Decorator that adds AIM verification to CrewAI task functions.

    Wraps a task function to verify execution with AIM before running,
    and logs the result after completion.

    Example:
        from crewai import Task
        from aim_sdk import AIMClient
        from aim_sdk.integrations.crewai import aim_verified_task

        aim_client = AIMClient.auto_register_or_load("my-agent", "http://localhost:8080")

        @aim_verified_task(agent=aim_client, risk_level="high")
        def sensitive_research_task(context):
            '''Perform sensitive research operation'''
            # Task implementation
            return research_results

    Args:
        agent: AIMClient instance for verification (optional, will auto-load if not provided)
        action_name: Custom action name for AIM logs (default: "crewai_task:<function_name>")
        risk_level: Risk level for this task ("low", "medium", "high")
        auto_load_agent: Agent name to auto-load from credentials (default: "crewai-agent")

    Returns:
        Decorated function with AIM verification

    Raises:
        PermissionError: If AIM verification fails
    """
    def decorator(func: Callable) -> Callable:
        @wraps(func)
        def wrapper(*args, **kwargs) -> Any:
            # Get or load AIM agent
            _agent = agent
            if _agent is None:
                try:
                    _agent = AIMClient.from_credentials(auto_load_agent)
                except FileNotFoundError:
                    print(
                        f"⚠️  Warning: No AIM agent configured for task '{func.__name__}', "
                        f"running without verification"
                    )
                    # Run without verification if no agent available
                    return func(*args, **kwargs)

            # Determine action name
            _action_name = action_name or f"crewai_task:{func.__name__}"

            # Get resource (first argument if available)
            resource = ""
            if args:
                resource = str(args[0])[:100]
            elif kwargs:
                first_value = next(iter(kwargs.values()), "")
                resource = str(first_value)[:100]

            # Verify with AIM
            try:
                verification_result = _agent.verify_action(
                    action_type=_action_name,
                    resource=resource,
                    context={
                        "function": func.__name__,
                        "risk_level": risk_level,
                        "framework": "crewai",
                        "task_type": "crewai_task"
                    },
                    timeout_seconds=5
                )
            except Exception as e:
                raise PermissionError(
                    f"AIM verification failed for task '{_action_name}': {e}"
                )

            verification_id = verification_result.get("verification_id")

            # Execute task
            try:
                result = func(*args, **kwargs)

                # Log success to AIM
                if verification_id:
                    try:
                        _agent.log_action_result(
                            verification_id=verification_id,
                            success=True,
                            result_summary=f"Task '{func.__name__}' completed successfully"
                        )
                    except Exception as e:
                        # Don't fail the task if logging fails
                        print(f"⚠️  Warning: AIM result logging failed: {e}")

                return result

            except Exception as e:
                # Log failure to AIM
                if verification_id:
                    try:
                        _agent.log_action_result(
                            verification_id=verification_id,
                            success=False,
                            error_message=str(e)
                        )
                    except Exception as log_error:
                        print(f"⚠️  Warning: AIM error logging failed: {log_error}")

                # Re-raise the original exception
                raise

        return wrapper
    return decorator
