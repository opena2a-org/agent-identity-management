"""
AIM SDK Exception Classes
"""


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
