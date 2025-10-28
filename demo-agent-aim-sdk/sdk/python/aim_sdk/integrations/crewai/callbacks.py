"""
AIM CrewAI Task Callbacks

Callback handlers for automatic logging of CrewAI task executions.
"""

from typing import Any, Dict, Optional
import time

from aim_sdk.client import AIMClient


class AIMTaskCallback:
    """
    Callback handler for automatic AIM logging of CrewAI task executions.

    This callback can be attached to CrewAI tasks to automatically log
    all task executions, completions, and failures to AIM.

    Example:
        from crewai import Agent, Task, Crew
        from aim_sdk import AIMClient
        from aim_sdk.integrations.crewai import AIMTaskCallback

        aim_client = AIMClient.auto_register_or_load("my-crew", "http://localhost:8080")
        aim_callback = AIMTaskCallback(agent=aim_client, verbose=True)

        # Tasks will be logged automatically
        research_task = Task(
            description="Research AI safety",
            agent=researcher,
            callback=aim_callback.on_task_complete
        )

    Note:
        CrewAI's callback system is simpler than LangChain's.
        This implementation provides basic task completion logging.
    """

    def __init__(
        self,
        agent: AIMClient,
        log_inputs: bool = True,
        log_outputs: bool = True,
        verbose: bool = False
    ):
        """
        Initialize AIM Task Callback.

        Args:
            agent: AIMClient instance for logging
            log_inputs: Whether to log task inputs
            log_outputs: Whether to log task outputs
            verbose: Whether to print debug information
        """
        self.agent = agent
        self.log_inputs = log_inputs
        self.log_outputs = log_outputs
        self.verbose = verbose
        self._task_start_times: Dict[str, float] = {}

    def on_task_start(self, task: Any, inputs: Optional[Dict[str, Any]] = None) -> None:
        """
        Called when a task starts execution.

        Args:
            task: CrewAI Task instance
            inputs: Task inputs (if available)
        """
        task_id = id(task)
        self._task_start_times[task_id] = time.time()

        if self.verbose:
            task_desc = getattr(task, 'description', 'unknown task')
            print(f"🔧 AIM: Task started - {task_desc[:50]}")

    def on_task_complete(self, output: Any) -> None:
        """
        Called when a task completes successfully.

        This method can be used as the callback parameter in CrewAI tasks.

        Args:
            output: Task output/result
        """
        if self.verbose:
            print(f"✅ AIM: Task completed")

        # Log to AIM
        try:
            # Get output summary
            output_summary = "[hidden]"
            if self.log_outputs:
                if isinstance(output, str):
                    output_summary = output[:500]
                else:
                    output_summary = str(output)[:500]

            # Log as action (no verification_id since this is post-execution logging)
            self.agent.verify_action(
                action_type="crewai_task:completed",
                resource="",
                context={
                    "output_summary": output_summary,
                    "status": "completed",
                    "framework": "crewai"
                },
                timeout_seconds=1
            )

            if self.verbose:
                print("✅ AIM: Task completion logged")

        except Exception as e:
            if self.verbose:
                print(f"⚠️  AIM logging error: {e}")

    def on_task_error(self, error: Exception, task: Optional[Any] = None) -> None:
        """
        Called when a task fails with an error.

        Args:
            error: Exception that occurred
            task: CrewAI Task instance (if available)
        """
        if self.verbose:
            task_desc = "unknown task"
            if task:
                task_desc = getattr(task, 'description', 'unknown task')
            print(f"❌ AIM: Task failed - {task_desc[:50]}: {error}")

        # Log error to AIM
        try:
            self.agent.verify_action(
                action_type="crewai_task:failed",
                resource="",
                context={
                    "error": str(error),
                    "error_type": type(error).__name__,
                    "status": "failed",
                    "framework": "crewai"
                },
                timeout_seconds=1
            )

            if self.verbose:
                print("✅ AIM: Task failure logged")

        except Exception as e:
            if self.verbose:
                print(f"⚠️  AIM logging error: {e}")
