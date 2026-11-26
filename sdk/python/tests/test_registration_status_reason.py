"""
What the shortest registration call prints: the status the server answers
with, and, for an agent the registration left pending, the reason the server
gives in statusReason.

The server verifies an agent at registration in monitoring mode, the mode a
new organization starts in, when it holds its own key and its trust score is
at least 0.3 (apps/backend/internal/application/agent_service.go,
autoVerifyRefusal). Otherwise it answers pending with the reason, which the
SDK prints under the status line.
"""

import json
import re
from unittest.mock import patch

import pytest
import responses

from aim_sdk import console as console_module
from aim_sdk.client import register_agent

AIM_URL = "http://aim.test"
AGENTS_URL = f"{AIM_URL}/api/v1/agents"
AGENT_ID = "550e8400-e29b-41d4-a716-446655440000"
STRICT_REASON = (
    "pending: the organization is in strict mode; an administrator verifies "
    "the agent under Agents in the dashboard"
)


class PlainConsole:
    """Prints what rich prints for this SDK's console markup, without colour
    or wrapping, so the lines can be compared as written."""

    def print(self, text=""):
        print(re.sub(r"\\\[|\[/?[a-z0-9 #_]*\]", lambda m: "[" if m.group(0) == "\\[" else "", str(text)))


def answer(status, reason=None):
    def callback(request):
        body = json.loads(request.body)
        agent = {
            "id": AGENT_ID,
            "name": body["name"],
            "displayName": body["displayName"],
            "description": body["description"],
            "publicKey": body["publicKey"],
            "status": status,
            "trustScore": 0.59,
        }
        if reason:
            agent["statusReason"] = reason
        return 201, {"Content-Type": "application/json"}, json.dumps(agent)
    return callback


@pytest.fixture
def plain_console(monkeypatch):
    monkeypatch.setattr(console_module, "RICH_AVAILABLE", True)
    monkeypatch.setattr(console_module.console, "console", PlainConsole())
    monkeypatch.setattr(console_module.console, "quiet", False)


def secure_shortest(status, reason=None):
    """secure("my-first-agent", capabilities=["db:read"]), signed in with an API key."""
    responses.add_callback(responses.POST, AGENTS_URL, callback=answer(status, reason))
    with patch("aim_sdk.client.load_sdk_credentials", return_value=None), \
            patch("aim_sdk.client._load_credentials", return_value=None), \
            patch("aim_sdk.client._save_credentials"), \
            patch("aim_sdk.client.security_logger"):
        register_agent("my-first-agent", capabilities=["db:read"], aim_url=AIM_URL, api_key="aim_test_not_real",
                       auto_hooks=False)
    return json.loads(responses.calls[0].request.body)


@responses.activate
def test_shortest_call_sends_what_verification_at_registration_needs_and_prints_verified(plain_console, capsys):
    sent = secure_shortest("verified")

    # The server's metadata check needs a display name and a description; the
    # shortest call derives both from the name, as the TypeScript and Java SDKs do.
    assert sent["displayName"] == "my-first-agent"
    assert sent["description"] == "Agent my-first-agent registered via AIM SDK"
    assert sent["capabilities"] == ["db:read"]
    assert len(sent["publicKey"]) == 44, "the public half of the agent's own Ed25519 key"

    printed = capsys.readouterr().out.split("\n")
    assert "  Status: ● verified  Trust: 59%" in printed
    assert not [line for line in printed if "pending" in line]


@responses.activate
def test_a_pending_registration_prints_the_reason_under_the_status(plain_console, capsys):
    secure_shortest("pending", STRICT_REASON)

    printed = capsys.readouterr().out.split("\n")
    status = printed.index("  Status: ○ pending  Trust: 59%")
    assert printed[status + 1] == "  " + STRICT_REASON


@responses.activate
def test_a_pending_registration_without_rich_prints_the_whole_reason_in_the_panel(monkeypatch, capsys):
    monkeypatch.setattr(console_module, "RICH_AVAILABLE", False)
    monkeypatch.setattr(console_module.console, "quiet", False)
    secure_shortest("pending", "pending: trust score 0.25 is below the threshold 0.30")

    rows = [line.strip("│ ").strip() for line in capsys.readouterr().out.split("\n") if line.startswith("│")]
    status = rows.index("Status:      pending")
    reason = " ".join(rows[status + 1:status + 3])
    assert reason == "pending: trust score 0.25 is below the threshold 0.30"
