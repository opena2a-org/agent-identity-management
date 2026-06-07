"""Tests for the correlated-record schema, builder, and indicator reducer."""
import json

import pytest

from aim_sdk.telemetry import (
    DetectionInput,
    EnforcementInput,
    IntentInput,
    SharedIndicatorContext,
    TELEMETRY_SCHEMA_VERSION,
    assert_observed_invariant,
    build_correlated_record,
    record_from_dict,
    to_shared_indicator,
)

CID = "cde_000000000_aabbccddeeff"


def _enf(decision="deny"):
    return EnforcementInput(
        decision=decision,
        outcome="DENY_INTENT" if decision == "deny" else "ALLOW",
        capability="fs:write",
        resource="/etc/secret",
        source="aim-pdp",
        denied_reason="blocked injected exfil" if decision == "deny" else None,
        credential_ref="ref:db-prod",
    )


def _intent():
    return IntentInput(intent_class="exfil", confidence=0.7, blocked=True, source="nanomind-intent")


def _detection():
    return DetectionInput(
        injection_detected=True, confidence=0.84, detector="nanomind-guard",
        technique_source="interim-mapping", technique_id="T-2002", attack_class="indirect",
        input_ref="sha256:abcd",
    )


def test_builder_sets_observed_and_attribution_flags():
    rec = build_correlated_record(CID, "agent-1", _enf(), intent=_intent(), detection=_detection())
    assert rec.enforcement.observed is True
    assert rec.intent.observed is False
    assert rec.detection.observed is False
    assert rec.detection.attribution == "inferred"
    assert_observed_invariant(rec)  # does not raise


def test_completeness_derivation():
    assert build_correlated_record(CID, "a", _enf()).assembly.completeness == "partial-missing-both"
    assert build_correlated_record(CID, "a", _enf(), intent=_intent()).assembly.completeness == "partial-missing-detection"
    assert build_correlated_record(CID, "a", _enf(), detection=_detection()).assembly.completeness == "partial-missing-intent"
    assert build_correlated_record(CID, "a", _enf(), intent=_intent(), detection=_detection()).assembly.completeness == "full"


def test_to_dict_is_camelcase_and_roundtrips():
    rec = build_correlated_record(CID, "agent-1", _enf(), intent=_intent(), detection=_detection())
    d = rec.to_dict()
    assert d["schemaVersion"] == TELEMETRY_SCHEMA_VERSION
    assert set(["correlationId", "recordId", "agentId", "createdAt", "enforcement", "assembly", "intent", "detection"]).issubset(d.keys())
    assert d["enforcement"]["observed"] is True
    assert d["detection"]["attribution"] == "inferred"
    assert d["assembly"]["joinedBy"] == "correlationId"
    # round-trip through JSON
    back = record_from_dict(json.loads(json.dumps(d)))
    assert back.enforcement.decision == "deny"
    assert back.detection.technique_id == "T-2002"
    assert back.intent.intent_class == "exfil"


def test_shared_indicator_strips_identifiers():
    rec = build_correlated_record(CID, "agent-secret", _enf(), intent=_intent(), detection=_detection())
    ind = to_shared_indicator(rec, SharedIndicatorContext(
        sensor_token="tok", agent_category="cat", day_since_install=3, runtime_env="python"))
    assert ind.event_type == "denied_injection_attempt"
    assert ind.technique_id == "T-2002"
    assert ind.technique_source == "interim-mapping"
    assert ind.detection_confidence == 0.84
    assert ind.enforcement_outcome == "deny"
    assert ind.runtime_env == "python"
    # no identifiers anywhere on the indicator
    blob = json.dumps(ind.__dict__)
    for leaked in ("agent-secret", CID, "/etc/secret", "ref:db-prod", "sha256:abcd", "blocked injected exfil"):
        assert leaked not in blob


def test_shared_indicator_event_type_for_allow_and_non_injection():
    allow = build_correlated_record(CID, "a", _enf("allow"))
    ctx = SharedIndicatorContext(sensor_token="t", agent_category="c", day_since_install=0, runtime_env="python")
    assert to_shared_indicator(allow, ctx).event_type == "allow_action"
    deny_no_inj = build_correlated_record(CID, "a", _enf("deny"))
    assert to_shared_indicator(deny_no_inj, ctx).event_type == "deny_action"


def test_assert_observed_invariant_rejects_tampering():
    rec = build_correlated_record(CID, "a", _enf(), detection=_detection())
    rec.detection.observed = True  # tamper
    with pytest.raises(ValueError):
        assert_observed_invariant(rec)
