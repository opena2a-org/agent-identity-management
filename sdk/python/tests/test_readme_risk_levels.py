"""
The risk level each @agent.perform_action example in the README's Capability
decorators section names in its comment is the level detect_risk_level
returns for that capability.

The README said db:read is medium (db: medium, :read low, the higher wins).
detect_risk_level reads SPECIFIC_CAPABILITY_MAP before the namespace and
action tables, and that map lists db:read as low.
"""

import re
from pathlib import Path

import pytest

from aim_sdk.risk_detector import detect_risk_level

README = Path(__file__).resolve().parents[1] / "README.md"

# A decorator line whose comment opens with a risk level, for example
#   @agent.perform_action(capability="db:read")   # low (...)
EXAMPLE = re.compile(
    r'^@agent\.perform_action\(capability="(?P<capability>[^"]+)"\)'
    r'\s+#\s+(?P<level>low|medium|high|critical)\b'
)


def _examples():
    return [
        (match.group("capability"), match.group("level"))
        for match in map(EXAMPLE.match, README.read_text(encoding="utf-8").splitlines())
        if match
    ]


def test_the_scan_finds_the_db_read_example():
    assert "db:read" in [capability for capability, _ in _examples()]


@pytest.mark.parametrize("capability, level", _examples())
def test_each_example_names_the_detected_level(capability, level):
    assert detect_risk_level(capability) == level


@pytest.mark.parametrize("capability, level", [
    ("user:delete", "critical"),
    ("db:read", "low"),
    ("payment:read", "critical"),
])
def test_the_levels_the_rule_sentence_names(capability, level):
    # The sentence before the examples names these three.
    assert detect_risk_level(capability) == level
