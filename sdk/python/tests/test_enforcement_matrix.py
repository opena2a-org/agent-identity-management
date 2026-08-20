"""
The enforcement matrix: does the wrapped function body run?

This is the 2.0.0 release gate. It drives the REAL AIMClient and the REAL
decorators with a real Ed25519 keypair, stubbing only `session.request`. The
observable is whether the wrapped body executed -- never a log line, never a
mock's call count, because a log line can be emitted next to an action that ran
anyway and that is precisely the defect this release fixes.

NOT marked `integration`. `pytest.ini` hard-codes `-m "not integration"`, so a
test marked integration is deselected and never runs; all four tests in
`test_decorator.py` are marked that way and none of them has ever executed in
CI. Do not add the marker here.

Every case below was measured against 1.24.1 before the fix. The pre-fix
behaviour is recorded in each assertion's comment, so the suite is red-proofed by
construction: on 1.24.1 the DENY row alone fails for five of the six entry
points.
"""

import base64
import json

import pytest
import requests
from nacl.encoding import Base64Encoder
from nacl.signing import SigningKey

import aim_sdk.client as client_module
from aim_sdk.client import AIMClient
from aim_sdk.decision import (
    EnforcementMode,
    ModeSource,
    Outcome,
    UnknownSource,
    get_mode_cache,
    parse_enforcement_mode,
)
from aim_sdk.decorators import aim_verify, aim_verify_database
from aim_sdk.exceptions import ActionDeniedError, VerificationUnavailableError

# The fifth decorator. Loaded by PATH, not by package import:
# `aim_sdk/integrations/langchain/__init__.py` imports `langchain_core`, which is
# not a dependency of this SDK and is absent in CI. Importing the package would
# make this entry point skip wherever langchain is not installed -- which is
# everywhere our tests actually run -- and an entry point that skips in CI is an
# entry point nobody is testing. The decorator module itself imports only
# `aim_sdk`, so loading the file directly is both sufficient and honest.
import importlib.util as _ilu
import os as _os

_LC_PATH = _os.path.join(
    _os.path.dirname(_os.path.dirname(_os.path.abspath(__file__))),
    "aim_sdk", "integrations", "langchain", "decorators.py",
)
_lc_spec = _ilu.spec_from_file_location("_aim_langchain_decorators", _LC_PATH)
_lc_module = _ilu.module_from_spec(_lc_spec)
_lc_spec.loader.exec_module(_lc_module)
langchain_aim_verify = _lc_module.aim_verify
from aim_sdk.strict_mode import reset_warning_state

VERIFICATION_ID = "11111111-2222-3333-4444-555555555555"
AGENT_ID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"

# ---------------------------------------------------------------------------
# The six ruled cases, plus the two extra live paths measured alongside them.
# ---------------------------------------------------------------------------
ALLOW = "allow"
DENY = "deny"
HTTP_403 = "http_403"
HTTP_429 = "http_429"
HTTP_500 = "http_500"
NETWORK = "network"
HTTP_404 = "http_404"
TIMEOUT = "timeout"

SERVER_ANSWERED_NO_DECISION = [HTTP_429, HTTP_500, HTTP_404]
TRANSPORT_FAILURES = [NETWORK, TIMEOUT]
ALL_CASES = [ALLOW, DENY, HTTP_403] + SERVER_ANSWERED_NO_DECISION + TRANSPORT_FAILURES

ENTRY_POINTS = [
    "aim_verify",
    "aim_verify_database",
    "perform_action",
    "track_action",
    "require_approval",
    "langchain_aim_verify",
]


class FakeResponse:
    def __init__(self, status_code, payload=None, headers=None):
        self.status_code = status_code
        self._payload = payload
        self.text = json.dumps(payload or {})
        self.headers = headers or {}

    def json(self):
        if self._payload is None:
            raise json.JSONDecodeError("no json", "", 0)
        return self._payload

    def raise_for_status(self):
        if self.status_code >= 400:
            raise requests.exceptions.HTTPError(f"HTTP {self.status_code}")


@pytest.fixture(autouse=True)
def _isolate(monkeypatch):
    """
    The mode cache is process-scoped and keyed by agent id, so without clearing
    it the first ALLOW warms every later case in the same process and the whole
    matrix silently measures one row.

    AIM_STRICT_MODE is cleared for the same reason: a value inherited from the
    developer's shell would ratchet every case to strict and turn the monitoring
    rows green for the wrong reason.
    """
    get_mode_cache().clear()
    reset_warning_state()
    monkeypatch.delenv("AIM_STRICT_MODE", raising=False)
    # The 429 retry is real and sleeps. It has its own test; here it would add
    # seconds per cell and measure nothing extra.
    monkeypatch.setattr(client_module, "RATE_LIMIT_RETRY_ATTEMPTS", 0)
    yield
    get_mode_cache().clear()


