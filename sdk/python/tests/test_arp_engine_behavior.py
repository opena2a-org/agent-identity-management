"""
The ARP engine port (``aim_sdk.arp``) on Python's real primitives, and the
constraints it ships under.

test_arp_engine_parity.py pins the port's classifications to the TS reference.
This file covers what only the Python side has: the interceptors patching
``builtins.open``, ``subprocess`` and ``socket`` for real and putting them
back, the engine's synchronous pipeline, the twin's local logs, the
coordinator's L2 gate and budget, the monitors against a live process table and
directory -- and that the package is a telemetry producer only: nothing on the
verify/deny path imports it, it imports nothing outside the standard library,
and importing it patches nothing.
"""
import ast
import builtins
import io
import json
import logging
import os
import socket
import subprocess
import sys
import threading
import time
from pathlib import Path

import pytest

import aim_sdk
from aim_sdk.arp import (
    ARPEvent,
    EnforcementResult,
    EventEngine,
    FilesystemInterceptor,
    FilesystemMonitor,
    InProcessBehavioralRiskSource,
    IntelligenceConfig,
    IntelligenceCoordinator,
    NetworkInterceptor,
    ProcessInterceptor,
    ProcessMonitor,
    RuntimeTwin,
    describe_l2_status,
)
from aim_sdk.arp.log_rotation import rotate_log_if_needed
from aim_sdk.arp.monitors import LISTING_COMMAND, parse_proc_net_tcp, parse_ps_listing

AIM_SDK_DIR = Path(aim_sdk.__file__).resolve().parent
ARP_DIR = AIM_SDK_DIR / "arp"


def _parse(path):
    return ast.parse(Path(path).read_text(encoding="utf-8"), filename=str(path))


def _imports_arp(tree, module_parts):
    """True when ``tree`` imports aim_sdk.arp in any form."""
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for alias in node.names:
                if alias.name.split(".")[:2] == ["aim_sdk", "arp"]:
                    return True
        elif isinstance(node, ast.ImportFrom):
            if node.level == 0:
                parts = (node.module or "").split(".")
            else:
                base = module_parts[: len(module_parts) - node.level]
                parts = base + ((node.module or "").split(".") if node.module else [])
            if parts[:2] == ["aim_sdk", "arp"]:
                return True
            if parts == ["aim_sdk"] and any(a.name == "arp" for a in node.names):
                return True
    return False


def _aim_sdk_modules():
    for dirpath, _dirs, files in os.walk(AIM_SDK_DIR):
        for name in sorted(files):
            if name.endswith(".py"):
                path = Path(dirpath) / name
                rel = path.relative_to(AIM_SDK_DIR.parent).with_suffix("")
                parts = list(rel.parts)
                if parts[-1] == "__init__":
                    parts = parts[:-1] + ["__init__"]
                yield path, parts


class _Recorder:
    def __init__(self):
        self.events = []

    def emit(self, event):
        self.events.append(event)
        return event


# --------------------------------------------------------------------------- #
# A telemetry producer only
# --------------------------------------------------------------------------- #
@pytest.mark.parametrize("module", ["enforcement.py", "decision.py"])
def test_the_verification_modules_do_not_import_the_engine(module):
    path = AIM_SDK_DIR / module
    assert not _imports_arp(_parse(path), ["aim_sdk", module[:-3]])
    source = path.read_text(encoding="utf-8")
    assert "aim_sdk.arp" not in source and "from .arp" not in source


def test_no_module_outside_the_engine_imports_it():
    """So nothing ARP detects can reach verify_action or any deny path."""
    importers = [
        str(path.relative_to(AIM_SDK_DIR))
        for path, parts in _aim_sdk_modules()
        if not str(path).startswith(str(ARP_DIR) + os.sep) and _imports_arp(_parse(path), parts)
    ]
    assert importers == []


