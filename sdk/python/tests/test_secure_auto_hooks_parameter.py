"""
secure(..., auto_hooks=False) is documented in the README and must be accepted,
and it must install no framework hook.

On the published 2.0.1 the keyword did not exist:
inspect.signature(secure).bind("x", auto_hooks=False) raised TypeError, so the
documented call failed. The first test is RED on 2.0.1 and GREEN on the fix; the
two activate_hooks tests are controls, green on both.
"""

import inspect
import sys
import types

from aim_sdk import secure
from aim_sdk.auto_hooks import activate_hooks


def test_secure_accepts_auto_hooks_as_a_keyword_only_parameter():
    params = inspect.signature(secure).parameters
    assert "auto_hooks" in params, "secure() must accept auto_hooks; the README documents it"
    parameter = params["auto_hooks"]
    assert parameter.kind is inspect.Parameter.KEYWORD_ONLY
    assert parameter.default is True


class _Client:
    agent_id = "agent-123"


def _fake_openai(monkeypatch):
    """A minimal openai module tree with the attribute path the hook patches."""

    def original(self, *args, **kwargs):
        return "original"

    completions_cls = type("Completions", (), {"create": original})
    module = types.ModuleType("openai")
    module.resources = types.SimpleNamespace(
        chat=types.SimpleNamespace(
            completions=types.SimpleNamespace(Completions=completions_cls)
        )
    )
    monkeypatch.setitem(sys.modules, "openai", module)
    return module, completions_cls, original


def test_auto_hooks_false_installs_nothing(monkeypatch):
    module, completions_cls, original = _fake_openai(monkeypatch)
    assert activate_hooks(_Client(), auto_hooks=False) == []
    assert completions_cls.create is original
    assert not hasattr(module, "_aim_hooked")


def test_auto_hooks_true_patches_the_detected_framework(monkeypatch):
    module, completions_cls, original = _fake_openai(monkeypatch)
    hooked = activate_hooks(_Client(), auto_hooks=True)
    assert "openai" in hooked
    assert completions_cls.create is not original
    assert getattr(module, "_aim_hooked", False) is True


def test_a_detected_library_whose_hook_cannot_be_installed_logs_a_warning(monkeypatch, caplog):
    """The README promises a warning for a hook that cannot be installed; this proves it."""
    import logging

    broken = types.ModuleType("openai")  # detected, but without the attribute path the hook patches
    monkeypatch.setitem(sys.modules, "openai", broken)
    with caplog.at_level(logging.WARNING, logger="aim_sdk.auto_hooks"):
        hooked = activate_hooks(_Client(), auto_hooks=True)
    assert "openai" not in hooked
    warnings = [r for r in caplog.records if r.name == "aim_sdk.auto_hooks" and r.levelno == logging.WARNING]
    assert len(warnings) == 1, "exactly one warning for the one library whose hook could not be installed"
    assert "openai" in warnings[0].getMessage()
