"""
L1: the runtime twin, a behavioral anomaly scorer.

Ported from ``sdk/typescript/src/arp/intelligence/runtime-twin.ts``. Every L0
event is converted to a behavioral event and scored against a baseline learned
from the first 100 events; the score maps to the same response table as the TS
twin. Each processed event is appended to ``~/.opena2a/arp/events.jsonl`` and
each escalation (score above 0.4) to ``l1-escalations.jsonl``, in the record
shape the TS twin writes, rotated the same way.

Not ported: the fleet gradient upload. The TS twin can, on an explicit opt-in,
POST a differentially private gradient to the registry. This port makes no
network call; it scores and logs locally only.
"""
from __future__ import annotations

import hashlib
import json
import os
import re
import uuid
from typing import Any, Callable, Dict, List, Optional

from ._reentry import arp_internal
from .log_rotation import rotate_log_if_needed
from .types import ARPEvent, iso_from_ms, js_json, now_ms

#: Score thresholds to actions, as in the TS ``RESPONSE_TABLE``.
RESPONSE_TABLE = (
    (0.2, "allow", "Normal behavior"),
    (0.4, "alert", "Unusual pattern detected"),
    (0.6, "throttle", "Suspicious behavior — rate limited"),
    (0.8, "suspend", "High anomaly — agent suspended"),
    (1.0, "kill", "Critical anomaly — agent terminated"),
)

#: Events the baseline learns from before scoring starts.
BASELINE_MIN_EVENTS = 100

_SAFE_AGENT_ID = re.compile(r"^[A-Za-z0-9_.@-]+$")


def default_arp_dir() -> str:
    return os.path.join(os.path.expanduser("~"), ".opena2a", "arp")


class _Baseline:
    __slots__ = (
        "event_type_counts", "avg_timing_delta", "std_timing_delta",
        "capability_set", "avg_response_size", "total_events", "error_rate",
    )

    def __init__(self) -> None:
        self.event_type_counts: Dict[str, int] = {}
        self.avg_timing_delta = 0.0
        self.std_timing_delta = 0.0
        self.capability_set: set = set()
        self.avg_response_size = 0.0
        self.total_events = 0
        self.error_rate = 0.0

    @classmethod
    def from_dict(cls, raw: Dict[str, Any]) -> "_Baseline":
        b = cls()
        b.event_type_counts = dict(raw.get("eventTypeCounts") or {})
        b.avg_timing_delta = float(raw.get("avgTimingDelta") or 0)
        b.std_timing_delta = float(raw.get("stdTimingDelta") or 0)
        b.capability_set = set(raw.get("capabilitySet") or [])
        b.avg_response_size = float(raw.get("avgResponseSize") or 0)
        b.total_events = int(raw.get("totalEvents") or 0)
        b.error_rate = float(raw.get("errorRate") or 0)
        return b


def _event_type_for(event: ARPEvent) -> str:
    data = event.data or {}
    source = str(data.get("source") or data.get("monitor") or "")
    if "network" in source:
        return "EXTERNAL_CALL"
    if "filesystem" in source:
        return "MEMORY_READ"
    if "mcp" in source:
        return "MCP_CALL"
    if "capability" in source:
        return "CAPABILITY_CHECK"
    return "TOOL_CALL"


def _l0_decision_for(event: ARPEvent) -> str:
    data = event.data or {}
    if data.get("_initialAction") in ("kill", "pause"):
        return "block"
    if event.category in ("threat", "violation"):
        return "alert"
    if event.severity in ("high", "critical"):
        return "alert"
    return "allow"


