"""
AAP grant client — the developer surface for the Agent Authorization Protocol.

An agent references a grant (`grant://name`); the SDK talks to the Secretless broker
daemon over its local Unix socket; the broker verifies the ATX, authorizes, resolves a
scoped credential, performs the operation in an ephemeral worker, and returns ONLY the
result. No credential value and no backend identifier ever enters the agent process.

This module is intentionally thin: the broker (in Secretless AI) does all the work. See
the Agent Authorization Protocol spec (opena2a-standards/agent-authorization-protocol).

PROVISIONAL / EXPERIMENTAL. AAP is at spec 0.4.0-draft. This client and the broker
`/grant` wire format it depends on may change in a future minor release without a major
version bump. Pin an exact aim-sdk version if you depend on it.

Broker-side readiness is tracked in
opena2a-standards/agent-authorization-protocol#1, which is the source of truth rather
than this docstring. As of 2026-08-19 a stock Secretless broker did not construct a
grant resolver, so `/grant` answered 404 and every call here failed; see that issue for
the current state.
"""

from __future__ import annotations

import http.client
import json
import os
import socket
from contextvars import ContextVar
from typing import Any, Dict, Optional

DEFAULT_SOCKET_PATH = os.path.expanduser("~/.secretless-ai/broker.sock")
DEFAULT_TOKEN_PATH = os.path.expanduser("~/.secretless-ai/broker.token")


class BrokerGrantError(Exception):
    """The broker could not be reached or returned an unexpected error."""


class GrantDeniedError(BrokerGrantError):
    """The broker denied the grant. By design it reveals no reason (AAP §6.6)."""


class _UnixHTTPConnection(http.client.HTTPConnection):
    """HTTPConnection over a Unix domain socket (the broker's primary transport)."""

    def __init__(self, socket_path: str, timeout: float = 5.0):
        super().__init__("localhost", timeout=timeout)
        self._socket_path = socket_path

    def connect(self) -> None:  # noqa: D401
        sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        sock.settimeout(self.timeout)
        sock.connect(self._socket_path)
        self.sock = sock


class BrokerClient:
    """Client for the Secretless broker's AAP `/grant` endpoint.

    Defaults to the broker's Unix socket; pass ``http_url`` for the HTTP fallback
    (e.g. inside Docker). The bearer token is read from the broker token file.
    """

    def __init__(
        self,
        socket_path: Optional[str] = None,
        http_url: Optional[str] = None,
        token: Optional[str] = None,
        token_path: Optional[str] = None,
        timeout: float = 5.0,
    ):
        self.socket_path = socket_path or DEFAULT_SOCKET_PATH
        self.http_url = http_url
        self.timeout = timeout
        self._token = token
        self._token_path = token_path or DEFAULT_TOKEN_PATH

    def _health_probe(self) -> str:
        """A runnable /health command for whichever transport this client is using.

        `-f` is load-bearing, not decoration: a bare `curl` prints the error body and
        still exits 0, so a broker with no /health route hands the user a page of JSON
        and a clean exit, which reads as success. With -f the command fails loudly
        (exit 22) and stays quiet on the happy path.

        Only values this client already holds are interpolated -- never anything from
        the broker's response.
        """
        if self.http_url:
            return f"curl -fsS {self.http_url.rstrip('/')}/health"
        return f"curl -fsS --unix-socket {self.socket_path} http://localhost/health"

    def _bearer(self) -> str:
        if self._token:
            return self._token
        try:
            with open(self._token_path, "r", encoding="utf-8") as fh:
                return fh.read().strip()
        except OSError as exc:
            raise BrokerGrantError(
                f"broker token not found at {self._token_path}; is the broker running?"
            ) from exc

    def grant(
        self,
        agent_id: str,
        atx: Dict[str, Any],
        grant: str,
        operation: Dict[str, Any],
    ) -> Any:
        """Resolve a grant and return ONLY the operation result.

        Raises GrantDeniedError on a 403 (uniform opaque denial) and BrokerGrantError
        on transport/other failures.
        """
        payload = json.dumps(
            {"agentId": agent_id, "atx": atx, "grant": grant, "operation": operation}
        ).encode("utf-8")
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self._bearer()}",
        }

        if self.http_url:
            url = self.http_url.rstrip("/")
            scheme, _, host = url.partition("://")
            conn: http.client.HTTPConnection = (
                http.client.HTTPSConnection(host, timeout=self.timeout)
                if scheme.lower() == "https"
                else http.client.HTTPConnection(host, timeout=self.timeout)
            )
        else:
            conn = _UnixHTTPConnection(self.socket_path, timeout=self.timeout)

        try:
            conn.request("POST", "/grant", body=payload, headers=headers)
            resp = conn.getresponse()
            body = resp.read().decode("utf-8")
        except OSError as exc:
            raise BrokerGrantError(f"could not reach broker: {exc}") from exc
        finally:
            conn.close()

        if resp.status == 200:
            try:
                return json.loads(body).get("result")
            except json.JSONDecodeError as exc:
                raise BrokerGrantError("broker returned non-JSON success body") from exc
        if resp.status == 403:
            raise GrantDeniedError("grant denied")
        if resp.status == 404:
            # The broker is up and answering, it just has no grant resolver wired. A bare
            # "status 404" here sends the caller looking for a typo in their grant
            # reference, which is the wrong place. Say which side is missing, and hand
            # back a check that runs on the transport THIS client is actually using -- a
            # hardcoded --unix-socket line is wrong for anyone on the http_url fallback.
            raise BrokerGrantError(
                "/grant returned 404. The broker answered, so the socket and token "
                "reached it. The most likely cause is that this build has no AAP grant "
                "surface configured: a stock broker does not construct a grant "
                "resolver, and then every grant call 404s. Check the broker is the one "
                f"you meant with `{self._health_probe()}`. If that succeeds and grants "
                "still 404, the remaining candidates are a grant reference this broker "
                "has no binding for, or an http_url pointing at something other than "
                "the broker. Background: "
                "opena2a-standards/agent-authorization-protocol#1."
            )
        raise BrokerGrantError(f"broker returned status {resp.status}")


class GrantSession:
    """A grant bound to the broker. Performs operations and returns only results.

    The agent code holds a GrantSession, never a credential. Each call sends the logical
    operation (method + path, no host, no token) to the broker and gets the result back.
    """

    def __init__(self, broker: BrokerClient, grant: str, agent_id: str, atx: Dict[str, Any]):
        self._broker = broker
        self._grant = grant
        self._agent_id = agent_id
        self._atx = atx

    @property
    def reference(self) -> str:
        return self._grant

    def request(
        self,
        method: str,
        path: str,
        query: Optional[Dict[str, str]] = None,
        body: Any = None,
    ) -> Any:
        """Perform a logical operation against the granted resource; return only the result."""
        operation: Dict[str, Any] = {"method": method, "path": path}
        if query is not None:
            operation["query"] = query
        if body is not None:
            operation["body"] = body
        return self._broker.grant(self._agent_id, self._atx, self._grant, operation)


# Active grant for the currently-executing @perform_action body (when grant= is set).
_current_grant: ContextVar[Optional[GrantSession]] = ContextVar("aim_current_grant", default=None)


def current_grant() -> Optional[GrantSession]:
    """Return the GrantSession bound by the enclosing @perform_action(grant=...), if any."""
    return _current_grant.get()
