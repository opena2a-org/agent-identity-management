"""
AIM Client - Core SDK functionality for automatic identity verification
"""

import base64
import functools
import hashlib
import json
import time
from typing import Any, Callable, Optional, Dict, List
from datetime import datetime, timezone

import requests
from nacl.signing import SigningKey, VerifyKey
from nacl.encoding import Base64Encoder

from .exceptions import (
    AuthenticationError,
    VerificationError,
    ActionDeniedError,
    ConfigurationError
)
from .oauth import OAuthTokenManager, load_sdk_credentials
from .capability_detection import auto_detect_capabilities


class AIMClient:
    """
    AIM SDK Client for automatic identity verification.

    This client handles all cryptographic signing and verification automatically,
    allowing agents to focus on business logic while AIM ensures security compliance.

    Args:
        agent_id: UUID of the agent registered with AIM
        public_key: Base64-encoded Ed25519 public key (from AIM registration)
        private_key: Base64-encoded Ed25519 private key (from AIM registration)
        aim_url: Base URL of AIM server (e.g., https://aim.example.com)
        timeout: HTTP request timeout in seconds (default: 30)
        auto_retry: Whether to automatically retry failed requests (default: True)
        max_retries: Maximum number of retry attempts (default: 3)

    Example:
        client = AIMClient(
            agent_id="550e8400-e29b-41d4-a716-446655440000",
            public_key="base64-public-key",
            private_key="base64-private-key",
            aim_url="https://aim.example.com"
        )

        @client.perform_action("read_database", resource="users_table")
        def get_users():
            return database.query("SELECT * FROM users")
    """

    def __init__(
        self,
        agent_id: str,
        public_key: str = None,
        private_key: str = None,
        aim_url: str = None,
        api_key: str = None,
        timeout: int = 30,
        auto_retry: bool = True,
        max_retries: int = 3,
        sdk_token_id: Optional[str] = None,
        oauth_token_manager: Optional[Any] = None
    ):
        # Validate required parameters
        if not agent_id:
            raise ConfigurationError("agent_id is required")
        if not aim_url:
            raise ConfigurationError("aim_url is required")

        # Require either API key OR (public_key + private_key)
        if not api_key and (not public_key or not private_key):
            raise ConfigurationError(
                "Either api_key OR (public_key + private_key) is required.\n"
                "Use api_key for SDK API mode or keys for cryptographic signing."
            )

        self.agent_id = agent_id
        self.aim_url = aim_url.rstrip('/')
        self.api_key = api_key
        self.timeout = timeout
        self.auto_retry = auto_retry
        self.max_retries = max_retries
        self.oauth_token_manager = oauth_token_manager

        # Initialize Ed25519 signing key (only if using cryptographic mode)
        self.signing_key = None
        self.public_key = public_key

        if private_key and public_key:
            try:
                private_key_bytes = base64.b64decode(private_key)
                # Ed25519 private key from Go is 64 bytes (32-byte seed + 32-byte public key)
                # PyNaCl SigningKey expects only the 32-byte seed
                if len(private_key_bytes) == 64:
                    # Extract seed (first 32 bytes)
                    seed = private_key_bytes[:32]
                    self.signing_key = SigningKey(seed)
                elif len(private_key_bytes) == 32:
                    # Already just the seed
                    self.signing_key = SigningKey(private_key_bytes)
                else:
                    raise ValueError(f"Invalid private key length: {len(private_key_bytes)} bytes (expected 32 or 64)")
            except Exception as e:
                raise ConfigurationError(f"Invalid private key format: {e}")

            # Verify public key matches
            try:
                expected_public_key = self.signing_key.verify_key.encode(encoder=Base64Encoder).decode('utf-8')
                if expected_public_key != public_key:
                    raise ConfigurationError("Public key does not match private key")
            except Exception as e:
                raise ConfigurationError(f"Key validation failed: {e}")

        # Load SDK token ID from credentials if not provided (only in OAuth mode)
        # Skip if using API key mode to avoid unnecessary credential loading
        if not sdk_token_id and not api_key:
            sdk_creds = load_sdk_credentials(use_secure_storage=False)  # Disable secure storage for speed
            if sdk_creds and 'sdk_token_id' in sdk_creds:
                sdk_token_id = sdk_creds['sdk_token_id']

        self.sdk_token_id = sdk_token_id

        # Session for connection pooling
        self.session = requests.Session()
        headers = {
            'User-Agent': f'AIM-Python-SDK/1.0.0',
            'Content-Type': 'application/json'
        }

        # Add SDK token header for usage tracking if available
        if sdk_token_id:
            headers['X-SDK-Token'] = sdk_token_id

        self.session.headers.update(headers)

    def _sign_message(self, message: str) -> str:
        """
        Sign a message using Ed25519 private key.

        Args:
            message: The message to sign

        Returns:
            Base64-encoded signature
        """
        message_bytes = message.encode('utf-8')
        signed = self.signing_key.sign(message_bytes)
        signature = signed.signature
        return base64.b64encode(signature).decode('utf-8')

    def _make_request(
        self,
        method: str,
        endpoint: str,
        data: Optional[Dict] = None,
        retry_count: int = 0,
        custom_headers: Optional[Dict] = None
    ) -> Dict:
        """
        Make authenticated HTTP request to AIM server.

        Args:
            method: HTTP method (GET, POST, etc.)
            endpoint: API endpoint path
            data: Request payload (for POST/PUT)
            retry_count: Current retry attempt number

        Returns:
            Response JSON data

        Raises:
            AuthenticationError: If authentication fails
            VerificationError: If request fails after retries
        """
        url = f"{self.aim_url}{endpoint}"

        # Prepare additional headers (merge with session headers)
        additional_headers = {}

        # Add API key authentication if available
        if self.api_key:
            additional_headers['X-API-Key'] = self.api_key

        # Add OAuth authorization if token manager is available (takes precedence over API key)
        if self.oauth_token_manager:
            try:
                access_token = self.oauth_token_manager.get_access_token()
                if access_token:
                    additional_headers['Authorization'] = f'Bearer {access_token}'
            except Exception:
                # If OAuth token fails, fall back to API key if available
                pass

        # Merge session headers with additional headers
        merged_headers = {**self.session.headers, **additional_headers}

        try:
            response = self.session.request(
                method=method,
                url=url,
                json=data,
                headers=merged_headers,
                timeout=self.timeout
            )

            # Handle authentication errors
            if response.status_code == 401:
                raise AuthenticationError("Authentication failed - invalid agent credentials")

            # Handle forbidden errors
            if response.status_code == 403:
                raise AuthenticationError("Forbidden - insufficient permissions")

            # Retry on server errors if enabled
            if response.status_code >= 500 and self.auto_retry and retry_count < self.max_retries:
                time.sleep(2 ** retry_count)  # Exponential backoff
                return self._make_request(method, endpoint, data, retry_count + 1)

            # Debug 400 errors (disabled in production)
            # if response.status_code == 400:
            #     print(f"[DEBUG] 400 Bad Request - Response body: {response.text}")

            response.raise_for_status()
            return response.json()

        except requests.exceptions.Timeout:
            if self.auto_retry and retry_count < self.max_retries:
                time.sleep(2 ** retry_count)
                return self._make_request(method, endpoint, data, retry_count + 1)
            raise VerificationError("Request timeout")

        except requests.exceptions.ConnectionError:
            if self.auto_retry and retry_count < self.max_retries:
                time.sleep(2 ** retry_count)
                return self._make_request(method, endpoint, data, retry_count + 1)
            raise VerificationError("Connection failed")

        except requests.exceptions.RequestException as e:
            raise VerificationError(f"Request failed: {e}")

    def verify_action(
        self,
        action_type: str,
        resource: Optional[str] = None,
        context: Optional[Dict[str, Any]] = None,
        timeout_seconds: int = 300
    ) -> Dict:
        """
        Request verification for an action from AIM.

        This method:
        1. Creates a verification request with action details
        2. Signs the request with the agent's private key
        3. Sends the request to AIM
        4. Waits for approval/denial (up to timeout_seconds)
        5. Returns verification result

        Args:
            action_type: Type of action (e.g., "read_database", "send_email")
            resource: Resource being accessed (e.g., "users_table", "admin@example.com")
            context: Additional context about the action
            timeout_seconds: Maximum time to wait for approval (default: 300s = 5min)

        Returns:
            Verification result dict with keys:
            - verified: bool (whether action is approved)
            - verification_id: str (unique ID for this verification)
            - approved_by: str (user who approved, if applicable)
            - expires_at: str (ISO timestamp when approval expires)

        Raises:
            ActionDeniedError: If action is explicitly denied
            VerificationError: If verification request fails
        """
        # Create verification request payload
        timestamp = datetime.utcnow().isoformat() + 'Z'  # Match backend expected format
        unix_timestamp = int(time.time())

        # Create signature for Ed25519 verification
        # The backend verifies the signature by reconstructing the JSON payload
        # We need to create a signature of the JSON payload itself
        signature_payload = {
            "action_type": action_type,
            "agent_id": self.agent_id,
            "context": context or {},
            "resource": resource,
            "timestamp": timestamp
        }
        
        # Create deterministic JSON (sorted keys, spaces after colons and commas)
        signature_message = json.dumps(signature_payload, sort_keys=True, separators=(', ', ': '))
        
        # Sign with Ed25519
        signature = self._sign_message(signature_message)

        request_payload = {
            "agent_id": self.agent_id,
            "action_type": action_type,
            "resource": resource,
            "context": context or {},
            "timestamp": timestamp,
            "signature": signature,  # Ed25519 signature in body
            "public_key": self.public_key  # Public key in body
        }

        # SDK API endpoint
        endpoint = "/api/v1/sdk-api/verifications"

        # Debug logging (disabled in production)
        # print(f"[DEBUG] Sending verification request:")
        # print(f"[DEBUG]   Endpoint: POST {endpoint}")
        # print(f"[DEBUG]   Agent ID: {self.agent_id}")

        # Send verification request (API key is already in default headers)
        try:
            result = self._make_request(
                method="POST",
                endpoint=endpoint,
                data=request_payload
            )

            verification_id = result.get("id")
            status = result.get("status")

            # If auto-approved, return immediately
            if status == "approved":
                return {
                    "verified": True,
                    "verification_id": verification_id,
                    "approved_by": result.get("approved_by"),
                    "expires_at": result.get("expires_at")
                }

            # If denied, raise error
            if status == "denied":
                reason = result.get("denial_reason", "Action denied by policy")
                raise ActionDeniedError(f"Action denied: {reason}")

            # If pending, poll for result
            if status == "pending":
                return self._wait_for_approval(verification_id, timeout_seconds)

            raise VerificationError(f"Unexpected verification status: {status}")

        except (AuthenticationError, ActionDeniedError):
            raise
        except Exception as e:
            raise VerificationError(f"Verification request failed: {e}")

    def _wait_for_approval(self, verification_id: str, timeout_seconds: int) -> Dict:
        """
        Poll AIM server for verification approval.

        Args:
            verification_id: ID of the verification request
            timeout_seconds: Maximum time to wait

        Returns:
            Verification result dict

        Raises:
            ActionDeniedError: If action is denied
            VerificationError: If timeout or polling fails
        """
        start_time = time.time()
        poll_interval = 2  # Start with 2 second polls

        while time.time() - start_time < timeout_seconds:
            try:
                result = self._make_request(
                    method="GET",
                    endpoint=f"/api/v1/sdk-api/verifications/{verification_id}"
                )

                status = result.get("status")

                if status == "approved":
                    return {
                        "verified": True,
                        "verification_id": verification_id,
                        "approved_by": result.get("approved_by"),
                        "expires_at": result.get("expires_at")
                    }

                if status == "denied":
                    reason = result.get("denial_reason", "Action denied")
                    raise ActionDeniedError(f"Action denied: {reason}")

                # Still pending, wait and retry
                time.sleep(poll_interval)
                poll_interval = min(poll_interval * 1.5, 10)  # Exponential backoff up to 10s

            except (AuthenticationError, ActionDeniedError):
                raise
            except Exception as e:
                # Continue polling on transient errors
                time.sleep(poll_interval)

        raise VerificationError(f"Verification timeout after {timeout_seconds} seconds")

    def log_action_result(
        self,
        verification_id: str,
        success: bool,
        result_summary: Optional[str] = None,
        error_message: Optional[str] = None
    ):
        """
        Log the result of an action execution to AIM.

        This helps AIM track agent behavior and build trust scores.

        Args:
            verification_id: ID from verify_action response
            success: Whether the action succeeded
            result_summary: Brief summary of the result
            error_message: Error message if action failed
        """
        try:
            self._make_request(
                method="POST",
                endpoint=f"/api/v1/sdk-api/verifications/{verification_id}/result",
                data={
                    "result": "success" if success else "failure",
                    "result_summary": result_summary,
                    "error_message": error_message,
                    "timestamp": datetime.now(timezone.utc).isoformat()
                }
            )
        except Exception:
            # Don't fail the action if logging fails
            pass

    def request_capability(
        self,
        capability_type: str,
        reason: str
    ) -> Dict:
        """
        Request an additional capability for the agent.

        When an agent needs a capability that wasn't granted during registration,
        it can request it through this method. The request will be sent to admins
        for approval.

        Args:
            capability_type: Type of capability being requested (e.g., "write_database", "send_bulk_email")
            reason: Business justification for the capability request (minimum 10 characters)

        Returns:
            Dict containing:
            - id: str - Capability request ID
            - agent_id: str - Agent ID
            - capability_type: str - Requested capability
            - status: str - Request status ("pending", "approved", "rejected")
            - requested_at: str - ISO timestamp of request

        Raises:
            ConfigurationError: If reason is too short or capability_type is invalid
            VerificationError: If request fails

        Example:
            result = client.request_capability(
                capability_type="write_database",
                reason="Need to update user records in the database for analytics"
            )
            print(f"Request ID: {result['id']}, Status: {result['status']}")
        """
        # Validate inputs
        if not capability_type or not isinstance(capability_type, str):
            raise ConfigurationError("capability_type must be a non-empty string")
        
        if not reason or len(reason) < 10:
            raise ConfigurationError("reason must be at least 10 characters")

        # Prepare request payload
        request_data = {
            "capability_type": capability_type,
            "reason": reason
        }

        try:
            # Make request to SDK API endpoint
            result = self._make_request(
                method="POST",
                endpoint=f"/api/v1/sdk-api/agents/{self.agent_id}/capability-requests",
                data=request_data
            )

            return result

        except Exception as e:
            raise VerificationError(f"Capability request failed: {e}")

    def report_detections(
        self,
        detections: list
    ) -> Dict:
        """
        Report detected MCP servers to AIM.

        This allows agents to automatically report MCP servers they discover
        through various detection methods (SDK imports, Claude config parsing, etc.).

        Args:
            detections: List of detection events, each containing:
                - mcpServer: str - Name/identifier of the MCP server
                - detectionMethod: str - Method used to detect (sdk_import, claude_config, etc.)
                - confidence: float - Confidence score (0-100)
                - details: Dict - Optional additional details
                - sdkVersion: str - Optional SDK version
                - timestamp: str - ISO timestamp of detection

        Returns:
            Dict with keys:
                - success: bool
                - detectionsProcessed: int
                - newMCPs: List[str] - New MCP servers added
                - existingMCPs: List[str] - Previously detected MCP servers
                - message: str

        Example:
            detections = [
                {
                    "mcpServer": "@modelcontextprotocol/server-filesystem",
                    "detectionMethod": "sdk_import",
                    "confidence": 95.0,
                    "details": {
                        "packageName": "@modelcontextprotocol/server-filesystem",
                        "version": "0.1.0"
                    },
                    "sdkVersion": "aim-sdk-python@1.0.0",
                    "timestamp": "2025-10-09T12:00:00Z"
                }
            ]
            result = client.report_detections(detections)
            print(f"Processed {result['detectionsProcessed']} detections")
            print(f"New MCPs: {result['newMCPs']}")

        Raises:
            AuthenticationError: If authentication fails
            VerificationError: If request fails
        """
        try:
            result = self._make_request(
                method="POST",
                endpoint=f"/api/v1/detection/agents/{self.agent_id}/report",
                data={"detections": detections}
            )
            return result

        except (AuthenticationError, VerificationError):
            raise
        except Exception as e:
            raise VerificationError(f"Detection report failed: {e}")

    def register_mcp(
        self,
        mcp_server_id: str,
        detection_method: str = "manual",
        confidence: float = 100.0,
        metadata: Optional[Dict[str, Any]] = None
    ) -> Dict:
        """
        Register an MCP server to this agent's "talks_to" list.

        This creates a relationship between the agent and an MCP server,
        indicating that the agent communicates with this MCP server.

        Args:
            mcp_server_id: MCP server ID or name to register
            detection_method: How the MCP was detected ("manual", "auto_sdk", "auto_config", "cli")
            confidence: Detection confidence score (0-100, default: 100 for manual)
            metadata: Optional additional context about the detection

        Returns:
            Dict with keys:
                - success: bool
                - message: str
                - added: int - Number of MCP servers added
                - agent_id: str
                - mcp_server_ids: List[str]

        Example:
            # Register filesystem MCP server
            result = client.register_mcp(
                mcp_server_id="filesystem-mcp-server",
                detection_method="manual",
                confidence=100.0
            )
            print(f"Registered {result['added']} MCP server(s)")

        Raises:
            AuthenticationError: If authentication fails
            VerificationError: If request fails
        """
        try:
            result = self._make_request(
                method="POST",
                endpoint=f"/api/v1/sdk-api/agents/{self.agent_id}/mcp-servers",
                data={
                    "mcp_server_ids": [mcp_server_id],
                    "detected_method": detection_method,
                    "confidence": confidence,
                    "metadata": metadata or {}
                }
            )
            return result

        except (AuthenticationError, VerificationError):
            raise
        except Exception as e:
            raise VerificationError(f"MCP registration failed: {e}")

    def report_capabilities(
        self,
        capabilities: List[str],
        scope: Optional[Dict[str, Any]] = None
    ) -> Dict:
        """
        Report agent capabilities to AIM (API key mode).

        This method is used when the SDK is running in API key mode to report
        detected capabilities to the backend. Capabilities are granted individually.

        Args:
            capabilities: List of capability types to report
            scope: Optional scope information for the capabilities

        Returns:
            Dict with keys:
                - granted: int - Number of capabilities granted
                - total: int - Total capabilities attempted

        Example:
            # Report detected capabilities
            result = client.report_capabilities([
                "network_access",
                "make_api_calls",
                "read_files"
            ])
            print(f"Granted {result['granted']}/{result['total']} capabilities")

        Raises:
            AuthenticationError: If authentication fails
            VerificationError: If request fails
        """
        if not self.api_key:
            raise ConfigurationError("report_capabilities requires API key authentication mode")

        granted_count = 0
        total_count = len(capabilities)

        # Temporarily disable auto-retry for capability reporting to handle duplicates faster
        original_auto_retry = self.auto_retry
        self.auto_retry = False

        try:
            for capability_type in capabilities:
                try:
                    # Use SDK API endpoint for capability grant
                    result = self._make_request(
                        method="POST",
                        endpoint=f"/api/v1/sdk-api/agents/{self.agent_id}/capabilities",
                        data={
                            "capabilityType": capability_type,
                            "scope": scope or {
                                "source": "python_sdk_auto_detection",
                                "detectedAt": datetime.now(timezone.utc).isoformat()
                            }
                        }
                    )

                    if result:
                        granted_count += 1

                except Exception as e:
                    # Capability might already exist (duplicate key error) - count as granted
                    # Check both the exception message and type
                    error_str = str(e).lower()
                    is_duplicate = (
                        "duplicate" in error_str or
                        "already exists" in error_str or
                        "unique constraint" in error_str or
                        "500" in error_str  # Backend returns 500 for duplicate key violations
                    )
                    if is_duplicate:
                        granted_count += 1
                    # Continue even if one capability fails for other reasons
                    continue

        finally:
            # Restore original auto-retry setting
            self.auto_retry = original_auto_retry

        return {
            "granted": granted_count,
            "total": total_count
        }

    def report_sdk_integration(
        self,
        sdk_version: str,
        platform: str = "python",
        capabilities: Optional[List[str]] = None
    ) -> Dict:
        """
        Report SDK integration status to AIM dashboard.

        This updates the Detection tab to show that the AIM SDK is installed
        and integrated with the agent, enabling auto-detection features.

        Args:
            sdk_version: SDK version string (e.g., "aim-sdk-python@1.0.0")
            platform: Platform/language (e.g., "python", "javascript", "go")
            capabilities: Optional list of SDK capabilities enabled

        Returns:
            Dict with keys:
                - success: bool
                - detectionsProcessed: int
                - message: str

        Example:
            # Report SDK integration
            result = client.report_sdk_integration(
                sdk_version="aim-sdk-python@1.0.0",
                platform="python",
                capabilities=["auto_detect_mcps", "capability_detection"]
            )
            print(f"SDK integration reported: {result['message']}")

        Raises:
            AuthenticationError: If authentication fails
            VerificationError: If request fails
        """
        try:
            # Create SDK integration detection event
            detection_event = {
                "mcpServer": "aim-sdk-integration",
                "detectionMethod": "sdk_integration",
                "confidence": 100.0,
                "details": {
                    "platform": platform,
                    "capabilities": capabilities or [],
                    "integrated": True
                },
                "sdkVersion": sdk_version,
                "timestamp": datetime.now(timezone.utc).isoformat()
            }

            result = self._make_request(
                method="POST",
                endpoint=f"/api/v1/sdk-api/agents/{self.agent_id}/detection/report",
                data={"detections": [detection_event]}
            )
            return result

        except (AuthenticationError, VerificationError):
            raise
        except Exception as e:
            raise VerificationError(f"SDK integration report failed: {e}")

    def perform_action(
        self,
        action_type: str,
        resource: Optional[str] = None,
        context: Optional[Dict[str, Any]] = None,
        timeout_seconds: int = 300
    ):
        """
        Decorator for automatic action verification.

        This decorator wraps a function to automatically:
        1. Request verification from AIM before execution
        2. Wait for approval
        3. Execute the function if approved
        4. Log the result back to AIM

        Args:
            action_type: Type of action being performed
            resource: Resource being accessed
            context: Additional context
            timeout_seconds: Max time to wait for approval

        Example:
            @client.perform_action("read_database", resource="users_table")
            def get_users():
                return database.query("SELECT * FROM users")

            # When called, this will:
            # 1. Request verification from AIM
            # 2. Wait for approval
            # 3. Execute the query if approved
            # 4. Log the result to AIM
            users = get_users()

        Raises:
            ActionDeniedError: If AIM denies the action
            VerificationError: If verification fails
        """
        def decorator(func: Callable) -> Callable:
            @functools.wraps(func)
            def wrapper(*args, **kwargs):
                # Request verification
                verification_result = self.verify_action(
                    action_type=action_type,
                    resource=resource,
                    context=context,
                    timeout_seconds=timeout_seconds
                )

                verification_id = verification_result["verification_id"]

                # Execute the function
                try:
                    result = func(*args, **kwargs)

                    # Log success
                    self.log_action_result(
                        verification_id=verification_id,
                        success=True,
                        result_summary=f"Action '{action_type}' completed successfully"
                    )

                    return result

                except Exception as e:
                    # Log failure
                    self.log_action_result(
                        verification_id=verification_id,
                        success=False,
                        error_message=str(e)
                    )
                    raise

            return wrapper
        return decorator

    def close(self):
        """Close the HTTP session."""
        self.session.close()

    def __enter__(self):
        """Context manager entry."""
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        """Context manager exit."""
        self.close()


