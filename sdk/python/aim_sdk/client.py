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

import re
import warnings

from .exceptions import (
    AuthenticationError,
    VerificationError,
    ActionDeniedError,
    ConfigurationError
)

# Get version - we need to compute it directly to avoid circular import
import os as _os
def _get_client_version():
    """Get version from VERSION file or fallback."""
    version_file = _os.path.join(_os.path.dirname(__file__), "..", "VERSION")
    if _os.path.exists(version_file):
        with open(version_file, "r", encoding="utf-8") as vf:
            return vf.read().strip()
    try:
        from importlib.metadata import version
        return version("aim-sdk")
    except Exception:
        return "0.0.0"
__version__ = _get_client_version()
from .oauth import OAuthTokenManager, load_sdk_credentials
from .capability_detection import auto_detect_capabilities
from .console import console
from .risk_detector import detect_risk_level
from .security_logging import (
    security_logger,
    AuthnEventType,
    AuthzEventType,
    AgentEventType,
    SecurityEventType,
    EventSeverity,
)


# Capability format validation pattern (namespace:action)
CAPABILITY_PATTERN = re.compile(r'^[a-z][a-z0-9]*:[a-z][a-z0-9_]*$')

# Reserved namespaces for core capabilities
RESERVED_NAMESPACES = ['file', 'db', 'api', 'network', 'system', 'user', 'mcp', 'data']


# =============================================================================
# Agent Type Constants
# =============================================================================
# These constants define the valid agent types for registration and filtering.
# Use these constants instead of raw strings to prevent typos and enable IDE
# autocomplete.

class AgentType:
    """
    Constants for agent types used during registration and filtering.

    Agent types are organized into categories:

    **LLM Providers** - Agents built directly on LLM provider APIs:
        - CLAUDE: Anthropic Claude models
        - GPT: OpenAI GPT models
        - GEMINI: Google Gemini models
        - LLAMA: Meta Llama models
        - MISTRAL: Mistral AI models
        - COHERE: Cohere models

    **Frameworks** - Agents built using AI/LLM frameworks:
        - LANGCHAIN: LangChain framework agents
        - LLAMAINDEX: LlamaIndex framework agents
        - AUTOGEN: Microsoft AutoGen agents
        - CREWAI: CrewAI multi-agent systems
        - LANGGRAPH: LangGraph workflow agents
        - HAYSTACK: Haystack pipeline agents
        - SEMANTIC_KERNEL: Microsoft Semantic Kernel agents

    **Copilots & Assistants** - Interactive assistant agents:
        - COPILOT: GitHub Copilot and similar code assistants
        - ASSISTANT: General-purpose assistants
        - CHATBOT: Conversational chatbots

    **Autonomous Agents** - Self-directed agent systems:
        - AUTOGPT: AutoGPT autonomous agents
        - BABYAGI: BabyAGI task-driven agents

    **Generic Types**:
        - CUSTOM: Custom/other agent types
        - AI_AGENT: Legacy type for backward compatibility

    Example:
        >>> from aim_sdk import register_agent, AgentType
        >>> agent = register_agent("my-agent", agent_type=AgentType.LANGCHAIN)
    """

    # LLM Provider-based agents
    CLAUDE = "claude"
    GPT = "gpt"
    GEMINI = "gemini"
    LLAMA = "llama"
    MISTRAL = "mistral"
    COHERE = "cohere"

    # Framework-based agents
    LANGCHAIN = "langchain"
    LLAMAINDEX = "llamaindex"
    AUTOGEN = "autogen"
    CREWAI = "crewai"
    LANGGRAPH = "langgraph"
    HAYSTACK = "haystack"
    SEMANTIC_KERNEL = "semantic_kernel"

    # Copilot/Assistant types
    COPILOT = "copilot"
    ASSISTANT = "assistant"
    CHATBOT = "chatbot"

    # Autonomous agents
    AUTOGPT = "autogpt"
    BABYAGI = "babyagi"

    # Generic types
    CUSTOM = "custom"

    # Legacy support (deprecated, use specific types instead)
    AI_AGENT = "ai_agent"

    @classmethod
    def all_types(cls) -> list:
        """Return all valid agent types."""
        return [
            cls.CLAUDE, cls.GPT, cls.GEMINI, cls.LLAMA, cls.MISTRAL, cls.COHERE,
            cls.LANGCHAIN, cls.LLAMAINDEX, cls.AUTOGEN, cls.CREWAI, cls.LANGGRAPH,
            cls.HAYSTACK, cls.SEMANTIC_KERNEL,
            cls.COPILOT, cls.ASSISTANT, cls.CHATBOT,
            cls.AUTOGPT, cls.BABYAGI,
            cls.CUSTOM, cls.AI_AGENT
        ]

    @classmethod
    def llm_providers(cls) -> list:
        """Return LLM provider agent types."""
        return [cls.CLAUDE, cls.GPT, cls.GEMINI, cls.LLAMA, cls.MISTRAL, cls.COHERE]

    @classmethod
    def frameworks(cls) -> list:
        """Return framework-based agent types."""
        return [
            cls.LANGCHAIN, cls.LLAMAINDEX, cls.AUTOGEN, cls.CREWAI,
            cls.LANGGRAPH, cls.HAYSTACK, cls.SEMANTIC_KERNEL
        ]

    @classmethod
    def copilots_and_assistants(cls) -> list:
        """Return copilot and assistant agent types."""
        return [cls.COPILOT, cls.ASSISTANT, cls.CHATBOT]

    @classmethod
    def autonomous_agents(cls) -> list:
        """Return autonomous agent types."""
        return [cls.AUTOGPT, cls.BABYAGI]


def validate_capability_format(capability: str) -> tuple[bool, str]:
    """
    Validate that a capability string matches the namespace:action format.

    Args:
        capability: The capability string to validate

    Returns:
        Tuple of (is_valid, error_message)

    Example:
        >>> validate_capability_format("file:read")
        (True, "")
        >>> validate_capability_format("read_files")
        (False, "Capability must match format 'namespace:action' (e.g., 'file:read', 'payment:process')")
    """
    if not capability:
        return False, "Capability cannot be empty"

    if not CAPABILITY_PATTERN.match(capability):
        return False, (
            f"Capability '{capability}' must match format 'namespace:action' "
            "(e.g., 'file:read', 'payment:process'). "
            "Namespace and action must be lowercase alphanumeric, action can contain underscores."
        )

    return True, ""