def test_the_import_check_sees_every_form_of_import():
    forms = [
        ("import aim_sdk.arp", ["aim_sdk", "client"]),
        ("from aim_sdk.arp import EventEngine", ["aim_sdk", "client"]),
        ("from aim_sdk import arp", ["aim_sdk", "client"]),
        ("from .arp import engine", ["aim_sdk", "client"]),
        ("from . import arp", ["aim_sdk", "client"]),
        ("from ..arp import engine", ["aim_sdk", "integrations", "x"]),
    ]
    for source, parts in forms:
        assert _imports_arp(ast.parse(source), parts), source
    assert not _imports_arp(ast.parse("from .telemetry import x"), ["aim_sdk", "client"])


def test_the_engine_imports_only_the_standard_library_and_nothing_that_talks_to_a_network():
    stdlib = getattr(sys, "stdlib_module_names", None)
    if stdlib is None:  # Python < 3.10
        pytest.skip("sys.stdlib_module_names requires Python 3.10+")
    network = {"urllib", "http", "requests", "ssl", "ftplib", "smtplib", "xmlrpc"}
    for path in sorted(ARP_DIR.glob("*.py")):
        for node in ast.walk(_parse(path)):
            names = []
            if isinstance(node, ast.Import):
                names = [a.name.split(".")[0] for a in node.names]
            elif isinstance(node, ast.ImportFrom) and node.level == 0:
                names = [(node.module or "").split(".")[0]]
            for name in names:
                assert name in stdlib or name == "__future__", f"{path.name} imports {name}"
                assert name not in network, f"{path.name} imports {name}"


def test_importing_the_engine_patches_nothing():
    code = (
        "import builtins, io, os, socket, subprocess\n"
        "before = (builtins.open, io.open, os.remove, os.unlink, os.mkdir, os.system,\n"
        "          subprocess.Popen.__init__, socket.socket.__dict__.get('connect'))\n"
        "import aim_sdk.arp\n"
        "after = (builtins.open, io.open, os.remove, os.unlink, os.mkdir, os.system,\n"
        "         subprocess.Popen.__init__, socket.socket.__dict__.get('connect'))\n"
        "assert all(a is b for a, b in zip(before, after)), 'patched on import'\n"
    )
    # Import the package by path so the SDK's own dependencies are not needed.
    env = dict(os.environ, PYTHONPATH=str(AIM_SDK_DIR.parent))
    stub = "import sys, types; sys.modules['aim_sdk'] = types.ModuleType('aim_sdk'); " \
           "sys.modules['aim_sdk'].__path__ = [%r]\n" % str(AIM_SDK_DIR)
    result = subprocess.run([sys.executable, "-c", stub + code], env=env,
                            capture_output=True, text=True, timeout=60)
    assert result.returncode == 0, result.stderr


# --------------------------------------------------------------------------- #
# Interceptors on the real primitives
# --------------------------------------------------------------------------- #
@pytest.fixture
def engine():
    eng = EventEngine(rules=[])
    eng.seen = []
    eng.on_event(eng.seen.append)
    return eng


def _descriptions(eng, source):
    return [e.description for e in eng.seen if e.source == source]


def test_filesystem_interceptor_reports_builtin_open_pathlib_remove_and_mkdir(engine, tmp_path):
    secret = tmp_path / ".aws"
    secret.mkdir()
    hook = FilesystemInterceptor(engine, allowed_paths=[str(tmp_path / "work")])
    hook.start()
    try:
        with open(secret / "credentials", "w") as fh:
            fh.write("[default]\n")
        (secret / "config").write_text("x")                   # pathlib -> io.open
        assert (secret / "credentials").read_text() == "[default]\n"
        os.mkdir(tmp_path / "work")
        with open(tmp_path / "work" / "out.txt", "w") as fh:
            fh.write("ok")
        with open(tmp_path / "elsewhere.txt", "w") as fh:
            fh.write("ok")
        with open(tmp_path / "work" / "out.txt") as fh:         # plain read: no event
            assert fh.read() == "ok"
        os.remove(secret / "config")
    finally:
        hook.stop()

    got = [(e.category, e.severity, e.data["operation"]) for e in engine.seen]
    assert got == [
        ("violation", "high", "open:w"),
        ("violation", "high", "open:w"),
        ("violation", "high", "read"),
        ("normal", "info", "mkdir"),
        ("normal", "info", "open:w"),
        ("anomaly", "medium", "open:w"),
        ("violation", "critical", "delete"),
    ]
    assert all(e.data["intercepted"] is True for e in engine.seen)
    assert engine.seen[0].data["path"] == str(secret / "credentials")


