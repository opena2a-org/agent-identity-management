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

    def test_404_says_the_broker_lacks_a_grant_surface(self, monkeypatch):
        """A 404 is not a bad grant reference: the broker is up and has no resolver.

        The bare `broker returned status 404` this used to raise sent the caller looking
        for a typo in their own grant reference, which is the one place the fault is not.
        Assert on what the message has to TELL them, not on its exact wording.
        """
        client = BrokerClient(socket_path="/nonexistent.sock", token="t")

        class _Resp:
            status = 404
            def read(self):
                return b'{"error":"Not found"}'

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
        with pytest.raises(BrokerGrantError) as excinfo:
            client.grant("a", {}, "grant://x", {"method": "GET", "path": "/"})

        message = str(excinfo.value)
        # It must NOT be the old bare-status message, and must not be the denial class.
        assert message != "broker returned status 404"
        assert not isinstance(excinfo.value, GrantDeniedError)
        # It must name the likely cause and give a check to run...
        assert "404" in message
        assert "grant surface" in message
        assert "/health" in message
        # ...while stating the transport fact rather than ruling other causes out. A 404
        # also fits an unbound grant reference or a misaimed http_url, and the message
        # must leave the caller those options instead of asserting one cause.
        assert "the socket and token reached it" in message
        assert "most likely" in message
        assert "no binding for" in message

    @pytest.mark.parametrize(
        "kwargs, must_contain, must_not_contain",
        [
            # A custom socket must appear; the DEFAULT path must not be assumed.
            (
                {"socket_path": "/run/custom/broker.sock"},
                ["curl -fsS --unix-socket /run/custom/broker.sock", "http://localhost/health"],
                [".secretless-ai"],
            ),
            # On the http_url fallback (e.g. Docker) a --unix-socket line is unrunnable.
            (
                {"http_url": "http://broker.internal:7000/"},
                ["curl -fsS http://broker.internal:7000/health"],
                ["--unix-socket"],
            ),
            # An uppercase scheme must still be recognised as TLS. This case previously
            # escaped every patch and reached real DNS, because only HTTPConnection was
            # stubbed while an https URL takes HTTPSConnection.
            (
                {"http_url": "HTTPS://broker.internal/"},
                ["curl -fsS HTTPS://broker.internal/health"],
                ["--unix-socket"],
            ),
        ],
    )
    def test_404_verify_command_matches_the_transport_in_use(
        self, monkeypatch, kwargs, must_contain, must_not_contain
    ):
        """The Verify command must run on the caller's actual transport.

        A hardcoded `--unix-socket ~/.secretless-ai/broker.sock` is wrong twice over: for
        a custom socket path, and for anyone on the http_url fallback. A remediation
        command that does not run is a dead end wearing a fix's clothes.
        """
        client = BrokerClient(token="t", **kwargs)

        class _Resp:
            status = 404
            def read(self):
                return b'{"error":"Not found"}'

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
        monkeypatch.setattr(
            "aim_sdk.grant_client.http.client.HTTPConnection", lambda *a, **k: _Conn()
        )
        monkeypatch.setattr(
            "aim_sdk.grant_client.http.client.HTTPSConnection", lambda *a, **k: _Conn()
        )
        with pytest.raises(BrokerGrantError) as excinfo:
            client.grant("a", {}, "grant://x", {"method": "GET", "path": "/"})

        message = str(excinfo.value)
        # L2: pytest.raises(BrokerGrantError) also accepts GrantDeniedError, which
        # subclasses it. Without this the case survives a mutation that swaps the class.
        assert not isinstance(excinfo.value, GrantDeniedError)
        for fragment in must_contain:
            assert fragment in message, f"{fragment!r} missing from: {message}"
        for fragment in must_not_contain:
            assert fragment not in message, f"{fragment!r} wrongly present in: {message}"

    @pytest.mark.parametrize(
        "url, expect_tls",
        [
            ("https://broker.internal/", True),
            ("HTTPS://broker.internal/", True),   # the case that leaked
            ("Https://broker.internal/", True),   # and its mixed-case sibling
            ("http://broker.internal/", False),
            ("HTTP://broker.internal/", False),
        ],
    )
    def test_scheme_comparison_is_case_insensitive(self, monkeypatch, url, expect_tls):
        """An uppercase https:// must not fall through to a plaintext connection.

        grant() sends `Authorization: Bearer <broker token>`. When the scheme compare was
        an exact lowercase match, `HTTPS://` selected HTTPConnection, so the token went
        out in cleartext on port 80 while the caller believed they were on TLS.

        The oracle has to record WHICH constructor ran. Stubbing both with the same
        double cannot tell them apart, so a test written that way stays green when the
        comparison regresses -- which is exactly what happened before this test existed.
        """
        client = BrokerClient(http_url=url, token="t")
        used = []

        def _make(kind):
            def _ctor(*a, **k):
                used.append(kind)
                class _Resp:
                    status = 200
                    def read(self):
                        return b'{"result":"ok"}'
                class _Conn:
                    def request(self, *a, **k):
                        pass
                    def getresponse(self):
                        return _Resp()
                    def close(self):
                        pass
                return _Conn()
            return _ctor

        monkeypatch.setattr(client, "_bearer", lambda: "t")
        monkeypatch.setattr(
            "aim_sdk.grant_client.http.client.HTTPSConnection", _make("tls")
        )
        monkeypatch.setattr(
            "aim_sdk.grant_client.http.client.HTTPConnection", _make("plain")
        )
        client.grant("a", {}, "grant://x", {"method": "GET", "path": "/"})

        assert used == ["tls" if expect_tls else "plain"], (
            f"{url!r} selected {used!r}; a plaintext connection here sends the bearer "
            f"token in the clear"
        )

    def test_other_statuses_still_fall_through_to_the_bare_message(self, monkeypatch):
        """The 404 branch must not swallow every non-200. 500 keeps the generic path."""
        client = BrokerClient(socket_path="/nonexistent.sock", token="t")

        class _Resp:
            status = 500
            def read(self):
                return b"boom"

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
        with pytest.raises(BrokerGrantError) as excinfo:
            client.grant("a", {}, "grant://x", {"method": "GET", "path": "/"})
        assert str(excinfo.value) == "broker returned status 500"
        assert "grant surface" not in str(excinfo.value)

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