def _make_client(case, tally, wire_mode=None):
    sk = SigningKey.generate()
    client = AIMClient(
        agent_id=AGENT_ID,
        public_key=sk.verify_key.encode(encoder=Base64Encoder).decode(),
        private_key=base64.b64encode(bytes(sk)).decode(),
        aim_url="http://aim.invalid",
        sdk_token_id="test-token",
        telemetry={"enabled": False},
    )

    def stub(method, url, **kwargs):
        if url.endswith("/execution-status"):
            tally["exec_reports"] += 1
            # setdefault: several call sites build the tally with only the two
            # counters, and an execution report is worthless as evidence unless
            # the URL it was sent to is inspectable -- defect (B) was a report
            # that returned before it ever built one.
            tally.setdefault("exec_urls", []).append(url)
            tally.setdefault("exec_payloads", []).append(kwargs.get("json"))
            return FakeResponse(200, {"ok": True})
        if url.endswith("/result"):
            tally["result_reports"] += 1
            return FakeResponse(200, {"ok": True})
        if "/verifications" not in url:
            return FakeResponse(200, {"ok": True})

        # A real deployment has ONE enforcement mode, so an answer carries the
        # same mode the cache holds. A cold cache means the answer carries no
        # mode at all -- a pre-2.0.0 backend, or one whose org lookup failed.
        mode_payload = {} if wire_mode is None else {"enforcementMode": wire_mode}
        if case == ALLOW:
            return FakeResponse(
                200,
                {"id": VERIFICATION_ID, "status": "approved",
                 "approvedBy": "system", "expiresAt": "2026-08-14T00:00:00Z",
                 **mode_payload},
            )
        if case == DENY:
            return FakeResponse(
                200,
                {"id": VERIFICATION_ID, "status": "denied",
                 "denialReason": "policy: blocked by test", **mode_payload},
            )
        if case == HTTP_403:
            # THE production denial. `CreateVerification` builds a full
            # VerificationResponse and returns it with status 403 -- id,
            # denialReason and enforcementMode, all camelCase, no `error` key
            # anywhere. This stub used to send `{"error": ...}`, a shape the
            # backend has never emitted, and that single wrong fixture is why 165
            # tests could not see that the 403 path dropped the mode and the id.
            return FakeResponse(
                403,
                {"id": VERIFICATION_ID, "status": "denied",
                 "denialReason": "policy: blocked by test", **mode_payload},
            )
        if case == HTTP_429:
            return FakeResponse(429, {"error": "rate limit exceeded"}, {"Retry-After": "1"})
        if case == HTTP_500:
            return FakeResponse(500, {"error": "internal server error"})
        if case == HTTP_404:
            return FakeResponse(404, {"error": "not found"})
        if case == NETWORK:
            raise requests.exceptions.ConnectionError("connection refused")
        if case == TIMEOUT:
            raise requests.exceptions.Timeout("timed out")
        raise AssertionError(f"unknown case {case}")

    client.session.request = stub
    return client


def _warm_cache(mode):
    """Prime the process cache with a mode AIM actually stated."""
    tally = {"exec_reports": 0, "result_reports": 0}
    client = _make_client(ALLOW, tally, wire_mode=mode)
    client.verify_capability(capability="warm:up", resource="warm")


_UNSET = object()


def run_case(entry_point, case, warm=None, wire_mode=_UNSET, tally=None):
    """
    Drive one entry point through one case.

    Returns (body_ran, raised_type_name).

    `warm` primes the process mode cache AND, by default, is the mode the wire
    answer carries -- one real deployment has one mode. `wire_mode` splits the
    two apart, which is the only way to test that a FRESH mode off a 403 body
    beats a stale cached one: pass warm="strict", wire_mode="monitoring".
    Pass `tally` to inspect what the client sent while the case ran.
    """
    get_mode_cache().clear()
    if warm is not None:
        _warm_cache(warm)

    effective_wire = warm if wire_mode is _UNSET else wire_mode
    tally = {"exec_reports": 0, "result_reports": 0} if tally is None else tally
    client = _make_client(case, tally, wire_mode=effective_wire)
    ran = {"body": False}

    def body():
        ran["body"] = True
        return "BODY-RETURN"

    if entry_point == "aim_verify":
        fn = aim_verify(client, action_type="db:delete")(body)
    elif entry_point == "aim_verify_database":
        fn = aim_verify_database(client)(body)
    elif entry_point == "perform_action":
        fn = client.perform_action(capability="db:delete", auto_register=False)(body)
    elif entry_point == "track_action":
        fn = client.track_action(risk_level="high", action_name="db:delete")(body)
    elif entry_point == "require_approval":
        fn = client.require_approval(risk_level="high", action_name="db:delete")(body)
    elif entry_point == "langchain_aim_verify":
        fn = langchain_aim_verify(agent=client, action_name="db:delete")(body)
    else:
        raise AssertionError(entry_point)

    raised = None
    try:
        fn()
    except BaseException as exc:  # noqa: BLE001 - the type is the assertion
        raised = type(exc).__name__
    return ran["body"], raised


# ---------------------------------------------------------------------------
# The headline defect: an explicit denial must not run the body.
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("entry_point", ENTRY_POINTS)
def test_explicit_denial_blocks_in_strict_mode(entry_point):
    """
    THE defect. On 1.24.1 this FAILS for aim_verify, aim_verify_database,
    track_action and require_approval: a denial raised ActionDeniedError, which
    was not a PermissionError, so it fell past `except PermissionError` into the
    blanket `except Exception` fail-open branch and the body ran. Measured
    pre-fix: aim_verify RAN with no exception at all.
    """
    ran, raised = run_case(entry_point, DENY, warm="strict")
    assert ran is False, f"{entry_point} executed a DENIED action in strict mode"
    assert raised == "ActionDeniedError", f"{entry_point} raised {raised}"


