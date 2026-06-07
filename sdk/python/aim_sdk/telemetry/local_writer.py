"""
Causal-denial telemetry -- local correlated-record writer.

The full correlated record is authoritative and stays on the machine. It is
written to an append-only JSON-lines log, separate from the generic audit log so
the two schemas never mix.

Every write is best-effort: a failure returns False and is swallowed. The
telemetry pipeline must never be a runtime dependency -- if capture fails,
enforcement and signing continue unaffected (zero-failures principle).
"""
from __future__ import annotations

import json
import os
import time
from datetime import datetime, timezone
from typing import List, Optional

from .correlated_record import CorrelatedRecord, record_from_dict

CORRELATED_FILE = "correlated-events.jsonl"
_MAX_RECORD_SIZE = 1024 * 1024  # 1MB per record
_MAX_LOG_SIZE = 50 * 1024 * 1024  # rotate at 50MB
_MAX_ROTATED_LOGS = 5


def default_data_dir() -> str:
    """Default OpenA2A data directory (~/.opena2a)."""
    return os.path.join(os.path.expanduser("~"), ".opena2a")


def _rotate_if_needed(file_path: str) -> None:
    try:
        size = os.path.getsize(file_path)
    except OSError:
        # File doesn't exist yet -- nothing to rotate.
        return
    if size <= _MAX_LOG_SIZE:
        return

    suffix = f"{int(time.time() * 1000)}.{os.getpid()}.{os.urandom(4).hex()}"
    try:
        os.rename(file_path, f"{file_path}.{suffix}")
    except OSError:
        # Another process may have rotated already -- safe to continue.
        pass

    try:
        directory = os.path.dirname(file_path)
        base = os.path.basename(file_path)
        rotated = sorted(
            (f for f in os.listdir(directory) if f.startswith(base + ".") and f != base),
            reverse=True,
        )
        for old in rotated[_MAX_ROTATED_LOGS:]:
            try:
                os.unlink(os.path.join(directory, old))
            except OSError:
                pass  # best-effort
    except OSError:
        pass  # cleanup is best-effort


def write_correlated_record(
    record: CorrelatedRecord, data_dir: Optional[str] = None
) -> bool:
    """
    Append a correlated record to the local log. Returns True on success, False
    on any failure. Never raises.
    """
    if data_dir is None:
        data_dir = default_data_dir()
    try:
        serialized = json.dumps(record.to_dict(), separators=(",", ":"))
        if len(serialized) > _MAX_RECORD_SIZE:
            # Preserve the authoritative enforcement fact + join key; drop the rest.
            trimmed = {
                "schemaVersion": record.schema_version,
                "correlationId": record.correlation_id,
                "recordId": record.record_id,
                "agentId": record.agent_id,
                "createdAt": record.created_at,
                "enforcement": record.enforcement.to_dict(),
                "assembly": {
                    "joinedBy": record.assembly.joined_by,
                    "completeness": "partial-missing-both",
                    "joinerVersion": record.assembly.joiner_version,
                },
                "_truncated": True,
            }
            serialized = json.dumps(trimmed, separators=(",", ":"))

        os.makedirs(data_dir, exist_ok=True)
        file_path = os.path.join(data_dir, CORRELATED_FILE)
        _rotate_if_needed(file_path)
        with open(file_path, "a", encoding="utf-8") as fh:
            fh.write(serialized + "\n")
        return True
    except Exception:
        return False


def read_correlated_records(
    data_dir: Optional[str] = None,
    since: Optional[str] = None,
    limit: Optional[int] = None,
) -> List[CorrelatedRecord]:
    """Read correlated records back from the local log. Returns [] on any error."""
    if data_dir is None:
        data_dir = default_data_dir()
    try:
        file_path = os.path.join(data_dir, CORRELATED_FILE)
        if not os.path.exists(file_path):
            return []

        with open(file_path, "r", encoding="utf-8") as fh:
            lines = [ln for ln in fh.read().strip().split("\n") if ln]

        records: List[CorrelatedRecord] = []
        for line in lines:
            try:
                records.append(record_from_dict(json.loads(line)))
            except Exception:
                # Skip a corrupt/truncated line rather than failing the whole read.
                continue

        if since:
            since_ms = _to_ms(since)
            if since_ms is not None:
                records = [r for r in records if (_to_ms(r.created_at) or 0) > since_ms]
        if limit and limit > 0:
            records = records[-limit:]
        return records
    except Exception:
        return []


def _to_ms(iso: str) -> Optional[int]:
    """Parse an ISO-8601 timestamp to epoch milliseconds. None on failure."""
    if not iso:
        return None
    try:
        s = iso.strip()
        if s.endswith("Z"):
            s = s[:-1] + "+00:00"
        dt = datetime.fromisoformat(s)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return int(dt.timestamp() * 1000)
    except Exception:
        return None
