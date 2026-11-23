"""
L2: the 3-layer intelligence coordinator.

Ported from ``sdk/typescript/src/arp/intelligence/coordinator.ts``.
:meth:`IntelligenceCoordinator.analyze` runs one event through, in order:

1. guard-anomaly drift recording (when a drift detector is attached),
2. the comply gate (only when the capability manifest sets
   ``comply.enforce is True``),
3. behavioral risk fusion (when a risk source, e.g. the runtime twin, is attached),
4. L1 statistical anomaly scoring,
5. L2 assessment by an LLM adapter, only when L2 was switched on explicitly
   (``enabled=True`` and an adapter given) and L1 flagged the event.

The Python SDK builds no L2 adapters of its own and makes no network call: L2
runs only through an adapter object the caller passes in, with
``name``, ``assess(prompt, max_tokens)``, ``estimate_cost(input_tokens,
output_tokens)`` and ``health_check()`` (and optionally ``endpoint``).

Everything here classifies ARP events for local response and telemetry. It is
not an AIM authorization path: ``aim_sdk.enforcement`` and ``aim_sdk.decision``
do not import it.
"""
from __future__ import annotations

import logging
import re
import threading
from dataclasses import dataclass
from typing import Any, Callable, Dict, List, Mapping, Optional, Sequence

from .anomaly import AnomalyDetector
from .behavioral_risk import DEFAULT_BEHAVIORAL_RISK_TIMEOUT_MS
from .budget import BudgetController
from .types import (
    ARPEvent,
    LLMAssessment,
    js_json,
    raise_category,
    raise_severity,
    severity_index,
)

logger = logging.getLogger("aim_sdk.arp")

#: Input tokens assumed for an L2 call's budget reservation.
ESTIMATED_PROMPT_TOKENS = 200
L2_HEALTH_CHECK_TIMEOUT_MS = 2000
L2_FAILURE_MESSAGE_MAX_CHARS = 200
_PRINTABLE_URL = re.compile(r"^https?://[A-Za-z0-9.-]+(?::\d{1,5})?$")

#: Fields of ``event.data`` that may reach an L2 adapter verbatim. An allowlist:
#: every other field is replaced by a description of its shape.
L2_ALLOWED_DATA_FIELDS = frozenset([
    "patternId", "patternCategory", "direction", "toolName", "from", "to",
    "protocol", "method", "eventType", "port", "pid",
])

AI_LAYER_SOURCES = frozenset(["prompt", "mcp-protocol", "a2a-protocol"])

_BATCH_ACTIONS = {"ALLOW": "allow", "ALERT": "alert", "PAUSE": "pause", "KILL": "kill"}
_RECOMMENDATION_ORDER = ("allow", "alert", "pause", "kill")
_BATCH_VERDICT_LINE = re.compile(
    r"^(?:[-•]\s*)?(?:EVENT[\s_]*)?#?(\d+)\s*[:.)-]\s*([A-Z]+)(?=$|[\s—–:,.;-])(.*)$",
    re.IGNORECASE,
)
_BIDI = frozenset(
    "\u061c\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069"
)


@dataclass
class IntelligenceConfig:
    """The TS ``IntelligenceConfig`` fields the Python coordinator reads.

    ``enabled`` absent (``None``) means L2 is off; it must be a bool when given.
    ``adapter`` is an adapter *object*: the Python SDK builds none from a name.
    """

    enabled: Optional[bool] = None
    adapter: Any = None
    budget_usd: Optional[float] = None
    max_tokens_per_call: Optional[int] = None
    max_calls_per_hour: Optional[int] = None
    min_severity_for_llm: Optional[str] = None
    enable_batching: bool = False
    batch_window_ms: Optional[float] = None
    behavioral_risk_timeout_ms: Optional[float] = None

    def __post_init__(self) -> None:
        if self.enabled is not None and not isinstance(self.enabled, bool):
            raise TypeError(
                "intelligence.enabled must be a boolean (received %r)" % (self.enabled,)
            )