@pytest.mark.parametrize("entry_point", ENTRY_POINTS)
def test_explicit_denial_blocks_in_monitoring_mode(entry_point):
    """
    An explicit denial blocks in EVERY mode, including the schema default.

    This assertion was inverted until 2026-08-13, on the ground that monitoring
    mode's contract is "warning logged, function executes anyway". That ground was
    measured wrong. `VerifyCapability` already applies monitoring mode server-side
    at every policy gate -- under monitoring a policy refusal is converted to an
    approval before it is ever sent -- so a denial that survives to the wire is
    one the server refused to override *while holding the org row and knowing the
    mode*. Honouring monitoring again in the SDK overrode that a second time, and
    what it overrode was, measurably, an administrator pressing Deny in the JIT
    dashboard.

    Monitoring mode governs what happens when AIM cannot give an answer (the
    UNKNOWN rows below, which are still lenient here), not what happens when AIM
    says no.
    """
    ran, raised = run_case(entry_point, DENY, warm="monitoring")
    assert ran is False, f"{entry_point} executed a DENIED action in monitoring mode"
    assert raised == "ActionDeniedError", f"{entry_point} raised {raised}"


@pytest.mark.parametrize("entry_point", ENTRY_POINTS)
def test_denial_under_unresolved_mode_blocks(entry_point):
    """
    A denial with no resolvable enforcement mode fails CLOSED. The mode is
    unknown exactly when the backend's own organization lookup failed, and a
    degraded upstream must not be able to produce the lenient outcome by
    failing. Absence of a customer decision is not consent to leniency.
    """
    ran, raised = run_case(entry_point, DENY, warm=None)
    assert ran is False, f"{entry_point} ran a denial while the mode was unknown"
    assert raised == "ActionDeniedError"


# ---------------------------------------------------------------------------
# HTTP 403: the ONLY wire form a real denial takes.
#
# `CreateVerification` returns status 403 for every denial it emits. Until
# 2026-08-13 this suite had zero direction coverage of it: HTTP_403 appeared in
# three list literals and its only consumer asserted that the entry points AGREED
# with each other, not which way they agreed. Six entry points unanimously
# running a denied action satisfies an agreement test perfectly.
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("entry_point", ENTRY_POINTS)
@pytest.mark.parametrize("warm", [None, "strict", "monitoring"])
def test_http_403_blocks_in_every_mode(entry_point, warm):
    """
    The production denial, across every entry point and every mode, warm cache.

    Pre-fix this ran the body for all six entry points under monitoring: the 403
    branch resolved the mode from a hard-coded `None`, so with a warm monitoring
    cache the decision arrived as DENY+MONITORING and the old DENY row ran it.
    """
    ran, raised = run_case(entry_point, HTTP_403, warm=warm)
    assert ran is False, f"{entry_point} executed a 403-denied action (mode={warm})"
    assert raised == "ActionDeniedError", f"{entry_point} raised {raised}"


@pytest.mark.parametrize("entry_point", ENTRY_POINTS)
@pytest.mark.parametrize("wire_mode", [None, "strict", "monitoring"])
def test_http_403_blocks_on_a_cold_cache(entry_point, wire_mode):
    """
    Same matrix with a COLD cache, so the mode can only come from the 403 body
    itself. This is the half of the cross-product that defect (A) made
    unreachable -- with the mode hard-coded to None the body was never read, so
    every cold 403 resolved to UNKNOWN no matter what the server said.
    """
    ran, raised = run_case(entry_point, HTTP_403, warm=None, wire_mode=wire_mode)
    assert ran is False, f"{entry_point} executed a cold 403 (wire mode={wire_mode})"
    assert raised == "ActionDeniedError"


def test_403_reads_the_mode_off_the_response_not_the_stale_cache():
    """
    Defect (A), pinned directly.

    A 403 body states the organization's mode. Resolution is ordered -- a fresh
    answer beats a cached one -- so a process holding a stale `strict` must adopt
    the `monitoring` the server just stated. Pre-fix `mode_source` was CACHE here
    and `mode` was STRICT, because `parse_enforcement_mode(None)` was hard-coded
    and the body was never consulted.

    This is also the ordering constraint made executable: a tree with (A) fixed
    but the DENY row still consulting the mode reaches this line with
    MONITORING/RESPONSE and then RUNS the action -- strictly worse than the
    branch it replaced, which at least blocked warm-strict processes.
    """
    _warm_cache("strict")
    client = _make_client(
        HTTP_403, {"exec_reports": 0, "result_reports": 0}, wire_mode="monitoring"
    )
    with pytest.raises(ActionDeniedError) as excinfo:
        client.verify_capability(capability="db:delete")

    assert excinfo.value.mode is EnforcementMode.MONITORING
    assert excinfo.value.mode_source is ModeSource.RESPONSE
    # And the denial still blocks, which is the entire point of reading it.
    assert isinstance(excinfo.value, ActionDeniedError)


