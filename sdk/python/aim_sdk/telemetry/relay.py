"""
Causal-denial telemetry -- SDK -> Registry relay (the leg that ships indicators up).

Reads full :class:`CorrelatedRecord`s from the local authoritative log
(``~/.opena2a/correlated-events.jsonl``), reduces each to an anonymized
:class:`SharedIndicator` via :func:`to_shared_indicator` (which strips every
identifier -- payloads, paths, credentials, resource/capability names,
correlationId, agentId, inputRef), and POSTs the denied-injection indicators to
the Registry's PUBLIC, count-only telemetry endpoint.

CA decision (ARC-AIM-RUNTIME-PLAN.md s5.3/s5.4):
  - D-R1: targets the PUBLIC ``POST /api/v1/telemetry/runtime`` (count-only, no
    side-effects). The authenticated ``/internal/telemetry/denial-correlated``
    path is reserved for a trusted internal aggregator -- an end-user SDK does
    not hold an ``internal:telemetry:write`` service-account token.
  - D-R2: the indicator carries no package identity; ``packageName`` is attached
    at POST time from static relay config (the integrator's self-declared sensor
    package, like sensorToken), defaulting to a sentinel.
  - D-R3: the relay has its OWN master switch, default OFF, separate from
    telemetry capture. Sharing is a second, explicit opt-in.
  - D-R5: only ``denied_injection_attempt`` indicators are uploaded.

Everything here is opt-in, consent-gated, off the enforcement critical path, and
best-effort: a failure is swallowed and never affects verify_capability.
"""
from __future__ import annotations

import hashlib
import json
import os
import re
import socket
import threading
from datetime import datetime, timezone
from typing import Callable, List, Optional

import requests

from .correlated_record import (
    CorrelatedRecord,
    SharedIndicator,
    SharedIndicatorContext,
    TELEMETRY_SCHEMA_VERSION,
    to_shared_indicator,
)
from .local_writer import read_correlated_records

#: Default Registry base URL (shared with hackmyagent's GTIN forwarder).
DEFAULT_REGISTRY_URL = "https://api.oa2a.org"
#: Public, count-only runtime telemetry path.
RUNTIME_TELEMETRY_PATH = "/api/v1/telemetry/runtime"
#: The only event type this relay uploads (CA D-R5).
RELAY_EVENT_TYPE = "denied_injection_attempt"
#: Sentinel package label used when the integrator declares none (CA D-R2).
SENTINEL_PACKAGE_NAME = "_unattributed"

_DEFAULT_BATCH_SIZE = 50
_DEFAULT_INTERVAL_MS = 60_000
_DEFAULT_TIMEOUT_MS = 10_000
_CURSOR_FILE = "correlated-relay-cursor.json"

# Egress validation: the relay GUARANTEES the shape of what leaves the machine
# rather than trusting the server to reject malformed values after transmission.
# Record-derived optional fields (techniqueId, techniqueSource, confidence) are
# validated here and OMITTED if they do not match -- a record tampered with
# locally cannot smuggle free text out via these fields.
_TECHNIQUE_ID_RE = re.compile(r"^T-\d{1,6}$")
_VALID_TECHNIQUE_SOURCES = frozenset(["apia", "interim-mapping"])
_MAX_AGENT_CATEGORY = 128
# Bound on same-millisecond boundary recordIds carried in the cursor.
_MAX_BOUNDARY_IDS = 1000


def open_a2a_home() -> str:
    """
    Resolve the OpenA2A home dir. An operator may override it with OPENA2A_HOME,
    but only an ABSOLUTE path is honored, and it is canonicalized with normpath()
    so a value like ``/a/../b`` collapses to its real location (no unresolved
    ``..`` segments). A relative or empty value is ignored in favor of ~/.opena2a
    rather than creating files relative to the current working directory.
    """
    configured = os.environ.get("OPENA2A_HOME")
    if configured and os.path.isabs(configured):
        return os.path.normpath(configured)
    return os.path.join(os.path.expanduser("~"), ".opena2a")


