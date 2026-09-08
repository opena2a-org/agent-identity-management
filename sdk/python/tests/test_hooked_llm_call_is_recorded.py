"""
An installed OpenAI hook must record the hooked call.

In every published aim-sdk that carried auto_hooks.py (0.5.3, 1.22.1 through
2.0.1) the OpenAI and Anthropic hooks referenced AgentEventType.AGENT_ACTION,
a member the security logger never defined, so every hooked call raised
AttributeError inside the hook's try/except and recorded nothing. This test is
RED on those versions (the spy is never called) and GREEN once the member
exists. The control asserts the wrapped call still returns its original value,
which holds on both sides: the hooks never raise.
"""

import sys
import types

from aim_sdk import security_logging
from aim_sdk.auto_hooks import activate_hooks


class _Client:
    agent_id = "agent-123"


def _fake_openai(monkeypatch):
    def original(self, *args, **kwargs):
        return "original-result"

    completions_cls = type("Completions", (), {"create": original})
    module = types.ModuleType("openai")
    module.resources = types.SimpleNamespace(
        chat=types.SimpleNamespace(
            completions=types.SimpleNamespace(Completions=completions_cls)
        )
    )
    monkeypatch.setitem(sys.modules, "openai", module)
    return completions_cls


def test_hooked_openai_call_records_one_agent_action_event(monkeypatch):
    completions_cls = _fake_openai(monkeypatch)
    calls = []
    monkeypatch.setattr(
        security_logging.security_logger,
        "log_agent_event",
        lambda event_type, **kwargs: calls.append((event_type, kwargs)),
    )
    assert "openai" in activate_hooks(_Client(), auto_hooks=True)

    result = completions_cls.create(completions_cls(), model="test-model")

    assert result == "original-result", "the wrapped call must still return its value"
    assert len(calls) == 1, "the hook must record exactly one event"
    event_type, kwargs = calls[0]
    assert event_type is security_logging.AgentEventType.AGENT_ACTION
    assert kwargs["agent_id"] == "agent-123"
    assert kwargs["details"]["method"] == "chat.completions.create"
    assert kwargs["details"]["resource"] == "openai:test-model"
