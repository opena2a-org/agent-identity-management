"""
AIM-13 -- MCP detection payloads report the installed package version.

AC1  Rows built through `auto_detect_mcps()`, `MCPDetector()` and
     `MCPDetector.get_runtime_detections()` with no `sdk_version` argument
     carry `sdkVersion` equal to f"aim-sdk-python@{aim_sdk.__version__}".
AC2  None of the three `sdk_version` parameters carries a version-bearing
     default bound at module import: each default is `None` and each resolves
     the package version inside its own body at call time.
AC3  Neither `aim_sdk/detection.py` nor `aim_sdk/protocol_detection.py` binds
     a module-level `__version__` to a string constant.
AC4  No live site under `aim_sdk/` reports a hard-coded SDK version: no
     `aim-sdk-python@<d>.<d>.<d>` string constant outside a docstring, and no
     detection entry point called with a string-constant `sdk_version`.
AC5  `VERSION` moved past 2.0.2, `setup.py` still derives its version from
     the VERSION file, and the topmost CHANGELOG section equals VERSION,
     names issue #464 and states the fix.
"""

import ast
import inspect
import json
import re
from pathlib import Path

import pytest

import aim_sdk
from aim_sdk import detection
from aim_sdk.detection import MCPDetector, auto_detect_mcps

SDK_ROOT = Path(aim_sdk.__file__).resolve().parent          # .../sdk/python/aim_sdk
PYTHON_SDK_ROOT = SDK_ROOT.parent                           # .../sdk/python

EXPECTED_SDK_VERSION = f"aim-sdk-python@{aim_sdk.__version__}"
HARDCODED_VERSION_RE = re.compile(r"aim-sdk-python@\d+\.\d+\.\d+")


# ---------------------------------------------------------------------------
# Fixtures: deterministic rows without network
# ---------------------------------------------------------------------------

@pytest.fixture
def planted_claude_config(tmp_path, monkeypatch):
    """Plant one mcpServers entry under a temporary HOME.

    The resolver at detection._get_claude_config_path is pathlib.Path.home()
    based, so pointing HOME at tmp_path makes the claude_config rows
    deterministic and network-free.
    """
    config_dir = tmp_path / ".claude"
    config_dir.mkdir()
    config_path = config_dir / "claude_desktop_config.json"
    config_path.write_text(json.dumps({
        "mcpServers": {
            "filesystem": {
                "command": "npx",
                "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
            }
        }
    }))
    monkeypatch.setenv("HOME", str(tmp_path))
    monkeypatch.setenv("USERPROFILE", str(tmp_path))
    return "filesystem"


@pytest.fixture
def clean_call_tracker(monkeypatch):
    """Isolate the global runtime-call tracker for the test."""
    monkeypatch.setattr(detection, "_mcp_call_tracker", {})


# ---------------------------------------------------------------------------
# AC1 -- default rows carry the installed package version
# ---------------------------------------------------------------------------

def test_AIM_13_AC1_auto_detect_mcps_default_row_carries_installed_version(planted_claude_config):
    rows = auto_detect_mcps()
    config_rows = [r for r in rows if r["detectionMethod"] == "claude_config"]
    assert config_rows, "planted mcpServers entry should yield a claude_config row"
    assert config_rows[0]["mcpServer"] == planted_claude_config
    for row in config_rows:
        assert row["sdkVersion"] == EXPECTED_SDK_VERSION


def test_AIM_13_AC1_mcp_detector_default_row_carries_installed_version(planted_claude_config):
    rows = MCPDetector().detect_all()
    config_rows = [r for r in rows if r["detectionMethod"] == "claude_config"]
    assert config_rows, "planted mcpServers entry should yield a claude_config row"
    for row in config_rows:
        assert row["sdkVersion"] == EXPECTED_SDK_VERSION


def test_AIM_13_AC1_runtime_detections_default_row_carries_installed_version(clean_call_tracker):
    MCPDetector.track_mcp_call("filesystem", "read_file")
    rows = MCPDetector.get_runtime_detections()
    runtime_rows = [r for r in rows if r["detectionMethod"] == "sdk_runtime"]
    assert len(runtime_rows) == 1
    assert runtime_rows[0]["mcpServer"] == "filesystem"
    assert runtime_rows[0]["sdkVersion"] == EXPECTED_SDK_VERSION


# ---------------------------------------------------------------------------
# AC2 -- no version-bearing default bound at module import
# ---------------------------------------------------------------------------

def test_AIM_13_AC2_all_three_sdk_version_defaults_are_none():
    assert inspect.signature(MCPDetector.__init__).parameters["sdk_version"].default is None
    assert inspect.signature(MCPDetector.get_runtime_detections).parameters["sdk_version"].default is None
    assert inspect.signature(auto_detect_mcps).parameters["sdk_version"].default is None


def test_AIM_13_AC2_each_entry_point_resolves_the_package_version_at_call_time(
    planted_claude_config, clean_call_tracker, monkeypatch
):
    # If the version were bound at import, mutating the package attribute now
    # could not reach rows built afterwards.
    monkeypatch.setattr(aim_sdk, "__version__", "9.9.9-sentinel")
    sentinel = "aim-sdk-python@9.9.9-sentinel"

    assert MCPDetector().sdk_version == sentinel

    MCPDetector.track_mcp_call("filesystem", "read_file")
    runtime_rows = MCPDetector.get_runtime_detections()
    assert runtime_rows and all(r["sdkVersion"] == sentinel for r in runtime_rows)

    config_rows = [
        r for r in auto_detect_mcps() if r["detectionMethod"] == "claude_config"
    ]
    assert config_rows and all(r["sdkVersion"] == sentinel for r in config_rows)


