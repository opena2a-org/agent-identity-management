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
from unittest import mock

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
    """`aim-sdk init`, with `token` in AIM_BOOTSTRAP_TOKEN, the one place it reads the token from."""
    env = {} if token is None else {cli.BOOTSTRAP_TOKEN_ENV: token}
    with mock.patch.dict(os.environ, env):
        return cli.init(SimpleNamespace(token=None, url=url, name=name))


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


def test_name_flag_names_the_agent(fake_server, agents_dir, monkeypatch, capsys):
    server = fake_server()
    monkeypatch.setenv(cli.BOOTSTRAP_TOKEN_ENV, TOKEN)

    assert run_init(server.url, name="billing-bot") == 0

    assert server.requests[0]["headers"]["x-aim-bootstrap-token"] == TOKEN
    assert server.requests[0]["body"]["name"] == "billing-bot"
    assert (agents_dir / "billing-bot.json").exists()
    out = capsys.readouterr()
    assert_token_absent(out.out, out.err)


@pytest.mark.parametrize("flag", [["--token", TOKEN], ["=".join(["--token", TOKEN])], ["--tok", TOKEN]],
                         ids=["separate-value", "equals-form", "abbreviation"])
def test_token_flag_is_refused_before_any_request_and_never_printed(fake_server, agents_dir, monkeypatch, capsys,
                                                                    flag):
    # As `npx @opena2a/aim-sdk init` does: a flag value is visible in the process
    # list and shell history, so the flag is refused even with the variable set.
    server = fake_server()
    monkeypatch.setenv(cli.BOOTSTRAP_TOKEN_ENV, TOKEN)
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "init", *flag, "--url", server.url])

    assert cli.main() == 1

    assert server.requests == []
    assert saved_files(agents_dir) == []
    out = capsys.readouterr()
    assert out.out.splitlines()[0] == (
        "Error: the bootstrap token is read from AIM_BOOTSTRAP_TOKEN, not from a flag "
        "(a flag value is visible in the process list and shell history).")
    assert f"  {cli.BOOTSTRAP_TOKEN_ENV}=<token> aim-sdk init" in out.out.splitlines()
    assert_token_absent(out.out, out.err)


def test_help_does_not_offer_a_token_flag(monkeypatch, capsys):
    monkeypatch.setattr(sys, "argv", ["aim-sdk", "init", "--help"])
    with pytest.raises(SystemExit):
        cli.main()
    out = capsys.readouterr().out
    assert "--token" not in out
    assert "read from AIM_BOOTSTRAP_TOKEN only" in " ".join(out.split())


@pytest.mark.parametrize("name", ["../../evil-py", "agents/evil", "..\\evil", "/etc/evil", ".", ".."])
def test_a_name_that_is_a_path_is_refused_before_the_token_is_sent(fake_server, agents_dir, capsys, name):
    server = fake_server()

    assert run_init(server.url, token=TOKEN, name=name) == 1

    assert server.requests == []
    assert saved_files(agents_dir) == []
    out = capsys.readouterr()
    assert "Error: an agent name cannot hold '/' or '\\', or be '.' or '..'. Nothing was sent." in out.out
    assert "Registering agent" not in out.out
    assert_token_absent(out.out, out.err)


@pytest.mark.parametrize("name, saved_as", [("ops agent", "ops_agent.json"), ("bot.v2", "bot_v2.json"),
                                            ("...x", "___x.json")])
def test_a_name_with_spaces_or_dots_that_is_not_a_path_is_registered(fake_server, agents_dir, capsys, name, saved_as):
    server = fake_server()

    assert run_init(server.url, token=TOKEN, name=name) == 0

    assert server.requests[0]["body"]["name"] == name
    assert saved_files(agents_dir) == [saved_as]


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


def test_a_closed_port_says_the_token_has_not_been_used(agents_dir, capsys):
    import socket

    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        port = s.getsockname()[1]

    assert run_init(f"http://127.0.0.1:{port}", token=TOKEN, name="py-agent") == 1

    lines = capsys.readouterr().out.splitlines()
    assert lines[-2].startswith(
        f"Could not register the agent: Could not reach AIM at 127.0.0.1:{port}: connection refused (ConnectionError).")
    assert lines[-1] == "The request was not sent, so the token has not been used."
    assert saved_files(agents_dir) == []


