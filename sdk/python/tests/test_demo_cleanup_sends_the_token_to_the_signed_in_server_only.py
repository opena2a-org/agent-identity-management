"""
`aim-sdk demo --cleanup` sends the stored sign-in's access token to the server
that sign-in is for, and to no other server. OAuthTokenManager obtains and
refreshes that token at the sign-in's aimUrl, so a --url naming another server
would carry a token one server issued to a server that did not issue it. Such
a --url is refused before any request, with the sign-in to run first, and
nothing is deleted. A --url naming the sign-in's server, with or without a
trailing "/", and no --url, clean up as before.

Before this change the demo sent its lookup, GET /api/v1/sdk-api/agents/demo-agent
with the stored sign-in's bearer token, to the server --url named.
"""
import json
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest import mock

import pytest

from aim_sdk import demo as demo_mod

TOKEN = "FAKE-ACCESS-TOKEN"


class Recorder:
    """A loopback server that records the method, path and headers of each request and answers 404."""

    def __init__(self):
        recorder = self

        class Handler(BaseHTTPRequestHandler):
            def _record(self):
                recorder.requests.append({
                    "method": self.command,
                    "path": self.path,
                    "headers": {k.lower(): v for k, v in self.headers.items()},
                })
                data = json.dumps({"error": "not found"}).encode()
                self.send_response(404)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(data)))
                self.end_headers()
                self.wfile.write(data)

            do_GET = do_POST = do_DELETE = _record

            def log_message(self, *args):
                pass

        self.requests = []
        self.server = HTTPServer(("127.0.0.1", 0), Handler)
        self.url = f"http://127.0.0.1:{self.server.server_port}"
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()

    def close(self):
        self.server.shutdown()
        self.server.server_close()


@pytest.fixture
def servers():
    """servers() starts a Recorder; each is closed after the test."""
    started = []

    def start():
        server = Recorder()
        started.append(server)
        return server

    yield start
    for server in started:
        server.close()


def sign_in_for(server):
    return {"aimUrl": server.url, "refreshToken": "rt", "accessToken": "at"}


def cleanup(creds, url=None):
    """Runs `aim-sdk demo --cleanup [--url url]` with creds stored and the token manager answering TOKEN."""
    with mock.patch("aim_sdk.credentials.load_sdk_credentials", return_value=creds), \
         mock.patch("aim_sdk.oauth.OAuthTokenManager") as manager, \
         mock.patch("aim_sdk.credentials.delete_agent_credentials", return_value=False) as delete_creds:
        manager.return_value.get_access_token.return_value = TOKEN
        rc = demo_mod.run(cleanup=True, url=url)
    return rc, manager, delete_creds


def test_a_url_naming_another_server_is_refused_before_any_request(servers, capsys):
    signed_in, other = servers(), servers()

    rc, manager, delete_creds = cleanup(sign_in_for(signed_in), url=other.url)

    assert rc == 1
    assert capsys.readouterr().out.splitlines() == [
        f"The sign-in stored on this machine is for another server, so its token is not sent to {other.url}.",
        f"To clean up there, sign in to it first: aim-sdk login --force --url {other.url}",
    ]
    assert other.requests == []
    assert signed_in.requests == []
    manager.return_value.get_access_token.assert_not_called()
    delete_creds.assert_not_called()


def test_the_refusal_does_not_print_a_stored_address_with_a_user_name_or_password(servers, capsys):
    other = servers()
    stored = {"aimUrl": "http://FAKE-user:FAKE-password@127.0.0.1:1", "refreshToken": "rt", "accessToken": "at"}

    rc, _, _ = cleanup(stored, url=other.url)

    out = capsys.readouterr().out
    assert rc == 1
    assert "FAKE-user" not in out and "FAKE-password" not in out
    assert f"aim-sdk login --force --url {other.url}" in out
    assert other.requests == []


@pytest.mark.parametrize("url_for", [lambda url: None, lambda url: url, lambda url: url + "/"],
                         ids=["no-url", "as-stored", "trailing-slash"])
def test_the_signed_in_server_is_asked_with_the_token(servers, capsys, url_for):
    signed_in = servers()

    rc, _, _ = cleanup(sign_in_for(signed_in), url=url_for(signed_in.url))

    assert rc == 0
    assert "No demo agent found on the server. Nothing to clean up." in capsys.readouterr().out
    assert [(r["method"], r["path"]) for r in signed_in.requests] == [
        ("GET", f"/api/v1/sdk-api/agents/{demo_mod.DEMO_AGENT_NAME}"),
    ]
    assert signed_in.requests[0]["headers"].get("authorization") == f"Bearer {TOKEN}"
