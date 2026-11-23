"""
L0: the event engine and cross-monitor correlation.

Ported from ``sdk/typescript/src/arp/engine/event-engine.ts`` and
``engine/correlation.ts``. The engine receives events from monitors and
interceptors, evaluates the alert rules, hands each matched rule's action to an
executor, and emits a synthetic correlation event when two or more monitor
sources fire inside the correlation window.

The TS engine is asynchronous; this one is synchronous: :meth:`EventEngine.emit`
runs the handlers, the rules and the correlation check before it returns.
"""
from __future__ import annotations

import random
import re
import string
import threading
import uuid
from typing import Any, Callable, Dict, List, Mapping, Optional, Sequence

from ._reentry import arp_internal
from .types import (
    SEVERITY_ORDER,
    AlertCondition,
    AlertRule,
    ARPEvent,
    EnforcementResult,
    iso_from_ms,
    js_string,
    ms_from_iso,
    now_ms,
    severity_gte,
    severity_index,
)

EventHandler = Callable[[ARPEvent], Any]
EnforcementHandler = Callable[[EnforcementResult], Any]
Clock = Callable[[], float]

_BASE36 = string.digits + string.ascii_lowercase


def default_rules() -> List[AlertRule]:
    """Default rules: the TS ``defaultRules()``, rule for rule."""
    return [
        AlertRule(
            name="critical-threat",
            condition=AlertCondition(category="threat", min_severity="critical"),
            action="kill",
            require_llm_confirmation=True,
        ),
        AlertRule(
            name="high-violation",
            condition=AlertCondition(category="violation", min_severity="high"),
            action="alert",
        ),
        AlertRule(
            name="anomaly-burst",
            condition=AlertCondition(
                category="anomaly", threshold={"count": 10, "window_ms": 60000}
            ),
            action="alert",
            require_llm_confirmation=True,
        ),
        # AI-layer default rules
        AlertRule(
            name="prompt-injection-detected",
            condition=AlertCondition(source="prompt", category="threat", min_severity="high"),
            action="alert",
        ),
        AlertRule(
            name="credential-leak-detected",
            condition=AlertCondition(
                source="prompt", category="threat", field_match={"data.patternId": "OL-001"}
            ),
            action="alert",
        ),
        AlertRule(
            name="mcp-exploitation-detected",
            condition=AlertCondition(source="mcp-protocol", category="threat", min_severity="high"),
            action="alert",
        ),
        AlertRule(
            name="a2a-spoofing-detected",
            condition=AlertCondition(source="a2a-protocol", category="threat", min_severity="high"),
            action="alert",
        ),
    ]


def _get_nested_value(obj: Any, path: str) -> Any:
    current = obj
    for part in path.split("."):
        if not isinstance(current, Mapping):
            return _MISSING
        if part not in current:
            return _MISSING
        current = current[part]
    return current


class _Missing:
    pass


_MISSING = _Missing()


def match_pattern(value: str, pattern: str) -> bool:
    """Simple glob: ``*`` matches any sequence, everything else is literal."""
    regex = "^" + ".*".join(re.escape(part) for part in pattern.split("*")) + "$"
    return re.match(regex, value, flags=re.DOTALL) is not None