def test_filesystem_interceptor_puts_every_original_back(engine):
    before = (builtins.open, io.open, os.remove, os.unlink, os.mkdir)
    hook = FilesystemInterceptor(engine)
    hook.start()
    assert builtins.open is not before[0] and io.open is builtins.open
    hook.start()  # a second start does not wrap twice
    hook.stop()
    hook.stop()
    assert (builtins.open, io.open, os.remove, os.unlink, os.mkdir) == before
    assert not hook.is_running()


def test_interceptors_stopped_out_of_order_still_unwind_to_the_original(engine):
    original = builtins.open
    first, second = FilesystemInterceptor(engine), FilesystemInterceptor(engine)
    first.start()
    second.start()
    first.stop()
    second.stop()
    assert builtins.open is original
    assert "connect" not in socket.socket.__dict__


def test_filesystem_interceptor_skips_excluded_paths_and_arps_own_writes(tmp_path):
    eng = EventEngine(rules=[])
    seen = []
    log = tmp_path / "handler.log"

    def write_log(event):
        seen.append(event)
        with open(log, "a") as fh:  # a handler's own write is not intercepted
            fh.write(event.description + "\n")

    eng.on_event(write_log)
    data_dir = tmp_path / "arp-data"
    data_dir.mkdir()
    hook = FilesystemInterceptor(eng, exclude_paths=[str(data_dir)])
    hook.start()
    try:
        with open(data_dir / ".env", "w") as fh:
            fh.write("x")
        with open(tmp_path / ".env", "w") as fh:
            fh.write("x")
    finally:
        hook.stop()
    assert [e.data["path"] for e in seen] == [str(tmp_path / ".env")]
    assert log.read_text().count("\n") == 1


def test_an_error_inside_arp_never_reaches_the_caller(tmp_path):
    class Broken:
        def emit(self, event):
            raise RuntimeError("engine down")

    hooks = [FilesystemInterceptor(Broken()), ProcessInterceptor(Broken()), NetworkInterceptor(Broken())]
    for hook in hooks:
        hook.start()
    try:
        with open(tmp_path / ".env", "w") as fh:
            fh.write("still written")
        assert subprocess.run([sys.executable, "-c", "pass"]).returncode == 0
    finally:
        for hook in hooks:
            hook.stop()
    assert (tmp_path / ".env").read_text() == "still written"


def test_process_interceptor_reports_popen_run_shell_and_os_system(engine):
    original_init, original_system = subprocess.Popen.__init__, os.system
    hook = ProcessInterceptor(engine)
    hook.start()
    try:
        assert subprocess.run([sys.executable, "-c", "pass"]).returncode == 0
        out = subprocess.run("echo hi | cat", shell=True, capture_output=True, text=True)
        assert out.stdout == "hi\n"
        subprocess.run(" ; ".join(["true"] * 11), shell=True)
        assert os.system("true") == 0
        assert isinstance(subprocess.Popen([sys.executable, "-c", "pass"]), subprocess.Popen)
    finally:
        hook.stop()
    assert subprocess.Popen.__init__ is original_init and os.system is original_system

    events = [e for e in engine.seen if e.source == "process" and "correlationKey" not in e.data]
    assert events[0].data["binary"] == os.path.basename(sys.executable)
    assert events[0].data["args"] == ["-c", "pass"]
    assert events[1].data["command"] == "echo hi | cat"
    assert events[1].category == "normal"
    assert (events[2].category, events[2].severity) == ("threat", "critical")
    assert events[2].data["subcommandCount"] == 11
    assert events[3].data["command"] == "true"
    assert len(events) == 5


