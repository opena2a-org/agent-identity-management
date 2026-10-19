"""The star-import tests give the same answer when pytest is handed the probes.

``python -m pytest tests/star_imports/detection.py tests/test_release_203_...py``
failed the star-import test: pytest rewrote the probe because it was named
on the command line, and the rewrite bound ``@py_builtins`` and ``@pytest_ar``
into the namespace the test reads.
"""

import ast
import subprocess
import sys
from pathlib import Path

TESTS = Path(__file__).resolve().parent
PROBES = sorted(
    p for p in (TESTS / "star_imports").glob("*.py") if p.name != "__init__.py"
)
STAR_IMPORT_TESTS = [
    f"{TESTS / 'test_release_203_p2_p3_findings.py'}::"
    "test_star_import_binds_no_stdlib_or_typing_name",
    f"{TESTS / 'test_cli_import_cost.py'}::"
    "test_star_import_and_submodule_attributes_still_work",
]


def test_every_probe_opts_out_of_assertion_rewriting():
    assert PROBES
    for probe in PROBES:
        doc = ast.get_docstring(ast.parse(probe.read_text(encoding="utf-8")))
        assert doc and "PYTEST_DONT_REWRITE" in doc, probe.name


def test_star_import_tests_pass_with_the_probes_on_the_command_line(tmp_path):
    proc = subprocess.run(
        [
            sys.executable, "-m", "pytest", "-p", "no:cacheprovider", "-q",
            *(str(p) for p in PROBES),
            *STAR_IMPORT_TESTS,
        ],
        cwd=tmp_path, capture_output=True, text=True, timeout=300,
    )
    assert proc.returncode == 0, proc.stdout[-3000:] + proc.stderr[-1000:]
