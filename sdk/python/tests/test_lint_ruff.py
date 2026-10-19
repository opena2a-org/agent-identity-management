"""
aim_sdk/client.py and aim_sdk/credentials.py pass ruff's pyflakes rules (F)
and its pycodestyle error rules E4, E7 and E9.

The rules are named here and ruff runs with --isolated, so neither a
configuration file nor the installed ruff's default rule set decides what is
checked. Skipped when ruff is not installed; the dev extra installs it.
"""

import importlib.util
import subprocess
import sys
from pathlib import Path

import pytest

SDK_ROOT = Path(__file__).resolve().parents[1]
RULES = "E4,E7,E9,F"
FILES = ["aim_sdk/client.py", "aim_sdk/credentials.py"]

pytestmark = pytest.mark.skipif(
    importlib.util.find_spec("ruff") is None,
    reason="ruff is not installed (pip install -e '.[dev]')",
)


def test_client_and_credentials_pass_ruff(tmp_path):
    result = subprocess.run(
        [
            sys.executable, "-m", "ruff", "check", "--isolated", "--no-cache",
            "--select", RULES, "--output-format", "concise",
            *(str(SDK_ROOT / f) for f in FILES),
        ],
        cwd=tmp_path,
        capture_output=True,
        text=True,
        timeout=120,
    )
    assert result.returncode == 0, result.stdout + result.stderr