def _as_config(intelligence: Any) -> IntelligenceConfig:
    if intelligence is None:
        return IntelligenceConfig()
    if isinstance(intelligence, IntelligenceConfig):
        return intelligence
    if isinstance(intelligence, Mapping):
        return IntelligenceConfig(**dict(intelligence))
    raise TypeError("intelligence must be an IntelligenceConfig or a mapping")


# --------------------------------------------------------------------------- #
# L2 resolution and status
# --------------------------------------------------------------------------- #
def _adapter_kind(name: Any) -> str:
    return name if name in ("anthropic", "openai", "ollama") else "other"


def _resolve_l2_adapter(config: IntelligenceConfig) -> Dict[str, Any]:
    if config.enabled is not True:
        return {"adapter": None, "asked": False, "code": "not-enabled",
                "reason": "intelligence.enabled is not true"}
    adapter = config.adapter
    if not adapter:
        return {"adapter": None, "asked": True, "code": "no-adapter",
                "reason": "no 'intelligence.adapter' is configured"}
    if isinstance(adapter, str):
        return {"adapter": None, "name": adapter, "asked": True, "code": "adapter-failed",
                "reason": "the Python SDK builds no L2 adapter from a name (%r); pass an "
                          "adapter object with assess() and estimate_cost()" % adapter}
    if not callable(getattr(adapter, "assess", None)) or not callable(
        getattr(adapter, "estimate_cost", None)
    ):
        name = getattr(adapter, "name", None)
        return {"adapter": None, "name": name, "asked": True, "code": "adapter-failed",
                "reason": "the L2 adapter has no assess() or estimate_cost()"}
    return {"adapter": adapter, "name": getattr(adapter, "name", "custom"), "asked": True}


def describe_l2_status(intelligence: Any) -> Dict[str, Any]:
    """Whether L2 would run for ``intelligence``, and if not, why (TS ``describeL2Status``)."""
    resolved = _resolve_l2_adapter(_as_config(intelligence))
    if resolved["adapter"] is not None:
        return {"running": True, "adapter": resolved["name"]}
    if resolved.get("name") is None:
        return {"running": False, "reason": resolved["reason"]}
    return {"running": False, "adapter": resolved["name"], "reason": resolved["reason"]}


def _l2_status_of(resolved: Dict[str, Any]) -> Dict[str, Any]:
    if resolved["adapter"] is not None:
        return {"running": True, "adapter": _adapter_kind(getattr(resolved["adapter"], "name", None))}
    if resolved.get("name") is None:
        return {"running": False, "reason": resolved["code"]}
    return {"running": False, "adapter": _adapter_kind(resolved["name"]), "reason": resolved["code"]}


# --------------------------------------------------------------------------- #
# Pure helpers (exported so the behavioral suite can pin them to the TS reference)
# --------------------------------------------------------------------------- #
def _utf16_len(s: str) -> int:
    return len(s.encode("utf-16-le")) // 2


def describe_withheld(value: Any) -> str:
    """Describe a withheld value by length bucket and character classes only."""
    s = value if isinstance(value, str) else js_json(value)
    n = _utf16_len(s)
    if n == 0:
        bucket = "empty"
    elif n <= 8:
        bucket = "up to 8 chars"
    elif n <= 16:
        bucket = "9-16 chars"
    elif n <= 32:
        bucket = "17-32 chars"
    elif n <= 64:
        bucket = "33-64 chars"
    elif n <= 128:
        bucket = "65-128 chars"
    else:
        bucket = "over 128 chars"
    if n <= 8:
        return "<withheld: %s>" % bucket
    classes = []
    if re.search(r"[a-z]", s):
        classes.append("lower")
    if re.search(r"[A-Z]", s):
        classes.append("upper")
    if re.search(r"[0-9]", s):
        classes.append("digit")
    if re.search(r"[^A-Za-z0-9]", s):
        classes.append("symbol")
    return "<withheld: %s%s>" % (bucket, (", " + "+".join(classes)) if classes else "")