def _load_or_create_salt(data_dir: str) -> str:
    """
    Read the persisted salt, or create a fresh owner-only one. The salt's
    confidentiality is what anonymizes the sensor token (host+user are
    low-entropy), so an EXISTING salt is reused only when it is a regular file,
    owned by us, with no group/other permission bits. A pre-created
    world-readable or wrong-owner salt is NOT trusted: it is overwritten with a
    fresh 0600 salt, closing the "attacker pre-creates a readable salt to
    de-anonymize the sensor token" vector. Permission checks apply on POSIX only
    (mode bits are not meaningful on Windows, where ACLs govern access).
    """
    salt_path = os.path.join(data_dir, "gtin-sensor-salt")
    is_posix = hasattr(os, "getuid")
    if os.path.exists(salt_path):
        try:
            st = os.stat(salt_path)
            import stat as _stat

            is_file = _stat.S_ISREG(st.st_mode)
            owner_only = (not is_posix) or (st.st_mode & 0o077) == 0
            same_owner = (not is_posix) or st.st_uid == os.getuid()  # type: ignore[attr-defined]
            if is_file and owner_only and same_owner:
                with open(salt_path, "r", encoding="utf-8") as fh:
                    existing = fh.read().strip()
                if existing:
                    return existing
            # Untrusted (loose perms, wrong owner, not a file, or empty) -- replace.
        except OSError:
            pass  # fall through to (re)create

    salt = os.urandom(32).hex()
    os.makedirs(data_dir, exist_ok=True)
    # Create with 0600 from the start (mirror writeFileSync({mode:0o600})).
    fd = os.open(salt_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    try:
        os.write(fd, salt.encode("utf-8"))
    finally:
        os.close(fd)
    # Tighten perms even if the file pre-existed with looser bits.
    try:
        os.chmod(salt_path, 0o600)
    except OSError:
        pass  # best-effort
    return salt


def derive_sensor_token(data_dir: Optional[str] = None) -> str:
    """
    Stable per-device sensor token: sha256(hostname + username + persisted salt).
    Uses the SAME salt file as hackmyagent's GTIN module so one device maps to one
    anonymous sensor identity across tools. Best-effort; falls back to an ephemeral
    token if the salt cannot be persisted.
    """
    if data_dir is None:
        data_dir = open_a2a_home()
    try:
        salt = _load_or_create_salt(data_dir)
    except Exception:
        # Cannot persist a salt -- use an ephemeral one. Still anonymous; just not
        # stable across process restarts.
        salt = os.urandom(32).hex()

    host = "unknown"
    user = "unknown"
    try:
        host = socket.gethostname() or "unknown"
    except Exception:
        pass
    try:
        import getpass

        user = getpass.getuser() or "unknown"
    except Exception:
        pass
    return hashlib.sha256(f"{host}|{user}|{salt}".encode("utf-8")).hexdigest()


def detect_runtime_env() -> str:
    """Runtime environment for the shared indicator. This is the Python SDK."""
    return "python"


def days_since_install(data_dir: Optional[str] = None) -> int:
    """
    Whole days since the relay was first used on this device. Marker file shared
    with the GTIN module. Best-effort; returns 0 if it cannot be read/written.
    """
    if data_dir is None:
        data_dir = open_a2a_home()
    try:
        marker_path = os.path.join(data_dir, "gtin-install-date")
        install_dt: Optional[datetime] = None
        if os.path.exists(marker_path):
            with open(marker_path, "r", encoding="utf-8") as fh:
                raw = fh.read().strip()
            install_dt = _parse_iso(raw)
            if install_dt is None:
                install_dt = datetime.now(timezone.utc)
                _write_marker(marker_path, install_dt)
        else:
            install_dt = datetime.now(timezone.utc)
            os.makedirs(data_dir, exist_ok=True)
            _write_marker(marker_path, install_dt)
        diff_ms = datetime.now(timezone.utc).timestamp() - install_dt.timestamp()
        return max(0, int(diff_ms // (24 * 60 * 60)))
    except Exception:
        return 0


def _write_marker(marker_path: str, dt: datetime) -> None:
    fd = os.open(marker_path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    try:
        os.write(fd, _iso(dt).encode("utf-8"))
    finally:
        os.close(fd)


def _iso(dt: datetime) -> str:
    return dt.strftime("%Y-%m-%dT%H:%M:%S.") + f"{dt.microsecond // 1000:03d}Z"


def _parse_iso(raw: str) -> Optional[datetime]:
    if not raw:
        return None
    try:
        s = raw.strip()
        if s.endswith("Z"):
            s = s[:-1] + "+00:00"
        dt = datetime.fromisoformat(s)
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt
    except Exception:
        return None


def _to_ms(iso: str) -> Optional[int]:
    dt = _parse_iso(iso)
    return int(dt.timestamp() * 1000) if dt else None


# Transport: (url, body_dict, timeout_seconds) -> http_status_code. May raise on
# network/timeout (treated as a transient 'failed'). Injectable for tests.
Transport = Callable[[str, dict, float], int]


def _default_transport(url: str, body: dict, timeout: float) -> int:
    resp = requests.post(
        url,
        data=json.dumps(body),
        headers={
            "Content-Type": "application/json",
            "User-Agent": "OpenA2A-AIM-SDK-Relay",
        },
        timeout=timeout,
    )
    return resp.status_code


class FlushResult:
    """Outcome of a single flush, for tests/observability. Never raised."""

    __slots__ = ("considered", "uploaded", "stopped_on_error")

    def __init__(self, considered: int = 0, uploaded: int = 0,
                 stopped_on_error: bool = False) -> None:
        self.considered = considered
        self.uploaded = uploaded
        self.stopped_on_error = stopped_on_error

    def __repr__(self) -> str:
        return (f"FlushResult(considered={self.considered}, uploaded={self.uploaded}, "
                f"stopped_on_error={self.stopped_on_error})")


class _RelayCursor:
    __slots__ = ("last_created_at", "last_record_ids")

    def __init__(self, last_created_at: str, last_record_ids: List[str]) -> None:
        self.last_created_at = last_created_at
        self.last_record_ids = last_record_ids


class CorrelatedRelay:
    """
    Ships locally-captured causal-denial indicators to the Registry.

    Call :meth:`start` for a managed daemon flush timer, or :meth:`flush_once` to
    drive it manually (tests, one-shot CLI). Inert unless ``enabled`` is True.
    """

    def __init__(
        self,
        enabled: bool = False,
        registry_url: Optional[str] = None,
        package_name: Optional[str] = None,
        package_version: Optional[str] = None,
        agent_category: Optional[str] = None,
        data_dir: Optional[str] = None,
        batch_size: Optional[int] = None,
        interval_ms: Optional[int] = None,
        timeout_ms: Optional[int] = None,
        sensor_token: Optional[str] = None,
        transport: Optional[Transport] = None,
        on_error: Optional[Callable[[BaseException], None]] = None,
    ) -> None:
        self._enabled = enabled is True
        base = (registry_url or DEFAULT_REGISTRY_URL).rstrip("/")
        self._url = base + RUNTIME_TELEMETRY_PATH
        self._package_name = (package_name or "").strip() or SENTINEL_PACKAGE_NAME
        self._package_version = (package_version or "").strip() or None
        self._agent_category = ((agent_category or "").strip() or "unspecified")[:_MAX_AGENT_CATEGORY]
        self._data_dir = data_dir or open_a2a_home()
        self._batch_size = batch_size if batch_size and batch_size > 0 else _DEFAULT_BATCH_SIZE
        self._interval_ms = interval_ms if interval_ms and interval_ms > 0 else _DEFAULT_INTERVAL_MS
        self._timeout_ms = timeout_ms if timeout_ms and timeout_ms > 0 else _DEFAULT_TIMEOUT_MS
        self._sensor_token = sensor_token or derive_sensor_token(self._data_dir)
        self._transport = transport or _default_transport
        self._on_error = on_error or (lambda err: None)

        self._timer_thread: Optional[threading.Thread] = None
        self._timer_stop = threading.Event()
        self._flush_lock = threading.Lock()
        self._flushing = False

    @property
    def enabled(self) -> bool:
        return self._enabled

    def start(self) -> None:
        """Start a managed daemon flush timer. No-op when disabled or running."""
        if not self._enabled or self._timer_thread is not None:
            return
        self._timer_stop.clear()
        interval = max(0.05, self._interval_ms / 1000.0)

        def _loop() -> None:
            while not self._timer_stop.wait(interval):
                try:
                    self.flush_once()
                except Exception as err:  # pragma: no cover - flush_once never raises
                    self._on_error(err)

        self._timer_thread = threading.Thread(
            target=_loop, name="aim-telemetry-relay", daemon=True
        )
        self._timer_thread.start()

    def stop(self) -> None:
        """Stop the managed interval. Does not flush."""
        self._timer_stop.set()
        thread = self._timer_thread
        self._timer_thread = None
        if thread is not None:
            thread.join(timeout=2.0)

    def flush_once(self) -> FlushResult:
        """
        Read fresh records, upload eligible indicators, advance the cursor. Never
        raises. Returns a summary. Re-entrancy-guarded so overlapping timer ticks
        cannot double-send.
        """
        result = FlushResult()
        if not self._enabled:
            return result
        # Non-blocking re-entrancy guard.
        with self._flush_lock:
            if self._flushing:
                return result
            self._flushing = True
        try:
            cursor = self._read_cursor()
            fresh = self._read_fresh(cursor)
            result.considered = len(fresh)
            if not fresh:
                return result

            ctx = SharedIndicatorContext(
                sensor_token=self._sensor_token,
                agent_category=self._agent_category,
                day_since_install=days_since_install(self._data_dir),
                runtime_env=detect_runtime_env(),
            )

            last_created_at = cursor.last_created_at if cursor else None
            boundary_ids: List[str] = list(cursor.last_record_ids) if cursor else []

            for rec in fresh:
                if result.uploaded >= self._batch_size:
                    break

                indicator = to_shared_indicator(rec, ctx)
                if indicator.event_type == RELAY_EVENT_TYPE:
                    outcome = self._post(indicator)
                    if outcome == "failed":
                        # Transient failure: leave this record (and the rest) for
                        # the next flush; do NOT advance the cursor past it.
                        result.stopped_on_error = True
                        break
                    if outcome == "uploaded":
                        result.uploaded += 1
                    # 'rejected' (4xx): non-retryable, advance without counting.

                # Record handled (uploaded, or skipped because ineligible). Advance.
                if rec.created_at == last_created_at:
                    boundary_ids.append(rec.record_id)
                    if len(boundary_ids) > _MAX_BOUNDARY_IDS:
                        boundary_ids = boundary_ids[-_MAX_BOUNDARY_IDS:]
                else:
                    last_created_at = rec.created_at
                    boundary_ids = [rec.record_id]

            prev_created = cursor.last_created_at if cursor else None
            prev_len = len(cursor.last_record_ids) if cursor else 0
            if last_created_at and last_created_at != prev_created:
                self._write_cursor(_RelayCursor(last_created_at, boundary_ids))
            elif last_created_at and len(boundary_ids) != prev_len:
                self._write_cursor(_RelayCursor(last_created_at, boundary_ids))
        except Exception as err:
            self._on_error(err)
        finally:
            self._flushing = False
        return result

    def _read_fresh(self, cursor: Optional[_RelayCursor]) -> List[CorrelatedRecord]:
        """
        Records not yet handled, in stable (createdAt, recordId) order. Assumes
        the local log's createdAt is roughly monotonic (it is -- records are
        appended in assembly order by a single writer). A record appended out of
        order with a createdAt earlier than the cursor is skipped; acceptable
        best-effort data loss for anonymized count telemetry.
        """
        since_param: Optional[str] = None
        if cursor:
            cur_ms = _to_ms(cursor.last_created_at)
            if cur_ms is not None:
                since_param = _iso(datetime.fromtimestamp((cur_ms - 1) / 1000.0, tz=timezone.utc))
        records = read_correlated_records(self._data_dir, since=since_param)

        cutoff = _to_ms(cursor.last_created_at) if cursor else None
        fresh: List[CorrelatedRecord] = []
        for r in records:
            t = _to_ms(r.created_at)
            if t is None:
                continue
            if cutoff is None or t > cutoff:
                fresh.append(r)
            elif t == cutoff and cursor and r.record_id not in cursor.last_record_ids:
                fresh.append(r)

        fresh.sort(key=lambda r: ((_to_ms(r.created_at) or 0), r.record_id))
        return fresh

    def _post(self, indicator: SharedIndicator) -> str:
        """
        POST one indicator. Never raises.
          - 'uploaded': 2xx -- counted and cursor advances.
          - 'rejected': 4xx -- non-retryable (bad/duplicate indicator); cursor
             advances but it is NOT counted as an upload.
          - 'failed':   5xx / network / timeout -- transient; cursor stays put.
        """
        body = {
            "schemaVersion": TELEMETRY_SCHEMA_VERSION,
            "sensorToken": indicator.sensor_token,
            "eventType": indicator.event_type,
            "packageName": self._package_name,
            "runtimeEnv": indicator.runtime_env,
            "triggeredAt": indicator.triggered_at,
            "daySinceInstall": indicator.day_since_install,
            "enforcementOutcome": indicator.enforcement_outcome,
            "agentCategory": indicator.agent_category,
        }
        if self._package_version:
            body["packageVersion"] = self._package_version
        # Validate record-derived optional fields before egress; omit on mismatch
        # so malformed/tampered values never leave the machine.
        if indicator.technique_id and _TECHNIQUE_ID_RE.match(indicator.technique_id):
            body["techniqueId"] = indicator.technique_id
        if indicator.technique_source and indicator.technique_source in _VALID_TECHNIQUE_SOURCES:
            body["techniqueSource"] = indicator.technique_source
        if (
            isinstance(indicator.detection_confidence, (int, float))
            and not isinstance(indicator.detection_confidence, bool)
            and 0 <= indicator.detection_confidence <= 1
        ):
            body["detectionConfidence"] = indicator.detection_confidence

        try:
            status = self._transport(self._url, body, self._timeout_ms / 1000.0)
        except Exception as err:
            self._on_error(err)
            return "failed"

        # 4xx (bad/duplicate indicator) is non-retryable from our side: treat as
        # handled so the cursor advances. Only 5xx / network errors block.
        if 200 <= status < 300:
            return "uploaded"
        if 400 <= status < 500:
            self._on_error(RuntimeError(f"relay rejected indicator: HTTP {status}"))
            return "rejected"
        self._on_error(RuntimeError(f"relay upload failed: HTTP {status}"))
        return "failed"

    # Cursor persistence assumes a SINGLE relay process per data_dir (the common
    # case: one SDK instance per agent process). It is not file-locked: two
    # processes sharing a data_dir can race and re-send overlapping batches. That
    # only produces duplicate count-only indicators on the public endpoint --
    # acceptable for best-effort anonymized telemetry -- and never corrupts a
    # record or leaks data.
    def _cursor_path(self) -> str:
        return os.path.join(self._data_dir, _CURSOR_FILE)

    def _read_cursor(self) -> Optional[_RelayCursor]:
        try:
            with open(self._cursor_path(), "r", encoding="utf-8") as fh:
                parsed = json.load(fh)
            if isinstance(parsed.get("lastCreatedAt"), str) and isinstance(
                parsed.get("lastRecordIds"), list
            ):
                return _RelayCursor(parsed["lastCreatedAt"], list(parsed["lastRecordIds"]))
        except Exception:
            # No cursor yet, or corrupt -- start from the beginning.
            pass
        return None

    def _write_cursor(self, cursor: _RelayCursor) -> None:
        try:
            os.makedirs(self._data_dir, exist_ok=True)
            payload = json.dumps(
                {"lastCreatedAt": cursor.last_created_at, "lastRecordIds": cursor.last_record_ids}
            )
            path = self._cursor_path()
            fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
            try:
                os.write(fd, payload.encode("utf-8"))
            finally:
                os.close(fd)
        except Exception as err:
            self._on_error(err)
