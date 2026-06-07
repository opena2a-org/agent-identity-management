"""
Causal-denial telemetry -- correlated-record schema.

One record joins three signals around a single denied (or allowed) action:
  - enforcement: the authorization outcome -- an OBSERVED FACT (from FGA).
  - intent:      what the agent was trying to do -- an INFERENCE (NanoMind).
  - detection:   the injection cause -- an INFERENCE, never ground truth.

The record structurally separates observed fact from detection inference:
``observed=True`` appears ONLY on enforcement. This enforces the rule that the
injection attribution is never labelled as the enforcement cause.

Two tiers exist. The full :class:`CorrelatedRecord` is authoritative and stays
local. The :class:`SharedIndicator` is the only thing that may leave the machine:
anonymized, carrying no payloads, paths, credentials, identifiers, or the
correlation key.

Records are serialized to disk and the wire as camelCase JSON (the Registry and
the TS SDK both speak camelCase); the Python dataclasses use snake_case fields
and :meth:`to_dict`/:func:`record_from_dict` bridge the two.
"""
from __future__ import annotations

import time
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Dict, Optional

#: Schema version stamped on every record and indicator.
TELEMETRY_SCHEMA_VERSION = "1.0"

# EnforcementDecision: "allow" | "deny"
# EnforcementOutcome: "ALLOW" | "DENY" | "DENY_INTENT" | "DENY_CONTEXT"
#                     | "DENY_CHAIN" | "DENY_ATTRIBUTE" | "ERROR"
# TechniqueSource: "apia" | "interim-mapping"
# Completeness: "full" | "partial-missing-detection" | "partial-missing-intent"
#               | "partial-missing-both"


def _now_iso() -> str:
    """UTC ISO-8601 timestamp with millisecond precision and a trailing Z
    (mirrors TS Date.toISOString)."""
    now = datetime.now(timezone.utc)
    return now.strftime("%Y-%m-%dT%H:%M:%S.") + f"{now.microsecond // 1000:03d}Z"


def _drop_none(d: Dict[str, object]) -> Dict[str, object]:
    """Omit keys whose value is None (matches TS optional-field omission)."""
    return {k: v for k, v in d.items() if v is not None}


# --------------------------------------------------------------------------- #
# Caller-supplied parts. ``observed``/``attribution`` are set by the builder,
# so the fact/inference separation cannot be bypassed by a caller.
# --------------------------------------------------------------------------- #
@dataclass
class EnforcementInput:
    decision: str  # "allow" | "deny"
    outcome: str
    capability: str
    resource: str
    source: str
    occurred_at: Optional[str] = None
    denied_by: Optional[str] = None
    denied_reason: Optional[str] = None
    scope: Optional[str] = None
    credential_ref: Optional[str] = None
    policy_id: Optional[str] = None
    trace_id: Optional[str] = None


@dataclass
class IntentInput:
    intent_class: str
    confidence: float
    blocked: bool
    source: str
    model_version: Optional[str] = None


@dataclass
class DetectionInput:
    injection_detected: bool
    confidence: float
    detector: str
    technique_source: str  # "apia" | "interim-mapping"
    attack_class: Optional[str] = None
    technique_id: Optional[str] = None
    detector_version: Optional[str] = None
    input_ref: Optional[str] = None
    detected_at: Optional[str] = None


# --------------------------------------------------------------------------- #
# Assembled parts (with the builder-owned flags).
# --------------------------------------------------------------------------- #
@dataclass
class EnforcementFact:
    """OBSERVED FACT -- the authorization outcome."""
    decision: str
    outcome: str
    capability: str
    resource: str
    source: str
    occurred_at: str
    observed: bool = True
    denied_by: Optional[str] = None
    denied_reason: Optional[str] = None
    scope: Optional[str] = None
    credential_ref: Optional[str] = None
    policy_id: Optional[str] = None
    trace_id: Optional[str] = None

    def to_dict(self) -> Dict[str, object]:
        return _drop_none({
            "observed": True,
            "decision": self.decision,
            "outcome": self.outcome,
            "deniedBy": self.denied_by,
            "deniedReason": self.denied_reason,
            "capability": self.capability,
            "resource": self.resource,
            "scope": self.scope,
            "credentialRef": self.credential_ref,
            "policyId": self.policy_id,
            "occurredAt": self.occurred_at,
            "traceId": self.trace_id,
            "source": self.source,
        })


@dataclass
class IntentInference:
    """INFERENCE -- the classified intent of the action."""
    intent_class: str
    confidence: float
    blocked: bool
    source: str
    observed: bool = False
    model_version: Optional[str] = None

    def to_dict(self) -> Dict[str, object]:
        return _drop_none({
            "observed": False,
            "intentClass": self.intent_class,
            "confidence": self.confidence,
            "blocked": self.blocked,
            "source": self.source,
            "modelVersion": self.model_version,
        })


