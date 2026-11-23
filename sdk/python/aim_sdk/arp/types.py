"""
Core ARP types, ported from ``sdk/typescript/src/arp/types.ts``.

Python attributes are snake_case; :meth:`ARPEvent.to_dict` renders the TS wire
shape (camelCase keys) so both SDKs write the same event records.
"""
from __future__ import annotations

import json
import math
import time
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Callable, Dict, Mapping, Optional, Protocol

#: Monitor sources, as in the TS ``MonitorType``.
MONITOR_TYPES = (
    "process", "network", "filesystem", "skill", "heartbeat",
    "prompt", "mcp-protocol", "a2a-protocol",
)
#: Event categories, least to most serious.
CATEGORY_ORDER = ("normal", "anomaly", "violation", "threat")
#: Event severities, least to most serious.
SEVERITY_ORDER = ("info", "low", "medium", "high", "critical")
#: Which layer classified an event.
CLASSIFIED_BY = (
    "L0-rules", "L0-comply", "L1-statistical", "L1-behavioral-risk",
    "L1-guard-anomaly", "L2-llm",
)
#: Actions an alert rule can ask for.
ENFORCEMENT_ACTIONS = ("log", "alert", "pause", "kill")
#: What happened to a matched rule's action.
ENFORCEMENT_OUTCOMES = ("executed", "failed", "no-target", "not-executed")


def severity_index(severity: str) -> int:
    """Position of ``severity`` in :data:`SEVERITY_ORDER`; -1 when unknown (as TS indexOf)."""
    try:
        return SEVERITY_ORDER.index(severity)
    except ValueError:
        return -1


def severity_gte(a: str, b: str) -> bool:
    return severity_index(a) >= severity_index(b)


def category_index(category: str) -> int:
    try:
        return CATEGORY_ORDER.index(category)
    except ValueError:
        return -1


def raise_category(current: str, floor: str) -> str:
    return current if category_index(current) >= category_index(floor) else floor


def raise_severity(current: str, floor: str) -> str:
    return current if severity_index(current) >= severity_index(floor) else floor


def now_ms() -> float:
    return time.time() * 1000.0


def iso_from_ms(ms: float) -> str:
    """``Date.prototype.toISOString`` for a millisecond epoch: ms precision, trailing Z."""
    dt = datetime.fromtimestamp(ms / 1000.0, tz=timezone.utc)
    return dt.strftime("%Y-%m-%dT%H:%M:%S.") + "%03dZ" % (int(ms) % 1000)


def ms_from_iso(value: str) -> float:
    """Parse an ISO timestamp (``...Z`` or offset) to a millisecond epoch; NaN when unparseable."""
    try:
        text = value[:-1] + "+00:00" if value.endswith("Z") else value
        return datetime.fromisoformat(text).timestamp() * 1000.0
    except (TypeError, ValueError, AttributeError):
        return math.nan


def js_string(value: Any) -> str:
    """JavaScript ``String(value)`` for the JSON-shaped values an event carries."""
    if value is None:
        return "null"
    if value is True:
        return "true"
    if value is False:
        return "false"
    if isinstance(value, float):
        if math.isnan(value):
            return "NaN"
        if math.isinf(value):
            return "Infinity" if value > 0 else "-Infinity"
        if value.is_integer():
            return str(int(value))
        return repr(value)
    if isinstance(value, (list, tuple)):
        return ",".join("" if v is None else js_string(v) for v in value)
    if isinstance(value, Mapping):
        return "[object Object]"
    return str(value)


def _js_json_value(value: Any) -> Any:
    if isinstance(value, bool) or value is None:
        return value
    if isinstance(value, float):
        if math.isnan(value) or math.isinf(value):
            return None
        if value.is_integer():
            return int(value)
        return value
    if isinstance(value, Mapping):
        return {str(k): _js_json_value(v) for k, v in value.items()}
    if isinstance(value, (list, tuple)):
        return [_js_json_value(v) for v in value]
    if isinstance(value, (set, frozenset)):
        return [_js_json_value(v) for v in value]
    return value


