"""Tests for the local correlated-record writer."""
import os
import tempfile

from aim_sdk.telemetry import (
    CORRELATED_FILE,
    EnforcementInput,
    build_correlated_record,
    read_correlated_records,
    write_correlated_record,
)

CID = "cde_000000000_aabbccddeeff"


def _rec(created_at=None):
    rec = build_correlated_record(
        CID, "agent-1",
        EnforcementInput(decision="deny", outcome="DENY", capability="db:read",
                         resource="users", source="aim"),
    )
    if created_at:
        rec.created_at = created_at
    return rec


def test_write_then_read_roundtrip():
    with tempfile.TemporaryDirectory() as d:
        assert write_correlated_record(_rec(), d) is True
        assert write_correlated_record(_rec(), d) is True
        recs = read_correlated_records(d)
        assert len(recs) == 2
        assert recs[0].enforcement.decision == "deny"
        assert os.path.exists(os.path.join(d, CORRELATED_FILE))


def test_read_missing_dir_returns_empty():
    assert read_correlated_records("/nonexistent/path/xyz") == []


def test_since_and_limit_filters():
    with tempfile.TemporaryDirectory() as d:
        write_correlated_record(_rec("2026-06-06T00:00:00.000Z"), d)
        write_correlated_record(_rec("2026-06-06T00:00:05.000Z"), d)
        write_correlated_record(_rec("2026-06-06T00:00:10.000Z"), d)
        after = read_correlated_records(d, since="2026-06-06T00:00:03.000Z")
        assert len(after) == 2
        limited = read_correlated_records(d, limit=1)
        assert len(limited) == 1


def test_corrupt_line_is_skipped():
    with tempfile.TemporaryDirectory() as d:
        write_correlated_record(_rec(), d)
        with open(os.path.join(d, CORRELATED_FILE), "a", encoding="utf-8") as fh:
            fh.write("{ this is not valid json\n")
        write_correlated_record(_rec(), d)
        recs = read_correlated_records(d)
        assert len(recs) == 2  # corrupt middle line skipped, valid ones kept