def _warn_deprecated_capability_format(capability: str):
    """Warn if capability doesn't match the new namespace:action format."""
    is_valid, error_msg = validate_capability_format(capability)
    if not is_valid:
        warnings.warn(
            f"DEPRECATION WARNING: {error_msg}\n"
            "The old capability format (e.g., 'read_files') is deprecated. "
            "Please update to the new namespace:action format (e.g., 'file:read').\n"
            "See https://docs.aim.dev/capabilities for the full list of standard capabilities.",
            DeprecationWarning,
            stacklevel=3
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

        # Add Ed25519 signature authentication if signing key is available (highest priority)
        if self.signing_key and self.public_key and self.agent_id:
            try:
                import time
                import json

                # Create timestamp
                timestamp = str(int(time.time()))

                # Create message to sign: method + endpoint + timestamp + body
                message_parts = [method.upper(), endpoint, timestamp]
                json_body_str = None
                if data:
                    json_body_str = json.dumps(data, sort_keys=True)
                    message_parts.append(json_body_str)
                message = '\n'.join(message_parts)

                # Sign the message
                signature = self.signing_key.sign(message.encode('utf-8')).signature
                signature_b64 = Base64Encoder.encode(signature).decode('utf-8')

                # Add Ed25519 signature headers
                additional_headers['X-Agent-ID'] = self.agent_id
                additional_headers['X-Signature'] = signature_b64
                additional_headers['X-Timestamp'] = timestamp
                additional_headers['X-Public-Key'] = self.public_key

                # CRITICAL: Use pre-serialized JSON to ensure exact same format as signed
                if json_body_str:
                    additional_headers['Content-Type'] = 'application/json'

            except Exception as e:
                # If Ed25519 signing fails, fall back to other methods
                import logging
                logger = logging.getLogger(__name__)
                logger.warning(f"Ed25519 signing failed: {e}")

        # Add API key authentication if available (fallback)
        elif self.api_key:
            additional_headers['X-API-Key'] = self.api_key

        # Add OAuth authorization if token manager is available (fallback)
        elif self.oauth_token_manager:
            try:
                access_token = self.oauth_token_manager.get_access_token()
                if access_token:
                    additional_headers['Authorization'] = f'Bearer {access_token}'
            except Exception:
                # If OAuth token fails, no authentication will be added
                pass

        # Merge session headers with additional headers (additional_headers take precedence)
        merged_headers = {**self.session.headers, **additional_headers}

        try:
            # CRITICAL: If we have pre-serialized JSON (for Ed25519 signing), use it directly
            # Otherwise use json=data to let requests serialize it
            if 'json_body_str' in locals() and json_body_str is not None:
                response = self.session.request(
                    method=method,
                    url=url,
                    data=json_body_str,
                    headers=merged_headers,
                    timeout=self.timeout
                )
            else:
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

    def verify_capability(
        self,
        capability: str,
        resource: Optional[str] = None,
        context: Optional[Dict[str, Any]] = None,
        timeout_seconds: int = 300
    ) -> Dict:
        """
        Request verification for a capability from AIM.

        This method:
        1. Creates a verification request with capability details
        2. Signs the request with the agent's private key
        3. Sends the request to AIM
        4. Waits for approval/denial (up to timeout_seconds)
        5. Returns verification result

        Args:
            capability: Capability to verify (e.g., "db:read", "email:send", "file:write")
            resource: Resource being accessed (e.g., "users_table", "admin@example.com")
            context: Additional context about the capability usage
            timeout_seconds: Maximum time to wait for approval (default: 300s = 5min)

        Returns:
            Verification result dict with keys:
            - verified: bool (whether capability is approved)
            - verification_id: str (unique ID for this verification)
            - approved_by: str (user who approved, if applicable)
            - expires_at: str (ISO timestamp when approval expires)

        Raises:
            ActionDeniedError: If capability is explicitly denied
            VerificationError: If verification request fails
        """
        # Create verification request payload
        timestamp = datetime.utcnow().isoformat() + 'Z'  # Match backend expected format


        # Create signature for Ed25519 verification
        # The backend verifies the signature by reconstructing the JSON payload
        # We need to create a signature of the JSON payload itself
        # NOTE: Backend expects "action_type" not "capability" for signature verification
        signature_payload = {
            "action_type": capability,  # Backend expects action_type for signature
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
            "agentId": self.agent_id,  # camelCase for backend
            "capability": capability,
            "resource": resource,
            "context": context or {},
            "timestamp": timestamp,
            "signature": signature,  # Ed25519 signature in body
            "publicKey": self.public_key  # camelCase for backend
        }

        # SDK API endpoint
        endpoint = "/api/v1/sdk-api/verifications"

        # Send verification request using direct HTTP call to avoid double-signing
        try:
            url = f"{self.aim_url}{endpoint}"

            # Prepare headers - NO AUTH TOKENS for verification endpoint!
            # Verification uses cryptographic signature authentication (Ed25519)
            headers = {
                'Content-Type': 'application/json',
                'User-Agent': f'AIM-Python-SDK/1.0.0'
            }

            # Add SDK token header if available (for usage tracking only, not auth)
            if self.sdk_token_id:
                headers['X-SDK-Token'] = self.sdk_token_id
            
            response = self.session.request(
                method="POST",
                url=url,
                json=request_payload,
                headers=headers,
                timeout=self.timeout
            )

            # Handle authentication errors
            if response.status_code == 401:
                try:
                    error_detail = response.json().get("error", "unknown error")
                except:
                    error_detail = response.text
                raise AuthenticationError(f"Authentication failed - invalid agent credentials: {error_detail}")

            # Handle forbidden errors (denied action or agent status issue)
            if response.status_code == 403:
                try:
                    error_detail = response.json().get("error", "insufficient permissions")
                except:
                    error_detail = response.text or "insufficient permissions"
                raise AuthenticationError(f"Forbidden - {error_detail}")

            # Handle 404 - endpoint not found (server may not be running or endpoint doesn't exist)
            if response.status_code == 404:
                console.warning("AIM verification endpoint not found (404). Server may not be running.")
                return {
                    "verified": False,
                    "verification_id": None,
                    "status": "pending",
                    "error": "Endpoint not found - server may not be available"
                }

            # Handle other HTTP errors gracefully
            if response.status_code >= 400:
                error_msg = f"HTTP {response.status_code} error"
                try:
                    error_detail = response.json().get("error", response.text)
                    error_msg = f"{error_msg}: {error_detail}"
                except:
                    error_msg = f"{error_msg}: {response.text[:200]}"
                
                console.warning(f"Verification request failed: {error_msg}")
                return {
                    "verified": False,
                    "verification_id": None,
                    "status": "pending",
                    "error": error_msg
                }

            response.raise_for_status()
            result = response.json()

            verification_id = result.get("id")
            status = result.get("status")

            # If auto-approved, return immediately
            if status == "approved":
                security_logger.log_authorization(
                    AuthzEventType.CAPABILITY_GRANTED,
                    action=capability,
                    resource=resource,
                    granted=True,
                    agent_id=self.agent_id,
                    details={
                        "verification_id": verification_id,
                        "approved_by": result.get("approved_by"),
                        "auto_approved": True
                    }
                )
                return {
                    "verified": True,
                    "verification_id": verification_id,
                    "approved_by": result.get("approved_by"),
                    "expires_at": result.get("expires_at")
                }

            # If denied, raise error
            if status == "denied":
                reason = result.get("denial_reason", "Action denied by policy")
                security_logger.log_authorization(
                    AuthzEventType.CAPABILITY_DENIED,
                    action=capability,
                    resource=resource,
                    granted=False,
                    agent_id=self.agent_id,
                    details={"verification_id": verification_id, "denial_reason": reason}
                )
                raise ActionDeniedError(f"Action denied: {reason}")

            # If pending, poll for result
            if status == "pending":
                return self._wait_for_approval(verification_id, timeout_seconds)

            raise VerificationError(f"Unexpected verification status: {status}")

        except (AuthenticationError, ActionDeniedError) as e:
            security_logger.log_authorization(
                AuthzEventType.CAPABILITY_CHECK,
                action=capability,
                resource=resource,
                granted=False,
                agent_id=self.agent_id,
                error=str(e)
            )
            raise
        except requests.exceptions.RequestException as e:
            # Handle network errors (connection refused, timeout, etc.)
            security_logger.log_authorization(
                AuthzEventType.CAPABILITY_CHECK,
                action=capability,
                resource=resource,
                granted=False,
                agent_id=self.agent_id,
                error=f"Network error: {type(e).__name__}: {str(e)}"
            )
            console.warning(f"Network error during verification: {type(e).__name__}: {str(e)}")
            return {
                "verified": False,
                "verification_id": None,
                "status": "pending",
                "error": f"Network error: {type(e).__name__}: {str(e)}"
            }
        except json.JSONDecodeError as e:
            # Handle JSON parsing errors
            console.warning(f"Invalid JSON response from server: {str(e)}")
            return {
                "verified": False,
                "verification_id": None,
                "status": "pending",
                "error": f"JSON decode error: {str(e)}"
            }
        except Exception as e:
            # Catch all other exceptions
            console.warning(f"Unexpected error during verification: {type(e).__name__}: {str(e)}")
            return {
                "verified": False,
                "verification_id": None,
                "status": "pending",
                "error": f"Unexpected error: {type(e).__name__}: {str(e)}"
            }

    # Backwards compatibility alias
    def verify_action(
        self,
        action_type: str,
        resource: Optional[str] = None,
        context: Optional[Dict[str, Any]] = None,
        timeout_seconds: int = 300
    ) -> Dict:
        """
        DEPRECATED: Use verify_capability() instead.

        This method is kept for backwards compatibility but will be removed in a future version.
        """
        import warnings
        warnings.warn(
            "verify_action() is deprecated, use verify_capability() instead",
            DeprecationWarning,
            stacklevel=2
        )
        return self.verify_capability(
            capability=action_type,
            resource=resource,
            context=context,
            timeout_seconds=timeout_seconds
        )

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
                # Use direct HTTP call to avoid signature issues
                url = f"{self.aim_url}/api/v1/sdk-api/verifications/{verification_id}"
                
                # Prepare headers - use API key if available, otherwise OAuth
                headers = {
                    'Content-Type': 'application/json',
                    'User-Agent': f'AIM-Python-SDK/1.0.0'
                }
                
                if self.api_key:
                    headers['X-API-Key'] = self.api_key
                elif self.oauth_token_manager:
                    try:
                        access_token = self.oauth_token_manager.get_access_token()
                        if access_token:
                            headers['Authorization'] = f'Bearer {access_token}'
                    except Exception:
                        pass  # Continue without OAuth if it fails
                
                # Add SDK token header if available
                if self.sdk_token_id:
                    headers['X-SDK-Token'] = self.sdk_token_id
                
                response = self.session.request(
                    method="GET",
                    url=url,
                    headers=headers,
                    timeout=self.timeout
                )
                
                # Handle authentication errors
                if response.status_code == 401:
                    raise AuthenticationError("Authentication failed - invalid agent credentials")

                # Handle forbidden errors
                if response.status_code == 403:
                    raise AuthenticationError("Forbidden - insufficient permissions")

                # Handle 404 - endpoint not found
                if response.status_code == 404:
                    console.warning("Verification endpoint not found (404). Cannot poll for approval.")
                    raise VerificationError("Verification endpoint not available - cannot complete approval process")

                # Handle other HTTP errors
                if response.status_code >= 400:
                    error_msg = f"HTTP {response.status_code} error"
                    try:
                        error_detail = response.json().get("error", response.text)
                        error_msg = f"{error_msg}: {error_detail}"
                    except:
                        error_msg = f"{error_msg}: {response.text[:200]}"
                    # Continue polling on transient errors, but log the issue
                    console.warning(f"Error polling verification status: {error_msg}")
                    time.sleep(poll_interval)
                    poll_interval = min(poll_interval * 1.5, 10)
                    continue

                response.raise_for_status()
                result = response.json()

                status = result.get("status")

                if status == "approved":
                    security_logger.log_authorization(
                        AuthzEventType.JIT_APPROVED,
                        action="capability_verification",
                        resource=None,
                        granted=True,
                        agent_id=self.agent_id,
                        details={
                            "verification_id": verification_id,
                            "approved_by": result.get("approved_by"),
                            "expires_at": result.get("expires_at")
                        }
                    )
                    return {
                        "verified": True,
                        "verification_id": verification_id,
                        "approved_by": result.get("approved_by"),
                        "expires_at": result.get("expires_at")
                    }

                if status == "denied":
                    reason = result.get("denial_reason", "Action denied")
                    security_logger.log_authorization(
                        AuthzEventType.JIT_DENIED,
                        action="capability_verification",
                        resource=None,
                        granted=False,
                        agent_id=self.agent_id,
                        details={"verification_id": verification_id, "denial_reason": reason}
                    )
                    raise ActionDeniedError(f"Action denied: {reason}")

                # Still pending, wait and retry
                time.sleep(poll_interval)
                poll_interval = min(poll_interval * 1.5, 10)  # Exponential backoff up to 10s

            except (AuthenticationError, ActionDeniedError, VerificationError):
                raise
            except requests.exceptions.RequestException as e:
                # Handle network errors - continue polling on transient network issues
                console.warning(f"Network error while polling: {type(e).__name__}: {str(e)}")
                time.sleep(poll_interval)
                poll_interval = min(poll_interval * 1.5, 10)
            except json.JSONDecodeError as e:
                # Handle JSON parsing errors - continue polling
                console.warning(f"Invalid JSON response while polling: {str(e)}")
                time.sleep(poll_interval)
                poll_interval = min(poll_interval * 1.5, 10)
            except Exception as e:
                # Continue polling on any other transient errors
                console.warning(f"Unexpected error while polling: {type(e).__name__}: {str(e)}")
                time.sleep(poll_interval)
                poll_interval = min(poll_interval * 1.5, 10)

        security_logger.log_authorization(
            AuthzEventType.JIT_TIMEOUT,
            action="capability_verification",
            resource=None,
            granted=False,
            agent_id=self.agent_id,
            details={"verification_id": verification_id, "timeout_seconds": timeout_seconds}
        )
        raise VerificationError(f"Verification timeout after {timeout_seconds} seconds")

    def log_capability_result(
        self,
        verification_id: str,
        success: bool,
        result_summary: Optional[str] = None,
        error_message: Optional[str] = None
    ):
        """
        Log the result of a capability execution to AIM.

        This helps AIM track agent behavior and build trust scores.

        Args:
            verification_id: ID from verify_capability response
            success: Whether the capability execution succeeded
            result_summary: Brief summary of the result
            error_message: Error message if execution failed
        """
        try:
            # Use direct HTTP call to avoid signature issues
            url = f"{self.aim_url}/api/v1/sdk-api/verifications/{verification_id}/result"

            # Prepare headers - use API key if available, otherwise OAuth
            headers = {
                'Content-Type': 'application/json',
                'User-Agent': f'AIM-Python-SDK/1.0.0'
            }

            if self.api_key:
                headers['X-API-Key'] = self.api_key
            elif self.oauth_token_manager:
                try:
                    access_token = self.oauth_token_manager.get_access_token()
                    if access_token:
                        headers['Authorization'] = f'Bearer {access_token}'
                except Exception:
                    pass  # Continue without OAuth if it fails

            # Add SDK token header if available
            if self.sdk_token_id:
                headers['X-SDK-Token'] = self.sdk_token_id

            response = self.session.request(
                method="POST",
                url=url,
                json={
                    "result": "success" if success else "failure",
                    "result_summary": result_summary,
                    "error_message": error_message,
                    "timestamp": datetime.now(timezone.utc).isoformat()
                },
                headers=headers,
                timeout=self.timeout
            )

            # Don't raise on errors for logging - just continue
            response.raise_for_status()
        except Exception:
            # Don't fail the action if logging fails
            pass

    def report_execution_status(
        self,
        verification_id: str,
        executed: bool,
        strict_mode: bool,
        execution_error: Optional[str] = None
    ):
        """
        Report execution status to AIM backend.

        This method is called by decorators after a function execution attempt to report
        whether the function was actually executed. This enables accurate dashboard messaging:
        - If strict_mode=True and verification denied: executed=False (blocked)
        - If strict_mode=False and verification denied: executed=True (logged but allowed)
        - If verification approved: executed=True (normal execution)

        Args:
            verification_id: ID from verify_capability response
            executed: Whether the decorated function was actually executed
            strict_mode: Whether SDK was in strict mode (AIM_STRICT_MODE env var)
            execution_error: Error message if execution failed

        Note:
            This method is fire-and-forget - it won't raise exceptions on failure.
        """
        if not verification_id:
            return  # Can't report without verification ID

        try:
            url = f"{self.aim_url}/api/v1/sdk-api/verifications/{verification_id}/execution-status"

            headers = {
                'Content-Type': 'application/json',
                'User-Agent': f'AIM-Python-SDK/1.0.0'
            }

            if self.api_key:
                headers['X-API-Key'] = self.api_key
            elif self.oauth_token_manager:
                try:
                    access_token = self.oauth_token_manager.get_access_token()
                    if access_token:
                        headers['Authorization'] = f'Bearer {access_token}'
                except Exception:
                    pass  # Continue without OAuth if it fails

            if self.sdk_token_id:
                headers['X-SDK-Token'] = self.sdk_token_id

            payload = {
                "executed": executed,
                "strictMode": strict_mode,
                "executedAt": datetime.now(timezone.utc).isoformat()
            }

            if execution_error:
                payload["executionError"] = execution_error

            response = self.session.request(
                method="POST",
                url=url,
                json=payload,
                headers=headers,
                timeout=self.timeout
            )

            # Don't raise on errors - this is fire-and-forget
            response.raise_for_status()
        except Exception:
            # Don't fail the action if reporting fails
            pass

    # Backwards compatibility alias
    def log_action_result(
        self,
        verification_id: str,
        success: bool,
        result_summary: Optional[str] = None,
        error_message: Optional[str] = None
    ):
        """
        DEPRECATED: Use log_capability_result() instead.

        Log the result of an action execution to AIM.
        """
        import warnings
        warnings.warn(
            "log_action_result() is deprecated, use log_capability_result() instead",
            DeprecationWarning,
            stacklevel=2
        )
        return self.log_capability_result(
            verification_id=verification_id,
            success=success,
            result_summary=result_summary,
            error_message=error_message
        )

    def request_capability(
        self,
        capability_type: str,
        reason: str,
        metadata: Optional[Dict] = None
    ) -> Dict:
        """
        Request an additional capability for the agent.

        When an agent needs a capability that wasn't granted during registration,
        it can request it through this method. The request will be sent to admins
        for approval.

        Args:
            capability_type: Type of capability being requested (e.g., "write_database", "send_bulk_email")
            reason: Business justification for the capability request (minimum 10 characters)
            metadata: Optional additional context for the request (e.g., {"target_table": "users", "scope": "read-write"})

        Returns:
            Dict containing:
            - id: str - Capability request ID
            - agent_id: str - Agent ID
            - capability_type: str - Requested capability
            - status: str - Request status ("pending", "approved", "rejected")
            - requested_at: str - ISO timestamp of request
            - metadata: dict - Optional metadata provided with the request

        Raises:
            ConfigurationError: If reason is too short or capability_type is invalid
            VerificationError: If request fails

        Example:
            result = client.request_capability(
                capability_type="write_database",
                reason="Need to update user records in the database for analytics",
                metadata={"target_table": "users", "scope": "read-write"}
            )
            print(f"Request ID: {result['id']}, Status: {result['status']}")
        """
        # Validate inputs
        if not capability_type or not isinstance(capability_type, str):
            raise ConfigurationError("capability_type must be a non-empty string")

        if not reason or len(reason) < 10:
            raise ConfigurationError("reason must be at least 10 characters")

        # Prepare request payload (using camelCase for API consistency)
        request_data = {
            "capabilityType": capability_type,
            "reason": reason
        }

        # Add metadata if provided
        if metadata:
            request_data["metadata"] = metadata

        try:
            # Make request to SDK API endpoint
            result = self._make_request(
                method="POST",
                endpoint=f"/api/v1/sdk-api/agents/{self.agent_id}/capability-requests",
                data=request_data
            )

            return result

        except VerificationError as e:
            # Handle 409 Conflict - capability request already exists
            if "409" in str(e):
                # Fetch existing capability requests and return the matching one
                try:
                    existing = self._make_request(
                        method="GET",
                        endpoint=f"/api/v1/sdk-api/agents/{self.agent_id}/capability-requests"
                    )
                    # Find the matching capability request
                    requests_list = existing.get("requests", existing) if isinstance(existing, dict) else existing
                    if isinstance(requests_list, list):
                        for req in requests_list:
                            if req.get("capabilityType") == capability_type or req.get("capability_type") == capability_type:
                                console.info(f"Capability request for '{capability_type}' already exists (status: {req.get('status', 'unknown')})")
                                return req
                    # If we can't find the specific request, return a helpful message
                    return {
                        "status": "already_exists",
                        "message": f"A capability request for '{capability_type}' already exists for this agent",
                        "capability_type": capability_type
                    }
                except Exception:
                    # If we can't fetch existing, return helpful info
                    return {
                        "status": "already_exists",
                        "message": f"A capability request for '{capability_type}' already exists for this agent",
                        "capability_type": capability_type
                    }
            raise VerificationError(f"Capability request failed: {e}")
        except Exception as e:
            raise VerificationError(f"Capability request failed: {e}")

    def register_capability(
        self,
        capability_type: str,
        description: str = "",
        risk_level: str = "medium"
    ) -> Dict:
        """
        Register a capability that this agent intends to use.

        This is different from request_capability() - it declares the agent's
        intended capabilities for visibility and tracking, rather than requesting
        admin approval for a new capability.

        Behavior depends on organization's enforcement mode:
        - MONITORING mode: Capabilities are automatically granted
        - STRICT mode: Creates a pending request requiring admin approval

        Note: Capabilities declared during initial agent registration (via the
        `capabilities` parameter in `secure()`) are ALWAYS auto-granted regardless
        of enforcement mode. This method is for adding capabilities AFTER registration.

        Args:
            capability_type: The capability to register (e.g., "weather:check", "db:read")
            description: Optional description of what this capability does
            risk_level: Risk level ("low", "medium", "high", "critical")

        Returns:
            Dict containing:
            - success: bool - Whether registration succeeded
            - capability_type: str - The registered capability
            - status: str - "granted" (monitoring), "pending" (strict), "already_exists"
            - message: str - Human-readable message
            - request_id: str - Only present if status is "pending"

        Example:
            # Register capabilities after agent startup
            result = agent.register_capability("weather:check", "Check weather data", risk_level="low")
            if result["status"] == "pending":
                print("Awaiting admin approval...")
            elif result["status"] == "granted":
                print("Capability granted!")
        """
        # Track locally to avoid duplicate registrations in same session
        if not hasattr(self, '_registered_capabilities'):
            self._registered_capabilities = set()

        if capability_type in self._registered_capabilities:
            return {
                "success": True,
                "capability_type": capability_type,
                "status": "already_registered",
                "message": f"Capability '{capability_type}' already registered in this session"
            }

        # Validate capability format
        _warn_deprecated_capability_format(capability_type)

        try:
            # Use the capability registration endpoint
            request_data = {
                "capabilityType": capability_type,
                "description": description or f"Registered capability: {capability_type}",
                "riskLevel": risk_level,
            }

            result = self._make_request(
                method="POST",
                endpoint=f"/api/v1/sdk-api/agents/{self.agent_id}/capabilities/register",
                data=request_data
            )

            # Track as registered (even if pending - we don't want duplicate requests)
            self._registered_capabilities.add(capability_type)

            status = result.get("status", "registered")
            message = result.get("message", f"Capability '{capability_type}' registered")

            # Display appropriate console output based on status
            if status == "granted":
                console.capability_registered(capability_type, risk_level)
            elif status == "pending":
                console.info(f"Capability '{capability_type}' pending admin approval (strict mode)")
            elif status == "already_exists":
                pass  # Silent - already exists

            response = {
                "success": True,
                "capability_type": capability_type,
                "status": status,
                "message": message
            }

            # Include request_id if pending
            if status == "pending" and "requestId" in result:
                response["request_id"] = result["requestId"]

            return response

        except VerificationError as e:
            error_str = str(e)
            # Handle 409 Conflict - capability already exists
            if "409" in error_str or "already" in error_str.lower():
                self._registered_capabilities.add(capability_type)
                return {
                    "success": True,
                    "capability_type": capability_type,
                    "status": "already_exists",
                    "message": f"Capability '{capability_type}' already exists"
                }
            # Handle 404 - endpoint not available (older server version)
            if "404" in error_str:
                # Silently succeed - server doesn't support capability registration
                self._registered_capabilities.add(capability_type)
                return {
                    "success": True,
                    "capability_type": capability_type,
                    "status": "not_tracked",
                    "message": "Capability registration not supported by server"
                }
            raise
        except Exception as e:
            # Don't fail the agent if registration fails - just log it
            console.warning(f"Failed to register capability '{capability_type}': {e}")
            return {
                "success": False,
                "capability_type": capability_type,
                "status": "error",
                "message": str(e)
            }

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
        Register an MCP server to this agent's mcp_servers list.

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

    def attest_mcp(
        self,
        mcp_server_id: str,
        capabilities: Optional[List[str]] = None,
        connection_successful: bool = True,
        health_check_passed: bool = True,
        connection_latency_ms: int = 0,
        mcp_url: Optional[str] = None,
        mcp_name: Optional[str] = None
    ) -> Dict:
        """
        Attest to an MCP server's capabilities and availability.

        This creates a cryptographically signed attestation that this agent
        has verified the MCP server is operational and provides the claimed capabilities.
        Attestations from multiple agents build trust in MCP servers.

        The attestation flow:
        1. Request a challenge from the server (proof of private key possession)
        2. Sign the attestation payload including the challenge
        3. Submit the signed attestation

        Args:
            mcp_server_id: UUID of the MCP server to attest
            capabilities: List of MCP capabilities/tools verified (e.g., ["read_file", "write_file"])
            connection_successful: Whether connection to MCP server succeeded
            health_check_passed: Whether MCP server health check passed
            connection_latency_ms: Connection latency in milliseconds
            mcp_url: Optional MCP server URL (for audit trail)
            mcp_name: Optional MCP server name (for audit trail)

        Returns:
            Dict with keys:
                - success: bool
                - attestationId: str - Unique attestation ID
                - mcpConfidenceScore: float - Updated MCP confidence score
                - attestationCount: int - Total attestations for this MCP
                - message: str

        Example:
            # Attest to filesystem MCP server
            result = client.attest_mcp(
                mcp_server_id="550e8400-e29b-41d4-a716-446655440000",
                capabilities=["read_file", "write_file", "list_directory"],
                connection_successful=True,
                health_check_passed=True,
                connection_latency_ms=45
            )
            print(f"Attestation recorded: {result['attestationId']}")
            print(f"MCP confidence score: {result['mcpConfidenceScore']}%")

        Raises:
            AuthenticationError: If authentication fails or agent not verified
            VerificationError: If attestation request fails
            ConfigurationError: If signing keys not configured
        """
        if not self.signing_key:
            raise ConfigurationError(
                "attest_mcp requires Ed25519 signing keys. "
                "API key mode does not support cryptographic attestations."
            )

        try:
            # Step 1: Request a challenge for proof of private key possession
            challenge_url = f"{self.aim_url}/api/v1/mcp-servers/{mcp_server_id}/challenge?agent_id={self.agent_id}"
            challenge_response = self.session.get(challenge_url, timeout=self.timeout)

            if challenge_response.status_code == 404:
                raise VerificationError(f"MCP server {mcp_server_id} not found")
            if challenge_response.status_code == 403:
                raise AuthenticationError("Only verified agents can attest MCP servers")

            challenge_response.raise_for_status()
            challenge_data = challenge_response.json()
            challenge = challenge_data.get("challenge")

            if not challenge:
                raise VerificationError("Failed to obtain attestation challenge")

            # Step 2: Create attestation payload
            timestamp = datetime.now(timezone.utc).isoformat()

            attestation_payload = {
                "agentId": self.agent_id,
                "mcpUrl": mcp_url or "",
                "mcpName": mcp_name or "",
                "capabilitiesFound": capabilities or [],
                "connectionSuccessful": connection_successful,
                "healthCheckPassed": health_check_passed,
                "connectionLatencyMs": connection_latency_ms,
                "timestamp": timestamp,
                "sdkVersion": f"aim-sdk-python@{__version__}",
                "challenge": challenge  # Include server-generated challenge
            }

            # Step 3: Create canonical JSON and sign it
            # The backend verifies by reconstructing this exact JSON
            canonical_json = json.dumps(attestation_payload, sort_keys=True, separators=(',', ':'))
            signature = self._sign_message(canonical_json)

            # Step 4: Submit the attestation
            attest_url = f"{self.aim_url}/api/v1/mcp-servers/{mcp_server_id}/attest"
            attest_request = {
                "attestation": attestation_payload,
                "signature": signature
            }

            attest_response = self.session.post(
                attest_url,
                json=attest_request,
                timeout=self.timeout
            )

            if attest_response.status_code == 403:
                error_msg = attest_response.json().get("message", "Attestation forbidden")
                raise AuthenticationError(f"Attestation denied: {error_msg}")

            attest_response.raise_for_status()
            return attest_response.json()

        except (AuthenticationError, ConfigurationError):
            raise
        except requests.exceptions.RequestException as e:
            raise VerificationError(f"MCP attestation failed: {e}")
        except Exception as e:
            raise VerificationError(f"MCP attestation failed: {e}")

    def use_mcp_tool(
        self,
        server_id: str,
        tool_name: str,
        mcp_url: str = "",
        mcp_name: str = "",
        auto_attest: bool = True,
        force_attest: bool = False,
        discovery_timeout: float = 30.0
    ) -> Dict:
        """
        Record MCP tool usage with smart attestation for supply chain security.

        This is a convenience method that wraps integrations.mcp.use_mcp_tool(),
        providing smart attestation that automatically creates attestations when:
        - First use of an MCP server by this agent
        - First use of a NEW tool on a known server
        - Attestation is stale (>24 hours old)
        - Capability drift detected (tools added/removed)

        Args:
            server_id: UUID of the MCP server being used
            tool_name: Name of the tool being used (e.g., "read_file", "search")
            mcp_url: URL of the MCP server (required for attestation on first use)
            mcp_name: Name of the MCP server (required for attestation on first use)
            auto_attest: Enable smart attestation (default: True)
            force_attest: Force attestation even if cached (default: False)
            discovery_timeout: Timeout for capability discovery in seconds (default: 30.0)

        Returns:
            Dict containing connection response and attestation info

        Example:
            result = client.use_mcp_tool(
                server_id="04531081-dd02-43aa-9067-a4e656de5591",
                tool_name="read_file",
                mcp_url="npx -y @modelcontextprotocol/server-filesystem /tmp",
                mcp_name="filesystem-mcp"
            )

            if result.get("attestation"):
                print(f"Attestation: {result['attestation']['reason']}")
        """
        from aim_sdk.integrations.mcp import use_mcp_tool
        return use_mcp_tool(
            aim_client=self,
            server_id=server_id,
            tool_name=tool_name,
            mcp_url=mcp_url,
            mcp_name=mcp_name,
            auto_attest=auto_attest,
            force_attest=force_attest,
            discovery_timeout=discovery_timeout
        )

    def get_attestation_cache(self):
        """
        Get the attestation cache for this agent.

        Returns:
            AttestationCache instance for this agent

        Example:
            cache = client.get_attestation_cache()
            stats = cache.get_supply_chain_report()
            print(f"MCP servers used: {stats['mcpServerCount']}")
        """
        from aim_sdk.attestation_cache import AttestationCache
        return AttestationCache(agent_id=self.agent_id)

    def report_mcp_supply_chain(self) -> Dict:
        """
        Report MCP supply chain analytics to the AIM backend.

        This syncs local tool usage statistics to the backend for:
        - Dashboard visualization of MCP usage patterns
        - Anomaly detection (sudden spike in dangerous tool usage)
        - Compliance reporting (which agents used which tools when)

        Returns:
            Dict with keys:
                - success: bool
                - serversReported: int
                - totalInvocations: int
                - message: str

        Example:
            result = client.report_mcp_supply_chain()
            print(f"Reported {result['serversReported']} MCP servers")
        """
        try:
            from aim_sdk.attestation_cache import AttestationCache

            cache = AttestationCache(agent_id=self.agent_id)
            report = cache.get_supply_chain_report()

            # Transform report for backend
            payload = {
                "agentId": self.agent_id,
                "mcpServers": {},
                "reportedAt": report["generatedAt"]
            }

            # Get full cache data for detailed reporting
            full_stats = cache.get_all_stats()
            servers_data = full_stats.get("mcpServers", {})

            for server_id, server_stats in servers_data.items():
                payload["mcpServers"][server_id] = {
                    "toolUsage": server_stats.get("toolsUsed", {}),
                    "lastAttestedAt": server_stats.get("lastAttestedAt"),
                    "capabilitiesAttested": server_stats.get("capabilitiesAttested", []),
                    "confidenceScore": server_stats.get("confidenceScore"),
                    "firstSeenAt": server_stats.get("firstSeenAt")
                }

            # Submit to backend
            result = self._make_request(
                method="POST",
                endpoint=f"/api/v1/sdk-api/agents/{self.agent_id}/mcp-usage-report",
                data=payload
            )

            return {
                "success": True,
                "serversReported": len(payload["mcpServers"]),
                "totalInvocations": report["totalToolInvocations"],
                "message": f"Reported {len(payload['mcpServers'])} MCP servers with {report['totalToolInvocations']} tool invocations"
            }

        except Exception as e:
            console.warning(f"Failed to report MCP supply chain: {e}")
            return {
                "success": False,
                "serversReported": 0,
                "totalInvocations": 0,
                "error": str(e)
            }

    def get_mcp_supply_chain_report(self) -> Dict:
        """
        Generate a local MCP supply chain analytics report.

        Returns a comprehensive view of MCP usage patterns for this agent,
        including attestation status, tool usage counts, and top tools.

        Returns:
            Dict with supply chain analytics:
                - agentId: str
                - mcpServerCount: int
                - totalToolsUsed: int
                - totalToolInvocations: int
                - servers: dict of server stats
                - generatedAt: str

        Example:
            report = client.get_mcp_supply_chain_report()
            for server_id, stats in report["servers"].items():
                print(f"{server_id}: {stats['invocationCount']} invocations")
                for tool, count in stats["topTools"]:
                    print(f"  {tool}: {count}")
        """
        from aim_sdk.attestation_cache import AttestationCache
        cache = AttestationCache(agent_id=self.agent_id)
        return cache.get_supply_chain_report()

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

    # ==========================================================================
    # Agent Management Methods
    # These methods allow authenticated users to manage agents programmatically
    # Consistent with Generic SDK feature set
    # ==========================================================================

    def create_new_agent(
        self,
        name: str,
        display_name: Optional[str] = None,
        description: Optional[str] = None,
        agent_type: str = "ai_agent",
        version: Optional[str] = None,
        repository_url: Optional[str] = None,
        documentation_url: Optional[str] = None,
        capabilities: Optional[List[str]] = None,
        mcp_servers: Optional[List[str]] = None,
        # Backward compatibility alias
        talks_to: Optional[List[str]] = None  # DEPRECATED: Use mcp_servers instead
    ) -> Dict:
        """
        Create/register a new agent through an authenticated client.

        This method allows an authenticated user (via OAuth or API key) to create
        a new agent programmatically. The agent will be associated with the user's
        organization.

        This is the "Generic SDK" way to register agents - use this when you need
        to create agents on behalf of others or automate agent provisioning.

        Args:
            name: Unique name/identifier for the agent (required)
            display_name: Human-readable display name (defaults to name)
            description: Agent description (defaults to auto-generated)
            agent_type: Type of agent. Use AgentType constants for type safety.
                Valid types include:
                - LLM Providers: "claude", "gpt", "gemini", "llama", "mistral", "cohere"
                - Frameworks: "langchain", "llamaindex", "autogen", "crewai",
                  "langgraph", "haystack", "semantic_kernel"
                - Copilots: "copilot", "assistant", "chatbot"
                - Autonomous: "autogpt", "babyagi"
                - Generic: "custom", "ai_agent" (legacy)
                Default: "ai_agent"
            version: Agent version (e.g., "1.0.0")
            repository_url: GitHub/GitLab repository URL
            documentation_url: Documentation URL
            capabilities: List of capabilities the agent has
            mcp_servers: List of MCP server names the agent communicates with

        Returns:
            Dict containing the newly created agent details:
            - id: str - Agent ID (UUID)
            - name: str - Agent name
            - display_name: str - Display name
            - description: str - Description
            - agent_type: str - Agent type
            - status: str - Verification status
            - trust_score: float - Initial trust score
            - public_key: str - Agent's public key (for verification)
            - private_key: str - Agent's private key (STORE SECURELY!)
            - organization_id: str - Organization ID
            - created_at: str - ISO timestamp

        Example:
            # Using OAuth credentials (SDK download mode)
            from aim_sdk import AgentType
            client = AIMClient(agent_id="admin-agent", aim_url="https://aim.example.com", ...)

            # Create a LangChain agent
            new_agent = client.create_new_agent(
                name="my-langchain-agent",
                display_name="My LangChain Agent",
                description="An agent for processing data",
                agent_type=AgentType.LANGCHAIN,
                capabilities=["db:read", "file:write"]
            )

            # SECURITY: Never print private keys - they're saved to secure file storage
            # Credentials are stored in ~/.aim/agents/{name}.json with 0600 permissions
            print(f"✅ Created agent: {new_agent['id']}")
            print(f"🔐 Credentials saved to: ~/.aim/agents/{name}.json")

        Raises:
            ConfigurationError: If name is missing or invalid
            AuthenticationError: If authentication fails
            VerificationError: If agent creation fails
        """
        # Validate required parameters
        if not name or not isinstance(name, str):
            raise ConfigurationError("name is required and must be a non-empty string")

        # Generate Ed25519 keypair for the new agent
        from nacl.signing import SigningKey
        from nacl.encoding import Base64Encoder

        signing_key = SigningKey.generate()
        private_key_bytes = bytes(signing_key)  # 32-byte seed
        public_key_bytes = signing_key.verify_key.encode()  # 32-byte public key

        # For Go compatibility, create 64-byte private key (seed + public key)
        private_key_full = private_key_bytes + public_key_bytes
        private_key_b64 = base64.b64encode(private_key_full).decode('utf-8')
        public_key_b64 = base64.b64encode(public_key_bytes).decode('utf-8')

        # Handle backward compatibility: merge talks_to into mcp_servers
        if talks_to is not None:
            import warnings
            warnings.warn(
                "The 'talks_to' parameter is deprecated. Use 'mcp_servers' instead.",
                DeprecationWarning,
                stacklevel=2
            )
            if mcp_servers is None:
                mcp_servers = talks_to
            else:
                # Merge both, preferring mcp_servers but adding any unique from talks_to
                mcp_servers = list(set(mcp_servers + talks_to))

        # Prepare registration payload
        registration_data = {
            "name": name,
            "displayName": display_name or name,
            "description": description or f"Agent {name} created via AIM SDK",
            "agentType": agent_type,
            "publicKey": public_key_b64
        }

        if version:
            registration_data["version"] = version
        if repository_url:
            registration_data["repositoryUrl"] = repository_url
        if documentation_url:
            registration_data["documentationUrl"] = documentation_url
        if capabilities:
            registration_data["capabilities"] = capabilities
        if mcp_servers:
            registration_data["talksTo"] = mcp_servers  # Backend still uses talksTo

        try:
            # Use authenticated endpoint
            result = self._make_request(
                method="POST",
                endpoint="/api/v1/agents",
                data=registration_data
            )

            # Add the private key to the result (backend doesn't store it)
            if result:
                result["private_key"] = private_key_b64
                
                # CRITICAL: Use the backend's public key (what's in database), not what we generated
                # The backend's public key is the source of truth since it's stored in the database
                backend_pub_key = result.get('public_key') or result.get('publicKey')
                if backend_pub_key:
                    result["public_key"] = backend_pub_key
                else:
                    # Fallback: use generated key if backend didn't return one
                    result["public_key"] = public_key_b64

                # Normalize agent_id field
                if "id" in result and "agent_id" not in result:
                    result["agent_id"] = result["id"]

            return result

        except (AuthenticationError, VerificationError):
            raise
        except Exception as e:
            raise VerificationError(f"Agent creation failed: {e}")

    def list_agents(
        self,
        limit: int = 50,
        offset: int = 0,
        status: Optional[str] = None,
        agent_type: Optional[str] = None
    ) -> Dict:
        """
        List agents in the user's organization.

        Args:
            limit: Maximum number of agents to return (default: 50, max: 100)
            offset: Pagination offset (default: 0)
            status: Filter by status ("pending", "verified", "denied")
            agent_type: Filter by agent type ("ai_agent", "mcp_server")

        Returns:
            Dict containing:
            - agents: List[Dict] - List of agent objects
            - total: int - Total number of agents
            - limit: int - Items per page
            - offset: int - Current offset

        Example:
            result = client.list_agents(limit=10)
            for agent in result["agents"]:
                print(f"{agent['name']}: {agent['status']} (trust: {agent['trust_score']})")

        Raises:
            AuthenticationError: If authentication fails
            VerificationError: If request fails
        """
        # Build query params
        params = []
        params.append(f"limit={min(limit, 100)}")
        params.append(f"offset={offset}")
        if status:
            params.append(f"status={status}")
        if agent_type:
            params.append(f"agent_type={agent_type}")

        query_string = "&".join(params)

        try:
            result = self._make_request(
                method="GET",
                endpoint=f"/api/v1/agents?{query_string}"
            )
            return result

        except (AuthenticationError, VerificationError):
            raise
        except Exception as e:
            raise VerificationError(f"Failed to list agents: {e}")

    def get_agent_details(
        self,
        agent_id: Optional[str] = None
    ) -> Dict:
        """
        Get details of a specific agent.

        Args:
            agent_id: Agent ID to fetch (defaults to current agent)

        Returns:
            Dict containing full agent details:
            - id: str - Agent ID
            - name: str - Agent name
            - display_name: str - Display name
            - description: str - Description
            - agent_type: str - Agent type
            - status: str - Verification status
            - trust_score: float - Current trust score
            - capabilities: List[str] - Granted capabilities
            - mcp_servers: List[str] - MCP servers the agent communicates with
            - organization_id: str - Organization ID
            - created_at: str - ISO timestamp
            - updated_at: str - ISO timestamp

        Example:
            # Get current agent details
            my_agent = client.get_agent_details()
            print(f"Trust Score: {my_agent['trust_score']}")

            # Get another agent's details
            other_agent = client.get_agent_details("uuid-of-other-agent")

        Raises:
            AuthenticationError: If authentication fails
            VerificationError: If request fails
        """
        target_agent_id = agent_id or self.agent_id

        try:
            result = self._make_request(
                method="GET",
                endpoint=f"/api/v1/agents/{target_agent_id}"
            )
            return result

        except (AuthenticationError, VerificationError):
            raise
        except Exception as e:
            raise VerificationError(f"Failed to get agent details: {e}")

    def update_agent(
        self,
        agent_id: Optional[str] = None,
        display_name: Optional[str] = None,
        description: Optional[str] = None,
        version: Optional[str] = None,
        repository_url: Optional[str] = None,
        documentation_url: Optional[str] = None
    ) -> Dict:
        """
        Update an agent's details.

        Args:
            agent_id: Agent ID to update (defaults to current agent)
            display_name: New display name
            description: New description
            version: New version
            repository_url: New repository URL
            documentation_url: New documentation URL

        Returns:
            Dict containing updated agent details

        Example:
            # Update current agent
            updated = client.update_agent(
                display_name="My Updated Agent",
                version="2.0.0"
            )

        Raises:
            ConfigurationError: If no updates provided
            AuthenticationError: If authentication fails
            VerificationError: If request fails
        """
        target_agent_id = agent_id or self.agent_id

        # Build update payload
        update_data = {}
        if display_name is not None:
            update_data["display_name"] = display_name
        if description is not None:
            update_data["description"] = description
        if version is not None:
            update_data["version"] = version
        if repository_url is not None:
            update_data["repository_url"] = repository_url
        if documentation_url is not None:
            update_data["documentation_url"] = documentation_url

        if not update_data:
            raise ConfigurationError("At least one field must be provided for update")

        try:
            result = self._make_request(
                method="PUT",
                endpoint=f"/api/v1/agents/{target_agent_id}",
                data=update_data
            )
            return result

        except (AuthenticationError, VerificationError):
            raise
        except Exception as e:
            raise VerificationError(f"Failed to update agent: {e}")

    def delete_agent(
        self,
        agent_id: str
    ) -> Dict:
        """
        Delete/deactivate an agent (soft delete).

        WARNING: This action cannot be easily undone. The agent will be marked
        as deleted and will no longer be able to authenticate or perform actions.

        Args:
            agent_id: Agent ID to delete (required, cannot delete current agent)

        Returns:
            Dict containing:
            - success: bool
            - message: str

        Example:
            result = client.delete_agent("uuid-of-agent-to-delete")
            print(result["message"])

        Raises:
            ConfigurationError: If trying to delete current agent
            AuthenticationError: If authentication fails
            VerificationError: If request fails
        """
        if agent_id == self.agent_id:
            raise ConfigurationError("Cannot delete the currently authenticated agent")

        try:
            result = self._make_request(
                method="DELETE",
                endpoint=f"/api/v1/agents/{agent_id}"
            )
            return result or {"success": True, "message": "Agent deleted successfully"}

        except (AuthenticationError, VerificationError):
            raise
        except Exception as e:
            raise VerificationError(f"Failed to delete agent: {e}")

    def perform_action(
        self,
        capability: str = None,
        resource: Optional[str] = None,
        context: Optional[Dict[str, Any]] = None,
        timeout_seconds: int = 300,
        jit_access: bool = False,
        risk_level: str = None,
        auto_register: bool = True
    ):
        """
        Decorator for automatic capability verification and action tracking.

        This decorator wraps a function to automatically:
        1. Auto-register the capability if not already registered (configurable)
        2. Request capability verification from AIM before execution
        3. Wait for approval (if jit_access=True, waits for admin approval)
        4. Execute the function if approved
        5. Log the result back to AIM

        Args:
            capability: Capability being used (e.g., "file:read", "db:query").
                       If not provided, uses the function name.
            resource: Resource being accessed (optional)
            context: Additional context (optional)
            timeout_seconds: Max time to wait for approval (default: 5 minutes)
            jit_access: If True, requires real-time admin approval for sensitive operations
            risk_level: Risk level ("low", "medium", "high", "critical"). If not provided,
                       auto-detects based on capability name.
            auto_register: If True (default), automatically registers capability on first use

        Example - Simple usage (uses function name as capability):
            @agent.perform_action()  # auto-detects risk level
            def search_flights(destination):
                return api.search(destination)

        Example - Explicit capability:
            @agent.perform_action(capability="db:read", resource="users_table")
            def get_users():
                return database.query("SELECT * FROM users")

        Example - JIT Access (waits for admin approval):
            @agent.perform_action(capability="payment:refund", jit_access=True, risk_level="critical")
            def process_refund(order_id: str, amount: float):
                '''Waits for AIM approval before executing'''
                return stripe.refund(order_id, amount)

        Risk Levels:
            - "low": Read operations, safe actions (auto-approved)
            - "medium": Write operations, data modification
            - "high": Sensitive operations (may require approval)
            - "critical": Destructive operations (requires approval)

        Raises:
            ActionDeniedError: If AIM denies the capability
            VerificationError: If verification fails or times out
        """
        def decorator(func: Callable) -> Callable:
            # Use function name as capability if not provided
            cap = capability or func.__name__

            # Auto-detect risk level if not explicitly provided
            detected_risk = risk_level if risk_level else detect_risk_level(cap)

            @functools.wraps(func)
            def wrapper(*args, **kwargs):
                # Warn if using deprecated capability format (snake_case instead of namespace:action)
                _warn_deprecated_capability_format(cap)

                # Auto-register capability on first use (if enabled)
                if auto_register:
                    try:
                        self.register_capability(
                            capability_type=cap,
                            description=f"Auto-registered by @perform_action decorator for {func.__name__}",
                            risk_level=detected_risk
                        )
                        console.capability_registered(cap, detected_risk)
                    except Exception as e:
                        # Don't fail the action if registration fails - just log it
                        console.warning(f"Auto-registration of '{cap}' failed: {e}")

                # Build context with JIT access info
                merged_context = context.copy() if context else {}
                if jit_access:
                    merged_context["jit_access"] = True
                    merged_context["risk_level"] = detected_risk
                    merged_context["function"] = func.__name__
                    merged_context["module"] = func.__module__
                    console.jit_waiting(cap, detected_risk, timeout_seconds)

                # Request capability verification
                verification_result = self.verify_capability(
                    capability=cap,
                    resource=resource,
                    context=merged_context,
                    timeout_seconds=timeout_seconds
                )

                verification_id = verification_result["verification_id"]

                # Execute the function
                try:
                    result = func(*args, **kwargs)

                    # Log success
                    self.log_capability_result(
                        verification_id=verification_id,
                        success=True,
                        result_summary=f"Capability '{cap}' executed successfully"
                    )

                    return result

                except Exception as e:
                    # Log failure
                    self.log_capability_result(
                        verification_id=verification_id,
                        success=False,
                        error_message=str(e)
                    )
                    raise

            return wrapper
        return decorator

    def track_action(
        self,
        risk_level: str = "low",
        action_name: Optional[str] = None,
        resource: Optional[str] = None
    ):
        """
        DEPRECATED: Use @agent.perform_action() instead.

        This method is deprecated and will be removed in a future version.
        Use perform_action() which provides the same functionality plus JIT access support.

        Migration:
            # Old (deprecated):
            @agent.track_action(risk_level="medium")
            def my_function(): pass

            # New (recommended):
            @agent.perform_action(capability="my_function", risk_level="medium")
            def my_function(): pass
        """
        import warnings
        warnings.warn(
            "track_action() is deprecated. Use perform_action() instead. "
            "Example: @agent.perform_action(capability='action_name', risk_level='medium')",
            DeprecationWarning,
            stacklevel=2
        )
        def decorator(func: Callable) -> Callable:
            @functools.wraps(func)
            def wrapper(*args, **kwargs):
                # Use function name if action_name not provided
                action = action_name or func.__name__

                # Warn if using deprecated capability format (snake_case instead of namespace:action)
                _warn_deprecated_capability_format(action)

                # Build context with risk level
                context = {
                    "risk_level": risk_level,
                    "function_name": func.__name__,
                    "module": func.__module__
                }

                # Add args/kwargs to context (for audit trail)
                if args:
                    context["args"] = str(args)
                if kwargs:
                    context["kwargs"] = str(kwargs)

                # Request capability verification
                try:
                    verification_result = self.verify_capability(
                        capability=action,
                        resource=resource,
                        context=context,
                        timeout_seconds=300
                    )
                except Exception as e:
                    # Handle any exceptions during verification
                    console.warning(f"Verification request failed: {type(e).__name__}: {str(e)}")
                    console.info(f"Capability '{action}' cannot proceed without verification")
                    return {
                        "error": True,
                        "error_type": type(e).__name__,
                        "error_message": str(e),
                        "capability": action,
                        "status": "verification_failed"
                    }

                # Check if verification result has an error
                if verification_result.get("error"):
                    error_msg = verification_result.get("error", "Unknown verification error")
                    console.warning(f"Verification returned error: {error_msg}")
                    console.info(f"Capability '{action}' cannot proceed without successful verification")
                    return {
                        "error": True,
                        "error_type": "VerificationError",
                        "error_message": error_msg,
                        "capability": action,
                        "status": "verification_failed"
                    }

                if not verification_result.get("verified", False):
                    reason = verification_result.get("reason", verification_result.get("error", "Unknown reason"))
                    console.warning(f"Capability '{action}' not verified: {reason}")
                    return {
                        "error": True,
                        "error_type": "CapabilityDenied",
                        "error_message": f"Capability '{action}' denied: {reason}",
                        "capability": action,
                        "status": "denied"
                    }

                verification_id = verification_result.get("verification_id")

                try:
                    # Execute the function
                    result = func(*args, **kwargs)

                    # Log success (handle errors in logging gracefully)
                    try:
                        self.log_capability_result(
                            verification_id=verification_id,
                            success=True,
                            result_summary=f"Capability '{action}' executed successfully"
                        )
                    except Exception as log_error:
                        # Don't fail the function if logging fails
                        print(f" Warning: Failed to log capability result: {str(log_error)}")

                    return result

                except Exception as e:
                    # Log failure (handle errors in logging gracefully)
                    try:
                        self.log_capability_result(
                            verification_id=verification_id,
                            success=False,
                            error_message=str(e)
                        )
                    except Exception as log_error:
                        # Don't fail if logging fails
                        print(f"  Warning: Failed to log capability failure: {str(log_error)}")

                    # Return error result instead of raising
                    print(f" Error executing capability '{action}': {type(e).__name__}: {str(e)}")
                    return {
                        "error": True,
                        "error_type": type(e).__name__,
                        "error_message": str(e),
                        "capability": action,
                        "status": "execution_failed"
                    }

            return wrapper
        return decorator

    def require_approval(
        self,
        risk_level: str = "high",
        action_name: Optional[str] = None,
        resource: Optional[str] = None,
        timeout_seconds: int = 3600
    ):
        """
        Decorator for actions requiring human approval before execution.

        This decorator pauses execution until an admin approves the action
        in the AIM dashboard. Use for high-risk or destructive operations.

        Args:
            risk_level: Risk level ("high" or "critical")
            action_name: Custom action name (default: function name)
            resource: Resource being accessed (optional)
            timeout_seconds: Max time to wait for approval (default: 1 hour)

        Example:
            @agent.require_approval(risk_level="critical")
            def delete_all_users():
                database.execute("DELETE FROM users")

            # Execution PAUSES here
            # Admin receives alert: "Agent wants to delete all users"
            # Function only executes if admin approves
            delete_all_users()

        Risk Levels:
            - "high": Sensitive operations requiring review
            - "critical": Destructive operations requiring urgent approval
        """
        # Validate risk level
        if risk_level not in ["high", "critical"]:
            raise ValueError(
                f"require_approval() only supports 'high' or 'critical' risk levels, "
                f"got: {risk_level}. Use perform_action() for lower risk levels."
            )

        def decorator(func: Callable) -> Callable:
            @functools.wraps(func)
            def wrapper(*args, **kwargs):
                # Use function name if action_name not provided
                action = action_name or func.__name__

                # Build context with risk level and approval requirement
                context = {
                    "risk_level": risk_level,
                    "requires_approval": True,
                    "function_name": func.__name__,
                    "module": func.__module__,
                    "warning": f" CRITICAL: This action requires human approval!"
                }

                # Add args/kwargs to context
                if args:
                    context["args"] = str(args)
                if kwargs:
                    context["kwargs"] = str(kwargs)

                print(f"\n⏸️  Waiting for approval: {action}...")
                print(f"   Risk Level: {risk_level.upper()}")
                print(f"   Check AIM dashboard to approve/deny this capability")
                print(f"   Timeout: {timeout_seconds} seconds")

                # Request capability verification with extended timeout
                try:
                    verification_result = self.verify_capability(
                        capability=action,
                        resource=resource,
                        context=context,
                        timeout_seconds=timeout_seconds
                    )
                except Exception as e:
                    # Handle any exceptions during verification
                    print(f" Warning: Verification request failed: {type(e).__name__}: {str(e)}")
                    print(f"   Capability '{action}' cannot proceed without verification.")
                    print(f"   Returning error result instead of raising exception.")
                    return {
                        "error": True,
                        "error_type": type(e).__name__,
                        "error_message": str(e),
                        "capability": action,
                        "status": "verification_failed"
                    }

                # Check if verification result has an error
                if verification_result.get("error"):
                    error_msg = verification_result.get("error", "Unknown verification error")
                    print(f" Warning: Verification returned error: {error_msg}")
                    print(f"   Capability '{action}' cannot proceed without successful verification.")
                    return {
                        "error": True,
                        "error_type": "VerificationError",
                        "error_message": error_msg,
                        "capability": action,
                        "status": "verification_failed"
                    }

                if not verification_result.get("verified", False):
                    reason = verification_result.get("reason", verification_result.get("error", "Unknown reason"))
                    print(f" Capability '{action}' DENIED or not verified: {reason}")
                    return {
                        "error": True,
                        "error_type": "CapabilityDenied",
                        "error_message": f"Capability '{action}' DENIED: {reason}",
                        "capability": action,
                        "status": "denied"
                    }

                print(f"✅ Capability '{action}' APPROVED by admin")
                verification_id = verification_result.get("verification_id")

                try:
                    # Execute the function
                    result = func(*args, **kwargs)

                    # Log success (handle errors in logging gracefully)
                    try:
                        self.log_capability_result(
                            verification_id=verification_id,
                            success=True,
                            result_summary=f"Capability '{action}' executed successfully"
                        )
                    except Exception as log_error:
                        # Don't fail the function if logging fails
                        print(f"  Warning: Failed to log capability result: {str(log_error)}")

                    return result

                except Exception as e:
                    # Log failure (handle errors in logging gracefully)
                    try:
                        self.log_capability_result(
                            verification_id=verification_id,
                            success=False,
                            error_message=str(e)
                        )
                    except Exception as log_error:
                        # Don't fail if logging fails
                        print(f"  Warning: Failed to log capability failure: {str(log_error)}")

                    # Return error result instead of raising
                    print(f" Error executing capability '{action}': {type(e).__name__}: {str(e)}")
                    return {
                        "error": True,
                        "error_type": type(e).__name__,
                        "error_message": str(e),
                        "capability": action,
                        "status": "execution_failed"
                    }

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
# ONE-LINE AGENT REGISTRATION - Enterprise security simplified
# ============================================================================

import os
import pathlib

# Import centralized credential management
from .credentials import (
    get_agent_credentials_path,
    load_agent_credentials as _load_agent_creds_from_module,
    save_agent_credentials as _save_agent_creds_to_module,
    list_agent_credentials,
    AGENTS_DIR,
    LEGACY_CREDENTIALS_FILE,
)


def _get_credentials_path():
    """
    Get path to credentials file.

    DEPRECATED: This function is kept for backward compatibility.
    Agent credentials are now stored per-agent in ~/.aim/agents/{agent-name}.json

    Returns:
        Legacy credentials path for migration purposes
    """
    return LEGACY_CREDENTIALS_FILE


def _update_agent_capabilities(aim_url: str, headers: Dict[str, str], agent_id: str, capabilities: List[str]):
    """
    Update an existing agent's capabilities.

    Behavior depends on organization's enforcement mode:
    - MONITORING MODE: New capabilities are auto-granted (permissive)
    - STRICT MODE: New capabilities are rejected, must use request_capability()

    This function will:
    - Identify new capabilities
    - In strict mode: warn that escalations are rejected
    - In monitoring mode: capabilities are auto-granted by the backend
    - Capability reductions are always allowed (safe direction)

    Args:
        aim_url: AIM server URL
        headers: Request headers with auth token
        agent_id: Agent ID
        capabilities: List of capability strings requested
    """
    try:
        # Get existing capabilities first
        get_url = f"{aim_url.rstrip('/')}/api/v1/agents/{agent_id}/capabilities"
        get_response = requests.get(get_url, headers=headers, timeout=30)

        existing_caps = set()
        if get_response.status_code == 200:
            caps_data = get_response.json()
            # Handle both formats: {"capabilities": [...]} or just [...]
            if isinstance(caps_data, list):
                caps_list = caps_data
            else:
                caps_list = caps_data.get("capabilities", [])
            existing_caps = {cap.get("action") for cap in caps_list if isinstance(cap, dict)}

        # Identify capability changes
        requested_caps = set(capabilities)
        new_caps = requested_caps - existing_caps  # New capabilities
        removed_caps = existing_caps - requested_caps  # Reductions (safe)

        # Inform about new capabilities
        # Note: Backend handles enforcement mode - in monitoring mode these will be auto-granted,
        # in strict mode they will be rejected
        if new_caps:
            console.info(
                f"New capabilities requested: {list(new_caps)}\n"
                "   In MONITORING mode: auto-granted by backend\n"
                "   In STRICT mode: rejected - use agent.request_capability() for admin approval"
            )

        # Capability reductions are allowed (safe direction)
        if removed_caps:
            console.info(f"Capabilities to be removed: {list(removed_caps)}")

        # Report what capabilities are currently active
        if existing_caps:
            console.info(f"Current capabilities: {list(existing_caps)}")

    except Exception as e:
        console.warning(f"Failed to check capabilities: {e}")


def _save_credentials(agent_name: str, credentials: Dict[str, Any]):
    """
    Save agent credentials to per-agent file.

    Credentials are stored in ~/.aim/agents/{agent-name}.json
    This ensures clean separation from SDK credentials and allows
    multiple agents to have their own credential files.

    Args:
        agent_name: Name of the agent
        credentials: Credentials dict from registration response
    """
    # Prepare the agent credential data
    agent_data = {
        "agent_id": credentials["agent_id"],
        "public_key": credentials["public_key"],
        "private_key": credentials["private_key"],
        "aim_url": credentials["aim_url"],
        "status": credentials.get("status", "unknown"),
        "trust_score": credentials.get("trust_score"),
        "registered_at": datetime.now(timezone.utc).isoformat()
    }

    # Use centralized credentials module for storage
    _save_agent_creds_to_module(agent_name, agent_data)


def _load_credentials(agent_name: str) -> Optional[Dict[str, Any]]:
    """
    Load agent credentials from per-agent file.

    Credentials are stored in ~/.aim/agents/{agent-name}.json
    This function also handles migration from legacy format
    where all agents were stored in a single credentials.json file.

    Args:
        agent_name: Name of the agent

    Returns:
        Credentials dict if found, None otherwise
    """
    # Use centralized credentials module (handles migration automatically)
    return _load_agent_creds_from_module(agent_name)


def register_agent(
    name: str,
    aim_url: Optional[str] = None,
    api_key: Optional[str] = None,
    display_name: Optional[str] = None,
    description: Optional[str] = None,
    agent_type: str = "ai_agent",
    version: str = "1.0.0",  # Default version for new agents
    repository_url: Optional[str] = None,
    documentation_url: Optional[str] = None,
    organization_domain: Optional[str] = None,
    mcp_servers: Optional[list] = None,  # Explicit MCP servers to register
    capabilities: Optional[list] = None,
    tags: Optional[list] = None,  # Tag IDs to apply during registration
    metadata: Optional[dict] = None,  # Custom metadata (model, department, etc.)
    auto_detect: bool = True,
    force_new: bool = False,
    sdk_token_id: Optional[str] = None,
    # Backward compatibility alias
    talks_to: Optional[list] = None,  # DEPRECATED: Use mcp_servers instead
) -> AIMClient:
    """
    ONE-LINE agent registration with AIM - Radical simplicity meets enterprise security

    This is the magic function that makes AIM effortless to use while maintaining production-ready protection.
    Call this once and your agent is registered, verified, and ready to use.

    Agent type is automatically detected from your imports (LangChain, CrewAI, Anthropic, OpenAI, etc.)

    **ZERO CONFIG MODE** (SDK Download):
        from aim_sdk import secure
        agent = secure("my-agent")
        # That's it! Agent type + capabilities auto-detected from imports.

    **MANUAL MODE** (pip install):
        from aim_sdk import secure
        agent = secure("my-agent", api_key="aim_abc123")
        # Still auto-detects agent type, capabilities + MCPs

    Args:
        name: Agent name (unique identifier)
        aim_url: AIM server URL (auto-detected from SDK credentials if available)
        api_key: AIM API key (only required if no SDK credentials found)
        display_name: Human-readable display name (defaults to name)
        description: Agent description (defaults to auto-generated)
        agent_type: Type of agent. Use AgentType constants for type safety.
            Valid types include:
            - LLM Providers: "claude", "gpt", "gemini", "llama", "mistral", "cohere"
            - Frameworks: "langchain", "llamaindex", "autogen", "crewai",
              "langgraph", "haystack", "semantic_kernel"
            - Copilots: "copilot", "assistant", "chatbot"
            - Autonomous: "autogpt", "babyagi"
            - Generic: "custom", "ai_agent" (legacy)
            Default: "ai_agent"
        version: Agent version (default: "1.0.0")
        repository_url: GitHub/GitLab repository URL
        documentation_url: Documentation URL
        organization_domain: Organization domain for auto-approval
        mcp_servers: MCP servers this agent communicates with. Supports two formats:
            - Simple names: ["github", "filesystem"] - for quick reference only
            - Full definitions: [{"name": "github", "url": "...", "version": "1.0.0", ...}]
              Full definition fields:
              - name (required): MCP server name
              - url (optional): MCP server endpoint URL
              - version (optional): Server version
              - description (optional): Human-readable description
              - tags (optional): List of tags for categorization
        capabilities: Override auto-detected capabilities (manual specification).
            Note: On re-registration, new capabilities are REJECTED to prevent
            privilege escalation. Use agent.request_capability() for new capabilities.
        tags: List of tags for categorization (e.g., ["production", "customer-facing"])
        metadata: Custom metadata dict (e.g., {"model": "gpt-4", "department": "support"})
        auto_detect: Auto-detect capabilities and MCPs (default: True)
        force_new: Force new registration even if credentials exist
        sdk_token_id: SDK token for usage tracking (auto-loaded if available)
        talks_to: DEPRECATED - Use mcp_servers instead (kept for backward compatibility)

    Security:
        Capability changes on re-registration depend on organization enforcement mode:

        STRICT MODE (default):
            - New capabilities are REJECTED to prevent privilege escalation
            - To add new capabilities:
              1. Use agent.request_capability("new:capability", "reason")
              2. Admin approves in dashboard
              3. Capability is granted

        MONITORING MODE:
            - New capabilities are auto-granted (permissive for development/testing)
            - All capability changes are logged for audit

        In both modes:
            - Capability reduction is always allowed (removing capabilities is safe)
            - All changes are logged for security audit

    Returns:
        AIMClient instance ready to use

    Examples:
        # SDK Download Mode (ZERO CONFIG) - agent type auto-detected from imports:
        >>> from aim_sdk import secure
        >>> agent = secure("my-agent")

        # With explicit agent type:
        >>> from aim_sdk import secure, AgentType
        >>> agent = secure("my-agent", agent_type=AgentType.LANGCHAIN)

        # Manual Install Mode:
        >>> from aim_sdk import secure
        >>> agent = secure("my-agent", api_key="aim_abc123")

        # Power User Mode (disable auto-detection):
        >>> from aim_sdk import secure, AgentType
        >>> agent = secure(
        ...     "my-agent",
        ...     api_key="aim_abc123",
        ...     agent_type=AgentType.CREWAI,
        ...     auto_detect=False,
        ...     capabilities=["db:read", "api:call"],
        ...     mcp_servers=["custom-mcp-server"]
        ... )

    Raises:
        ConfigurationError: If registration fails or required credentials missing
        AuthenticationError: If authentication fails
    """
    # 1. Check for existing credentials (unless force_new)
    if not force_new:
        existing_creds = _load_credentials(name)
        if existing_creds:
            # Log agent loaded from cache
            security_logger.log_agent_event(
                AgentEventType.AGENT_LOADED,
                agent_id=existing_creds['agent_id'],
                agent_name=name,
                details={
                    "source": "cached_credentials",
                    "status": existing_creds.get('status', 'active'),
                    "trust_score": existing_creds.get('trust_score')
                }
            )

            # Use beautiful console output
            console.agent_found(
                name=name,
                agent_id=existing_creds['agent_id'],
                status=existing_creds.get('status', 'active'),
                trust_score=existing_creds.get('trust_score', 0.85)
            )

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

    # Force API key mode if sdk_token_id is explicitly set to None
    if sdk_token_id is None and api_key:
        # FORCE API KEY MODE: Skip SDK credentials check
        auth_mode = "api_key"
        if not aim_url:
            raise ConfigurationError("aim_url is required when using API key mode")
        console.info("API Key Mode: Using API key authentication")
    elif sdk_creds:
        # SDK MODE: Use embedded OAuth credentials
        auth_mode = "oauth"
        # Support both camelCase and snake_case for backward compatibility
        aim_url = aim_url or sdk_creds.get("aimUrl") or sdk_creds.get("aim_url")
        sdk_token_id = sdk_token_id or sdk_creds.get("sdkTokenId") or sdk_creds.get("sdk_token_id")

        if not aim_url:
            raise ConfigurationError("aim_url not found in SDK credentials")

      

    elif api_key:
        # MANUAL MODE: Use API key
        auth_mode = "api_key"

        if not aim_url:
            raise ConfigurationError("aim_url is required when using API key mode")

        console.info("Manual Mode: Using API key authentication")

    else:
        # No authentication found
        raise ConfigurationError(
            "No authentication credentials found.\n"
            "Either download SDK from dashboard (OAuth mode) or provide api_key parameter (Manual mode)."
        )

    # 2.5. Handle backward compatibility: merge talks_to into mcp_servers
    if talks_to is not None:
        import warnings
        warnings.warn(
            "The 'talks_to' parameter is deprecated. Use 'mcp_servers' instead.",
            DeprecationWarning,
            stacklevel=2
        )
        if mcp_servers is None:
            mcp_servers = talks_to
        else:
            # Merge both, preferring mcp_servers but adding any unique from talks_to
            mcp_servers = list(set(mcp_servers + talks_to))

    # 3. Auto-detect capabilities, MCPs, and agent type (unless manually specified)
    if auto_detect:
        # Auto-detect agent type (unless manually provided)
        # Only auto-detect if agent_type is at the default value "ai_agent"
        if agent_type == "ai_agent":
            from .capability_detection import auto_detect_agent_type

            detected_type, detection_reason = auto_detect_agent_type()
            if detected_type != "custom":
                agent_type = detected_type
                console.detection_result("Agent Type", agent_type, detection_reason)
            else:
                # Keep ai_agent as default when nothing specific is detected
                console.detection_none("Agent Type", fallback="ai_agent")

        # Auto-detect capabilities (unless manually provided)
        if not capabilities:
            from .detection import auto_detect_mcps

            detected_caps = auto_detect_capabilities()
            if detected_caps:
                capabilities = detected_caps
                cap_display = ', '.join(capabilities[:5])
                if len(capabilities) > 5:
                    cap_display += f" (+{len(capabilities) - 5} more)"
                console.detection_result("Capabilities", cap_display, f"{len(capabilities)} detected")
            else:
                console.detection_none("Capabilities")

        # Auto-detect MCP servers (unless manually provided)
        if not mcp_servers:
            from .detection import auto_detect_mcps

            mcp_detections = auto_detect_mcps()
            if mcp_detections:
                mcp_servers = [d["mcpServer"] for d in mcp_detections]
                mcp_display = ', '.join(mcp_servers[:3])
                if len(mcp_servers) > 3:
                    mcp_display += f" (+{len(mcp_servers) - 3} more)"
                console.detection_result("MCP Servers", mcp_display, f"{len(mcp_servers)} detected")
            else:
                console.detection_none("MCP Servers")

    # 4. Prepare registration request
    registration_data = {
        "name": name,
        "displayName": display_name or name,
        "description": description or f"Agent {name} registered via AIM SDK",
        "agentType": agent_type,
        "version": version  # Always include version (defaults to "1.0.0")
    }

    if metadata:
        registration_data["metadata"] = metadata
    if repository_url:
        registration_data["repositoryUrl"] = repository_url
    if documentation_url:
        registration_data["documentationUrl"] = documentation_url
    if organization_domain:
        registration_data["organizationDomain"] = organization_domain

    # Process MCP servers: extract names for talksTo, preserve full definitions for later
    mcp_server_names = []
    mcp_full_definitions = []  # Store full definitions for post-registration MCP creation
    if mcp_servers:
        for mcp in mcp_servers:
            if isinstance(mcp, str):
                # Simple string name
                mcp_server_names.append(mcp)
            elif isinstance(mcp, dict):
                # Full definition with name, url, version, etc.
                mcp_name = mcp.get("name")
                if mcp_name:
                    mcp_server_names.append(mcp_name)
                    mcp_full_definitions.append(mcp)
                else:
                    console.warning(f"MCP server definition missing 'name' field: {mcp}")
        if mcp_server_names:
            registration_data["talksTo"] = mcp_server_names  # Backend expects list of names

    if capabilities:
        registration_data["capabilities"] = capabilities
    if tags:
        registration_data["tagIds"] = tags  # ✅ Tags applied during registration

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
                mcp_server_names=mcp_server_names,
                mcp_full_definitions=mcp_full_definitions
            )
        else:
            # API Key Mode: Use public endpoint with API key header
            return _register_via_api_key(
                name=name,
                aim_url=aim_url,
                api_key=api_key,
                registration_data=registration_data,
                sdk_token_id=sdk_token_id,
                mcp_server_names=mcp_server_names,
                mcp_full_definitions=mcp_full_definitions
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
    mcp_server_names: List[str],
    mcp_full_definitions: List[Dict[str, Any]]
) -> AIMClient:
    """Register agent using OAuth token from SDK credentials"""
    # Generate Ed25519 keypair client-side (for OAuth mode)
    
    signing_key = SigningKey.generate()
    private_key_bytes = bytes(signing_key) + bytes(signing_key.verify_key)  # 64 bytes (seed + public)
    public_key_bytes = bytes(signing_key.verify_key)

    private_key_b64 = base64.b64encode(private_key_bytes).decode('utf-8')
    public_key_b64 = base64.b64encode(public_key_bytes).decode('utf-8')

    # Add public key to registration data (use camelCase)
    registration_data["publicKey"] = public_key_b64


    # Initialize OAuth token manager - let it discover credentials automatically
    # OAuthTokenManager._discover_credentials_path() checks:
    # 1. Home directory (~/.aim/credentials.json)
    # 2. SDK package directory (for downloaded SDKs with embedded credentials)
    # If SDK credentials exist, they're auto-copied to home directory
    token_manager = OAuthTokenManager()  # Auto-discover credentials
    access_token = token_manager.get_access_token()

    if not access_token:
        raise ConfigurationError("Failed to obtain OAuth access token")

    # Set up headers for API calls
    headers = {
        "Content-Type": "application/json",
        "Authorization": f"Bearer {access_token}"
    }

    if sdk_token_id:
        headers["X-SDK-Token"] = sdk_token_id

    # FIRST: Check if agent already exists (get or create pattern)
    # Use sdk-api endpoint which accepts both ID and name
    check_url = f"{aim_url.rstrip('/')}/api/v1/sdk-api/agents/{name}"
    check_response = requests.get(check_url, headers=headers, timeout=30)

    if check_response.status_code == 200:
        # Agent exists - connect to it
        console.info(f"Agent '{name}' already exists, connecting...")
        response_data = check_response.json()
        existing_agent = response_data.get("agent", response_data)

        # Load saved credentials
        saved_creds = _load_credentials(name)
        if saved_creds and saved_creds.get("private_key"):
            # Use saved credentials
            credentials = saved_creds
            credentials["agent_id"] = existing_agent.get("id")
            credentials["aim_url"] = aim_url

            # Update capabilities if new ones provided
            if registration_data.get("capabilities"):
                _update_agent_capabilities(aim_url, headers, existing_agent["id"], registration_data["capabilities"])

            client = AIMClient(
                agent_id=credentials["agent_id"],
                public_key=credentials["public_key"],
                private_key=credentials["private_key"],
                aim_url=credentials["aim_url"],
                oauth_token_manager=token_manager
            )
            console.success(f"Connected to existing agent: {name}")
            return client
        else:
            # No saved credentials - regenerate new key pair and update the agent
            console.info(f"Regenerating credentials for existing agent '{name}'...")

            # Generate new key pair
            signing_key = SigningKey.generate()
            verify_key = signing_key.verify_key
            private_key = signing_key.encode(encoder=Base64Encoder).decode('utf-8')
            public_key = verify_key.encode(encoder=Base64Encoder).decode('utf-8')

            # Update agent's public key on the server via PUT /keys endpoint
            update_url = f"{aim_url.rstrip('/')}/api/v1/agents/{existing_agent['id']}/keys"
            update_response = requests.put(
                update_url,
                json={"publicKey": public_key},
                headers=headers,
                timeout=30
            )

            if update_response.status_code not in [200, 201]:
                raise ConfigurationError(
                    f"Agent '{name}' exists but credentials could not be regenerated. "
                    "Delete the agent from the dashboard and try again, or use force_new=True."
                )

            # Build credentials
            credentials = {
                "agent_id": existing_agent["id"],
                "public_key": public_key,
                "private_key": private_key,
                "aim_url": aim_url,
                "status": existing_agent.get("status", "active"),
                "trust_score": existing_agent.get("trustScore")
            }

            # Save credentials locally
            _save_credentials(name, credentials)

            # Update capabilities if new ones provided
            if registration_data.get("capabilities"):
                _update_agent_capabilities(aim_url, headers, existing_agent["id"], registration_data["capabilities"])

            client = AIMClient(
                agent_id=credentials["agent_id"],
                public_key=credentials["public_key"],
                private_key=credentials["private_key"],
                aim_url=credentials["aim_url"],
                oauth_token_manager=token_manager
            )
            console.success(f"Reconnected to existing agent: {name}")
            return client

    # Agent doesn't exist - create it
    url = f"{aim_url.rstrip('/')}/api/v1/agents"
    response = requests.post(
        url,
        json=registration_data,
        headers=headers,
        timeout=30
    )

    if response.status_code not in [200, 201]:
        error_msg = response.json().get("error", "Unknown error")
        security_logger.log_agent_event(
            AgentEventType.AGENT_REGISTRATION_FAILED,
            agent_name=name,
            error=error_msg,
            details={
                "registration_mode": "oauth",
                "http_status": response.status_code
            }
        )
        raise ConfigurationError(f"Registration failed: {error_msg}")

    credentials = response.json()

    # Backend returns 'id' but we need 'agent_id' for consistency
    if "id" in credentials and "agent_id" not in credentials:
        credentials["agent_id"] = credentials["id"]

    # Add client-side generated private key to credentials (backend doesn't send it back)
    credentials["private_key"] = private_key_b64
    
    # CRITICAL: Use the backend's public key (what's in database), not what we generated
    # The backend's public key is the source of truth since it's stored in the database
    backend_pub_key = credentials.get('public_key') or credentials.get('publicKey')
    if backend_pub_key:
        credentials["public_key"] = backend_pub_key
    else:
        # Fallback: use generated key if backend didn't return one
        credentials["public_key"] = public_key_b64
    
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

    # Register MCP servers in background (non-blocking for instant startup)
    if mcp_full_definitions or mcp_server_names:
        _start_background_mcp_registration(
            client, aim_url, headers, mcp_server_names, mcp_full_definitions, credentials["agent_id"]
        )

    # Log agent registration success
    security_logger.log_agent_event(
        AgentEventType.AGENT_REGISTERED,
        agent_id=credentials["agent_id"],
        agent_name=name,
        details={
            "registration_mode": "oauth",
            "agent_type": registration_data.get("agentType"),
            "capabilities_count": len(registration_data.get("capabilities", [])),
            "mcp_servers_count": len(mcp_server_names)
        }
    )

    # Show beautiful registration success message
    capabilities = registration_data.get("capabilities")
    _print_registration_success(name, credentials, capabilities, mcp_server_names)
    return client


