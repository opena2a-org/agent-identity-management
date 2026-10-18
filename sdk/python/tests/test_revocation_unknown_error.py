"""
The answer AIM gives when it cannot read the revocation state of the ATC a
request presented: 503, a JSON body whose `error` is `atc_revocation_unknown`,
and `Retry-After`. The client raises RevocationUnknownError for it, a member of
the did-not-decide family (VerificationUnavailableError), at the first such
answer and without a wait of its own.

Measured before this file, against the loopback stub below with the server's
answer (`Retry-After: 5`): the client sent the request 4 times, slept 1, 2 and
4 seconds between them, and raised VerificationError ("Request failed: 503
Server Error: Service Unavailable for url: ..."), which a handler for
VerificationUnavailableError does not catch.
"""
import base64
import json
import pickle
import threading
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

import pytest
from nacl.signing import SigningKey

from aim_sdk import (
    AIMClient,
    AIMError,
    ActionDeniedError,
    AuthenticationError,
    RevocationUnknownError,
    VerificationError,
    VerificationUnavailableError,
)

RESOLVE_PATH = "/api/v1/secrets/resolve"
# The server's answer, as docs/specs/atc-verification-v1.md section 4.2 shows it.
REVOCATION_UNKNOWN = {"error": "atc_revocation_unknown", "message": "the revocation state of this ATC could not be read"}


class _Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length") or 0))
        aim = self.server.aim
        aim.hits.append(f"POST {self.path}")
        status, body, headers = aim.answer
        raw = body if isinstance(body, bytes) else json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json" if not isinstance(body, bytes) else "text/html")
        self.send_header("Content-Length", str(len(raw)))
        for name, value in headers.items():
            self.send_header(name, value)
        self.end_headers()
        self.wfile.write(raw)


class Aim:
    """A loopback AIM server that gives every request the same answer."""

    def __init__(self):
        self.answer = (503, REVOCATION_UNKNOWN, {"Retry-After": "5"})
        self.hits = []
        self._server = ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
        self._server.aim = self
        self.url = f"http://127.0.0.1:{self._server.server_address[1]}"
        threading.Thread(target=self._server.serve_forever, kwargs={"poll_interval": 0.01}, daemon=True).start()

    def stop(self):
        self._server.shutdown()
        self._server.server_close()


@pytest.fixture
def aim():
    a = Aim()
    yield a
    a.stop()


@pytest.fixture
def sleeps(monkeypatch):
    slept = []
    monkeypatch.setattr("aim_sdk.client.time.sleep", slept.append)
    return slept


def _client(aim, **kwargs):
    """An AIMClient with an Ed25519 key made for this test, which secrets.resolve signs with."""
    key = SigningKey.generate()
    return AIMClient(
        agent_id="test-agent-id",
        public_key=base64.b64encode(bytes(key.verify_key)).decode(),
        private_key=base64.b64encode(bytes(key)).decode(),
        aim_url=aim.url,
        telemetry={"enabled": False},
        **kwargs,
    )


def _resolve_error(aim, **kwargs):
    with pytest.raises(AIMError) as raised:
        _client(aim, **kwargs).secrets.resolve("github-creds", "read")
    return raised.value


def test_the_answer_raises_revocation_unknown_at_the_first_answer_with_its_wait(aim, sleeps):
    err = _resolve_error(aim)
    assert type(err) is RevocationUnknownError
    assert aim.hits == [f"POST {RESOLVE_PATH}"], "the request is sent once"
    assert sleeps == [], "the client waits for nothing; the wait is the caller's"
    assert (err.code, err.status, err.retry_after) == ("atc_revocation_unknown", 503, 5)
    assert (err.revocation_state, err.reason) == (None, None)
    assert str(err) == (
        "AIM did not decide: it could not read the revocation state of the ATC (HTTP 503, "
        "atc_revocation_unknown). The ATC was neither accepted nor judged revoked. Send the request "
        "again in 5 seconds with the same ATC while it is valid; a new ATC does not help."
    )


@pytest.mark.parametrize("auto_retry", [True, False])
def test_the_request_is_sent_once_whatever_auto_retry_is(aim, sleeps, auto_retry):
    err = _resolve_error(aim, auto_retry=auto_retry)
    assert type(err) is RevocationUnknownError
    assert len(aim.hits) == 1
    assert sleeps == []


def test_it_is_of_the_did_not_decide_family_and_never_a_denial(aim, sleeps):
    err = _resolve_error(aim)
    assert isinstance(err, VerificationUnavailableError)
    assert not isinstance(err, (VerificationError, ActionDeniedError, PermissionError, AuthenticationError))
    assert err.decision is None