def allowlist_event_data_for_l2(data: Optional[Mapping[str, Any]]) -> Dict[str, Any]:
    """``event.data`` with every non-allowlisted field replaced by its shape."""
    out: Dict[str, Any] = {}
    if not data:
        return out
    for k, v in data.items():
        out[k] = v if k in L2_ALLOWED_DATA_FIELDS else describe_withheld(v)
    return out


def build_agent_context(
    agent_name: str,
    agent_description: Optional[str] = None,
    declared_capabilities: Optional[Sequence[str]] = None,
) -> str:
    parts = ["Agent: %s" % agent_name]
    if agent_description:
        parts.append("Purpose: %s" % agent_description)
    if declared_capabilities:
        parts.append("Capabilities: %s" % ", ".join(declared_capabilities))
    return "\n".join(parts)


def build_micro_prompt(agent_context: str, event: ARPEvent) -> str:
    if event.source in AI_LAYER_SOURCES:
        return _build_ai_layer_prompt(agent_context, event)
    data_summary = js_json(allowlist_event_data_for_l2(event.data))[:500]
    return (
        "SECURITY ASSESSMENT — answer concisely.\n\n"
        "%s\n\n"
        "Event: %s monitor detected %s (%s)\n"
        "Detail: %s\n"
        "Data: %s\n\n"
        "Is this behavior consistent with the agent's declared purpose and capabilities?\n"
        "Respond in exactly this format:\n"
        "CONSISTENT: YES or NO\n"
        "CONFIDENCE: 0.0-1.0\n"
        "REASONING: one sentence\n"
        "ACTION: ALLOW, ALERT, PAUSE, or KILL"
    ) % (agent_context, event.source, event.category, event.severity, event.description, data_summary)


def _or(value: Any, default: Any) -> Any:
    return default if value is None else value


def _build_ai_layer_prompt(agent_context: str, event: ARPEvent) -> str:
    data = event.data
    pattern_id = _or(data.get("patternId"), "unknown")
    pattern_category = _or(data.get("patternCategory"), event.source)
    matched_text = _or(data.get("matchedText"), "")
    direction = _or(data.get("direction"), "")

    content = ""
    if event.source == "prompt":
        if direction == "input":
            content = "User/agent sent input that matched pattern %s (%s)." % (pattern_id, pattern_category)
        else:
            content = "LLM response matched output leak pattern %s (%s)." % (pattern_id, pattern_category)
    elif event.source == "mcp-protocol":
        content = 'MCP tool call "%s" has parameters matching exploitation pattern %s.' % (
            _or(data.get("toolName"), "unknown"), pattern_id)
    elif event.source == "a2a-protocol":
        content = 'A2A message from "%s" to "%s" matched attack pattern %s.' % (
            _or(data.get("from"), "unknown"), _or(data.get("to"), "unknown"), pattern_id)

    return (
        "AI-LAYER THREAT ASSESSMENT — determine if this is a true positive or false positive.\n\n"
        "%s\n\n"
        "Detection: %s\n"
        "Context: %s\n"
        "Matched text: %s\n"
        "Pattern: %s — %s\n"
        "Severity: %s\n\n"
        "Could this be legitimate usage given the agent's purpose, or is this a genuine attack attempt?\n"
        "Consider: obfuscation techniques, benign edge cases, and the agent's declared capabilities.\n\n"
        "Respond in exactly this format:\n"
        "CONSISTENT: YES (benign/false positive) or NO (genuine threat)\n"
        "CONFIDENCE: 0.0-1.0\n"
        "REASONING: one sentence explaining why\n"
        "ACTION: ALLOW, ALERT, PAUSE, or KILL"
    ) % (agent_context, event.description, content, describe_withheld(matched_text),
         pattern_id, pattern_category, event.severity)


