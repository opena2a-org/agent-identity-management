"""
Behavioral parity of the ARP engine port (``aim_sdk.arp``) with the TypeScript
reference (``sdk/typescript/src/arp``).

``fixtures/ts_arp/behavioral-cases.json`` holds inputs to the engine, the
interceptors, the monitors, the runtime twin and the coordinator, each with the
output the TS reference gives for it. The TS suite
(``sdk/typescript/tests/arp/python-port-parity.test.ts``) runs the same file
against the TS code; this suite runs it against the port. A behavior change on
either side fails one of the two until the other side and the file follow.

The second half pins the tables both implementations classify with (sensitive
paths, suspicious binaries and hosts, default rules, response bands, the L2
data allowlist) to the TS source text, so a list edited on one side only fails
here even when no case happens to exercise the new entry.
"""
import json
import re
from pathlib import Path

import pytest

from aim_sdk.arp import (
    ARPEvent,
    AlertRule,
    EventEngine,
    FilesystemInterceptor,
    FilesystemMonitor,
    IntelligenceConfig,
    IntelligenceCoordinator,
    NetworkInterceptor,
    NetworkMonitor,
    ProcessInterceptor,
    ProcessMonitor,
    RuntimeTwin,
    default_rules,
)
from aim_sdk.arp import coordinator as coordinator_module
from aim_sdk.arp import interceptors as interceptors_module
from aim_sdk.arp import monitors as monitors_module
from aim_sdk.arp import runtime_twin as twin_module
from aim_sdk.arp.monitors import (
    get_descendants,
    parse_hex_addr,
    parse_lsof,
    parse_netstat,
    parse_ss,
    split_addr_port,
)

CASES_FILE = Path(__file__).parent / "fixtures" / "ts_arp" / "behavioral-cases.json"
CASES = json.loads(CASES_FILE.read_text(encoding="utf-8"))["cases"]

#   .../sdk/python/tests/<this file>  ->  parents[2] == .../sdk
TS_ARP = Path(__file__).resolve().parents[2] / "typescript" / "src" / "arp"


def _plain(value):
    """JSON round trip, as the TS runner applies to every output."""
    return json.loads(json.dumps(value))


class _Recorder:
    """An engine stand-in that records what a monitor or interceptor emits."""

    def __init__(self):
        self.emitted = []

    def emit(self, event):
        self.emitted.append(event)
        return event


def _shape(event):
    out = event.to_dict()
    del out["id"]
    del out["timestamp"]
    return _plain(out)


def _event(raw):
    raw = _plain(raw)
    return ARPEvent(
        id="parity-event",
        timestamp="2026-01-01T00:00:00.000Z",
        source=raw["source"],
        category=raw["category"],
        severity=raw["severity"],
        description=raw["description"],
        data=raw["data"],
    )


class _Adapter:
    name = "parity"

    def __init__(self, content, input_tokens, output_tokens, cost, batch_cost=None):
        self.content = content
        self.input_tokens = input_tokens
        self.output_tokens = output_tokens
        self.cost = cost
        self.batch_cost = batch_cost
        self.prompts = []

    def assess(self, prompt, max_tokens):
        self.prompts.append(prompt)
        return {"content": self.content, "input_tokens": self.input_tokens,
                "output_tokens": self.output_tokens, "model": "parity"}

    def estimate_cost(self, input_tokens, output_tokens):
        if self.batch_cost is not None:
            return self.cost if output_tokens == 0 else self.batch_cost
        return self.cost

    def health_check(self):
        return True


class _RiskSource:
    def __init__(self, result):
        self.result = result

    def get_behavioral_risk_signal(self, event, timeout_ms):
        return _plain(self.result)


class _Drift:
    def __init__(self, status):
        self.status = status

    def record(self, label):
        return _plain(self.status)


def _run_engine(inp):
    rules = None if inp["rules"] is None else [AlertRule.from_dict(r) for r in inp["rules"]]
    engine = EventEngine(rules=rules)
    seen, enforcement = [], []
    engine.on_event(seen.append)
    engine.on_enforcement(lambda r: enforcement.append({
        "action": r.action, "success": r.success, "outcome": r.outcome,
        "reason": r.reason, "eventDescription": r.event.description,
    }))
    for raw in inp["events"]:
        engine.emit(_plain(raw))
    return {"events": [_shape(e) for e in seen], "enforcement": enforcement}


