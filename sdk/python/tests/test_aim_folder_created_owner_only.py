"""
~/.aim and ~/.aim/agents are created with mode 0700 by each path in this SDK
that creates them. A folder that exists with group or other permission bits
loses those bits and keeps its owner bits, and one line on stderr names it with
its old and new modes; a folder with no group or other bits keeps its mode
(#860).

Measured under umask 022 before this change: save_sdk_credentials() (which
`aim-sdk login` calls), the lock-file opener, save_agent_credentials() (which
`aim-sdk init` calls), AttestationCache, save_capabilities_config() and
SecureCredentialStorage.save_credentials() each left a new ~/.aim at 0755, and
an existing ~/.aim/agents at 0555 became 0700 with nothing printed.

A folder created with a wider mode and narrowed afterwards would end at 0700
too, so the creation tests also record every os.chmod call and require that
none of them named the folder.
"""

import os
import stat
from types import SimpleNamespace
from unittest import mock

import pytest

from aim_sdk import cli, credentials, secure_storage
from aim_sdk.attestation_cache import AttestationCache
from aim_sdk.capability_detection import save_capabilities_config

pytestmark = pytest.mark.skipif(os.name == "nt", reason="POSIX file modes")

AIM_URL = "http://127.0.0.1:8080"
TOKEN = "aim_ob_FAKE-NOT-A-TOKEN"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
AGENT_KEYS = {"agent_id": "agent-860", "private_key": "FAKE-PRIVATE-KEY", "public_key": "FAKE-PUBLIC-KEY"}
SDK_CREDENTIALS = {"aimUrl": AIM_URL, "refreshToken": "FAKE-REFRESH-860"}


def changed_line(folder, before, after):
    return f"Changed the mode of {folder} from {before} to {after}, removing its group and other permissions."


@pytest.fixture(autouse=True)
def default_umask():
    """The common default umask, under which a plain mkdir() makes a 0755 folder."""
    previous = os.umask(0o022)
    yield
    os.umask(previous)


@pytest.fixture
def aim_dir(isolated_home):
    return isolated_home / ".aim"


@pytest.fixture
def agents_dir(aim_dir):
    return aim_dir / "agents"


@pytest.fixture
def chmod_calls(monkeypatch):
    """The path of every os.chmod call, so a folder narrowed after its creation is seen."""
    calls = []
    real_chmod = os.chmod

    def recording_chmod(path, mode, *args, **kwargs):
        calls.append(os.fspath(path))
        return real_chmod(path, mode, *args, **kwargs)

    monkeypatch.setattr(os, "chmod", recording_chmod)
    return calls


def mode_of(path):
    return stat.S_IMODE(os.stat(path).st_mode)


def mode_lines(text):
    return [line for line in text.splitlines() if line.startswith("Changed the mode of ")]


def existing_folder(path, mode):
    """A folder left by an earlier version or another tool, with `mode`; folders above it are 0700."""
    path.mkdir(parents=True, mode=0o700)
    path.parent.chmod(0o700)
    path.chmod(mode)


# --- each path that creates ~/.aim -------------------------------------------

class PlainCipher:
    """Stands in for the keyring-backed cipher, so no keyring is read."""

    def encrypt(self, data):
        return data


def save_sdk(monkeypatch):
    assert credentials.save_sdk_credentials(dict(SDK_CREDENTIALS)) is True


def take_the_lock(monkeypatch):
    with credentials.sdk_credentials_lock():
        pass


def save_agent(monkeypatch):
    assert credentials.save_agent_credentials("my-agent", dict(AGENT_KEYS)) is True


def check_the_agents_folder(monkeypatch):
    credentials.check_agent_credentials_dir()


def migrate_legacy_agents(monkeypatch):
    credentials._migrate_legacy_agent_credentials({"my-agent": dict(AGENT_KEYS)})


def record_an_attestation(monkeypatch):
    AttestationCache(agent_id="agent-860").record_attestation("mcp-860", "attestation-860", ["read_file"])


