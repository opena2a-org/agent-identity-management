"""Tests for AIMConsole trust score normalization."""

from aim_sdk.console import AIMConsole


class TestTrustScoreNormalization:
    """Verify _normalize_trust_score handles both 0-1 and 0-100 formats."""

    def test_float_zero_to_one_unchanged(self):
        assert AIMConsole._normalize_trust_score(0.55) == 0.55

    def test_integer_zero_to_hundred_normalized(self):
        assert AIMConsole._normalize_trust_score(55) == 0.55

    def test_boundary_one_stays(self):
        assert AIMConsole._normalize_trust_score(1.0) == 1.0

    def test_boundary_zero_stays(self):
        assert AIMConsole._normalize_trust_score(0) == 0.0

    def test_hundred_normalized(self):
        assert AIMConsole._normalize_trust_score(100) == 1.0

    def test_none_returns_zero(self):
        assert AIMConsole._normalize_trust_score(None) == 0.0

    def test_high_float_normalized(self):
        # 85 from server should become 0.85
        assert AIMConsole._normalize_trust_score(85) == 0.85

    def test_low_float_unchanged(self):
        assert AIMConsole._normalize_trust_score(0.1) == 0.1