@dataclass
class DetectionInference:
    """INFERENCE -- the injection cause. Attribution is always "inferred"."""
    injection_detected: bool
    confidence: float
    detector: str
    technique_source: str
    detected_at: str
    observed: bool = False
    attribution: str = "inferred"
    attack_class: Optional[str] = None
    technique_id: Optional[str] = None
    detector_version: Optional[str] = None
    input_ref: Optional[str] = None

    def to_dict(self) -> Dict[str, object]:
        return _drop_none({
            "observed": False,
            "attribution": "inferred",
            "injectionDetected": self.injection_detected,
            "attackClass": self.attack_class,
            "techniqueId": self.technique_id,
            "techniqueSource": self.technique_source,
            "confidence": self.confidence,
            "detector": self.detector,
            "detectorVersion": self.detector_version,
            "inputRef": self.input_ref,
            "detectedAt": self.detected_at,
        })


@dataclass
class AssemblyMeta:
    joined_by: str
    completeness: str
    joiner_version: str

    def to_dict(self) -> Dict[str, object]:
        return {
            "joinedBy": self.joined_by,
            "completeness": self.completeness,
            "joinerVersion": self.joiner_version,
        }


@dataclass
class CorrelatedRecord:
    """Full, authoritative correlated record. Stays local."""
    schema_version: str
    correlation_id: str
    record_id: str
    agent_id: str
    created_at: str
    enforcement: EnforcementFact
    assembly: AssemblyMeta
    intent: Optional[IntentInference] = None
    detection: Optional[DetectionInference] = None

    def to_dict(self) -> Dict[str, object]:
        out: Dict[str, object] = {
            "schemaVersion": self.schema_version,
            "correlationId": self.correlation_id,
            "recordId": self.record_id,
            "agentId": self.agent_id,
            "createdAt": self.created_at,
            "enforcement": self.enforcement.to_dict(),
            "assembly": self.assembly.to_dict(),
        }
        if self.intent is not None:
            out["intent"] = self.intent.to_dict()
        if self.detection is not None:
            out["detection"] = self.detection.to_dict()
        return out


@dataclass
class SharedIndicator:
    """Anonymized indicator -- the only shape permitted to leave the machine."""
    schema_version: str
    sensor_token: str
    event_type: str
    enforcement_outcome: str
    agent_category: str
    day_since_install: int
    runtime_env: str
    triggered_at: str
    technique_id: Optional[str] = None
    technique_source: Optional[str] = None
    detection_confidence: Optional[float] = None


@dataclass
class SharedIndicatorContext:
    sensor_token: str
    agent_category: str
    day_since_install: int
    runtime_env: str
    #: Override the derived event type if needed.
    event_type: Optional[str] = None


def _derive_completeness(has_intent: bool, has_detection: bool) -> str:
    if has_intent and has_detection:
        return "full"
    if not has_intent and not has_detection:
        return "partial-missing-both"
    return "partial-missing-intent" if has_detection else "partial-missing-detection"


def _new_record_id() -> str:
    try:
        return str(uuid.uuid4())
    except Exception:
        return f"rec_{int(time.time() * 1000)}_{uuid.uuid1().int & 0xffffffff}"


def build_correlated_record(
    correlation_id: str,
    agent_id: str,
    enforcement: EnforcementInput,
    intent: Optional[IntentInput] = None,
    detection: Optional[DetectionInput] = None,
    joined_by: str = "correlationId",
    joiner_version: str = TELEMETRY_SCHEMA_VERSION,
) -> CorrelatedRecord:
    """
    Assemble a correlated record. The builder owns the observed/attribution flags
    so the fact/inference separation cannot be bypassed by a caller.
    """
    intent_inf: Optional[IntentInference] = None
    if intent is not None:
        intent_inf = IntentInference(
            intent_class=intent.intent_class,
            confidence=intent.confidence,
            blocked=intent.blocked,
            source=intent.source,
            model_version=intent.model_version,
            observed=False,
        )

    detection_inf: Optional[DetectionInference] = None
    if detection is not None:
        detection_inf = DetectionInference(
            injection_detected=detection.injection_detected,
            confidence=detection.confidence,
            detector=detection.detector,
            technique_source=detection.technique_source,
            attack_class=detection.attack_class,
            technique_id=detection.technique_id,
            detector_version=detection.detector_version,
            input_ref=detection.input_ref,
            detected_at=detection.detected_at or _now_iso(),
            observed=False,
            attribution="inferred",
        )

    enforcement_fact = EnforcementFact(
        decision=enforcement.decision,
        outcome=enforcement.outcome,
        capability=enforcement.capability,
        resource=enforcement.resource,
        source=enforcement.source,
        occurred_at=enforcement.occurred_at or _now_iso(),
        denied_by=enforcement.denied_by,
        denied_reason=enforcement.denied_reason,
        scope=enforcement.scope,
        credential_ref=enforcement.credential_ref,
        policy_id=enforcement.policy_id,
        trace_id=enforcement.trace_id,
        observed=True,
    )

    return CorrelatedRecord(
        schema_version=TELEMETRY_SCHEMA_VERSION,
        correlation_id=correlation_id,
        record_id=_new_record_id(),
        agent_id=agent_id,
        created_at=_now_iso(),
        enforcement=enforcement_fact,
        intent=intent_inf,
        detection=detection_inf,
        assembly=AssemblyMeta(
            joined_by=joined_by,
            completeness=_derive_completeness(intent_inf is not None, detection_inf is not None),
            joiner_version=joiner_version,
        ),
    )


