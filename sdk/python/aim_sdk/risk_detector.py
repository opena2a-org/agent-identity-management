"""
AIM SDK - Automatic Risk Level Detection

Automatically determines risk levels from capability strings based on
well-established patterns. Developers can still override when needed.

Usage:
    from aim_sdk.risk_detector import detect_risk_level

    # Auto-detect from capability
    risk = detect_risk_level("weather:fetch")     # Returns "low"
    risk = detect_risk_level("db:delete")         # Returns "high"
    risk = detect_risk_level("payment:process")   # Returns "critical"

    # Override when you know better
    risk = detect_risk_level("api:internal", override="critical")
"""

from typing import Optional


# Risk level constants
RISK_LOW = "low"
RISK_MEDIUM = "medium"
RISK_HIGH = "high"
RISK_CRITICAL = "critical"


# Default risk level when no pattern matches
DEFAULT_RISK_LEVEL = RISK_MEDIUM


# Namespace-based risk mappings (prefix patterns)
# More specific namespaces that indicate inherent risk
NAMESPACE_RISK_MAP = {
    # Critical risk namespaces - financial, admin, system
    "payment": RISK_CRITICAL,
    "admin": RISK_CRITICAL,
    "system": RISK_CRITICAL,
    "billing": RISK_CRITICAL,
    "finance": RISK_CRITICAL,

    # High risk namespaces - user data, external comms
    "email": RISK_HIGH,
    "notification": RISK_HIGH,
    "sms": RISK_HIGH,
    "user": RISK_HIGH,  # Default for user operations
    "auth": RISK_HIGH,
    "secret": RISK_HIGH,
    "credential": RISK_HIGH,

    # Medium risk namespaces - data access, file system
    "db": RISK_MEDIUM,
    "database": RISK_MEDIUM,
    "file": RISK_MEDIUM,
    "storage": RISK_MEDIUM,
    "cache": RISK_MEDIUM,

    # Low risk namespaces - public data, read-only services
    "api": RISK_LOW,
    "weather": RISK_LOW,
    "search": RISK_LOW,
    "geocode": RISK_LOW,
    "translate": RISK_LOW,
    "time": RISK_LOW,
    "math": RISK_LOW,
    "util": RISK_LOW,
}


# Action-based risk mappings (suffix patterns)
# Actions that indicate inherent risk regardless of namespace
ACTION_RISK_MAP = {
    # Low risk actions - read-only, no side effects
    "read": RISK_LOW,
    "fetch": RISK_LOW,
    "get": RISK_LOW,
    "list": RISK_LOW,
    "search": RISK_LOW,
    "query": RISK_LOW,
    "view": RISK_LOW,
    "check": RISK_LOW,
    "validate": RISK_LOW,
    "verify": RISK_LOW,
    "count": RISK_LOW,
    "exists": RISK_LOW,

    # Medium risk actions - modifications, creates
    "write": RISK_MEDIUM,
    "update": RISK_MEDIUM,
    "create": RISK_MEDIUM,
    "modify": RISK_MEDIUM,
    "edit": RISK_MEDIUM,
    "set": RISK_MEDIUM,
    "save": RISK_MEDIUM,
    "upload": RISK_MEDIUM,
    "import": RISK_MEDIUM,

    # High risk actions - destructive, external comms, sensitive
    "delete": RISK_HIGH,
    "remove": RISK_HIGH,
    "send": RISK_HIGH,
    "execute": RISK_HIGH,
    "run": RISK_HIGH,
    "invoke": RISK_HIGH,
    "call": RISK_HIGH,  # External calls
    "export": RISK_HIGH,
    "download": RISK_HIGH,
    "transfer": RISK_HIGH,
    "move": RISK_HIGH,
    "revoke": RISK_HIGH,
    "suspend": RISK_HIGH,
    "disable": RISK_HIGH,
    "block": RISK_HIGH,

    # Critical risk actions - irreversible, financial, admin
    "process": RISK_CRITICAL,  # Usually financial
    "refund": RISK_CRITICAL,
    "charge": RISK_CRITICAL,
    "pay": RISK_CRITICAL,
    "approve": RISK_CRITICAL,
    "admin": RISK_CRITICAL,
    "impersonate": RISK_CRITICAL,
    "escalate": RISK_CRITICAL,
    "purge": RISK_CRITICAL,
    "destroy": RISK_CRITICAL,
    "drop": RISK_CRITICAL,
    "truncate": RISK_CRITICAL,
    "wipe": RISK_CRITICAL,
    "terminate": RISK_CRITICAL,
}


