"""
AIM MCP Action Verification

Functions for verifying MCP tool/resource/prompt usage through AIM.
"""

from typing import Any, Dict, Optional
import requests

from aim_sdk.client import AIMClient


def verify_mcp_action(
    aim_client: AIMClient,
    mcp_server_id: str,
    action_type: str,
    resource: str = "",
    context: Optional[Dict[str, Any]] = None,
    risk_level: str = "medium",
    timeout_seconds: int = 5
) -> Dict[str, Any]:
    """
    Verify an MCP action (tool call, resource access, or prompt usage) with AIM.

    This function verifies that an MCP server action is authorized through AIM's
    verification system, providing audit trails and security checks.

    Args:
        aim_client: AIMClient instance for verification
        mcp_server_id: UUID of the MCP server performing the action
        action_type: Type of action (e.g., "mcp_tool:search", "mcp_resource:database")
        resource: Resource being accessed (optional)
        context: Additional context for the action
        risk_level: Risk level ("low", "medium", "high")
        timeout_seconds: Verification timeout in seconds

    Returns:
        Dictionary containing verification result:
        {
            "verification_id": "uuid",
            "status": "approved",
            "timestamp": "2025-10-08T...",
            ...
        }

    Raises:
        PermissionError: If verification fails or action is denied
        requests.exceptions.RequestException: If request fails

    Example:
        from aim_sdk.integrations.mcp import verify_mcp_action

        # Verify MCP tool usage
        verification = verify_mcp_action(
            aim_client=aim_client,
            mcp_server_id="server-uuid",
            action_type="mcp_tool:web_search",
            resource="search query: AI safety",
            context={"tool": "web_search", "params": {"q": "AI safety"}},
            risk_level="low"
        )

        print(f"Action verified: {verification['verification_id']}")
    """
    if not mcp_server_id:
        raise ValueError("mcp_server_id cannot be empty")

    if not action_type:
        raise ValueError("action_type cannot be empty")

    # Prepare verification payload
    payload = {
        "action_type": action_type,
        "resource": resource,
        "context": context or {},
        "risk_level": risk_level,
        "mcp_server_id": mcp_server_id
    }

    # Make API request
    headers = {"Content-Type": "application/json"}

    try:
        response = requests.post(
            f"{aim_client.aim_url}/api/v1/mcp-servers/{mcp_server_id}/verify",
            json=payload,
            headers=headers,
            timeout=timeout_seconds
        )
    except requests.exceptions.Timeout:
        raise TimeoutError(f"MCP action verification timed out after {timeout_seconds}s")
    except requests.exceptions.RequestException as e:
        raise requests.exceptions.RequestException(f"MCP verification request failed: {e}")

    if response.status_code == 200:
        return response.json()
    elif response.status_code == 403:
        error_msg = response.json().get("error", "Action denied")
        raise PermissionError(f"MCP action verification denied: {error_msg}")
    elif response.status_code == 404:
        raise ValueError(f"MCP server with ID '{mcp_server_id}' not found")
    elif response.status_code == 401:
        raise PermissionError("Authentication failed. Check your AIM credentials.")
    else:
        raise requests.exceptions.RequestException(
            f"MCP verification failed: {response.status_code} - {response.text}"
        )


def log_mcp_action_result(
    aim_client: AIMClient,
    verification_id: str,
    success: bool,
    result_summary: str = "",
    error_message: str = ""
) -> bool:
    """
    Log the result of an MCP action back to AIM.

    Args:
        aim_client: AIMClient instance
        verification_id: Verification ID from verify_mcp_action()
        success: Whether the action completed successfully
        result_summary: Summary of the action result
        error_message: Error message if action failed

    Returns:
        True if logging was successful

    Example:
        # After MCP tool execution
        log_mcp_action_result(
            aim_client=aim_client,
            verification_id=verification["verification_id"],
            success=True,
            result_summary="Web search completed: 10 results found"
        )
    """
    payload = {
        "success": success,
        "result_summary": result_summary if success else "",
        "error_message": error_message if not success else ""
    }

    headers = {"Content-Type": "application/json"}

    try:
        response = requests.post(
            f"{aim_client.aim_url}/api/v1/verifications/{verification_id}/result",
            json=payload,
            headers=headers,
            timeout=5
        )

        if response.status_code in [200, 201, 204]:
            return True
        else:
            # Don't fail if result logging fails
            print(f"⚠️  Warning: Failed to log MCP action result: {response.status_code}")
            return False

    except Exception as e:
        # Don't fail if result logging fails
        print(f"⚠️  Warning: MCP action result logging error: {e}")
        return False


class MCPActionWrapper:
    """
    Wrapper for MCP actions that automatically handles AIM verification.

    This class provides a convenient way to wrap MCP tool/resource/prompt calls
    with automatic AIM verification and result logging.

    Example:
        from aim_sdk.integrations.mcp import MCPActionWrapper

        mcp_wrapper = MCPActionWrapper(
            aim_client=aim_client,
            mcp_server_id="server-uuid"
        )

        # Execute MCP tool with automatic verification
        result = mcp_wrapper.execute_tool(
            tool_name="web_search",
            tool_function=lambda: search_web("AI safety"),
            risk_level="low"
        )
    """

    def __init__(
        self,
        aim_client: AIMClient,
        mcp_server_id: str,
        default_risk_level: str = "medium",
        verbose: bool = False
    ):
        """
        Initialize MCP Action Wrapper.

        Args:
            aim_client: AIMClient instance for verification
            mcp_server_id: UUID of the MCP server
            default_risk_level: Default risk level for actions
            verbose: Whether to print debug information
        """
        self.aim_client = aim_client
        self.mcp_server_id = mcp_server_id
        self.default_risk_level = default_risk_level
        self.verbose = verbose

    def execute_tool(
        self,
        tool_name: str,
        tool_function: callable,
        risk_level: Optional[str] = None,
        context: Optional[Dict[str, Any]] = None
    ) -> Any:
        """
        Execute an MCP tool with AIM verification.

        Args:
            tool_name: Name of the MCP tool
            tool_function: Function to execute the tool
            risk_level: Risk level (defaults to instance default)
            context: Additional context for verification

        Returns:
            Result from tool_function

        Raises:
            PermissionError: If verification fails
        """
        _risk_level = risk_level or self.default_risk_level

        if self.verbose:
            print(f"🔧 AIM: Verifying MCP tool '{tool_name}' (risk: {_risk_level})")

        # Verify with AIM
        try:
            verification = verify_mcp_action(
                aim_client=self.aim_client,
                mcp_server_id=self.mcp_server_id,
                action_type=f"mcp_tool:{tool_name}",
                context=context or {},
                risk_level=_risk_level
            )
            verification_id = verification.get("verification_id")

            if self.verbose:
                print(f"✅ AIM: Tool verified (id: {verification_id})")

        except Exception as e:
            if self.verbose:
                print(f"❌ AIM: Verification failed: {e}")
            raise

        # Execute tool
        try:
            result = tool_function()

            # Log success
            if verification_id:
                log_mcp_action_result(
                    aim_client=self.aim_client,
                    verification_id=verification_id,
                    success=True,
                    result_summary=f"Tool '{tool_name}' completed successfully"
                )

            if self.verbose:
                print(f"✅ AIM: Tool execution completed and logged")

            return result

        except Exception as e:
            # Log failure
            if verification_id:
                log_mcp_action_result(
                    aim_client=self.aim_client,
                    verification_id=verification_id,
                    success=False,
                    error_message=str(e)
                )

            if self.verbose:
                print(f"❌ AIM: Tool execution failed: {e}")

            raise