def test_403_carries_the_servers_stated_denial_reason():
    """
    The 403 body's reason key is `denialReason`. The SDK read `error`, which the
    backend has never sent on this route, so every production denial reached the
    developer as the literal fallback "insufficient permissions" -- a message
    that names no policy, no capability and no administrator.
    """
    client = _make_client(
        HTTP_403, {"exec_reports": 0, "result_reports": 0}, wire_mode="strict"
    )
    with pytest.raises(ActionDeniedError) as excinfo:
        client.verify_capability(capability="db:delete")
    assert "policy: blocked by test" in str(excinfo.value)
    assert "insufficient permissions" not in str(excinfo.value)


# Measured 2026-08-13: only these two entry points report an execution at all on
# the blocked path. `perform_action`, `track_action`, `require_approval` and the
# LangChain wrapper each `raise verdict.error` without reporting -- see
# `client.py` (three sites) -- so threading the id through the 403 fixes the
# report for two of six. Widening that is a backend-contract change with its own
# cost (four new POSTs on a blocked path, and a dashboard whose execution record
# changes meaning), so it is NOT smuggled into this commit. Tracked in #382,
# which carries the measurement and the acceptance criteria.
REPORTING_ENTRY_POINTS = ["aim_verify", "aim_verify_database"]
NON_REPORTING_ENTRY_POINTS = [
    ep for ep in ENTRY_POINTS if ep not in REPORTING_ENTRY_POINTS
]


@pytest.mark.parametrize("entry_point", REPORTING_ENTRY_POINTS)
def test_a_blocked_403_reports_the_execution_against_the_real_id(entry_point):
    """
    Defect (B), pinned.

    `report_execution_status` returns at its first line on a falsy id. The 403
    branch never read `id` off the body, so a blocked action produced no
    execution report at all: AIM recorded the denial and nothing else, so the
    one event that proves the control fired was the one never sent.

    Not claimed: that a dashboard currently shows the difference. Migration
    053's execution columns are write-only -- no production SELECT names them.
    The defect is a hole in the evidence, and it matters most at the moment the
    execution-outcome channel starts being read, because a gap in the data there
    reads as "the SDK ran denied actions".

    Asserts the id is IN THE URL, not merely that a POST happened -- a report
    sent to `.../None/execution-status` would satisfy a call-count assertion,
    and that is exactly the shape the pre-fix code produced.
    """
    tally = {"exec_reports": 0, "result_reports": 0}
    ran, raised = run_case(entry_point, HTTP_403, warm="strict", tally=tally)
    assert ran is False and raised == "ActionDeniedError"

    urls = tally.get("exec_urls", [])
    assert urls, f"{entry_point}: a blocked 403 sent no execution report"
    assert all(VERIFICATION_ID in u for u in urls), (
        f"{entry_point}: execution report was not attached to the verification "
        f"the server denied. URLs: {urls}"
    )
    assert all(p.get("executed") is False for p in tally["exec_payloads"]), (
        f"{entry_point}: reported executed=True for an action that was blocked"
    )


@pytest.mark.parametrize("entry_point", NON_REPORTING_ENTRY_POINTS)
def test_blocked_execution_reporting_gap_is_measured_not_assumed(entry_point):
    """
    CHARACTERIZATION, not an endorsement. These four entry points block correctly
    -- that is asserted above and is the release gate -- but send AIM no
    execution report when they do, so the dashboard shows their denials with no
    proof the action stopped. `perform_action` is the decorator the README Quick
    start teaches, so this is the common case, not an edge.

    This test exists so the gap is a measured fact in the suite rather than
    something rediscovered by the next person reading defect (B) and assuming it
    was fixed everywhere. When the gap is closed, move the entry point into
    REPORTING_ENTRY_POINTS -- this test going red IS the intended signal.
    """
    tally = {"exec_reports": 0, "result_reports": 0}
    ran, raised = run_case(entry_point, HTTP_403, warm="strict", tally=tally)
    assert ran is False and raised == "ActionDeniedError", (
        f"{entry_point} must still BLOCK -- only the reporting is missing"
    )
    assert tally.get("exec_urls", []) == [], (
        f"{entry_point} now reports blocked executions. Good -- move it into "
        f"REPORTING_ENTRY_POINTS and delete it from this test."
    )


# ---------------------------------------------------------------------------
# A server that answered is never permissive.
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("entry_point", ENTRY_POINTS)
@pytest.mark.parametrize("case", SERVER_ANSWERED_NO_DECISION)
def test_server_answer_without_decision_blocks_when_mode_unresolved(entry_point, case):
    """
    429, 500 and 404 are ANSWERS that carried no decision. On 1.24.1 every one of
    them returned `verified: False` and the decorators read that as "not
    permitted, but carry on" -- which is how 101 requests in a minute from the
    agent's own IP turned verification off for that IP, with no credentials and
    no exploit.

    UNKNOWN is an allowlist of transport failures, never a denylist of statuses,
    so none of these is eligible for the permissive path.
    """
    ran, raised = run_case(entry_point, case, warm=None)
    assert ran is False, f"{entry_point} ran the body on {case} with mode unresolved"
    assert raised == "VerificationUnavailableError"


@pytest.mark.parametrize("entry_point", ENTRY_POINTS)
@pytest.mark.parametrize("case", SERVER_ANSWERED_NO_DECISION)
def test_server_answer_without_decision_blocks_in_strict_mode(entry_point, case):
    ran, raised = run_case(entry_point, case, warm="strict")
    assert ran is False, f"{entry_point} ran the body on {case} in strict mode"
    assert raised == "VerificationUnavailableError"