def build_batch_prompt(agent_context: str, events: Sequence[ARPEvent]) -> str:
    summary = "\n".join(
        "%d. [%s] %s: %s" % (i + 1, e.source, e.severity, e.description) for i, e in enumerate(events)
    )
    return (
        "BATCH SECURITY ASSESSMENT — %d anomalies.\n\n"
        "%s\n\n"
        "Events:\n%s\n\n"
        "Is each event consistent with the agent's declared purpose and capabilities?\n"
        "Answer every event, one line each, in exactly this format:\n"
        "EVENT_NUM: ALLOW, ALERT, PAUSE, or KILL — one sentence reason"
    ) % (len(events), agent_context, summary)


_LEADING_FLOAT = re.compile(r"^[+-]?(?:\d+\.?\d*|\.\d+)(?:[eE][+-]?\d+)?")


def _parse_float(text: str) -> Optional[float]:
    """JavaScript ``parseFloat``: the longest leading number, or None."""
    stripped = text.lstrip()
    if stripped.startswith(("Infinity", "+Infinity")):
        return float("inf")
    if stripped.startswith("-Infinity"):
        return float("-inf")
    match = _LEADING_FLOAT.match(stripped)
    return float(match.group(0)) if match else None


def parse_assessment(content: str, tokens: int, cost: float) -> LLMAssessment:
    """Parse a single-event L2 answer. A missing ACTION line is deny-safe: alert, inconsistent."""
    consistent = False
    confidence = 0.5
    reasoning = "Assessment parse incomplete — defaulting to deny-safe"
    recommendation = "alert"
    has_action = False

    for line in content.strip().split("\n"):
        upper = line.upper()
        parts = line.split(":")
        if upper.startswith("CONSISTENT:"):
            consistent = "YES" in upper
        elif upper.startswith("CONFIDENCE:"):
            val = _parse_float(parts[1].strip() if len(parts) > 1 else "0.5")
            confidence = 0.5 if val is None else max(0.0, min(1.0, val))
        elif upper.startswith("REASONING:"):
            reasoning = ":".join(parts[1:]).strip()
        elif upper.startswith("ACTION:"):
            has_action = True
            action = parts[1].strip().upper() if len(parts) > 1 else ""
            recommendation = {"ALLOW": "allow", "ALERT": "alert", "PAUSE": "pause", "KILL": "kill"}.get(
                action, "alert"
            )

    if not has_action:
        recommendation = "alert"
        consistent = False

    return LLMAssessment(
        consistent=consistent, confidence=confidence, reasoning=reasoning,
        recommendation=recommendation, tokens_used=tokens, estimated_cost=cost,
    )


def parse_batch_assessments(
    content: str, event_count: int, tokens: int, cost: float
) -> List[LLMAssessment]:
    """Parse a batch answer; an event with no verdict line gets a deny-safe alert."""
    verdicts: Dict[int, Dict[str, str]] = {}
    for line in content.split("\n"):
        match = _BATCH_VERDICT_LINE.match(re.sub(r"[*`]", "", line).strip())
        if not match:
            continue
        index = int(match.group(1)) - 1
        recommendation = _BATCH_ACTIONS.get(match.group(2).upper())
        if recommendation is None or index < 0 or index >= event_count:
            continue
        held = verdicts.get(index)
        if held and _RECOMMENDATION_ORDER.index(held["recommendation"]) >= _RECOMMENDATION_ORDER.index(
            recommendation
        ):
            continue
        reasoning = re.sub(r"^[\s—–:,.;-]+", "", match.group(3)).strip()
        verdicts[index] = {
            "recommendation": recommendation,
            "reasoning": reasoning or "No reason given in the batch answer",
        }

    out = []
    for i in range(event_count):
        verdict = verdicts.get(i)
        out.append(LLMAssessment(
            consistent=bool(verdict) and verdict["recommendation"] == "allow",
            confidence=0.5,
            reasoning=(
                verdict["reasoning"] if verdict
                else "No action for this event in the batch answer — defaulting to deny-safe"
            ),
            recommendation=verdict["recommendation"] if verdict else "alert",
            tokens_used=tokens // event_count + (1 if i < tokens % event_count else 0),
            estimated_cost=cost / event_count,
        ))
    return out