def _register_via_api_key(
    name: str,
    aim_url: str,
    api_key: str,
    registration_data: Dict[str, Any],
    sdk_token_id: Optional[str],
    mcp_server_names: List[str],
    mcp_full_definitions: List[Dict[str, Any]]
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
        security_logger.log_agent_event(
            AgentEventType.AGENT_REGISTRATION_FAILED,
            agent_name=name,
            error=error_msg,
            details={
                "registration_mode": "api_key",
                "http_status": response.status_code
            }
        )
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

    # Register MCP servers in background (non-blocking for instant startup)
    if mcp_full_definitions or mcp_server_names:
        _start_background_mcp_registration(
            client, aim_url, headers, mcp_server_names, mcp_full_definitions, credentials["agent_id"]
        )

    # Log agent registration success
    security_logger.log_agent_event(
        AgentEventType.AGENT_REGISTERED,
        agent_id=credentials["agent_id"],
        agent_name=name,
        details={
            "registration_mode": "api_key",
            "agent_type": registration_data.get("agentType"),
            "capabilities_count": len(registration_data.get("capabilities", [])),
            "mcp_servers_count": len(mcp_server_names)
        }
    )

    # Show beautiful registration success message
    _print_registration_success(name, credentials, registration_data.get("capabilities"), mcp_server_names)
    return client


def _start_background_mcp_registration(
    client: AIMClient,
    aim_url: str,
    headers: Dict[str, str],
    mcp_server_names: List[str],
    mcp_full_definitions: List[Dict[str, Any]],
    agent_id: str
) -> None:
    """
    Start MCP registration in a background thread (non-blocking).

    Industry pattern: Agent registers immediately, MCP discovery happens async.
    This ensures fast startup regardless of how many MCP servers exist.
    """
    import threading

    def _background_task():
        try:
            all_mcp_definitions = list(mcp_full_definitions)

            # Convert simple string names to definitions (skip capability discovery for speed)
            for mcp_name in mcp_server_names:
                if any(d.get("name") == mcp_name for d in mcp_full_definitions):
                    continue
                simple_def = {
                    "name": mcp_name,
                    "description": f"MCP server '{mcp_name}' registered via SDK",
                    "url": "",
                    "version": "1.0.0",
                    "capabilities": []  # Capabilities discovered on-demand later
                }
                all_mcp_definitions.append(simple_def)

            # Register MCP servers (parallel, with short timeout)
            if all_mcp_definitions:
                _register_mcp_servers(client, aim_url, headers, all_mcp_definitions, agent_id)

            # Report detections for tracking
            if mcp_server_names:
                import datetime
                manual_detections = [
                    {
                        "mcpServer": mcp,
                        "detectionMethod": "manual",
                        "confidence": 100.0,
                        "sdkVersion": f"aim-sdk-python@{__version__}",
                        "timestamp": datetime.datetime.utcnow().isoformat() + "Z",
                        "details": {"source": "sdk_registration", "explicit": True}
                    }
                    for mcp in mcp_server_names
                ]
                try:
                    client.report_detections(manual_detections)
                except Exception:
                    pass
        except Exception:
            pass  # Never fail agent if MCP registration fails

    # Start background thread (daemon=True so it doesn't block program exit)
    thread = threading.Thread(target=_background_task, daemon=True)
    thread.start()

    # Store reference for optional waiting
    client._mcp_registration_thread = thread


def _register_single_mcp(
    client: AIMClient,
    aim_url: str,
    headers: Dict[str, str],
    mcp_def: Dict[str, Any],
    agent_id: str,
    detected_capabilities: Dict[str, List[str]]
) -> Dict[str, Any]:
    """
    Register a single MCP server (called in parallel).

    Returns:
        Dict with 'name', 'success', 'status' ('registered', 'exists', 'failed'), 'error'
    """
    mcp_name = mcp_def.get("name", "")
    result = {"name": mcp_name, "success": False, "status": "failed", "error": None}

    try:
        mcp_data = {
            "name": mcp_name,
            "description": mcp_def.get("description", f"MCP server registered via SDK"),
            "url": mcp_def.get("url", ""),
            "version": mcp_def.get("version", "1.0.0"),
            "capabilities": mcp_def.get("capabilities", []),
            "registeredByAgent": agent_id,
            "verificationMethod": "agent_attestation"
        }

        # Call the MCP servers API with shorter timeout (10s instead of 30s)
        url = f"{aim_url.rstrip('/')}/api/v1/mcp-servers"
        response = requests.post(url, json=mcp_data, headers=headers, timeout=10)

        mcp_server_id = None
        if response.status_code in [200, 201]:
            result["success"] = True
            result["status"] = "registered"
            try:
                response_data = response.json()
                mcp_server_id = response_data.get("id")
            except Exception:
                pass
        elif response.status_code == 409:
            result["success"] = True
            result["status"] = "exists"
            # Fetch existing MCP server ID so we can still attest
            try:
                response_data = response.json()
                mcp_server_id = response_data.get("id") or response_data.get("existingId")
                # If not in response, try to fetch by name
                if not mcp_server_id:
                    list_url = f"{aim_url.rstrip('/')}/api/v1/mcp-servers?search={mcp_name}"
                    list_resp = requests.get(list_url, headers=headers, timeout=5)
                    if list_resp.status_code == 200:
                        servers = list_resp.json().get("servers", [])
                        for srv in servers:
                            if srv.get("name") == mcp_name:
                                mcp_server_id = srv.get("id")
                                break
            except Exception:
                pass
        else:
            result["error"] = f"HTTP {response.status_code}"
            return result

        # Auto-attest to the MCP server with detected capabilities
        if mcp_server_id and client.signing_key:
            try:
                mcp_capabilities = detected_capabilities.get(mcp_name, [])
                if mcp_def.get("capabilities"):
                    mcp_capabilities = mcp_def.get("capabilities")

                client.attest_mcp(
                    mcp_server_id=mcp_server_id,
                    capabilities=mcp_capabilities,
                    connection_successful=True,
                    health_check_passed=True,
                    mcp_url=mcp_def.get("url", ""),
                    mcp_name=mcp_name
                )
                result["attested"] = True
                result["capabilities_count"] = len(mcp_capabilities)
            except Exception as e:
                result["attest_error"] = str(e)

    except requests.Timeout:
        result["error"] = "timeout"
    except Exception as e:
        result["error"] = str(e)

    return result


def _register_mcp_servers(
    client: AIMClient,
    aim_url: str,
    headers: Dict[str, str],
    mcp_definitions: List[Dict[str, Any]],
    agent_id: str
) -> None:
    """
    Register MCP server definitions in parallel for fast registration.

    Uses ThreadPoolExecutor to register multiple MCPs concurrently,
    dramatically reducing registration time for agents with many MCP servers.

    Args:
        client: AIMClient instance for API calls
        aim_url: Base URL of the AIM server
        headers: HTTP headers with authentication
        mcp_definitions: List of MCP server definitions with name, url, version, etc.
        agent_id: ID of the agent registering these servers
    """
    from concurrent.futures import ThreadPoolExecutor, as_completed

    if not mcp_definitions:
        return

    # First, try to detect MCP capabilities from Claude config
    detected_capabilities = _detect_mcp_capabilities_from_config()

    # Use parallel registration for performance (max 10 concurrent)
    max_workers = min(10, len(mcp_definitions))

    console.info(f"Registering {len(mcp_definitions)} MCP servers...")

    results = []
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        # Submit all registration tasks
        futures = {
            executor.submit(
                _register_single_mcp,
                client, aim_url, headers, mcp_def, agent_id, detected_capabilities
            ): mcp_def.get("name", "unknown")
            for mcp_def in mcp_definitions
        }

        # Collect results as they complete
        for future in as_completed(futures):
            mcp_name = futures[future]
            try:
                result = future.result(timeout=15)  # 15s per result max
                results.append(result)
            except Exception as e:
                results.append({"name": mcp_name, "success": False, "error": str(e)})

    # Summarize results
    registered = [r for r in results if r.get("status") == "registered"]
    existing = [r for r in results if r.get("status") == "exists"]
    failed = [r for r in results if not r.get("success")]

    if registered:
        console.success(f"Registered {len(registered)} MCP server(s)")
    if existing:
        console.info(f"{len(existing)} MCP server(s) already exist")
    if failed:
        failed_names = [r.get("name", "?") for r in failed[:3]]  # Show max 3
        if len(failed) > 3:
            console.warning(f"Failed to register {len(failed)} MCP server(s): {', '.join(failed_names)}... and {len(failed)-3} more")
        else:
            console.warning(f"Failed to register: {', '.join(failed_names)}")


def _detect_mcp_capabilities_from_config(
    server_names: Optional[List[str]] = None,
    timeout_per_server: float = 15.0  # 15s per server (runs in parallel)
) -> Dict[str, List[str]]:
    """
    Dynamically discover MCP server capabilities by querying each server.

    This function uses the official MCP protocol to query each server for its
    actual tools, resources, and prompts. Returns empty dict if discovery
    times out - registration will still succeed without capability info.

    Args:
        server_names: Optional list of specific server names to discover.
                      If None, discovers all servers in Claude Desktop config.
        timeout_per_server: Timeout for each server query (seconds)

    Returns:
        Dict mapping MCP server names to their list of capability names
    """
    try:
        from aim_sdk.detection import discover_mcp_capabilities

        return discover_mcp_capabilities(
            server_names=server_names,
            timeout_per_server=timeout_per_server
        )

    except ImportError:
        # MCP SDK not available - silently continue without capabilities
        return {}
    except Exception:
        # Discovery failed - silently continue without capabilities
        return {}


def _print_registration_success(
    name: str,
    credentials: Dict[str, Any],
    capabilities: Optional[List[str]] = None,
    mcp_servers: Optional[List[str]] = None
):
    """Print beautiful success message after registration using AIMConsole."""
    agent_id = credentials.get("agent_id") or credentials.get("id", "")
    agent_type = credentials.get("agentType") or credentials.get("agent_type", "ai_agent")
    version = credentials.get("version", "1.0.0")
    trust_score = credentials.get("trust_score") or credentials.get("trustScore", 0.85)
    status = credentials.get("status", "active")

    console.agent_registered(
        name=name,
        agent_id=agent_id,
        agent_type=agent_type,
        version=version,
        trust_score=trust_score,
        status=status,
        capabilities=capabilities,
        mcp_servers=mcp_servers
    )