# ============================================================================
# ONE-LINE AGENT REGISTRATION - "AIM is Stripe for AI Agent Identity"
# ============================================================================

import os
import pathlib


def _get_credentials_path():
    """Get path to credentials file (~/.aim/credentials.json)."""
    home = pathlib.Path.home()
    aim_dir = home / ".aim"
    aim_dir.mkdir(exist_ok=True)
    return aim_dir / "credentials.json"


def _save_credentials(agent_name: str, credentials: Dict[str, Any]):
    """
    Save agent credentials locally.

    Args:
        agent_name: Name of the agent
        credentials: Credentials dict from registration response
    """
    creds_path = _get_credentials_path()

    # Load existing credentials
    all_creds = {}
    if creds_path.exists():
        try:
            with open(creds_path, 'r') as f:
                all_creds = json.load(f)
        except Exception:
            pass  # Start fresh if corrupted

    # Add new agent credentials
    all_creds[agent_name] = {
        "agent_id": credentials["agent_id"],
        "public_key": credentials["public_key"],
        "private_key": credentials["private_key"],
        "aim_url": credentials["aim_url"],
        "status": credentials["status"],
        "trust_score": credentials["trust_score"],
        "registered_at": datetime.now(timezone.utc).isoformat()
    }

    # Save with secure permissions (owner read/write only)
    with open(creds_path, 'w') as f:
        json.dump(all_creds, f, indent=2)
    os.chmod(creds_path, 0o600)  # -rw------- (owner only)


