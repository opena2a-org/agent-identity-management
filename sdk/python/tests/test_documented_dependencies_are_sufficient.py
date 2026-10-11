"""
The dependency lists a user installs from are enough to run the SDK (#664).

The SDK download carries requirements.txt, and the guides told users to run
``pip install keyring PyNaCl requests cryptography``. Neither named PyJWT,
which aim_sdk.oauth imports, so with only those packages installed
``aim-sdk login`` and ``from aim_sdk import secure`` stopped with
``ModuleNotFoundError: No module named 'jwt'``. The suite did not notice:
conftest.py installs a stand-in ``jwt`` module when PyJWT is missing.

These tests hold requirements.txt and every documented install command to
setup.py's install_requires, and import the SDK in a fresh interpreter (no
conftest stand-ins) that refuses any third-party module the SDK imports
directly unless requirements.txt names the distribution providing it.
"""

import ast
import importlib.metadata
import re
import subprocess
import sys
from pathlib import Path

import pytest

SDK_DIR = Path(__file__).resolve().parent.parent
REPO_ROOT = SDK_DIR.parent.parent

INSTALL_COMMAND = re.compile(r"\b(?:pip install|poetry add|pipenv install)\s+([^`\n]+)")

# The install commands #664 named, by file and how many each carries.
ISSUE_SITES = {
    "docs/sdk/python.md": 3,
    "docs/integrations/mcp.md": 1,
    "docs/examples/weather-agent.md": 1,
    "docs/examples/flight-tracker.md": 1,
    "docs/examples/database-agent.md": 1,
}


def _normalize(name):
    return re.sub(r"[-_.]+", "-", name).lower()


def _distribution_name(requirement):
    match = re.match(r"\s*['\"]?([A-Za-z0-9][A-Za-z0-9._-]*)", requirement)
    return _normalize(match.group(1)) if match else None


def _install_requires():
    tree = ast.parse((SDK_DIR / "setup.py").read_text(encoding="utf-8"))
    for node in ast.walk(tree):
        if isinstance(node, ast.Call) and getattr(node.func, "id", None) == "setup":
            for keyword in node.keywords:
                if keyword.arg == "install_requires":
                    return {_distribution_name(elt.value) for elt in keyword.value.elts}
    raise AssertionError("setup.py has no install_requires")


def _requirements_txt():
    names = set()
    text = (SDK_DIR / "requirements.txt").read_text(encoding="utf-8")
    for line in text.splitlines():
        line = line.split("#", 1)[0].strip()
        if line:
            names.add(_distribution_name(line))
    return names


def _sdk_install_commands():
    """Every install command in the docs that installs the SDK's dependencies.

    A command that names PyNaCl is one: the SDK is the only thing these docs
    install it for.
    """
    paths = set((REPO_ROOT / "docs").rglob("*.md")) | set((SDK_DIR / "docs").rglob("*.md"))
    paths |= {REPO_ROOT / "README.md", SDK_DIR / "README.md"}
    commands = []
    for path in sorted(paths):
        lines = path.read_text(encoding="utf-8").splitlines()
        for lineno, line in enumerate(lines, start=1):
            for match in INSTALL_COMMAND.finditer(line):
                names = {
                    _distribution_name(token)
                    for token in match.group(1).split()
                    if not token.startswith("-")
                }
                if "pynacl" in names:
                    site = path.relative_to(REPO_ROOT).as_posix()
                    commands.append((f"{site}:{lineno}", names))
    return commands


SDK_INSTALL_COMMANDS = _sdk_install_commands()


def test_requirements_txt_lists_every_distribution_setup_py_requires():
    missing = _install_requires() - _requirements_txt()
    assert missing == set(), (
        f"sdk/python/requirements.txt ships in the SDK download but omits "
        f"{sorted(missing)}, which setup.py's install_requires declares"
    )


def test_the_install_commands_named_in_664_are_checked():
    found = {}
    for site, _names in SDK_INSTALL_COMMANDS:
        path = site.rsplit(":", 1)[0]
        found[path] = found.get(path, 0) + 1
    for path, count in ISSUE_SITES.items():
        assert found.get(path, 0) == count, (
            f"expected {count} SDK install command(s) in {path}, found "
            f"{found.get(path, 0)}; the scan no longer sees them"
        )


@pytest.mark.parametrize(
    "site,names", SDK_INSTALL_COMMANDS, ids=[site for site, _ in SDK_INSTALL_COMMANDS]
)
def test_documented_install_command_lists_every_distribution_setup_py_requires(site, names):
    missing = _install_requires() - names
    assert missing == set(), (
        f"{site} installs the SDK's dependencies but omits {sorted(missing)}, "
        f"which setup.py's install_requires declares"
    )


IMPORT_ONLY_REQUIREMENTS = r"""
import importlib.metadata
import re
import sys

sdk_dir, allowed = sys.argv[1], set(sys.argv[2].split(","))
provided_by = importlib.metadata.packages_distributions()


def normalize(name):
    return re.sub(r"[-_.]+", "-", name).lower()


def importer():
    frame = sys._getframe(2)
    while frame is not None and "importlib" in frame.f_code.co_filename:
        frame = frame.f_back
    return frame.f_globals.get("__name__", "") if frame is not None else ""


class OnlyRequirementsTxt:
    # Refuses a top-level import made directly by aim_sdk when the installed
    # distribution providing it is not named in requirements.txt, as if it
    # were not installed. Imports made by those distributions themselves pass,
    # and an optional import the SDK guards with try/except falls back.
    def find_spec(self, fullname, path=None, target=None):
        if path is not None or fullname not in provided_by:
            return None
        if any(normalize(dist) in allowed for dist in provided_by[fullname]):
            return None
        name = importer()
        if name == "aim_sdk" or name.startswith("aim_sdk."):
            raise ModuleNotFoundError("No module named " + repr(fullname), name=fullname)
        return None


sys.meta_path.insert(0, OnlyRequirementsTxt())
sys.path.insert(0, sdk_dir)

from aim_sdk import secure  # noqa: E402,F401  the Quick start's verify line
import aim_sdk.cli  # noqa: E402,F401  aim-sdk's entry point
import aim_sdk.credentials  # noqa: E402,F401  loaded by aim-sdk login
import aim_sdk.client  # noqa: E402,F401  loaded by aim-sdk login
import aim_sdk.oauth  # noqa: E402,F401  loaded by aim_sdk.client
"""


def test_sdk_imports_with_only_the_requirements_txt_distributions(tmp_path):
    if sys.version_info < (3, 10):
        pytest.skip("importlib.metadata.packages_distributions needs Python 3.10+")
    allowed = _requirements_txt()
    for dist in sorted(allowed):
        try:
            importlib.metadata.distribution(dist)
        except importlib.metadata.PackageNotFoundError:
            pytest.skip(f"{dist} is not installed; conftest.py stands in for it")

    result = subprocess.run(
        [
            sys.executable,
            "-I",
            "-c",
            IMPORT_ONLY_REQUIREMENTS,
            str(SDK_DIR),
            ",".join(sorted(allowed)),
        ],
        cwd=tmp_path,
        env={"PATH": "/usr/bin:/bin", "HOME": str(tmp_path)},
        capture_output=True,
        text=True,
        timeout=120,
    )
    assert result.returncode == 0, (
        f"with only {sorted(allowed)} installed, importing the SDK failed:\n"
        f"{result.stderr[-2000:]}"
    )
