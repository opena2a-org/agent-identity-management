"""
AIM SDK - the AIM_STRICT_MODE ratchet.

ONE parser, used by the SDK and by `demo_agent.py`. Before 2.0.0 there were two:
`decorators.py` accepted only the literal string ``"true"`` while `demo_agent.py`
accepted ``("true", "1", "yes")`` -- so ``AIM_STRICT_MODE=1`` executed a denied
action while our own demo printed "Strict Mode: ENABLED".

The variable is a RATCHET. It may only make enforcement stricter:

* a recognised true value forces strict mode, whatever the organization's
  enforcement mode says;
* a recognised false value is IGNORED with a warning -- it cannot relax an
  organization that is in strict mode;
* anything else is ignored with a warning.

A false value is a warning rather than a hard error on purpose: before 2.0.0 the
downward override was measurably a no-op (the organization's mode never reached
the decorator at all), so refusing to start would take agents down over a stale
`.env` while removing nothing anyone actually depended on.
"""

import os
import threading
from typing import Mapping, Optional

# Recognised spellings, compared case-insensitively after stripping whitespace.
TRUE_VALUES = frozenset({"true", "1", "yes", "on"})
FALSE_VALUES = frozenset({"false", "0", "no", "off"})

ENV_VAR = "AIM_STRICT_MODE"

# Warn once per distinct rejected value per process. A decorator resolves the
# ratchet on every wrapped call; without this a misconfigured agent would emit a
# warning per action for the life of the process.
_warned_values = set()
_warned_lock = threading.Lock()


def _warn_once(value: str, message: str) -> None:
    with _warned_lock:
        if value in _warned_values:
            return
        _warned_values.add(value)
    # Imported lazily: aim_sdk.console pulls in formatting machinery that the
    # parser itself does not need, and this module is imported at decoration time.
    from aim_sdk.console import console
    console.warning(message)


def parse_strict_mode(raw: Optional[str]) -> Optional[bool]:
    """
    Parse one raw AIM_STRICT_MODE value.

    Returns True for a recognised true value, False for a recognised false
    value, and None when the variable is unset, empty, or unrecognised.

    This is the pure form with no warnings and no environment access, so a test
    can assert the accepted spellings without capturing output.
    """
    if raw is None:
        return None
    normalized = raw.strip().lower()
    if not normalized:
        return None
    if normalized in TRUE_VALUES:
        return True
    if normalized in FALSE_VALUES:
        return False
    return None


def strict_mode_override(env: Optional[Mapping[str, str]] = None) -> bool:
    """
    Resolve the ratchet.

    Returns True only when AIM_STRICT_MODE names a recognised true value. Every
    other case returns False, meaning "the ratchet does not apply" -- NOT
    "enforcement is off". The caller keeps the organization's enforcement mode
    when this returns False.

    A recognised false value and an unrecognised value each warn once per
    process, naming the accepted spellings.
    """
    source = os.environ if env is None else env
    raw = source.get(ENV_VAR)
    if raw is None:
        return False

    parsed = parse_strict_mode(raw)
    if parsed is True:
        return True

    stripped = raw.strip()
    if not stripped:
        return False

    accepted = ", ".join(sorted(TRUE_VALUES))
    if parsed is False:
        _warn_once(
            stripped.lower(),
            f"{ENV_VAR}={stripped} is ignored. {ENV_VAR} can only raise enforcement, "
            f"never lower it -- it cannot put a strict-mode organization into "
            f"monitoring mode. Unset the variable to use the organization's "
            f"enforcement mode. To force strict mode set one of: {accepted}.",
        )
    else:
        _warn_once(
            stripped.lower(),
            f"{ENV_VAR}={stripped} is not a recognised value and is ignored. "
            f"To force strict mode set one of: {accepted}. Unset the variable to "
            f"use the organization's enforcement mode.",
        )
    return False


def reset_warning_state() -> None:
    """Clear the warn-once memory. For tests only."""
    with _warned_lock:
        _warned_values.clear()