def save_capabilities(monkeypatch):
    save_capabilities_config(["read_files"])


def save_secure_storage(monkeypatch):
    monkeypatch.setattr(secure_storage, "CRYPTOGRAPHY_AVAILABLE", True)
    monkeypatch.setattr(secure_storage, "KEYRING_AVAILABLE", True)
    monkeypatch.setattr(secure_storage.SecureCredentialStorage, "_get_cipher", lambda self: PlainCipher())
    secure_storage.SecureCredentialStorage().save_credentials(dict(AGENT_KEYS))


AIM_ONLY = (".aim",)
AIM_AND_AGENTS = (".aim", ".aim/agents")
CREATING_PATHS = [
    pytest.param(save_sdk, AIM_ONLY, id="save_sdk_credentials (aim-sdk login)"),
    pytest.param(take_the_lock, AIM_ONLY, id="sdk_credentials_lock"),
    pytest.param(save_agent, AIM_AND_AGENTS, id="save_agent_credentials (aim-sdk init)"),
    pytest.param(check_the_agents_folder, AIM_AND_AGENTS, id="check_agent_credentials_dir (aim-sdk init)"),
    pytest.param(migrate_legacy_agents, AIM_AND_AGENTS, id="legacy agent migration"),
    pytest.param(record_an_attestation, AIM_ONLY, id="AttestationCache"),
    pytest.param(save_capabilities, AIM_ONLY, id="save_capabilities_config"),
    pytest.param(save_secure_storage, AIM_ONLY, id="SecureCredentialStorage"),
]


@pytest.mark.parametrize("create, folders", CREATING_PATHS)
def test_each_path_creates_the_folders_0700(create, folders, isolated_home, chmod_calls, monkeypatch, capsys):
    assert not (isolated_home / ".aim").exists()

    create(monkeypatch)

    for folder in folders:
        path = isolated_home / folder
        assert oct(mode_of(path)) == oct(0o700), folder
        assert str(path) not in chmod_calls, f"{folder} was created wider and narrowed after"
    assert mode_lines(capsys.readouterr().err) == []


def test_a_folder_the_caller_names_is_created_and_kept_as_before(tmp_path, monkeypatch, capsys):
    # Only ~/.aim is the SDK's own folder: a cache_dir or credentials_path the
    # caller passes is created with the default mode, and an existing one keeps its mode.
    new_cache = tmp_path / "new-cache"
    AttestationCache(agent_id="agent-860", cache_dir=str(new_cache)).record_attestation(
        "mcp-860", "attestation-860", ["read_file"])
    existing_cache = tmp_path / "existing-cache"
    existing_cache.mkdir()
    existing_cache.chmod(0o755)
    AttestationCache(agent_id="agent-860", cache_dir=str(existing_cache)).record_attestation(
        "mcp-860", "attestation-860", ["read_file"])

    monkeypatch.setattr(secure_storage, "CRYPTOGRAPHY_AVAILABLE", True)
    monkeypatch.setattr(secure_storage, "KEYRING_AVAILABLE", True)
    monkeypatch.setattr(secure_storage.SecureCredentialStorage, "_get_cipher", lambda self: PlainCipher())
    storage_dir = tmp_path / "storage"
    secure_storage.SecureCredentialStorage(str(storage_dir / "credentials.json")).save_credentials(dict(AGENT_KEYS))

    assert oct(mode_of(new_cache)) == oct(0o755)
    assert oct(mode_of(existing_cache)) == oct(0o755)
    assert oct(mode_of(storage_dir)) == oct(0o755)
    assert mode_lines(capsys.readouterr().err) == []


# --- an existing ~/.aim ------------------------------------------------------

def test_an_existing_aim_folder_at_0755_becomes_0700_and_one_line_names_it(aim_dir, capsys):
    existing_folder(aim_dir, 0o755)

    assert credentials.save_sdk_credentials(dict(SDK_CREDENTIALS)) is True

    assert oct(mode_of(aim_dir)) == oct(0o700)
    assert mode_lines(capsys.readouterr().err) == [changed_line(aim_dir, "0755", "0700")]


