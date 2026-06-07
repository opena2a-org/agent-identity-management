"""
Causal-denial telemetry -- async joiner.

Buffers the three parts of a denied action (enforcement fact, intent inference,
detection inference) keyed by correlation ID, and emits one assembled record
when they meet -- fully when all parts arrive, or as a partial when the join
window elapses.

The joiner runs OFF the enforcement path. It holds parts in memory and emits via
a sink callback (default: the local writer, deferred to a background thread). It
is deterministic: time is supplied by an injectable clock, so production drives
it with a timer thread while tests drive it with an explicit clock.

An enforcement part is required to assemble a record (the fact anchors the join).
Parts that never receive an enforcement event within the window are dropped as
orphans rather than emitted.

Threading: the SDK is synchronous (requests-based), so ``ingest_*`` is called on
the caller's thread while ``flush_expired`` runs on a daemon timer thread. A lock
guards the buffer map. The default sink defers ``write_correlated_record`` to a
single daemon writer thread (mirrors the TS ``setImmediate`` deferral) so an
inline full-record assembly never adds disk latency to the enforcement path.
"""
from __future__ import annotations

import queue
import threading
import time
from typing import Callable, Dict, Optional

from .correlated_record import (
    CorrelatedRecord,
    DetectionInput,
    EnforcementInput,
    IntentInput,
    TELEMETRY_SCHEMA_VERSION,
    build_correlated_record,
)
from .local_writer import write_correlated_record

_DEFAULT_WINDOW_MS = 5000
# Hard cap on buffered correlation IDs. Without it, a flood of IDs arriving
# faster than the flush interval clears them grows the map unboundedly (memory
# exhaustion). At the cap, the oldest buffer is evicted to admit the new one.
_DEFAULT_MAX_BUFFERS = 10_000
# Bound on the deferred-write queue. On overflow the default sink falls back to a
# direct (synchronous) write -- still best-effort, never lost silently.
_DEFAULT_WRITE_QUEUE_MAX = 10_000


def _now_ms() -> int:
    return int(time.time() * 1000)


class _Buffer:
    __slots__ = ("agent_id", "enforcement", "intent", "detection", "first_seen_ms")

    def __init__(self, first_seen_ms: int) -> None:
        self.agent_id: Optional[str] = None
        self.enforcement: Optional[EnforcementInput] = None
        self.intent: Optional[IntentInput] = None
        self.detection: Optional[DetectionInput] = None
        self.first_seen_ms = first_seen_ms


