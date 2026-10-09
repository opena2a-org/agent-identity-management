"""The star-import tests give the same answer when pytest is handed the probes.

``python -m pytest tests/star_imports/detection.py tests/test_aim21_...py``
failed the AC9 star-import test: pytest rewrote the probe because it was named
on the command line, and the rewrite bound ``@py_builtins`` and ``@pytest_ar``
into the namespace the test reads.
"""

import ast
import subprocess
import sys
from pathlib import Path

TESTS = Path(__file__).resolve().parent
SDK_ROOT = TESTS.parent
PROBES = sorted(
    p for p in (TESTS / "star_imports").glob("*.py") if p.name != "__init__.py"
)
STAR_IMPORT_TESTS = [
    "tests/test_aim21_p2_p3_release_findings.py::"
    "test_AIM_21_AC9_star_import_binds_no_stdlib_or_typing_name",
    "tests/test_cli_import_cost.py::"
    "test_star_import_and_submodule_attributes_still_work",
]


def test_every_probe_opts_out_of_assertion_rewriting():
    assert PROBES
    for probe in PROBES:
        doc = ast.get_docstring(ast.parse(probe.read_text(encoding="utf-8")))
        assert doc and "PYTEST_DONT_REWRITE" in doc, probe.name


def test_star_import_tests_pass_with_the_probes_on_the_command_line():
    proc = subprocess.run(
        [
            sys.executable, "-m", "pytest", "-p", "no:cacheprovider", "-q",
            *(str(p.relative_to(SDK_ROOT)) for p in PROBES),
            *STAR_IMPORT_TESTS,
        ],
        cwd=SDK_ROOT, capture_output=True, text=True, timeout=300,
    )
    assert proc.returncode == 0, proc.stdout[-3000:] + proc.stderr[-1000:]
