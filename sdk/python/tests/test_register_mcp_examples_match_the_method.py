"""
Every example and manual script that calls ``AIMClient.register_mcp`` calls it
with the arguments it takes and reads members its answer has.

``register_mcp`` adds an MCP server to the agent's list. It takes the server's
ID or name and optionally ``detection_method``, ``confidence`` and
``metadata``, and returns what ``PUT /api/v1/agents/{id}/mcp-servers``
answers: ``message``, ``talksTo``, ``added_servers`` and ``total_count``. It
does not create an MCP server and does not attest one.

The MCP registration tutorial called it with ``server_name``, ``server_url``
and ``capabilities`` (Python raised ``TypeError``) and read an ``id``; five
manual scripts read ``added``, so each reported 0 servers for a call that
added one; and the MCP integration guide said the method attests a server.
"""

import ast
import inspect
import re
from pathlib import Path

import pytest

from aim_sdk import AIMClient

REPO = Path(__file__).resolve().parents[3]

PARAMETERS = [
    name for name in inspect.signature(AIMClient.register_mcp).parameters if name != "self"
]
ANSWER_MEMBERS = {"message", "talksTo", "added_servers", "total_count"}

# Markdown files with a Python example of register_mcp, and the manual scripts
# that call it.
EXAMPLE_DOCS = [
    "docs/tutorials/mcp-registration.md",
    "sdk/python/README.md",
]
SCRIPTS = sorted((REPO / "tests/scripts").glob("*.py"))

_PYTHON_BLOCK = re.compile(r"```python\n(.*?)```", re.S)


def _python_sources():
    for relative in EXAMPLE_DOCS:
        text = (REPO / relative).read_text(encoding="utf-8")
        for number, block in enumerate(_PYTHON_BLOCK.findall(text), start=1):
            yield f"{relative} (Python block {number})", block
    for script in SCRIPTS:
        yield str(script.relative_to(REPO)), script.read_text(encoding="utf-8")


def _register_mcp_uses():
    """(where, call, names bound to its answer, module) for every call."""
    for where, source in _python_sources():
        try:
            module = ast.parse(source)
        except SyntaxError:
            continue
        for node in ast.walk(module):
            if isinstance(node, ast.Assign) and isinstance(node.value, ast.Call):
                call = node.value
                targets = {t.id for t in node.targets if isinstance(t, ast.Name)}
            elif isinstance(node, ast.Expr) and isinstance(node.value, ast.Call):
                call, targets = node.value, set()
            else:
                continue
            if isinstance(call.func, ast.Attribute) and call.func.attr == "register_mcp":
                yield where, call, targets, module


USES = list(_register_mcp_uses())


def test_the_sources_are_found():
    # Guards the reader: the tutorial, the README and the scripts all call it.
    places = {where.split(" (")[0] for where, _, _, _ in USES}
    assert "docs/tutorials/mcp-registration.md" in places
    assert "sdk/python/README.md" in places
    assert sum(1 for p in places if p.startswith("tests/scripts/")) >= 5, places


@pytest.mark.parametrize("where,call", [(w, c) for w, c, _, _ in USES], ids=[w for w, _, _, _ in USES])
def test_register_mcp_is_called_with_the_arguments_it_takes(where, call):
    keywords = [k.arg for k in call.keywords if k.arg is not None]
    unknown = [k for k in keywords if k not in PARAMETERS]
    assert not unknown, f"{where}: register_mcp() takes {PARAMETERS}, not {unknown}"
    assert len(call.args) <= len(PARAMETERS), f"{where}: too many positional arguments"
    assert call.args or "mcp_server_id" in keywords or any(k.arg is None for k in call.keywords), (
        f"{where}: register_mcp() needs the MCP server's ID or name"
    )


def _members_read(module, names):
    """Members read from the answer: name['member'] and name.get('member', ...)."""
    for node in ast.walk(module):
        if (
            isinstance(node, ast.Subscript)
            and isinstance(node.value, ast.Name)
            and node.value.id in names
            and isinstance(node.slice, ast.Constant)
        ):
            yield node.slice.value
        if (
            isinstance(node, ast.Call)
            and isinstance(node.func, ast.Attribute)
            and node.func.attr == "get"
            and isinstance(node.func.value, ast.Name)
            and node.func.value.id in names
            and node.args
            and isinstance(node.args[0], ast.Constant)
        ):
            yield node.args[0].value


@pytest.mark.parametrize(
    "where,targets,module",
    [(w, t, m) for w, _, t, m in USES if t],
    ids=[w for w, _, t, _ in USES if t],
)
def test_the_answer_is_read_by_the_members_it_has(where, targets, module):
    read = set(_members_read(module, targets))
    assert read <= ANSWER_MEMBERS, (
        f"{where}: reads {sorted(read - ANSWER_MEMBERS)} from the answer of register_mcp(), "
        f"which has {sorted(ANSWER_MEMBERS)}"
    )


def test_the_integration_guide_does_not_say_register_mcp_attests():
    guide = (REPO / "sdk/python/docs/MCP_INTEGRATION.md").read_text(encoding="utf-8")
    claims = [
        line for line in guide.splitlines()
        if "register_mcp()" in line
        and "attest" in line.lower()
        and "does not attest" not in line.lower()
    ]
    assert claims == [], f"register_mcp() does not attest a server: {claims}"
