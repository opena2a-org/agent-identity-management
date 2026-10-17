#!/usr/bin/env python3
"""
Integration tests for AIM + CrewAI

Tests all three integration patterns:
1. AIMCrewWrapper - Wrap entire crews
2. @aim_verified_task - Explicit task verification
3. AIMTaskCallback - Callback for task logging

Each test asserts, through the backend's API (GET
/api/v1/verification-events/agent/{id}), that the backend recorded one
verification event, with the agent's id and the verified action, for each
framework call the test makes. No test calls a model provider: crews run on
a CrewAI LLM that answers every call with a fixed final answer.

Requires: the crewai extra (pip install -e ".[dev,mcp,crewai]") and a live
AIM backend at AIM_URL with the admin AIM_TEST_ADMIN_EMAIL /
AIM_TEST_ADMIN_PASSWORD. scripts/test-sdk-python-framework-integration.sh starts
one and runs these tests.
Run with: pytest -m integration tests/test_crewai_integration.py
"""

import asyncio
import os
import sys
import uuid

import pytest
import requests

# Skip entire module if crewai is not installed
crewai = pytest.importorskip("crewai", reason="crewai not installed")
base_llm = pytest.importorskip("crewai.llms.base_llm", reason="crewai not installed")

from crewai import Agent, Task, Crew

from aim_sdk import AIMClient
from aim_sdk.integrations.crewai import AIMCrewWrapper, aim_verified_task, AIMTaskCallback

AIM_URL = os.environ.get("AIM_URL", "http://localhost:8080")

FIXED_ANSWER = "AI safety practices: least privilege, audit logging, human review."


class _FixedAnswerLLM(base_llm.BaseLLM):
    """A CrewAI LLM that answers every call with FIXED_ANSWER and sends nothing."""

    def call(self, messages, tools=None, callbacks=None, available_functions=None,
             from_task=None, from_agent=None, response_model=None, **kwargs):
        return f"Thought: I now know the final answer\nFinal Answer: {FIXED_ANSWER}"


def _research_crew(task_callback=None) -> Crew:
    researcher = Agent(
        role="Researcher",
        goal="Find accurate information",
        backstory="Expert researcher with attention to detail",
        llm=_FixedAnswerLLM(model="fixed-answer"),
        verbose=False,
        allow_delegation=False
    )
    research_task = Task(
        description="Research the topic: AI safety best practices",
        agent=researcher,
        expected_output="Summary of AI safety best practices",
        callback=task_callback,
    )
    return Crew(agents=[researcher], tasks=[research_task], verbose=False)


class _Admin:
    """The seeded admin's session. Its repr never shows the access token."""

    def __init__(self, access_token: str):
        self.headers = {"Authorization": f"Bearer {access_token}"}

    def __repr__(self) -> str:
        return "<admin session>"


@pytest.fixture(scope="module")
def admin():
    """The admin the backend was seeded with, signed in."""
    password = os.environ.get("AIM_TEST_ADMIN_PASSWORD")
    if not password:
        pytest.fail("AIM_TEST_ADMIN_PASSWORD is not set; scripts/test-sdk-python-framework-integration.sh sets it")
    response = requests.post(
        f"{AIM_URL}/api/v1/auth/login/local",
        json={"email": os.environ.get("AIM_TEST_ADMIN_EMAIL", "admin@opena2a.org"), "password": password},
        timeout=10,
    )
    assert response.status_code == 200, (
        f"admin sign-in answered {response.status_code}; DATABASE_URL must name a database this run bootstrapped"
    )
    return _Admin(response.json()["accessToken"])


def _register(prefix: str, admin: _Admin) -> AIMClient:
    """Register a new agent through the SDK's API-key mode.

    The key is the one the backend returns when the admin creates an agent.
    auto_hooks=False keeps the count to the adapter under test: the default
    hooks attach an AIMTaskCallback to every task of every new Crew.
    """
    suffix = uuid.uuid4().hex[:8]
    response = requests.post(
        f"{AIM_URL}/api/v1/agents",
        headers=admin.headers,
        json={"name": f"{prefix}-key-{suffix}", "displayName": f"{prefix} key", "agentType": "ai_agent"},
        timeout=10,
    )
    assert response.status_code == 201, f"creating the key agent answered {response.status_code}"
    return AIMClient.auto_register_or_load(
        f"{prefix}-{suffix}",
        AIM_URL,
        api_key=response.json()["apiKey"]["key"],
        auto_hooks=False,
    )