def test_network_interceptor_reports_tcp_connects_and_not_unix_sockets(engine, tmp_path):
    server = socket.socket()
    server.bind(("127.0.0.1", 0))
    server.listen(1)
    port = server.getsockname()[1]
    hook = NetworkInterceptor(engine, allowed_hosts=["example.com"])
    hook.start()
    try:
        client = socket.create_connection(("127.0.0.1", port), timeout=5)
        client.close()
        probe = socket.socket()
        probe.connect_ex(("127.0.0.1", port))
        probe.close()
        if hasattr(socket, "AF_UNIX"):
            unix = socket.socket(socket.AF_UNIX)
            unix.connect_ex("arp-none.sock")  # relative: AF_UNIX paths are short
            unix.close()
    finally:
        hook.stop()
        server.close()
    assert "connect" not in socket.socket.__dict__
    assert "connect_ex" not in socket.socket.__dict__

    events = [e for e in engine.seen if e.source == "network"]
    assert [(e.data["remoteAddr"], e.data["remotePort"], e.category) for e in events] == [
        ("127.0.0.1", port, "anomaly"),
        ("127.0.0.1", port, "anomaly"),
    ]


def test_events_are_emitted_before_the_call_proceeds(tmp_path):
    order = []
    eng = EventEngine(rules=[])
    eng.on_event(lambda e: order.append(("event", os.path.exists(e.data["path"]))))
    hook = FilesystemInterceptor(eng)
    hook.start()
    try:
        with open(tmp_path / ".netrc", "w"):
            order.append(("opened", True))
    finally:
        hook.stop()
    assert order == [("event", False), ("opened", True)]


# --------------------------------------------------------------------------- #
# Engine
# --------------------------------------------------------------------------- #
def test_emit_refuses_an_event_without_a_data_dict():
    eng = EventEngine()
    with pytest.raises(TypeError, match="event must be a mapping"):
        eng.emit("not an event")
    with pytest.raises(TypeError, match="event.data must be a dict"):
        eng.emit({"source": "process", "category": "normal", "severity": "info", "description": "x"})


def test_the_executor_carries_out_matched_actions_and_handlers_see_one_answer():
    calls, results = [], []

    class Executor:
        def execute(self, action, event, target_pid=None):
            calls.append(action)
            if event.description == "throws":
                raise RuntimeError("no such pid")
            if event.description == "disagrees":
                return EnforcementResult(action=action, success=True, outcome="no-target",
                                         reason="gone", event=event)
            return EnforcementResult(action=action, success=True, reason="done", event=event)

    eng = EventEngine(executor=Executor())
    eng.on_enforcement(results.append)
    eng.on_event(lambda e: 1 / 0)  # a failing handler does not stop the pipeline
    for description in ("ok", "throws", "disagrees"):
        eng.emit({"source": "process", "category": "violation", "severity": "high",
                  "description": description, "data": {}})

    assert calls == ["alert", "alert", "alert"]
    assert [(r.success, r.outcome) for r in results] == [
        (True, "executed"), (False, "failed"), (False, "no-target"),
    ]
    assert results[1].reason == 'Rule "high-violation" matched; the alert action threw: no such pid'


def test_a_plain_callable_works_as_the_executor():
    eng = EventEngine(executor=lambda action, event: EnforcementResult(
        action=action, success=False, reason="refused", event=event))
    results = []
    eng.on_enforcement(results.append)
    eng.emit({"source": "network", "category": "threat", "severity": "critical",
              "description": "x", "data": {}})
    assert [(r.action, r.outcome) for r in results] == [("kill", "failed")]