def test_an_existing_aim_folder_at_0500_keeps_its_mode_and_nothing_is_printed(aim_dir, capsys):
    existing_folder(aim_dir, 0o500)
    try:
        saved = credentials.save_sdk_credentials(dict(SDK_CREDENTIALS))
        mode = mode_of(aim_dir)
    finally:
        aim_dir.chmod(0o700)

    assert oct(mode) == oct(0o500)
    assert mode_lines(capsys.readouterr().err) == []
    if os.geteuid() != 0:
        assert saved is False


def test_an_existing_aim_folder_at_0700_keeps_its_mode_and_is_not_chmod_ed(aim_dir, chmod_calls, capsys):
    existing_folder(aim_dir, 0o700)
    chmod_calls.clear()

    assert credentials.save_sdk_credentials(dict(SDK_CREDENTIALS)) is True

    assert oct(mode_of(aim_dir)) == oct(0o700)
    assert str(aim_dir) not in chmod_calls
    assert mode_lines(capsys.readouterr().err) == []


def test_a_regular_file_named_aim_keeps_its_mode(aim_dir, capsys):
    aim_dir.write_text("")
    aim_dir.chmod(0o644)

    assert credentials.save_sdk_credentials(dict(SDK_CREDENTIALS)) is False

    assert oct(mode_of(aim_dir)) == oct(0o644)
    assert mode_lines(capsys.readouterr().err) == []


# --- an existing ~/.aim/agents ----------------------------------------------

def test_an_existing_agents_folder_at_0755_becomes_0700_and_one_line_names_it(agents_dir, capsys):
    existing_folder(agents_dir, 0o755)

    assert credentials.save_agent_credentials("my-agent", dict(AGENT_KEYS)) is True

    assert oct(mode_of(agents_dir)) == oct(0o700)
    assert mode_lines(capsys.readouterr().err) == [changed_line(agents_dir, "0755", "0700")]


def test_both_folders_at_0755_become_0700_with_one_line_each_aim_first(aim_dir, agents_dir, capsys):
    existing_folder(agents_dir, 0o755)
    aim_dir.chmod(0o755)

    assert credentials.save_agent_credentials("my-agent", dict(AGENT_KEYS)) is True

    assert oct(mode_of(aim_dir)) == oct(0o700)
    assert oct(mode_of(agents_dir)) == oct(0o700)
    assert mode_lines(capsys.readouterr().err) == [
        changed_line(aim_dir, "0755", "0700"),
        changed_line(agents_dir, "0755", "0700"),
    ]


def test_an_agents_folder_at_0555_loses_only_its_group_and_other_bits(agents_dir, capsys):
    # The owner bits stay r-x: nothing is added, so the key file cannot be written.
    existing_folder(agents_dir, 0o555)
    try:
        saved = credentials.save_agent_credentials("my-agent", dict(AGENT_KEYS))
        mode = mode_of(agents_dir)
    finally:
        agents_dir.chmod(0o700)

    assert oct(mode) == oct(0o500)
    assert mode_lines(capsys.readouterr().err) == [changed_line(agents_dir, "0555", "0500")]
    if os.geteuid() != 0:
        assert saved is False


def test_an_agents_folder_at_0500_keeps_its_mode_and_nothing_is_printed(agents_dir, capsys):
    existing_folder(agents_dir, 0o500)
    try:
        saved = credentials.save_agent_credentials("my-agent", dict(AGENT_KEYS))
        mode = mode_of(agents_dir)
    finally:
        agents_dir.chmod(0o700)

    assert oct(mode) == oct(0o500)
    assert mode_lines(capsys.readouterr().err) == []
    if os.geteuid() != 0:
        assert saved is False


# --- the CLIs ----------------------------------------------------------------

