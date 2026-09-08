"""
PolicyCache must never answer allow from an unloaded cache.

Every published aim-sdk that carried auto_hooks.py (0.5.3, and 1.22.1 through
2.0.1) returned True from PolicyCache.check() whenever the policy fetch failed,
and the fetch always fails because no released AIM server registers
/api/v1/agents/{id}/policies. Cells (i) to (iv) below are RED on that code and
GREEN on the fix. Cells (v) and (vi) are the positive controls, green on both,
proving the loaded-document logic is exercised rather than short-circuited.

To see the red side, run this file against the published module from a
directory that is not sdk/python, so the wheel copy resolves first:

    PYTHONPATH=<extracted 2.0.1 wheel> python3 -m pytest <path to this file>

NOT marked integration: pytest.ini deselects that marker, so a test carrying it
never runs in CI.
"""

import pytest
import requests

from aim_sdk.auto_hooks import PolicyCache
from aim_sdk.exceptions import VerificationUnavailableError


class _Client:
    aim_url = "https://aim.example.test"
    agent_id = "agent-123"
    api_key = "test-only-placeholder"


class _Resp:
    def __init__(self, status, body):
        self.status_code = status
        self._body = body

    def json(self):
        return self._body


def _cache_with(monkeypatch, responder):
    # _refresh_if_needed imports requests and calls requests.get at call time,
    # so patching the module attribute intercepts the real fetch path.
    monkeypatch.setattr(requests, "get", responder)
    return PolicyCache(_Client(), ttl_seconds=300)


def test_404_raises_unavailable_and_names_the_route(monkeypatch):
    """Cell (i): 2.0.1 returned True here."""
    cache = _cache_with(monkeypatch, lambda url, **kw: _Resp(404, {"error": "not found"}))
    with pytest.raises(VerificationUnavailableError) as excinfo:
        cache.check("payment:refund")
    assert "/api/v1/agents/agent-123/policies" in str(excinfo.value)


def test_connection_error_raises_unavailable(monkeypatch):
    """Cell (ii): 2.0.1 returned True here."""

    def refuse(url, **kw):
        raise requests.ConnectionError("connection refused")

    cache = _cache_with(monkeypatch, refuse)
    with pytest.raises(VerificationUnavailableError):
        cache.check("payment:refund")


def test_loaded_document_without_default_action_denies_unlisted(monkeypatch):
    """Cell (iii): 2.0.1 returned True here."""
    cache = _cache_with(monkeypatch, lambda url, **kw: _Resp(200, {"rules": []}))
    assert cache.check("payment:refund") is False


def test_non_object_body_raises_unavailable(monkeypatch):
    """Cell (iv): 2.0.1 returned True here."""
    cache = _cache_with(monkeypatch, lambda url, **kw: _Resp(200, ["not", "a", "document"]))
    with pytest.raises(VerificationUnavailableError):
        cache.check("payment:refund")


def test_explicit_deny_rule_denies(monkeypatch):
    """Cell (v), positive control: green before and after the fix."""
    doc = {"defaultAction": "allow", "rules": [{"capability": "payment:refund", "action": "deny"}]}
    cache = _cache_with(monkeypatch, lambda url, **kw: _Resp(200, doc))
    assert cache.check("payment:refund") is False


def test_default_allow_allows_an_unlisted_capability(monkeypatch):
    """Cell (vi), positive control: green before and after the fix."""
    doc = {"defaultAction": "allow", "rules": [{"capability": "payment:refund", "action": "deny"}]}
    cache = _cache_with(monkeypatch, lambda url, **kw: _Resp(200, doc))
    assert cache.check("weather:read") is True


def test_default_action_deny_denies_an_unlisted_capability(monkeypatch):
    """A present defaultAction that is not "allow" denies; only "allow" allows."""
    doc = {"defaultAction": "deny", "rules": []}
    cache = _cache_with(monkeypatch, lambda url, **kw: _Resp(200, doc))
    assert cache.check("weather:read") is False


def test_a_rule_with_any_action_but_allow_denies(monkeypatch):
    """A matching rule decides, and only action "allow" allows; "audit" is a deny."""
    doc = {"defaultAction": "allow", "rules": [{"capability": "payment:refund", "action": "audit"}]}
    cache = _cache_with(monkeypatch, lambda url, **kw: _Resp(200, doc))
    assert cache.check("payment:refund") is False


def test_a_malformed_rule_entry_is_ignored_not_matched(monkeypatch):
    """A rules entry that is not an object cannot match; the document's default decides."""
    doc = {"rules": ["payment:refund"]}
    cache = _cache_with(monkeypatch, lambda url, **kw: _Resp(200, doc))
    assert cache.check("payment:refund") is False
