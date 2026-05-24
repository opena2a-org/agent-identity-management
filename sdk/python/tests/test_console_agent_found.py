"""Regression coverage for #175: AIMConsole.agent_found() must not print
the cached status or trust score. Those values live in the local credentials
file from registration time and never refresh, so anything printed would lie
once the agent is promoted or its trust score changes server-side."""

import io
from contextlib import redirect_stdout

from aim_sdk.console import AIMConsole


class TestAgentFoundBannerDoesNotLeakStaleFields:
    """The banner shown when SDK loads cached credentials must stick to
    identity (agent name + ID). Status and trust score belong on the
    dashboard, which is authoritative."""

    def _capture_banner(self) -> str:
        """Render the existing-credentials banner with both rich and plain
        backends captured. Each backend writes to a different stream, so we
        join the stdout-printed plain branch (RICH_AVAILABLE=False) and the
        rich console's recorded output (RICH_AVAILABLE=True)."""
        console = AIMConsole(quiet=False)
        buf = io.StringIO()

        # Rich backend (if available) writes to its own Console; redirect by
        # swapping the underlying file handle to our buffer.
        if console.console is not None:
            console.console.file = buf

        with redirect_stdout(buf):
            console.agent_found(
                name="flight-search-agent",
                agent_id="391be6bf-0b21-4b63-979d-d53d03f0deae",
            )

        return buf.getvalue()

    def test_does_not_print_status_label(self):
        out = self._capture_banner()
        assert "Status:" not in out, (
            "Banner must not show cached Status (#175): credentials file "
            "only has the registration-time snapshot. Got:\n" + out
        )

    def test_does_not_print_trust_score_label(self):
        out = self._capture_banner()
        assert "Trust Score:" not in out and "Trust:" not in out, (
            "Banner must not show cached Trust Score (#175). Got:\n" + out
        )

    def test_does_not_print_percent_sign(self):
        # The cached trust score was rendered as a percentage. If '%' appears
        # in the banner, something is leaking it back in.
        out = self._capture_banner()
        assert "%" not in out, (
            "Banner must not render any percentage (#175 — was cached "
            "trust). Got:\n" + out
        )

    def test_still_shows_name_and_id(self):
        out = self._capture_banner()
        assert "flight-search-agent" in out
        assert "391be6bf" in out, "Short-form ID prefix must still appear"

    def test_quiet_mode_suppresses_everything(self):
        console = AIMConsole(quiet=True)
        buf = io.StringIO()
        if console.console is not None:
            console.console.file = buf
        with redirect_stdout(buf):
            console.agent_found(
                name="any",
                agent_id="00000000-0000-0000-0000-000000000000",
            )
        assert buf.getvalue() == ""
