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
    # Use SDK-specific endpoint that accepts Ed25519 agent authentication
    # _make_request returns the parsed JSON response directly on success
    response = aim_client._make_request(
        method="POST",
        endpoint=f"/api/v1/sdk-api/agents/{aim_client.agent_id}/mcp-servers",
        data=payload
    )

    # _make_request already handles errors and returns parsed JSON on success
    return response


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
    # Use SDK-specific endpoint that lists MCPs for this agent's organization
    # This uses the agent's Ed25519 authentication automatically
    try:
        response = aim_client._make_request(
            method="GET",
            endpoint=f"/api/v1/sdk-api/agents/{aim_client.agent_id}/mcp-servers?limit={limit}&offset={offset}"
        )
        return response if isinstance(response, list) else response.get("servers", [])
    except AttributeError:
        # Fallback: Make request manually if _make_request doesn't exist
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


def use_mcp_tool(
    aim_client: AIMClient,
    server_id: str,
    tool_name: str,
    mcp_url: str = "",
    mcp_name: str = "",
    auto_attest: bool = True,
    force_attest: bool = False,
    discovery_timeout: float = 30.0
) -> Dict[str, Any]:
    """
    Record MCP tool usage with smart attestation for supply chain security.

    This function:
    1. Checks the attestation cache to determine if re-attestation is needed
    2. Creates attestations when: first use, new tool, stale cache (>24h), or capability drift
    3. Records tool usage for supply chain analytics
    4. Records the agent-MCP connection in the backend

    Smart Attestation Triggers:
    - First use of an MCP server by this agent
    - First use of a NEW tool on a known server
    - Attestation is stale (>24 hours old)
    - Capability drift detected (tools added/removed)

    Args:
        aim_client: AIMClient instance for authentication
        server_id: UUID of the MCP server being used
        tool_name: Name of the tool being used (e.g., "read_file", "search")
        mcp_url: URL of the MCP server (required for attestation on first use)
        mcp_name: Name of the MCP server (required for attestation on first use)
        auto_attest: Enable smart attestation (default: True)
        force_attest: Force attestation even if cached (default: False)
        discovery_timeout: Timeout for capability discovery in seconds (default: 30.0)

    Returns:
        Dictionary containing connection response and attestation info:
        {
            "success": True,
            "connection_id": "connection-uuid",
            "agent_id": "agent-uuid",
            "mcp_server_id": "server-uuid",
            "connection_type": "attested",
            "attestation": {  # Only present if attestation occurred
                "id": "attestation-uuid",
                "reason": "first_use" | "new_tool" | "stale" | "capability_drift" | "forced",
                "confidenceScore": 85.5,
                "capabilitiesAttested": [...]
            },
            "toolUsage": {  # Tool usage stats
                "count": 5,
                "firstUsed": "...",
                "lastUsed": "..."
            },
            ...
        }

    Raises:
        requests.exceptions.RequestException: If connection recording fails
        ValueError: If server_id is invalid or attestation fails

    Example:
        from aim_sdk import AIMClient
        from aim_sdk.integrations.mcp import use_mcp_tool

        aim_client = AIMClient.auto_register_or_load("my-agent", "http://localhost:8080")

        # Smart attestation happens automatically
        response = use_mcp_tool(
            aim_client=aim_client,
            server_id="04531081-dd02-43aa-9067-a4e656de5591",
            tool_name="read_file",
            mcp_url="npx -y @modelcontextprotocol/server-filesystem /tmp",
            mcp_name="filesystem-mcp"
        )

        # Check if attestation occurred
        if response.get("attestation"):
            print(f"Attestation created: {response['attestation']['reason']}")

        # Force re-attestation
        response = use_mcp_tool(
            aim_client=aim_client,
            server_id="...",
            tool_name="write_file",
            force_attest=True  # Force attestation even if cached
        )
    """
    from aim_sdk.attestation_cache import AttestationCache
    from aim_sdk.console import console

    if not server_id:
        raise ValueError("server_id cannot be empty")

    if not tool_name:
        raise ValueError("tool_name cannot be empty")

    result = {
        "success": False,
        "agent_id": aim_client.agent_id,
        "mcp_server_id": server_id,
        "tool_name": tool_name
    }

    # Initialize attestation cache
    cache = AttestationCache(agent_id=aim_client.agent_id)

    # Smart attestation logic
    attestation_result = None
    if auto_attest or force_attest:
        try:
            # Check if attestation is needed
            if force_attest:
                attest_decision = {"shouldAttest": True, "reason": "forced", "driftInfo": None}
            else:
                # First, try to discover current capabilities for drift detection
                current_capabilities = None
                if mcp_url:
                    try:
                        from aim_sdk.integrations.mcp.discovery import discover_capabilities
                        discovery = discover_capabilities(mcp_url, timeout_seconds=discovery_timeout)
                        if not discovery.error:
                            current_capabilities = discovery.tool_names
                    except Exception:
                        pass  # Continue without drift detection

                attest_decision = cache.should_attest(
                    mcp_server_id=server_id,
                    tool_name=tool_name,
                    current_capabilities=current_capabilities
                )

            if attest_decision["shouldAttest"]:
                reason = attest_decision["reason"]
                console.info(f"Smart attestation triggered: {reason}")

                # Check if we have required params for attestation
                if not mcp_url:
                    console.warning(
                        f"Attestation needed ({reason}) but mcp_url not provided. "
                        "Provide mcp_url for attestation."
                    )
                else:
                    # Perform attestation with auto-discovery
                    attestation_response = attest_mcp_server(
                        aim_client=aim_client,
                        server_id=server_id,
                        mcp_url=mcp_url,
                        mcp_name=mcp_name or server_id,
                        auto_discover=True,
                        discovery_timeout=discovery_timeout
                    )

                    # Get capabilities from attestation response
                    capabilities_attested = []
                    if "discovery" in attestation_response:
                        capabilities_attested = attestation_response["discovery"].get("discoveredTools", [])

                    # Record attestation in cache
                    attestation_id = attestation_response.get("attestationId") or attestation_response.get("id", "")
                    confidence_score = attestation_response.get("mcpConfidenceScore") or attestation_response.get("confidenceScore")

                    cache.record_attestation(
                        mcp_server_id=server_id,
                        attestation_id=attestation_id,
                        capabilities=capabilities_attested,
                        confidence_score=confidence_score
                    )

                    attestation_result = {
                        "id": attestation_id,
                        "reason": reason,
                        "confidenceScore": confidence_score,
                        "capabilitiesAttested": capabilities_attested
                    }

                    # Log capability drift if detected
                    drift_info = attest_decision.get("driftInfo")
                    if drift_info and drift_info.get("driftDetected"):
                        console.warning(
                            f"Capability drift detected (severity: {drift_info['severity']}): "
                            f"+{len(drift_info['addedCapabilities'])} added, "
                            f"-{len(drift_info['removedCapabilities'])} removed"
                        )
                        attestation_result["driftInfo"] = drift_info

                    console.success(f"Attestation recorded (confidence: {confidence_score}%)")

        except Exception as e:
            console.warning(f"Smart attestation failed: {e}")
            # Continue with tool usage recording even if attestation fails

    # Record tool usage in local cache for analytics
    tool_stats = cache.record_tool_usage(server_id, tool_name)
    result["toolUsage"] = tool_stats

    # Prepare connection payload
    payload = {
        "mcp_server_id": server_id,
        "tool_name": tool_name,
        "mcp_url": mcp_url,
        "mcp_name": mcp_name,
        "connection_type": "attested"
    }

    # Submit connection via AIM client's authenticated request method
    # This uses the agent's Ed25519 authentication automatically
    try:
        response = aim_client._make_request(
            method="POST",
            endpoint=f"/api/v1/sdk-api/agents/{aim_client.agent_id}/mcp-connections",
            data=payload
        )
        result.update(response)
        result["success"] = True
    except Exception as e:
        console.error(f"Failed to record MCP connection: {e}")
        result["error"] = str(e)

    # Add attestation result if it occurred
    if attestation_result:
        result["attestation"] = attestation_result

    return result


