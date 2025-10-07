"""
AIM Client - Core SDK functionality for automatic identity verification
"""

import base64
import functools
import hashlib
import json
import time
from typing import Any, Callable, Optional, Dict
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
        public_key: str,
        private_key: str,
        aim_url: str,
        timeout: int = 30,
        auto_retry: bool = True,
        max_retries: int = 3
    ):
        # Validate required parameters
        if not agent_id:
            raise ConfigurationError("agent_id is required")
        if not public_key:
            raise ConfigurationError("public_key is required")
        if not private_key:
            raise ConfigurationError("private_key is required")
        if not aim_url:
            raise ConfigurationError("aim_url is required")

        self.agent_id = agent_id
        self.aim_url = aim_url.rstrip('/')
        self.timeout = timeout
        self.auto_retry = auto_retry
        self.max_retries = max_retries

        # Initialize Ed25519 signing key
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

        self.public_key = public_key

        # Session for connection pooling
        self.session = requests.Session()
        self.session.headers.update({
            'User-Agent': f'AIM-Python-SDK/1.0.0',
            'Content-Type': 'application/json'
        })

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
        retry_count: int = 0
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

        try:
            response = self.session.request(
                method=method,
                url=url,
                json=data,
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
        timestamp = datetime.now(timezone.utc).isoformat()

        request_payload = {
            "agent_id": self.agent_id,
            "action_type": action_type,
            "resource": resource,
            "context": context or {},
            "timestamp": timestamp
        }

        # Create signature message (deterministic JSON)
        signature_message = json.dumps(request_payload, sort_keys=True)
        signature = self._sign_message(signature_message)

        # Add signature to payload
        request_payload["signature"] = signature
        request_payload["public_key"] = self.public_key

        # Send verification request
        try:
            result = self._make_request(
                method="POST",
                endpoint="/api/v1/verifications",
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
                    endpoint=f"/api/v1/verifications/{verification_id}"
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
                endpoint=f"/api/v1/verifications/{verification_id}/result",
                data={
                    "success": success,
                    "result_summary": result_summary,
                    "error_message": error_message,
                    "timestamp": datetime.now(timezone.utc).isoformat()
                }
            )
        except Exception:
            # Don't fail the action if logging fails
            pass

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