def js_json(value: Any) -> str:
    """``JSON.stringify(value)``: compact, insertion order, integral floats as integers."""
    return json.dumps(_js_json_value(value), separators=(",", ":"), ensure_ascii=False, default=str)


@dataclass
class LLMAssessment:
    """Result of an L2 assessment of one event."""

    consistent: bool
    confidence: float
    reasoning: str
    recommendation: str  # 'allow' | 'alert' | 'pause' | 'kill'
    tokens_used: int
    estimated_cost: float

    def to_dict(self) -> Dict[str, Any]:
        return {
            "consistent": self.consistent,
            "confidence": self.confidence,
            "reasoning": self.reasoning,
            "recommendation": self.recommendation,
            "tokensUsed": self.tokens_used,
            "estimatedCost": self.estimated_cost,
        }


@dataclass
class ARPEvent:
    """An event emitted by a monitor or interceptor (TS ``ARPEvent``)."""

    id: str
    timestamp: str
    source: str
    category: str
    severity: str
    description: str
    data: Dict[str, Any]
    classified_by: str = "L0-rules"
    llm_assessment: Optional[LLMAssessment] = None

    def to_dict(self) -> Dict[str, Any]:
        out: Dict[str, Any] = {
            "id": self.id,
            "timestamp": self.timestamp,
            "source": self.source,
            "category": self.category,
            "severity": self.severity,
            "description": self.description,
            "data": self.data,
            "classifiedBy": self.classified_by,
        }
        if self.llm_assessment is not None:
            out["llmAssessment"] = self.llm_assessment.to_dict()
        return out


@dataclass
class AlertCondition:
    """Trigger condition of an :class:`AlertRule` (TS ``AlertCondition``)."""

    source: Optional[str] = None
    category: Optional[str] = None
    min_severity: Optional[str] = None
    #: e.g. ``{"data.host": "*.evil.com"}``; keys walk the TS (camelCase) event shape.
    field_match: Optional[Dict[str, str]] = None
    #: ``{"count": N, "window_ms": W}``: trigger after N events in W milliseconds.
    threshold: Optional[Dict[str, int]] = None

    @classmethod
    def from_dict(cls, raw: Mapping[str, Any]) -> "AlertCondition":
        threshold = raw.get("threshold")
        if threshold is not None:
            threshold = {
                "count": threshold["count"],
                "window_ms": threshold.get("window_ms", threshold.get("windowMs")),
            }
        return cls(
            source=raw.get("source"),
            category=raw.get("category"),
            min_severity=raw.get("min_severity", raw.get("minSeverity")),
            field_match=raw.get("field_match", raw.get("fieldMatch")),
            threshold=threshold,
        )


@dataclass
class AlertRule:
    """An alert/enforcement rule (TS ``AlertRule``).

    ``require_llm_confirmation`` only tags the event for an L2 review; the
    rule's action is never held back for one (fail closed).
    """

    name: str
    condition: AlertCondition
    action: str  # 'log' | 'alert' | 'pause' | 'kill'
    require_llm_confirmation: bool = False

    @classmethod
    def from_dict(cls, raw: Mapping[str, Any]) -> "AlertRule":
        condition = raw.get("condition") or {}
        if not isinstance(condition, AlertCondition):
            condition = AlertCondition.from_dict(condition)
        return cls(
            name=raw["name"],
            condition=condition,
            action=raw["action"],
            require_llm_confirmation=bool(
                raw.get("require_llm_confirmation", raw.get("requireLlmConfirmation", False))
            ),
        )


@dataclass
class EnforcementResult:
    """The outcome of a matched rule's action (TS ``EnforcementResult``).

    This is ARP's local process response (log/alert/pause/kill), not an AIM
    authorization decision.
    """

    action: str
    success: bool
    reason: str
    event: ARPEvent
    outcome: Optional[str] = None
    target_pid: Optional[int] = None


#: ``execute(action, event, target_pid=None) -> EnforcementResult``
EnforcementExecutor = Callable[..., EnforcementResult]


class Monitor(Protocol):
    """What every monitor and interceptor provides (TS ``Monitor``)."""

    type: str

    def start(self) -> None: ...

    def stop(self) -> None: ...

    def is_running(self) -> bool: ...