@pytest.fixture
def exchanges(monkeypatch):
    """Stands in for the bootstrap-token exchange; records each call, which spends the token."""
    calls = []

    def exchange(aim_url, token, name, public_key):
        calls.append(name)
        return {"agentId": AGENT_ID, "name": name, "status": "pending", "publicKey": public_key}

    monkeypatch.setattr(cli, "exchange_bootstrap_token", exchange)
    return calls


def run_init():
    with mock.patch.dict(os.environ, {cli.BOOTSTRAP_TOKEN_ENV: TOKEN}):
        return cli.init(SimpleNamespace(token=None, url=AIM_URL, name="my-agent"))


def test_init_in_a_home_without_aim_creates_both_folders_0700_and_prints_no_mode_change(
        aim_dir, agents_dir, exchanges, capsys):
    assert run_init() == 0

    assert exchanges == ["my-agent"]
    assert oct(mode_of(aim_dir)) == oct(0o700)
    assert oct(mode_of(agents_dir)) == oct(0o700)
    out = capsys.readouterr()
    assert mode_lines(out.out + out.err) == []


def test_init_narrows_an_agents_folder_at_0755_and_prints_one_line(agents_dir, exchanges, capsys):
    existing_folder(agents_dir, 0o755)

    assert run_init() == 0

    assert oct(mode_of(agents_dir)) == oct(0o700)
    assert (agents_dir / "my-agent.json").exists()
    out = capsys.readouterr()
    assert mode_lines(out.err) == [changed_line(agents_dir, "0755", "0700")]
    assert mode_lines(out.out) == []


@pytest.mark.skipif(hasattr(os, "geteuid") and os.geteuid() == 0, reason="root writes to a 0500 folder")
@pytest.mark.parametrize("mode, now", [(0o500, 0o500), (0o555, 0o500)])
def test_init_refuses_an_agents_folder_its_owner_cannot_write_before_the_token_is_spent(
        agents_dir, exchanges, capsys, mode, now):
    existing_folder(agents_dir, mode)
    try:
        code = run_init()
        left = mode_of(agents_dir)
    finally:
        agents_dir.chmod(0o700)

    assert code == 1
    assert exchanges == []
    assert oct(left) == oct(now)
    assert list(agents_dir.iterdir()) == []
    out = capsys.readouterr()
    assert (f"Error: cannot save the agent's key file in {agents_dir} (EACCES); "
            f"the token has not been used.") in out.out.splitlines()
    expected = [] if mode == now else [changed_line(agents_dir, f"{mode:04o}", f"{now:04o}")]
    assert mode_lines(out.err) == expected


def test_login_narrows_an_aim_folder_at_0755_and_prints_one_line(aim_dir, monkeypatch, capsys):
    existing_folder(aim_dir, 0o755)
    device = {"deviceCode": "fake-device-code", "userCode": "BCDF-GHJK",
              "verificationUri": "http://127.0.0.1:3000/device", "expiresIn": 900, "interval": 5}
    monkeypatch.setattr(cli, "_probe_failure", lambda url, timeout: None)
    monkeypatch.setattr(cli, "request_device_code", lambda url: dict(device))
    monkeypatch.setattr(cli, "poll_device_token", lambda url, code, interval, deadline: {
        "accessToken": "FAKE-ACCESS-860", "refreshToken": "FAKE-REFRESH-860"})
    monkeypatch.setattr("aim_sdk.oauth.decode_jwt_claims", lambda token: {"email": "dev@example.com"})

    args = SimpleNamespace(url=AIM_URL, force=True, no_browser=True, allow_http=False)
    assert cli.login(args) == 0

    assert oct(mode_of(aim_dir)) == oct(0o700)
    assert (aim_dir / "sdk_credentials.json").exists()
    out = capsys.readouterr()
    assert mode_lines(out.err) == [changed_line(aim_dir, "0755", "0700")]
    assert mode_lines(out.out) == []
