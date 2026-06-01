"""Tests for the AAP grant surface: GrantSession + @perform_action(grant=...)."""

import json

import pytest

from aim_sdk.client import AIMClient
from aim_sdk.grant_client import (
    BrokerClient,
    GrantSession,
    GrantDeniedError,
    BrokerGrantError,
    current_grant,
)


class FakeBroker(BrokerClient):
    """In-memory broker double — records the last grant call, returns only a result."""

    def __init__(self, result=None, raises=None):
        # Skip BrokerClient.__init__ (no socket/token needed for the double).
        self.calls = []
        self._result = result if result is not None else {"rows": [{"id": 1}]}
        self._raises = raises

    def grant(self, agent_id, atx, grant, operation):
        self.calls.append(
            {"agent_id": agent_id, "atx": atx, "grant": grant, "operation": operation}
        )
        if self._raises is not None:
            raise self._raises
        return self._result


def _bare_agent(broker, atx=None):
    """An AIMClient with just the attributes the grant path needs (no network/__init__)."""
    agent = AIMClient.__new__(AIMClient)
    agent.agent_id = "aim_orders_reader"
    agent._grant_broker = broker
    agent.atx = atx if atx is not None else {"atcVersion": "1.0", "agentId": "aim_orders_reader"}
    # Stub the verification + logging touchpoints used by perform_action.
    agent.register_capability = lambda **kw: None
    agent.verify_capability = lambda **kw: {"verification_id": "ver-123"}
    agent.log_capability_result = lambda **kw: None
    return agent


class TestGrantSession:
    def test_request_sends_logical_operation_and_returns_only_result(self):
        broker = FakeBroker(result={"rows": [{"id": 1, "total": 42}]})
        session = GrantSession(broker, "grant://orders-db", "aim_orders_reader", {"atcVersion": "1.0"})

        result = session.request("GET", "/orders", query={"customer": "c-123"})

        assert result == {"rows": [{"id": 1, "total": 42}]}
        call = broker.calls[0]
        assert call["grant"] == "grant://orders-db"
        assert call["operation"] == {"method": "GET", "path": "/orders", "query": {"customer": "c-123"}}
        # The result handed back carries no credential/backend field.
        assert "token" not in json.dumps(result).lower()

    def test_denial_raises(self):
        broker = FakeBroker(raises=GrantDeniedError("grant denied"))
        session = GrantSession(broker, "grant://orders-db", "a", {})
        with pytest.raises(GrantDeniedError):
            session.request("GET", "/orders")


class TestPerformActionGrant:
    def test_injects_grant_session_as_kwarg(self):
        broker = FakeBroker(result={"rows": ["ok"]})
        agent = _bare_agent(broker)

        @agent.perform_action(capability="orders:read", grant="grant://orders-db")
        def recent_orders(customer_id, grant):
            assert isinstance(grant, GrantSession)
            assert grant.reference == "grant://orders-db"
            return grant.request("GET", "/orders", query={"customer": customer_id})

        out = recent_orders("c-123")
        assert out == {"rows": ["ok"]}
        assert broker.calls[0]["operation"]["query"] == {"customer": "c-123"}

    def test_grant_available_via_contextvar_when_not_a_param(self):
        broker = FakeBroker(result={"ok": True})
        agent = _bare_agent(broker)

        @agent.perform_action(capability="orders:read", grant="grant://orders-db")
        def do_it():
            session = current_grant()
            assert session is not None
            return session.request("POST", "/orders", body={"x": 1})

        assert do_it() == {"ok": True}
        # Context is cleared after the call returns.
        assert current_grant() is None

    def test_no_grant_means_no_session_and_no_broker_calls(self):
        broker = FakeBroker()
        agent = _bare_agent(broker)

        @agent.perform_action(capability="weather:read")
        def get_weather():
            assert current_grant() is None
            return "sunny"

        assert get_weather() == "sunny"
        assert broker.calls == []


class TestBrokerClientWire:
    def test_grant_denied_maps_403(self, monkeypatch):
        client = BrokerClient(socket_path="/nonexistent.sock", token="t")

        class _Resp:
            status = 403
            def read(self):
                return b'{"error":"denied"}'

        class _Conn:
            def request(self, *a, **k):
                pass
            def getresponse(self):
                return _Resp()
            def close(self):
                pass

        monkeypatch.setattr(client, "_bearer", lambda: "t")
        monkeypatch.setattr(
            "aim_sdk.grant_client._UnixHTTPConnection", lambda *a, **k: _Conn()
        )
        with pytest.raises(GrantDeniedError):
            client.grant("a", {}, "grant://x", {"method": "GET", "path": "/"})

    def test_grant_success_returns_result_only(self, monkeypatch):
        client = BrokerClient(socket_path="/nonexistent.sock", token="t")

        class _Resp:
            status = 200
            def read(self):
                return b'{"result":{"rows":[1,2,3]}}'

        class _Conn:
            captured = {}
            def request(self, method, path, body=None, headers=None):
                _Conn.captured = {"method": method, "path": path, "body": body, "headers": headers}
            def getresponse(self):
                return _Resp()
            def close(self):
                pass

        monkeypatch.setattr("aim_sdk.grant_client._UnixHTTPConnection", lambda *a, **k: _Conn())
        result = client.grant("a", {"atcVersion": "1.0"}, "grant://x", {"method": "GET", "path": "/orders"})

        assert result == {"rows": [1, 2, 3]}
        assert _Conn.captured["path"] == "/grant"
        assert _Conn.captured["headers"]["Authorization"] == "Bearer t"
