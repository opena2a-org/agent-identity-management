"""
Regression test: `aim-sdk login` must not crash with a raw EOFError traceback
when stdin is non-interactive (CI / piped) and credentials already exist.
Previously the unguarded input("Re-authenticate? [y/N]: ") raised EOFError and
dumped a stack trace; now it keeps the existing credentials and exits cleanly.
"""

from types import SimpleNamespace
from unittest.mock import patch
import io
from contextlib import redirect_stdout

from aim_sdk import cli

EXISTING = {
    "aimUrl": "https://api.aim.opena2a.org",
    "userEmail": "u@example.com",
    "refreshToken": "rt",
}


def _run_login():
    args = SimpleNamespace(url="https://api.aim.opena2a.org", force=False)
    buf = io.StringIO()
    with redirect_stdout(buf):
        rc = cli.login(args)
    return rc, buf.getvalue()


def test_login_eof_keeps_credentials_no_traceback():
    with patch("aim_sdk.credentials.load_sdk_credentials", return_value=EXISTING):
        with patch("builtins.input", side_effect=EOFError):
            rc, out = _run_login()
    assert rc == 0, out
    assert "keeping existing credentials" in out.lower()
    assert "--force" in out  # actionable next step
    assert "Traceback" not in out


def test_login_ctrl_c_at_prompt_is_clean():
    with patch("aim_sdk.credentials.load_sdk_credentials", return_value=EXISTING):
        with patch("builtins.input", side_effect=KeyboardInterrupt):
            rc, out = _run_login()
    assert rc == 0, out
    assert "Traceback" not in out


if __name__ == "__main__":
    import pytest
    pytest.main([__file__, "-v"])
