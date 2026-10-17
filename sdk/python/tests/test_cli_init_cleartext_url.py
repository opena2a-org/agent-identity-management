"""
`aim-sdk init` sends the bootstrap token over `http:` only to this machine.

An `http:` server address is accepted when its host is `localhost`,
`127.0.0.1` or `::1`, decided from the URL text and never by resolving it. Any
other `http:` address ends the command with exit 1 before the exchange
request: nothing is sent, nothing is saved and the token is not printed.
`https:` addresses are unchanged. An address with a backslash, where requests
ends the host and urlsplit does not, or with whitespace or a control
character, ends the command the same way on any scheme.

`requests.post` is replaced with a recorder, so no request reaches the network
and the recorder holds every request the command made.
"""
from types import SimpleNamespace

import pytest
import requests

from aim_sdk import cli
from aim_sdk import credentials as creds_module

TOKEN = "".join([cli.BOOTSTRAP_TOKEN_PREFIX, "Pl41nUrl", "q" * 30, "_-z"])
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
ORG_ID = "660e8400-e29b-41d4-a716-446655440000"
EXCHANGE_PATH = "/api/v1/onboarding/bootstrap-tokens/exchange"


class RecordedAnswer:
    """The 201 answer for a successful exchange, echoing the sent key."""

    status_code = 201

    def __init__(self, sent):
        self.headers = {}
        self._sent = sent

    def json(self):
        return {
            "agentId": AGENT_ID,
            "organizationId": ORG_ID,
            "name": self._sent.get("name"),
            "status": "pending",
            "publicKey": self._sent.get("publicKey"),
        }


@pytest.fixture
def recorder(monkeypatch):
    calls = []

    def post(url, json=None, headers=None, **kwargs):
        calls.append({"url": url, "headers": dict(headers or {})})
        return RecordedAnswer(json or {})

    monkeypatch.setattr(requests, "post", post)
    return calls


@pytest.fixture
def agents_dir(tmp_path, monkeypatch):
    directory = tmp_path / ".aim" / "agents"
    monkeypatch.setattr(creds_module, "AGENTS_DIR", directory)
    monkeypatch.delenv(cli.BOOTSTRAP_TOKEN_ENV, raising=False)
    return directory


def run_init(url, token=None, name=cli.DEFAULT_INIT_AGENT_NAME):
    return cli.init(SimpleNamespace(token=token, url=url, name=name))


def saved_files(agents_dir):
    return sorted(p.name for p in agents_dir.glob("*")) if agents_dir.exists() else []


def requests_made(recorder):
    """The recorded requests, with whether each carried the token instead of its value."""
    return [(c["url"], c["headers"].get(cli.BOOTSTRAP_TOKEN_HEADER) == TOKEN) for c in recorder]


def assert_nothing_sent_or_saved(recorder, agents_dir, out):
    assert requests_made(recorder) == []
    assert saved_files(agents_dir) == []
    for text in (out.out, out.err):
        assert TOKEN not in text
        assert TOKEN[len(cli.BOOTSTRAP_TOKEN_PREFIX):] not in text


# --- an http: address whose host is not this machine is refused --------------

def test_plain_http_to_another_host_exits_1_with_zero_requests(recorder, agents_dir, monkeypatch, capsys):
    monkeypatch.setenv(cli.BOOTSTRAP_TOKEN_ENV, TOKEN)

    code = run_init("http://aim.example.test")

    out = capsys.readouterr()
    assert_nothing_sent_or_saved(recorder, agents_dir, out)
    assert code == 1
    assert "Registering agent" not in out.out


def test_the_refusal_names_the_host_and_prints_the_https_form_of_the_command(recorder, agents_dir, capsys):
    assert run_init("http://aim.example.test:8080/api", token=TOKEN) == 1

    out = capsys.readouterr()
    assert_nothing_sent_or_saved(recorder, agents_dir, out)
    assert "aim.example.test" in out.out
    assert "http:" in out.out
    assert "nothing was sent or saved" in out.out.lower()
    assert "aim-sdk init --url https://aim.example.test" in out.out