@pytest.mark.parametrize("case", SERVER_ANSWERED_NO_DECISION)
def test_server_answer_runs_in_monitoring_mode(case):
    """
    A monitoring organization has no enforcement to bypass -- an explicit denial
    already executes there by design -- so leniency costs nothing, while blocking
    would break the one guarantee monitoring mode makes and would hand an
    unauthenticated party a stop button (a 429 is reachable by making the agent's
    IP busy, and fires by accident behind a NAT).
    """
    ran, raised = run_case("aim_verify", case, warm="monitoring")
    assert ran is True
    assert raised is None


def test_a_5xx_is_not_reported_as_a_denial():
    """
    On 1.24.1 a 500 surfaced as `PermissionError: AIM verification failed ...
    HTTP 500`, telling a developer they had been denied when AIM was merely down.
    VerificationUnavailableError must not be catchable as either a denial or a
    PermissionError.
    """
    client = _make_client(HTTP_500, {"exec_reports": 0, "result_reports": 0})
    with pytest.raises(VerificationUnavailableError) as excinfo:
        client.verify_capability(capability="db:delete")
    assert not isinstance(excinfo.value, ActionDeniedError)
    assert not isinstance(excinfo.value, PermissionError)


# ---------------------------------------------------------------------------
# The transport allowlist, and the 2.0.0 warning window.
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("entry_point", ENTRY_POINTS)
@pytest.mark.parametrize("case", TRANSPORT_FAILURES)
def test_transport_failure_blocks_in_strict_mode(entry_point, case):
    ran, raised = run_case(entry_point, case, warm="strict")
    assert ran is False, f"{entry_point} ran the body on {case} in strict mode"
    assert raised == "VerificationUnavailableError"


@pytest.mark.parametrize("entry_point", ENTRY_POINTS)
@pytest.mark.parametrize("case", TRANSPORT_FAILURES)
def test_transport_failure_with_unresolved_mode_runs_and_warns(entry_point, case):
    """
    The ONLY permissive cell, and it is the 2.0.0 warning window.

    The SDK can tell neither whether AIM exists nor what the organization wants.
    Blocking it is ruled, and it is a behaviour flip that takes down
    cold-starting agents during an AIM outage -- including monitoring-mode
    organizations who chose leniency and cannot tell us so. It ships as a major
    after a warning window; 2.0.0 IS that window.
    """
    from aim_sdk.enforcement import PendingEnforcementChange

    with pytest.warns(PendingEnforcementChange):
        ran, raised = run_case(entry_point, case, warm=None)
    assert ran is True, f"{entry_point} blocked {case} during the 2.0.0 warning window"
    assert raised is None


def test_the_permissive_class_is_reachable_only_from_a_transport_exception():
    """
    Guards the allowlist itself. `permissive_eligible` must never be reachable
    from a status code -- that is how an allowlist quietly becomes a denylist.
    """
    from aim_sdk.decision import VerificationDecision

    for source in (UnknownSource.SERVER_ANSWER, UnknownSource.NONE):
        d = VerificationDecision(
            outcome=Outcome.UNKNOWN,
            mode=EnforcementMode.UNKNOWN,
            mode_source=ModeSource.UNAVAILABLE,
            unknown_source=source,
        )
        assert d.permissive_eligible is False

    d = VerificationDecision(
        outcome=Outcome.UNKNOWN,
        mode=EnforcementMode.UNKNOWN,
        mode_source=ModeSource.UNAVAILABLE,
        unknown_source=UnknownSource.TRANSPORT,
    )
    assert d.permissive_eligible is True

    # An ALLOW or DENY is never "permissive-eligible" either -- that property is
    # about undetermined outcomes only.
    for outcome in (Outcome.ALLOW, Outcome.DENY):
        d = VerificationDecision(
            outcome=outcome,
            mode=EnforcementMode.UNKNOWN,
            mode_source=ModeSource.UNAVAILABLE,
            unknown_source=UnknownSource.TRANSPORT,
        )
        assert d.permissive_eligible is False


# ---------------------------------------------------------------------------
# Direction agreement. Disagreement is a release blocker.
# ---------------------------------------------------------------------------

@pytest.mark.parametrize("case", ALL_CASES)
@pytest.mark.parametrize("warm", [None, "strict", "monitoring"])
def test_all_entry_points_agree_on_direction(case, warm):
    """
    Every entry point must reach the same run/block answer for the same case and
    mode. On 1.24.1 they disagreed on nearly every row: on an explicit denial
    aim_verify ran the body, perform_action raised, track_action returned a
    sentinel dict, and require_approval crashed with AttributeError before it
    ever asked.
    """
    results = {ep: run_case(ep, case, warm)[0] for ep in ENTRY_POINTS}
    directions = set(results.values())
    assert len(directions) == 1, (
        f"direction disagreement on case={case} mode={warm}: {results}"
    )


# ---------------------------------------------------------------------------
# The ratchet.
# ---------------------------------------------------------------------------

