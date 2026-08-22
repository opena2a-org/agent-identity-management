"""
AIM SDK - the typed three-state verification decision, and the enforcement-mode cache.

Before 2.0.0 a verification result was a dict whose ``verified`` key meant two
different things. ``verified: False`` was returned both when AIM DENIED the action
and when AIM was never successfully asked -- a 404, a 429, a 500, a connection
error. Callers could not tell those apart, so the SDK treated "we could not
determine" exactly like "permitted, with a warning".

This module replaces that bool with three states that cannot be confused:

* ``ALLOW``   -- AIM answered and permitted the action.
* ``DENY``    -- AIM answered and refused it.
* ``UNKNOWN`` -- no decision was obtained.

``UNKNOWN`` carries a second field, :class:`UnknownSource`, because the two ways to
reach it are not equivalent and must not be enforced the same way:

* ``TRANSPORT``     -- timeout, connection, DNS, TLS. We cannot tell whether AIM
  exists. This is the ONLY class eligible for permissive handling, and the
  allowlist is exhaustive by construction: it is populated from the transport
  exception, never from a status code.
* ``SERVER_ANSWER`` -- 400, 404, 409, 429, 500, 502, 503. The server answered; it
  simply did not carry a decision. An answer is never permissive. Being lenient
  here is what made 101 requests in a minute from the agent's own IP switch
  verification off for that IP, with no credentials and no exploit.

Enforcement mode resolution is strictly ordered: a fresh AIM answer, then a
process-scoped in-memory last-known mode within TTL, then ``UNKNOWN``.

**The cache is never persisted to disk.** A disk cache is forgeable in the lenient
direction -- an adversary does not suppress the cached mode, they write
``monitoring`` into it. Holding it in memory reduces forgery to "the adversary has
code execution in the agent process", at which point every client-side control is
already lost. Any persistence of this cache -- disk, keyring, cross-process --
voids that reasoning and makes a signed, expiring mode assertion mandatory.
"""

import threading
import time
from dataclasses import dataclass
from enum import Enum
from typing import Optional

# Unmeasured default. No production distribution of agent process lifetimes or of
# enforcement-mode change frequency has been measured, so this number is a choice,
# not a finding. It bounds how long a process keeps enforcing a mode the
# organization may have already changed.
DEFAULT_MODE_TTL_SECONDS = 300

# A reason longer than this is truncated with an explicit marker. The server
# accepts denial reasons up to its body limit; nothing a human wrote for a human
# needs more than this on a terminal.
REASON_MAX_CHARS = 500
_TRUNCATION_MARKER = " [truncated]"


def _sanitize_reason(raw: object) -> str:
    """
    Make a server-supplied string safe to place in an exception message or a
    console line (#384).

    Total over any input: a non-str value (a server sending ``{"reason": 123}``
    or a JSON object) is coerced with ``str()`` first. Raising here would turn
    an explicit DENY into UNKNOWN at construction time -- a fail-open -- so this
    function must never raise.

    C0 control bytes other than newline and tab, DEL, and the C1 range are
    stripped: ESC and the single-byte CSI drive the reader's terminal instead of
    being read, and carriage return overwrites the line the developer just saw.
    Newline itself is kept (ordinary whitespace, per #384's acceptance) -- a
    multi-line reason is an accepted residual, visible as indented continuation
    text. Unicode bidi controls are deliberately NOT stripped -- legitimate RTL
    text may carry them, and this value renders in contexts we do not control.
    """
    if not isinstance(raw, str):
        raw = str(raw)
    cleaned = "".join(
        ch for ch in raw if not ((ch < " " and ch not in "\n\t") or "\x7f" <= ch <= "\x9f")
    )
    if len(cleaned) > REASON_MAX_CHARS:
        cleaned = cleaned[:REASON_MAX_CHARS] + _TRUNCATION_MARKER
    return cleaned


class Outcome(str, Enum):
    """What AIM decided, or that it did not decide."""

    ALLOW = "allow"
    DENY = "deny"
    UNKNOWN = "unknown"


class UnknownSource(str, Enum):
    """Why an UNKNOWN outcome is UNKNOWN. Meaningless on ALLOW and DENY."""

    NONE = "none"
    TRANSPORT = "transport"
    SERVER_ANSWER = "server_answer"


class EnforcementMode(str, Enum):
    """The organization's enforcement mode, as resolved for this decision."""

    STRICT = "strict"
    MONITORING = "monitoring"
    UNKNOWN = "unknown"


class ModeSource(str, Enum):
    """Where the enforcement mode came from. Ordered: RESPONSE, CACHE, then none."""

    RESPONSE = "response"
    CACHE = "cache"
    ENV_OVERRIDE = "env_override"
    UNAVAILABLE = "unavailable"