def test_threshold_rules_count_only_events_inside_the_window():
    clock = [1_000_000.0]
    eng = EventEngine(rules=[{"name": "burst", "condition": {"threshold": {"count": 3, "windowMs": 1000}},
                              "action": "alert"}], clock=lambda: clock[0])
    fired = []
    eng.on_enforcement(fired.append)
    for step in (0, 600, 600, 600):
        clock[0] += step
        eng.emit({"source": "process", "category": "normal", "severity": "info",
                  "description": "t%d" % clock[0], "data": {}})
    assert len(fired) == 0
    clock[0] += 100
    eng.emit({"source": "process", "category": "normal", "severity": "info", "description": "x", "data": {}})
    assert len(fired) == 1


def test_emit_is_safe_from_many_threads():
    eng = EventEngine(rules=[])
    seen = []
    eng.on_event(seen.append)

    def worker(n):
        for i in range(200):
            eng.emit({"source": "process", "category": "normal", "severity": "info",
                      "description": "%d-%d" % (n, i), "data": {}})

    threads = [threading.Thread(target=worker, args=(n,)) for n in range(4)]
    for t in threads:
        t.start()
    for t in threads:
        t.join()
    assert len(seen) == 800


# --------------------------------------------------------------------------- #
# Runtime twin
# --------------------------------------------------------------------------- #
def test_the_twin_logs_every_event_and_scores_once_warmed_up(tmp_path):
    clock = [1_000_000.0]
    twin = RuntimeTwin("agent-1", data_dir=str(tmp_path), clock=lambda: clock[0])
    source = InProcessBehavioralRiskSource(twin, "runtime-twin-inproc")
    eng = EventEngine(rules=[], clock=lambda: clock[0])
    twin.attach(eng)

    def emit(**data):
        clock[0] += 100
        return eng.emit({"source": "process", "category": "normal", "severity": "info",
                         "description": "x", "data": data})

    first = emit(capability="db:read")
    assert source.get_behavioral_risk_signal(first, 25)["code"] == "NOT_READY"
    for _ in range(99):
        emit(capability="db:read")

    lines = (tmp_path / "events.jsonl").read_text().splitlines()
    assert len(lines) == 100
    record = json.loads(lines[0])
    assert set(record) == {
        "agentId", "sessionId", "sequenceNum", "eventType", "capability", "toolName", "argHash",
        "timestampDelta", "wallClock", "responseSize", "responseCode", "l0Decision",
    }
    assert record["capability"] == "db:read" and record["sequenceNum"] == 1

    clock[0] += 100
    event = ARPEvent(id="e", timestamp="", source="process", category="threat", severity="critical",
                     description="x", data={"capability": "secrets:write", "_initialAction": "kill"})
    signal = source.get_behavioral_risk_signal(event, 25)
    assert signal["status"] == "ok"
    assert signal["signal"]["action"] == "throttle"   # 0.3 unknown + 0.3 block
    assert signal["signal"]["source"] == "runtime-twin-inproc"
    assert len((tmp_path / "events.jsonl").read_text().splitlines()) == 100  # scoring is read-only

    eng.emit({"source": "process", "category": "threat", "severity": "critical", "description": "x",
              "data": {"capability": "secrets:write"}})
    escalation = json.loads((tmp_path / "l1-escalations.jsonl").read_text().splitlines()[-1])
    assert escalation["agentId"] == "agent-1" and escalation["score"] > 0.4


def test_the_twin_loads_a_saved_baseline_but_never_from_an_unsafe_agent_id(tmp_path):
    saved = {"eventTypeCounts": {"TOOL_CALL": 150}, "avgTimingDelta": 0, "stdTimingDelta": 0,
             "capabilitySet": ["unknown"], "avgResponseSize": 0, "totalEvents": 150, "errorRate": 0}
    (tmp_path / "baseline-agent-1.json").write_text(json.dumps(saved))
    ready = RuntimeTwin("agent-1", data_dir=str(tmp_path))
    event = ARPEvent(id="e", timestamp="", source="process", category="normal", severity="info",
                     description="x", data={})
    assert ready.score_arp_event(event) == {"score": 0.0, "action": "allow", "reason": "Normal behavior"}
    assert RuntimeTwin("../agent-1", data_dir=str(tmp_path)).score_arp_event(event) is None
    assert RuntimeTwin("agent-1", enabled=False, data_dir=str(tmp_path)).score_arp_event(event) is None


