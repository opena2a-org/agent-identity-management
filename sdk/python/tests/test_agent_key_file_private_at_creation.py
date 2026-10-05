"""
The agents directory is created with mode 0700 and each agent key file with
mode 0600 at creation, so the private key is never on disk under the process's
default mode.

The modes are read while the key is being written, through the open file
descriptor, before any chmod that runs after the write could narrow them.
"""

import json
import os
import stat
from unittest.mock import patch

import pytest

from aim_sdk import credentials
from aim_sdk.credentials import (
    list_agent_credentials,
    load_agent_credentials,
    save_agent_credentials,
)

pytestmark = pytest.mark.skipif(os.name == "nt", reason="POSIX file modes")

AGENT_KEYS = {
    "agent_id": "agent-123",
    "private_key": "base64-private-key",
    "public_key": "base64-public-key",
}


def _mode(path) -> int:
    return stat.S_IMODE(os.stat(path).st_mode)


@pytest.fixture
def default_umask():
    """The common default umask, under which a plain open() makes a 0644 file."""
    previous = os.umask(0o022)
    yield
    os.umask(previous)


@pytest.fixture
def agents_dir(tmp_path, default_umask):
    path = tmp_path / ".aim" / "agents"
    with patch.object(credentials, "AGENTS_DIR", path):
        yield path


@pytest.fixture
def modes_at_write(agents_dir):
    """Record the file and directory modes at the moment the key JSON is written."""
    seen = []
    real_dump = json.dump

    def recording_dump(obj, fp, *args, **kwargs):
        seen.append({
            "file": stat.S_IMODE(os.fstat(fp.fileno()).st_mode),
            "dir": _mode(agents_dir),
        })
        return real_dump(obj, fp, *args, **kwargs)

    with patch.object(credentials.json, "dump", recording_dump):
        yield seen


def _temp_files(directory):
    return [p.name for p in directory.iterdir() if p.name.endswith(".tmp")]


def test_agent_key_file_is_0600_while_the_key_is_written(agents_dir, modes_at_write):
    assert save_agent_credentials("my-agent", dict(AGENT_KEYS)) is True

    assert len(modes_at_write) == 1
    assert oct(modes_at_write[0]["file"]) == oct(0o600)
    assert oct(_mode(agents_dir / "my-agent.json")) == oct(0o600)


def test_agents_dir_is_created_0700(agents_dir, modes_at_write):
    assert not agents_dir.exists()

    assert save_agent_credentials("my-agent", dict(AGENT_KEYS)) is True

    assert oct(modes_at_write[0]["dir"]) == oct(0o700)
    assert oct(_mode(agents_dir)) == oct(0o700)


def test_dir_and_file_left_wide_by_an_earlier_version_are_narrowed(agents_dir):
    agents_dir.mkdir(parents=True, mode=0o755)
    os.chmod(agents_dir, 0o755)
    old_file = agents_dir / "my-agent.json"
    old_file.write_text(json.dumps({"agent_id": "old"}))
    os.chmod(old_file, 0o644)

    assert save_agent_credentials("my-agent", dict(AGENT_KEYS)) is True

    assert oct(_mode(agents_dir)) == oct(0o700)
    assert oct(_mode(old_file)) == oct(0o600)
    assert json.loads(old_file.read_text())["agent_id"] == "agent-123"


def test_saved_key_loads_back_and_no_temp_file_is_left(agents_dir):
    assert save_agent_credentials("my-agent", dict(AGENT_KEYS)) is True

    loaded = load_agent_credentials("my-agent")
    assert loaded["private_key"] == "base64-private-key"
    assert [a["name"] for a in list_agent_credentials()] == ["my-agent"]
    assert _temp_files(agents_dir) == []


def test_failed_write_keeps_the_previous_file_and_leaves_no_temp_file(agents_dir, capsys):
    assert save_agent_credentials("my-agent", dict(AGENT_KEYS)) is True
    before = (agents_dir / "my-agent.json").read_text()

    unserialisable = dict(AGENT_KEYS, private_key=object())
    assert save_agent_credentials("my-agent", unserialisable) is False

    assert (agents_dir / "my-agent.json").read_text() == before
    assert _temp_files(agents_dir) == []
    assert "Failed to save agent credentials" in capsys.readouterr().err