@dataclass(frozen=True)
class VerificationDecision:
    """
    One verification decision.

    Deliberately NOT a Mapping. A dict-like shim would have to return something
    for ``verified`` on an UNKNOWN outcome, and any value it picked would rebuild
    the exact conflation this type exists to remove.
    """

    outcome: Outcome
    mode: EnforcementMode
    mode_source: ModeSource
    verification_id: Optional[str] = None
    status: Optional[str] = None
    reason: Optional[str] = None
    approved_by: Optional[str] = None
    expires_at: Optional[str] = None
    unknown_source: UnknownSource = UnknownSource.NONE

    def __post_init__(self):
        # Sanitised at construction rather than at each message site, so a new
        # message builder reading decision.reason cannot miss it (#384). This
        # covers only DECISION-BORNE strings: client.py also prints server text
        # directly (pre-decision console warnings, the 401 exception message)
        # and sanitises those at their own entry sites, with a control-byte
        # strip in AIMConsole as defence in depth. The other fields here are
        # returned to callers as data, and rewriting an id would corrupt it.
        if self.reason is not None:
            object.__setattr__(self, "reason", _sanitize_reason(self.reason))

    @property
    def allowed(self) -> bool:
        """True only on an explicit ALLOW. Never true on UNKNOWN."""
        return self.outcome is Outcome.ALLOW

    @property
    def denied(self) -> bool:
        return self.outcome is Outcome.DENY

    @property
    def undetermined(self) -> bool:
        return self.outcome is Outcome.UNKNOWN

    def to_legacy_dict(self) -> dict:
        """
        Project an ALLOW decision onto the dict `verify_capability` has always
        returned, plus the two additive mode keys.

        This is the ONLY place the legacy shape is constructed. 3.0.0 flips
        `verify_capability` to return the decision itself by dropping this call --
        if that flip ever needs more than one line, a caller has started
        hand-building the dict somewhere else and that is the defect.

        Only meaningful on ALLOW. DENY and UNKNOWN never return; they raise, so
        there is no `verified: False` anywhere in the SDK left to conflate.
        """
        if self.outcome is not Outcome.ALLOW:
            raise ValueError(
                "to_legacy_dict() is only defined for an ALLOW outcome; "
                f"got {self.outcome.value}. DENY and UNKNOWN raise instead of returning."
            )
        return {
            "verified": True,
            "verification_id": self.verification_id,
            "approved_by": self.approved_by,
            "expires_at": self.expires_at,
            # Additive in 2.0.0. The organization's enforcement mode finally
            # reaches the caller; before this it was read from a key no return
            # site ever emitted.
            "mode": self.mode.value,
            "mode_source": self.mode_source.value,
        }

    @property
    def permissive_eligible(self) -> bool:
        """
        Whether this outcome may be handled permissively at all.

        True only for a transport failure. A server that answered is never
        eligible, whatever it answered.
        """
        return (
            self.outcome is Outcome.UNKNOWN
            and self.unknown_source is UnknownSource.TRANSPORT
        )


def parse_enforcement_mode(raw: Optional[str]) -> EnforcementMode:
    """
    Map a wire value to an enforcement mode.

    An absent, empty, or unrecognised value is UNKNOWN -- never MONITORING. A
    degraded upstream must not be able to emit the healthy, lenient value by
    saying nothing, which is how the backend's own failed organization lookup
    used to produce "monitoring".
    """
    if raw is None:
        return EnforcementMode.UNKNOWN
    normalized = str(raw).strip().lower()
    if normalized == "strict":
        return EnforcementMode.STRICT
    if normalized == "monitoring":
        return EnforcementMode.MONITORING
    return EnforcementMode.UNKNOWN


@dataclass
class _CacheEntry:
    mode: EnforcementMode
    stored_at: float


class ModeCache:
    """
    Process-scoped, in-memory last-known enforcement mode, keyed by agent id.

    Uses a monotonic clock, so a system clock change cannot extend or expire an
    entry. Never written to disk, a keyring, or any cross-process store.
    """

    def __init__(self, ttl_seconds: int = DEFAULT_MODE_TTL_SECONDS):
        self.ttl_seconds = ttl_seconds
        self._entries = {}
        self._lock = threading.Lock()

    def put(self, agent_id: Optional[str], mode: EnforcementMode) -> None:
        """Record a mode AIM actually stated. UNKNOWN is never cached."""
        if not agent_id or mode is EnforcementMode.UNKNOWN:
            return
        with self._lock:
            self._entries[agent_id] = _CacheEntry(mode=mode, stored_at=time.monotonic())

    def get(self, agent_id: Optional[str]) -> Optional[EnforcementMode]:
        """Return the cached mode if one is present and within TTL, else None."""
        if not agent_id:
            return None
        with self._lock:
            entry = self._entries.get(agent_id)
            if entry is None:
                return None
            if time.monotonic() - entry.stored_at > self.ttl_seconds:
                del self._entries[agent_id]
                return None
            return entry.mode

    def clear(self) -> None:
        with self._lock:
            self._entries.clear()


# The single process-wide cache. Module-level so two AIMClient instances for the
# same agent in one process share a mode, and so it dies with the process.
_MODE_CACHE = ModeCache()


def get_mode_cache() -> ModeCache:
    return _MODE_CACHE


def resolve_mode(
    response_mode: EnforcementMode,
    agent_id: Optional[str],
    cache: Optional[ModeCache] = None,
) -> "tuple[EnforcementMode, ModeSource]":
    """
    Resolve the enforcement mode in the ruled order: fresh answer, then cache
    within TTL, then UNKNOWN.

    A fresh answer is also written back to the cache, so a later call that gets no
    mode can still enforce the organization's real setting.
    """
    store = cache if cache is not None else _MODE_CACHE

    if response_mode is not EnforcementMode.UNKNOWN:
        store.put(agent_id, response_mode)
        return response_mode, ModeSource.RESPONSE

    cached = store.get(agent_id)
    if cached is not None:
        return cached, ModeSource.CACHE

    return EnforcementMode.UNKNOWN, ModeSource.UNAVAILABLE