def assert_observed_invariant(record: CorrelatedRecord) -> None:
    """Defensive invariant check: only enforcement may be observed."""
    if record.enforcement.observed is not True:
        raise ValueError("enforcement fact must be observed=True")
    if record.intent is not None and record.intent.observed is not False:
        raise ValueError("intent must be an inference (observed=False)")
    if record.detection is not None and record.detection.observed is not False:
        raise ValueError("detection must be an inference (observed=False)")


def to_shared_indicator(
    record: CorrelatedRecord, ctx: SharedIndicatorContext
) -> SharedIndicator:
    """
    Reduce a full record to the anonymized shared indicator. Sensitive fields
    (correlationId, agentId, resource, capability, credentialRef, inputRef,
    deniedReason, traceId) are intentionally NOT carried over.
    """
    decision = record.enforcement.decision
    injected = bool(record.detection and record.detection.injection_detected)
    event_type = ctx.event_type or (
        "denied_injection_attempt"
        if injected and decision == "deny"
        else f"{decision}_action"
    )

    return SharedIndicator(
        schema_version=TELEMETRY_SCHEMA_VERSION,
        sensor_token=ctx.sensor_token,
        event_type=event_type,
        technique_id=record.detection.technique_id if record.detection else None,
        technique_source=record.detection.technique_source if record.detection else None,
        detection_confidence=record.detection.confidence if record.detection else None,
        enforcement_outcome=decision,
        agent_category=ctx.agent_category,
        day_since_install=ctx.day_since_install,
        runtime_env=ctx.runtime_env,
        triggered_at=_now_iso(),
    )


def record_from_dict(d: Dict[str, object]) -> CorrelatedRecord:
    """
    Reconstruct a :class:`CorrelatedRecord` from its camelCase JSON form (as
    written by the local writer). Used by the relay when reading records back.
    Raises on a structurally invalid record (caller swallows).
    """
    enf = d["enforcement"]  # type: ignore[index]
    assert isinstance(enf, dict)
    enforcement = EnforcementFact(
        decision=str(enf.get("decision")),
        outcome=str(enf.get("outcome")),
        capability=str(enf.get("capability", "")),
        resource=str(enf.get("resource", "")),
        source=str(enf.get("source", "")),
        occurred_at=str(enf.get("occurredAt", "")),
        denied_by=enf.get("deniedBy"),
        denied_reason=enf.get("deniedReason"),
        scope=enf.get("scope"),
        credential_ref=enf.get("credentialRef"),
        policy_id=enf.get("policyId"),
        trace_id=enf.get("traceId"),
        observed=True,
    )

    intent = None
    iv = d.get("intent")
    if isinstance(iv, dict):
        intent = IntentInference(
            intent_class=str(iv.get("intentClass", "")),
            confidence=float(iv.get("confidence", 0.0)),
            blocked=bool(iv.get("blocked", False)),
            source=str(iv.get("source", "")),
            model_version=iv.get("modelVersion"),
            observed=False,
        )

    detection = None
    dv = d.get("detection")
    if isinstance(dv, dict):
        detection = DetectionInference(
            injection_detected=bool(dv.get("injectionDetected", False)),
            confidence=float(dv.get("confidence", 0.0)),
            detector=str(dv.get("detector", "")),
            technique_source=str(dv.get("techniqueSource", "")),
            attack_class=dv.get("attackClass"),
            technique_id=dv.get("techniqueId"),
            detector_version=dv.get("detectorVersion"),
            input_ref=dv.get("inputRef"),
            detected_at=str(dv.get("detectedAt", "")),
            observed=False,
            attribution="inferred",
        )

    asm = d.get("assembly") or {}
    assert isinstance(asm, dict)
    assembly = AssemblyMeta(
        joined_by=str(asm.get("joinedBy", "correlationId")),
        completeness=str(asm.get("completeness", "partial-missing-both")),
        joiner_version=str(asm.get("joinerVersion", TELEMETRY_SCHEMA_VERSION)),
    )

    return CorrelatedRecord(
        schema_version=str(d.get("schemaVersion", TELEMETRY_SCHEMA_VERSION)),
        correlation_id=str(d.get("correlationId", "")),
        record_id=str(d.get("recordId", "")),
        agent_id=str(d.get("agentId", "")),
        created_at=str(d.get("createdAt", "")),
        enforcement=enforcement,
        intent=intent,
        detection=detection,
        assembly=assembly,
    )
