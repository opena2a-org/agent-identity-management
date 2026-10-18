"""
A2A (Agent-to-Agent) Protocol Support for AIM SDK

This module provides A2A protocol support for secure agent-to-agent communication
with AIM's security enhancements:
- Agent Card registration with AIM attestation
- Ed25519 request signing with anti-replay protection
- Trust score management
- Consent management for GDPR/PSD2 compliance
- Policy evaluation

Example:
    >>> from aim_sdk import AIMClient
    >>> from aim_sdk.a2a import A2AClient
    >>>
    >>> aim = AIMClient(agent_id="...", private_key="...", public_key="...", aim_url="...")
    >>> a2a = A2AClient(aim)
    >>>
    >>> # Register agent card
    >>> card = a2a.register_agent_card(card_url="https://example.com/.well-known/agent.json")
    >>>
    >>> # Get agent card with AIM extensions
    >>> enhanced_card = a2a.get_agent_card()
    >>>
    >>> # Sign an A2A request
    >>> signature = a2a.sign_request(method="POST", path="/tasks", body={"message": "Hello"})
"""

import base64
import hashlib
import json
import time
import uuid
import warnings
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, Union
from datetime import datetime, timezone
from urllib.parse import urlencode

import requests
from nacl.signing import SigningKey
from nacl.encoding import Base64Encoder


# The reason the server answers PUT /api/v1/a2a/trust/:id with 405, word for
# word: there is no write for the A2A trust score.
A2A_TRUST_SCORE_NOT_ASSERTED = (
    "the A2A trust score is measured from recorded interactions, not asserted; "
    "there is no write for it"
)


@dataclass
class A2AAgentCard:
    """Represents an A2A Agent Card with AIM extensions."""
    card_id: str
    agent_id: str
    card_url: str
    name: str = ""
    description: str = ""
    version: str = "1.0.0"
    skills: List[Dict[str, Any]] = field(default_factory=list)
    attestation_expires: Optional[datetime] = None
    aim_extensions: Optional[Dict[str, Any]] = None


@dataclass
class A2ARequestSignature:
    """Represents a signed A2A request."""
    agent_id: str
    timestamp: int
    nonce: str
    signature: str
    public_key: Optional[str] = None

    def to_headers(self) -> Dict[str, str]:
        """Convert signature to HTTP headers for A2A requests."""
        headers = {
            "X-A2A-Agent-ID": self.agent_id,
            "X-A2A-Timestamp": str(self.timestamp),
            "X-A2A-Nonce": self.nonce,
            "X-A2A-Signature": self.signature,
        }
        if self.public_key:
            headers["X-A2A-Public-Key"] = self.public_key
        return headers


@dataclass
class A2ATrustScore:
    """A2A-specific trust score for an agent."""
    agent_id: str
    a2a_trust_score: Optional[float]  # None while the agent is unscored (no task data yet)
    score_status: str = "unscored"  # "measured" | "unscored"
    peer_trust_average: Optional[float] = None
    unique_peers_count: int = 0
    tasks_completed: int = 0
    tasks_failed: int = 0
    computed_at: Optional[datetime] = None


@dataclass
class A2APeerTrust:
    """Trust relationship between two agents."""
    agent_id: str
    peer_agent_id: str
    peer_trust_score: float
    success_rate: Optional[float] = None
    tasks_initiated: int = 0
    tasks_received: int = 0
    first_interaction_at: Optional[datetime] = None
    last_interaction_at: Optional[datetime] = None


@dataclass
class A2AConsent:
    """User consent for cross-agent data sharing."""
    consent_id: str
    user_id: str
    grantor_agent_id: str
    recipient_agent_id: str
    scope: List[str]
    purpose: str
    data_types: List[str]
    granted_at: datetime
    expires_at: Optional[datetime] = None
    revoked: bool = False


