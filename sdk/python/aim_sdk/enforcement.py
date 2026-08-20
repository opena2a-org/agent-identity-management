"""
AIM SDK - the one enforcement rule.

Every entry point that verifies before running user code routes through
:func:`evaluate`. There are seven of them -- ``aim_verify`` and its four
convenience wrappers, ``perform_action``, ``track_action``, ``require_approval``,
and the separate ``aim_verify`` shipped for LangChain -- and before 2.0.0 they
each carried their own ``if`` ladder. They disagreed with each other: on an
explicit denial one raised, one returned a sentinel dict, one ran the function
anyway, and one crashed with ``AttributeError`` before it ever asked.

Direction agreement across entry points is a release gate. Keeping the rule in one
pure function is what makes agreement structural rather than a coincidence that
survives until the next edit.

The rule
--------

Given a :class:`~aim_sdk.decision.VerificationDecision` and the state of the
``AIM_STRICT_MODE`` ratchet:

======================  ==========  ============  =============
outcome                 strict      monitoring    mode unknown
======================  ==========  ============  =============
ALLOW                   run         run           run
DENY                    BLOCK       BLOCK         BLOCK
UNKNOWN, server answer  BLOCK       run + warn    BLOCK
UNKNOWN, transport      BLOCK       run + warn    run + notice
======================  ==========  ============  =============

What monitoring mode governs
----------------------------

Monitoring mode governs what happens when AIM cannot give an answer, not what
happens when AIM says no. A verification that could not be completed is logged
and the action proceeds; an explicit denial blocks in every mode, because it is a
decision AIM already made with your organization's enforcement mode in hand.

That last clause is a backend fact, not an SDK preference. ``VerifyCapability``
already applies monitoring mode server-side at every policy gate: under monitoring
a policy refusal is converted to an approval before it is ever sent. What still
arrives as a denial is the short list the server declines to override while
holding the org row and knowing the mode -- agent not found, agent marked
compromised (not mode-gated at all), capability lookup failed. The SDK used to
override that a second time, on a copy of the mode it read off the same
response. Measured consequence: an administrator pressing Deny in the JIT
dashboard did not stop the action.

Two cells still carry reasoning that is not obvious:

**DENY under an unknown mode blocks** for the same reason it blocks under
monitoring, plus one more: the mode is unknown exactly when AIM's organization
lookup failed, and a degraded upstream must not be able to produce the lenient
outcome by failing.

**A server that answered is not treated like a transport failure.** A 429 or a
5xx means AIM is reachable and the control is live; we simply did not get a
decision. Treating that as permissive is what let 101 requests in a minute from
the agent's own IP switch verification off for that IP, with no credentials and
no exploit. It stays lenient under monitoring only because that is the one thing
monitoring mode does mean -- an unanswered verification is logged and proceeds --
and because blocking it unconditionally would hand an unauthenticated party a
stop button, a 429 being reachable by making the agent's IP busy and firing by
accident behind a NAT.

**The cost of blocking DENY under monitoring, stated plainly.** An attacker who
can induce a 429 can now suppress a denial for the default population: before
this table a monitoring org had nothing to suppress. The bounded 429 retry in
``client.py`` narrows the window; it does not close it. The fix is that the rate
limiter must not sit in front of an authorization decision, and this table
promotes that from hygiene to live exposure.

**A TRANSPORT failure under an unknown mode runs in 2.0.0, and blocks in
3.0.0.** Read the qualifier carefully: this warning window applies ONLY to the
transport row. A server answer under an unknown mode blocks TODAY, in 2.0.0 --
see the truth table, where those are two different rows. Conflating them would
ship the 429 bypass under the banner of having fixed it.

The transport-plus-unknown-mode cell is the only one where the SDK can tell
neither whether AIM exists nor what the organization wants. Blocking it is ruled,
and it is a behaviour flip that takes down cold-starting agents during an AIM
outage -- including monitoring-mode organizations who chose leniency and have no
way to tell us so. It ships as a major after a warning window. 2.0.0 is that
window: the cell runs and says so.

One caveat about how small that population is. It is tempting to say a process
that ever received one good answer has a cached mode and never lands here. That
is only true within the cache TTL (300 s by default, on a monotonic clock). An
agent that verifies less often than the TTL is cold on EVERY call and lands in
this cell every time. The TTL is not removed to shrink the population: a cache
that never expires means an organization that flips monitoring to strict never
takes effect in a long-lived process, which fails in the worse direction.
"""

from dataclasses import dataclass
from typing import Optional

from aim_sdk.decision import (
    EnforcementMode,
    ModeSource,
    Outcome,
    UnknownSource,
    VerificationDecision,
)

# Named so a caller can suppress exactly this one without suppressing the rest.
class PendingEnforcementChange(FutureWarning):
    """The unknown/unknown cell runs today and will block in 3.0.0."""


@dataclass(frozen=True)
class Verdict:
    """What the caller must do. Exactly one of run / block."""

    run: bool
    effective_mode: EnforcementMode
    mode_source: ModeSource
    # Set when run is False. The caller raises it.
    error: Optional[BaseException] = None
    # Set when the caller should warn on the console before running.
    warning: Optional[str] = None
    # Set when the caller should emit PendingEnforcementChange before running.
    pending_change: Optional[str] = None
    # Whether the caller should report executed=True to AIM after running. False
    # only when nothing was verified, so there is no event to attach a report to.
    reportable: bool = True

    @property
    def blocked(self) -> bool:
        return not self.run


