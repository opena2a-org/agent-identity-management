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


class ActionDeniedError(AIMError):
    """Raised when AIM denies permission to perform an action"""
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
