"""
Causal-denial telemetry -- correlation envelope.

A correlation ID is minted at the earliest point in the request lifecycle
(content ingestion) and propagated best-effort to the injection detector and the
authorization call so that a single denied action's cause, intent, and
enforcement outcome can later be joined into one record.

Propagation is ALWAYS best-effort: a missing ID degrades telemetry quality but
must never block enforcement or signing. Nothing here raises.
"""
from __future__ import annotations

import os
import re
import time
from typing import Dict, Mapping, Optional, Union

#: Prefix marking an OpenA2A causal-denial-event correlation ID.
CORRELATION_ID_PREFIX = "cde_"

#: HTTP header that carries the correlation ID across process boundaries.
CORRELATION_HEADER = "x-opena2a-correlation-id"

#: W3C baggage key for in-process (trace-context) propagation.
CORRELATION_BAGGAGE_KEY = "opena2a.correlationId"

# Fixed-width, base36-encoded millisecond timestamp keeps IDs lexicographically
# ordered by time, which the async joiner relies on for time-window fallback.
_TS_WIDTH = 9
_CORRELATION_ID_RE = re.compile(r"^cde_[0-9a-z]{9}_[0-9a-f]{12}$")


def _base36(n: int) -> str:
    """Encode a non-negative int as lowercase base36 (no padding)."""
    if n == 0:
        return "0"
    digits = "0123456789abcdefghijklmnopqrstuvwxyz"
    out = []
    while n > 0:
        n, rem = divmod(n, 36)
        out.append(digits[rem])
    return "".join(reversed(out))


def mint_correlation_id() -> str:
    """
    Mint a time-ordered correlation ID. Never raises -- falls back to a non-crypto
    random suffix if the crypto source is unavailable.
    """
    ts = _base36(int(time.time() * 1000)).rjust(_TS_WIDTH, "0")[-_TS_WIDTH:]
    try:
        rand = os.urandom(6).hex()
    except Exception:
        # Best-effort fallback; correlation IDs are not a security boundary
        # (they only join records within one endpoint's local stream).
        import random

        rand = format(random.getrandbits(48), "012x")[-12:]
    return f"{CORRELATION_ID_PREFIX}{ts}_{rand}"


def is_correlation_id(value: object) -> bool:
    """True if the value is a well-formed correlation ID."""
    return isinstance(value, str) and bool(_CORRELATION_ID_RE.match(value))


def correlation_headers(correlation_id: Optional[str]) -> Dict[str, str]:
    """
    Build the header dict to attach to an outbound request (the Guard socket call
    or the authorize call). Returns an empty dict for an invalid ID so a caller
    can always merge it safely.
    """
    return {CORRELATION_HEADER: correlation_id} if is_correlation_id(correlation_id) else {}


def extract_correlation_id(
    headers: Optional[Mapping[str, Union[str, list, None]]]
) -> Optional[str]:
    """
    Extract a correlation ID from a headers-like mapping (case-insensitive).
    Returns None when absent or malformed -- never raises.
    """
    if not headers:
        return None
    try:
        for key in headers.keys():
            if key.lower() == CORRELATION_HEADER:
                raw = headers[key]
                value = raw[0] if isinstance(raw, (list, tuple)) else raw
                return value if is_correlation_id(value) else None
    except Exception:
        return None
    return None
