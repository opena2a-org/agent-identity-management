"""
AIM-01.AC1 -- ARP telemetry seam: cross-SDK schema parity with the TypeScript
reference.

The Python seam (``aim_sdk.telemetry``) and the TS seam
(``sdk/typescript/src/telemetry``) must feed ONE local log format: the same
``correlated-events.jsonl`` record shape and the same shared-indicator
reduction. These tests write a record from each representative input shape and
assert the emitted JSONL fields match the TS-derived fixtures in
``tests/fixtures/ts_telemetry/`` (see that directory's README for how the
fixtures were derived and why the comparison is canonicalized rather than raw
byte-ordered).
"""
import dataclasses
import json
import os
import re
from pathlib import Path

import pytest

from aim_sdk.telemetry import (
    CORRELATED_FILE,
    DetectionInput,
    EnforcementInput,
    IntentInput,
    SharedIndicatorContext,
    build_correlated_record,
    read_correlated_records,
    to_shared_indicator,
    write_correlated_record,
)

FIXTURE_DIR = Path(__file__).parent / "fixtures" / "ts_telemetry"

CASES = [
    "full-join",
    "deny-detection-only",
    "allow-intent-only",
    "enforcement-only",
]

# TS ``Date.prototype.toISOString`` -- millisecond precision, trailing Z.
ISO_MS_Z = re.compile(r"^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\.\d{3}Z$")

# The TS reference lives in the same repo. Absent from a packaged sdist, so the
# interface-drift guard skips rather than fails when it is not there.
#   .../sdk/python/tests/<this file>  ->  parents[2] == .../sdk
TS_REFERENCE = (
    Path(__file__).resolve().parents[2]
    / "typescript" / "src" / "telemetry" / "correlated-record.ts"
)


def _load(case):
    with open(FIXTURE_DIR / f"{case}.json", "r", encoding="utf-8") as fh:
        return json.load(fh)


def _snake(name):
    """camelCase (the wire/TS name) -> snake_case (the Python field name)."""
    return re.sub(r"(?<!^)(?=[A-Z])", "_", name).lower()


def _camel(name):
    """snake_case (the Python field name) -> camelCase (the wire/TS name)."""
    head, *rest = name.split("_")
    return head + "".join(part[:1].upper() + part[1:] for part in rest)


def _canonical(obj):
    """Canonical JSON bytes: key order removed, compact separators."""
    return json.dumps(obj, sort_keys=True, separators=(",", ":"))


def _build_from_fixture(fixture):
    """Build the Python record from the fixture's TS-named input.

    The camelCase -> snake_case conversion is itself part of the assertion: a
    field the Python dataclasses do not declare under the TS name raises
    TypeError here.
    """
    data = fixture["input"]
    enforcement = EnforcementInput(**{_snake(k): v for k, v in data["enforcement"].items()})
    intent = None
    if "intent" in data:
        intent = IntentInput(**{_snake(k): v for k, v in data["intent"].items()})
    detection = None
    if "detection" in data:
        detection = DetectionInput(**{_snake(k): v for k, v in data["detection"].items()})

    return build_correlated_record(
        correlation_id=data["correlationId"],
        agent_id=data["agentId"],
        enforcement=enforcement,
        intent=intent,
        detection=detection,
        joined_by=data["joinedBy"],
        joiner_version=data["joinerVersion"],
    )


def _indicator_wire(indicator):
    """The indicator as it is emitted: camelCase, undefined optionals omitted."""
    return {
        _camel(k): v
        for k, v in dataclasses.asdict(indicator).items()
        if v is not None
    }


def _strip_volatile(emitted, fixture):
    """Drop the per-call generated fields a fixture cannot pin."""
    return {k: v for k, v in emitted.items() if k not in fixture["volatileFields"]}


@pytest.mark.parametrize("case", CASES, ids=[f"AIM-01.AC1-{c}" for c in CASES])
def test_AIM_01_AC1_record_fields_match_the_ts_schema(case):
    fixture = _load(case)
    emitted = _build_from_fixture(fixture).to_dict()

    assert _canonical(_strip_volatile(emitted, fixture)) == _canonical(fixture["expectedRecord"])