def _recorded_actions(admin: _Admin, agent_id: str) -> list:
    """The action of every verification event the backend holds for the agent."""
    response = requests.get(
        f"{AIM_URL}/api/v1/verification-events/agent/{agent_id}",
        headers=admin.headers,
        params={"limit": 100},
        timeout=10,
    )
    assert response.status_code == 200, f"reading verification events answered {response.status_code}"
    events = response.json().get("events") or []
    assert [event.get("agentId") for event in events] == [agent_id] * len(events)
    return sorted(event.get("action") for event in events)


def _organization_event_total(admin: _Admin) -> int:
    response = requests.get(
        f"{AIM_URL}/api/v1/verification-events",
        headers=admin.headers,
        params={"limit": 1},
        timeout=10,
    )
    assert response.status_code == 200, f"reading verification events answered {response.status_code}"
    return response.json()["total"]


@pytest.mark.integration
def test_crew_wrapper(admin):
    """Test 1: AIMCrewWrapper - Wrap entire crew"""
    aim_client = _register("crewai-test-wrapper", admin)

    # Wrap with AIM
    verified_crew = AIMCrewWrapper(
        crew=_research_crew(),
        aim_agent=aim_client,
        risk_level="medium",
        verbose=True
    )

    # Execute crew (this will be verified by AIM)
    result = verified_crew.kickoff(inputs={})

    assert FIXED_ANSWER in str(result)
    assert _recorded_actions(admin, aim_client.agent_id) == ["crewai_crew:kickoff"]


@pytest.mark.integration
def test_crew_wrapper_async(admin):
    """Test 1b: AIMCrewWrapper.kickoff_async"""
    aim_client = _register("crewai-test-wrapper-async", admin)
    verified_crew = AIMCrewWrapper(crew=_research_crew(), aim_agent=aim_client, risk_level="medium")

    result = asyncio.run(verified_crew.kickoff_async(inputs={}))

    assert FIXED_ANSWER in str(result)
    assert _recorded_actions(admin, aim_client.agent_id) == ["crewai_crew:kickoff_async"]


@pytest.mark.integration
def test_aim_verified_task_decorator(admin):
    """Test 2: @aim_verified_task decorator - Explicit task verification"""
    aim_client = _register("crewai-test-decorator", admin)

    # Define task function with decorator
    @aim_verified_task(agent=aim_client, risk_level="high")
    def sensitive_analysis(topic: str) -> str:
        '''Perform sensitive data analysis'''
        return f"Analysis complete for: {topic}"

    # A task function that takes no argument
    @aim_verified_task(agent=aim_client, risk_level="low")
    def weekly_summary() -> str:
        '''Summarize the week'''
        return "Weekly summary"

    # Execute tasks (AIM verification happens automatically)
    result = sensitive_analysis("confidential research")

    assert result == "Analysis complete for: confidential research"
    assert weekly_summary() == "Weekly summary"
    assert _recorded_actions(admin, aim_client.agent_id) == [
        "crewai_task:sensitive_analysis",
        "crewai_task:weekly_summary",
    ]


@pytest.mark.integration
def test_task_callback(admin):
    """Test 3: AIMTaskCallback - Automatic task logging"""
    aim_client = _register("crewai-test-callback", admin)

    # Create callback handler
    aim_callback = AIMTaskCallback(
        agent=aim_client,
        log_inputs=True,
        log_outputs=True,
        verbose=True
    )

    # CrewAI calls the task callback when the task completes.
    result = _research_crew(task_callback=aim_callback.on_task_complete).kickoff(inputs={})
    assert FIXED_ANSWER in str(result)

    # CrewAI has no task error callback; the error path is called directly.
    aim_callback.on_task_error(Exception("Simulated task error"))

    assert _recorded_actions(admin, aim_client.agent_id) == [
        "crewai_task:completed",
        "crewai_task:failed",
    ]


@pytest.mark.integration
def test_graceful_degradation(admin):
    """Test 4: Graceful degradation when AIM not configured"""
    before = _organization_event_total(admin)

    # Define task without AIM agent (runs with a warning)
    @aim_verified_task()  # No agent specified, will try to auto-load "crewai-agent"
    def simple_task(input: str) -> str:
        '''A simple task'''
        return f"Processed: {input}"

    # No stored "crewai-agent" exists, so the task runs unverified and the
    # backend records nothing.
    assert simple_task("test data") == "Processed: test data"
    assert _organization_event_total(admin) == before


if __name__ == "__main__":
    sys.exit(pytest.main([__file__, "-v", "-m", "integration"]))