class CorrelationJoiner:
    """
    Correlation-ID-keyed joiner. Call ``ingest_*`` as parts arrive and
    ``flush_expired()`` on a timer (or ``start()``/``stop()`` for a managed
    daemon thread).
    """

    def __init__(
        self,
        window_ms: int = _DEFAULT_WINDOW_MS,
        now: Optional[Callable[[], int]] = None,
        on_record: Optional[Callable[[CorrelatedRecord], None]] = None,
        joiner_version: str = TELEMETRY_SCHEMA_VERSION,
        max_buffers: int = _DEFAULT_MAX_BUFFERS,
        data_dir: Optional[str] = None,
    ) -> None:
        self._window_ms = window_ms if window_ms and window_ms > 0 else _DEFAULT_WINDOW_MS
        self._now = now or _now_ms
        self._joiner_version = joiner_version
        self._max_buffers = max_buffers if max_buffers and max_buffers > 0 else _DEFAULT_MAX_BUFFERS
        self._data_dir = data_dir

        self._buffers: "Dict[str, _Buffer]" = {}
        self._lock = threading.Lock()

        #: Count of part-sets dropped for never receiving an enforcement fact.
        self.dropped_orphans = 0
        #: Count of buffers evicted because the max_buffers cap was reached.
        self.dropped_overflow = 0
        #: Count of assembled records dropped because the deferred-write queue was
        #: saturated. Dropping (rather than a synchronous write) keeps disk latency
        #: off the enforcement path under extreme flood.
        self.dropped_writes = 0

        # Deferred-write plumbing for the default sink only.
        self._custom_sink = on_record
        self._write_queue: "Optional[queue.Queue]" = None
        self._writer_thread: Optional[threading.Thread] = None
        self._writer_stop = threading.Event()
        self._writer_lock = threading.Lock()
        if on_record is None:
            self._write_queue = queue.Queue(maxsize=_DEFAULT_WRITE_QUEUE_MAX)
            self._on_record: Callable[[CorrelatedRecord], None] = self._deferred_default_sink
        else:
            self._on_record = on_record

        # Managed flush timer.
        self._flush_thread: Optional[threading.Thread] = None
        self._flush_stop = threading.Event()

    @property
    def pending(self) -> int:
        """Number of correlation IDs currently buffered."""
        with self._lock:
            return len(self._buffers)

    # ----------------------------------------------------------------- ingest
    def ingest_enforcement(self, correlation_id: str, agent_id: str,
                           enforcement: EnforcementInput) -> None:
        emit = None
        with self._lock:
            buf = self._buffer_for(correlation_id)
            buf.enforcement = enforcement
            buf.agent_id = agent_id
            emit = self._take_complete(correlation_id)
        if emit is not None:
            self._on_record(emit)

    def ingest_intent(self, correlation_id: str, intent: IntentInput) -> None:
        emit = None
        with self._lock:
            buf = self._buffer_for(correlation_id)
            buf.intent = intent
            emit = self._take_complete(correlation_id)
        if emit is not None:
            self._on_record(emit)

    def ingest_detection(self, correlation_id: str, detection: DetectionInput) -> None:
        emit = None
        with self._lock:
            buf = self._buffer_for(correlation_id)
            buf.detection = detection
            emit = self._take_complete(correlation_id)
        if emit is not None:
            self._on_record(emit)

    # ------------------------------------------------------------------ flush
    def flush_expired(self, now_ms: Optional[int] = None) -> None:
        """
        Emit/drop any buffers whose window has elapsed. Buffers with an
        enforcement fact emit a partial record; buffers without one are dropped
        as orphans. The sink is invoked outside the lock.
        """
        if now_ms is None:
            now_ms = self._now()
        to_emit = []
        with self._lock:
            expired_ids = [
                cid for cid, buf in self._buffers.items()
                if now_ms - buf.first_seen_ms >= self._window_ms
            ]
            for cid in expired_ids:
                buf = self._buffers.pop(cid)
                if buf.enforcement is not None:
                    to_emit.append(self._build(cid, buf, "fallback:window"))
                else:
                    self.dropped_orphans += 1
        for record in to_emit:
            self._on_record(record)

    # ----------------------------------------------------------- timer thread
    def start(self, interval_ms: int = 1000) -> None:
        """Start a managed daemon flush thread (production use)."""
        if self._flush_thread is not None:
            return
        self._flush_stop.clear()
        interval = max(0.05, interval_ms / 1000.0)

        def _loop() -> None:
            while not self._flush_stop.wait(interval):
                try:
                    self.flush_expired()
                except Exception:
                    pass  # best-effort; the flush thread must never die

        self._flush_thread = threading.Thread(
            target=_loop, name="aim-telemetry-joiner-flush", daemon=True
        )
        self._flush_thread.start()

    def stop(self) -> None:
        """Stop the managed flush + writer threads. Drains pending writes."""
        self._flush_stop.set()
        thread = self._flush_thread
        self._flush_thread = None
        if thread is not None:
            thread.join(timeout=2.0)
        self._stop_writer()

    # ------------------------------------------------------------- internals
    def _buffer_for(self, correlation_id: str) -> _Buffer:
        buf = self._buffers.get(correlation_id)
        if buf is None:
            # Enforce the hard cap before admitting a new ID. dict preserves
            # insertion order and first_seen_ms is stamped at insertion, so the
            # first entry is the oldest -- evict it (oldest-eviction).
            if len(self._buffers) >= self._max_buffers:
                try:
                    oldest = next(iter(self._buffers))
                    del self._buffers[oldest]
                    self.dropped_overflow += 1
                except StopIteration:
                    pass
            buf = _Buffer(self._now())
            self._buffers[correlation_id] = buf
        return buf

    def _take_complete(self, correlation_id: str) -> Optional[CorrelatedRecord]:
        """If all parts are present, pop the buffer and return the record."""
        buf = self._buffers.get(correlation_id)
        if buf and buf.enforcement and buf.intent and buf.detection:
            del self._buffers[correlation_id]
            return self._build(correlation_id, buf, "correlationId")
        return None

    def _build(self, correlation_id: str, buf: _Buffer, joined_by: str) -> CorrelatedRecord:
        # enforcement presence is guaranteed by every call site.
        return build_correlated_record(
            correlation_id=correlation_id,
            agent_id=buf.agent_id or "",
            enforcement=buf.enforcement,  # type: ignore[arg-type]
            intent=buf.intent,
            detection=buf.detection,
            joined_by=joined_by,
            joiner_version=self._joiner_version,
        )

    # ---------------------------------------------------------- writer thread
    def _deferred_default_sink(self, record: CorrelatedRecord) -> None:
        self._ensure_writer()
        try:
            self._write_queue.put_nowait(record)  # type: ignore[union-attr]
        except queue.Full:
            # Queue saturated. The enforcement path's invariant -- never pay disk
            # latency -- takes priority over capturing every record, so under
            # extreme flood we DROP (best-effort telemetry) and count it rather
            # than performing a synchronous write on the caller's thread.
            self.dropped_writes += 1

    def _ensure_writer(self) -> None:
        if self._writer_thread is not None:
            return
        with self._writer_lock:
            if self._writer_thread is not None:
                return
            self._writer_stop.clear()
            self._writer_thread = threading.Thread(
                target=self._writer_loop, name="aim-telemetry-writer", daemon=True
            )
            self._writer_thread.start()

    def _writer_loop(self) -> None:
        assert self._write_queue is not None
        while not self._writer_stop.is_set():
            try:
                record = self._write_queue.get(timeout=0.5)
            except queue.Empty:
                continue
            try:
                write_correlated_record(record, self._data_dir)
            except Exception:
                pass
            finally:
                self._write_queue.task_done()
        # Drain whatever remains so a graceful stop() does not lose records.
        while True:
            try:
                record = self._write_queue.get_nowait()
            except queue.Empty:
                break
            try:
                write_correlated_record(record, self._data_dir)
            except Exception:
                pass

    def _stop_writer(self) -> None:
        self._writer_stop.set()
        thread = self._writer_thread
        self._writer_thread = None
        if thread is not None:
            thread.join(timeout=2.0)


def is_high_confidence(record: CorrelatedRecord) -> bool:
    """
    A full record is the high-confidence training signal: all parts present, the
    enforcement fact observed, and the technique source known. Partial or
    fallback-joined records are telemetry-only.
    """
    return (
        record.assembly.completeness == "full"
        and record.enforcement.observed is True
        and record.assembly.joined_by == "correlationId"
        and record.detection is not None
        and record.detection.technique_source is not None
    )