@pytest.mark.parametrize("url", [
    "http://localhost.example.test",
    "http://127.0.0.1.example.test",
    "http://10.0.0.5:8080",
    "http://[2001:db8::1]:8080",
])
def test_a_host_that_only_resembles_or_is_not_a_loopback_host_is_refused(recorder, agents_dir, capsys, url):
    assert run_init(url, token=TOKEN) == 1

    assert_nothing_sent_or_saved(recorder, agents_dir, capsys.readouterr())


def test_the_refusal_names_the_host_without_the_address_user_part(recorder, agents_dir, capsys):
    assert run_init("http://FAKEUSER:FAKEPASS@aim.example.test", token=TOKEN) == 1

    out = capsys.readouterr()
    assert_nothing_sent_or_saved(recorder, agents_dir, out)
    assert "aim.example.test" in out.out
    assert "FAKEPASS" not in out.out
    assert "FAKEUSER" not in out.out


def test_a_host_that_holds_the_token_is_shown_redacted(recorder, agents_dir, capsys):
    host_token = cli.BOOTSTRAP_TOKEN_PREFIX + "h0stlabel"
    assert run_init(f"http://{host_token}.example.test", token=host_token) == 1

    out = capsys.readouterr()
    assert [c["url"] for c in recorder] == []
    assert host_token not in out.out and host_token not in out.err
    assert f"{cli.BOOTSTRAP_TOKEN_PREFIX}[REDACTED]" in out.out


# --- an address urlsplit and requests could read different hosts from --------

@pytest.mark.parametrize("url", [
    "http://aim.example.test\\@127.0.0.1",
    "http://aim.example.test\\@localhost",
    "http://aim.example.test:8080\\@[::1]",
    "https://aim.example.test\\@127.0.0.1",
])
def test_an_address_with_a_backslash_is_refused_with_zero_requests(recorder, agents_dir, capsys, url):
    # urlsplit reads the host after the "@"; requests ends it at the backslash
    # and would send the token to aim.example.test.
    assert run_init(url, token=TOKEN) == 1

    out = capsys.readouterr()
    assert_nothing_sent_or_saved(recorder, agents_dir, out)
    assert "Error: --url has a backslash." in out.out
    assert "aim.example.test" not in out.out
    assert "Registering agent" not in out.out


@pytest.mark.parametrize("url", [
    "http://local\thost:8080",
    "http://local\nhost:8080",
    "http://local\rhost:8080",
    "http://localhost :8080",
    "http://local\x00host:8080",
    "http://local\x1bhost:8080",
    "http://local\x7fhost:8080",
    "http://local\x85host:8080",
    "http://local host:8080",
])
def test_an_address_with_whitespace_or_a_control_character_is_refused_with_zero_requests(
        recorder, agents_dir, capsys, url):
    assert run_init(url, token=TOKEN) == 1

    out = capsys.readouterr()
    assert_nothing_sent_or_saved(recorder, agents_dir, out)
    assert "Error: --url has whitespace or a control character." in out.out
    assert "Registering agent" not in out.out


def test_whitespace_around_the_address_is_still_trimmed(recorder, agents_dir, capsys):
    assert run_init(" http://127.0.0.1:8080\n", token=TOKEN) == 0

    assert [c["url"] for c in recorder] == ["http://127.0.0.1:8080" + EXCHANGE_PATH]


def test_the_shared_address_check_still_accepts_http_on_any_host():
    # `aim-sdk login` uses the same check; its behaviour is not changed here.
    assert cli.invalid_server_url_reason("http://aim.example.test") is None


# --- http: on a loopback host and https: on any host reach the exchange -------

@pytest.mark.parametrize("url", [
    "http://127.0.0.1:8080",
    "http://localhost:8080",
    "http://[::1]:8080",
    "http://LOCALHOST:8080",
    "https://aim.example.test",
])
def test_loopback_http_and_https_send_one_request_with_the_token_header(recorder, agents_dir, capsys, url):
    assert run_init(url, token=TOKEN) == 0

    assert len(recorder) == 1
    assert recorder[0]["url"].endswith(EXCHANGE_PATH)
    assert recorder[0]["headers"][cli.BOOTSTRAP_TOKEN_HEADER] == TOKEN
    assert saved_files(agents_dir) == ["my-first-agent.json"]
    out = capsys.readouterr()
    assert TOKEN not in out.out and TOKEN not in out.err
