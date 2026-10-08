"""
`aim-sdk init` registers a first agent with the bootstrap token minted on the
dashboard's onboarding screen: it generates the Ed25519 key pair locally,
posts only the public key to POST /api/v1/onboarding/bootstrap-tokens/exchange
with the token in the X-AIM-Bootstrap-Token header, saves the credentials to
~/.aim/agents/<name>.json with 0600 permissions and prints the dashboard URL.

The fake backend is a real HTTP server on loopback that answers in the shapes
bootstrap_token_handler.go emits, so the tests see the exact bytes the CLI
puts on the wire. Every test asserts the token is never printed.
"""
import base64
import json
import logging
import os
import stat
import sys
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from types import SimpleNamespace

import pytest

from aim_sdk import cli
from aim_sdk import credentials as creds_module

TOKEN = "aim_ob_" + "Xk3q9TfA" + "b" * 35
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
ORG_ID = "660e8400-e29b-41d4-a716-446655440000"
EXCHANGE_PATH = "/api/v1/onboarding/bootstrap-tokens/exchange"


class FakeAIM:
    """A loopback server that records each request and answers from a script."""

    def __init__(self, answer):
        self.answer = answer
        self.requests = []
        fake = self

        class Handler(BaseHTTPRequestHandler):
            def do_POST(self):
                length = int(self.headers.get("Content-Length") or 0)
                body = self.rfile.read(length).decode() if length else ""
                fake.requests.append({
                    "path": self.path,
                    "headers": {k.lower(): v for k, v in self.headers.items()},
                    "body": json.loads(body) if body else None,
                })
                status, payload, extra = fake.answer(fake.requests[-1])
                data = json.dumps(payload).encode()
                self.send_response(status)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                for key, value in (extra or {}).items():
                    self.send_header(key, value)
                self.end_headers()
                self.wfile.write(data)

            def log_message(self, *args):
                pass

        self.server = HTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self.server.server_port}"
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()


def registered(request):
    """The 201 answer for a successful exchange, echoing the sent key."""
    body = request["body"] or {}
    return 201, {
        "agentId": AGENT_ID,
        "organizationId": ORG_ID,
        "name": body.get("name"),
        "displayName": body.get("name"),
        "status": "pending",
        "publicKey": body.get("publicKey"),
        "aimUrl": "http://internal:8080",
    }, None


@pytest.fixture
def fake_server():
    servers = []

    def start(answer=registered):
        server = FakeAIM(answer)
        servers.append(server)
        return server

    yield start
    for server in servers:
        server.close()


@pytest.fixture
def agents_dir(tmp_path, monkeypatch):
    directory = tmp_path / ".aim" / "agents"
    monkeypatch.setattr(creds_module, "AGENTS_DIR", directory)
    monkeypatch.delenv(cli.BOOTSTRAP_TOKEN_ENV, raising=False)
    return directory


def run_init(url, token=None, name=cli.DEFAULT_INIT_AGENT_NAME):
    return cli.init(SimpleNamespace(token=token, url=url, name=name))


def assert_token_absent(*texts):
    for text in texts:
        assert TOKEN not in text
        assert TOKEN[len("aim_ob_"):] not in text


# --- success ------------------------------------------------------------------

def test_env_token_registers_the_agent_and_saves_a_local_key(fake_server, agents_dir, monkeypatch, capsys):
    server = fake_server()
    monkeypatch.setenv(cli.BOOTSTRAP_TOKEN_ENV, TOKEN)

    assert run_init(server.url) == 0

    assert len(server.requests) == 1
    sent = server.requests[0]
    assert sent["path"] == EXCHANGE_PATH
    assert sent["headers"]["x-aim-bootstrap-token"] == TOKEN
    assert sent["body"]["name"] == "my-first-agent"
    assert sent["body"]["agentType"] == "custom"
    assert "bootstrapToken" not in sent["body"]
    assert "privateKey" not in sent["body"] and "private_key" not in sent["body"]
    assert len(base64.b64decode(sent["body"]["publicKey"])) == 32

    saved_file = agents_dir / "my-first-agent.json"
    saved = json.loads(saved_file.read_text())
    assert saved["agent_id"] == AGENT_ID
    assert saved["organization_id"] == ORG_ID
    assert saved["aim_url"] == server.url
    assert saved["public_key"] == sent["body"]["publicKey"]
    private = base64.b64decode(saved["private_key"])
    assert len(private) == 64
    assert private[32:] == base64.b64decode(saved["public_key"])

    out = capsys.readouterr()
    assert f"{server.url}/dashboard/agents/{AGENT_ID}" in out.out
    assert str(saved_file) in out.out
    assert_token_absent(out.out, out.err, saved_file.read_text())


