"""
Re-entry guard shared by the engine, the monitors and the interceptors.

An interceptor that patches ``builtins.open`` sees every file ARP itself opens:
the runtime twin appending to ``events.jsonl``, the budget controller saving
``budget.json``, a handler writing a log. Without a guard, that write is
intercepted, emitted, handled, written again, and so on. The TS reference
breaks the loop by excluding ARP's data directory; the Python port does that
too, and also marks the thread as "inside ARP" while the engine runs its
handlers and while a monitor runs its own ``ps``/``lsof`` call, so nothing ARP
does on that thread is intercepted.
"""
from __future__ import annotations

import contextlib
import threading

_state = threading.local()


def inside_arp() -> bool:
    """True while this thread is running ARP's own code (handlers, monitor polls)."""
    return getattr(_state, "depth", 0) > 0


@contextlib.contextmanager
def arp_internal():
    """Mark this thread as running ARP's own code for the duration of the block."""
    _state.depth = getattr(_state, "depth", 0) + 1
    try:
        yield
    finally:
        _state.depth -= 1