def apply_assessment(event: ARPEvent, assessment: LLMAssessment) -> None:
    event.llm_assessment = assessment
    event.classified_by = "L2-llm"
    if assessment.recommendation == "kill":
        event.severity = "critical"
        event.category = "threat"
    elif assessment.recommendation == "pause":
        event.severity = "high"
        event.category = "violation"


def _event_classification(event: ARPEvent) -> Optional[str]:
    raw = (event.data or {}).get("classification")
    return raw if isinstance(raw, str) and raw else None


def apply_comply_gate(event: ARPEvent, manifest: Mapping[str, Any]) -> bool:
    """Deny a classified event outside the manifest's comply envelope. True when denied."""
    classification = _event_classification(event)
    if classification is None:
        return False
    comply = manifest["comply"]
    if classification in comply.get("prohibited_classes", []):
        reason = "prohibited"
    elif classification not in comply.get("permitted_classes", []):
        reason = "unknown"  # parse-to-deny
    else:
        return False

    action = comply.get("on_violation")
    event.data["comply"] = {"classification": classification, "reason": reason, "action": action}
    event.classified_by = "L0-comply"
    if action == "alert":
        event.category = raise_category(event.category, "violation")
        event.severity = raise_severity(event.severity, "medium")
    elif action == "pause":
        event.category = raise_category(event.category, "violation")
        event.severity = raise_severity(event.severity, "high")
    elif action in ("kill", "deny"):
        event.category = "threat"
        event.severity = "critical"
    return True


def band_for_score(score: float) -> str:
    if score >= 0.8:
        return "critical"
    if score >= 0.6:
        return "high"
    if score >= 0.4:
        return "elevated"
    return "low"


def apply_behavioral_risk(event: ARPEvent, result: Mapping[str, Any]) -> None:
    """Record a risk result on ``event.data.behavioralRisk``; raise severity by band.

    An unavailable result is recorded and ignored: an outage is not a threat.
    """
    if result.get("status") != "ok":
        event.data["behavioralRisk"] = {"status": "unavailable", "code": result.get("code")}
        return
    signal = result["signal"]
    band = band_for_score(signal["score"])
    event.data["behavioralRisk"] = {
        "status": "ok",
        "score": signal["score"],
        "action": signal["action"],
        "source": signal["source"],
        "band": band,
    }
    if band == "critical":
        event.category = "threat"
        event.severity = "critical"
    elif band == "high":
        event.category = raise_category(event.category, "violation")
        event.severity = raise_severity(event.severity, "high")
    elif band == "elevated":
        event.category = raise_category(event.category, "anomaly")
        event.severity = raise_severity(event.severity, "medium")
    else:
        return
    if event.classified_by == "L0-rules":
        event.classified_by = "L1-behavioral-risk"


def apply_guard_anomaly(event: ARPEvent, detector: Any) -> None:
    """Record a classified event with the drift detector; lift severity on drift only."""
    classification = _event_classification(event)
    if classification is None:
        return
    status = detector.record(classification)
    event.data["guardAnomaly"] = status
    state = status.get("status") if isinstance(status, Mapping) else getattr(status, "status", None)
    if state == "drift":
        event.category = raise_category(event.category, "anomaly")
        event.severity = raise_severity(event.severity, "medium")
        if event.classified_by == "L0-rules":
            event.classified_by = "L1-guard-anomaly"


def sanitize_terminal_line(raw: str, max_chars: int) -> str:
    out = []
    for count, ch in enumerate(raw):
        if count == max_chars:
            return "".join(out) + " [truncated]"
        code = ord(ch)
        bad = code <= 0x1F or 0x7F <= code <= 0x9F or 0xD800 <= code <= 0xDFFF or ch in _BIDI
        out.append("\ufffd" if bad else ch)
    return "".join(out)


