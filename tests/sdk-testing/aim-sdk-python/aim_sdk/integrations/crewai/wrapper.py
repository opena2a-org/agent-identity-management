"""
AIM CrewAI Crew Wrapper

Wraps entire CrewAI crews with AIM verification and logging.
"""

from typing import Any, Dict, List, Optional, Union
from functools import wraps
import json

try:
    from crewai import Crew
except ImportError:
    raise ImportError(
        "CrewAI is required for this integration. "
        "Install it with: pip install crewai"
    )

from aim_sdk.client import AIMClient


class AIMCrewWrapper:
    """
    Wrapper for CrewAI Crew that adds AIM verification and logging.

    Automatically verifies and logs all crew executions through AIM,
    providing audit trails and security verification for multi-agent systems.

    Example:
        from crewai import Crew, Agent, Task
        from aim_sdk import AIMClient
        from aim_sdk.integrations.crewai import AIMCrewWrapper

        # Create crew
        crew = Crew(
            agents=[researcher, writer],
            tasks=[research_task, write_task]
        )

        # Wrap with AIM
        aim_client = AIMClient.auto_register_or_load("my-crew", "http://localhost:8080")
        verified_crew = AIMCrewWrapper(
            crew=crew,
            aim_agent=aim_client,
            risk_level="medium"
        )

        # All executions automatically verified and logged
        result = verified_crew.kickoff(inputs={"topic": "AI safety"})
    """

    def __init__(
        self,
        crew: Crew,
        aim_agent: AIMClient,
        risk_level: str = "medium",
        log_inputs: bool = True,
        log_outputs: bool = True,
        verbose: bool = False
    ):
        """
        Initialize AIM Crew Wrapper.

        Args:
            crew: CrewAI Crew instance to wrap
            aim_agent: AIMClient instance for verification
            risk_level: Risk level for crew execution ("low", "medium", "high")
            log_inputs: Whether to log crew inputs to AIM
            log_outputs: Whether to log crew outputs to AIM
            verbose: Whether to print debug information
        """
        self.crew = crew
        self.aim_agent = aim_agent
        self.risk_level = risk_level
        self.log_inputs = log_inputs
        self.log_outputs = log_outputs
        self.verbose = verbose

    def _sanitize_for_logging(self, data: Any, max_length: int = 500) -> str:
        """
        Sanitize data for logging (convert to string, truncate if needed).

        Args:
            data: Data to sanitize
            max_length: Maximum length of sanitized string

        Returns:
            Sanitized string representation
        """
        try:
            if isinstance(data, (dict, list)):
                data_str = json.dumps(data, indent=2)
            else:
                data_str = str(data)

            if len(data_str) > max_length:
                return data_str[:max_length] + "... [truncated]"
            return data_str
        except Exception:
            return "[unable to serialize]"

    def kickoff(self, inputs: Optional[Dict[str, Any]] = None) -> Any:
        """
        Execute crew with AIM verification and logging.

        Args:
            inputs: Inputs to pass to the crew

        Returns:
            Crew execution result

        Raises:
            PermissionError: If AIM verification fails
        """
        if self.verbose:
            print(f"🔧 AIM: Verifying crew execution (risk: {self.risk_level})")

        # Prepare resource for verification
        resource = ""
        if inputs and self.log_inputs:
            resource = self._sanitize_for_logging(inputs, max_length=100)

        # Verify with AIM
        try:
            verification_result = self.aim_agent.verify_action(
                action_type="crewai_crew:kickoff",
                resource=resource,
                context={
                    "crew_agents": len(self.crew.agents) if hasattr(self.crew, 'agents') else 0,
                    "crew_tasks": len(self.crew.tasks) if hasattr(self.crew, 'tasks') else 0,
                    "risk_level": self.risk_level,
                    "framework": "crewai"
                },
                timeout_seconds=5
            )
        except Exception as e:
            if self.verbose:
                print(f"⚠️  AIM verification error: {e}")
            raise PermissionError(f"AIM verification failed for crew execution: {e}")

        verification_id = verification_result.get("verification_id")

        if self.verbose:
            print(f"✅ AIM: Crew execution verified (id: {verification_id})")

        # Execute crew
        try:
            result = self.crew.kickoff(inputs=inputs)

            # Log success to AIM
            if verification_id:
                result_summary = "Crew execution completed successfully"
                if self.log_outputs:
                    result_summary = self._sanitize_for_logging(result, max_length=500)

                try:
                    self.aim_agent.log_action_result(
                        verification_id=verification_id,
                        success=True,
                        result_summary=result_summary
                    )
                except Exception as e:
                    if self.verbose:
                        print(f"⚠️  AIM result logging error: {e}")

            if self.verbose:
                print("✅ AIM: Crew execution completed and logged")

            return result

        except Exception as e:
            # Log failure to AIM
            if verification_id:
                try:
                    self.aim_agent.log_action_result(
                        verification_id=verification_id,
                        success=False,
                        error_message=str(e)
                    )
                except Exception as log_error:
                    if self.verbose:
                        print(f"⚠️  AIM result logging error: {log_error}")

            if self.verbose:
                print(f"❌ AIM: Crew execution failed: {e}")

            raise

    async def kickoff_async(self, inputs: Optional[Dict[str, Any]] = None) -> Any:
        """
        Execute crew asynchronously with AIM verification and logging.

        Args:
            inputs: Inputs to pass to the crew

        Returns:
            Crew execution result

        Raises:
            PermissionError: If AIM verification fails
        """
        if self.verbose:
            print(f"🔧 AIM: Verifying async crew execution (risk: {self.risk_level})")

        # Prepare resource for verification
        resource = ""
        if inputs and self.log_inputs:
            resource = self._sanitize_for_logging(inputs, max_length=100)

        # Verify with AIM
        try:
            verification_result = self.aim_agent.verify_action(
                action_type="crewai_crew:kickoff_async",
                resource=resource,
                context={
                    "crew_agents": len(self.crew.agents) if hasattr(self.crew, 'agents') else 0,
                    "crew_tasks": len(self.crew.tasks) if hasattr(self.crew, 'tasks') else 0,
                    "risk_level": self.risk_level,
                    "framework": "crewai",
                    "async": True
                },
                timeout_seconds=5
            )
        except Exception as e:
            if self.verbose:
                print(f"⚠️  AIM verification error: {e}")
            raise PermissionError(f"AIM verification failed for async crew execution: {e}")

        verification_id = verification_result.get("verification_id")

        if self.verbose:
            print(f"✅ AIM: Async crew execution verified (id: {verification_id})")

        # Execute crew asynchronously
        try:
            result = await self.crew.kickoff_async(inputs=inputs)

            # Log success to AIM
            if verification_id:
                result_summary = "Async crew execution completed successfully"
                if self.log_outputs:
                    result_summary = self._sanitize_for_logging(result, max_length=500)

                try:
                    self.aim_agent.log_action_result(
                        verification_id=verification_id,
                        success=True,
                        result_summary=result_summary
                    )
                except Exception as e:
                    if self.verbose:
                        print(f"⚠️  AIM result logging error: {e}")

            if self.verbose:
                print("✅ AIM: Async crew execution completed and logged")

            return result

        except Exception as e:
            # Log failure to AIM
            if verification_id:
                try:
                    self.aim_agent.log_action_result(
                        verification_id=verification_id,
                        success=False,
                        error_message=str(e)
                    )
                except Exception as log_error:
                    if self.verbose:
                        print(f"⚠️  AIM result logging error: {log_error}")

            if self.verbose:
                print(f"❌ AIM: Async crew execution failed: {e}")

            raise

    def __getattr__(self, name: str) -> Any:
        """
        Proxy all other attributes to the wrapped crew.

        This allows the wrapper to behave like the original crew
        for all attributes and methods not explicitly overridden.
        """
        return getattr(self.crew, name)
