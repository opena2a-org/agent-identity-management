"""Tests for the causal-denial correlation envelope (Python SDK parity)."""
from aim_sdk.telemetry import (
    CORRELATION_HEADER,
    correlation_headers,
    extract_correlation_id,
    is_correlation_id,
    mint_correlation_id,
)


def test_minted_ids_are_well_formed_and_unique():
    ids = {mint_correlation_id() for _ in range(200)}
    assert len(ids) == 200  # no collisions
    for cid in ids:
        assert is_correlation_id(cid)
        assert cid.startswith("cde_")


def test_minted_ids_are_time_ordered():
    a = mint_correlation_id()
    b = mint_correlation_id()
    # base36 ms timestamp prefix keeps them lexicographically ordered by time.
    assert a[:13] <= b[:13]


def test_is_correlation_id_rejects_malformed():
    assert not is_correlation_id("nope")
    assert not is_correlation_id("cde_short_abc")
    assert not is_correlation_id(None)
    assert not is_correlation_id(12345)
    assert not is_correlation_id("cde_000000000_GGGGGGGGGGGG")  # non-hex suffix


def test_correlation_headers_only_for_valid_id():
    cid = mint_correlation_id()
    assert correlation_headers(cid) == {CORRELATION_HEADER: cid}
    assert correlation_headers(None) == {}
    assert correlation_headers("garbage") == {}


def test_extract_is_case_insensitive():
    cid = mint_correlation_id()
    assert extract_correlation_id({"X-OpenA2A-Correlation-Id": cid}) == cid
    assert extract_correlation_id({CORRELATION_HEADER: cid}) == cid
    assert extract_correlation_id({"x-opena2a-correlation-id": [cid]}) == cid


def test_extract_returns_none_when_absent_or_malformed():
    assert extract_correlation_id(None) is None
    assert extract_correlation_id({}) is None
    assert extract_correlation_id({CORRELATION_HEADER: "garbage"}) is None
