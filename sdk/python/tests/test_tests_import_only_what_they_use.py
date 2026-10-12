"""
Every test file imports only the names it uses.

An unused import in a test reads as a dependency the test does not have: a
reader looks for where ``MagicMock`` or ``AuthenticationError`` is exercised
and finds nothing. pyflakes (installed with flake8 by the ``dev`` extra) finds
them. An import kept for its side effect, a module the code under test reads
from ``sys.modules`` for example, carries ``# noqa: F401`` on its line, as
flake8 reads it. A star import is a different finding and is not counted here.
"""

import ast
from pathlib import Path

import pytest

checker = pytest.importorskip("pyflakes.checker", reason="pyflakes not installed")
messages = pytest.importorskip("pyflakes.messages", reason="pyflakes not installed")

TESTS_DIR = Path(__file__).resolve().parent


def _unused_imports(path):
    source = path.read_text(encoding="utf-8")
    lines = source.splitlines()
    found = checker.Checker(ast.parse(source, filename=str(path)), filename=str(path))
    unused = []
    for message in found.messages:
        if not isinstance(message, messages.UnusedImport):
            continue
        name = message.message_args[0]
        if name.endswith(".*"):
            continue
        line = lines[message.lineno - 1]
        if "# noqa: F401" in line or line.rstrip().endswith("# noqa"):
            continue
        unused.append(f"{path.relative_to(TESTS_DIR.parent)}:{message.lineno}: {name}")
    return unused


def test_no_test_file_imports_a_name_it_does_not_use():
    unused = []
    for path in sorted(TESTS_DIR.rglob("*.py")):
        unused.extend(_unused_imports(path))

    assert unused == [], "imported but unused:\n" + "\n".join(unused)
