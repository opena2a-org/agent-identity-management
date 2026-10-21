"""
A patch a test puts on aim_sdk.credentials ends with that test (#828).

aim_sdk.oauth copies load_sdk_credentials and save_sdk_credentials from
aim_sdk.credentials when it is first imported, and cli.login() is one place
where that first import runs. tests/test_perform_action_authorization_claims.py
patches both names to stand-ins around cli.login(). Run before the quick-start
refresh test in a session where nothing had imported aim_sdk.oauth yet, it left
the stand-ins in aim_sdk.oauth, and both cases of the refresh test got None
from OAuthTokenManager().get_access_token(). The isolated_home fixture in
conftest.py now imports aim_sdk.oauth before a test patches anything.

In this process aim_sdk.oauth is already imported, so the test runs the two in
that order in a new interpreter: pytest, started in an empty temporary home
with PYTHONPATH set to the SDK directory.
"""
import os
import subprocess
import sys
from pathlib import Path

TESTS_DIR = Path(__file__).resolve().parent
SDK_ROOT = TESTS_DIR.parent
CLAIMS_TESTS = TESTS_DIR / "test_perform_action_authorization_claims.py"
QUICK_START_TESTS = TESTS_DIR / "test_readme_quick_start_runs_as_written.py"
REFRESH_TEST = "test_first_refresh_after_login_prints_the_rotation_line_only_when_the_server_rotates"


def test_quick_start_refresh_test_passes_after_the_login_claims_tests(tmp_path):
    home = tmp_path / "home"
    home.mkdir()
    env = {k: v for k, v in os.environ.items() if not k.startswith("AIM_")}
    python_path = [str(SDK_ROOT)] + ([env["PYTHONPATH"]] if env.get("PYTHONPATH") else [])
    env.update({
        "HOME": str(home),
        "USERPROFILE": str(home),
        "PYTHONPATH": os.pathsep.join(python_path),
    })
    proc = subprocess.run(
        [
            sys.executable, "-m", "pytest", "-p", "no:cacheprovider",
            f"--basetemp={tmp_path / 'basetemp'}",
            str(CLAIMS_TESTS), f"{QUICK_START_TESTS}::{REFRESH_TEST}",
        ],
        cwd=home, env=env, capture_output=True, text=True, timeout=300,
    )
    assert proc.returncode == 0, proc.stdout[-3000:] + proc.stderr[-1000:]
    for case in ("server-with-redis", "server-without-redis"):
        assert f"{REFRESH_TEST}[{case}] PASSED" in proc.stdout, proc.stdout[-3000:]
