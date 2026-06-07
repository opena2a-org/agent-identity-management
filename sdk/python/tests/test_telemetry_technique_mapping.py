"""Tests for the interim attack-class -> Threat Matrix technique mapping."""
from aim_sdk.telemetry import (
    INTERIM_ATTACK_CLASS_MAP,
    KNOWN_TECHNIQUE_IDS,
    interim_technique_fields,
    is_technique_id_format,
    is_valid_technique_id,
    map_attack_class,
)
from aim_sdk.telemetry.technique_mapping import _normalize


def test_every_mapped_technique_is_a_known_id():
    for entry in INTERIM_ATTACK_CLASS_MAP.values():
        assert entry.technique_id in KNOWN_TECHNIQUE_IDS


def test_known_id_count_snapshot():
    assert len(KNOWN_TECHNIQUE_IDS) == 61


def test_map_attack_class_normalizes_and_aliases():
    assert map_attack_class("prompt-injection").technique_id == "T-1003"
    assert map_attack_class("Indirect Injection").technique_id == "T-4002"
    assert map_attack_class("RAG").technique_id == "T-2002"
    assert map_attack_class("multiturn").technique_id == "T-2004"
    assert map_attack_class("unknown-class") is None
    assert map_attack_class(None) is None


def test_normalize_collapses_separators():
    assert _normalize("Multi Turn") == "multi-turn"
    assert _normalize("tool_output") == "tool-output"


def test_validators():
    assert is_valid_technique_id("T-2002")
    assert not is_valid_technique_id("T-9999")  # well-formed but not in snapshot
    assert not is_valid_technique_id("nope")
    assert is_technique_id_format("T-9999")  # format-only accepts newer IDs
    assert not is_technique_id_format("T-99")


def test_interim_technique_fields_always_returns_source():
    known = interim_technique_fields("indirect")
    assert known == {"technique_id": "T-4002", "technique_source": "interim-mapping"}
    unknown = interim_technique_fields("never-heard-of-it")
    assert unknown == {"technique_id": None, "technique_source": "interim-mapping"}
