"""
AIM MCP Server Registration

Helper functions for registering and managing MCP servers with AIM.
"""

from typing import Any, Dict, List, Optional
import requests

from aim_sdk.client import AIMClient


def register_mcp_server(
    aim_client: AIMClient,
    server_name: str,
    server_url: str,
    public_key: str,
    capabilities: List[str],
    description: str = "",
    version: str = "1.0.0",
    verification_url: Optional[str] = None
) -> Dict[str, Any]:
    """
    Register an MCP server with the AIM backend.

    This function registers an MCP (Model Context Protocol) server with AIM,
    enabling cryptographic verification and trust scoring for all interactions
    with the server.

    Args:
        aim_client: AIMClient instance for authentication
        server_name: Name of the MCP server
        server_url: Base URL of the MCP server
        public_key: Ed25519 public key for cryptographic verification
        capabilities: List of server capabilities (e.g., ["tools", "resources", "prompts"])
        description: Optional description of the MCP server
        version: Server version (default: "1.0.0")
        verification_url: Optional URL for verification challenges

    Returns:
        Dictionary containing server registration details:
        {
            "id": "server-uuid",
            "name": "server-name",
            "status": "pending",
            "trust_score": 50.0,
            ...
        }

    Raises:
        requests.exceptions.RequestException: If registration fails
        ValueError: If server_name or public_key is invalid

    Example:
        from aim_sdk import AIMClient
        from aim_sdk.integrations.mcp import register_mcp_server

        aim_client = AIMClient.auto_register_or_load("my-agent", "http://localhost:8080")

        server_info = register_mcp_server(
            aim_client=aim_client,
            server_name="research-mcp",
            server_url="http://localhost:3000",
            public_key="ed25519_abcd1234...",
            capabilities=["tools", "resources"],
            description="Research assistant MCP server"
        )

        print(f"Server registered: {server_info['id']}")
    """
    if not server_name or not server_name.strip():
        raise ValueError("server_name cannot be empty")

    if not public_key or len(public_key) < 32:
        raise ValueError("public_key must be a valid Ed25519 public key")

    if not capabilities:
        raise ValueError("capabilities list cannot be empty")

    # Prepare registration payload
    payload = {
        "name": server_name.strip(),
        "description": description.strip() if description else f"MCP Server: {server_name}",
        "url": server_url.strip(),
        "version": version,
        "public_key": public_key,
        "capabilities": capabilities,
    }

    if verification_url:
        payload["verification_url"] = verification_url

    # Make API request with AIM client's built-in request method
    # AIM client handles cryptographic signing automatically
    try:
        response = aim_client._make_request(
            method="POST",
            endpoint="/api/v1/mcp-servers",
            data=payload
        )
    except AttributeError:
        # Fallback: Make request manually if _make_request doesn't exist
        headers = {"Content-Type": "application/json"}
        response = requests.post(
            f"{aim_client.aim_url}/api/v1/mcp-servers",
            json=payload,
            headers=headers,
            timeout=10
        )

    if response.status_code == 201:
        server_data = response.json()
        return server_data
    elif response.status_code == 400:
        error_msg = response.json().get("error", "Bad request")
        raise ValueError(f"Invalid MCP server data: {error_msg}")
    elif response.status_code == 401:
        raise PermissionError("Authentication failed. Check your AIM credentials.")
    elif response.status_code == 409:
        raise ValueError(f"MCP server with name '{server_name}' already exists")
    else:
        raise requests.exceptions.RequestException(
            f"Failed to register MCP server: {response.status_code} - {response.text}"
        )


def list_mcp_servers(
    aim_client: AIMClient,
    limit: int = 50,
    offset: int = 0
) -> List[Dict[str, Any]]:
    """
    List all MCP servers registered with AIM for the current organization.

    Args:
        aim_client: AIMClient instance for authentication
        limit: Maximum number of servers to return (default: 50)
        offset: Number of servers to skip (for pagination, default: 0)

    Returns:
        List of MCP server dictionaries

    Example:
        from aim_sdk.integrations.mcp import list_mcp_servers

        servers = list_mcp_servers(aim_client, limit=10)
        for server in servers:
            print(f"{server['name']}: {server['status']} (trust: {server['trust_score']})")
    """
    headers = {"Content-Type": "application/json"}
    params = {"limit": limit, "offset": offset}

    response = requests.get(
        f"{aim_client.aim_url}/api/v1/mcp-servers",
        headers=headers,
        params=params,
        timeout=10
    )

    if response.status_code == 200:
        return response.json()
    elif response.status_code == 401:
        raise PermissionError("Authentication failed. Check your AIM credentials.")
    else:
        raise requests.exceptions.RequestException(
            f"Failed to list MCP servers: {response.status_code} - {response.text}"
        )


def get_mcp_server(
    aim_client: AIMClient,
    server_id: str
) -> Dict[str, Any]:
    """
    Get details of a specific MCP server.

    Args:
        aim_client: AIMClient instance for authentication
        server_id: UUID of the MCP server

    Returns:
        Dictionary containing server details

    Raises:
        requests.exceptions.RequestException: If request fails
        ValueError: If server not found
    """
    headers = {"Content-Type": "application/json"}

    response = requests.get(
        f"{aim_client.aim_url}/api/v1/mcp-servers/{server_id}",
        headers=headers,
        timeout=10
    )

    if response.status_code == 200:
        return response.json()
    elif response.status_code == 404:
        raise ValueError(f"MCP server with ID '{server_id}' not found")
    elif response.status_code == 401:
        raise PermissionError("Authentication failed. Check your AIM credentials.")
    else:
        raise requests.exceptions.RequestException(
            f"Failed to get MCP server: {response.status_code} - {response.text}"
        )


def delete_mcp_server(
    aim_client: AIMClient,
    server_id: str
) -> bool:
    """
    Delete an MCP server registration from AIM.

    Args:
        aim_client: AIMClient instance for authentication
        server_id: UUID of the MCP server to delete

    Returns:
        True if deletion was successful

    Raises:
        requests.exceptions.RequestException: If deletion fails
    """
    headers = {"Content-Type": "application/json"}

    response = requests.delete(
        f"{aim_client.aim_url}/api/v1/mcp-servers/{server_id}",
        headers=headers,
        timeout=10
    )

    if response.status_code == 204:
        return True
    elif response.status_code == 404:
        raise ValueError(f"MCP server with ID '{server_id}' not found")
    elif response.status_code == 401:
        raise PermissionError("Authentication failed. Check your AIM credentials.")
    else:
        raise requests.exceptions.RequestException(
            f"Failed to delete MCP server: {response.status_code} - {response.text}"
        )