def _load_credentials(agent_name: str) -> Optional[Dict[str, Any]]:
    """
    Load agent credentials from local storage.

    Args:
        agent_name: Name of the agent

    Returns:
        Credentials dict if found, None otherwise
    """
    creds_path = _get_credentials_path()
    if not creds_path.exists():
        return None

    try:
        with open(creds_path, 'r') as f:
            all_creds = json.load(f)
        return all_creds.get(agent_name)
    except Exception:
        return None


def register_agent(
    name: str,
    aim_url: Optional[str] = None,
    api_key: Optional[str] = None,
    display_name: Optional[str] = None,
    description: Optional[str] = None,
    agent_type: str = "ai_agent",
    version: Optional[str] = None,
    repository_url: Optional[str] = None,
    documentation_url: Optional[str] = None,
    organization_domain: Optional[str] = None,
    talks_to: Optional[list] = None,
    capabilities: Optional[list] = None,
    auto_detect: bool = True,
    force_new: bool = False,
    sdk_token_id: Optional[str] = None
) -> AIMClient:
    """
    ONE-LINE agent registration with AIM - "The Stripe Moment"

    This is the magic function that makes AIM "Stripe for AI Agent Identity".
    Call this once and your agent is registered, verified, and ready to use.

    **ZERO CONFIG MODE** (SDK Download):
        agent = register_agent("my-agent")
        # That's it! Everything auto-detected.

    **MANUAL MODE** (pip install):
        agent = register_agent("my-agent", api_key="aim_abc123")
        # Still auto-detects capabilities + MCPs

    Args:
        name: Agent name (unique identifier)
        aim_url: AIM server URL (auto-detected from SDK credentials if available)
        api_key: AIM API key (only required if no SDK credentials found)
        display_name: Human-readable display name (defaults to name)
        description: Agent description (defaults to auto-generated)
        agent_type: "ai_agent" or "mcp_server" (default: "ai_agent")
        version: Agent version (e.g., "1.0.0")
        repository_url: GitHub/GitLab repository URL
        documentation_url: Documentation URL
        organization_domain: Organization domain for auto-approval
        talks_to: Override auto-detected MCP servers (manual specification)
        capabilities: Override auto-detected capabilities (manual specification)
        auto_detect: Auto-detect capabilities and MCPs (default: True)
        force_new: Force new registration even if credentials exist
        sdk_token_id: SDK token for usage tracking (auto-loaded if available)

    Returns:
        AIMClient instance ready to use

    Examples:
        # SDK Download Mode (ZERO CONFIG):
        >>> agent = register_agent("my-agent")

        # Manual Install Mode:
        >>> agent = register_agent("my-agent", api_key="aim_abc123")

        # Power User Mode (disable auto-detection):
        >>> agent = register_agent(
        ...     "my-agent",
        ...     api_key="aim_abc123",
        ...     auto_detect=False,
        ...     capabilities=["custom_capability"],
        ...     talks_to=["custom-mcp-server"]
        ... )

    Raises:
        ConfigurationError: If registration fails or required credentials missing
        AuthenticationError: If authentication fails
    """
    # 1. Check for existing credentials (unless force_new)
    if not force_new:
        existing_creds = _load_credentials(name)
        if existing_creds:
            print(f"✅ Found existing credentials for '{name}'")
            print(f"   Agent ID: {existing_creds['agent_id']}")
            print(f"   Status: {existing_creds['status']}")
            print(f"   Trust Score: {existing_creds['trust_score']}")
            print(f"\n   To register a new agent, use force_new=True")

            # Create OAuth token manager if tokens are in credentials
            token_manager = None
            if "refresh_token" in existing_creds or "access_token" in existing_creds:
                # Create a temporary credentials file for the token manager
                from pathlib import Path
                temp_creds_path = Path.home() / ".aim" / f"temp_{name}_creds.json"
                token_manager = OAuthTokenManager(str(temp_creds_path))
                # Directly set the credentials with OAuth tokens
                token_manager.credentials = existing_creds
                token_manager.access_token = existing_creds.get("access_token")

            return AIMClient(
                agent_id=existing_creds["agent_id"],
                public_key=existing_creds["public_key"],
                private_key=existing_creds["private_key"],
                aim_url=existing_creds["aim_url"],
                api_key=api_key,  # Pass API key for verification requests
                oauth_token_manager=token_manager
            )

    # 2. Detect authentication mode (SDK vs Manual)
    sdk_creds = load_sdk_credentials()

    if sdk_creds:
        # SDK MODE: Use embedded OAuth credentials
        auth_mode = "oauth"
        aim_url = aim_url or sdk_creds.get("aim_url")
        sdk_token_id = sdk_token_id or sdk_creds.get("sdk_token_id")

        if not aim_url:
            raise ConfigurationError("aim_url not found in SDK credentials")

        print(f"🔐 SDK Mode: Using embedded OAuth credentials")

    elif api_key:
        # MANUAL MODE: Use API key
        auth_mode = "api_key"

        if not aim_url:
            raise ConfigurationError("aim_url is required when using API key mode")

        print(f"🔑 Manual Mode: Using API key authentication")

    else:
        # No authentication found
        raise ConfigurationError(
            "No authentication credentials found.\n"
            "Either download SDK from dashboard (OAuth mode) or provide api_key parameter (Manual mode)."
        )

    # 3. Auto-detect capabilities and MCPs (unless manually specified)
    if auto_detect:
        print(f"🔍 Auto-detecting agent capabilities and MCP servers...")

        # Auto-detect capabilities (unless manually provided)
        if not capabilities:
            from .detection import auto_detect_mcps

            detected_caps = auto_detect_capabilities()
            if detected_caps:
                capabilities = detected_caps
                print(f"   ✅ Detected {len(capabilities)} capabilities: {', '.join(capabilities[:5])}{' ...' if len(capabilities) > 5 else ''}")
            else:
                print(f"   ℹ️  No capabilities auto-detected (you can specify manually)")

        # Auto-detect MCP servers (unless manually provided)
        if not talks_to:
            from .detection import auto_detect_mcps

            mcp_detections = auto_detect_mcps()
            if mcp_detections:
                talks_to = [d["mcpServer"] for d in mcp_detections]
                print(f"   ✅ Detected {len(talks_to)} MCP servers: {', '.join(talks_to[:3])}{' ...' if len(talks_to) > 3 else ''}")
            else:
                print(f"   ℹ️  No MCP servers auto-detected")

    # 4. Prepare registration request
    registration_data = {
        "name": name,
        "display_name": display_name or name,
        "description": description or f"Agent {name} registered via AIM SDK",
        "agent_type": agent_type
    }

    if version:
        registration_data["version"] = version
    if repository_url:
        registration_data["repository_url"] = repository_url
    if documentation_url:
        registration_data["documentation_url"] = documentation_url
    if organization_domain:
        registration_data["organization_domain"] = organization_domain
    if talks_to:
        registration_data["talks_to"] = talks_to
    if capabilities:
        registration_data["capabilities"] = capabilities

    # 5. Register agent (mode-specific endpoint)
    try:
        if auth_mode == "oauth":
            # OAuth Mode: Use authenticated endpoint with OAuth token
            return _register_via_oauth(
                name=name,
                aim_url=aim_url,
                sdk_creds=sdk_creds,
                registration_data=registration_data,
                sdk_token_id=sdk_token_id,
                talks_to=talks_to
            )
        else:
            # API Key Mode: Use public endpoint with API key header
            return _register_via_api_key(
                name=name,
                aim_url=aim_url,
                api_key=api_key,
                registration_data=registration_data,
                sdk_token_id=sdk_token_id,
                talks_to=talks_to
            )

    except requests.RequestException as e:
        raise ConfigurationError(f"Failed to connect to AIM server: {e}")
    except Exception as e:
        raise ConfigurationError(f"Registration failed: {e}")


