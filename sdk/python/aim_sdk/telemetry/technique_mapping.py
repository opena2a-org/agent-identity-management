"""
Causal-denial telemetry -- interim attack-class to Threat Matrix mapping.

Until the injection analyzer (APIA) emits a precise technique ID, detection
inferences carry an interim mapping from a coarse injection attack class to a
representative Threat Matrix technique. Each entry is derived from the real
matrix taxonomy (agent-threat-matrix matrix.json v1.1): the attack class maps to
a matrix attackClass node that genuinely exists, and the technique ID is a member
of that node's technique set.

This is an INTERIM, judgment-based correspondence. The authoritative attack-class
to technique map is owned by Threat Matrix governance, and the canonical technique
registry is the Registry (``GET /api/v1/threat-matrix``). The snapshot below is
for offline, best-effort validation only; a record's ``technique_source`` stays
``interim-mapping`` so a trained detector can supersede it with ``apia`` and no
schema change.
"""
from __future__ import annotations

import re
from typing import Dict, NamedTuple, Optional

#: Threat Matrix snapshot this mapping was derived against.
MATRIX_SNAPSHOT_VERSION = "1.1"

#: Valid technique-ID format (snapshot).
TECHNIQUE_ID_RE = re.compile(r"^T-\d{4}$")

#: Snapshot of the 61 canonical technique IDs (matrix.json v1.1). The Registry
#: remains the source of truth; this is for offline best-effort validation.
KNOWN_TECHNIQUE_IDS = frozenset([
    "T-1001", "T-1002", "T-1003", "T-1004", "T-1005", "T-1006", "T-1007",
    "T-2001", "T-2002", "T-2003", "T-2004", "T-2005", "T-2006", "T-2007", "T-2008", "T-2009",
    "T-3001", "T-3002", "T-3003", "T-3004", "T-3005", "T-3006",
    "T-4001", "T-4002", "T-4003", "T-4004", "T-4005", "T-4006", "T-4007",
    "T-5001", "T-5002", "T-5003", "T-5004", "T-5005", "T-5006",
    "T-6001", "T-6002", "T-6003", "T-6004", "T-6005", "T-6006", "T-6007",
    "T-7001", "T-7002", "T-7003", "T-7004", "T-7005", "T-7006", "T-7007",
    "T-8001", "T-8002", "T-8003", "T-8004", "T-8005", "T-8006",
    "T-9001", "T-9002", "T-9003", "T-9004", "T-9005", "T-9006",
])


class InterimMappingEntry(NamedTuple):
    #: Representative Threat Matrix technique for this attack class.
    technique_id: str
    #: Matrix attackClass node this technique was drawn from (provenance).
    matrix_attack_class: str
    #: Why this correspondence (one line).
    note: str


#: Interim map: injection attack class -> representative technique. Keys are
#: normalized (lowercase, hyphenated). Every technique_id is a member of the
#: cited matrix_attack_class in matrix.json v1.1 (verified).
INTERIM_ATTACK_CLASS_MAP: Dict[str, InterimMappingEntry] = {
    # External content achieving override of agent behavior.
    "indirect": InterimMappingEntry("T-4002", "SOUL-HIJACK", "external content override"),
    # Instructions injected directly into the prompt / system prompt.
    "direct": InterimMappingEntry("T-1003", "SOUL-INJECT", "system-prompt injection"),
    # Injection carried in retrieved (RAG) content.
    "rag-embedded": InterimMappingEntry("T-2002", "RAG-POISON", "poisoned retrieval content"),
    # Gradual displacement of safety instructions across turns.
    "multi-turn": InterimMappingEntry("T-2004", "SOUL-DRIFT", "conversational drift"),
    # Malicious content arriving via a tool/MCP return channel.
    "tool-output": InterimMappingEntry("T-4007", "FAKETOOL-INJECT", "tool impersonation/injection"),
    # Overriding harm-avoidance / safety constraints.
    "jailbreak": InterimMappingEntry("T-2001", "SOUL-HV", "harm-avoidance override"),
}

# Common aliases normalized onto the canonical keys above.
_ALIASES: Dict[str, str] = {
    "prompt-injection": "direct",
    "direct-injection": "direct",
    "indirect-injection": "indirect",
    "indirect-prompt-injection": "indirect",
    "rag": "rag-embedded",
    "rag-poisoning": "rag-embedded",
    "multiturn": "multi-turn",
    "tool-injection": "tool-output",
}


def _normalize(attack_class: str) -> str:
    key = re.sub(r"[\s_]+", "-", attack_class.strip().lower())
    return _ALIASES.get(key, key)


def is_valid_technique_id(value: object) -> bool:
    """True if the ID is well-formed AND present in the snapshot."""
    return isinstance(value, str) and bool(TECHNIQUE_ID_RE.match(value)) and value in KNOWN_TECHNIQUE_IDS


def is_technique_id_format(value: object) -> bool:
    """True if the ID merely matches T-NNNN format (Registry may know newer IDs)."""
    return isinstance(value, str) and bool(TECHNIQUE_ID_RE.match(value))


def map_attack_class(attack_class: Optional[str]) -> Optional[InterimMappingEntry]:
    """
    Map a coarse attack class to its interim technique entry, or None when
    unknown. Never guesses -- an unmapped class yields no technique ID.
    """
    if not attack_class:
        return None
    return INTERIM_ATTACK_CLASS_MAP.get(_normalize(attack_class))


def interim_technique_fields(attack_class: Optional[str]) -> Dict[str, Optional[str]]:
    """
    Build the technique fields for a DetectionInput from an attack class. Returns
    ``technique_source: "interim-mapping"`` always; ``technique_id`` is set only
    when the class is known and maps to a valid technique.
    """
    entry = map_attack_class(attack_class)
    if entry and is_valid_technique_id(entry.technique_id):
        return {"technique_id": entry.technique_id, "technique_source": "interim-mapping"}
    return {"technique_id": None, "technique_source": "interim-mapping"}