@pytest.mark.parametrize("case", CASES, ids=[f"AIM-01.AC1-{c}" for c in CASES])
def test_AIM_01_AC1_written_jsonl_line_matches_the_ts_schema(case, tmp_path):
    """The record as it actually lands in the shared local log."""
    fixture = _load(case)
    record = _build_from_fixture(fixture)

    data_dir = str(tmp_path)
    assert write_correlated_record(record, data_dir) is True

    log_path = os.path.join(data_dir, CORRELATED_FILE)
    # Same log file name as the TS writer -- one local format for both SDKs.
    assert CORRELATED_FILE == "correlated-events.jsonl"

    with open(log_path, "r", encoding="utf-8") as fh:
        raw = fh.read()
    assert raw.endswith("\n")
    lines = [ln for ln in raw.split("\n") if ln]
    assert len(lines) == 1

    line = lines[0]
    parsed = json.loads(line)
    # Compact, one record per line -- as JSON.stringify emits.
    assert json.dumps(parsed, separators=(",", ":")) == line

    assert _canonical(_strip_volatile(parsed, fixture)) == _canonical(fixture["expectedRecord"])

    # The generated fields cannot be pinned in a fixture; assert their format.
    assert parsed["recordId"]
    assert ISO_MS_Z.match(parsed["createdAt"])

    # And the seam reads its own line back.
    back = read_correlated_records(data_dir)
    assert len(back) == 1
    assert back[0].correlation_id == fixture["input"]["correlationId"]


@pytest.mark.parametrize("case", CASES, ids=[f"AIM-01.AC1-{c}" for c in CASES])
def test_AIM_01_AC1_shared_indicator_reduction_matches_the_ts_schema(case):
    fixture = _load(case)
    record = _build_from_fixture(fixture)
    ctx = SharedIndicatorContext(
        **{_snake(k): v for k, v in fixture["sharedIndicatorContext"].items()}
    )

    wire = _indicator_wire(to_shared_indicator(record, ctx))
    assert ISO_MS_Z.match(wire["triggeredAt"])

    assert _canonical(_strip_volatile(wire, fixture)) == _canonical(fixture["expectedIndicator"])


@pytest.mark.parametrize(
    "interface,accessor",
    [
        ("CorrelatedRecord", lambda rec, ind: rec),
        ("EnforcementFact", lambda rec, ind: rec["enforcement"]),
        ("IntentInference", lambda rec, ind: rec["intent"]),
        ("DetectionInference", lambda rec, ind: rec["detection"]),
        ("AssemblyMeta", lambda rec, ind: rec["assembly"]),
        ("SharedIndicator", lambda rec, ind: ind),
    ],
    ids=[
        "AIM-01.AC1-CorrelatedRecord",
        "AIM-01.AC1-EnforcementFact",
        "AIM-01.AC1-IntentInference",
        "AIM-01.AC1-DetectionInference",
        "AIM-01.AC1-AssemblyMeta",
        "AIM-01.AC1-SharedIndicator",
    ],
)
def test_AIM_01_AC1_emitted_field_names_match_the_ts_interfaces(interface, accessor):
    """Drift guard read straight off the TS reference in this repo.

    The fixtures pin the shapes; this pins them to the source they were derived
    from, so a field added to (or renamed in) the TS interface fails here rather
    than silently forking the two SDKs' log format.
    """
    if not TS_REFERENCE.exists():  # packaged sdist: reference not shipped
        pytest.skip(f"TS reference not present at {TS_REFERENCE}")

    ts_fields = _ts_interface_fields(TS_REFERENCE.read_text(encoding="utf-8"), interface)
    assert ts_fields, f"could not parse TS interface {interface}"

    fixture = _load("full-join")  # every optional populated
    record = _build_from_fixture(fixture)
    ctx = SharedIndicatorContext(
        **{_snake(k): v for k, v in fixture["sharedIndicatorContext"].items()}
    )
    emitted = accessor(record.to_dict(), _indicator_wire(to_shared_indicator(record, ctx)))

    assert set(emitted.keys()) == ts_fields


def _ts_interface_fields(source, name):
    """Field names declared by ``export interface <name> { ... }``."""
    match = re.search(
        r"export interface " + re.escape(name) + r"\s*\{(.*?)\n\}", source, re.DOTALL
    )
    if not match:
        return set()
    fields = set()
    for line in match.group(1).split("\n"):
        found = re.match(r"\s*(?:readonly\s+)?([A-Za-z_][A-Za-z0-9_]*)\??\s*:", line)
        if found:
            fields.add(found.group(1))
    return fields
