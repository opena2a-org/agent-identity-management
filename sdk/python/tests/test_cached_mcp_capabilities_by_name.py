"""
The query the cached-capability lookup sends to
GET /api/v1/sdk-api/agents/{id}/mcp-servers/by-name.

The server's handler reads the MCP server name from the ``name`` query
parameter. This file holds the client to one value per name: a name holding
``&``, ``+``, ``#`` or a space reaches the server as the same name, and the
endpoint the client signs is the one it sends, query included, because the
server verifies the signature over the request URI as it arrives.

Everything runs offline: the session's request function is replaced by one
that records the request and answers a cached capability list.
"""

import base64
from urllib.parse import parse_qsl, urlsplit

import pytest
from nacl.encoding import Base64Encoder
from nacl.signing import SigningKey

from aim_sdk.client import AIMClient, _get_cached_mcp_capabilities

AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
BY_NAME_PATH = f"/api/v1/sdk-api/agents/{AGENT_ID}/mcp-servers/by-name"


class _Cached:
    status_code = 200
    headers = {}
    text = ""

    def json(self):
        return {"hasCachedCapabilities": True, "capabilities": ["read_file"]}

    def raise_for_status(self):
        pass


def _recording_client(**auth):
    client = AIMClient(
        agent_id=AGENT_ID,
        aim_url="https://aim.example.com",
        telemetry={"enabled": False},
        **auth,
    )
    sent = []

    def record(method=None, url=None, **kwargs):
        sent.append((method, url, kwargs.get("headers", {})))
        return _Cached()

    client.session.request = record
    return client, sent


def _name_sent(url):
    parts = urlsplit(url)
    assert parts.path == BY_NAME_PATH
    assert parts.fragment == "", "the name ends no query"
    return parse_qsl(parts.query, keep_blank_values=True, strict_parsing=True)


@pytest.mark.parametrize(
    "name",
    [
        "filesystem",
        "a&b",
        "a#b",
        "a+b",
        "two words",
        "a=b",
        "a?b",
        "café",
        "100%",
    ],
)
def test_the_server_name_is_sent_as_one_value(name):
    """A character that is syntax in a query string is data in a name: it adds
    no parameter, ends no query and is not read as a space."""
    client, sent = _recording_client(api_key="not-a-real-key")

    cached = _get_cached_mcp_capabilities(client, [name])

    assert len(sent) == 1, "one lookup per name"
    method, url, _ = sent[0]
    assert method == "GET"
    assert _name_sent(url) == [("name", name)]
    assert cached == {name: ["read_file"]}


def test_the_signed_endpoint_is_the_encoded_one_that_is_sent():
    signing_key = SigningKey.generate()
    client, sent = _recording_client(
        public_key=signing_key.verify_key.encode(encoder=Base64Encoder).decode(),
        private_key=base64.b64encode(bytes(signing_key)).decode(),
    )

    _get_cached_mcp_capabilities(client, ["a&b c"])

    assert len(sent) == 1
    method, url, headers = sent[0]
    parts = urlsplit(url)
    endpoint = f"{parts.path}?{parts.query}"
    assert _name_sent(url) == [("name", "a&b c")]

    message = "\n".join([method, endpoint, headers["X-Timestamp"]]).encode("utf-8")
    signature = base64.b64decode(headers["X-Signature"])
    # Raises BadSignatureError when the signed endpoint differs from the one
    # sent.
    signing_key.verify_key.verify(message, signature)
