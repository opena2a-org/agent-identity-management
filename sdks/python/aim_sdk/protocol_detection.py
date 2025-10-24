"""
AIM SDK - Protocol Auto-Detection

This module automatically detects the communication protocol used by an agent:
- MCP (Model Context Protocol) - Agent calls MCP servers or runs as MCP server
- A2A (Agent-to-Agent) - Agent communicates with other agents
- OAuth - Agent uses OAuth for authentication
- SAML - Agent uses SAML for authentication
- DID - Agent uses Decentralized Identifiers
- ACP - Agent uses Agent Communication Protocol

Detection is based on runtime context, environment variables, and imported packages.
"""

import os
import sys
from typing import Optional, Dict, Any, List
from datetime import datetime, timezone

__version__ = "1.0.0"


class ProtocolDetector:
    """
    Auto-detector for communication protocol used by an agent.

    This class analyzes the runtime environment to determine which protocol
    the agent is using for communication.

    Example:
        from aim_sdk import ProtocolDetector

        detector = ProtocolDetector()
        protocol = detector.detect_protocol()
        print(f"Detected protocol: {protocol}")  # Output: "mcp", "a2a", etc.
    """

    def __init__(self):
        """Initialize the protocol detector."""
        self._protocol_indicators = {
            "mcp": [
                "MCP_SERVER_MODE",
                "MCP_SERVER_NAME",
                "MCP_TRANSPORT",
                "@modelcontextprotocol",
                "mcp_server"
            ],
            "a2a": [
                "A2A_AGENT_MODE",
                "AGENT_TO_AGENT",
                "A2A_ENDPOINT",
                "opena2a",
                "agent_communication"
            ],
            "oauth": [
                "OAUTH_CLIENT_ID",
                "OAUTH_CLIENT_SECRET",
                "OAUTH_TOKEN_URL",
                "OAUTH_PROVIDER"
            ],
            "saml": [
                "SAML_IDP_URL",
                "SAML_ENTITY_ID",
                "SAML_CERT",
                "SAML_SSO_URL"
            ],
            "did": [
                "DID_METHOD",
                "DID_RESOLVER",
                "DECENTRALIZED_ID"
            ],
            "acp": [
                "ACP_AGENT_ID",
                "ACP_PROTOCOL_VERSION"
            ]
        }

    def detect_protocol(self, explicit_protocol: Optional[str] = None) -> str:
        """
        Detect the communication protocol used by this agent.

        Detection precedence:
        1. Explicit protocol declaration (user override)
        2. Environment variables (strongest signal)
        3. Imported packages (MCP/A2A libraries)
        4. Default to "mcp" if no indicators found

        Args:
            explicit_protocol: User-provided protocol override

        Returns:
            Protocol name: "mcp", "a2a", "oauth", "saml", "did", "acp"
        """
        # 1. User explicitly declared protocol (highest priority)
        if explicit_protocol:
            return explicit_protocol.lower()

        # 2. Check environment variables (strong signal)
        env_protocol = self._detect_from_environment()
        if env_protocol:
            return env_protocol

        # 3. Check imported packages (weaker signal)
        import_protocol = self._detect_from_imports()
        if import_protocol:
            return import_protocol

        # 4. Default to MCP (most common for AI agents)
        return "mcp"

    def _detect_from_environment(self) -> Optional[str]:
        """
        Detect protocol from environment variables.

        Returns:
            Protocol name if detected, None otherwise
        """
        for protocol, indicators in self._protocol_indicators.items():
            for indicator in indicators:
                if indicator in os.environ:
                    return protocol

        return None

    def _detect_from_imports(self) -> Optional[str]:
        """
        Detect protocol from imported Python packages.

        Scans sys.modules to find protocol-specific imports.

        Returns:
            Protocol name if detected, None otherwise
        """
        loaded_modules = list(sys.modules.keys())

        # Check for MCP packages
        mcp_modules = [
            "mcp",
            "mcp_server",
            "mcp_client",
            "modelcontextprotocol"
        ]
        if any(mcp_mod in mod for mod in loaded_modules for mcp_mod in mcp_modules):
            return "mcp"

        # Check for A2A packages
        a2a_modules = [
            "opena2a",
            "a2a_client",
            "agent_communication"
        ]
        if any(a2a_mod in mod for mod in loaded_modules for a2a_mod in a2a_modules):
            return "a2a"

        # Check for OAuth packages
        oauth_modules = [
            "oauthlib",
            "requests_oauthlib",
            "authlib"
        ]
        if any(oauth_mod in mod for mod in loaded_modules for oauth_mod in oauth_modules):
            # Only return oauth if OAUTH env vars are present
            if any(key.startswith("OAUTH_") for key in os.environ):
                return "oauth"

        return None

    def get_detection_confidence(self, protocol: str) -> float:
        """
        Calculate confidence score for detected protocol (0-100).

        Confidence factors:
        - Explicit declaration: 100%
        - Environment variable match: 90%
        - Multiple indicators: 80%
        - Single import match: 60%
        - Default (mcp): 50%

        Args:
            protocol: Detected protocol name

        Returns:
            Confidence score (0-100)
        """
        confidence = 50.0  # Base confidence (default)

        # Check environment variables
        env_matches = sum(
            1 for indicator in self._protocol_indicators.get(protocol, [])
            if indicator in os.environ
        )
        if env_matches > 0:
            confidence = 90.0 + (env_matches - 1) * 2  # 90% + bonus for multiple matches

        # Check imports
        loaded_modules = list(sys.modules.keys())
        import_matches = sum(
            1 for indicator in self._protocol_indicators.get(protocol, [])
            if any(indicator.lower() in mod.lower() for mod in loaded_modules)
        )
        if import_matches > 0 and confidence < 70:
            confidence = 60.0 + (import_matches - 1) * 5  # 60% + bonus for multiple matches

        return min(confidence, 100.0)  # Cap at 100%

    def get_protocol_details(self, protocol: str) -> Dict[str, Any]:
        """
        Get detailed information about the detected protocol.

        Args:
            protocol: Protocol name

        Returns:
            Dict with protocol details including indicators found
        """
        indicators_found = []

        # Check environment variables
        for indicator in self._protocol_indicators.get(protocol, []):
            if indicator in os.environ:
                indicators_found.append({
                    "type": "environment",
                    "indicator": indicator,
                    "value": os.environ[indicator][:50]  # Truncate for security
                })

        # Check imports
        loaded_modules = list(sys.modules.keys())
        for indicator in self._protocol_indicators.get(protocol, []):
            matching_modules = [
                mod for mod in loaded_modules
                if indicator.lower() in mod.lower()
            ]
            if matching_modules:
                indicators_found.append({
                    "type": "import",
                    "indicator": indicator,
                    "modules": matching_modules[:5]  # Limit to first 5 matches
                })

        return {
            "protocol": protocol,
            "confidence": self.get_detection_confidence(protocol),
            "indicators_found": indicators_found,
            "detected_at": datetime.now(timezone.utc).isoformat()
        }


def auto_detect_protocol(explicit_protocol: Optional[str] = None) -> str:
    """
    Convenience function for quick protocol detection.

    Args:
        explicit_protocol: Optional user-provided protocol override

    Returns:
        Protocol name: "mcp", "a2a", "oauth", "saml", "did", "acp"

    Example:
        from aim_sdk import auto_detect_protocol

        protocol = auto_detect_protocol()
        print(f"Using protocol: {protocol}")
    """
    detector = ProtocolDetector()
    return detector.detect_protocol(explicit_protocol)