def test_a_connection_closed_without_an_answer_says_the_token_may_have_been_used(agents_dir, capsys):
    received = []

    class ClosesWithoutAnswer(BaseHTTPRequestHandler):
        def do_POST(self):
            self.rfile.read(int(self.headers.get("Content-Length") or 0))
            received.append(self.headers.get(cli.BOOTSTRAP_TOKEN_HEADER))
            # No status line: the client reads a connection closed after its request.
            self.close_connection = True

        def log_message(self, *args):
            pass

    server = HTTPServer(("127.0.0.1", 0), ClosesWithoutAnswer)
    thread = threading.Thread(target=server.handle_request, daemon=True)
    thread.start()
    try:
        assert run_init(f"http://127.0.0.1:{server.server_port}", token=TOKEN) == 1
    finally:
        thread.join(5)
        server.server_close()

    assert received == [TOKEN]
    out = capsys.readouterr()
    assert out.out.splitlines()[-1] == (
        "The server may have received the request before it failed, so the token may have been used. "
        "If the agent is listed in the dashboard, delete it, then run aim-sdk init again with a new token.")
    assert "The request was not sent" not in out.out
    assert saved_files(agents_dir) == []
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


# --- the exchange answer: fields are checked, server text is one plain line ------

ESC = "\x1b"
RLO = "\u202e"
SECOND_LINE = "SECOND LINE: text chosen by the server"


def answering(**fields):
    """A 201 that echoes the sent public key, with `fields` over the usual answer."""
    def answer(request):
        status, body, extra = registered(request)
        body.update(fields)
        return status, body, extra
    return answer


def saved_files(agents_dir):
    return sorted(p.name for p in agents_dir.iterdir()) if agents_dir.exists() else []


def test_answer_agent_id_with_an_escape_sequence_and_a_newline_is_refused(fake_server, agents_dir, capsys):
    server = fake_server(answering(agentId=f"agt_9{ESC}[31mRED{ESC}[0m\n{SECOND_LINE}"))

    assert run_init(server.url, token=TOKEN) == 1

    assert len(server.requests) == 1
    assert saved_files(agents_dir) == []
    out = capsys.readouterr()
    assert ESC not in out.out + out.err
    assert SECOND_LINE not in (out.out + out.err).splitlines()
    assert "agent ID this tool cannot use" in out.out
    assert "Nothing was saved" in out.out
    assert "delete it in the dashboard" in out.out


def test_answer_agent_id_with_a_right_to_left_override_is_refused(fake_server, agents_dir, capsys):
    server = fake_server(answering(agentId=f"agt_{RLO}9"))

    assert run_init(server.url, token=TOKEN) == 1

    assert saved_files(agents_dir) == []
    out = capsys.readouterr()
    assert RLO not in out.out + out.err


@pytest.mark.parametrize("agent_id", [["agt_1"], {"id": "agt_1"}, 7], ids=["list", "object", "number"])
def test_answer_agent_id_that_is_not_a_string_is_refused(fake_server, agents_dir, capsys, agent_id):
    server = fake_server(answering(agentId=agent_id))

    assert run_init(server.url, token=TOKEN) == 1

    assert saved_files(agents_dir) == []
    assert "Nothing was saved" in capsys.readouterr().out


def test_answer_status_with_an_escape_sequence_is_saved_and_printed_as_unknown(fake_server, agents_dir, capsys):
    server = fake_server(answering(status=f"active{ESC}[2K"))

    assert run_init(server.url, token=TOKEN) == 0

    saved = json.loads((agents_dir / "my-first-agent.json").read_text())
    assert saved["status"] == "unknown"
    out = capsys.readouterr()
    assert "   Status:      unknown" in out.out.splitlines()
    assert ESC not in out.out + out.err


def test_answer_organization_id_with_an_escape_sequence_is_saved_as_null(fake_server, agents_dir):
    server = fake_server(answering(organizationId=f"org{ESC}[31m"))

    assert run_init(server.url, token=TOKEN) == 0

    saved = json.loads((agents_dir / "my-first-agent.json").read_text())
    assert saved["organization_id"] is None