def test_saved_private_key_signs_for_the_registered_public_key(fake_server, agents_dir, monkeypatch):
    nacl = pytest.importorskip("nacl")
    if not getattr(nacl, "__file__", None):
        pytest.skip("the offline nacl stand-in does no real cryptography")
    from nacl.signing import SigningKey
    server = fake_server()
    monkeypatch.setenv(cli.BOOTSTRAP_TOKEN_ENV, TOKEN)

    assert run_init(server.url) == 0

    saved = json.loads((agents_dir / "my-first-agent.json").read_text())
    signing_key = SigningKey(base64.b64decode(saved["private_key"])[:32])
    assert signing_key.verify_key.encode() == base64.b64decode(saved["public_key"])


def test_credentials_file_is_owner_only(fake_server, agents_dir, monkeypatch):
    server = fake_server()
    monkeypatch.setenv(cli.BOOTSTRAP_TOKEN_ENV, TOKEN)

    assert run_init(server.url) == 0

    mode = stat.S_IMODE(os.stat(agents_dir / "my-first-agent.json").st_mode)
    assert mode == 0o600


def test_credentials_file_is_created_owner_only_not_chmod_ed_after(agents_dir, monkeypatch):
    # With the chmod disabled, only the mode passed at creation protects the
    # private key, which is the window between open() and chmod() closed.
    monkeypatch.setattr(creds_module.os, "chmod", lambda *a, **k: None)
    old_umask = os.umask(0o022)
    try:
        assert creds_module.save_agent_credentials("a", {"agent_id": AGENT_ID, "private_key": "k"})
    finally:
        os.umask(old_umask)

    mode = stat.S_IMODE(os.stat(agents_dir / "a.json").st_mode)
    assert mode == 0o600


def test_token_flag_and_name_flag(fake_server, agents_dir, monkeypatch, capsys):
    server = fake_server()
    monkeypatch.setenv(cli.BOOTSTRAP_TOKEN_ENV, "aim_ob_from-env")

    assert run_init(server.url, token=TOKEN, name="billing-bot") == 0

    assert server.requests[0]["headers"]["x-aim-bootstrap-token"] == TOKEN
    assert server.requests[0]["body"]["name"] == "billing-bot"
    assert (agents_dir / "billing-bot.json").exists()
    out = capsys.readouterr()
    assert_token_absent(out.out, out.err)


def test_trailing_slash_url_is_normalised(fake_server, agents_dir, monkeypatch):
    server = fake_server()
    monkeypatch.setenv(cli.BOOTSTRAP_TOKEN_ENV, TOKEN)

    assert run_init(server.url + "/") == 0
    assert server.requests[0]["path"] == EXCHANGE_PATH


def test_token_is_absent_from_every_log_record(fake_server, agents_dir, monkeypatch, caplog):
    server = fake_server()
    monkeypatch.setenv(cli.BOOTSTRAP_TOKEN_ENV, TOKEN)

    with caplog.at_level(logging.DEBUG):
        assert run_init(server.url) == 0

    assert caplog.records, "expected the HTTP client or credential logger to log something"
    assert_token_absent(caplog.text)


# --- refusals before any request -----------------------------------------------

def test_missing_token_sends_nothing(fake_server, agents_dir, capsys):
    server = fake_server()

    assert run_init(server.url) == 1

    assert server.requests == []
    assert cli.BOOTSTRAP_TOKEN_ENV in capsys.readouterr().out


def test_value_that_is_not_a_bootstrap_token_is_refused_and_not_echoed(fake_server, agents_dir, capsys):
    server = fake_server()
    # Assembled at run time, so the tracked file carries no sk- shaped literal.
    other_secret = "sk-" + "not-a-bootstrap-token-0123456789"

    assert run_init(server.url, token=other_secret) == 1

    assert server.requests == []
    out = capsys.readouterr()
    assert other_secret not in out.out and other_secret not in out.err