class RuntimeTwin:
    """In-process behavioral twin (TS ``RuntimeTwin``).

    :param agent_id: agent name; also names the optional saved baseline
        ``baseline-<agent_id>.json`` in ``data_dir``.
    :param enabled: when false, :meth:`attach` and :meth:`score_arp_event` do nothing.
    :param data_dir: where the logs and baseline live; ``~/.opena2a/arp`` by default.
    :param clock: milliseconds since the epoch; for tests.
    """

    def __init__(
        self,
        agent_id: str,
        enabled: bool = True,
        data_dir: Optional[str] = None,
        clock: Optional[Callable[[], float]] = None,
    ):
        self.agent_id = agent_id
        self.session_id = str(uuid.uuid4())
        self.enabled = enabled
        self._clock = clock or now_ms
        self._data_dir = data_dir or default_arp_dir()
        self.event_log_path = os.path.join(self._data_dir, "events.jsonl")
        self.escalation_log_path = os.path.join(self._data_dir, "l1-escalations.jsonl")
        self._baseline: Optional[_Baseline] = None
        self._buffer: List[Dict[str, Any]] = []
        self._sequence_num = 0
        self._last_event_time = self._clock()
        self._load_baseline()

    # ------------------------------------------------------------------ #
    def attach(self, engine: Any) -> None:
        """Process every event the engine emits. Never changes the L0 decision."""
        if not self.enabled:
            return

        def _on_event(event: ARPEvent) -> None:
            behavioral = self._convert_event(event, mutate=True)
            result = self.process_event(behavioral)
            if result["score"] > 0.4:
                self._log_escalation(event, result)

        engine.on_event(_on_event)

    def score_arp_event(self, event: ARPEvent) -> Optional[Dict[str, Any]]:
        """Score an event WITHOUT changing any twin state.

        ``None`` while the twin is disabled or the baseline holds fewer than
        100 events; callers report that as ``NOT_READY`` rather than a 0.0.
        """
        if not self.enabled:
            return None
        if self._baseline is None or self._baseline.total_events < BASELINE_MIN_EVENTS:
            return None
        behavioral = self._convert_event(event, mutate=False)
        score = self._compute_anomaly_score(behavioral)
        action, label = self._get_response(score)
        return {"score": score, "action": action, "reason": label}

    def process_event(self, event: Dict[str, Any]) -> Dict[str, Any]:
        """Record a behavioral event and return its anomaly score."""
        self._buffer.append(event)
        self._append_to_log(event)
        if len(self._buffer) > 1000:
            self._buffer = self._buffer[-500:]
        score = self._compute_anomaly_score(event)
        action, label = self._get_response(score)
        return {"score": score, "action": action, "reason": label}

    def _set_baseline_for_test(self, baseline: Dict[str, Any]) -> None:
        """Test seam: install a baseline (TS-shaped dict) without 100 warm-up events."""
        self._baseline = _Baseline.from_dict(baseline)

    def shutdown(self) -> None:
        """Nothing to flush: this port sends no fleet gradient."""

    # ------------------------------------------------------------------ #
    def _convert_event(self, event: ARPEvent, mutate: bool) -> Dict[str, Any]:
        now = self._clock()
        if mutate:
            delta = now - self._last_event_time
            self._last_event_time = now
            self._sequence_num += 1
            sequence_num = self._sequence_num
        else:
            delta = now - self._last_event_time if self._last_event_time > 0 else 0
            sequence_num = self._sequence_num + 1

        data = event.data or {}
        response_size = data.get("responseSize")
        if isinstance(response_size, bool) or not isinstance(response_size, (int, float)):
            response_size = 0
        return {
            "agentId": self.agent_id,
            "sessionId": self.session_id,
            "sequenceNum": sequence_num,
            "eventType": _event_type_for(event),
            "capability": str(data.get("capability") or data.get("type") or "unknown"),
            "toolName": str(data["toolName"]) if data.get("toolName") else None,
            "argHash": hashlib.sha256(js_json(data).encode("utf-8")).hexdigest()[:16],
            "timestampDelta": delta,
            "wallClock": now,
            "responseSize": response_size,
            "responseCode": 1 if data.get("error") else 0,
            "l0Decision": _l0_decision_for(event),
        }

    def _compute_anomaly_score(self, event: Dict[str, Any]) -> float:
        b = self._baseline
        if b is None or b.total_events < BASELINE_MIN_EVENTS:
            self._update_baseline(event)
            return 0.0

        score = 0.0
        if event["capability"] not in b.capability_set:
            score += 0.3

        if b.std_timing_delta > 0:
            z = abs(event["timestampDelta"] - b.avg_timing_delta) / b.std_timing_delta
            if z > 3:
                score += 0.2
            if z > 5:
                score += 0.2

        type_ratio = (b.event_type_counts.get(event["eventType"]) or 0) / b.total_events
        if type_ratio < 0.01:
            score += 0.15

        if event["l0Decision"] == "block":
            score += 0.3
        if event["l0Decision"] == "alert":
            score += 0.15

        recent = self._buffer[-10:]
        if recent:
            avg_delta = sum(e["timestampDelta"] for e in recent) / len(recent)
            if avg_delta < 10 and len(recent) >= 10:
                score += 0.2
        if len([e for e in recent if e["responseCode"] > 0]) > 5:
            score += 0.15

        return min(score, 1.0)

    def _update_baseline(self, event: Dict[str, Any]) -> None:
        if self._baseline is None:
            self._baseline = _Baseline()
        b = self._baseline
        b.total_events += 1
        b.event_type_counts[event["eventType"]] = b.event_type_counts.get(event["eventType"], 0) + 1
        b.capability_set.add(event["capability"])

        n = b.total_events
        old_avg = b.avg_timing_delta
        b.avg_timing_delta = old_avg + (event["timestampDelta"] - old_avg) / n
        diff = event["timestampDelta"] - b.avg_timing_delta
        variance = (b.std_timing_delta ** 2 * (n - 1) + (event["timestampDelta"] - old_avg) * diff) / n
        b.std_timing_delta = max(variance, 0.0) ** 0.5

    @staticmethod
    def _get_response(score: float):
        for threshold, action, label in RESPONSE_TABLE:
            if score <= threshold:
                return action, label
        return "kill", "Critical anomaly"

    def _load_baseline(self) -> None:
        if not _SAFE_AGENT_ID.match(self.agent_id or ""):
            return  # never build a path from an id that could leave the directory
        path = os.path.join(self._data_dir, "baseline-%s.json" % self.agent_id)
        try:
            with arp_internal():
                if os.path.exists(path):
                    with open(path, "r", encoding="utf-8") as fh:
                        self._baseline = _Baseline.from_dict(json.load(fh))
        except (OSError, ValueError, TypeError, AttributeError):
            pass

    def _append(self, path: str, record: Dict[str, Any]) -> None:
        try:
            with arp_internal():
                os.makedirs(os.path.dirname(path), exist_ok=True)
                rotate_log_if_needed(path)
                with open(path, "a", encoding="utf-8") as fh:
                    fh.write(js_json(record) + "\n")
        except OSError:
            pass

    def _append_to_log(self, event: Dict[str, Any]) -> None:
        self._append(self.event_log_path, event)

    def _log_escalation(self, original: ARPEvent, result: Dict[str, Any]) -> None:
        self._append(self.escalation_log_path, {
            "timestamp": iso_from_ms(self._clock()),
            "agentId": self.agent_id,
            "eventId": original.id,
            "score": result["score"],
            "action": result["action"],
            "reason": result["reason"],
            "category": original.category,
        })