def _denial_message(
    what: str, decision: VerificationDecision, mode: EnforcementMode
) -> str:
    """
    The message a blocked developer reads.

    It must not offer switching the organization to monitoring mode. That was the
    remediation before this release and it no longer works -- an explicit denial
    blocks in every mode -- so printing it would send a blocked developer to
    change a setting, watch nothing happen, and have no next step. It also must
    not assert the mode is strict, which was false for the default population the
    moment denials started blocking under monitoring.
    """
    reason = decision.reason or "no reason given"
    return (
        f"AIM denied {what}: {reason}. "
        f"The action was blocked and not executed. AIM denies an action after "
        f"applying your organization's enforcement mode (currently "
        f"{mode.value}), so a denial blocks in every mode -- monitoring mode "
        f"governs verifications AIM could not answer, not ones it refused. "
        f"To permit this capability, grant it to this agent in the AIM dashboard "
        f"under Agents. If the agent should not be denied at all, check its status "
        f"there: an agent marked compromised, suspended or unverified is refused "
        f"regardless of its capabilities."
    )


def _unavailable_message(
    what: str, decision: VerificationDecision, mode: EnforcementMode
) -> str:
    """
    The message read when AIM never returned a decision and the action blocked.

    ``mode`` is threaded through because this message is produced on two
    different rows -- strict, and mode-unknown-with-a-server-answer -- and the
    single sentence it used to print ("the enforcement mode is strict") was false
    on the second one.
    """
    detail = decision.reason or "no detail available"
    if decision.unknown_source is UnknownSource.SERVER_ANSWER:
        cause = (
            f"AIM answered but returned no decision ({detail}). The service is "
            f"reachable, so this is not a network problem."
        )
    else:
        cause = f"AIM could not be reached ({detail})."
    if mode is EnforcementMode.STRICT:
        why = (
            "The organization's enforcement mode is strict, so the action was "
            "blocked and not executed."
        )
    else:
        why = (
            "This process could not resolve the organization's enforcement mode, "
            "so the action was blocked and not executed rather than assuming the "
            "lenient setting."
        )
    return (
        f"AIM could not verify {what}. {cause} {why} "
        f"This is NOT a denial -- AIM never returned a decision. "
        f"Catch VerificationUnavailableError to handle it separately from "
        f"ActionDeniedError."
    )


def evaluate(
    decision: VerificationDecision,
    strict_override: bool = False,
    what: str = "this action",
) -> Verdict:
    """
    Apply the enforcement rule. Pure: no I/O, no environment access, no logging.

    Args:
        decision: the typed three-state decision.
        strict_override: whether the ``AIM_STRICT_MODE`` ratchet is engaged.
            Resolve it with :func:`aim_sdk.strict_mode.strict_mode_override`; this
            function does not read the environment so it stays testable.
        what: a short human label for the thing being verified, used in messages.

    Returns:
        A :class:`Verdict`. The caller runs the wrapped body iff ``verdict.run``,
        and raises ``verdict.error`` otherwise.
    """
    # Lazy import: aim_sdk.exceptions is cheap, but keeping the import here makes
    # this module importable from exceptions-adjacent code without a cycle.
    from aim_sdk.exceptions import ActionDeniedError, VerificationUnavailableError

    if strict_override:
        mode = EnforcementMode.STRICT
        mode_source = ModeSource.ENV_OVERRIDE
    else:
        mode = decision.mode
        mode_source = decision.mode_source

    if decision.outcome is Outcome.ALLOW:
        return Verdict(run=True, effective_mode=mode, mode_source=mode_source)

    monitoring = mode is EnforcementMode.MONITORING

    # An explicit denial blocks in EVERY mode. Monitoring is not consulted here:
    # the server already applied it before deciding, so honouring it a second time
    # is the SDK overriding an answer AIM gave while holding the org row.
    if decision.outcome is Outcome.DENY:
        return Verdict(
            run=False,
            effective_mode=mode,
            mode_source=mode_source,
            error=ActionDeniedError(_denial_message(what, decision, mode)),
        )

    # Outcome.UNKNOWN from here down.
    if monitoring:
        return Verdict(
            run=True,
            effective_mode=mode,
            mode_source=mode_source,
            warning=(
                f"AIM could not verify {what}: "
                f"{decision.reason or 'no detail available'}. The organization's "
                f"enforcement mode is monitoring, so the action was executed anyway."
            ),
            # Nothing was verified, so there is no verification event to report
            # an execution against.
            reportable=decision.verification_id is not None,
        )

    if mode is EnforcementMode.STRICT:
        return Verdict(
            run=False,
            effective_mode=mode,
            mode_source=mode_source,
            error=VerificationUnavailableError(_unavailable_message(what, decision, mode)),
        )

    # Mode is UNKNOWN. A server that answered is still an answer and is never
    # permissive; only a transport failure reaches the 2.0.0 warning window.
    if not decision.permissive_eligible:
        return Verdict(
            run=False,
            effective_mode=mode,
            mode_source=mode_source,
            error=VerificationUnavailableError(_unavailable_message(what, decision, mode)),
        )

    return Verdict(
        run=True,
        effective_mode=mode,
        mode_source=mode_source,
        pending_change=(
            f"AIM could not be reached to verify {what} "
            f"({decision.reason or 'no detail available'}), and this process has "
            f"no cached enforcement mode for this agent, so the action was "
            f"executed unverified. Starting in aim-sdk 3.0.0 this case will block "
            f"instead. To get the blocking behaviour now, set AIM_STRICT_MODE=true."
        ),
        reportable=decision.verification_id is not None,
    )