def test_a_handler_for_the_family_catches_it_after_the_handler_for_it(aim, sleeps):
    client = _client(aim)
    caught = []
    for _ in range(2):
        try:
            client.secrets.resolve("github-creds", "read")
        except RevocationUnknownError as e:
            caught.append(("revocation unknown", e.retry_after))
        except VerificationUnavailableError:
            caught.append("no decision")
    try:
        client.secrets.resolve("github-creds", "read")
    except VerificationUnavailableError as e:
        caught.append(("no decision", type(e).__name__))
    assert caught == [("revocation unknown", 5), ("revocation unknown", 5), ("no decision", "RevocationUnknownError")]
    assert len(aim.hits) == 3
    assert sleeps == []


@pytest.mark.parametrize("extra,state,reason,tail", [
    ({"revocationState": "unavailable", "reason": "noList"}, "unavailable", "noList",
     " Revocation state: unavailable, reason noList."),
    ({"revocationState": "unavailable"}, "unavailable", None, " Revocation state: unavailable."),
    ({"reason": "timeout"}, None, "timeout", " Reason: timeout."),
    ({"revocationState": "\x1b[2J", "reason": "x" * 33}, None, None, "a new ATC does not help."),
    ({"revocationState": 5, "reason": ["noList"]}, None, None, "a new ATC does not help."),
    ({"revocationState": "un available", "reason": ""}, None, None, "a new ATC does not help."),
], ids=["state-and-reason", "state-only", "reason-only", "not-a-token", "not-a-string", "space-or-empty"])
def test_revocation_state_and_reason_are_the_bodys_when_they_are_short_tokens(aim, sleeps, extra, state, reason, tail):
    aim.answer = (503, {**REVOCATION_UNKNOWN, **extra}, {"Retry-After": "5"})
    err = _resolve_error(aim)
    assert type(err) is RevocationUnknownError
    assert (err.revocation_state, err.reason) == (state, reason)
    assert str(err).endswith(tail)
    assert "\x1b" not in str(err)


@pytest.mark.parametrize("header,retry_after,phrase", [
    ("5", 5, "again in 5 seconds with"),
    ("1", 1, "again in 1 second with"),
    ("0", 0, "again now with"),
    (" 30 ", 30, "again in 30 seconds with"),
    ("-3", None, "again later with"),
    ("1.5", None, "again later with"),
    ("Wed, 21 Oct 2026 07:28:00 GMT", None, "again later with"),
    ("9" * 10, None, "again later with"),
    (None, None, "again later with"),
], ids=["5", "1", "0", "padded", "negative", "fraction", "date", "ten-digits", "absent"])
def test_retry_after_is_whole_seconds_or_none_and_never_a_default(aim, sleeps, header, retry_after, phrase):
    aim.answer = (503, REVOCATION_UNKNOWN, {} if header is None else {"Retry-After": header})
    err = _resolve_error(aim)
    assert type(err) is RevocationUnknownError
    assert err.retry_after == retry_after
    assert phrase in str(err)
    assert sleeps == []


@pytest.mark.parametrize("status,body", [
    (503, {"error": "Nonce check unavailable; retry the request with a new nonce"}),
    (503, b"<html><body>upstream</body></html>"),
    (500, REVOCATION_UNKNOWN),
    (502, REVOCATION_UNKNOWN),
], ids=["another-503", "html-503", "500-naming-the-code", "502-naming-the-code"])
def test_any_other_server_error_keeps_the_retry_and_verification_error(aim, sleeps, status, body):
    aim.answer = (status, body, {"Retry-After": "5"})
    err = _resolve_error(aim)
    assert type(err) is VerificationError
    assert len(aim.hits) == 4
    assert sleeps == [1, 2, 4]


def test_a_401_naming_an_atc_code_is_still_an_authentication_error(aim, sleeps):
    aim.answer = (401, {"error": "atc_revoked", "message": "ATC is revoked"}, {})
    err = _resolve_error(aim)
    assert type(err) is AuthenticationError
    assert len(aim.hits) == 1


def test_it_survives_a_pickle_round_trip():
    err = RevocationUnknownError(retry_after=5, revocation_state="unavailable", reason="noList")
    copy = pickle.loads(pickle.dumps(err))
    assert type(copy) is RevocationUnknownError
    assert (copy.retry_after, copy.revocation_state, copy.reason) == (5, "unavailable", "noList")
    assert str(copy) == str(err)


@pytest.mark.parametrize("retry_after", [-1, True, "5", 1.5])
def test_a_retry_after_that_is_not_whole_seconds_is_none(retry_after):
    err = RevocationUnknownError(retry_after=retry_after)
    assert err.retry_after is None
    assert "again later with" in str(err)