class A2AClient:
    """
    A2A Protocol Client for secure agent-to-agent communication.

    This client provides methods for:
    - Registering and managing A2A Agent Cards
    - Signing and verifying A2A requests
    - Managing trust relationships between agents
    - Recording and checking consent for data sharing
    - Evaluating A2A policies

    Args:
        aim_client: An initialized AIMClient instance
        timeout: HTTP request timeout in seconds

    Example:
        >>> from aim_sdk import AIMClient
        >>> from aim_sdk.a2a import A2AClient
        >>>
        >>> aim = AIMClient(agent_id="...", private_key="...", public_key="...", aim_url="...")
        >>> a2a = A2AClient(aim)
        >>>
        >>> # Register agent card
        >>> card = a2a.register_agent_card(card_url="https://example.com/.well-known/agent.json")
    """

    def __init__(self, aim_client, timeout: int = 30):
        """Initialize A2A client with an AIM client."""
        self.aim_client = aim_client
        self.timeout = timeout
        self._session = requests.Session()
        # The package version, as client.py sends, marked as the A2A client. The
        # "AIM-Python-SDK/" prefix is what the backend's SDK tracking matches.
        # Imported here: aim_sdk/__init__ imports this module.
        from . import __version__ as sdk_version
        self._session.headers.update({
            "Content-Type": "application/json",
            "User-Agent": f"AIM-Python-SDK/{sdk_version} (A2A)",
        })

    @property
    def agent_id(self) -> str:
        """Get the agent ID from the AIM client."""
        return self.aim_client.agent_id

    @property
    def aim_url(self) -> str:
        """Get the AIM URL from the AIM client."""
        return self.aim_client.aim_url

    def _make_request(
        self,
        method: str,
        endpoint: str,
        data: Optional[Dict] = None,
        auth: bool = True
    ) -> Dict[str, Any]:
        """
        Make an authenticated request to the AIM A2A API.

        Args:
            method: HTTP method
            endpoint: API endpoint (e.g., /api/v1/a2a/agents/{id}/card)
            data: Request body data
            auth: Whether to add authentication headers

        Returns:
            Response JSON data
        """
        url = f"{self.aim_url}{endpoint}"
        headers = {}

        if auth:
            # Use AIM client's authentication - try OAuth first, then signing key, then API key
            access_token = None
            if hasattr(self.aim_client, 'oauth_token_manager') and self.aim_client.oauth_token_manager:
                try:
                    access_token = self.aim_client.oauth_token_manager.get_access_token()
                except Exception:
                    pass

            if access_token:
                headers['Authorization'] = f'Bearer {access_token}'
            elif self.aim_client.signing_key and self.aim_client.public_key:
                timestamp = str(int(time.time()))
                message_parts = [method.upper(), endpoint, timestamp]
                if data:
                    message_parts.append(json.dumps(data, sort_keys=True))
                message = '\n'.join(message_parts)

                signature = self.aim_client.signing_key.sign(message.encode('utf-8')).signature
                signature_b64 = Base64Encoder.encode(signature).decode('utf-8')

                headers.update({
                    'X-Agent-ID': self.agent_id,
                    'X-Signature': signature_b64,
                    'X-Timestamp': timestamp,
                    'X-Public-Key': self.aim_client.public_key,
                })
            elif self.aim_client.api_key:
                headers['X-API-Key'] = self.aim_client.api_key

        response = self._session.request(
            method=method,
            url=url,
            json=data,
            headers=headers,
            timeout=self.timeout
        )

        if not response.ok:
            error_msg = response.text
            try:
                error_json = response.json()
                error_msg = error_json.get('error', error_msg)
            except:
                pass
            raise A2AError(f"A2A request failed: {error_msg}", status_code=response.status_code)

        return response.json()

    # =========================================================================
    # Agent Card Operations
    # =========================================================================

    def register_agent_card(self, card_url: str) -> A2AAgentCard:
        """
        Register an A2A Agent Card for this agent.

        The card will be fetched from the URL, validated, and an AIM attestation
        will be created for it.

        Args:
            card_url: URL to the agent's A2A Agent Card JSON

        Returns:
            A2AAgentCard with registration details

        Example:
            >>> card = a2a.register_agent_card("https://example.com/.well-known/agent.json")
            >>> print(f"Card registered with {len(card.skills)} skills")
        """
        response = self._make_request(
            "POST",
            f"/api/v1/a2a/agents/{self.agent_id}/card",
            data={"cardUrl": card_url}
        )

        return A2AAgentCard(
            card_id=response.get("cardId", ""),
            agent_id=response.get("agentId", self.agent_id),
            card_url=card_url,
            skills=[{"id": s} for s in response.get("skills", [])],
            attestation_expires=_parse_datetime(response.get("attestationExpires")),
        )

    def get_agent_card(self, agent_id: Optional[str] = None) -> Dict[str, Any]:
        """
        Get an A2A Agent Card with AIM extensions.

        Args:
            agent_id: Agent ID to get card for (defaults to this agent)

        Returns:
            Enhanced Agent Card with AIM attestation and trust info

        Example:
            >>> card = a2a.get_agent_card()
            >>> print(f"Trust score: {card['aim']['trustScore']}")
        """
        target_id = agent_id or self.agent_id
        return self._make_request("GET", f"/api/v1/a2a/agents/{target_id}/card")

    def refresh_card_attestation(self) -> Dict[str, Any]:
        """
        Refresh the AIM attestation for this agent's card.

        Returns:
            Updated card info with new attestation expiry

        Example:
            >>> result = a2a.refresh_card_attestation()
            >>> print(f"Attestation valid until: {result['attestationExpires']}")
        """
        return self._make_request(
            "POST",
            f"/api/v1/a2a/agents/{self.agent_id}/card/refresh"
        )

    # =========================================================================
    # Multi-Agent Skill Attestation (Consensus Verification)
    # =========================================================================

    def attest_skill(
        self,
        target_agent_id: str,
        skill_id: str,
        attestation_type: str = "SKILL_VERIFICATION",
        confidence: float = 1.0,
        evidence: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """
        Attest to another agent's skill capability.

        This creates a signed attestation that contributes to multi-agent
        consensus verification. Skills become "verified" when they reach
        the consensus threshold (3+ unique attesters, 2+ unique owners).

        Args:
            target_agent_id: Agent whose skill is being attested
            skill_id: ID of the skill being attested
            attestation_type: Type of attestation (SKILL_VERIFICATION, QUALITY, SECURITY)
            confidence: Confidence level (0.0 to 1.0)
            evidence: Optional evidence supporting the attestation

        Returns:
            The created attestation record

        Example:
            >>> attestation = a2a.attest_skill(
            ...     target_agent_id="agent-123",
            ...     skill_id="code-analysis",
            ...     confidence=0.95,
            ...     evidence={"test_run_id": "abc123", "accuracy": 0.98}
            ... )
        """
        return self._make_request(
            "POST",
            "/api/v1/a2a/attestations",
            data={
                "attestingAgentId": self.agent_id,
                "attestedAgentId": target_agent_id,
                "skillId": skill_id,
                "attestationType": attestation_type,
                "confidence": confidence,
                "evidence": evidence or {}
            }
        )

    def get_skill_attestations(
        self,
        agent_id: str,
        skill_id: Optional[str] = None
    ) -> List[Dict[str, Any]]:
        """
        Get attestations for an agent's skill(s).

        Args:
            agent_id: Agent to get attestations for
            skill_id: Optional specific skill (all skills if not specified)

        Returns:
            List of attestation records

        Example:
            >>> attestations = a2a.get_skill_attestations("agent-123", "code-analysis")
            >>> print(f"Found {len(attestations)} attestations")
        """
        params = {}
        if skill_id:
            params["skillId"] = skill_id
        return self._make_request(
            "GET",
            f"/api/v1/a2a/agents/{agent_id}/attestations",
            params=params
        ).get("attestations", [])

    def get_consensus_status(
        self,
        agent_id: str,
        skill_id: str
    ) -> Dict[str, Any]:
        """
        Get the consensus verification status for a skill.

        Returns whether a skill has reached the verification threshold
        through multi-agent consensus.

        Args:
            agent_id: Agent that owns the skill
            skill_id: Skill to check

        Returns:
            Consensus status including:
            - isVerified: Whether consensus threshold is met
            - attestationCount: Total number of attestations
            - uniqueAttesters: Number of unique attesting agents
            - uniqueOwners: Number of unique organization owners
            - confidenceScore: Calculated confidence score

        Example:
            >>> status = a2a.get_consensus_status("agent-123", "code-analysis")
            >>> if status["isVerified"]:
            ...     print("Skill is verified by consensus!")
        """
        return self._make_request(
            "GET",
            f"/api/v1/a2a/agents/{agent_id}/skills/{skill_id}/consensus"
        )

    def revoke_attestation(self, attestation_id: str, reason: str) -> Dict[str, Any]:
        """
        Revoke a previously made attestation.

        Args:
            attestation_id: ID of the attestation to revoke
            reason: Reason for revocation

        Returns:
            The revoked attestation record
        """
        return self._make_request(
            "POST",
            f"/api/v1/a2a/attestations/{attestation_id}/revoke",
            data={"reason": reason}
        )

    # =========================================================================
    # Request Signing
    # =========================================================================

    def sign_request(
        self,
        method: str,
        path: str,
        body: Optional[Union[Dict, str, bytes]] = None
    ) -> A2ARequestSignature:
        """
        Sign an outgoing A2A request.

        This creates a signed request that can be verified by the receiving agent
        using AIM's verification endpoint.

        Args:
            method: HTTP method (GET, POST, etc.)
            path: Request path
            body: Request body (dict, string, or bytes)

        Returns:
            A2ARequestSignature with signature details

        Example:
            >>> sig = a2a.sign_request("POST", "/tasks/send", {"message": "Hello"})
            >>> # Use sig.to_headers() to add to your HTTP request
            >>> headers = sig.to_headers()
        """
        body_str = ""
        if body:
            if isinstance(body, (dict, list)):
                body_str = json.dumps(body, sort_keys=True)
            elif isinstance(body, bytes):
                body_str = body.decode('utf-8')
            else:
                body_str = str(body)

        response = self._make_request(
            "POST",
            f"/api/v1/a2a/agents/{self.agent_id}/sign",
            data={
                "method": method,
                "path": path,
                "body": body_str
            }
        )

        return A2ARequestSignature(
            agent_id=response.get("agentId", self.agent_id),
            timestamp=response.get("timestamp", int(time.time())),
            nonce=response.get("nonce", ""),
            signature=response.get("signature", ""),
            public_key=self.aim_client.public_key
        )

    def verify_request(
        self,
        agent_id: str,
        timestamp: int,
        nonce: str,
        signature: str,
        request_hash: str
    ) -> Dict[str, Any]:
        """
        Verify an incoming A2A request signature.

        Args:
            agent_id: ID of the agent that signed the request
            timestamp: Unix timestamp from the signature
            nonce: Nonce from the signature
            signature: Base64-encoded signature
            request_hash: SHA256 hash of the request (hex)

        Returns:
            Verification result with agent info if valid

        Example:
            >>> result = a2a.verify_request(
            ...     agent_id="...",
            ...     timestamp=1234567890,
            ...     nonce="abc123",
            ...     signature="base64...",
            ...     request_hash="sha256..."
            ... )
            >>> if result["valid"]:
            ...     print(f"Request from {result['agentName']} with trust score {result['trustScore']}")
        """
        return self._make_request(
            "POST",
            "/api/v1/a2a/verify",
            data={
                "agentId": agent_id,
                "timestamp": timestamp,
                "nonce": nonce,
                "signature": signature,
                "requestHash": request_hash
            },
            auth=False  # Public endpoint
        )

    # =========================================================================
    # Trust Score Operations
    # =========================================================================

    def get_a2a_trust_score(self, agent_id: Optional[str] = None) -> A2ATrustScore:
        """
        Get the A2A-specific trust score for an agent.

        Args:
            agent_id: Agent ID to get score for (defaults to this agent)

        Returns:
            A2ATrustScore with score details

        Example:
            >>> score = a2a.get_a2a_trust_score()
            >>> print(f"A2A Trust: {score.a2a_trust_score:.2f}")
        """
        target_id = agent_id or self.agent_id
        response = self._make_request("GET", f"/api/v1/a2a/agents/{target_id}/trust-score")

        return A2ATrustScore(
            agent_id=target_id,
            a2a_trust_score=response.get("a2aTrustScore"),
            score_status=response.get("scoreStatus", "measured" if response.get("a2aTrustScore") is not None else "unscored"),
            peer_trust_average=response.get("peerTrustAverage"),
            unique_peers_count=response.get("uniquePeersCount", 0),
            tasks_completed=response.get("tasksCompleted", 0),
            tasks_failed=response.get("tasksFailed", 0),
            computed_at=_parse_datetime(response.get("computedAt")),
        )

    def compute_a2a_trust_score(self) -> A2ATrustScore:
        """
        Compute and update the A2A trust score for this agent.

        Returns:
            Updated A2ATrustScore

        Example:
            >>> score = a2a.compute_a2a_trust_score()
            >>> print(f"New A2A Trust: {score.a2a_trust_score:.2f}")
        """
        response = self._make_request(
            "POST",
            f"/api/v1/a2a/agents/{self.agent_id}/trust-score/compute"
        )

        return A2ATrustScore(
            agent_id=self.agent_id,
            a2a_trust_score=response.get("a2aTrustScore"),
            score_status=response.get("scoreStatus", "measured" if response.get("a2aTrustScore") is not None else "unscored"),
            peer_trust_average=response.get("peerTrustAverage"),
            unique_peers_count=response.get("uniquePeersCount", 0),
            tasks_completed=response.get("tasksCompleted", 0),
            tasks_failed=response.get("tasksFailed", 0),
            computed_at=_parse_datetime(response.get("computedAt")),
        )

    def get_peer_trust(self, peer_agent_id: str) -> A2APeerTrust:
        """
        Get the trust relationship with a specific peer agent.

        Args:
            peer_agent_id: ID of the peer agent

        Returns:
            A2APeerTrust with relationship details

        Example:
            >>> trust = a2a.get_peer_trust("peer-agent-id")
            >>> print(f"Trust with peer: {trust.peer_trust_score:.2f}")
        """
        response = self._make_request(
            "GET",
            f"/api/v1/a2a/agents/{self.agent_id}/peers/{peer_agent_id}/trust"
        )

        return A2APeerTrust(
            agent_id=self.agent_id,
            peer_agent_id=peer_agent_id,
            peer_trust_score=response.get("peerTrustScore", 0.0),
            success_rate=response.get("successRate"),
            tasks_initiated=response.get("tasksInitiated", 0),
            tasks_received=response.get("tasksReceived", 0),
            first_interaction_at=_parse_datetime(response.get("firstInteractionAt")),
            last_interaction_at=_parse_datetime(response.get("lastInteractionAt")),
        )

    # =========================================================================
    # Skill Operations
    # =========================================================================

    def get_skills(self, agent_id: Optional[str] = None) -> List[Dict[str, Any]]:
        """
        Get A2A skills for an agent.

        Args:
            agent_id: Agent ID to get skills for (defaults to this agent)

        Returns:
            List of skills with details

        Example:
            >>> skills = a2a.get_skills()
            >>> for skill in skills:
            ...     print(f"{skill['name']}: {skill['description']}")
        """
        target_id = agent_id or self.agent_id
        response = self._make_request("GET", f"/api/v1/a2a/agents/{target_id}/skills")
        return response.get("skills", [])

    def search_skills(self, query: str, limit: int = 20) -> List[Dict[str, Any]]:
        """
        Search for A2A skills across all agents.

        Args:
            query: Search query
            limit: Maximum number of results

        Returns:
            List of matching skills

        Example:
            >>> skills = a2a.search_skills("data analysis")
            >>> print(f"Found {len(skills)} matching skills")
        """
        response = self._make_request(
            "GET",
            f"/api/v1/a2a/skills/search?q={query}&limit={limit}"
        )
        return response.get("skills", [])

    # =========================================================================
    # Task Logging
    # =========================================================================

    def log_task(
        self,
        external_task_id: str,
        remote_agent_id: str,
        skill_id: Optional[str] = None,
        context_id: Optional[str] = None
    ) -> Dict[str, Any]:
        """
        Log an A2A task for audit trail.

        Args:
            external_task_id: External ID of the task
            remote_agent_id: ID of the remote agent
            skill_id: Optional skill ID being invoked
            context_id: Optional context/session ID

        Returns:
            Created task details

        Example:
            >>> task = a2a.log_task(
            ...     external_task_id="task-123",
            ...     remote_agent_id="remote-agent-id",
            ...     skill_id="analyze-data"
            ... )
        """
        return self._make_request(
            "POST",
            "/api/v1/a2a/tasks",
            data={
                "externalTaskId": external_task_id,
                "clientAgentId": self.agent_id,
                "remoteAgentId": remote_agent_id,
                "skillId": skill_id,
                "contextId": context_id
            }
        )

    def update_task_state(
        self,
        task_id: str,
        state: str,
        error_code: Optional[str] = None,
        error_message: Optional[str] = None
    ) -> Dict[str, Any]:
        """
        Update the state of an A2A task.

        Args:
            task_id: Task ID to update
            state: New state (SUBMITTED, WORKING, INPUT_NEEDED, COMPLETED, FAILED, CANCELLED)
            error_code: Optional error code if failed
            error_message: Optional error message if failed

        Returns:
            Update confirmation

        Example:
            >>> a2a.update_task_state("task-id", "COMPLETED")
        """
        return self._make_request(
            "PUT",
            f"/api/v1/a2a/tasks/{task_id}/state",
            data={
                "state": state,
                "errorCode": error_code or "",
                "errorMessage": error_message or ""
            }
        )

    def get_task_history(
        self,
        target_agent_id: str,
        limit: int = 50
    ) -> List[Dict[str, Any]]:
        """
        Get task history for interactions with a target agent.

        Lists the tasks visible to the caller's organization in which the
        target agent was the client or the remote agent.

        Args:
            target_agent_id: ID of the target agent (a UUID)
            limit: Maximum number of results

        Returns:
            List of task entries

        Raises:
            ValueError: target_agent_id is not a UUID. The server ignores an
                agentId it cannot read as one and answers with every task the
                organization can see, so the request is not sent.

        Example:
            >>> history = a2a.get_task_history("other-agent-id", limit=10)
            >>> for task in history:
            ...     print(f"{task['externalTaskId']}: {task['state']}")
        """
        agent_id = str(uuid.UUID(str(target_agent_id)))
        response = self._make_request(
            "GET",
            f"/api/v1/a2a/tasks?{urlencode({'agentId': agent_id, 'limit': limit})}"
        )
        return response.get("tasks", [])

    # =========================================================================
    # Consent Management
    # =========================================================================

    def record_consent(
        self,
        user_id: str,
        recipient_agent_id: str,
        scope: List[str],
        purpose: str,
        data_types: List[str],
        expires_in_hours: int = 24,
        consent_method: str = "api"
    ) -> A2AConsent:
        """
        Record user consent for cross-agent data sharing.

        Args:
            user_id: ID of the user granting consent
            recipient_agent_id: ID of the agent receiving data
            scope: Consent scopes (e.g., ["pii", "payment"])
            purpose: Purpose of the consent
            data_types: Types of data being shared (e.g., ["email", "name"])
            expires_in_hours: Hours until consent expires
            consent_method: How consent was obtained

        Returns:
            A2AConsent with details

        Example:
            >>> consent = a2a.record_consent(
            ...     user_id="user-123",
            ...     recipient_agent_id="other-agent-id",
            ...     scope=["pii"],
            ...     purpose="Process order",
            ...     data_types=["email", "address"]
            ... )
        """
        response = self._make_request(
            "POST",
            "/api/v1/a2a/consent",
            data={
                "userId": user_id,
                "grantorAgentId": self.agent_id,
                "recipientAgentId": recipient_agent_id,
                "scope": scope,
                "purpose": purpose,
                "dataTypes": data_types,
                "expiresInHours": expires_in_hours,
                "consentMethod": consent_method
            }
        )

        return A2AConsent(
            consent_id=response.get("id", ""),
            user_id=user_id,
            grantor_agent_id=self.agent_id,
            recipient_agent_id=recipient_agent_id,
            scope=scope,
            purpose=purpose,
            data_types=data_types,
            granted_at=_parse_datetime(response.get("grantedAt")) or datetime.now(timezone.utc),
            expires_at=_parse_datetime(response.get("expiresAt")),
        )

    def check_consent(
        self,
        user_id: str,
        recipient_agent_id: str,
        scope: str
    ) -> bool:
        """
        Check if consent exists for a specific scope.

        Args:
            user_id: ID of the user
            recipient_agent_id: ID of the recipient agent
            scope: Scope to check

        Returns:
            True if consent exists, False otherwise

        Example:
            >>> if a2a.check_consent("user-123", "other-agent", "pii"):
            ...     # Share data
        """
        response = self._make_request(
            "GET",
            f"/api/v1/a2a/consent/check?userId={user_id}&grantorAgentId={self.agent_id}&recipientAgentId={recipient_agent_id}&scope={scope}"
        )
        return response.get("hasConsent", False)

    def revoke_consent(self, consent_id: str, reason: str = "User revoked consent") -> Dict[str, Any]:
        """
        Revoke a consent record.

        Args:
            consent_id: ID of the consent to revoke
            reason: Reason for revocation

        Returns:
            Revocation confirmation

        Example:
            >>> a2a.revoke_consent("consent-id", "User request")
        """
        return self._make_request(
            "POST",
            f"/api/v1/a2a/consent/{consent_id}/revoke",
            data={"reason": reason}
        )

    def list_user_consents(self, user_id: str, include_revoked: bool = False) -> List[A2AConsent]:
        """
        List all consent records for a user.

        Args:
            user_id: ID of the user
            include_revoked: Whether to include revoked consents

        Returns:
            List of A2AConsent records

        Example:
            >>> consents = a2a.list_user_consents("user-123")
            >>> for consent in consents:
            ...     print(f"{consent.purpose}: {consent.scope}")
        """
        response = self._make_request(
            "GET",
            f"/api/v1/a2a/consent/user/{user_id}?includeRevoked={str(include_revoked).lower()}"
        )

        return [
            A2AConsent(
                consent_id=c.get("id", ""),
                user_id=user_id,
                grantor_agent_id=c.get("grantorAgentId", ""),
                recipient_agent_id=c.get("recipientAgentId", ""),
                scope=c.get("scope", []),
                purpose=c.get("purpose", ""),
                data_types=c.get("dataTypes", []),
                granted_at=_parse_datetime(c.get("grantedAt")) or datetime.now(timezone.utc),
                expires_at=_parse_datetime(c.get("expiresAt")),
                revoked=c.get("revoked", False),
            )
            for c in response.get("consents", [])
        ]

    # =========================================================================
    # Intent-Based Discovery
    # =========================================================================

    def route_by_intent(
        self,
        intent: str,
        min_trust_score: float = 0.0
    ) -> Dict[str, Any]:
        """
        Find an agent capable of handling a natural language intent.

        Uses PostgreSQL full-text search to find agents with matching skills.

        Args:
            intent: Natural language description of what you want to do
            min_trust_score: Minimum trust score required (0.0-1.0)

        Returns:
            Best matching agent with skill details, or error if none found

        Example:
            >>> result = a2a.route_by_intent("send an email to john@example.com")
            >>> if result.get("agent"):
            ...     print(f"Found: {result['agent']['agentName']} with {result['agent']['skillName']}")
        """
        return self._make_request(
            "GET",
            f"/api/v1/a2a/route?intent={intent}&minTrustScore={min_trust_score}"
        )

    def get_capable_agents(
        self,
        intent: str,
        min_trust_score: float = 0.0,
        limit: int = 10
    ) -> List[Dict[str, Any]]:
        """
        Find all agents capable of handling a natural language intent.

        Args:
            intent: Natural language description of what you want to do
            min_trust_score: Minimum trust score required (0.0-1.0)
            limit: Maximum number of results

        Returns:
            List of matching agents with skill details, sorted by relevance

        Example:
            >>> agents = a2a.get_capable_agents("analyze data", limit=5)
            >>> for agent in agents:
            ...     print(f"{agent['agentName']}: {agent['skillName']} (trust: {agent['trustScore']})")
        """
        response = self._make_request(
            "GET",
            f"/api/v1/a2a/capable-of?intent={intent}&minTrustScore={min_trust_score}&limit={limit}"
        )
        return response.get("agents", [])

    # =========================================================================
    # Security Policy Enforcement
    # =========================================================================

    def check_security(
        self,
        target_agent_id: str,
        skill_id: Optional[str] = None,
        request_signature: Optional[str] = None,
        request_nonce: Optional[str] = None
    ) -> Dict[str, Any]:
        """
        Check if an A2A request passes security policies.

        Evaluates the request against the target organization's security settings:
        - Agent revocation status
        - Allowed/blocked agent lists
        - Trust score requirements
        - Verified skill requirements
        - Request signing requirements
        - Nonce anti-replay validation

        Args:
            target_agent_id: ID of the agent being called
            skill_id: Optional skill ID being invoked
            request_signature: Optional request signature
            request_nonce: Optional request nonce for anti-replay

        Returns:
            Security check result with allowed status and any violations

        Example:
            >>> result = a2a.check_security("target-agent-id", skill_id="analyze-data")
            >>> if result["allowed"]:
            ...     # Proceed with request
            ... else:
            ...     print(f"Blocked: {result['violations']}")
        """
        return self._make_request(
            "POST",
            "/api/v1/a2a/security/check",
            data={
                "requestingAgentId": self.agent_id,
                "targetAgentId": target_agent_id,
                "skillId": skill_id or "",
                "requestSignature": request_signature or "",
                "requestNonce": request_nonce or ""
            }
        )

    @staticmethod
    def _check_organization(organization_id: Optional[str], answered: Optional[str]) -> None:
        """
        Raise A2AError when organization_id names an organization other than
        the one the server answered for. The server answers for the
        organization of the caller's credentials, whatever the caller names.
        """
        if organization_id is None or answered is None:
            return
        if str(organization_id).lower() != str(answered).lower():
            raise A2AError(
                f"the server answers for the caller's organization ({answered}), "
                f"not for organization {organization_id}"
            )

    def get_security_settings(self, organization_id: Optional[str] = None) -> Dict[str, Any]:
        """
        Get the A2A security settings of the caller's organization.

        Args:
            organization_id: Optional. The server answers for the organization
                of the caller's credentials; when given, this must be it.

        Returns:
            Security settings including enforcement mode and requirements

        Raises:
            A2AError: organization_id is not the caller's organization.

        Example:
            >>> settings = a2a.get_security_settings()
            >>> print(f"Mode: {settings['enforcementMode']}")  # 'monitor' or 'strict'
        """
        settings = self._make_request("GET", "/api/v1/a2a/security/settings")
        self._check_organization(organization_id, settings.get("organizationId"))
        return settings

    def get_security_violations(
        self,
        organization_id: Optional[str] = None,
        limit: int = 50,
        offset: int = 0
    ) -> List[Dict[str, Any]]:
        """
        Get the A2A security violations of the caller's organization.

        Args:
            organization_id: Optional. The server answers for the organization
                of the caller's credentials; when given, this must be it. It
                is checked against the security settings, with one more
                request, before the violations are requested.
            limit: Maximum number of results
            offset: Pagination offset

        Returns:
            List of security violations

        Raises:
            A2AError: organization_id is not the caller's organization.

        Example:
            >>> violations = a2a.get_security_violations()
            >>> for v in violations:
            ...     print(f"{v['violationType']}: {v['actionTaken']}")
        """
        if organization_id is not None:
            # An answer with no violations names no organization, so the
            # settings answer, which names the caller's, is checked first.
            self.get_security_settings(organization_id)
        response = self._make_request(
            "GET",
            f"/api/v1/a2a/security/violations?{urlencode({'limit': limit, 'offset': offset})}"
        )
        # The server answers with the list itself, or null when there is none.
        if isinstance(response, dict):
            violations = response.get("violations") or []
        else:
            violations = response or []
        for violation in violations:
            self._check_organization(organization_id, violation.get("organizationId"))
        return violations

    # =========================================================================
    # Policy Evaluation
    # =========================================================================

    def evaluate_policy(
        self,
        event: str,
        remote_agent_id: str,
        skill_id: Optional[str] = None,
        organization_id: Optional[str] = None
    ) -> Dict[str, Any]:
        """
        Evaluate A2A policies for a given action.

        Args:
            event: Event type (e.g., "task.create", "task.received")
            remote_agent_id: ID of the remote agent
            skill_id: Optional skill ID
            organization_id: Optional organization ID

        Returns:
            Policy decision with details

        Example:
            >>> decision = a2a.evaluate_policy(
            ...     event="task.create",
            ...     remote_agent_id="other-agent-id"
            ... )
            >>> if decision["decision"] == "ALLOW":
            ...     # Proceed with action
        """
        data = {
            "event": event,
            "clientAgentId": self.agent_id,
            "remoteAgentId": remote_agent_id,
        }
        if skill_id:
            data["skillId"] = skill_id
        if organization_id:
            data["organizationId"] = organization_id

        return self._make_request("POST", "/api/v1/a2a/policies/evaluate", data=data)

    # =========================================================================
    # Agent Card List & Update Operations
    # =========================================================================

    def list_agent_cards(self, limit: int = 100, offset: int = 0) -> List[A2AAgentCard]:
        """
        List all registered A2A agent cards.

        Args:
            limit: Maximum number of results
            offset: Pagination offset

        Returns:
            List of A2AAgentCard objects

        Example:
            >>> cards = a2a.list_agent_cards(limit=10)
            >>> for card in cards:
            ...     print(f"{card.name}: {card.card_url}")
        """
        response = self._make_request(
            "GET",
            f"/api/v1/a2a/cards?limit={limit}&offset={offset}"
        )
        return [
            A2AAgentCard(
                card_id=c.get("id", ""),
                agent_id=c.get("agentId", ""),
                card_url=c.get("cardUrl", ""),
                name=c.get("name", ""),
                description=c.get("description", ""),
                version=c.get("version", "1.0.0"),
                skills=c.get("skills", []),
                attestation_expires=_parse_datetime(c.get("attestationExpires")),
                aim_extensions=c.get("aimExtensions"),
            )
            for c in response.get("cards", [])
        ]

    # The server's answer to a card update that carries neither cardUrl nor
    # cardData.
    CARD_DOCUMENT_REQUIRED = (
        "cardUrl or cardData is required: this route replaces the agent card "
        "document and does not edit fields"
    )

    def update_agent_card(
        self,
        name: Optional[str] = None,
        description: Optional[str] = None,
        version: Optional[str] = None,
        skills: Optional[List[Union[str, Dict[str, Any]]]] = None,
        *,
        card_url: Optional[str] = None,
        card_data: Optional[Dict[str, Any]] = None,
    ) -> A2AAgentCard:
        """
        Replace the current agent's A2A card document.

        The server fetches the card from `card_url` or takes `card_data`,
        recomputes the card's hash, issues a new attestation and stores the
        card in place of the previous one. The route does not edit a card
        field by field.

        Args:
            name: Deprecated. The route does not read it; put it in card_data.
            description: Deprecated, as name.
            version: Deprecated, as name.
            skills: Deprecated, as name.
            card_url: URL of the card document, which the server fetches.
            card_data: The card document itself, as a dictionary.

        Returns:
            A2AAgentCard with the stored card's ID, its URL, the identifiers of
            its skills (each as a dictionary with an `id`) and the expiry of the
            new attestation

        Raises:
            ValueError: neither card_url nor card_data is given, card_data is
                not a dictionary, or name, description, version or skills is
                given. The message names the arguments the route does not
                read. No request is sent.

        Example:
            >>> card = a2a.update_agent_card(card_data={
            ...     "name": "My Updated Agent",
            ...     "description": "Now with more capabilities",
            ...     "url": "https://agent.example.com",
            ...     "version": "1.1.0",
            ...     "skills": [{"id": "summarize", "name": "Summarize"}],
            ... })
        """
        unread = [
            member
            for member, value in (
                ("name", name),
                ("description", description),
                ("version", version),
                ("skills", skills),
            )
            if value is not None
        ]
        if not card_url and card_data is None:
            message = self.CARD_DOCUMENT_REQUIRED
            if unread:
                message += "; it does not read " + ", ".join(unread)
            raise ValueError(message)
        if unread:
            raise ValueError(
                "update_agent_card replaces the agent card document and does not edit fields; "
                f"it does not read {', '.join(unread)}: put them in card_data"
            )
        if card_data is not None and not isinstance(card_data, dict):
            raise ValueError("card_data must be the card document, as a dictionary")

        data: Dict[str, Any] = {}
        if card_url:
            data["cardUrl"] = card_url
        if card_data is not None:
            data["cardData"] = card_data

        response = self._make_request(
            "PUT",
            f"/api/v1/a2a/cards/{self.agent_id}",
            data=data
        )

        return A2AAgentCard(
            card_id=response.get("cardId", ""),
            agent_id=response.get("agentId", self.agent_id),
            card_url=response.get("cardUrl", card_url or ""),
            skills=[{"id": s} for s in response.get("skills", [])],
            attestation_expires=_parse_datetime(response.get("attestationExpires")),
        )

    # =========================================================================
    # Extended Trust Score Operations
    # =========================================================================

    def update_trust_score(
        self,
        target_agent_id: str,
        score: float,
        confidence: float,
        factors: Optional[Dict[str, float]] = None
    ) -> A2ATrustScore:
        """
        Deprecated, and removed at the next major version: the A2A trust score
        is measured from recorded interactions and recomputed by the server; it
        is not asserted by a caller, and the server answers a write to it with
        405. The method sends nothing and raises A2ATrustScoreWriteRefused,
        which names the route that reads the score and the one that records an
        interaction (record_successful_interaction, record_failed_interaction).
        Kept until then so existing code still imports.

        Args:
            target_agent_id: ID of the agent the caller meant to score
            score: Ignored
            confidence: Ignored
            factors: Ignored

        Raises:
            A2ATrustScoreWriteRefused: always. No request is sent.
        """
        raise A2ATrustScoreWriteRefused(target_agent_id)

    def record_successful_interaction(
        self,
        target_agent_id: str,
        task_type: str,
        duration_ms: int
    ) -> A2ATrustScore:
        """
        Record a successful interaction with another agent.

        This updates the trust score based on successful task completion.

        Args:
            target_agent_id: ID of the agent interacted with
            task_type: Type of task completed
            duration_ms: Duration of the task in milliseconds

        Returns:
            Updated A2ATrustScore

        Example:
            >>> score = a2a.record_successful_interaction(
            ...     target_agent_id="other-agent",
            ...     task_type="data_analysis",
            ...     duration_ms=5000
            ... )
        """
        response = self._make_request(
            "POST",
            f"/api/v1/a2a/trust/{target_agent_id}/interaction",
            data={
                "success": True,
                "taskType": task_type,
                "durationMs": duration_ms,
            }
        )

        return A2ATrustScore(
            agent_id=target_agent_id,
            a2a_trust_score=response.get("a2aTrustScore"),
            score_status=response.get("scoreStatus", "measured" if response.get("a2aTrustScore") is not None else "unscored"),
            peer_trust_average=response.get("peerTrustAverage"),
            unique_peers_count=response.get("uniquePeersCount", 0),
            tasks_completed=response.get("tasksCompleted", 0),
            tasks_failed=response.get("tasksFailed", 0),
            computed_at=_parse_datetime(response.get("computedAt")),
        )

    def record_failed_interaction(
        self,
        target_agent_id: str,
        task_type: str,
        error_reason: str
    ) -> A2ATrustScore:
        """
        Record a failed interaction with another agent.

        This updates the trust score based on task failure.

        Args:
            target_agent_id: ID of the agent interacted with
            task_type: Type of task that failed
            error_reason: Reason for the failure

        Returns:
            Updated A2ATrustScore

        Example:
            >>> score = a2a.record_failed_interaction(
            ...     target_agent_id="other-agent",
            ...     task_type="data_analysis",
            ...     error_reason="Timeout exceeded"
            ... )
        """
        response = self._make_request(
            "POST",
            f"/api/v1/a2a/trust/{target_agent_id}/interaction",
            data={
                "success": False,
                "taskType": task_type,
                "errorReason": error_reason,
            }
        )

        return A2ATrustScore(
            agent_id=target_agent_id,
            a2a_trust_score=response.get("a2aTrustScore"),
            score_status=response.get("scoreStatus", "measured" if response.get("a2aTrustScore") is not None else "unscored"),
            peer_trust_average=response.get("peerTrustAverage"),
            unique_peers_count=response.get("uniquePeersCount", 0),
            tasks_completed=response.get("tasksCompleted", 0),
            tasks_failed=response.get("tasksFailed", 0),
            computed_at=_parse_datetime(response.get("computedAt")),
        )

    def list_peer_trusts(self) -> List[A2APeerTrust]:
        """
        List all peer trust relationships for the current agent.

        Returns:
            List of A2APeerTrust relationships

        Example:
            >>> peers = a2a.list_peer_trusts()
            >>> for peer in peers:
            ...     print(f"{peer.peer_agent_id}: {peer.peer_trust_score}")
        """
        response = self._make_request("GET", "/api/v1/a2a/peers")

        return [
            A2APeerTrust(
                agent_id=self.agent_id,
                peer_agent_id=p.get("peerAgentId", ""),
                peer_trust_score=p.get("peerTrustScore", 0.0),
                success_rate=p.get("successRate"),
                tasks_initiated=p.get("tasksInitiated", 0),
                tasks_received=p.get("tasksReceived", 0),
                first_interaction_at=_parse_datetime(p.get("firstInteractionAt")),
                last_interaction_at=_parse_datetime(p.get("lastInteractionAt")),
            )
            for p in response.get("peers", [])
        ]

    # =========================================================================
    # Skill Registration & Management
    # =========================================================================

    def register_skill(
        self,
        name: str,
        description: str,
        input_schema: Optional[Dict[str, Any]] = None,
        output_schema: Optional[Dict[str, Any]] = None,
        tags: Optional[List[str]] = None,
        *,
        input_modes: Optional[List[str]] = None,
        output_modes: Optional[List[str]] = None,
        examples: Optional[List[str]] = None
    ) -> Dict[str, Any]:
        """
        Register a new skill for the current agent.

        Args:
            name: Skill name
            description: Skill description
            input_schema, output_schema: Deprecated and not sent; passing
                either emits a DeprecationWarning. The server reads no schema:
                it reads a skill's input and output modes (input_modes,
                output_modes).
            tags: Optional tags for discovery
            input_modes: The A2A input modes of the skill, the media types it
                accepts, e.g. ["text/plain"]
            output_modes: The A2A output modes of the skill, the media types
                it answers in
            examples: Example prompts for the skill, as strings

        Returns:
            Created skill details

        Example:
            >>> skill = a2a.register_skill(
            ...     name="analyze_code",
            ...     description="Analyzes code for quality issues",
            ...     tags=["code", "analysis"],
            ...     input_modes=["text/plain"],
            ...     output_modes=["application/json"]
            ... )
        """
        if input_schema is not None or output_schema is not None:
            warnings.warn(
                "The input_schema and output_schema arguments of register_skill() are "
                "deprecated and are not sent: the server reads no schema, only a skill's "
                "input and output modes. Pass input_modes and output_modes.",
                DeprecationWarning,
                stacklevel=2
            )

        data: Dict[str, Any] = {
            "agentId": self.agent_id,
            "name": name,
            "description": description,
        }
        if tags:
            data["tags"] = tags
        if input_modes:
            data["inputModes"] = input_modes
        if output_modes:
            data["outputModes"] = output_modes
        if examples:
            data["examples"] = examples

        return self._make_request("POST", "/api/v1/a2a/skills", data=data)

    def delete_skill(self, skill_id: str) -> Dict[str, Any]:
        """
        Delete a skill.

        Args:
            skill_id: ID of the skill to delete

        Returns:
            Deletion confirmation

        Example:
            >>> a2a.delete_skill("skill-123")
        """
        return self._make_request("DELETE", f"/api/v1/a2a/skills/{skill_id}")

    def list_skills(self) -> List[Dict[str, Any]]:
        """
        List the skills of the current agent.

        Requests GET /api/v1/a2a/agents/{agentId}/skills with this client's
        agent ID, the request get_skills() makes when it is given no agent ID.

        Returns:
            List of skills

        Example:
            >>> skills = a2a.list_skills()
            >>> for skill in skills:
            ...     print(f"{skill['skillId']}: {skill['name']}")
        """
        return self.get_skills()

    # =========================================================================
    # Security Settings Update
    # =========================================================================

    def update_security_settings(
        self,
        organization_id: Optional[str] = None,
        enforcement_mode: Optional[str] = None,
        require_signing: Optional[bool] = None,
        min_trust_score: Optional[float] = None,
        require_verified_skills: Optional[bool] = None,
        allowed_agents: Optional[List[str]] = None,
        blocked_agents: Optional[List[str]] = None
    ) -> Dict[str, Any]:
        """
        Update the A2A security settings of the caller's organization.

        The server replaces every setting with the ones the request carries,
        so this reads the current settings, changes the arguments that are
        not None, and writes the whole set back. A change made by someone
        else between the read and the write is overwritten.

        Args:
            organization_id: Optional. The server answers for the organization
                of the caller's credentials; when given, this must be it, and
                nothing is written otherwise.
            enforcement_mode: 'monitor' or 'strict'
            require_signing: Require Ed25519 request signing
            min_trust_score: Minimum trust score required (0.0-1.0)
            require_verified_skills: Require skills to be verified
            allowed_agents: List of allowed agent IDs (allowlist)
            blocked_agents: List of blocked agent IDs (blocklist)

        Returns:
            Updated security settings

        Raises:
            A2AError: organization_id is not the caller's organization.

        Example:
            >>> settings = a2a.update_security_settings(
            ...     enforcement_mode="strict",
            ...     min_trust_score=0.7
            ... )
        """
        data = dict(self.get_security_settings(organization_id))
        changes = {
            "enforcementMode": enforcement_mode,
            "requireRequestSigning": require_signing,
            "minTrustScore": min_trust_score,
            "requireVerifiedSkills": require_verified_skills,
            "allowedAgentIds": allowed_agents,
            "blockedAgentIds": blocked_agents,
        }
        data.update({key: value for key, value in changes.items() if value is not None})

        return self._make_request(
            "PUT",
            "/api/v1/a2a/security/settings",
            data=data
        )

    # =========================================================================
    # URL-Based Discovery
    # =========================================================================

    def discover_agent(self, agent_url: str) -> A2AAgentCard:
        """
        Discover and register an agent by URL.

        Fetches the agent's .well-known/agent.json card and registers it.

        Args:
            agent_url: Base URL of the agent (e.g., "https://agent.example.com")

        Returns:
            Registered A2AAgentCard

        Example:
            >>> card = a2a.discover_agent("https://analysis-agent.example.com")
            >>> print(f"Discovered: {card.name} with {len(card.skills)} skills")
        """
        # Construct the well-known agent card URL
        if agent_url.endswith('/'):
            card_url = f"{agent_url}.well-known/agent.json"
        else:
            card_url = f"{agent_url}/.well-known/agent.json"

        return self.register_agent_card(card_url)


class A2AError(Exception):
    """Exception raised for A2A protocol errors."""

    def __init__(self, message: str, status_code: Optional[int] = None):
        super().__init__(message)
        self.status_code = status_code


class A2ATrustScoreWriteRefused(A2AError):
    """
    Raised by update_trust_score without a request: the A2A trust score has
    no write. status_code is 405, the status the server answers such a write
    with; reason is the server's reason; read_route and record_route are the
    routes it names instead.
    """

    reason = A2A_TRUST_SCORE_NOT_ASSERTED
    read_route = "GET /api/v1/a2a/trust/:id"
    record_route = "POST /api/v1/a2a/trust/:id/interaction"

    def __init__(self, target_agent_id: str):
        super().__init__(
            f"{self.reason}. Read the score of {target_agent_id} with "
            f"{self.read_route}, and record an interaction with it with "
            f"{self.record_route} (record_successful_interaction, "
            "record_failed_interaction). No request was sent.",
            status_code=405,
        )
        self.target_agent_id = target_agent_id


def _parse_datetime(value: Optional[str]) -> Optional[datetime]:
    """Parse an ISO datetime string to datetime object."""
    if not value:
        return None
    try:
        # Handle various ISO formats
        value = value.replace('Z', '+00:00')
        return datetime.fromisoformat(value)
    except:
        return None


# Convenience function for simple intent-based agent interaction
def do(intent: str, min_trust_score: float = 0.0) -> Dict[str, Any]:
    """
    Find and return an agent capable of handling a natural language intent.

    This is a convenience function for the common pattern of finding an agent
    by intent. It creates a client from environment variables and routes the intent.

    Args:
        intent: Natural language description of what you want to do
        min_trust_score: Minimum trust score required (0.0-1.0)

    Returns:
        Agent details if found, or error information

    Example:
        >>> from aim_sdk.a2a import do
        >>> result = do("send email to john@example.com")
        >>> if result.get("agent"):
        ...     agent = result["agent"]
        ...     print(f"Use {agent['agentName']} with skill {agent['skillId']}")
    """
    client = create_a2a_client_from_env()
    return client.route_by_intent(intent, min_trust_score)


# Convenience function for creating A2A client from environment
def create_a2a_client_from_env() -> A2AClient:
    """
    Create an A2A client from environment variables.

    Expected environment variables:
    - AIM_AGENT_ID: Agent ID
    - AIM_PUBLIC_KEY: Base64-encoded public key
    - AIM_PRIVATE_KEY: Base64-encoded private key
    - AIM_URL: AIM server URL

    Returns:
        Configured A2AClient

    Example:
        >>> a2a = create_a2a_client_from_env()
    """
    import os
    from .client import AIMClient

    aim = AIMClient(
        agent_id=os.environ.get("AIM_AGENT_ID", ""),
        public_key=os.environ.get("AIM_PUBLIC_KEY", ""),
        private_key=os.environ.get("AIM_PRIVATE_KEY", ""),
        aim_url=os.environ.get("AIM_URL", "http://localhost:8080"),
    )

    return A2AClient(aim)