# ---------------------------------------------------------------------------
# AC3 -- no module-scope __version__ string literal in the two modules
# ---------------------------------------------------------------------------

def _module_scope_version_string_assignments(path: Path):
    tree = ast.parse(path.read_text(encoding="utf-8"))
    hits = []
    for node in tree.body:
        if isinstance(node, ast.Assign):
            targets = node.targets
            value = node.value
        elif isinstance(node, ast.AnnAssign):
            targets = [node.target]
            value = node.value
        else:
            continue
        for target in targets:
            if (
                isinstance(target, ast.Name)
                and target.id == "__version__"
                and isinstance(value, ast.Constant)
                and isinstance(value.value, str)
            ):
                hits.append(f"{path.name}:{node.lineno}")
    return hits


@pytest.mark.parametrize("module_file", ["detection.py", "protocol_detection.py"])
def test_AIM_13_AC3_no_module_level_version_string_literal(module_file):
    hits = _module_scope_version_string_assignments(SDK_ROOT / module_file)
    assert hits == [], f"module-scope __version__ string literal at {hits}"


@pytest.mark.parametrize("module_name", ["detection", "protocol_detection"])
def test_AIM_13_AC3_any_surviving_version_attribute_equals_the_package_version(module_name):
    module = getattr(__import__(f"aim_sdk.{module_name}"), module_name)
    if hasattr(module, "__version__"):
        assert module.__version__ == aim_sdk.__version__


# ---------------------------------------------------------------------------
# AC4 -- no hard-coded SDK version anywhere in the package
# ---------------------------------------------------------------------------

def _docstring_constant_ids(tree: ast.AST):
    doc_ids = set()
    for node in ast.walk(tree):
        if isinstance(node, (ast.Module, ast.FunctionDef, ast.AsyncFunctionDef, ast.ClassDef)):
            body = getattr(node, "body", [])
            if (
                body
                and isinstance(body[0], ast.Expr)
                and isinstance(body[0].value, ast.Constant)
                and isinstance(body[0].value.value, str)
            ):
                doc_ids.add(id(body[0].value))
    return doc_ids


def _call_target_name(func: ast.expr):
    if isinstance(func, ast.Name):
        return func.id
    if isinstance(func, ast.Attribute):
        return func.attr
    return None


def test_AIM_13_AC4_no_hardcoded_sdk_version_literal_or_argument_in_the_package():
    entry_points = {"MCPDetector", "auto_detect_mcps", "get_runtime_detections"}
    offenders = []

    for py_file in sorted(SDK_ROOT.rglob("*.py")):
        tree = ast.parse(py_file.read_text(encoding="utf-8"))
        doc_ids = _docstring_constant_ids(tree)
        rel = py_file.relative_to(SDK_ROOT)

        for node in ast.walk(tree):
            if (
                isinstance(node, ast.Constant)
                and isinstance(node.value, str)
                and id(node) not in doc_ids
                and HARDCODED_VERSION_RE.search(node.value)
            ):
                offenders.append(f"{rel}:{node.lineno} literal {node.value!r}")

            if isinstance(node, ast.Call) and _call_target_name(node.func) in entry_points:
                candidates = [
                    kw.value for kw in node.keywords if kw.arg == "sdk_version"
                ]
                if node.args:
                    candidates.append(node.args[0])
                for arg in candidates:
                    if isinstance(arg, ast.Constant) and isinstance(arg.value, str):
                        offenders.append(
                            f"{rel}:{node.lineno} string-constant sdk_version argument"
                        )

    assert offenders == [], "hard-coded SDK version sites: " + "; ".join(offenders)


# ---------------------------------------------------------------------------
# AC5 -- release notes name the fix and the version moved past 2.0.2
# ---------------------------------------------------------------------------

def test_AIM_13_AC5_version_moved_past_2_0_2_and_changelog_names_the_fix():
    version = (PYTHON_SDK_ROOT / "VERSION").read_text(encoding="utf-8").strip()
    assert tuple(int(p) for p in version.split(".")) > (2, 0, 2)

    setup_text = (PYTHON_SDK_ROOT / "setup.py").read_text(encoding="utf-8")
    assert '"VERSION"' in setup_text
    assert "version=version" in setup_text

    changelog = (PYTHON_SDK_ROOT / "CHANGELOG.md").read_text(encoding="utf-8")
    # Keep a Changelog keeps an ``[Unreleased]`` section above the newest release, so the
    # newest RELEASE heading, not the first heading, must name the installed version, and
    # the section read is that release's own, wherever it sits.
    headings = [h for h in re.findall(r"^## \[(.+?)\]", changelog, flags=re.MULTILINE)
                if h != "Unreleased"]
    assert headings, "CHANGELOG has no release headings"
    assert headings[0] == version

    sections = re.split(r"^## \[", changelog, flags=re.MULTILINE)[1:]
    top_section = next(s for s in sections if s.startswith(version + "]"))
    assert "#464" in top_section
    assert "MCP detection" in top_section
    assert "installed package version" in top_section