def test_refusal_reason_with_control_and_bidi_characters_is_printed_as_one_plain_line(fake_server, agents_dir, capsys):
    reason = f"{ESC}[31mserver text{ESC}[0m\n{SECOND_LINE}{RLO}"
    server = fake_server(lambda r: (400, {"error": reason}, None))

    assert run_init(server.url, token=TOKEN) == 1

    out = capsys.readouterr()
    printed = out.out + out.err
    assert ESC not in printed
    assert RLO not in printed
    assert SECOND_LINE not in printed.splitlines()
    refusal = [line for line in printed.splitlines() if "Could not register the agent" in line]
    assert len(refusal) == 1
    assert "server text" in refusal[0] and SECOND_LINE in refusal[0]


def test_redirect_location_with_an_escape_sequence_is_printed_without_it(fake_server, agents_dir, capsys):
    server = fake_server(lambda r: (302, {}, {"Location": f"https://elsewhere.example.test/{ESC}[31mx"}))

    assert run_init(server.url, token=TOKEN) == 1

    out = capsys.readouterr()
    assert ESC not in out.out + out.err
    assert "redirect" in out.out
    assert saved_files(agents_dir) == []


def test_printed_name_is_the_local_name_not_the_one_in_the_answer(fake_server, agents_dir, capsys):
    server = fake_server(answering(name=f"other{ESC}[31m"))

    assert run_init(server.url, token=TOKEN, name="billing-bot") == 0

    out = capsys.readouterr()
    assert "   Name:        billing-bot" in out.out.splitlines()
    assert ESC not in out.out + out.err


def test_answer_with_a_uuid_agent_id_and_status_pending_is_saved_and_printed(fake_server, agents_dir, capsys):
    server = fake_server(answering(agentId=AGENT_ID, organizationId=ORG_ID, status="pending"))

    assert run_init(server.url, token=TOKEN) == 0

    saved = json.loads((agents_dir / "my-first-agent.json").read_text())
    assert saved["agent_id"] == AGENT_ID
    assert saved["organization_id"] == ORG_ID
    assert saved["status"] == "pending"
    lines = capsys.readouterr().out.splitlines()
    assert f"   Agent ID:    {AGENT_ID}" in lines
    assert "   Status:      pending" in lines
    assert f"   Dashboard:   {server.url}/dashboard/agents/{AGENT_ID}" in lines


def test_terminal_line_replaces_control_and_bidi_characters_and_caps_the_length():
    replaced = ["\x00", "\t", "\n", "\r", ESC, "\x7f", "\x85", "\x9b", "\u061c", "\u200e", "\u200f",
                "\u202a", "\u202b", "\u202c", "\u202d", RLO, "\u2066", "\u2067", "\u2068", "\u2069", "\ud800"]
    for ch in replaced:
        assert cli._terminal_line(f"a{ch}b") == "a�b", repr(ch)
    kept = "HTTP 400: café, שלום, 名前 (x/y?z#1%2.3:4) \U0001F916"
    assert cli._terminal_line(kept) == kept
    cap = cli.TERMINAL_LINE_MAX_CHARS
    assert cli._terminal_line("x" * cap) == "x" * cap
    assert cli._terminal_line("x" * (cap + 1)) == "x" * cap + " [truncated]"


# --- the dashboard link: the address the server reports, else one derived from --url --

SELF_HOSTED_API = "https://aim-api.example.com"


def dashboard_lines(out):
    return [line for line in out.splitlines() if line.startswith("   Dashboard:")]


def exchanged_with(**fields):
    """An exchange_bootstrap_token stand-in: the 201 answer's fields, with `fields` over them."""
    def exchange(aim_url, token, name, public_key):
        answer = {"agentId": AGENT_ID, "organizationId": ORG_ID, "name": name, "displayName": name,
                  "status": "pending", "publicKey": public_key, "aimUrl": aim_url}
        answer.update(fields)
        return answer
    return exchange


def test_a_self_hosted_api_on_its_own_host_links_to_the_dashboard_the_server_reports(agents_dir, monkeypatch,
                                                                                    capsys):
    monkeypatch.setattr(cli, "exchange_bootstrap_token", exchanged_with(dashboardUrl="https://aim.example.com"))

    assert run_init(SELF_HOSTED_API, token=TOKEN) == 0

    out = capsys.readouterr().out
    assert dashboard_lines(out) == [f"   Dashboard:   https://aim.example.com/dashboard/agents/{AGENT_ID}"]
    assert f"{SELF_HOSTED_API}/dashboard" not in out


@pytest.mark.parametrize("reported", ["https://aim.example.com/", " https://aim.example.com\t"],
                         ids=["trailing-slash", "surrounding-whitespace"])
