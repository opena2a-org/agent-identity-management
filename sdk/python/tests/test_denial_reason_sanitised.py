"""
Issue #384: a server-supplied denial reason reaches the developer's terminal
through exception messages and the monitoring-mode warning. Control bytes in it
can rewrite what the developer sees; an unbounded one can scroll the real
message away.

The guarantee under test: the reason is sanitised where it enters the decision
(VerificationDecision construction), not at each message site, so a new message
builder cannot miss it.
"""

from aim_sdk.decision import (
    EnforcementMode,
    ModeSource,
    Outcome,
    UnknownSource,
    VerificationDecision,
)
from aim_sdk.enforcement import _denial_message, _unavailable_message


def _deny(reason):
    return VerificationDecision(
        outcome=Outcome.DENY,
        mode=EnforcementMode.STRICT,
        mode_source=ModeSource.RESPONSE,
        reason=reason,
    )

def _unknown(reason):
    return VerificationDecision(
        outcome=Outcome.UNKNOWN,
        mode=EnforcementMode.STRICT,
        mode_source=ModeSource.RESPONSE,
        reason=reason,
        unknown_source=UnknownSource.SERVER_ANSWER,
    )


def _control_bytes_in(text):
    """Every char that could drive a terminal rather than be read by a human."""
    return [
        ch
        for ch in text
        if (ch < " " and ch not in "\n\t") or "\x7f" <= ch <= "\x9f"
    ]


class TestControlBytesAreStripped:
    def test_ansi_escape_sequence_is_inert(self):
        # ESC[2K erases the line, ESC[1A moves the cursor up: together they
        # overwrite what the developer just read.
        decision = _deny("\x1b[2K\x1b[1Adenied \x1b[32mALLOWED\x1b[0m")
        assert "\x1b" not in decision.reason
        assert "denied" in decision.reason  # the text itself survives

    def test_c1_csi_is_stripped(self):
        # 0x9b is the single-byte CSI; it drives terminals with no ESC at all.
        decision = _deny("bad\x9b31mactor")
        assert _control_bytes_in(decision.reason) == []
        assert "bad" in decision.reason and "actor" in decision.reason

    def test_carriage_return_is_dropped(self):
        # \r is the overwrite primitive: everything before it gets rewritten.
        decision = _deny("real reason\rFAKE LINE")
        assert "\r" not in decision.reason

    def test_delete_byte_is_stripped(self):
        decision = _deny("a\x7fb")
        assert "\x7f" not in decision.reason

    def test_newline_and_tab_survive(self):
        decision = _deny("line one\n\tindented")
        assert decision.reason == "line one\n\tindented"

    def test_null_and_bell_are_stripped(self):
        decision = _deny("a\x00b\x07c")
        assert decision.reason == "abc"


class TestNonStrReasonNeverCrashesConstruction:
    """
    A server sending {"reason": 123} or a JSON object must not make the
    constructor raise: an exception here converts an explicit DENY into
    UNKNOWN — a fail-open. Found by adversarial review of the first cut of
    this fix, which crashed on exactly these inputs.
    """

    def test_int_reason_stays_deny(self):
        decision = _deny(123)
        assert decision.denied
        assert decision.reason == "123"

    def test_dict_reason_stays_deny(self):
        decision = _deny({"code": "quota", "limit": 5})
        assert decision.denied
        assert isinstance(decision.reason, str)
        assert "quota" in decision.reason

    def test_list_reason_stays_deny(self):
        decision = _deny(["a", "b"])
        assert decision.denied
        assert isinstance(decision.reason, str)

    def test_hostile_non_str_is_still_sanitised(self):
        decision = _deny({"k": "\x1b[2K" + "z" * 100_000})
        assert _control_bytes_in(decision.reason) == []
        assert len(decision.reason) == 512


class TestLengthIsBounded:
    def test_long_reason_is_truncated_with_marker(self):
        decision = _deny("x" * 100_000)
        # Exact bound, pinned from both sides: 500 kept chars + the 12-char
        # marker. A cap that drifted anywhere in 500-588 passed the old
        # <=600 assertion; only 512 passes this one.
        assert len(decision.reason) == 512
        assert decision.reason == "x" * 500 + " [truncated]"

    def test_short_reason_is_untouched(self):
        decision = _deny("insufficient capability grant")
        assert decision.reason == "insufficient capability grant"

    def test_reason_at_cap_gets_no_marker(self):
        decision = _deny("y" * 500)
        assert decision.reason == "y" * 500

    def test_none_reason_stays_none(self):
        decision = _deny(None)
        assert decision.reason is None


