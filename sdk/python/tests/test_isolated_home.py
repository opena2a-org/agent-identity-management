"""
Every test runs against an empty temporary home (#726).

aim_sdk.credentials computes its paths from Path.home() at import time, and
load_sdk_credentials() also adopts SDK credentials from a `.aim` folder above
the working directory. A checkout inside a home directory that holds
`aim-sdk login` credentials therefore handed that sign-in to the code under
test. The isolated_home fixture in conftest.py closes both routes.
"""
import json
import os
from pathlib import Path

from aim_sdk import credentials, grant_client


def _inside(path, root) -> bool:
    path, root = Path(path).resolve(), Path(root).resolve()
    return path == root or root in path.parents


def test_home_is_an_empty_directory_under_the_pytest_base_temp(tmp_path_factory):
    home = Path.home()
    assert _inside(home, tmp_path_factory.getbasetemp()), home
    assert list(home.iterdir()) == []
    assert os.environ["USERPROFILE"] == os.environ["HOME"] == str(home)


def test_paths_computed_at_import_time_point_into_the_temporary_home(tmp_path_factory):
    home = Path.home()
    assert _inside(home, tmp_path_factory.getbasetemp()), home
    for path in (
        credentials.AIM_DIR,
        credentials.SDK_CREDENTIALS_FILE,
        credentials.AGENTS_DIR,
        credentials.LEGACY_CREDENTIALS_FILE,
        grant_client.DEFAULT_SOCKET_PATH,
        grant_client.DEFAULT_TOKEN_PATH,
    ):
        assert _inside(path, home), path


def test_credentials_in_a_folder_above_the_working_directory_are_not_adopted(
    tmp_path, tmp_path_factory, monkeypatch
):
    # Checked first: without the fixture this test stops here, before
    # load_sdk_credentials() can read or write a real home directory.
    assert _inside(credentials.SDK_CREDENTIALS_FILE, tmp_path_factory.getbasetemp())

    outer = tmp_path / "outer"
    (outer / ".aim").mkdir(parents=True)
    (outer / ".aim" / "sdk_credentials.json").write_text(json.dumps({
        "aimUrl": "http://127.0.0.1:9",
        "refreshToken": "FAKE-REFRESH-726",
        "type": "sdk_oauth",
    }))
    project = outer / "project"
    project.mkdir()
    monkeypatch.chdir(project)

    assert credentials.load_sdk_credentials() is None
    assert not credentials.SDK_CREDENTIALS_FILE.exists()
