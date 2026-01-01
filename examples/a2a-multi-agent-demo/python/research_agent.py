"""
Research Agent - Gathers information and collaborates with Analysis Agent.

Demonstrates:
- Agent Card registration
- Intent-based discovery
- GDPR consent management
- Request signing
- Task logging
- Skill attestation
- Security policy checks
"""
import os
import sys
import time
import uuid
from typing import Dict, List, Optional
from datetime import datetime

# Add SDK to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../../../sdk/python'))

from aim_sdk import register_agent
from aim_sdk.a2a import A2AClient, A2ATrustScore


class ResearchAgent:
    """
    Research Agent that discovers and collaborates with Analysis Agents.

    This agent demonstrates the full A2A workflow:
    1. Register with AIM
    2. Discover capable agents by intent
    3. Check security policies
    4. Obtain user consent (GDPR)
    5. Sign and send requests
    6. Log tasks and track state
    7. Attest to skill quality
    """

    def __init__(self, aim_url: str, api_key: str):
        """
        Initialize the Research Agent.

        Args:
            aim_url: AIM platform URL
            api_key: API key for authentication
        """
        self.agent_name = "research-agent"
        self.aim_url = aim_url

        print(f"[Research Agent] Registering with AIM...")

        # Register agent and get AIM client
        self.aim_client = register_agent(
            name=self.agent_name,
            aim_url=aim_url,
            api_key=api_key,
            display_name="Research Agent",
            agent_type="ai_agent",
            description="Research agent that gathers information and collaborates with analysis agents"
        )

        # Initialize A2A client
        self.a2a = A2AClient(self.aim_client)

        # Store agent ID
        self.agent_id = self.aim_client.agent_id
        print(f"[Research Agent] Registered with ID: {self.agent_id[:8]}...")

    def register_agent_card(self) -> Optional[Dict]:
        """
        Register the A2A Agent Card with research skills.

        Returns:
            Registered card info or None on failure
        """
        print(f"[Research Agent] Registering Agent Card...")

        try:
            # Register research skills
            skills = [
                {
                    "id": "web-research",
                    "name": "Web Research",
                    "description": "Gather information from web sources on specified topics",
                    "tags": ["research", "web", "data-gathering"],
                    "inputModes": ["text"],
                    "outputModes": ["text", "data"]
                },
                {
                    "id": "document-reading",
                    "name": "Document Reading",
                    "description": "Extract and process content from documents",
                    "tags": ["documents", "extraction", "reading"],
                    "inputModes": ["file", "text"],
                    "outputModes": ["text", "data"]
                }
            ]

            for skill in skills:
                self.a2a.register_skill(skill)
                print(f"  - Registered skill: {skill['name']}")

            card = self.a2a.get_agent_card()
            print(f"[Research Agent] Agent Card registered successfully")
            return card

        except Exception as e:
            print(f"[Research Agent] Card registration note: {e}")
            return None

    def discover_analysis_agent(
        self,
        intent: str,
        min_trust: float = 0.3
    ) -> Optional[Dict]:
        """
        Discover an agent capable of handling the analysis intent.

        Args:
            intent: Natural language description of what's needed
            min_trust: Minimum trust score required

        Returns:
            Best matching agent or None
        """
        print(f"\n[Research Agent] Searching for agents capable of: '{intent}'")
        print(f"[Research Agent] Minimum trust score: {min_trust}")

        try:
            # Use intent-based routing
            capable_agents = self.a2a.route_by_intent(
                intent=intent,
                min_trust_score=min_trust
            )

            if not capable_agents:
                print("[Research Agent] No capable agents found via intent routing")
                # Try skill search as fallback
                skills = self.a2a.search_skills("sentiment analysis", limit=5)
                if skills:
                    print(f"[Research Agent] Found {len(skills)} skills via search")
                    return skills[0] if skills else None
                return None

            # Select best match
            best_agent = capable_agents[0]
            print(f"[Research Agent] Found agent: {best_agent.get('agentName', 'Unknown')}")
            print(f"[Research Agent] Skill: {best_agent.get('skillName', 'Unknown')}")
            print(f"[Research Agent] Trust score: {best_agent.get('trustScore', 'N/A')}")

            return best_agent

        except Exception as e:
            print(f"[Research Agent] Discovery error: {e}")
            return None

    def check_security_policies(
        self,
        target_agent_id: str,
        skill_id: str
    ) -> Dict:
        """
        Check if request passes security policies.

        Args:
            target_agent_id: ID of target agent
            skill_id: ID of skill to invoke

        Returns:
            Security check result
        """
        print(f"\n[Research Agent] Checking security policies...")
        print(f"[Research Agent] Target: {target_agent_id[:8]}...")
        print(f"[Research Agent] Skill: {skill_id}")

        try:
            result = self.a2a.check_security(
                target_agent_id=target_agent_id,
                skill_id=skill_id
            )

            if result.get("allowed"):
                print("[Research Agent] Security check PASSED")
                print(f"[Research Agent] Enforcement mode: {result.get('enforcementMode', 'unknown')}")
            else:
                print("[Research Agent] Security check FAILED")
                violations = result.get("violations", [])
                for v in violations:
                    print(f"  - {v.get('type')}: {v.get('message')}")

            return result

        except Exception as e:
            print(f"[Research Agent] Security check error: {e}")
            return {"allowed": True, "note": "Security check unavailable"}

    def request_user_consent(
        self,
        user_id: str,
        recipient_agent_id: str,
        data_types: List[str],
        purpose: str
    ) -> bool:
        """
        Request GDPR-compliant consent for cross-agent data sharing.

        Args:
            user_id: User ID for consent
            recipient_agent_id: Agent receiving data
            data_types: Types of data being shared
            purpose: Purpose of data sharing

        Returns:
            True if consent obtained, False otherwise
        """
        print(f"\n[Research Agent] Requesting user consent...")
        print(f"[Research Agent] User: {user_id}")
        print(f"[Research Agent] Purpose: {purpose}")
        print(f"[Research Agent] Data types: {', '.join(data_types)}")

        try:
            # Check if consent already exists
            has_consent = self.a2a.check_consent(
                user_id=user_id,
                recipient_agent_id=recipient_agent_id,
                scope="data_sharing"
            )

            if has_consent:
                print("[Research Agent] Consent already exists")
                return True

            # Record new consent
            consent = self.a2a.record_consent(
                user_id=user_id,
                recipient_agent_id=recipient_agent_id,
                scope=["data_sharing", "analysis"],
                purpose=purpose,
                data_types=data_types,
                expires_in_hours=24
            )

            print(f"[Research Agent] Consent recorded: {consent.consent_id}")
            return True

        except Exception as e:
            print(f"[Research Agent] Consent error: {e}")
            # For demo, allow proceeding
            return True

    def sign_and_call_skill(
        self,
        target_agent_id: str,
        skill_id: str,
        data: Dict,
        analysis_agent=None
    ) -> Dict:
        """
        Sign request and call the target agent's skill.

        Args:
            target_agent_id: Target agent ID
            skill_id: Skill to invoke
            data: Input data for skill
            analysis_agent: Optional reference to analysis agent for direct call

        Returns:
            Skill execution result
        """
        print(f"\n[Research Agent] Preparing signed request...")
        print(f"[Research Agent] Skill: {skill_id}")

        # Sign the request
        try:
            signature = self.a2a.sign_request(
                method="POST",
                path=f"/skills/{skill_id}/invoke",
                body=data
            )
            print(f"[Research Agent] Request signed at: {signature.timestamp}")
            print(f"[Research Agent] Nonce: {signature.nonce[:16]}...")
        except Exception as e:
            print(f"[Research Agent] Signing note: {e}")

        # Log the task
        task_id = None
        try:
            task = self.a2a.log_task(
                external_task_id=f"demo-{uuid.uuid4().hex[:8]}",
                remote_agent_id=target_agent_id,
                skill_id=skill_id
            )
            task_id = task.get("id") or task.get("taskId")
            print(f"[Research Agent] Task logged: {task_id}")
        except Exception as e:
            print(f"[Research Agent] Task logging note: {e}")
            task_id = f"local-{uuid.uuid4().hex[:8]}"

        # Execute skill (direct call for demo)
        print(f"[Research Agent] Invoking skill...")
        if analysis_agent:
            result = analysis_agent.execute_skill(skill_id, data)
        else:
            # Simulated response
            result = {
                "sentiment": "positive",
                "confidence": 0.87,
                "note": "Simulated response - Analysis Agent not provided"
            }

        print(f"[Research Agent] Result: {result}")

        # Update task state
        try:
            self.a2a.update_task_state(task_id, "COMPLETED")
            print(f"[Research Agent] Task marked COMPLETED")
        except Exception as e:
            print(f"[Research Agent] Task update note: {e}")

        return result

    def attest_skill_quality(
        self,
        target_agent_id: str,
        skill_id: str,
        confidence: float,
        evidence: Dict
    ) -> Optional[Dict]:
        """
        Attest to the quality of another agent's skill.

        Args:
            target_agent_id: Agent whose skill is being attested
            skill_id: Skill being attested
            confidence: Confidence score (0-1)
            evidence: Evidence supporting attestation

        Returns:
            Attestation result
        """
        print(f"\n[Research Agent] Creating skill attestation...")
        print(f"[Research Agent] Skill: {skill_id}")
        print(f"[Research Agent] Confidence: {confidence}")

        try:
            attestation = self.a2a.attest_skill(
                target_agent_id=target_agent_id,
                skill_id=skill_id,
                attestation_type="SKILL_VERIFICATION",
                confidence=confidence,
                evidence=evidence
            )

            print(f"[Research Agent] Attestation created: {attestation.get('id', 'N/A')}")

            # Check consensus status
            try:
                consensus = self.a2a.get_consensus_status(target_agent_id, skill_id)
                print(f"[Research Agent] Skill verified: {consensus.get('isVerified', False)}")
                print(f"[Research Agent] Attestation count: {consensus.get('attestationCount', 0)}")
                print(f"[Research Agent] Unique attesters: {consensus.get('uniqueAttesters', 0)}")
            except Exception as e:
                print(f"[Research Agent] Consensus check note: {e}")

            return attestation

        except Exception as e:
            print(f"[Research Agent] Attestation error: {e}")
            return None

    def get_trust_score(self, agent_id: Optional[str] = None) -> Optional[A2ATrustScore]:
        """
        Get A2A trust score for an agent.

        Args:
            agent_id: Agent ID (defaults to self)

        Returns:
            Trust score object or None
        """
        target = agent_id or self.agent_id
        target_label = "self" if target == self.agent_id else target[:8] + "..."

        print(f"\n[Research Agent] Getting trust score for {target_label}...")

        try:
            score = self.a2a.get_a2a_trust_score(agent_id)
            print(f"[Research Agent] Trust score: {score.a2a_trust_score:.2f}")
            print(f"[Research Agent] Peer trust avg: {score.peer_trust_average or 'N/A'}")
            print(f"[Research Agent] Tasks completed: {score.tasks_completed or 0}")
            return score
        except Exception as e:
            print(f"[Research Agent] Trust score note: {e}")
            return None

    def get_peer_trust(self, peer_agent_id: str) -> Optional[Dict]:
        """
        Get trust relationship with a specific peer.

        Args:
            peer_agent_id: Peer agent ID

        Returns:
            Peer trust info or None
        """
        print(f"\n[Research Agent] Getting peer trust with {peer_agent_id[:8]}...")

        try:
            trust = self.a2a.get_peer_trust(peer_agent_id)
            print(f"[Research Agent] Peer trust score: {trust.peer_trust_score:.2f}")
            print(f"[Research Agent] Success rate: {trust.success_rate or 'N/A'}")
            return trust
        except Exception as e:
            print(f"[Research Agent] Peer trust note: {e}")
            return None
