"""An unmeasured trust score must never be printed as if it were measured.

Regression tests for #390. Three distinct defects sit on this path, and each of
them independently produces a number the server never sent:

1. ``client.py`` defaulted to ``0.85``, so an agent the server returned no score
   for was announced to the operator as "Trust Score: 85%".
2. The same line used ``or`` rather than an explicit ``is None`` test, so a
   genuine server-sent ``0`` -- a real, meaningful, maximally-untrusted score --
   is falsy and fell through to that same fabricated 85%.
3. ``AIMConsole._normalize_trust_score`` mapped ``None`` to ``0.0``, which
   rendered an unknown score as a measured, alarming red "0%" and made the
   ``N/A`` branch in both formatters unreachable, because normalization runs
   before them.

A number on screen is a claim of measurement. Absent must render as absent.
"""

import pytest

from aim_sdk.console import AIMConsole


class TestNormalizeDoesNotInventAZero:
    def test_none_stays_none(self):
        # Was 0.0, which is a measurement claim about an agent nobody scored.
        assert AIMConsole._normalize_trust_score(None) is None

    def test_genuine_zero_is_preserved_and_is_not_none(self):
        # 0 is a real score and must survive as one, distinct from "unknown".
        assert AIMConsole._normalize_trust_score(0) == 0.0
        assert AIMConsole._normalize_trust_score(0) is not None

    def test_existing_normalization_still_holds(self):
        assert AIMConsole._normalize_trust_score(0.55) == 0.55
        assert AIMConsole._normalize_trust_score(55) == 0.55
        assert AIMConsole._normalize_trust_score(100) == 1.0
        assert AIMConsole._normalize_trust_score(1.0) == 1.0


class TestFormattersDistinguishUnknownFromZero:
    def test_inline_formatter_reports_unknown_not_zero_percent(self):
        out = AIMConsole()._format_trust_score_inline(None)
        assert "0%" not in out
        assert "N/A" in out

    def test_labelled_formatter_reports_unknown_not_zero_percent(self):
        out = AIMConsole()._format_trust_score(None)
        assert "0%" not in out
        assert "N/A" in out

    def test_a_genuine_zero_still_renders_as_zero(self):
        # The complement: fixing "unknown" must not swallow a real 0.
        assert "0%" in AIMConsole()._format_trust_score_inline(0)
        assert "0%" in AIMConsole()._format_trust_score(0)


class TestRegistrationPanelNeverReportsAScoreTheServerDidNotSend:
    """The end-to-end shape of #390, driven through the public entry point.

    These assert the VALUE the panel hands the renderer, not the rendered text.
    That is deliberate: #390 is a defect in what the value is, and scraping
    stdout here is not a sound oracle. ``AIMConsole`` binds rich's output stream
    at construction and the module-global console is built at import time, so
    whether ``capsys`` observes the panel depends on which test imported the
    module first. An earlier draft of this file did scrape stdout, passed alone,
    and failed in the full suite for exactly that reason.
    """

    @staticmethod
    def _reported_score(monkeypatch, credentials):
        from aim_sdk import client as client_mod

        seen = {}

        def spy(**kwargs):
            seen.update(kwargs)

        monkeypatch.setattr(client_mod.console, "agent_registered", spy)
        client_mod._print_registration_success("probe-agent", credentials)
        assert seen, "the panel never called agent_registered — oracle is blind"
        return seen["trust_score"]

    def test_absent_score_is_reported_as_unknown_not_as_85_percent(self, monkeypatch):
        score = self._reported_score(monkeypatch, {"agent_id": "abcdef1234567890"})
        assert score is None, f"fabricated {score!r} for an agent the server did not score"

    def test_server_sent_zero_survives_as_zero(self, monkeypatch):
        # The `or` bug: 0 is falsy, so it fell through to the 0.85 default.
        score = self._reported_score(
            monkeypatch, {"agent_id": "abcdef1234567890", "trust_score": 0}
        )
        assert score == 0, f"a genuine server-sent 0 was reported as {score!r}"

    def test_server_sent_zero_camel_case_survives_as_zero(self, monkeypatch):
        score = self._reported_score(
            monkeypatch, {"agent_id": "abcdef1234567890", "trustScore": 0}
        )
        assert score == 0

    def test_a_real_score_is_passed_through_unchanged(self, monkeypatch):
        # Non-vacuity control: the assertions above must be able to observe a
        # real score, or they would pass against a panel that reports nothing.
        score = self._reported_score(
            monkeypatch, {"agent_id": "abcdef1234567890", "trust_score": 0.85}
        )
        assert score == 0.85

    def test_camel_case_real_score_is_passed_through(self, monkeypatch):
        score = self._reported_score(
            monkeypatch, {"agent_id": "abcdef1234567890", "trustScore": 0.42}
        )
        assert score == 0.42

    def test_the_simple_renderer_accepts_an_unknown_score(self):
        # The value fix pushes None into the renderer, where the non-rich path
        # formatted with `:.0%` and would raise TypeError on None.
        AIMConsole(quiet=False)._agent_registered_simple(
            "probe-agent", "abcdef1234567890", "ai_agent", "1.0.0", None, "active"
        )