def _register_via_oauth(
    name: str,
    aim_url: str,
    sdk_creds: Dict[str, Any],
    registration_data: Dict[str, Any],
    sdk_token_id: Optional[str],
    talks_to: Optional[List[str]]
) -> AIMClient:
    """Register agent using OAuth token from SDK credentials"""
    print(f"[DEBUG] _register_via_oauth() called")
    print(f"[DEBUG] sdk_creds type: {type(sdk_creds)}")
    print(f"[DEBUG] sdk_creds keys: {sdk_creds.keys() if sdk_creds else 'None'}")

    # Generate Ed25519 keypair client-side (for OAuth mode)
    print(f"🔐 Generating Ed25519 keypair...")
    signing_key = SigningKey.generate()
    private_key_bytes = bytes(signing_key) + bytes(signing_key.verify_key)  # 64 bytes (seed + public)
    public_key_bytes = bytes(signing_key.verify_key)

    private_key_b64 = base64.b64encode(private_key_bytes).decode('utf-8')
    public_key_b64 = base64.b64encode(public_key_bytes).decode('utf-8')

    # Add public key to registration data
    registration_data["public_key"] = public_key_b64
    print(f"✅ Keypair generated")

    # Initialize OAuth token manager with SDK credentials path
    # OAuthTokenManager expects a file path, not the credentials dict
    # Check for .aim directory (not plaintext file - it might be encrypted!)
    from pathlib import Path
    sdk_dir = Path.cwd() / ".aim"
    print(f"[DEBUG] Checking SDK dir: {sdk_dir}")
    print(f"[DEBUG] Dir exists: {sdk_dir.exists()}")

    if not sdk_dir.exists():
        # Fall back to home directory
        sdk_dir = Path.home() / ".aim"
        print(f"[DEBUG] Fallback to home dir: {sdk_dir}")
        print(f"[DEBUG] Home dir exists: {sdk_dir.exists()}")

    # OAuthTokenManager will handle finding encrypted or plaintext credentials
    sdk_creds_path = sdk_dir / "credentials.json"

    print(f"[DEBUG] Creating OAuthTokenManager with path: {sdk_creds_path}")
    token_manager = OAuthTokenManager(str(sdk_creds_path))
    print(f"[DEBUG] OAuthTokenManager created, calling get_access_token()...")
    access_token = token_manager.get_access_token()
    print(f"[DEBUG] get_access_token() returned: {access_token[:80] if access_token else 'None'}...")

    if not access_token:
        print(f"[DEBUG] No access token obtained, raising error")
        raise ConfigurationError("Failed to obtain OAuth access token")

    # Call authenticated endpoint
    url = f"{aim_url.rstrip('/')}/api/v1/agents"

    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {access_token}"
    }

    if sdk_token_id:
        headers["X-SDK-Token"] = sdk_token_id

    response = requests.post(
        url,
        json=registration_data,
        headers=headers,
        timeout=30
    )

    if response.status_code not in [200, 201]:
        error_msg = response.json().get("error", "Unknown error")
        raise ConfigurationError(f"Registration failed: {error_msg}")

    credentials = response.json()

    # Backend returns 'id' but we need 'agent_id' for consistency
    if "id" in credentials and "agent_id" not in credentials:
        credentials["agent_id"] = credentials["id"]

    # Add client-side generated private key to credentials (backend doesn't send it back)
    credentials["private_key"] = private_key_b64
    credentials["public_key"] = public_key_b64  # Ensure public key is in response
    credentials["aim_url"] = aim_url  # Ensure URL is in response

    # Add OAuth tokens to credentials so they can be used for future API calls
    if token_manager and token_manager.credentials:
        if "refresh_token" in token_manager.credentials:
            credentials["refresh_token"] = token_manager.credentials["refresh_token"]
        if "access_token" in token_manager.credentials:
            credentials["access_token"] = token_manager.credentials["access_token"]

    # Save credentials locally
    _save_credentials(name, credentials)

    # Report MCP detections if any
    client = AIMClient(
        agent_id=credentials["agent_id"],
        public_key=credentials["public_key"],
        private_key=credentials["private_key"],
        aim_url=credentials["aim_url"],
        oauth_token_manager=token_manager  # Pass token manager for OAuth authentication
    )

    if talks_to:
        from .detection import auto_detect_mcps
        mcp_detections = auto_detect_mcps()
        if mcp_detections:
            try:
                result = client.report_detections(mcp_detections)
                print(f"   📡 Reported {result.get('detectionsProcessed', 0)} MCP detections")
            except Exception:
                pass  # Don't fail registration if reporting fails

    _print_registration_success(credentials)
    return client