@pytest.mark.parametrize("url", ["", "localhost:8080", "ftp://aim.example.com"])
def test_invalid_url_sends_nothing(agents_dir, capsys, url):
    assert run_init(url, token=TOKEN) == 1
    out = capsys.readouterr()
    assert "--url" in out.out
    assert_token_absent(out.out, out.err)


def test_existing_credentials_are_kept_and_the_token_is_not_spent(fake_server, agents_dir, capsys):
    server = fake_server()
    agents_dir.mkdir(parents=True)
    existing = agents_dir / "my-first-agent.json"
    existing.write_text('{"agent_id": "earlier", "private_key": "earlier-key"}')

    assert run_init(server.url, token=TOKEN) == 1

    assert server.requests == []
    assert json.loads(existing.read_text())["private_key"] == "earlier-key"
    assert "--name" in capsys.readouterr().out


# --- server refusals -----------------------------------------------------------

@pytest.mark.parametrize("code,message", [
    ("bootstrap_token_used", "The bootstrap token has already registered an agent. Generate a new one from the onboarding screen."),
    ("bootstrap_token_expired", "The bootstrap token has expired. Generate a new one from the onboarding screen."),
])
def test_refused_token_prints_the_server_reason_and_saves_nothing(fake_server, agents_dir, capsys, code, message):
    server = fake_server(lambda r: (401, {"error": message, "code": code}, None))

    assert run_init(server.url, token=TOKEN) == 1

    assert not (agents_dir / "my-first-agent.json").exists()
    out = capsys.readouterr()
    assert message in out.out
    assert_token_absent(out.out, out.err)


def test_name_conflict_says_the_token_is_still_usable(fake_server, agents_dir, capsys):
    server = fake_server(lambda r: (409, {"error": "agent with name my-first-agent already exists"}, None))

    assert run_init(server.url, token=TOKEN) == 1

    assert not (agents_dir / "my-first-agent.json").exists()
    out = capsys.readouterr().out
    assert "already exists" in out
    assert "--name" in out


def test_server_reason_that_echoes_the_token_is_redacted(fake_server, agents_dir, capsys):
    server = fake_server(lambda r: (400, {"error": f"bad token {TOKEN}"}, None))

    assert run_init(server.url, token=TOKEN) == 1

    out = capsys.readouterr()
    assert "aim_ob_[REDACTED]" in out.out
    assert_token_absent(out.out, out.err)


def test_redirect_is_not_followed_so_the_token_does_not_reach_another_host(fake_server, agents_dir, capsys):
    elsewhere = fake_server()
    server = fake_server(lambda r: (307, {}, {"Location": elsewhere.url + EXCHANGE_PATH}))

    assert run_init(server.url, token=TOKEN) == 1

    assert len(server.requests) == 1
    assert elsewhere.requests == []
    assert not (agents_dir / "my-first-agent.json").exists()
    assert "redirect" in capsys.readouterr().out


def test_server_that_swaps_the_public_key_is_refused(fake_server, agents_dir, capsys):
    def swapped(request):
        status, body, extra = registered(request)
        body["publicKey"] = base64.b64encode(b"\x01" * 32).decode()
        return status, body, extra

    server = fake_server(swapped)

    assert run_init(server.url, token=TOKEN) == 1

    assert not (agents_dir / "my-first-agent.json").exists()
    assert "public key" in capsys.readouterr().out


def test_unreachable_server_does_not_print_the_token(agents_dir, capsys):
    import socket

    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        port = s.getsockname()[1]

    assert run_init(f"http://127.0.0.1:{port}", token=TOKEN) == 1

    out = capsys.readouterr()
    assert "Could not register the agent" in out.out
    assert_token_absent(out.out, out.err)


# --- wiring --------------------------------------------------------------------

def test_main_routes_init_with_defaults(monkeypatch):
    seen = {}
    monkeypatch.setattr(cli, "init", lambda args: seen.update(vars(args)) or 0)
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "init"])
    assert cli.main() == 0
    assert seen["name"] == "my-first-agent"
    assert seen["url"] == cli.DEFAULT_AIM_URL
    assert seen["token"] is None


def test_help_documents_the_environment_variable(monkeypatch, capsys):
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "init", "--help"])
    with pytest.raises(SystemExit) as exc:
        cli.main()
    assert exc.value.code == 0
    out = capsys.readouterr().out
    assert cli.BOOTSTRAP_TOKEN_ENV in out
    assert "--url" in out and "--name" in out