def test_log_rotation_keeps_the_newest_copies(tmp_path):
    log = tmp_path / "events.jsonl"
    for _ in range(4):
        log.write_text("x" * 20)
        assert rotate_log_if_needed(str(log), max_bytes=10, keep=2)
        time.sleep(0.002)
    assert not rotate_log_if_needed(str(log), max_bytes=10)
    assert len([p for p in tmp_path.iterdir() if p.name.startswith("events.jsonl.")]) == 2


# --------------------------------------------------------------------------- #
# Coordinator
# --------------------------------------------------------------------------- #
class _Adapter:
    name = "ollama"
    endpoint = "http://127.0.0.1:11434"

    def __init__(self, fail=False):
        self.fail = fail
        self.calls = 0

    def assess(self, prompt, max_tokens):
        self.calls += 1
        if self.fail:
            raise ConnectionError("refused")
        return {"content": "CONSISTENT: NO\nCONFIDENCE: 0.9\nREASONING: r\nACTION: PAUSE",
                "input_tokens": 100, "output_tokens": 20}

    def estimate_cost(self, input_tokens, output_tokens):
        return 0.001

    def health_check(self):
        return not self.fail


def _flagged():
    return ARPEvent(id="e", timestamp="", source="network", category="anomaly", severity="medium",
                    description="x", data={})


def test_l2_is_off_unless_switched_on_with_an_adapter_object(tmp_path, caplog):
    assert describe_l2_status(None) == {"running": False, "reason": "intelligence.enabled is not true"}
    assert IntelligenceCoordinator("a", str(tmp_path)).get_l2_status() == {
        "running": False, "reason": "not-enabled"}
    with caplog.at_level(logging.WARNING, logger="aim_sdk.arp"):
        by_name = IntelligenceCoordinator("a", str(tmp_path), {"enabled": True, "adapter": "anthropic"})
    assert by_name.get_l2_status() == {"running": False, "adapter": "anthropic", "reason": "adapter-failed"}
    assert "L2 intelligence is not running" in caplog.text
    assert IntelligenceCoordinator("a", str(tmp_path), {"enabled": True}).get_l2_status() == {
        "running": False, "reason": "no-adapter"}
    with pytest.raises(TypeError, match="must be a boolean"):
        IntelligenceConfig(enabled="false")

    adapter = _Adapter()
    off = IntelligenceCoordinator("a", str(tmp_path), {"adapter": adapter})
    assert off.analyze(_flagged()) is None and adapter.calls == 0


def test_l2_calls_stay_inside_the_hourly_cap_and_failures_are_counted(tmp_path, caplog):
    adapter = _Adapter()
    coordinator = IntelligenceCoordinator(
        "a", str(tmp_path / "ok"), IntelligenceConfig(enabled=True, adapter=adapter, max_calls_per_hour=1))
    event = _flagged()
    assessment = coordinator.analyze(event)
    assert assessment.recommendation == "pause"
    assert (event.category, event.severity, event.classified_by) == ("violation", "high", "L2-llm")
    assert coordinator.analyze(_flagged()) is None and adapter.calls == 1
    status = coordinator.get_budget_status()
    assert status["totalCalls"] == 1 and status["callsThisHour"] == 1
    assert json.loads((tmp_path / "ok" / "budget.json").read_text())["totalCalls"] == 1

    broken = _Adapter(fail=True)
    failing = IntelligenceCoordinator("a", str(tmp_path / "bad"), {"enabled": True, "adapter": broken})
    with caplog.at_level(logging.WARNING, logger="aim_sdk.arp"):
        assert failing.analyze(_flagged()) is None
        assert failing.analyze(_flagged()) is None
        failing.check_l2_health(timeout_ms=500)
    assert failing.get_budget_status()["failedCalls"] == 2
    assert failing.get_budget_status()["callsThisHour"] == 0
    assert caplog.text.count("L2 call failed") == 1  # once per run of failures
    assert "curl -sS http://127.0.0.1:11434/api/tags" in caplog.text


