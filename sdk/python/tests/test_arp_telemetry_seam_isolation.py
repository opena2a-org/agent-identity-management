"""
AIM-01.AC2 / AC3 / AC4 -- the hard constraints on the ARP telemetry seam.

AC2 (2026-06-07 CA decision): detection output is a telemetry PRODUCER only. It
never enters ``verify_action`` or any PDP deny path, and the verification
modules do not import the seam.

AC3: the port stops at stage 1. No guard-socket client (stage 2) and no
interceptor patching of ``builtins`` / ``subprocess`` / ``socket`` (stage 3),
and the stage boundary is stated in the package's own docstring.

AC4: the seam is stdlib-only, matching the TS module's zero-runtime-dep posture.
"""
import ast
import os
from pathlib import Path

import pytest

import aim_sdk.telemetry as seam
from aim_sdk import AIMClient
from aim_sdk.exceptions import ActionDeniedError
from aim_sdk.telemetry import CorrelationJoiner, DetectionInput, IntentInput

AIM_SDK_DIR = Path(seam.__file__).resolve().parent.parent   # .../aim_sdk
SEAM_DIR = AIM_SDK_DIR / "telemetry"

#: The modules that decide allow/deny. Nothing here may know the seam exists.
VERIFICATION_MODULES = ["enforcement.py", "decision.py"]

#: Stage-1 seam modules. ``relay.py`` is the (already-shipped) sharing pipeline,
#: not part of the stage-1 seam, and uses ``requests`` -- an existing SDK
#: dependency -- so it is excluded from the stdlib-only assertion.
SEAM_MODULES = [
    "correlation.py",
    "correlated_record.py",
    "joiner.py",
    "local_writer.py",
    "technique_mapping.py",
]


def _parse(path):
    return ast.parse(Path(path).read_text(encoding="utf-8"), filename=str(path))


def _python_files(root):
    for dirpath, _dirnames, filenames in os.walk(root):
        for name in sorted(filenames):
            if name.endswith(".py"):
                yield Path(dirpath) / name


def _imported_roots(tree):
    """Top-level module names imported by a tree; relative imports are skipped."""
    roots = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                roots.add(alias.name.split(".")[0])
        elif isinstance(node, ast.ImportFrom):
            if node.level == 0 and node.module:
                roots.add(node.module.split(".")[0])
    return roots


# --------------------------------------------------------------------------- #
# AC2 -- detection output never reaches a verify/deny path.
# --------------------------------------------------------------------------- #
@pytest.mark.parametrize(
    "module", VERIFICATION_MODULES, ids=[f"AIM-01.AC2-{m}" for m in VERIFICATION_MODULES]
)
def test_AIM_01_AC2_verification_modules_do_not_import_the_seam(module):
    path = AIM_SDK_DIR / module

    # Every form: `import aim_sdk.telemetry`, `from aim_sdk.telemetry import x`,
    # `from .telemetry import x`, `from . import telemetry`.
    for node in ast.walk(_parse(path)):
        if isinstance(node, ast.Import):
            for alias in node.names:
                assert "telemetry" not in alias.name.split("."), f"{module}: {alias.name}"
        elif isinstance(node, ast.ImportFrom):
            assert "telemetry" not in (node.module or "").split("."), f"{module}: {node.module}"
            for alias in node.names:
                assert alias.name != "telemetry", f"{module}: from . import {alias.name}"


def test_AIM_01_AC2_only_the_telemetry_recorder_consumes_detection():
    """Call-graph: exactly one function in the client touches the detection part."""
    tree = _parse(AIM_SDK_DIR / "client.py")

    consumers = set()
    for node in ast.walk(tree):
        if not isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)):
            continue
        for inner in ast.walk(node):
            if not isinstance(inner, ast.Call):
                continue
            func = inner.func
            if isinstance(func, ast.Attribute) and func.attr == "ingest_detection":
                consumers.add(node.name)
            if (
                isinstance(func, ast.Attribute)
                and func.attr == "get"
                and inner.args
                and isinstance(inner.args[0], ast.Constant)
                and inner.args[0].value == "detection"
            ):
                consumers.add(node.name)

    assert consumers == {"_record_verification_telemetry"}


class _FakeResponse:
    def __init__(self, status_code, payload):
        self.status_code = status_code
        self._payload = payload
        self.text = ""

    def json(self):
        return self._payload

    def raise_for_status(self):
        return None


class _SessionSpy:
    def __init__(self, payload):
        self.payload = payload
        self.last_headers = None
        self.headers = {}

    def request(self, method, url, json=None, headers=None, data=None, timeout=None):
        self.last_headers = headers or {}
        return _FakeResponse(200, self.payload)

    def close(self):
        pass


class _ExplodingDetection:
    """A detection part that records every attribute read of it.

    Nothing on the verify/deny path may inspect a detection inference. Reads are
    recorded rather than raised because the telemetry recorder swallows
    exceptions -- a raise would be silently absorbed and the test would pass
    vacuously.
    """

    def __init__(self, touched):
        object.__setattr__(self, "_touched", touched)

    def __getattribute__(self, name):
        object.__getattribute__(self, "_touched").append(name)
        return object.__getattribute__(self, name)


class _SentinelJoiner(CorrelationJoiner):
    """Captures the detection part instead of assembling it into a record."""

    def __init__(self, seen, **kwargs):
        super().__init__(**kwargs)
        self.seen = seen

    def ingest_detection(self, correlation_id, detection):
        self.seen.append(detection)


def _client(payload, joiner):
    client = AIMClient(
        agent_id="agent-1", api_key="k", aim_url="http://localhost:9",
        telemetry={"enabled": True, "joiner": joiner},
    )
    client.session = _SessionSpy(payload)
    return client