def describe_l2_failure(error: BaseException) -> str:
    """The error's code, or its message reduced to text that cannot carry the prompt."""
    code = getattr(error, "code", None)
    if isinstance(code, str) and re.match(r"^[A-Z][A-Z0-9_]{1,63}$", code):
        return code
    if isinstance(error, ValueError) and type(error).__name__ == "JSONDecodeError":
        return "the answer was not valid JSON"
    message = str(error)
    status = re.match(r"^HTTP \d{3}\b", message)
    if status:
        return status.group(0)
    line = re.sub(r"\.+$", "", sanitize_terminal_line(message.strip(), L2_FAILURE_MESSAGE_MAX_CHARS))
    return line if line else "the call threw without a message"


def _l2_failure_next_step(adapter: Any) -> str:
    endpoint = getattr(adapter, "endpoint", None)
    if not (isinstance(endpoint, str) and _PRINTABLE_URL.match(endpoint)):
        endpoint = None
    name = getattr(adapter, "name", None)
    if name == "ollama":
        if endpoint:
            return "Check that an Ollama server is listening there: curl -sS %s/api/tags" % endpoint
        return "Check that intelligence.adapterConfig.host is the URL of the Ollama server."
    if name in ("anthropic", "openai"):
        key = "ANTHROPIC_API_KEY" if name == "anthropic" else "OPENAI_API_KEY"
        check = ("Check that %s or intelligence.adapterConfig.apiKey holds a valid key, and "
                 "that this host reaches the endpoint" % key)
        return "%s: curl -sSI %s" % (check, endpoint) if endpoint else check + "."
    return "Check the adapter settings in intelligence.adapterConfig."


def _response_field(response: Any, name: str, camel: str) -> Any:
    if isinstance(response, Mapping):
        return response.get(name, response.get(camel))
    return getattr(response, name, getattr(response, camel, None))