# Specific capability overrides for common patterns
# Format: "namespace:action" -> risk_level
# These take highest precedence
SPECIFIC_CAPABILITY_MAP = {
    # User operations - escalate delete/impersonate to critical
    "user:delete": RISK_CRITICAL,
    "user:impersonate": RISK_CRITICAL,
    "user:admin": RISK_CRITICAL,

    # Auth operations - sensitive
    "auth:login": RISK_MEDIUM,
    "auth:logout": RISK_LOW,
    "auth:reset_password": RISK_HIGH,
    "auth:change_password": RISK_HIGH,

    # API calls - generally low unless external
    "api:call": RISK_LOW,
    "api:webhook": RISK_MEDIUM,
    "api:external": RISK_HIGH,

    # Database - read is low, others escalate
    "db:read": RISK_LOW,
    "db:query": RISK_LOW,
    "db:write": RISK_MEDIUM,
    "db:delete": RISK_HIGH,
    "db:admin": RISK_CRITICAL,
    "db:drop": RISK_CRITICAL,

    # File operations
    "file:read": RISK_LOW,
    "file:write": RISK_MEDIUM,
    "file:delete": RISK_HIGH,
    "file:execute": RISK_CRITICAL,

    # Network operations
    "network:read": RISK_LOW,
    "network:connect": RISK_MEDIUM,
    "network:listen": RISK_HIGH,
    "network:external": RISK_HIGH,

    # MCP operations
    "mcp:tool_use": RISK_MEDIUM,
    "mcp:resource_read": RISK_LOW,

    # System operations - all high/critical
    "system:read": RISK_MEDIUM,
    "system:exec": RISK_CRITICAL,
    "system:admin": RISK_CRITICAL,
}


def detect_risk_level(capability: str, override: Optional[str] = None) -> str:
    """
    Automatically detect the risk level of a capability based on its name.

    The detection follows this priority order:
    1. Explicit override (if provided)
    2. Specific capability mapping (e.g., "user:delete" -> critical)
    3. Action-based mapping (e.g., ":delete" -> high)
    4. Namespace-based mapping (e.g., "payment:" -> critical)
    5. Default to "medium"

    Args:
        capability: The capability string in format "namespace:action"
        override: Optional explicit risk level that takes highest precedence

    Returns:
        Risk level: "low", "medium", "high", or "critical"

    Examples:
        >>> detect_risk_level("weather:fetch")
        'low'
        >>> detect_risk_level("db:read")
        'low'
        >>> detect_risk_level("db:delete")
        'high'
        >>> detect_risk_level("payment:process")
        'critical'
        >>> detect_risk_level("user:delete")
        'critical'
        >>> detect_risk_level("api:internal", override="critical")
        'critical'
    """
    # Priority 1: Explicit override always wins
    if override is not None:
        if override in (RISK_LOW, RISK_MEDIUM, RISK_HIGH, RISK_CRITICAL):
            return override
        # Invalid override, fall through to detection

    # Normalize capability to lowercase
    capability = capability.lower().strip()

    # Priority 2: Check specific capability map
    if capability in SPECIFIC_CAPABILITY_MAP:
        return SPECIFIC_CAPABILITY_MAP[capability]

    # Parse namespace and action
    if ":" not in capability:
        return DEFAULT_RISK_LEVEL

    parts = capability.split(":", 1)
    if len(parts) != 2:
        return DEFAULT_RISK_LEVEL

    namespace, action = parts

    # Priority 3: Check action-based mapping (actions are more specific)
    # This catches patterns like "custom:delete" -> high
    if action in ACTION_RISK_MAP:
        action_risk = ACTION_RISK_MAP[action]

        # If namespace has higher inherent risk, use that instead
        namespace_risk = NAMESPACE_RISK_MAP.get(namespace)
        if namespace_risk:
            return _higher_risk(action_risk, namespace_risk)

        return action_risk

    # Priority 4: Check namespace-based mapping
    if namespace in NAMESPACE_RISK_MAP:
        return NAMESPACE_RISK_MAP[namespace]

    # Priority 5: Default
    return DEFAULT_RISK_LEVEL


