"""
AIM SDK - CrewAI Integration

Seamless integration between AIM (Agent Identity Management) and CrewAI
for automatic verification and audit logging of multi-agent AI systems.

Available integrations:
- AIMCrewWrapper: Wrap entire crews with AIM verification
- aim_verified_task: Decorator for individual tasks
- AIMTaskCallback: Callback for task execution logging

Usage:
    from aim_sdk.integrations.crewai import AIMCrewWrapper

    verified_crew = AIMCrewWrapper(
        crew=my_crew,
        aim_agent=aim_client,
        risk_level="medium"
    )
    result = verified_crew.kickoff()
"""

from aim_sdk.integrations.crewai.wrapper import AIMCrewWrapper
from aim_sdk.integrations.crewai.decorators import aim_verified_task
from aim_sdk.integrations.crewai.callbacks import AIMTaskCallback

__all__ = [
    "AIMCrewWrapper",
    "aim_verified_task",
    "AIMTaskCallback",
]