class CorrelationEngine:
    """Cross-monitor correlation over a sliding window (TS ``CorrelationEngine``).

    Returns a synthetic ``threat`` event when events from two or more different
    sources fall inside the window, once per distinct source set.
    """

    def __init__(self, window_ms: float = 60000, clock: Optional[Clock] = None):
        self._window_ms = window_ms
        self._clock = clock or now_ms
        self._recent: List[ARPEvent] = []
        self._emitted: set = set()

    def correlate(self, event: ARPEvent) -> Optional[ARPEvent]:
        now = self._clock()
        cutoff = now - self._window_ms

        while self._recent:
            oldest = ms_from_iso(self._recent[0].timestamp)
            if oldest < cutoff:
                self._recent.pop(0)
            else:
                break

        if len(self._emitted) > 1000:
            self._emitted.clear()

        self._recent.append(event)

        sources = {e.source for e in self._recent}
        if len(sources) < 2:
            return None

        sorted_sources = sorted(sources)
        key = "+".join(sorted_sources)
        if key in self._emitted:
            return None
        self._emitted.add(key)

        highest = "info"
        for e in self._recent:
            if severity_index(e.severity) > severity_index(highest):
                highest = e.severity
        escalated = SEVERITY_ORDER[min(severity_index(highest) + 1, len(SEVERITY_ORDER) - 1)]

        suffix = "".join(random.choice(_BASE36) for _ in range(6))
        window_s = self._window_ms / 1000
        window_text = js_string(float(window_s))
        return ARPEvent(
            id="corr-%d-%s" % (int(now), suffix),
            timestamp=iso_from_ms(now),
            source=self._recent[-1].source,
            category="threat",
            severity=escalated,
            description=(
                "Cross-monitor correlation: events detected from %s within %ss window"
                % (", ".join(sorted_sources), window_text)
            ),
            data={
                "correlatedSources": sorted_sources,
                "eventCount": len(self._recent),
                "windowMs": self._window_ms,
                "correlationKey": key,
            },
            classified_by="L1-statistical",
        )

    def reset(self) -> None:
        self._recent.clear()
        self._emitted.clear()

    def get_window_size(self) -> int:
        return len(self._recent)