def _higher_risk(risk1: str, risk2: str) -> str:
    """Return the higher of two risk levels."""
    risk_order = {
        RISK_LOW: 0,
        RISK_MEDIUM: 1,
        RISK_HIGH: 2,
        RISK_CRITICAL: 3,
    }

    r1 = risk_order.get(risk1, 1)
    r2 = risk_order.get(risk2, 1)

    if r1 >= r2:
        return risk1
    return risk2


def get_risk_level_description(risk_level: str) -> str:
    """Get a human-readable description of what a risk level means."""
    descriptions = {
        RISK_LOW: "Logged for audit trail. Minimal monitoring.",
        RISK_MEDIUM: "Logged with pattern analysis. Monitored for unusual behavior.",
        RISK_HIGH: "Logged with alerts. Closely monitored. May affect trust score.",
        RISK_CRITICAL: "Logged with alerts. May require admin approval (JIT access).",
    }
    return descriptions.get(risk_level, "Unknown risk level")


def explain_risk_detection(capability: str) -> dict:
    """
    Explain why a capability was assigned a particular risk level.
    Useful for debugging and documentation.

    Args:
        capability: The capability string

    Returns:
        Dict with risk level, reason, and recommendations
    """
    capability = capability.lower().strip()

    # Check specific mapping
    if capability in SPECIFIC_CAPABILITY_MAP:
        return {
            "capability": capability,
            "risk_level": SPECIFIC_CAPABILITY_MAP[capability],
            "reason": f"Specific mapping for '{capability}'",
            "source": "specific_capability_map",
        }

    if ":" not in capability:
        return {
            "capability": capability,
            "risk_level": DEFAULT_RISK_LEVEL,
            "reason": "Invalid capability format (missing namespace:action)",
            "source": "default",
        }

    parts = capability.split(":", 1)
    namespace, action = parts

    # Check action mapping
    if action in ACTION_RISK_MAP:
        action_risk = ACTION_RISK_MAP[action]
        namespace_risk = NAMESPACE_RISK_MAP.get(namespace)

        if namespace_risk:
            final_risk = _higher_risk(action_risk, namespace_risk)
            return {
                "capability": capability,
                "risk_level": final_risk,
                "reason": f"Action '{action}' ({action_risk}) combined with namespace '{namespace}' ({namespace_risk})",
                "source": "action_and_namespace",
            }

        return {
            "capability": capability,
            "risk_level": action_risk,
            "reason": f"Action '{action}' maps to '{action_risk}' risk",
            "source": "action_map",
        }

    # Check namespace mapping
    if namespace in NAMESPACE_RISK_MAP:
        return {
            "capability": capability,
            "risk_level": NAMESPACE_RISK_MAP[namespace],
            "reason": f"Namespace '{namespace}' maps to '{NAMESPACE_RISK_MAP[namespace]}' risk",
            "source": "namespace_map",
        }

    return {
        "capability": capability,
        "risk_level": DEFAULT_RISK_LEVEL,
        "reason": f"No matching pattern found, using default '{DEFAULT_RISK_LEVEL}'",
        "source": "default",
    }