def _run_coordinator(inp, tmp_path):
    l2 = inp.get("l2")
    adapter = _Adapter(l2["content"], l2["inputTokens"], l2["outputTokens"], l2["cost"]) if l2 else None
    coordinator = IntelligenceCoordinator(
        "parity-agent",
        str(tmp_path),
        intelligence=IntelligenceConfig(enabled=True, adapter=adapter) if l2 else None,
        agent_description="Books flights for staff",
        declared_capabilities=["flights:search", "flights:book"],
        manifest=inp.get("manifest"),
        behavioral_risk_source=_RiskSource(inp["risk"]) if inp.get("risk") else None,
        guard_anomaly=_Drift(inp["guardStatus"]) if inp.get("guardStatus") else None,
    )
    event = _event(inp["event"])
    assessment = coordinator.analyze(event)
    coordinator.stop()
    return {
        "event": _shape(event),
        "assessment": assessment.to_dict() if assessment else None,
        "prompts": adapter.prompts if adapter else [],
    }


def _run_batch_parse(inp, tmp_path):
    adapter = _Adapter(inp["content"], inp["tokens"], 0, inp["cost"], batch_cost=0.001)
    coordinator = IntelligenceCoordinator(
        "parity-agent", str(tmp_path),
        intelligence=IntelligenceConfig(enabled=True, adapter=adapter, enable_batching=True),
    )
    events = []
    for i in range(inp["count"]):
        event = _event({"source": "network", "category": "anomaly", "severity": "medium",
                        "description": "event %d" % (i + 1), "data": {}})
        events.append(event)
        assert coordinator.analyze(event) is None  # queued for the batch
    coordinator.stop()
    return [_plain(e.llm_assessment.to_dict()) if e.llm_assessment else None for e in events]


def _run_case(kind, inp, tmp_path):
    if kind == "filesystem-interceptor":
        rec = _Recorder()
        hook = FilesystemInterceptor(rec, inp["allowedPaths"])
        if inp["op"] == "write":
            hook.handle_write(inp["path"], inp["operation"])
        elif inp["op"] == "read":
            hook.handle_read(inp["path"])
        else:
            hook.handle_delete(inp["path"])
        return _plain(rec.emitted)
    if kind == "process-spawn":
        rec = _Recorder()
        ProcessInterceptor(rec).handle_spawn(inp["command"], inp["args"])
        return _plain(rec.emitted)
    if kind == "process-exec":
        rec = _Recorder()
        ProcessInterceptor(rec).handle_exec(inp["command"])
        return _plain(rec.emitted)
    if kind == "network-interceptor":
        rec = _Recorder()
        NetworkInterceptor(rec, inp["allowedHosts"]).handle_connection(inp["host"], inp["port"])
        return _plain(rec.emitted)
    if kind == "engine":
        return _plain(_run_engine(inp))
    if kind == "coordinator":
        return _plain(_run_coordinator(inp, tmp_path))
    if kind == "batch-parse":
        return _plain(_run_batch_parse(inp, tmp_path))
    if kind == "twin-score":
        twin = RuntimeTwin("parity-agent", data_dir=str(tmp_path))
        twin._set_baseline_for_test(inp["baseline"])
        return _plain(twin.score_arp_event(_event(inp["event"])))
    if kind == "monitor-hex-addr":
        return _plain(parse_hex_addr(inp["value"]))
    if kind == "monitor-split-addr-port":
        return _plain(split_addr_port(inp["value"]))
    if kind == "monitor-ss":
        return _plain(parse_ss(inp["output"]))
    if kind == "monitor-lsof":
        return _plain(parse_lsof(inp["output"]))
    if kind == "monitor-netstat":
        return _plain(parse_netstat(inp["output"]))
    if kind == "process-monitor":
        rec = _Recorder()
        monitor = ProcessMonitor(rec)
        monitor._agent_pid = inp["agentPid"]
        monitor._known = set(inp["known"])
        monitor.process_snapshot(get_descendants(_plain(inp["table"]), inp["agentPid"]))
        return _plain(rec.emitted)
    if kind == "network-monitor":
        rec = _Recorder()
        monitor = NetworkMonitor(rec, 10000, inp["allowedHosts"])
        monitor._known = set(inp["known"])
        for connections in inp["polls"]:
            monitor.process_connections(_plain(connections))
        return _plain(rec.emitted)
    if kind == "filesystem-monitor":
        rec = _Recorder()
        FilesystemMonitor(rec, [], inp["allowedPaths"]).handle_event(
            inp["eventType"], inp["filename"], inp["basePath"]
        )
        return _plain(rec.emitted)
    raise AssertionError("unknown case kind: %s" % kind)