class EventEngine:
    """Central event bus (TS ``EventEngine``).

    :param rules: alert rules; :func:`default_rules` when ``None``.
    :param executor: carries out each matched rule's action before the
        enforcement handlers run: an object with
        ``execute(action, event, target_pid=None)`` or a callable with that
        signature, returning an :class:`EnforcementResult`. Without one a
        matched action is not carried out and handlers receive
        ``success=False``, ``outcome="not-executed"``.
    :param clock: milliseconds since the epoch; for tests.
    """

    MAX_BUFFER_SIZE = 10000

    def __init__(
        self,
        rules: Optional[Sequence[Any]] = None,
        executor: Any = None,
        clock: Optional[Clock] = None,
        correlation_window_ms: float = 60000,
    ):
        if rules is None:
            self._rules = default_rules()
        else:
            self._rules = [r if isinstance(r, AlertRule) else AlertRule.from_dict(r) for r in rules]
        self._executor = executor
        self._clock = clock or now_ms
        self._handlers: List[EventHandler] = []
        self._enforcement_handlers: List[EnforcementHandler] = []
        self._buffer: List[ARPEvent] = []
        self._correlation = CorrelationEngine(correlation_window_ms, clock=self._clock)
        self._lock = threading.RLock()

    @property
    def rules(self) -> List[AlertRule]:
        return list(self._rules)

    def on_event(self, handler: EventHandler) -> None:
        """Register a handler for every event (logging, reporting, L1/L2)."""
        self._handlers.append(handler)

    def on_enforcement(self, handler: EnforcementHandler) -> None:
        """Register a handler for the outcome of each matched rule's action."""
        self._enforcement_handlers.append(handler)

    def emit(self, event: Mapping[str, Any]) -> ARPEvent:
        """Emit an event from a monitor: run handlers, evaluate rules, correlate.

        ``event`` holds ``source``, ``category``, ``severity``, ``description``
        and ``data`` (a dict). ``id``, ``timestamp`` and ``classified_by`` are
        assigned here.
        """
        if not isinstance(event, Mapping):
            raise TypeError(
                "EventEngine.emit(event): event must be a mapping with "
                "{ source, category, severity, description, data }"
            )
        data = event.get("data")
        if not isinstance(data, dict):
            raise TypeError(
                "EventEngine.emit(event): event.data must be a dict (received %s)"
                % ("undefined" if data is None and "data" not in event else type(data).__name__)
            )

        with self._lock, arp_internal():
            full = ARPEvent(
                id=str(uuid.uuid4()),
                timestamp=iso_from_ms(self._clock()),
                source=event.get("source"),
                category=event.get("category"),
                severity=event.get("severity"),
                description=event.get("description"),
                data=data,
                classified_by="L0-rules",
            )

            self._buffer.append(full)
            if len(self._buffer) > self.MAX_BUFFER_SIZE:
                self._buffer = self._buffer[-(self.MAX_BUFFER_SIZE // 2):]

            for handler in list(self._handlers):
                try:
                    handler(full)
                except Exception:
                    pass  # handler errors don't block the pipeline

            for rule in self._evaluate_rules(full):
                # A matched rule's action fires now; requireLlmConfirmation only
                # tags the event for an L2 follow-up (fail closed).
                if rule.require_llm_confirmation:
                    full.data["_llmReviewRequested"] = True
                    full.data["_initialAction"] = rule.action
                    full.data["_matchedRule"] = rule.name

                result = self._enforce(rule, full)
                for handler in list(self._enforcement_handlers):
                    try:
                        handler(result)
                    except Exception:
                        pass

            # Synthetic correlation events are not correlated again (no loops).
            if not full.data.get("correlationKey"):
                correlated = self._correlation.correlate(full)
                if correlated is not None:
                    self.emit({
                        "source": correlated.source,
                        "category": correlated.category,
                        "severity": correlated.severity,
                        "description": correlated.description,
                        "data": correlated.data,
                    })

            return full

    def reclassify(
        self, event: ARPEvent, new_category: str, new_severity: str, classified_by: str
    ) -> ARPEvent:
        """Update an event's classification (called by L1/L2 after re-analysis)."""
        event.category = new_category
        event.severity = new_severity
        event.classified_by = classified_by
        return event

    def get_recent_events(self, window_ms: float, source: Optional[str] = None) -> List[ARPEvent]:
        cutoff = self._clock() - window_ms
        out = []
        for e in list(self._buffer):
            if ms_from_iso(e.timestamp) < cutoff:
                continue
            if source and e.source != source:
                continue
            out.append(e)
        return out

    # ------------------------------------------------------------------ #
    def _call_executor(self, action: str, event: ARPEvent) -> EnforcementResult:
        execute = getattr(self._executor, "execute", None)
        if execute is None:
            execute = self._executor
        return execute(action, event)

    def _enforce(self, rule: AlertRule, event: ARPEvent) -> EnforcementResult:
        action = rule.action
        if self._executor is None:
            return EnforcementResult(
                action=action,
                success=False,
                outcome="not-executed",
                reason=(
                    'Rule "%s" matched: %s (not carried out: no enforcement executor is attached)'
                    % (rule.name, event.description)
                ),
                event=event,
            )
        try:
            result = self._call_executor(action, event)
        except Exception as exc:  # noqa: BLE001 - any executor failure is reported
            return EnforcementResult(
                action=action,
                success=False,
                outcome="failed",
                reason='Rule "%s" matched; the %s action threw: %s' % (rule.name, action, exc),
                event=event,
            )
        # Make `success` and `outcome` agree, as the TS engine does.
        if result.outcome is None:
            result.outcome = "executed" if result.success else "failed"
            return result
        result.success = result.outcome == "executed"
        return result

    def _evaluate_rules(self, event: ARPEvent) -> List[AlertRule]:
        return [rule for rule in self._rules if self._rule_matches(rule, event)]

    def _rule_matches(self, rule: AlertRule, event: ARPEvent) -> bool:
        c = rule.condition
        if c.source and c.source != event.source:
            return False
        if c.category and c.category != event.category:
            return False
        if c.min_severity and not severity_gte(event.severity, c.min_severity):
            return False
        if c.field_match:
            shape = event.to_dict()
            for key, pattern in c.field_match.items():
                value = _get_nested_value(shape, key)
                if value is _MISSING:
                    return False
                if not match_pattern(js_string(value), pattern):
                    return False
        if c.threshold:
            recent = self.get_recent_events(c.threshold["window_ms"], c.source)
            if len(recent) < c.threshold["count"]:
                return False
        return True
