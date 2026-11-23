"""
Size-capped JSONL logs, ported from ``sdk/typescript/src/arp/reporting/log-rotation.ts``.

A log above :data:`LOG_ROTATE_BYTES` is renamed to
``<name>.<13-digit ms>.<pid>.<8 hex>`` and only the
:data:`LOG_ROTATED_COPIES_KEPT` newest renamed copies are kept, the same names
the TS SDK writes, so the two SDKs can share ``~/.opena2a/arp``.
"""
from __future__ import annotations

import os
import re
import secrets
import time

LOG_ROTATE_BYTES = 50 * 1024 * 1024
LOG_ROTATED_COPIES_KEPT = 5


def rotate_log_if_needed(
    file_path: str, max_bytes: int = LOG_ROTATE_BYTES, keep: int = LOG_ROTATED_COPIES_KEPT
) -> bool:
    """Rename ``file_path`` aside when it is above ``max_bytes``. True when rotated."""
    try:
        # Rotate the link's target, not the link.
        file_path = os.path.realpath(file_path)
        size = os.stat(file_path).st_size
    except OSError:
        return False
    if size <= max_bytes:
        return False

    suffix = "%013d.%d.%s" % (int(time.time() * 1000), os.getpid(), secrets.token_hex(4))
    try:
        os.rename(file_path, "%s.%s" % (file_path, suffix))
    except OSError:
        return False  # another process rotated it first
    _delete_rotated_copies_beyond(file_path, keep)
    return True


def _delete_rotated_copies_beyond(file_path: str, keep: int) -> None:
    try:
        directory = os.path.dirname(file_path)
        rotated = re.compile(r"^%s\.\d{13}\.\d+\.[0-9a-f]{8}$" % re.escape(os.path.basename(file_path)))
        copies = sorted(name for name in os.listdir(directory) if rotated.match(name))
        for name in copies[: max(len(copies) - keep, 0)]:
            try:
                os.unlink(os.path.join(directory, name))
            except OSError:
                pass
    except OSError:
        pass