# Every ratchet test below runs on an UNKNOWN cell (a 500), NOT on a denial.
#
# A denial now blocks under monitoring on its own, so `AIM_STRICT_MODE=true` +
# DENY + monitoring asserts an outcome the ratchet did not cause: disabling
# `strict_mode_override()` entirely leaves all 36 of those cases green. The
# ratchet's whole job is to raise enforcement where the org's own mode would have
# been lenient, and after 2026-08-13 the only rows where that is still true are
# the UNKNOWN ones. Measured: on the DENY cell the control mutation is 41 passed
# (fully vacuous); on the cells below it is 36 failed, 5 passed.

@pytest.mark.parametrize("value", ["true", "TRUE", "1", "yes", "on", " on "])
@pytest.mark.parametrize("entry_point", ENTRY_POINTS)
def test_strict_mode_ratchet_forces_blocking(entry_point, value, monkeypatch):
    """
    On 1.24.1 `decorators.py` accepted only the literal "true" while
    `demo_agent.py` accepted ("true", "1", "yes") and printed
    "Strict Mode: ENABLED - Unauthorized actions WILL BE BLOCKED".
    So AIM_STRICT_MODE=1 executed a denied action while our own demo told the
    user the control was on. One parser now, and it is the enforcing one.

    Cell: a 500 under a warm MONITORING org, which runs the body without the
    ratchet and must block with it.
    """
    monkeypatch.setenv("AIM_STRICT_MODE", value)
    ran, raised = run_case(entry_point, HTTP_500, warm="monitoring")
    assert ran is False, f"AIM_STRICT_MODE={value!r} did not force blocking"
    assert raised == "VerificationUnavailableError"


@pytest.mark.parametrize("value", ["false", "0", "no", "off", "nonsense"])
def test_strict_mode_ratchet_never_lowers_enforcement(value, monkeypatch):
    """
    The ratchet may only raise. A false value cannot put a strict-mode
    organization into monitoring mode.

    Cell: a 500 under a warm STRICT org. Monitoring would run this cell, so if a
    false value were read as "set monitoring" the body would execute. On the DENY
    cell this test was vacuous even before 2026-08-13 -- strict and monitoring
    both blocked a denial, so nothing it asserted could distinguish them.
    """
    monkeypatch.setenv("AIM_STRICT_MODE", value)
    ran, raised = run_case("aim_verify", HTTP_500, warm="strict")
    assert ran is False, f"AIM_STRICT_MODE={value!r} relaxed a strict organization"
    assert raised == "VerificationUnavailableError"


def test_strict_mode_false_is_ignored_not_fatal(monkeypatch):
    """
    `=false` is ignored with a warning rather than being a hard error: it is
    measurably a no-op today (the mode never reached the decorator at all), so
    refusing to start would take agents down over a stale `.env` while removing
    nothing anyone depended on.

    Cell: a 500 under a warm MONITORING org -- the org's own leniency must
    survive `=false`. On the DENY cell this asserted `ran is True`, which after
    2026-08-13 is not "the ratchet was ignored" but "the denial did not block",
    so it both flipped and stopped measuring the ratchet.
    """
    monkeypatch.setenv("AIM_STRICT_MODE", "false")
    ran, raised = run_case("aim_verify", HTTP_500, warm="monitoring")
    assert ran is True
    assert raised is None


# ---------------------------------------------------------------------------
# The mode channel itself.
# ---------------------------------------------------------------------------

def test_enforcement_mode_reaches_the_caller():
    """
    Defect (a). `decorators.py` read `verification["enforcementMode"]`, a key no
    return site in client.py ever emitted, so it fell back to "monitoring" on
    every call and the dashboard setting had no effect on the Python SDK at all.
    """
    client = _make_client(ALLOW, {"exec_reports": 0, "result_reports": 0}, wire_mode="strict")
    result = client.verify_capability(capability="db:read")
    assert result["mode"] == "strict"
    assert result["mode_source"] == "response"
    # The legacy keys are preserved exactly.
    assert result["verified"] is True
    assert result["verification_id"] == VERIFICATION_ID


def test_absent_enforcement_mode_is_unknown_not_monitoring():
    """
    A degraded upstream must not be able to emit the healthy, lenient value by
    saying nothing.
    """
    assert parse_enforcement_mode(None) is EnforcementMode.UNKNOWN
    assert parse_enforcement_mode("") is EnforcementMode.UNKNOWN
    assert parse_enforcement_mode("unknown") is EnforcementMode.UNKNOWN
    assert parse_enforcement_mode("garbage") is EnforcementMode.UNKNOWN
    assert parse_enforcement_mode("strict") is EnforcementMode.STRICT
    assert parse_enforcement_mode("monitoring") is EnforcementMode.MONITORING


def test_mode_cache_serves_a_later_call_that_gets_no_mode():
    """
    Mode resolution is ordered: fresh answer, then the process-scoped in-memory
    last-known mode within TTL, then UNKNOWN.
    """
    _warm_cache("strict")
    client = _make_client(HTTP_500, {"exec_reports": 0, "result_reports": 0})
    with pytest.raises(VerificationUnavailableError) as excinfo:
        client.verify_capability(capability="db:delete")
    assert excinfo.value.mode is EnforcementMode.STRICT
    assert excinfo.value.mode_source is ModeSource.CACHE


