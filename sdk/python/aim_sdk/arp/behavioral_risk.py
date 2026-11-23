"""
Behavioral risk signal channel, ported from
``sdk/typescript/src/arp/intelligence/behavioral-risk.ts``.

The coordinator asks a *source* how anomalous an event looks. A source has
``get_behavioral_risk_signal(event, timeout_ms)`` and returns either::

    {"status": "ok", "signal": {"score", "action", "reason", "source", "computedAtMs"}}
    {"status": "unavailable", "code": <code>, "reason": <text>}

Only the in-process source is ported; the TS unix-socket source is not.
"""
from __future__ import annotations

from typing import Any, Dict

from .types import now_ms

DEFAULT_BEHAVIORAL_RISK_TIMEOUT_MS = 25

UNAVAILABLE_CODES = (
    "NOT_READY", "TIMEOUT", "TRANSPORT_ERROR", "PARSE_ERROR",
    "CIRCUIT_OPEN", "DISABLED", "INTERNAL_ERROR",
)


class InProcessBehavioralRiskSource:
    """Scores events with a twin in this process (TS ``InProcessBehavioralRiskSource``)."""

    def __init__(self, twin: Any, source_name: str = "nanomind-l1-inproc"):
        self._twin = twin
        self._source_name = source_name

    def get_behavioral_risk_signal(self, event: Any, timeout_ms: float) -> Dict[str, Any]:
        try:
            result = self._twin.score_arp_event(event)
        except Exception as exc:  # noqa: BLE001 - a twin bug is not a threat signal
            return {
                "status": "unavailable",
                "code": "INTERNAL_ERROR",
                "reason": "scoreARPEvent threw: %s" % exc,
            }
        if result is None:
            return {
                "status": "unavailable",
                "code": "NOT_READY",
                "reason": "behavioral twin baseline is not yet trained",
            }
        return {
            "status": "ok",
            "signal": {
                "score": result["score"],
                "action": result["action"],
                "reason": result["reason"],
                "source": self._source_name,
                "computedAtMs": now_ms(),
            },
        }

    def close(self) -> None:
        """No transport resources to release."""