def test_the_shared_file_covers_every_ported_component():
    kinds = {c["kind"] for c in CASES}
    assert kinds == {
        "filesystem-interceptor", "process-spawn", "process-exec", "network-interceptor",
        "engine", "coordinator", "batch-parse", "twin-score",
        "monitor-hex-addr", "monitor-split-addr-port", "monitor-ss", "monitor-lsof",
        "monitor-netstat", "process-monitor", "network-monitor", "filesystem-monitor",
    }
    assert len(CASES) > 100


@pytest.mark.parametrize("case", CASES, ids=["%s: %s" % (c["kind"], c["name"]) for c in CASES])
def test_port_gives_the_ts_output(case, tmp_path):
    assert _run_case(case["kind"], case["input"], tmp_path) == case["expected"]


# --------------------------------------------------------------------------- #
# The classification tables, pinned to the TS source text.
# --------------------------------------------------------------------------- #
def _ts(relative):
    path = TS_ARP / relative
    if not path.exists():
        pytest.skip("TS reference not present (packaged sdist)")
    return path.read_text(encoding="utf-8")


def _string_array(source, name):
    match = re.search(r"const %s\b[^=]*=\s*(?:new Set\()?\[(.*?)\]" % name, source, re.S)
    assert match, "%s not found in the TS source" % name
    return re.findall(r"'([^']*)'", match.group(1))


@pytest.mark.parametrize("relative", ["interceptors/filesystem.ts", "monitors/filesystem.ts"])
def test_sensitive_paths_match_the_ts_lists(relative):
    assert list(monitors_module.SENSITIVE_PATHS) == _string_array(_ts(relative), "SENSITIVE_PATHS")


@pytest.mark.parametrize("relative", ["interceptors/process.ts", "monitors/process.ts"])
def test_suspicious_binaries_match_the_ts_lists(relative):
    assert list(monitors_module.SUSPICIOUS_BINARIES) == _string_array(_ts(relative), "SUSPICIOUS_BINARIES")


@pytest.mark.parametrize("relative", ["interceptors/network.ts", "monitors/network.ts"])
def test_suspicious_hosts_match_the_ts_lists(relative):
    assert list(monitors_module.SUSPICIOUS_HOSTS) == _string_array(_ts(relative), "SUSPICIOUS_HOSTS")


def test_max_subcommands_matches_the_ts_constant():
    match = re.search(r"const MAX_SUBCOMMANDS = (\d+);", _ts("interceptors/process.ts"))
    assert match and int(match.group(1)) == interceptors_module.MAX_SUBCOMMANDS


def test_ps_listing_arguments_match_the_ts_constant():
    assert list(monitors_module.LISTING_ARGS) == _string_array(_ts("monitors/process.ts"), "LISTING_ARGS")


def test_l2_data_allowlist_matches_the_ts_set():
    source = _ts("intelligence/coordinator.ts")
    assert sorted(coordinator_module.L2_ALLOWED_DATA_FIELDS) == sorted(
        _string_array(source, "L2_ALLOWED_DATA_FIELDS")
    )


def test_runtime_twin_response_table_matches_the_ts_table():
    source = _ts("intelligence/runtime-twin.ts")
    rows = re.findall(
        r"\{ threshold: ([\d.]+), action: '(\w+)', label: '([^']*)' \}", source
    )
    assert [(float(t), a, label) for t, a, label in rows] == list(twin_module.RESPONSE_TABLE)


def test_default_rules_match_the_ts_rules():
    source = _ts("engine/event-engine.ts")
    body = source[source.index("function defaultRules()"):]
    names = re.findall(r"name: '([^']+)'", body)
    actions = re.findall(r"action: '([^']+)'", body)
    confirm = [m.start() for m in re.finditer(r"requireLlmConfirmation: true", body)]
    starts = [m.start() for m in re.finditer(r"name: '", body)] + [len(body)]
    confirmed = [any(starts[i] < c < starts[i + 1] for c in confirm) for i in range(len(names))]

    rules = default_rules()
    assert [r.name for r in rules] == names
    assert [r.action for r in rules] == actions
    assert [r.require_llm_confirmation for r in rules] == confirmed
