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
from dataclasses import dataclass, field
from typing import Any, Dict, List, Optional, Union
from datetime import datetime, timezone

import requests
from nacl.signing import SigningKey
from nacl.encoding import Base64Encoder


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
    a2a_trust_score: float
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
        self._session.headers.update({
            "Content-Type": "application/json",
            "User-Agent": f"AIM-Python-SDK/A2A-1.0.0",
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
            # Use AIM client's authentication
            if self.aim_client.signing_key and self.aim_client.public_key:
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
            a2a_trust_score=response.get("a2aTrustScore", 0.0),
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
            a2a_trust_score=response.get("a2aTrustScore", 0.0),
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

    def get_security_settings(self, organization_id: str) -> Dict[str, Any]:
        """
        Get A2A security settings for an organization.

        Args:
            organization_id: Organization ID

        Returns:
            Security settings including enforcement mode and requirements

        Example:
            >>> settings = a2a.get_security_settings("org-id")
            >>> print(f"Mode: {settings['enforcementMode']}")  # 'monitor' or 'strict'
        """
        return self._make_request(
            "GET",
            f"/api/v1/a2a/security/settings/{organization_id}"
        )

    def get_security_violations(
        self,
        organization_id: str,
        limit: int = 50,
        offset: int = 0
    ) -> List[Dict[str, Any]]:
        """
        Get security violations for an organization.

        Args:
            organization_id: Organization ID
            limit: Maximum number of results
            offset: Pagination offset

        Returns:
            List of security violations

        Example:
            >>> violations = a2a.get_security_violations("org-id")
            >>> for v in violations:
            ...     print(f"{v['violationType']}: {v['actionTaken']}")
        """
        response = self._make_request(
            "GET",
            f"/api/v1/a2a/security/violations/{organization_id}?limit={limit}&offset={offset}"
        )
        return response.get("violations", [])

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


class A2AError(Exception):
    """Exception raised for A2A protocol errors."""

    def __init__(self, message: str, status_code: Optional[int] = None):
        super().__init__(message)
        self.status_code = status_code


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
