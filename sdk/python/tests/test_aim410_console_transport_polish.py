"""Issue #410: console and transport polish from the 2.0.1 walkthrough.

Items 2 (login banner), 5 (raw urllib3 text on logout) and 6 (``status --json``)
were already fixed on main when this was picked up; the banner and ``--json``
stay covered by their own tests. This file pins items 1, 2 (the panels), 3 and
4, and one adjacent logout defect found while reproducing item 5: the legacy
``~/.aim/credentials.json`` survived ``logout``, so the next ``status``
migrated it again and read as signed in.
"""

import contextlib
import io
import json
import os
import subprocess
import sys
from pathlib import Path

import pytest

import aim_sdk
import aim_sdk.console as console_module
from aim_sdk.a2a import A2AClient
from aim_sdk.console import AIMConsole, _short_id
from aim_sdk.telemetry import relay

SDK_ROOT = Path(__file__).resolve().parents[1]
BORDER_WIDTH = 51


@pytest.fixture
def plain_console(monkeypatch):
    monkeypatch.setattr(console_module, "RICH_AVAILABLE", False)
    return AIMConsole()


def _panel_rows(fn):
    buf = io.StringIO()
    with contextlib.redirect_stdout(buf):
        fn()
    return [line for line in buf.getvalue().splitlines() if line and line[0] in "╭├│╰"]


# --- item 1: short ids -------------------------------------------------------

@pytest.mark.parametrize("agent_id", ["agt_123", "abc", "a" * 15])
def test_short_ids_are_shown_whole(agent_id):
    assert _short_id(agent_id) == agent_id


def test_long_ids_keep_first_eight_and_last_four():
    assert _short_id("3f2a9c1e-7b44-4d2e-9a61-0c5e8f1d2b77") == "3f2a9c1e...2b77"


def test_registered_panel_does_not_duplicate_a_short_id(plain_console):
    rows = _panel_rows(lambda: plain_console._agent_registered_simple(
        "demo", "agt_123", "claude", "1.0", 0.5, "active"))
    id_row = next(r for r in rows if "ID:" in r)
    assert "agt_123" in id_row
    assert "..." not in id_row


# --- item 2: every panel row is as wide as its border ------------------------

@pytest.mark.parametrize("render", [
    lambda c: c._agent_registered_simple(
        "demo-agent", "agt_123", "claude", "1.0", 0.87, "active",
        ["read", "write", "net", "x"], ["fs"]),
    lambda c: c._agent_registered_simple(
        "a-very-long-agent-name-that-overflows-the-panel-field", "3f2a9c1e-7b44-4d2e-9a61-0c5e8f1d2b77",
        "claude", "1.0", None, "pending"),
    lambda c: c.agent_found("demo-agent", "abc"),
    lambda c: c.jit_waiting("fs:write", "high", 300),
])
def test_plain_panels_align(plain_console, render):
    rows = _panel_rows(lambda: render(plain_console))
    assert rows, "the plain-text panel must render"
    assert {len(r) for r in rows} == {BORDER_WIDTH}, rows


# --- item 3: out-of-range trust scores are not graded -----------------------

@pytest.mark.parametrize("score", [150, -5, float("nan")])
def test_out_of_range_score_renders_invalid(score):
    c = AIMConsole()
    assert "invalid" in c._format_trust_score(score)
    assert "invalid" in c._format_trust_score_inline(score)
    assert "%" not in c._format_trust_score(score)


@pytest.mark.parametrize("score,expected", [(0.87, "87%"), (87, "87%"), (0, "0%"), (1, "100%")])
def test_in_range_scores_still_grade(score, expected):
    assert expected in AIMConsole()._format_trust_score(score)


def test_registered_panel_marks_out_of_range_score_invalid(plain_console):
    rows = _panel_rows(lambda: plain_console._agent_registered_simple(
        "demo", "agt_123", "claude", "1.0", 1.5, "active"))
    trust_row = next(r for r in rows if "Trust Score:" in r)
    assert "invalid" in trust_row and "150%" not in trust_row


# --- item 4: User-Agent strings carry the package version -------------------

def test_a2a_client_sends_the_package_version():
    client = A2AClient(aim_client=object())
    ua = client._session.headers["User-Agent"]
    assert ua == f"AIM-Python-SDK/{aim_sdk.__version__} (A2A)"
    assert ua.startswith("AIM-Python-SDK/"), "the backend's SDK tracking matches this prefix"


def test_relay_sends_the_package_version(monkeypatch):
    seen = {}

    class _Resp:
        status_code = 202

    def fake_post(url, data=None, headers=None, timeout=None):
        seen["headers"] = headers
        return _Resp()

    monkeypatch.setattr(relay.requests, "post", fake_post)
    assert relay._default_transport("http://127.0.0.1:9/x", {"a": 1}, 1.0) == 202
    assert seen["headers"]["User-Agent"] == f"OpenA2A-AIM-SDK-Relay/{aim_sdk.__version__}"


# --- adjacent: logout clears the legacy SDK credentials ----------------------

def _run_cli(home: Path, *args: str) -> subprocess.CompletedProcess:
    env = {k: v for k, v in os.environ.items() if not k.startswith("AIM_")}
    env.update({"HOME": str(home), "USERPROFILE": str(home)})
    return subprocess.run(
        [sys.executable, "-m", "aim_sdk", *args],
        cwd=SDK_ROOT, env=env, capture_output=True, text=True, timeout=60,
    )


def test_logout_removes_legacy_sdk_credentials_so_status_stays_signed_out(tmp_path):
    aim_dir = tmp_path / ".aim"
    aim_dir.mkdir()
    legacy = aim_dir / "credentials.json"
    legacy.write_text(json.dumps({
        "aim_url": "http://127.0.0.1:9",
        "refresh_token": "legacy.refresh.token",
        "access_token": "legacy.access.token",
        "user_id": "u1",
    }))

    _run_cli(tmp_path, "logout")
    assert not legacy.exists()
    assert not (aim_dir / "sdk_credentials.json").exists()

    status = _run_cli(tmp_path, "status")
    assert "Not authenticated" in status.stdout
    assert not (aim_dir / "sdk_credentials.json").exists()


def test_logout_keeps_agent_credentials_that_share_the_legacy_file(tmp_path):
    aim_dir = tmp_path / ".aim"
    aim_dir.mkdir()
    legacy = aim_dir / "credentials.json"
    legacy.write_text(json.dumps({
        "aim_url": "http://127.0.0.1:9",
        "refresh_token": "legacy.refresh.token",
        "my-agent": {"agent_id": "agt_1", "private_key": "k"},
    }))

    _run_cli(tmp_path, "logout")
    kept = json.loads(legacy.read_text())
    assert kept["my-agent"] == {"agent_id": "agt_1", "private_key": "k"}
    assert "refresh_token" not in kept
    if os.name == "posix":
        assert (legacy.stat().st_mode & 0o777) == 0o600
