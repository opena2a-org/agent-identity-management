"""
Every AgentEventType member referenced under aim_sdk/ must exist.

The OpenAI and Anthropic auto-instrumentation hooks referenced
AgentEventType.AGENT_ACTION, a member the enum never defined, inside a
try/except that swallowed the AttributeError, so the hooks recorded nothing in
every published version that carried them and nothing noticed. A reference to
an undefined member is a call that can never succeed; this test makes the
class of defect a failing test instead of a silent skip, the way
test_cited_commands.py does for commands named in help text.
"""

import pathlib
import re

from aim_sdk.security_logging import AgentEventType

PACKAGE = pathlib.Path(__file__).resolve().parent.parent / "aim_sdk"
REFERENCE = re.compile(r"AgentEventType\.([A-Z_][A-Z0-9_]*)")


def _references():
    found = {}
    for path in sorted(PACKAGE.rglob("*.py")):
        for match in REFERENCE.finditer(path.read_text(encoding="utf-8")):
            found.setdefault(match.group(1), []).append(str(path.relative_to(PACKAGE)))
    return found


def test_the_package_references_at_least_one_event_type():
    assert _references(), "the scan found no AgentEventType references; the regex or the path is wrong"


def test_every_referenced_agent_event_type_is_defined():
    defined = {member.name for member in AgentEventType}
    undefined = {name: files for name, files in _references().items() if name not in defined}
    assert not undefined, f"referenced but not defined on AgentEventType: {undefined}"
