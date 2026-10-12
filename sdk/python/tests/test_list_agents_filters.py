"""
The query AIMClient.list_agents sends to GET /api/v1/agents.

The server's list handler reads ``limit``, ``offset``, ``status`` and the
agent type as ``agentType`` or ``agent_type``, and applies the two filters in
its query (TestListAgents_StatusAndAgentTypeNoAgentHoldsMatchNoAgent in the
backend's handlers package sends the query this client builds). This file
holds the client to that query: the names the handler reads, and each value
written as one value, so a status holding ``&`` or ``=`` stays a status.

Everything runs offline: the session's request function is replaced by one
that records the URL and answers a list.
"""

from urllib.parse import parse_qsl, urlsplit

import pytest

from aim_sdk.client import AIMClient

AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"


class _Listed:
    status_code = 200
    headers = {}
    text = ""

    def json(self):
        return {"agents": [], "total": 0, "limit": 50, "offset": 0}

    def raise_for_status(self):
        pass


@pytest.fixture
def listing_client():
    client = AIMClient(
        agent_id=AGENT_ID,
        api_key="test-key",
        aim_url="https://aim.example.com",
        telemetry={"enabled": False},
    )
    urls = []

    def record(method=None, url=None, **kwargs):
        urls.append((method, url))
        return _Listed()

    client.session.request = record
    return client, urls


def _query(urls):
    assert len(urls) == 1, "list_agents sends one request"
    method, url = urls[0]
    assert method == "GET"
    parts = urlsplit(url)
    assert parts.path == "/api/v1/agents"
    return parse_qsl(parts.query, keep_blank_values=True, strict_parsing=True)


def test_filters_are_sent_under_the_names_the_server_reads(listing_client):
    client, urls = listing_client

    client.list_agents(limit=10, offset=5, status="verified", agent_type="langchain")

    assert _query(urls) == [
        ("limit", "10"),
        ("offset", "5"),
        ("status", "verified"),
        ("agent_type", "langchain"),
    ]


def test_without_filters_only_the_page_is_sent(listing_client):
    client, urls = listing_client

    client.list_agents()

    assert _query(urls) == [("limit", "50"), ("offset", "0")]


def test_the_limit_is_capped_at_one_hundred(listing_client):
    client, urls = listing_client

    client.list_agents(limit=500)

    assert _query(urls) == [("limit", "100"), ("offset", "0")]


@pytest.mark.parametrize(
    "value",
    [
        "a&limit=1",
        "a=b",
        "a#fragment",
        "a+b",
        "two words",
        "a?b",
        "café",
        "100%",
    ],
)
def test_a_filter_value_is_written_as_one_value(listing_client, value):
    """A character that is syntax in a query string is data in a value: it
    adds no parameter, ends no query and changes no other value."""
    client, urls = listing_client

    client.list_agents(status=value, agent_type=value)

    assert _query(urls) == [
        ("limit", "50"),
        ("offset", "0"),
        ("status", value),
        ("agent_type", value),
    ]
