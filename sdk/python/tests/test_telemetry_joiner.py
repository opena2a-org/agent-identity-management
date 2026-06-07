"""
Tests for the async joiner. Deterministic: a mutable clock is injected and
records are captured via a sink list, so no timers or filesystem are used.
"""
from aim_sdk.telemetry import (
    CorrelationJoiner,
    DetectionInput,
    EnforcementInput,
    IntentInput,
    is_high_confidence,
)

CID = "cde_000000000_aabbccddeeff"


def _enf():
    return EnforcementInput(decision="deny", outcome="DENY_INTENT", capability="fs:write",
                            resource="/etc/x", source="aim-pdp",
                            occurred_at="2026-06-06T00:00:00.000Z")


def _intent():
    return IntentInput(intent_class="exfil", confidence=0.7, blocked=True, source="nanomind-intent")


def _detection():
    return DetectionInput(injection_detected=True, confidence=0.84, detector="nanomind-guard",
                          technique_source="interim-mapping", technique_id="T-2002",
                          detected_at="2026-06-06T00:00:00.000Z")


class _Harness:
    def __init__(self, window_ms=5000):
        self.clock = 1000
        self.records = []
        self.joiner = CorrelationJoiner(
            window_ms=window_ms, now=lambda: self.clock,
            on_record=lambda r: self.records.append(r),
        )

    def advance(self, ms):
        self.clock += ms


def test_emits_full_record_immediately_any_order():
    h = _Harness()
    h.joiner.ingest_detection(CID, _detection())
    h.joiner.ingest_enforcement(CID, "agent-1", _enf())
    h.joiner.ingest_intent(CID, _intent())
    assert len(h.records) == 1
    assert h.records[0].assembly.completeness == "full"
    assert h.records[0].assembly.joined_by == "correlationId"
    assert h.joiner.pending == 0
    assert is_high_confidence(h.records[0]) is True


def test_does_not_emit_before_window():
    h = _Harness()
    h.joiner.ingest_enforcement(CID, "a", _enf())
    h.joiner.ingest_intent(CID, _intent())
    h.advance(4999)
    h.joiner.flush_expired()
    assert len(h.records) == 0
    assert h.joiner.pending == 1


def test_emits_partial_after_window():
    h = _Harness()
    h.joiner.ingest_enforcement(CID, "a", _enf())
    h.joiner.ingest_intent(CID, _intent())
    h.advance(5000)
    h.joiner.flush_expired()
    assert len(h.records) == 1
    assert h.records[0].assembly.completeness == "partial-missing-detection"
    assert h.records[0].assembly.joined_by == "fallback:window"
    assert is_high_confidence(h.records[0]) is False
    assert h.joiner.pending == 0


def test_enforcement_only_is_partial_missing_both():
    h = _Harness()
    h.joiner.ingest_enforcement(CID, "a", _enf())
    h.advance(5000)
    h.joiner.flush_expired()
    assert h.records[0].assembly.completeness == "partial-missing-both"


def test_drops_orphans_without_enforcement():
    h = _Harness()
    h.joiner.ingest_intent(CID, _intent())
    h.joiner.ingest_detection(CID, _detection())
    h.advance(5000)
    h.joiner.flush_expired()
    assert len(h.records) == 0
    assert h.joiner.dropped_orphans == 1
    assert h.joiner.pending == 0


def test_no_double_emit_after_full_join():
    h = _Harness()
    h.joiner.ingest_enforcement(CID, "a", _enf())
    h.joiner.ingest_intent(CID, _intent())
    h.joiner.ingest_detection(CID, _detection())
    assert len(h.records) == 1
    h.advance(10000)
    h.joiner.flush_expired()
    assert len(h.records) == 1


def test_separate_ids_are_independent():
    h = _Harness()
    a = "cde_000000000_aaaaaaaaaaaa"
    b = "cde_000000000_bbbbbbbbbbbb"
    h.joiner.ingest_enforcement(a, "x", _enf())
    h.joiner.ingest_enforcement(b, "x", _enf())
    h.joiner.ingest_intent(a, _intent())
    h.joiner.ingest_detection(a, _detection())
    assert len(h.records) == 1  # a completed
    assert h.joiner.pending == 1  # b still waiting


def test_caps_buffers_with_oldest_eviction():
    records = []
    joiner = CorrelationJoiner(window_ms=60_000, max_buffers=3,
                              on_record=lambda r: records.append(r))
    for s in ("aaaaaaaaaaaa", "bbbbbbbbbbbb", "cccccccccccc", "dddddddddddd"):
        joiner.ingest_enforcement(f"cde_000000000_{s}", "x", _enf())
    assert joiner.pending == 3
    assert joiner.dropped_overflow == 1
    # newest survivor still completes; eviction took the oldest ('a')
    joiner.ingest_intent("cde_000000000_dddddddddddd", _intent())
    joiner.ingest_detection("cde_000000000_dddddddddddd", _detection())
    assert len(records) == 1


def test_default_sink_defers_write_to_disk():
    import tempfile
    import time

    from aim_sdk.telemetry import read_correlated_records

    with tempfile.TemporaryDirectory() as d:
        joiner = CorrelationJoiner(data_dir=d)  # default deferred sink
        joiner.ingest_enforcement(CID, "agent-1", _enf())
        joiner.ingest_intent(CID, _intent())
        joiner.ingest_detection(CID, _detection())
        # write happens on the background writer thread; give it a moment
        for _ in range(50):
            if read_correlated_records(d):
                break
            time.sleep(0.02)
        recs = read_correlated_records(d)
        joiner.stop()
        assert len(recs) == 1
        assert recs[0].assembly.completeness == "full"


def test_overflow_drops_and_counts_without_sync_write():
    # Under deferred-queue saturation the sink must DROP + count, never perform a
    # synchronous disk write on the caller's (enforcement) thread.
    import queue as _q

    joiner = CorrelationJoiner(data_dir="/telemetry/should/not/write/here")
    # Pre-fill a tiny queue and stop the writer from draining it.
    joiner._write_queue = _q.Queue(maxsize=1)
    joiner._ensure_writer = lambda: None  # type: ignore[assignment]
    rec = build_record()
    joiner._write_queue.put_nowait(rec)  # saturate
    joiner._deferred_default_sink(rec)   # overflow -> drop, no raise
    joiner._deferred_default_sink(rec)   # again
    assert joiner.dropped_writes == 2
    # the saturating queue still holds exactly its one item (no drain, no extra)
    assert joiner._write_queue.qsize() == 1


def build_record():
    from aim_sdk.telemetry import build_correlated_record
    return build_correlated_record(CID, "a", _enf(), intent=_intent(), detection=_detection())