class TestMessagesBuiltFromHostileDecisionsAreInert:
    def test_denial_message_carries_no_control_bytes(self):
        decision = _deny("\x1b[2K\rspoof\x9b0m" + "z" * 100_000)
        message = _denial_message("capability 'x' for agent 'a'", decision, decision.mode)
        assert _control_bytes_in(message) == []
        # Bounded by the reason cap plus the builder's fixed guidance text --
        # not by the 100k input.
        assert len(message) < 2_000

    def test_unavailable_message_carries_no_control_bytes(self):
        decision = _unknown("\x1b]0;owned\x07" + "w" * 100_000)
        message = _unavailable_message("capability 'x' for agent 'a'", decision, decision.mode)
        assert _control_bytes_in(message) == []
        # Bounded by the reason cap plus the builder's fixed guidance text --
        # not by the 100k input.
        assert len(message) < 2_000

    def test_monitoring_mode_warning_is_inert(self):
        # The warning named in #384: monitoring mode runs the action and warns
        # with the server's reason. Exercised through evaluate(), the real
        # integration, not by formatting the string by hand.
        from aim_sdk.enforcement import evaluate

        decision = VerificationDecision(
            outcome=Outcome.UNKNOWN,
            mode=EnforcementMode.MONITORING,
            mode_source=ModeSource.RESPONSE,
            reason="\x1b[2K\rfake-all-clear\x9b0m" + "q" * 100_000,
            unknown_source=UnknownSource.SERVER_ANSWER,
        )
        verdict = evaluate(decision, what="capability 'x' for agent 'a'")
        assert verdict.run is True
        assert verdict.warning is not None
        assert _control_bytes_in(verdict.warning) == []
        assert len(verdict.warning) < 2_000


class TestClientSinksAreSanitised:
    """
    client.py prints or raises server text BEFORE any decision exists (found by
    adversarial review of the first cut of this fix): the >=400 console
    warning, the JIT poll warning, and the 401 exception message.
    Construction-time sanitisation cannot reach them; each sanitises at its own
    entry, driven here through the real _decide_capability with only
    session.request stubbed (the shape test_enforcement_matrix.py established).
    """

    HOSTILE = "\x1b[2K\x1b[1A\rfake ALLOWED\x9b31m" + "s" * 100_000

    def _client(self, status, payload):
        import base64

        from nacl.encoding import Base64Encoder
        from nacl.signing import SigningKey

        from aim_sdk.client import AIMClient

        sk = SigningKey.generate()
        client = AIMClient(
            agent_id="agent-under-test",
            public_key=sk.verify_key.encode(encoder=Base64Encoder).decode(),
            private_key=base64.b64encode(bytes(sk)).decode(),
            aim_url="http://aim.invalid",
            sdk_token_id="test-token",
            telemetry={"enabled": False},
        )

        class FakeResponse:
            status_code = status
            text = "raw body"
            headers = {}

            def json(self_inner):
                return payload

        def stub(method, url, **kwargs):
            return FakeResponse()

        client.session.request = stub
        return client

    def test_server_answer_console_warning_is_inert_and_bounded(self, capsys, monkeypatch):
        import aim_sdk.client as client_module
        from aim_sdk.decision import Outcome

        monkeypatch.setattr(client_module, "RATE_LIMIT_RETRY_ATTEMPTS", 0)
        client = self._client(429, {"error": self.HOSTILE})
        decision = client._decide_capability("cap")
        assert decision.outcome is Outcome.UNKNOWN
        out = capsys.readouterr().out
        # The branch under test printed — without this the control-byte assert
        # would pass vacuously on empty output.
        assert "Verification request failed" in out
        assert _control_bytes_in(out.replace("\n", "")) == []
        assert len(out) < 3_000  # bounded by the cap, not the 100k input

    def test_401_exception_message_is_inert_and_bounded(self):
        import pytest

        from aim_sdk.exceptions import AuthenticationError

        client = self._client(401, {"error": self.HOSTILE})
        with pytest.raises(AuthenticationError) as excinfo:
            client._decide_capability("cap")
        message = str(excinfo.value)
        assert _control_bytes_in(message) == []
        assert len(message) < 2_000

    def test_console_sink_strips_control_bytes(self, capsys):
        # Defence in depth: even a call site that forgot to sanitise cannot
        # push control bytes through the console's free-text methods.
        from aim_sdk.console import AIMConsole

        AIMConsole().warning("before\x1b[2K\x9b31m\rafter")
        out = capsys.readouterr().out
        assert _control_bytes_in(out.replace("\n", "")) == []
        assert "before" in out and "after" in out