# --------------------------------------------------------------------------- #
class IntelligenceCoordinator:
    """The 3-layer intelligence coordinator (TS ``IntelligenceCoordinator``)."""

    def __init__(
        self,
        agent_name: str,
        data_dir: str,
        intelligence: Any = None,
        agent_description: Optional[str] = None,
        declared_capabilities: Optional[Sequence[str]] = None,
        manifest: Optional[Mapping[str, Any]] = None,
        behavioral_risk_source: Any = None,
        guard_anomaly: Any = None,
        clock: Optional[Callable[[], float]] = None,
    ):
        self._config = _as_config(intelligence)
        self._budget = BudgetController(
            data_dir,
            budget_usd=self._config.budget_usd,
            max_calls_per_hour=self._config.max_calls_per_hour,
            clock=clock,
        )
        self._anomaly = AnomalyDetector(clock=clock)
        self._manifest = manifest
        self._behavioral_risk_source = behavioral_risk_source
        self._guard_anomaly = guard_anomaly
        self._agent_context = build_agent_context(agent_name, agent_description, declared_capabilities)

        self._batch_queue: List[ARPEvent] = []
        self._batch_timer: Optional[threading.Timer] = None
        self._batch_lock = threading.Lock()
        self._batch_assessed_handlers: List[Callable[[ARPEvent], Any]] = []
        self._l2_failure_reported = False
        self._l2_health_checked = False

        resolved = _resolve_l2_adapter(self._config)
        self._adapter = resolved["adapter"]
        self._l2_status = _l2_status_of(resolved)
        if resolved["adapter"] is None and resolved["asked"]:
            logger.warning(
                "[ARP] L2 intelligence is not running (%s). L0 and L1 are unaffected and still "
                "gate. To enable it, set intelligence.enabled to True and pass an L2 adapter "
                "object as intelligence.adapter.",
                resolved["reason"],
            )

    # --- manifest / sources ------------------------------------------- #
    def set_capability_manifest(self, manifest: Optional[Mapping[str, Any]]) -> None:
        self._manifest = manifest

    def get_capability_manifest(self) -> Optional[Mapping[str, Any]]:
        return self._manifest

    def set_behavioral_risk_source(self, source: Any) -> None:
        self._behavioral_risk_source = source

    def get_behavioral_risk_source(self) -> Any:
        return self._behavioral_risk_source

    def set_guard_anomaly(self, detector: Any) -> None:
        self._guard_anomaly = detector

    def get_guard_anomaly(self) -> Any:
        return self._guard_anomaly

    # --- analysis ------------------------------------------------------ #
    def analyze(self, event: ARPEvent) -> Optional[LLMAssessment]:
        """Run ``event`` through the stack; mutates its category, severity, classified_by.

        Returns the L2 assessment when L2 was called for it; ``None`` otherwise,
        including for an event queued for a batch (see :meth:`on_batch_assessed`).
        """
        if self._guard_anomaly is not None:
            apply_guard_anomaly(event, self._guard_anomaly)

        # Comply enforcement is opt-in: only a manifest with enforce === True acts.
        manifest = self._manifest
        if manifest is not None and (manifest.get("comply") or {}).get("enforce") is True:
            if apply_comply_gate(event, manifest):
                return None

        if self._behavioral_risk_source is not None:
            timeout_ms = _or(self._config.behavioral_risk_timeout_ms, DEFAULT_BEHAVIORAL_RISK_TIMEOUT_MS)
            try:
                risk = self._behavioral_risk_source.get_behavioral_risk_signal(event, timeout_ms)
            except Exception as exc:  # noqa: BLE001 - every error path is "unavailable"
                risk = {"status": "unavailable", "code": "INTERNAL_ERROR", "reason": str(exc)}
            apply_behavioral_risk(event, risk)

        if self._anomaly.score(event) > 2.0:
            if event.category == "normal":
                event.category = "anomaly"
            if severity_index(event.severity) < severity_index("medium"):
                event.severity = "medium"
            event.classified_by = "L1-statistical"
        self._anomaly.record(event)

        if self._should_escalate_to_l2(event):
            if self._config.enable_batching and event.severity != "critical":
                return self._queue_for_batch(event)
            return self._assess_with_llm(event)
        return None

    def on_batch_assessed(self, handler: Callable[[ARPEvent], Any]) -> None:
        """Called with each event of a batch once the batch is answered."""
        self._batch_assessed_handlers.append(handler)

    def get_budget_status(self) -> Dict[str, Any]:
        return self._budget.get_status()

    def get_l2_status(self) -> Dict[str, Any]:
        return dict(self._l2_status)

    def check_l2_health(self, timeout_ms: float = L2_HEALTH_CHECK_TIMEOUT_MS) -> None:
        """Ask the adapter whether it answers; report it once if not. Never raises."""
        if self._l2_health_checked:
            return
        self._l2_health_checked = True
        if self._config.enabled is not True or self._adapter is None:
            return
        adapter = self._adapter
        outcome: Dict[str, Any] = {}

        def _run() -> None:
            try:
                healthy = adapter.health_check()
                outcome["failure"] = None if healthy is True else "health check failed"
            except Exception as exc:  # noqa: BLE001
                outcome["failure"] = "health check failed: %s" % describe_l2_failure(exc)

        worker = threading.Thread(target=_run, name="aim-arp-l2-health", daemon=True)
        worker.start()
        worker.join(timeout_ms / 1000.0)
        failure = outcome.get(
            "failure", "health check did not answer within %d ms" % timeout_ms
        )
        if failure is not None:
            self._report_l2_call_failed(adapter, failure)

    def stop(self) -> None:
        """Cancel the batch timer and flush what is queued."""
        with self._batch_lock:
            timer, self._batch_timer = self._batch_timer, None
        if timer is not None:
            timer.cancel()
        self.flush_batch()

    # --- L2 internals --------------------------------------------------- #
    def _should_escalate_to_l2(self, event: ARPEvent) -> bool:
        if self._config.enabled is not True or self._adapter is None:
            return False
        if event.category == "normal":
            return False
        min_sev = self._config.min_severity_for_llm or "medium"
        return severity_index(event.severity) >= severity_index(min_sev)

    def _report_l2_call_failed(self, adapter: Any, message: str) -> None:
        if self._l2_failure_reported:
            return
        self._l2_failure_reported = True
        logger.warning(
            "[ARP] L2 call failed (%s, %s): %s. L0 and L1 are unaffected and still gate. %s",
            getattr(adapter, "name", "custom"),
            getattr(adapter, "endpoint", None) or "endpoint not reported by the adapter",
            message,
            _l2_failure_next_step(adapter),
        )

    def _call_within_budget(self, prompt: str, max_tokens: int):
        adapter = self._adapter
        if adapter is None:
            return None
        reservation = self._budget.reserve(adapter.estimate_cost(ESTIMATED_PROMPT_TOKENS, max_tokens))
        if reservation is None:
            return None
        try:
            response = adapter.assess(prompt, max_tokens)
        except Exception as exc:
            try:
                self._budget.release(reservation)
            finally:
                message = describe_l2_failure(exc)
                try:
                    self._budget.record_failure(getattr(adapter, "name", "custom"), message)
                finally:
                    self._report_l2_call_failed(adapter, message)
            raise
        self._l2_failure_reported = False

        input_tokens = _response_field(response, "input_tokens", "inputTokens") or 0
        output_tokens = _response_field(response, "output_tokens", "outputTokens") or 0
        cost = float("nan")
        try:
            cost = adapter.estimate_cost(input_tokens, output_tokens)
        finally:
            self._budget.record(reservation, cost, input_tokens + output_tokens)
        return response, cost, input_tokens + output_tokens

    def _assess_with_llm(self, event: ARPEvent) -> Optional[LLMAssessment]:
        prompt = build_micro_prompt(self._agent_context, event)
        max_tokens = self._config.max_tokens_per_call or 300
        try:
            answered = self._call_within_budget(prompt, max_tokens)
        except Exception:
            return None  # counted and reported; L1 classification stands
        if answered is None:
            return None
        response, cost, tokens = answered
        content = _response_field(response, "content", "content") or ""
        assessment = parse_assessment(content, tokens, cost)
        apply_assessment(event, assessment)
        return assessment

    def _batch_max_tokens(self) -> int:
        return min((self._config.max_tokens_per_call or 300) * 2, 1000)

    def _can_pay_for_batch(self) -> bool:
        adapter = self._adapter
        if adapter is None:
            return False
        try:
            return self._budget.can_reserve(
                adapter.estimate_cost(ESTIMATED_PROMPT_TOKENS, self._batch_max_tokens())
            )
        except Exception:  # noqa: BLE001
            return False

    def _queue_for_batch(self, event: ARPEvent) -> None:
        if not self._can_pay_for_batch():
            return None
        with self._batch_lock:
            self._batch_queue.append(event)
            if self._batch_timer is None:
                window_s = _or(self._config.batch_window_ms, 300000) / 1000.0
                timer = threading.Timer(window_s, self._on_batch_timer)
                timer.daemon = True
                self._batch_timer = timer
                timer.start()
        return None

    def _on_batch_timer(self) -> None:
        with self._batch_lock:
            self._batch_timer = None
        try:
            self.flush_batch()
        except Exception:  # noqa: BLE001
            pass

    def flush_batch(self) -> None:
        """Send the queued batch now (the timer calls this when the window closes)."""
        with self._batch_lock:
            if not self._batch_queue or self._adapter is None:
                return
            events, self._batch_queue = self._batch_queue, []
        prompt = build_batch_prompt(self._agent_context, events)
        try:
            answered = self._call_within_budget(prompt, self._batch_max_tokens())
        except Exception:
            return
        if answered is None:
            return
        response, cost, tokens = answered
        content = _response_field(response, "content", "content") or ""
        for event, assessment in zip(events, parse_batch_assessments(content, len(events), tokens, cost)):
            apply_assessment(event, assessment)
        for event in events:
            for handler in list(self._batch_assessed_handlers):
                try:
                    handler(event)
                except Exception:  # noqa: BLE001
                    pass
