"""
The namespace routes read the namespace ID from the request path. These tests
hold that SecretsClient refuses an ID that is not a UUID before a request is
made, and sends a UUID as one path segment.
"""

import base64
import threading
import uuid
from http.server import BaseHTTPRequestHandler, HTTPServer
from unittest.mock import patch

import pytest
from nacl.signing import SigningKey

from aim_sdk.client import AIMClient
from aim_sdk.exceptions import SecretsError


NAMESPACE_ID = "0f8fad5b-d9cb-469f-a165-70867728950e"

REFUSAL = (
    "namespace_id is not a UUID; the request was not sent. Pass the id the server "
    "answered for the namespace, a UUID such as the id of an entry list_namespaces() "
    "returns."
)

# Each method that puts a namespace ID in the path, with the request it makes.
CALLS = [
    ("get_namespace", lambda secrets, id_: secrets.get_namespace(id_),
     "GET", "/api/v1/secrets/namespaces/{}"),
    ("delete_namespace", lambda secrets, id_: secrets.delete_namespace(id_),
     "DELETE", "/api/v1/secrets/namespaces/{}"),
    ("store_credential", lambda secrets, id_: secrets.store_credential(id_, b"\x01"),
     "POST", "/api/v1/secrets/namespaces/{}/credentials"),
    ("rotate_credential", lambda secrets, id_: secrets.rotate_credential(id_, b"\x01"),
     "POST", "/api/v1/secrets/namespaces/{}/rotate"),
]

# Not a UUID in the form the server answers. Sent as given, ".." climbs out of
# the namespaces route, "/" adds a segment, and "?" and "#" end the path.
NOT_UUIDS = [
    f"../../agents/{NAMESPACE_ID}",
    f"{NAMESPACE_ID}/../../../agents",
    "a/b",
    f"{NAMESPACE_ID}/",
    f"{NAMESPACE_ID}?agentId={NAMESPACE_ID}",
    f"{NAMESPACE_ID}#x",
    f"{NAMESPACE_ID}\n",
    f" {NAMESPACE_ID}",
    f"{{{NAMESPACE_ID}}}",
    f"urn:uuid:{NAMESPACE_ID}",
    NAMESPACE_ID.replace("-", ""),
    NAMESPACE_ID[:-1] + "g",
    NAMESPACE_ID.replace("0", "０"),
    "%2e%2e",
    "ns-123",
    "",
    None,
    42,
    NAMESPACE_ID.encode(),
]


def make_client(aim_url="http://127.0.0.1:8080"):
    """An AIMClient with a new Ed25519 key."""
    signing_key = SigningKey.generate()
    return AIMClient(
        agent_id="test-agent-id",
        public_key=base64.b64encode(bytes(signing_key.verify_key)).decode("utf-8"),
        private_key=base64.b64encode(bytes(signing_key._signing_key)).decode("utf-8"),
        aim_url=aim_url,
    )


@pytest.mark.parametrize("name,call,method,path", CALLS, ids=[c[0] for c in CALLS])
@pytest.mark.parametrize("namespace_id", NOT_UUIDS, ids=repr)
def test_an_id_that_is_not_a_uuid_is_refused_and_nothing_is_sent(name, call, method, path, namespace_id):
    client = make_client()

    with patch.object(client, "_make_request", return_value={}) as mock_req:
        with pytest.raises(SecretsError) as refused:
            call(client.secrets, namespace_id)

    assert str(refused.value) == REFUSAL
    mock_req.assert_not_called()


@pytest.mark.parametrize("name,call,method,path", CALLS, ids=[c[0] for c in CALLS])
@pytest.mark.parametrize("namespace_id,segment", [
    (NAMESPACE_ID, NAMESPACE_ID),
    (NAMESPACE_ID.upper(), NAMESPACE_ID.upper()),
    (uuid.UUID(NAMESPACE_ID), NAMESPACE_ID),
], ids=["lower case", "upper case", "uuid.UUID"])
def test_a_uuid_is_sent_as_the_namespace_segment(name, call, method, path, namespace_id, segment):
    client = make_client()

    with patch.object(client, "_make_request", return_value={}) as mock_req:
        call(client.secrets, namespace_id)

    mock_req.assert_called_once()
    assert mock_req.call_args.args == (method, path.format(segment))


class _RecordingHandler(BaseHTTPRequestHandler):
    """Answers every request with {} and records its method and path."""

    def _answer(self):
        self.server.received.append(f"{self.command} {self.path}")
        length = int(self.headers.get("Content-Length") or 0)
        if length:
            self.rfile.read(length)
        body = b"{}"
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    do_GET = do_POST = do_DELETE = _answer

    def log_message(self, *args):
        pass


@pytest.fixture
def loopback_server():
    server = HTTPServer(("127.0.0.1", 0), _RecordingHandler)
    server.received = []
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    try:
        yield server
    finally:
        server.shutdown()
        server.server_close()
        thread.join()


def test_a_server_on_loopback_receives_no_request_for_an_id_that_climbs_out_of_the_route(loopback_server):
    client = make_client(f"http://127.0.0.1:{loopback_server.server_address[1]}")

    for name, call, _, _ in CALLS:
        with pytest.raises(SecretsError):
            call(client.secrets, f"../../agents/{NAMESPACE_ID}")
    assert loopback_server.received == []

    for _, call, _, _ in CALLS:
        call(client.secrets, NAMESPACE_ID)
    assert loopback_server.received == [
        f"{method} {path.format(NAMESPACE_ID)}" for _, _, method, path in CALLS
    ]