def get_attestation_challenge(
    aim_client: AIMClient,
    server_id: str
) -> Dict[str, Any]:
    """
    Request a server-side challenge for proof of private key possession.

    This function requests a nonce from the AIM backend that must be signed
    and included in the attestation payload. This proves the agent holds the
    private key at attestation time and prevents replay attacks.

    Args:
        aim_client: AIMClient instance for authentication
        server_id: UUID of the MCP server to attest

    Returns:
        Dictionary containing challenge data:
        {
            "challenge": "base64-encoded-nonce",
            "expiresAt": "2025-01-15T10:05:00Z",
            "mcpServerId": "server-uuid"
        }

    Raises:
        requests.exceptions.RequestException: If challenge request fails
    """
    response = aim_client._make_request(
        method="GET",
        endpoint=f"/api/v1/mcp-servers/{server_id}/challenge?agent_id={aim_client.agent_id}"
    )
    return response


def attest_mcp_server(
    aim_client: AIMClient,
    server_id: str,
    mcp_url: str,
    mcp_name: str,
    capabilities_found: Optional[List[str]] = None,
    connection_successful: bool = True,
    health_check_passed: bool = True,
    connection_latency_ms: float = 0.0,
    use_challenge: bool = True,
    auto_discover: bool = False,
    discovery_timeout: float = 30.0
) -> Dict[str, Any]:
    """
    Submit cryptographically signed attestation for an MCP server.

    This function allows an agent to attest to the authenticity and functionality
    of an MCP server by cryptographically signing attestation data. Attestations
    increase the confidence score of the MCP server and help other agents trust it.

    The attestation uses a challenge-response mechanism:
    1. Request a challenge nonce from the server
    2. Include the challenge in the signed attestation payload
    3. Server validates the challenge hasn't been used (prevents replay attacks)

    Capability Discovery:
    - By default, you must provide capabilities_found manually
    - Set auto_discover=True to automatically query the MCP server for capabilities
    - Auto-discovery uses the official MCP protocol (tools/list) - lightweight, on-demand only
    - Discovery only happens at attestation time, no background processes

    Args:
        aim_client: AIMClient instance for authentication and signing
        server_id: UUID of the MCP server to attest
        mcp_url: URL/command of the MCP server (e.g., "npx -y @modelcontextprotocol/server-filesystem /tmp")
        mcp_name: Name of the MCP server
        capabilities_found: List of capabilities detected on the MCP server (required unless auto_discover=True)
        connection_successful: Whether connection to MCP was successful (default: True)
        health_check_passed: Whether health check passed (default: True)
        connection_latency_ms: Connection latency in milliseconds (default: 0.0)
        use_challenge: Whether to use challenge-response for proof of key possession (default: True)
        auto_discover: If True, automatically query MCP server for capabilities using MCP protocol (default: False)
        discovery_timeout: Timeout for auto-discovery in seconds (default: 30.0)

    Returns:
        Dictionary containing attestation response:
        {
            "success": True,
            "attestation_id": "attestation-uuid",
            "mcp_confidence_score": 85.5,
            "attestation_count": 3,
            "discovered_capabilities": [...],  # If auto_discover=True
            ...
        }

    Raises:
        requests.exceptions.RequestException: If attestation fails
        ValueError: If server_id is invalid, attestation is rejected, or capabilities not provided

    Example:
        from aim_sdk import AIMClient
        from aim_sdk.integrations.mcp import attest_mcp_server

        aim_client = AIMClient.auto_register_or_load("my-agent", "http://localhost:8080")

        # Manual capability list (default, no extra overhead)
        response = attest_mcp_server(
            aim_client=aim_client,
            server_id="04531081-dd02-43aa-9067-a4e656de5591",
            mcp_url="npx -y @modelcontextprotocol/server-filesystem /tmp",
            mcp_name="filesystem-mcp",
            capabilities_found=["read_file", "write_file", "search"]
        )

        # OR with auto-discovery (opt-in, queries MCP server once)
        response = attest_mcp_server(
            aim_client=aim_client,
            server_id="04531081-dd02-43aa-9067-a4e656de5591",
            mcp_url="npx -y @modelcontextprotocol/server-filesystem /tmp",
            mcp_name="filesystem-mcp",
            auto_discover=True  # Will query server for all capabilities
        )

        print(f"Attestation successful! New confidence score: {response['mcp_confidence_score']}%")
    """
    import time
    from datetime import datetime, timezone

    if not server_id:
        raise ValueError("server_id cannot be empty")

    # Step 0: Handle capability discovery
    discovery_result = None
    if auto_discover:
        # Import discovery module (lazy import to avoid overhead when not used)
        from aim_sdk.integrations.mcp.discovery import discover_capabilities

        print(f"🔍 Auto-discovering capabilities from MCP server: {mcp_url}")
        discovery_result = discover_capabilities(mcp_url, timeout_seconds=discovery_timeout)

        if discovery_result.error:
            if capabilities_found:
                print(f"⚠️  Auto-discovery failed ({discovery_result.error}), using provided capabilities")
            else:
                raise ValueError(
                    f"Auto-discovery failed and no capabilities_found provided: {discovery_result.error}"
                )
        else:
            # Use discovered capabilities
            capabilities_found = discovery_result.tool_names
            connection_latency_ms = discovery_result.connection_latency_ms
            print(f"✅ Discovered {len(capabilities_found)} tools: {capabilities_found}")

    # Validate capabilities are provided
    if not capabilities_found:
        raise ValueError(
            "capabilities_found is required. Either provide a list of capabilities "
            "or set auto_discover=True to query the MCP server."
        )

    # Step 1: Get challenge from server (proof of key possession)
    challenge = None
    if use_challenge:
        try:
            challenge_response = get_attestation_challenge(aim_client, server_id)
            challenge = challenge_response.get("challenge")
            print(f"🔐 Obtained attestation challenge (expires: {challenge_response.get('expiresAt')})")
        except Exception as e:
            print(f"⚠️  Warning: Could not get challenge, proceeding without: {e}")
            # Continue without challenge for backward compatibility

    # Step 2: Build attestation payload (including challenge for proof of key possession)
    attestation_data = {
        "agent_id": str(aim_client.agent_id),
        "capabilities_found": capabilities_found,
        "connection_latency_ms": connection_latency_ms,
        "connection_successful": connection_successful,
        "health_check_passed": health_check_passed,
        "mcp_name": mcp_name,
        "mcp_url": mcp_url,
        "sdk_version": "1.1.0",  # Version bump for challenge-response support
        "timestamp": datetime.now(timezone.utc).isoformat(),
    }

    # Include challenge if we got one (fields must be in alphabetical order for signing)
    if challenge:
        attestation_data["challenge"] = challenge

    # Step 3: Sign the attestation data using the agent's Ed25519 private key
    # The signature is computed over the canonical JSON representation
    import json
    import base64

    # Sort keys for canonical JSON (required for signature verification)
    canonical_json = json.dumps(attestation_data, sort_keys=True, separators=(',', ':'))

    # Use AIM client's signing key (PyNaCl SigningKey)
    if not hasattr(aim_client, 'signing_key') or aim_client.signing_key is None:
        raise ValueError("AIMClient must have Ed25519 signing key for attestations")

    # Sign the canonical JSON using PyNaCl SigningKey
    signature_bytes = aim_client.signing_key.sign(canonical_json.encode('utf-8')).signature
    signature_b64 = base64.b64encode(signature_bytes).decode('utf-8')

    # Step 4: Prepare and submit API request
    payload = {
        "attestation": attestation_data,
        "signature": signature_b64
    }

    # Submit attestation via AIM client's authenticated request method
    response = aim_client._make_request(
        method="POST",
        endpoint=f"/api/v1/mcp-servers/{server_id}/attest",
        data=payload
    )

    if challenge:
        print(f"✅ Attestation submitted with proof of key possession")
    else:
        print(f"✅ Attestation submitted (legacy mode, no challenge)")

    # Add discovery information to response if auto_discover was used
    if discovery_result and not discovery_result.error:
        response["discovery"] = {
            "serverName": discovery_result.server_name,
            "serverVersion": discovery_result.server_version,
            "protocolVersion": discovery_result.protocol_version,
            "discoveredTools": discovery_result.tool_names,
            "discoveredResources": discovery_result.resource_uris,
            "discoveredPrompts": discovery_result.prompt_names,
            "discoveryTimeMs": discovery_result.discovery_time_ms,
            "connectionLatencyMs": discovery_result.connection_latency_ms
        }

    return response