def test_AIM_01_AC2_no_verify_or_deny_path_reads_the_detection_output():
    touched = []
    seen = []
    joiner = _SentinelJoiner(seen, now=lambda: 0, on_record=lambda r: None)
    client = _client({"id": "v1", "status": "approved"}, joiner)
    sentinel = _ExplodingDetection(touched)

    result = client.verify_capability(
        "db:read", resource="users", telemetry={"detection": sentinel}
    )

    assert result["verified"] is True
    # The seam received it (the test is not vacuous) ...
    assert len(seen) == 1 and seen[0] is sentinel
    # ... and no verify/deny code path read a single field off it.
    assert touched == []
    client.close()


def test_AIM_01_AC2_detection_output_cannot_change_a_verdict():
    """A maximal-confidence injection cannot deny, and no detection cannot allow."""
    records = []
    allow_joiner = CorrelationJoiner(now=lambda: 0, on_record=records.append)
    allowed = _client({"id": "v1", "status": "approved"}, allow_joiner)
    flagged = DetectionInput(
        injection_detected=True, confidence=1.0, detector="nanomind-guard",
        technique_source="apia", technique_id="T-2002",
    )
    intent = IntentInput(
        intent_class="exfil", confidence=1.0, blocked=True, source="nanomind-intent"
    )
    result = allowed.verify_capability(
        "db:read", resource="users",
        telemetry={"intent": intent, "detection": flagged},
    )
    # Detection said "injection, confidence 1.0"; the PDP said allow. Allow wins.
    assert result["verified"] is True
    # It was still captured as telemetry.
    assert len(records) == 1
    assert records[0].detection.injection_detected is True
    assert records[0].enforcement.decision == "allow"
    allowed.close()

    clean = DetectionInput(
        injection_detected=False, confidence=0.0, detector="nanomind-guard",
        technique_source="apia",
    )
    denied = _client(
        {"id": "v1", "status": "denied", "denialReason": "policy X"},
        CorrelationJoiner(now=lambda: 0, on_record=lambda r: None),
    )
    # A clean detection cannot rescue a denial either.
    with pytest.raises(ActionDeniedError):
        denied.verify_capability(
            "db:write", resource="secrets", telemetry={"detection": clean}
        )
    denied.close()


# --------------------------------------------------------------------------- #
# AC3 -- stages 2 and 3 are not begun.
# --------------------------------------------------------------------------- #
def test_AIM_01_AC3_stage_boundary_is_documented():
    doc = (seam.__doc__ or "").lower()
    assert "stage 1 of 3" in doc
    assert "guard-socket client" in doc          # stage 2, named
    assert "engine port" in doc                  # stage 3, named
    assert "arp-python-sdk-parity" in doc        # the recorded build order


def test_AIM_01_AC3_no_guard_socket_client_in_the_seam():
    # No AF_UNIX client anywhere in the telemetry package (the guard socket is
    # stage 2). Checked on the AST, so the docstring naming it does not count.
    for path in _python_files(SEAM_DIR):
        tree = _parse(path)
        for node in ast.walk(tree):
            if isinstance(node, ast.Attribute) and node.attr == "AF_UNIX":
                pytest.fail(f"{path.name}: AF_UNIX socket client is stage 2")
            if (
                isinstance(node, ast.Call)
                and isinstance(node.func, ast.Attribute)
                and node.func.attr == "socket"
                and isinstance(node.func.value, ast.Name)
                and node.func.value.id == "socket"
            ):
                pytest.fail(f"{path.name}: socket client is stage 2")

    # And no stage-2/stage-3 module has appeared in the SDK.
    assert not (AIM_SDK_DIR / "arp").exists()
    for path in _python_files(AIM_SDK_DIR):
        assert "guard" not in path.name.lower()
        assert "interceptor" not in path.name.lower()


def test_AIM_01_AC3_no_interceptor_patching_of_builtins_subprocess_or_socket():
    patched_modules = {"builtins", "subprocess", "socket", "os"}

    for path in _python_files(AIM_SDK_DIR):
        for node in ast.walk(_parse(path)):
            # `builtins.open = ...` / `subprocess.Popen = ...`
            if isinstance(node, ast.Assign):
                for target in node.targets:
                    if (
                        isinstance(target, ast.Attribute)
                        and isinstance(target.value, ast.Name)
                        and target.value.id in patched_modules
                    ):
                        pytest.fail(
                            f"{path.name}: patches {target.value.id}.{target.attr} "
                            "-- interceptors are stage 3"
                        )
            # `setattr(builtins, "open", ...)`
            if (
                isinstance(node, ast.Call)
                and isinstance(node.func, ast.Name)
                and node.func.id == "setattr"
                and node.args
                and isinstance(node.args[0], ast.Name)
                and node.args[0].id in patched_modules
            ):
                pytest.fail(
                    f"{path.name}: setattr on {node.args[0].id} "
                    "-- interceptors are stage 3"
                )


# --------------------------------------------------------------------------- #
# AC4 -- no dependency added: the seam is stdlib-only.
# --------------------------------------------------------------------------- #
@pytest.mark.parametrize(
    "module", SEAM_MODULES, ids=[f"AIM-01.AC4-{m}" for m in SEAM_MODULES]
)
def test_AIM_01_AC4_seam_modules_are_stdlib_only(module):
    import sys

    stdlib = getattr(sys, "stdlib_module_names", None)
    if stdlib is None:  # Python < 3.10
        pytest.skip("sys.stdlib_module_names requires Python 3.10+")

    for root in _imported_roots(_parse(SEAM_DIR / module)):
        assert root in stdlib or root == "aim_sdk", (
            f"{module} imports non-stdlib {root!r}: the seam matches the TS "
            "module's zero-runtime-dep posture"
        )
