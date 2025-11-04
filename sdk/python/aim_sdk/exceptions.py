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
