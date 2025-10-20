"""
AIM MCP Server Auto-Capability Detection

Helper functions for automatically detecting and reporting agent capabilities to AIM.
This enables runtime capability detection and risk assessment.
"""

from typing import Any, Dict, List, Optional
from datetime import datetime, timezone
import requests

from aim_sdk.client import AIMClient


def auto_detect_capabilities(
    aim_client: AIMClient,
    agent_id: str,
    detected_capabilities: Optional[List[Dict[str, Any]]] = None,
    auto_detect_from_mcp: bool = True
) -> Dict[str, Any]:
    """
    Automatically detect and report agent capabilities to AIM backend.

    This function reports detected agent capabilities (file system, database,
    code execution, network access, etc.) to the AIM backend for risk assessment
    and trust score calculation.

    The backend will:
    1. Analyze each detected capability for security risks
    2. Calculate an overall risk score (0-100)
    3. Determine trust score impact (-15 to +5)
    4. Generate security alerts for high-risk capabilities
    5. Update the agent's trust score accordingly

    Args:
        aim_client: AIMClient instance for authentication
        agent_id: UUID of the agent to report capabilities for
        detected_capabilities: List of manually detected capabilities
            Each capability should be a dict with:
            - capability_type: str (e.g., "file_read", "database_write", "code_execution")
            - capability_scope: Dict[str, Any] (e.g., {"paths": ["/etc"], "permissions": "read"})
            - risk_level: str (optional, "LOW", "MEDIUM", "HIGH", "CRITICAL")
            - detected_via: str (optional, e.g., "mcp_tool", "static_analysis", "runtime")
        auto_detect_from_mcp: Whether to automatically detect capabilities from MCP servers

    Returns:
        Dictionary containing capability detection results:
        {
            "agent_id": "agent-uuid",
            "capabilities_reported": 5,
            "risk_assessment": {
                "risk_level": "MEDIUM",
                "overall_risk_score": 65.0,
                "trust_score_impact": -8.5,
                "alerts": [
                    {
                        "severity": "HIGH",
                        "message": "Agent has code execution capability",
                        "capability_type": "code_execution"
                    }
                ]
            },
            "new_capabilities": 3,
            "existing_capabilities": 2,
            "timestamp": "2025-10-19T12:00:00Z"
        }

    Raises:
        requests.exceptions.RequestException: If detection fails
        ValueError: If agent_id is invalid or no capabilities detected

    Example:
        from aim_sdk import AIMClient
        from aim_sdk.integrations.mcp import auto_detect_capabilities

        aim_client = AIMClient.auto_register_or_load("my-agent", "http://localhost:8080")

        # Option 1: Auto-detect from MCP servers
        result = auto_detect_capabilities(
            aim_client=aim_client,
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            auto_detect_from_mcp=True
        )

        # Option 2: Manually report specific capabilities
        capabilities = [
            {
                "capability_type": "file_read",
                "capability_scope": {
                    "paths": ["/etc/hosts", "/var/log"],
                    "permissions": "read"
                },
                "risk_level": "MEDIUM",
                "detected_via": "static_analysis"
            },
            {
                "capability_type": "database_write",
                "capability_scope": {
                    "database": "postgres://prod-db",
                    "tables": ["users", "transactions"]
                },
                "risk_level": "HIGH",
                "detected_via": "runtime"
            }
        ]

        result = auto_detect_capabilities(
            aim_client=aim_client,
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            detected_capabilities=capabilities,
            auto_detect_from_mcp=False
        )

        print(f"Risk Level: {result['risk_assessment']['risk_level']}")
        print(f"Risk Score: {result['risk_assessment']['overall_risk_score']}")
        print(f"Trust Impact: {result['risk_assessment']['trust_score_impact']}")

        # Check for security alerts
        for alert in result['risk_assessment']['alerts']:
            if alert['severity'] in ['HIGH', 'CRITICAL']:
                print(f"⚠️ {alert['severity']}: {alert['message']}")
    """
    if not agent_id or not agent_id.strip():
        raise ValueError("agent_id cannot be empty")

    # Validate that we have something to detect
    if not detected_capabilities and not auto_detect_from_mcp:
        raise ValueError(
            "Either detected_capabilities must be provided or auto_detect_from_mcp must be True"
        )

    # Prepare detection payload
    payload = {
        "detected_at": datetime.now(timezone.utc).isoformat(),
        "capabilities": detected_capabilities or [],
        "risk_assessment": {
            "risk_level": "UNKNOWN",
            "overall_risk_score": 0.0,
            "trust_score_impact": 0.0,
            "alerts": []
        }
    }

    # If auto-detection from MCP is enabled, include flag in metadata
    if auto_detect_from_mcp:
        payload["metadata"] = {
            "auto_detect_from_mcp": True,
            "detection_method": "mcp_tools"
        }

    # Make API request with AIM client's built-in request method
    # AIM client handles cryptographic signing automatically
    try:
        response = aim_client._make_request(
            method="POST",
            endpoint=f"/api/v1/detection/agents/{agent_id}/capabilities/report",
            data=payload
        )

        # Backend returns the detection result with risk assessment
        return {
            "agent_id": response.get("agent_id", agent_id),
            "capabilities_reported": response.get("capabilities_reported", 0),
            "risk_assessment": response.get("risk_assessment", {
                "risk_level": "UNKNOWN",
                "overall_risk_score": 0.0,
                "trust_score_impact": 0.0,
                "alerts": []
            }),
            "new_capabilities": response.get("new_capabilities", 0),
            "existing_capabilities": response.get("existing_capabilities", 0),
            "timestamp": response.get("timestamp", datetime.now(timezone.utc).isoformat())
        }

    except requests.exceptions.HTTPError as e:
        if e.response.status_code == 404:
            raise ValueError(f"Agent not found: {agent_id}")
        elif e.response.status_code == 403:
            raise PermissionError(f"Not authorized to report capabilities for agent: {agent_id}")
        else:
            raise


def get_agent_capabilities(
    aim_client: AIMClient,
    agent_id: str
) -> Dict[str, Any]:
    """
    Get all capabilities currently registered for an agent.

    Args:
        aim_client: AIMClient instance for authentication
        agent_id: UUID of the agent

    Returns:
        Dictionary containing agent capabilities:
        {
            "capabilities": [
                {
                    "id": "cap-uuid",
                    "capability_type": "file_read",
                    "capability_scope": {...},
                    "granted_at": "2025-10-19T12:00:00Z",
                    "risk_level": "MEDIUM"
                }
            ],
            "total": 5
        }

    Example:
        capabilities = get_agent_capabilities(
            aim_client=aim_client,
            agent_id="550e8400-e29b-41d4-a716-446655440000"
        )

        for cap in capabilities['capabilities']:
            print(f"{cap['capability_type']}: {cap['risk_level']}")
    """
    if not agent_id or not agent_id.strip():
        raise ValueError("agent_id cannot be empty")

    try:
        response = aim_client._make_request(
            method="GET",
            endpoint=f"/api/v1/agents/{agent_id}/capabilities"
        )

        return {
            "capabilities": response.get("capabilities", []),
            "total": response.get("total", 0)
        }

    except requests.exceptions.HTTPError as e:
        if e.response.status_code == 404:
            raise ValueError(f"Agent not found: {agent_id}")
        elif e.response.status_code == 403:
            raise PermissionError(f"Not authorized to view capabilities for agent: {agent_id}")
        else:
            raise
