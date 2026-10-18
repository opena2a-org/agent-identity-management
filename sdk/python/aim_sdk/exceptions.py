"""
AIM SDK Exception Classes
"""

import re


class AIMError(Exception):
    """Base exception for all AIM SDK errors"""
    pass


class AuthenticationError(AIMError):
    """Raised when authentication with AIM fails"""
    pass


class VerificationError(AIMError):
    """Raised when action verification fails or is rejected"""
    pass


class _CarriesDecision:
    """Mixin: an exception that carries the typed decision that produced it.

    From 2.0.0 the three-state verification decision is the exception channel.
    ``verify_capability`` returns only on ALLOW; DENY raises
    :class:`ActionDeniedError` and an undetermined outcome raises
    :class:`VerificationUnavailableError`. Both carry the decision, so a caller --
    and every decorator in this SDK -- can read the organization's enforcement
    mode on the paths where enforcement is actually decided.

    Without this, the mode channel would exist only on the ALLOW path, which is
    the one path that does not need it.
    """

    def __init__(self, message: str, decision=None):
        super().__init__(message)
        self.decision = decision

    @property
    def mode(self):
        """The enforcement mode resolved for this decision, or None."""
        return getattr(self.decision, "mode", None)

    @property
    def mode_source(self):
        """Where that mode came from (response, cache, or unavailable)."""
        return getattr(self.decision, "mode_source", None)


class ActionDeniedError(_CarriesDecision, AIMError, PermissionError):
    """Raised when AIM denies permission to perform an action.

    AIM ANSWERED and refused. This is not "AIM could not be reached" -- that is
    ``VerificationUnavailableError``, which is deliberately not catchable by a
    handler written for this one.

    ``PermissionError`` is a second base as a COMPATIBILITY measure, added in
    2.0.0. ``README.md`` documents that strict mode raises ``PermissionError``,
    and the decorators have always had an ``except PermissionError`` handler --
    but this class did not inherit from it, so a denial fell through to the
    generic ``except Exception`` fail-open branch and the action ran. Every
    existing ``except PermissionError`` and ``except ActionDeniedError`` handler
    keeps working.

    The base is compatibility, not the mechanism: enforcement is decided by the
    typed outcome in ``aim_sdk.decision``, never by an exception's base class.

    Note one consequence of the base: ``PermissionError`` descends from
    ``OSError``, so a denial now also satisfies ``except OSError`` (and its alias
    ``IOError``). A wrapped function that catches ``OSError`` broadly will catch a
    denial too.
    """
    pass


class VerificationUnavailableError(_CarriesDecision, AIMError):
    """Raised when no verification decision could be obtained.

    AIM was never successfully asked, or answered without a decision: a timeout,
    a connection or DNS or TLS failure, a 429, a 5xx, a 404.

    This deliberately does NOT subclass ``VerificationError`` or
    ``ActionDeniedError``. A handler written for "AIM said no" must not silently
    absorb "AIM was never asked" -- before 2.0.0 a 5xx surfaced as
    ``PermissionError: AIM verification failed ... HTTP 500``, which told a
    developer they had been denied when AIM was merely down.
    """
    pass


# A word of an answer's revocationState or reason that the SDK keeps: a short
# token. Any other value is server-chosen text and is kept out of the message.
_REVOCATION_WORD = re.compile(r'[A-Za-z][A-Za-z0-9_-]{0,31}')


def _revocation_word(value):
    """`value` when it is a string _REVOCATION_WORD matches, otherwise None."""
    if isinstance(value, str) and _REVOCATION_WORD.fullmatch(value):
        return value
    return None


class RevocationUnknownError(VerificationUnavailableError):
    """Raised when AIM answered ``503`` with ``atc_revocation_unknown``: it
    could not read the revocation state of the ATC the request presented.
    ``client.secrets.resolve()`` raises it.

    AIM did not decide. The ATC was neither accepted nor judged revoked, so
    the same request can be sent again with the same ATC, while it is valid,
    after ``retry_after`` seconds. A new ATC does not help.

    One of the did-not-decide family: it subclasses
    :class:`VerificationUnavailableError`, so a handler for that class catches
    this one too, and a handler for this class goes first. The client raises
    it at the first such answer and does not send the request again itself,
    whatever ``auto_retry`` is.

    Attributes:
        code: ``"atc_revocation_unknown"``, the answer's ``error``.
        status: ``503``.
        retry_after: The answer's ``Retry-After`` in whole seconds, or None
            when it is absent or not a whole number of seconds.
        revocation_state: The answer's ``revocationState`` when it is a short
            token, otherwise None.
        reason: The answer's ``reason`` when it is a short token, otherwise
            None.
    """

    code = "atc_revocation_unknown"
    status = 503

    def __init__(self, retry_after=None, revocation_state=None, reason=None):
        if isinstance(retry_after, bool) or not isinstance(retry_after, int) or retry_after < 0:
            retry_after = None
        self.retry_after = retry_after
        self.revocation_state = _revocation_word(revocation_state)
        self.reason = _revocation_word(reason)
        super().__init__(self._message())

    def __reduce__(self):
        return (type(self), (self.retry_after, self.revocation_state, self.reason))

    def _message(self):
        if self.retry_after is None:
            when = "later"
        elif self.retry_after == 0:
            when = "now"
        else:
            when = f"in {self.retry_after} second{'' if self.retry_after == 1 else 's'}"
        message = (
            f"AIM did not decide: it could not read the revocation state of the ATC "
            f"(HTTP {self.status}, {self.code}). The ATC was neither accepted nor judged "
            f"revoked. Send the request again {when} with the same ATC while it is valid; "
            f"a new ATC does not help."
        )
        if self.revocation_state is not None:
            message += f" Revocation state: {self.revocation_state}"
            message += f", reason {self.reason}." if self.reason is not None else "."
        elif self.reason is not None:
            message += f" Reason: {self.reason}."
        return message


class ConfigurationError(AIMError):
    """Raised when SDK is misconfigured"""
    pass


class StaleCredentialsError(ConfigurationError):
    """Raised when cached agent credentials reference an agent that no longer
    exists in the AIM backend, and re-registration would silently wipe
    admin-curated state (status, trust score, granted capabilities, MCPs).

    The SDK refuses to auto-re-register; the operator must take an explicit
    action to recover. The error message includes the local credential path
    and the recovery steps.
    """
    pass


class SecretsError(AIMError):
    """Raised when a secrets operation fails"""
    pass
