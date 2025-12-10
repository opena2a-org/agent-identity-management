"""
Attestation Cache System for Smart MCP Attestation.

Tracks attestation state to enable intelligent re-attestation decisions,
supporting AIM's goal as the industry MCP supply chain security platform.

Features:
- Persistent cache storage for attestation state
- Smart attestation triggers (first use, new tool, stale, drift)
- Capability drift detection with severity levels
- Tool usage analytics for supply chain visibility
"""

import json
import os
from datetime import datetime, timedelta
from pathlib import Path
from typing import Any, Dict, List, Optional, Set

from aim_sdk.console import console


class AttestationCache:
    """
    Tracks attestation state for smart re-attestation decisions.

    Storage: ~/.aim/attestation_cache.json

    Cache entry structure per agent:
    {
        "agentId": "uuid",
        "mcpServers": {
            "mcp_server_id": {
                "lastAttestedAt": "2025-12-10T13:30:00Z",
                "attestationId": "uuid",
                "capabilitiesAttested": ["read_file", "write_file", ...],
                "toolsUsed": {
                    "read_file": {"count": 5, "firstUsed": "...", "lastUsed": "..."},
                    "write_file": {"count": 2, "firstUsed": "...", "lastUsed": "..."}
                },
                "firstSeenAt": "2025-12-01T00:00:00Z",
                "confidenceScore": 85.5
            }
        }
    }
    """

    DEFAULT_TTL_HOURS = 24
    CACHE_FILENAME = "attestation_cache.json"

    def __init__(
        self,
        agent_id: str,
        cache_dir: Optional[str] = None,
        ttl_hours: int = DEFAULT_TTL_HOURS
    ):
        """
        Initialize the attestation cache.

        Args:
            agent_id: The agent ID this cache belongs to
            cache_dir: Directory to store cache file (default: ~/.aim)
            ttl_hours: Hours before attestation is considered stale
        """
        self.agent_id = agent_id
        self.ttl_hours = ttl_hours

        if cache_dir:
            self._cache_dir = Path(cache_dir)
        else:
            self._cache_dir = Path.home() / ".aim"

        self._cache_file = self._cache_dir / self.CACHE_FILENAME
        self._cache: Dict[str, Any] = self._load_cache()

    def _load_cache(self) -> Dict[str, Any]:
        """Load cache from disk or initialize empty cache."""
        if self._cache_file.exists():
            try:
                with open(self._cache_file, "r") as f:
                    all_caches = json.load(f)
                    # Get cache for this specific agent
                    if self.agent_id in all_caches:
                        return all_caches[self.agent_id]
            except (json.JSONDecodeError, IOError) as e:
                console.warning(f"Failed to load attestation cache: {e}")

        # Return empty cache structure
        return {
            "agentId": self.agent_id,
            "mcpServers": {}
        }

    def _save_cache(self) -> None:
        """Persist cache to disk."""
        try:
            # Ensure directory exists with secure permissions
            self._cache_dir.mkdir(parents=True, exist_ok=True)

            # Load all agent caches
            all_caches = {}
            if self._cache_file.exists():
                try:
                    with open(self._cache_file, "r") as f:
                        all_caches = json.load(f)
                except (json.JSONDecodeError, IOError):
                    pass

            # Update this agent's cache
            all_caches[self.agent_id] = self._cache

            # Write with secure permissions
            with open(self._cache_file, "w") as f:
                json.dump(all_caches, f, indent=2, default=str)

            # Set file permissions (0600 - owner read/write only)
            os.chmod(self._cache_file, 0o600)

        except IOError as e:
            console.warning(f"Failed to save attestation cache: {e}")

    def should_attest(
        self,
        mcp_server_id: str,
        tool_name: Optional[str] = None,
        current_capabilities: Optional[List[str]] = None
    ) -> Dict[str, Any]:
        """
        Determine if attestation is needed for an MCP server/tool.

        Attestation triggers:
        1. First use of an MCP server by this agent
        2. First use of a NEW tool on a known server
        3. Attestation is stale (>TTL hours old)
        4. Capability drift detected (tools added/removed)

        Args:
            mcp_server_id: The MCP server ID
            tool_name: Optional tool being used
            current_capabilities: Optional list of current capabilities for drift detection

        Returns:
            {
                "shouldAttest": bool,
                "reason": str,
                "driftInfo": {...} or None
            }
        """
        server_cache = self._cache.get("mcpServers", {}).get(mcp_server_id)

        # Trigger 1: First use of server
        if not server_cache:
            return {
                "shouldAttest": True,
                "reason": "first_use",
                "driftInfo": None
            }

        # Trigger 2: First use of a new tool
        if tool_name:
            tools_used = server_cache.get("toolsUsed", {})
            capabilities_attested = set(server_cache.get("capabilitiesAttested", []))

            if tool_name not in tools_used and tool_name not in capabilities_attested:
                return {
                    "shouldAttest": True,
                    "reason": "new_tool",
                    "driftInfo": None
                }

        # Trigger 3: Stale attestation
        last_attested = server_cache.get("lastAttestedAt")
        if last_attested:
            try:
                last_time = datetime.fromisoformat(last_attested.replace("Z", "+00:00"))
                stale_threshold = datetime.now(last_time.tzinfo) - timedelta(hours=self.ttl_hours)

                if last_time < stale_threshold:
                    return {
                        "shouldAttest": True,
                        "reason": "stale",
                        "driftInfo": None
                    }
            except (ValueError, TypeError):
                # Invalid timestamp, trigger re-attestation
                return {
                    "shouldAttest": True,
                    "reason": "invalid_timestamp",
                    "driftInfo": None
                }

        # Trigger 4: Capability drift
        if current_capabilities:
            drift_info = self.detect_capability_drift(mcp_server_id, current_capabilities)
            if drift_info.get("driftDetected"):
                return {
                    "shouldAttest": True,
                    "reason": "capability_drift",
                    "driftInfo": drift_info
                }

        return {
            "shouldAttest": False,
            "reason": "cached",
            "driftInfo": None
        }

    def detect_capability_drift(
        self,
        mcp_server_id: str,
        current_capabilities: List[str]
    ) -> Dict[str, Any]:
        """
        Detect if MCP server capabilities have changed since last attestation.

        Severity levels:
        - low: New tools added (expansion) - generally safe
        - medium: Tools removed (contraction) - potential concern
        - high: >30% change or core tools removed - security alert

        Args:
            mcp_server_id: The MCP server ID
            current_capabilities: List of currently discovered capabilities

        Returns:
            {
                "driftDetected": bool,
                "addedCapabilities": ["new_tool_1", ...],
                "removedCapabilities": ["old_tool", ...],
                "severity": "low" | "medium" | "high",
                "changePercentage": float
            }
        """
        server_cache = self._cache.get("mcpServers", {}).get(mcp_server_id)

        if not server_cache:
            return {
                "driftDetected": False,
                "addedCapabilities": [],
                "removedCapabilities": [],
                "severity": "low",
                "changePercentage": 0.0
            }

        previous_caps = set(server_cache.get("capabilitiesAttested", []))
        current_caps = set(current_capabilities)

        added = list(current_caps - previous_caps)
        removed = list(previous_caps - current_caps)

        if not added and not removed:
            return {
                "driftDetected": False,
                "addedCapabilities": [],
                "removedCapabilities": [],
                "severity": "low",
                "changePercentage": 0.0
            }

        # Calculate change percentage
        total_original = len(previous_caps) if previous_caps else 1
        change_count = len(added) + len(removed)
        change_percentage = (change_count / total_original) * 100

        # Determine severity
        severity = "low"
        if removed:
            severity = "medium"
        if change_percentage > 30:
            severity = "high"

        return {
            "driftDetected": True,
            "addedCapabilities": added,
            "removedCapabilities": removed,
            "severity": severity,
            "changePercentage": round(change_percentage, 2)
        }

    def record_attestation(
        self,
        mcp_server_id: str,
        attestation_id: str,
        capabilities: List[str],
        confidence_score: Optional[float] = None
    ) -> None:
        """
        Record a successful attestation in the cache.

        Args:
            mcp_server_id: The MCP server ID
            attestation_id: The attestation ID returned by backend
            capabilities: List of capabilities that were attested
            confidence_score: Optional confidence score from backend
        """
        now = datetime.utcnow().isoformat() + "Z"

        if "mcpServers" not in self._cache:
            self._cache["mcpServers"] = {}

        server_cache = self._cache["mcpServers"].get(mcp_server_id, {})

        # Preserve existing tool usage data
        existing_tool_usage = server_cache.get("toolsUsed", {})
        first_seen = server_cache.get("firstSeenAt", now)

        self._cache["mcpServers"][mcp_server_id] = {
            "lastAttestedAt": now,
            "attestationId": attestation_id,
            "capabilitiesAttested": capabilities,
            "toolsUsed": existing_tool_usage,
            "firstSeenAt": first_seen,
            "confidenceScore": confidence_score
        }

        self._save_cache()

    def record_tool_usage(
        self,
        mcp_server_id: str,
        tool_name: str
    ) -> Dict[str, Any]:
        """
        Record a tool usage event for supply chain analytics.

        Args:
            mcp_server_id: The MCP server ID
            tool_name: The tool that was used

        Returns:
            Updated tool usage stats
        """
        now = datetime.utcnow().isoformat() + "Z"

        if "mcpServers" not in self._cache:
            self._cache["mcpServers"] = {}

        if mcp_server_id not in self._cache["mcpServers"]:
            self._cache["mcpServers"][mcp_server_id] = {
                "firstSeenAt": now,
                "toolsUsed": {}
            }

        server_cache = self._cache["mcpServers"][mcp_server_id]

        if "toolsUsed" not in server_cache:
            server_cache["toolsUsed"] = {}

        tool_stats = server_cache["toolsUsed"].get(tool_name, {
            "count": 0,
            "firstUsed": now,
            "lastUsed": now
        })

        tool_stats["count"] = tool_stats.get("count", 0) + 1
        tool_stats["lastUsed"] = now

        server_cache["toolsUsed"][tool_name] = tool_stats

        self._save_cache()

        return tool_stats

    def get_server_stats(self, mcp_server_id: str) -> Optional[Dict[str, Any]]:
        """
        Get attestation and usage statistics for an MCP server.

        Args:
            mcp_server_id: The MCP server ID

        Returns:
            Server cache entry or None if not found
        """
        return self._cache.get("mcpServers", {}).get(mcp_server_id)

    def get_all_stats(self) -> Dict[str, Any]:
        """
        Get all attestation and usage statistics.

        Returns:
            Complete cache for this agent
        """
        return self._cache

    def get_supply_chain_report(self) -> Dict[str, Any]:
        """
        Generate a supply chain analytics report.

        Returns:
            {
                "agentId": str,
                "mcpServerCount": int,
                "totalToolsUsed": int,
                "totalToolInvocations": int,
                "servers": {
                    "server_id": {
                        "attestationAge": "2h 30m",
                        "toolCount": int,
                        "invocationCount": int,
                        "topTools": [("tool_name", count), ...]
                    }
                },
                "generatedAt": str
            }
        """
        servers_data = self._cache.get("mcpServers", {})

        report = {
            "agentId": self.agent_id,
            "mcpServerCount": len(servers_data),
            "totalToolsUsed": 0,
            "totalToolInvocations": 0,
            "servers": {},
            "generatedAt": datetime.utcnow().isoformat() + "Z"
        }

        for server_id, server_cache in servers_data.items():
            tools_used = server_cache.get("toolsUsed", {})
            tool_count = len(tools_used)

            invocation_count = sum(
                t.get("count", 0) for t in tools_used.values()
            )

            # Sort tools by usage count
            top_tools = sorted(
                [(name, stats.get("count", 0)) for name, stats in tools_used.items()],
                key=lambda x: x[1],
                reverse=True
            )[:5]  # Top 5 tools

            # Calculate attestation age
            attestation_age = "never"
            last_attested = server_cache.get("lastAttestedAt")
            if last_attested:
                try:
                    last_time = datetime.fromisoformat(last_attested.replace("Z", "+00:00"))
                    age = datetime.now(last_time.tzinfo) - last_time
                    hours, remainder = divmod(age.total_seconds(), 3600)
                    minutes = remainder // 60
                    attestation_age = f"{int(hours)}h {int(minutes)}m"
                except (ValueError, TypeError):
                    pass

            report["servers"][server_id] = {
                "attestationAge": attestation_age,
                "toolCount": tool_count,
                "invocationCount": invocation_count,
                "topTools": top_tools,
                "confidenceScore": server_cache.get("confidenceScore")
            }

            report["totalToolsUsed"] += tool_count
            report["totalToolInvocations"] += invocation_count

        return report

    def clear_server(self, mcp_server_id: str) -> bool:
        """
        Clear cached data for a specific MCP server.

        Args:
            mcp_server_id: The MCP server ID to clear

        Returns:
            True if server was cleared, False if not found
        """
        if mcp_server_id in self._cache.get("mcpServers", {}):
            del self._cache["mcpServers"][mcp_server_id]
            self._save_cache()
            return True
        return False

    def clear_all(self) -> None:
        """Clear all cached attestation data for this agent."""
        self._cache = {
            "agentId": self.agent_id,
            "mcpServers": {}
        }
        self._save_cache()