def test_mode_cache_never_stores_unknown():
    """
    Caching UNKNOWN would let one failed organization lookup pin an agent to the
    unresolved state for the whole TTL.
    """
    cache = get_mode_cache()
    cache.clear()
    cache.put(AGENT_ID, EnforcementMode.UNKNOWN)
    assert cache.get(AGENT_ID) is None
    cache.put(AGENT_ID, EnforcementMode.MONITORING)
    assert cache.get(AGENT_ID) is EnforcementMode.MONITORING


def test_mode_cache_expires_on_a_monotonic_clock(monkeypatch):
    """
    A system clock change must not extend or expire an entry.
    """
    from aim_sdk import decision as decision_module

    fake_now = {"t": 1000.0}
    monkeypatch.setattr(decision_module.time, "monotonic", lambda: fake_now["t"])
    cache = decision_module.ModeCache(ttl_seconds=300)
    cache.put(AGENT_ID, EnforcementMode.STRICT)
    fake_now["t"] = 1000.0 + 299
    assert cache.get(AGENT_ID) is EnforcementMode.STRICT
    fake_now["t"] = 1000.0 + 301
    assert cache.get(AGENT_ID) is None


def test_mode_cache_is_not_persisted_anywhere(tmp_path, monkeypatch):
    """
    The cache must stay in memory. A disk cache is forgeable in the lenient
    direction: the adversary does not suppress the cached mode, they write
    `monitoring` into it.
    """
    import ast

    import aim_sdk.decision as decision_source

    # Parsed, not grepped. A substring guard would fire on the module docstring,
    # which names `keyring` precisely to explain why the cache must never use
    # one -- and a guard that a comment can trip is a guard that gets deleted.
    tree = ast.parse(open(decision_source.__file__, encoding="utf-8").read())

    forbidden_modules = {"json", "pickle", "shelve", "keyring", "pathlib", "os", "sqlite3", "tempfile"}
    forbidden_calls = {"open"}

    imported = set()
    called = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            imported.update(alias.name.split(".")[0] for alias in node.names)
        elif isinstance(node, ast.ImportFrom) and node.module:
            imported.add(node.module.split(".")[0])
        elif isinstance(node, ast.Call) and isinstance(node.func, ast.Name):
            called.add(node.func.id)

    leaked = imported & forbidden_modules
    assert not leaked, (
        f"aim_sdk/decision.py imports {sorted(leaked)} -- the mode cache must never "
        f"be persisted. A disk cache is forgeable in the lenient direction: the "
        f"adversary does not suppress the cached mode, they write 'monitoring' into "
        f"it. Persistence voids the refusal to sign the mode assertion and makes a "
        f"signed, expiring assertion mandatory."
    )
    assert not (called & forbidden_calls), (
        f"aim_sdk/decision.py calls {sorted(called & forbidden_calls)} -- the mode "
        f"cache must never touch the filesystem."
    )


# ---------------------------------------------------------------------------
# The execution-report id.
# ---------------------------------------------------------------------------

def test_execution_report_is_actually_sent():
    """
    Defect (c). `decorators.py` read `verificationId`/`id` while the client emits
    `verification_id`, so `report_execution_status` returned early on a falsy id
    before it ever built a URL. AIM has never received an execution report from
    the Python SDK. All eight call sites were fed by that one value.
    """
    tally = {"exec_reports": 0, "result_reports": 0}
    client = _make_client(ALLOW, tally, wire_mode="strict")

    @aim_verify(client, action_type="db:read")
    def work():
        return "ok"

    assert work() == "ok"
    assert tally["exec_reports"] == 1, "no execution report reached the backend"


def test_blocked_action_reports_that_it_did_not_execute():
    tally = {"exec_reports": 0, "result_reports": 0}
    get_mode_cache().clear()
    _warm_cache("strict")
    client = _make_client(DENY, tally, wire_mode="strict")

    @aim_verify(client, action_type="db:delete")
    def work():  # pragma: no cover - must not run
        raise AssertionError("body ran on a denied action")

    with pytest.raises(ActionDeniedError):
        work()
    assert tally["exec_reports"] == 1


def test_execution_report_timeout_is_bounded():
    """
    Fixing the id turns eight no-op calls into eight real POSTs. The report is
    fire-and-forget, so it can never raise -- an unbounded wait would not fail
    loudly, it would just make every wrapped invocation slower with no signal,
    on the exact path that is already degraded.
    """
    seen = {}
    client = _make_client(ALLOW, {"exec_reports": 0, "result_reports": 0}, wire_mode="strict")
    client.timeout = 30

    original = client.session.request

    def capture(method, url, **kwargs):
        if url.endswith("/execution-status"):
            seen["timeout"] = kwargs.get("timeout")
        return original(method, url, **kwargs)

    client.session.request = capture

    @aim_verify(client, action_type="db:read")
    def work():
        return "ok"

    work()
    assert seen["timeout"] == client_module.EXECUTION_REPORT_TIMEOUT_SECONDS
    assert seen["timeout"] < 30


# ---------------------------------------------------------------------------
# The 429 retry.
# ---------------------------------------------------------------------------