def _register_via_api_key(
    name: str,
    aim_url: str,
    api_key: str,
    registration_data: Dict[str, Any],
    sdk_token_id: Optional[str],
    talks_to: Optional[List[str]]
) -> AIMClient:
    """Register agent using API key (manual mode)"""
    # Call public registration endpoint
    url = f"{aim_url.rstrip('/')}/api/v1/public/agents/register"

    headers = {
        "Content-Type": "application/json",
        "X-AIM-API-Key": api_key
    }

    if sdk_token_id:
        headers["X-SDK-Token"] = sdk_token_id

    response = requests.post(
        url,
        json=registration_data,
        headers=headers,
        timeout=30
    )

    if response.status_code != 201:
        error_msg = response.json().get("error", "Unknown error")
        raise ConfigurationError(f"Registration failed: {error_msg}")

    credentials = response.json()

    # Save credentials locally
    _save_credentials(name, credentials)

    # Report MCP detections if any
    client = AIMClient(
        agent_id=credentials["agent_id"],
        public_key=credentials["public_key"],
        private_key=credentials["private_key"],
        aim_url=credentials["aim_url"]
    )

    if talks_to:
        from .detection import auto_detect_mcps
        mcp_detections = auto_detect_mcps()
        if mcp_detections:
            try:
                result = client.report_detections(mcp_detections)
                print(f"   📡 Reported {result.get('detectionsProcessed', 0)} MCP detections")
            except Exception:
                pass  # Don't fail registration if reporting fails

    _print_registration_success(credentials)
    return client


def _print_registration_success(credentials: Dict[str, Any]):
    """Print success message after registration"""
    print(f"\n🎉 Agent registered successfully!")
    print(f"   Agent ID: {credentials['agent_id']}")
    print(f"   Name: {credentials['name']}")
    print(f"   Status: {credentials['status']}")
    print(f"   Trust Score: {credentials.get('trust_score', 'N/A')}")
    print(f"   Message: {credentials.get('message', 'Agent created')}")
    print(f"\n   ⚠️  Credentials saved to: {_get_credentials_path()}")
    print(f"   🔐 Private key will NOT be retrievable again - keep it safe!\n")
