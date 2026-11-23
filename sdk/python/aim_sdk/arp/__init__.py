"""
Agent Runtime Protection (ARP) for Python: the engine port.

This package is **stage 3 of 3** of the runtime-protection port tracked in
issue #441: the event engine (L0), the runtime twin (L1), the intelligence
coordinator (L2), the polling monitors, and the interceptors that patch
Python's own primitives (``builtins.open``, ``subprocess``, ``socket``). It
mirrors ``sdk/typescript/src/arp`` and ships with a behavioral suite
(``tests/test_arp_engine_parity.py`` with the shared cases in
``tests/fixtures/ts_arp/``, which the TS suite runs too) that fails when either
SDK drifts from the other.

ARP is a telemetry PRODUCER only. Its classifications drive local response and
telemetry; they never enter ``verify_action`` or any AIM allow/deny path.
``aim_sdk.enforcement`` and ``aim_sdk.decision`` do not import this package,
and no other ``aim_sdk`` module does either: it is used only by code that
imports it explicitly.

Wiring, as the TS ``AgentRuntimeProtection`` does it::

    from aim_sdk.arp import (
        EventEngine, IntelligenceCoordinator, RuntimeTwin,
        InProcessBehavioralRiskSource, FilesystemInterceptor,
        ProcessInterceptor, NetworkInterceptor,
    )

    engine = EventEngine()
    twin = RuntimeTwin("my-agent")
    coordinator = IntelligenceCoordinator(
        "my-agent", data_dir,
        behavioral_risk_source=InProcessBehavioralRiskSource(twin, "runtime-twin-inproc"),
    )
    engine.on_event(coordinator.analyze)
    twin.attach(engine)
    for hook in (ProcessInterceptor(engine), NetworkInterceptor(engine),
                 FilesystemInterceptor(engine, exclude_paths=[data_dir])):
        hook.start()

Not ported, and so absent here: the TS fleet-gradient upload, the vendor L2
adapters (the Python coordinator takes an adapter object from the caller), the
behavioral-risk unix-socket source, and the AI-layer (prompt/MCP/A2A)
interceptors. This package makes no network call of its own.
"""
from .anomaly import AnomalyDetector
from .behavioral_risk import DEFAULT_BEHAVIORAL_RISK_TIMEOUT_MS, InProcessBehavioralRiskSource
from .budget import BudgetController
from .coordinator import (
    IntelligenceConfig,
    IntelligenceCoordinator,
    allowlist_event_data_for_l2,
    describe_l2_status,
)
from .engine import CorrelationEngine, EventEngine, default_rules
from .interceptors import (
    MAX_SUBCOMMANDS,
    FilesystemInterceptor,
    NetworkInterceptor,
    ProcessInterceptor,
    analyze_command_complexity,
)
from .monitors import (
    SENSITIVE_PATHS,
    SUSPICIOUS_BINARIES,
    SUSPICIOUS_HOSTS,
    FilesystemMonitor,
    NetworkMonitor,
    ProcessMonitor,
)
from .runtime_twin import RuntimeTwin
from .types import (
    ARPEvent,
    AlertCondition,
    AlertRule,
    EnforcementResult,
    LLMAssessment,
)

__all__ = [
    "ARPEvent",
    "AlertCondition",
    "AlertRule",
    "EnforcementResult",
    "LLMAssessment",
    "EventEngine",
    "CorrelationEngine",
    "default_rules",
    "RuntimeTwin",
    "InProcessBehavioralRiskSource",
    "DEFAULT_BEHAVIORAL_RISK_TIMEOUT_MS",
    "AnomalyDetector",
    "BudgetController",
    "IntelligenceConfig",
    "IntelligenceCoordinator",
    "allowlist_event_data_for_l2",
    "describe_l2_status",
    "ProcessMonitor",
    "NetworkMonitor",
    "FilesystemMonitor",
    "FilesystemInterceptor",
    "ProcessInterceptor",
    "NetworkInterceptor",
    "analyze_command_complexity",
    "MAX_SUBCOMMANDS",
    "SENSITIVE_PATHS",
    "SUSPICIOUS_BINARIES",
    "SUSPICIOUS_HOSTS",
]