def test_429_is_retried_before_being_classified(monkeypatch):
    """
    A 429 is our own rate limiter declining to answer. The limiter returns
    without invoking the handler, so no verification event exists and a retry
    cannot duplicate one. Obtaining the decision beats enforcing on its absence.
    """
    monkeypatch.setattr(client_module, "RATE_LIMIT_RETRY_ATTEMPTS", 2)
    monkeypatch.setattr(client_module.time, "sleep", lambda _s: None)

    calls = {"n": 0}
    sk = SigningKey.generate()
    client = AIMClient(
        agent_id=AGENT_ID,
        public_key=sk.verify_key.encode(encoder=Base64Encoder).decode(),
        private_key=base64.b64encode(bytes(sk)).decode(),
        aim_url="http://aim.invalid",
        sdk_token_id="test-token",
        telemetry={"enabled": False},
    )

    def stub(method, url, **kwargs):
        if "/verifications" not in url or url.endswith(("/execution-status", "/result")):
            return FakeResponse(200, {"ok": True})
        calls["n"] += 1
        if calls["n"] == 1:
            return FakeResponse(429, {"error": "rate limited"}, {"Retry-After": "1"})
        return FakeResponse(200, {"id": VERIFICATION_ID, "status": "approved",
                                  "enforcementMode": "strict"})

    client.session.request = stub
    result = client.verify_capability(capability="db:read")
    assert result["verified"] is True
    assert calls["n"] == 2, "the 429 was not retried"


def test_5xx_is_never_retried(monkeypatch):
    """
    A 5xx may have partly executed the handler, so retrying it could duplicate a
    verification event on an authorization path. 429 only.
    """
    monkeypatch.setattr(client_module, "RATE_LIMIT_RETRY_ATTEMPTS", 2)
    monkeypatch.setattr(client_module.time, "sleep", lambda _s: None)

    calls = {"n": 0}
    tally = {"exec_reports": 0, "result_reports": 0}
    client = _make_client(HTTP_500, tally)
    original = client.session.request

    def counting(method, url, **kwargs):
        if "/verifications" in url and not url.endswith(("/execution-status", "/result")):
            calls["n"] += 1
        return original(method, url, **kwargs)

    client.session.request = counting
    with pytest.raises(VerificationUnavailableError):
        client.verify_capability(capability="db:read")
    assert calls["n"] == 1, "a 5xx was retried"


def test_retry_after_is_bounded(monkeypatch):
    """
    A server asking us to wait longer than the cap does not get to hold the
    agent. The retry degrades latency; it must not become the stall.
    """
    monkeypatch.setattr(client_module, "RATE_LIMIT_RETRY_ATTEMPTS", 2)
    slept = []
    monkeypatch.setattr(client_module.time, "sleep", lambda s: slept.append(s))

    calls = {"n": 0}
    tally = {"exec_reports": 0, "result_reports": 0}
    client = _make_client(ALLOW, tally, wire_mode="strict")

    def stub(method, url, **kwargs):
        if "/verifications" not in url or url.endswith(("/execution-status", "/result")):
            return FakeResponse(200, {"ok": True})
        calls["n"] += 1
        return FakeResponse(429, {"error": "rate limited"}, {"Retry-After": "3600"})

    client.session.request = stub
    with pytest.raises(VerificationUnavailableError):
        client.verify_capability(capability="db:read")
    assert slept == [], "an out-of-range Retry-After was honoured"
    assert calls["n"] == 1


# ---------------------------------------------------------------------------
# The exception contract.
# ---------------------------------------------------------------------------

def test_action_denied_is_catchable_as_permission_error():
    """
    README documents that strict mode raises PermissionError, and the decorators
    have always had an `except PermissionError` handler -- but the class did not
    inherit from it, which is why a denial fell through to the fail-open branch.
    The base is compatibility; enforcement is decided by the typed outcome.
    """
    assert issubclass(ActionDeniedError, PermissionError)
    from aim_sdk.exceptions import AIMError

    assert issubclass(ActionDeniedError, AIMError)
    try:
        raise ActionDeniedError("denied: policy")
    except PermissionError as exc:
        assert type(exc) is ActionDeniedError
    else:  # pragma: no cover
        raise AssertionError("ActionDeniedError was not caught as PermissionError")


def test_unavailable_does_not_subclass_verification_error():
    """
    A handler written for "AIM said no" must not silently absorb "AIM was never
    asked".
    """
    from aim_sdk.exceptions import AIMError, VerificationError

    assert issubclass(VerificationUnavailableError, AIMError)
    assert not issubclass(VerificationUnavailableError, VerificationError)
    assert not issubclass(VerificationUnavailableError, PermissionError)
    assert not issubclass(VerificationUnavailableError, ActionDeniedError)


def test_both_exceptions_carry_the_decision():
    """
    The mode channel must exist on the paths where enforcement is actually
    decided, not only on the one path that does not need it.
    """
    client = _make_client(DENY, {"exec_reports": 0, "result_reports": 0}, wire_mode="strict")
    with pytest.raises(ActionDeniedError) as excinfo:
        client.verify_capability(capability="db:delete")
    assert excinfo.value.decision is not None
    assert excinfo.value.decision.outcome is Outcome.DENY
    assert excinfo.value.mode is EnforcementMode.STRICT


def test_no_verified_false_remains_in_the_sdk():
    """
    The conflation is removed by construction, not by discipline: there is no
    longer any return path that can carry `verified: False`, so nothing is left
    to confuse "denied" with "could not determine".
    """
    import aim_sdk.client as source_module

    source = open(source_module.__file__, encoding="utf-8").read()
    assert '"verified": False' not in source
    assert "'verified': False" not in source