def test_the_reported_address_is_printed_without_a_trailing_slash_or_surrounding_whitespace(
        fake_server, agents_dir, capsys, reported):
    server = fake_server(answering(dashboardUrl=reported))

    assert run_init(server.url, token=TOKEN) == 0

    out = capsys.readouterr().out
    assert dashboard_lines(out) == [f"   Dashboard:   https://aim.example.com/dashboard/agents/{AGENT_ID}"]


def test_a_dashboard_served_under_a_path_keeps_the_path(fake_server, agents_dir, capsys):
    server = fake_server(answering(dashboardUrl="https://example.com/aim/"))

    assert run_init(server.url, token=TOKEN) == 0

    out = capsys.readouterr().out
    assert dashboard_lines(out) == [f"   Dashboard:   https://example.com/aim/dashboard/agents/{AGENT_ID}"]


def test_a_reported_address_as_long_as_the_line_cap_is_printed(fake_server, agents_dir, capsys):
    base = "https://aim.example.com/"
    reported = base + "a" * (cli.TERMINAL_LINE_MAX_CHARS - len(base))
    server = fake_server(answering(dashboardUrl=reported))

    assert run_init(server.url, token=TOKEN) == 0

    out = capsys.readouterr().out
    assert len(reported) == cli.TERMINAL_LINE_MAX_CHARS
    assert dashboard_lines(out) == [f"   Dashboard:   {reported}/dashboard/agents/{AGENT_ID}"]


def test_a_loopback_address_from_a_server_on_this_machine_is_printed(fake_server, agents_dir, capsys):
    server = fake_server(answering(dashboardUrl="http://localhost:3000"))

    assert run_init(server.url, token=TOKEN) == 0

    out = capsys.readouterr().out
    assert dashboard_lines(out) == [f"   Dashboard:   http://localhost:3000/dashboard/agents/{AGENT_ID}"]


@pytest.mark.parametrize("reported", ["http://localhost:3000", "http://127.0.0.1:3000/", "http://[::1]:3000"])
def test_a_loopback_address_from_a_server_on_another_host_is_not_printed(agents_dir, monkeypatch, capsys,
                                                                          reported):
    monkeypatch.setattr(cli, "exchange_bootstrap_token", exchanged_with(dashboardUrl=reported))

    assert run_init(SELF_HOSTED_API, token=TOKEN) == 0

    out = capsys.readouterr().out
    assert dashboard_lines(out) == [f"   Dashboard:   {SELF_HOSTED_API}/dashboard/agents/{AGENT_ID}"]


@pytest.mark.parametrize("reported", [
    None,
    7,
    ["https://aim.example.com"],
    {"url": "https://aim.example.com"},
    "",
    "   ",
    "aim.example.com",
    "javascript:alert(1)",
    "ftp://aim.example.com",
    "https://",
    "https://someone@aim.example.com",
    "https://aim.example.com/?next=x",
    "https://aim.example.com/#top",
    "https://aim.example.com:http",
    "https://aim.example.com:99999",
    "https://aim.example.com/a b",
    "https://aim.example.com\\@elsewhere.example.test",
    f"https://aim.example.com/{ESC}[31m",
    f"https://aim.example.com/x\n{SECOND_LINE}",
    f"https://aim.example.com/{RLO}x",
    "https://aim.example.com/\ud800x",
    "https://aim.example.com/" + "a" * cli.TERMINAL_LINE_MAX_CHARS,
], ids=["null", "number", "array", "object", "empty", "spaces", "no-scheme", "javascript", "ftp", "no-host",
        "user-part", "query", "fragment", "port-not-a-number", "port-out-of-range", "space-in-path", "backslash",
        "escape-sequence", "newline", "right-to-left-override", "lone-surrogate", "over-the-line-cap"])
def test_a_reported_value_init_does_not_print_falls_back_to_the_address_derived_from_url(
        fake_server, agents_dir, capsys, reported):
    server = fake_server(answering(dashboardUrl=reported))

    assert run_init(server.url, token=TOKEN) == 0

    out = capsys.readouterr()
    assert dashboard_lines(out.out) == [f"   Dashboard:   {server.url}/dashboard/agents/{AGENT_ID}"]
    printed = out.out + out.err
    assert ESC not in printed and RLO not in printed
    assert SECOND_LINE not in printed