def test_the_full_pipeline_wires_like_the_ts_runtime_protection(tmp_path):
    twin = RuntimeTwin("agent-1", data_dir=str(tmp_path))
    coordinator = IntelligenceCoordinator(
        "agent-1", str(tmp_path),
        behavioral_risk_source=InProcessBehavioralRiskSource(twin, "runtime-twin-inproc"))
    eng = EventEngine()
    eng.on_event(coordinator.analyze)
    twin.attach(eng)
    hook = FilesystemInterceptor(eng, exclude_paths=[str(tmp_path)])
    hook.start()
    try:
        with open(Path(os.getcwd()) / ".pgpass", "w") as fh:
            fh.write("x")
    finally:
        hook.stop()
    event = next(e for e in eng.get_recent_events(60000) if e.source == "filesystem")
    assert event.data["behavioralRisk"] == {"status": "unavailable", "code": "NOT_READY"}
    assert (tmp_path / "events.jsonl").exists()


# --------------------------------------------------------------------------- #
# Monitors
# --------------------------------------------------------------------------- #
def test_ps_listing_leaves_out_its_own_row():
    listing = (
        "  101     1 me   0.0  0.1 /usr/bin/python3 agent.py\n"
        "  202   101 me   0.0  0.0 %s\n"
        "  203   101 me  99.0  0.0 %s\n"
        "garbage\n" % (LISTING_COMMAND, LISTING_COMMAND)
    )
    assert [r["pid"] for r in parse_ps_listing(listing, listing_pid=202, agent_pid=101)] == [101, 203]


def test_proc_net_tcp_keeps_established_rows_only():
    content = (
        "  sl  local_address rem_address   st\n"
        "   0: 0100007F:1F90 0F02000A:01BB 01 0\n"
        "   1: 0100007F:1F91 00000000:0000 0A 0\n"
    )
    assert parse_proc_net_tcp(content) == [{
        "protocol": "tcp", "localAddr": "127.0.0.1", "localPort": 8080,
        "remoteAddr": "10.0.2.15", "remotePort": 443, "state": "ESTABLISHED",
    }]


@pytest.mark.skipif(sys.platform == "win32", reason="ps is POSIX")
def test_process_monitor_sees_a_child_start_and_end():
    rec = _Recorder()
    monitor = ProcessMonitor(rec, interval_ms=60000)
    monitor.start()
    if monitor.read_process_table() is None:
        monitor.stop()
        pytest.skip("ps is not available here")
    child = subprocess.Popen([sys.executable, "-c", "import time; time.sleep(30)"])
    try:
        monitor.poll()
        started = [e for e in rec.events if e["data"].get("pid") == child.pid]
        assert started and started[0]["description"].startswith("New child process: PID %d" % child.pid)
        assert any(e["data"].get("binary", "").startswith("python") for e in started) or \
            os.path.basename(sys.executable) not in ("python", "python3")
    finally:
        child.kill()
        child.wait()
    rec.events.clear()
    monitor.poll()
    monitor.stop()
    assert {"source": "process", "category": "normal", "severity": "info",
            "description": "Child process terminated: PID %d" % child.pid,
            "data": {"pid": child.pid, "action": "terminated"}} in rec.events
    assert not monitor.is_running()


def test_filesystem_monitor_compares_snapshots(tmp_path):
    rec = _Recorder()
    (tmp_path / "keep.txt").write_text("a")
    monitor = FilesystemMonitor(rec, watch_paths=[str(tmp_path)], interval_ms=60000)
    monitor.start()
    try:
        assert monitor.is_running()
        (tmp_path / "new.txt").write_text("b")
        (tmp_path / ".npmrc").write_text("token")
        monitor.poll()
    finally:
        monitor.stop()
    assert [(e["category"], e["data"]["path"]) for e in rec.events] == [
        ("violation", ".npmrc"), ("normal", "new.txt"),
    ]
