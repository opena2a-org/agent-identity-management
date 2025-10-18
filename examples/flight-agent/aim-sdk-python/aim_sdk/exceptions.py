"""
AIM SDK Exception Classes

Enhanced error messages that help users understand and resolve issues quickly.
"""


class AIMError(Exception):
    """Base exception for all AIM SDK errors"""

    def __init__(self, message: str, help_url: str = None, solution: str = None):
        """
        Initialize AIM error with helpful context.

        Args:
            message: Error message
            help_url: Optional URL to documentation
            solution: Optional suggested solution
        """
        self.message = message
        self.help_url = help_url
        self.solution = solution

        # Build comprehensive error message
        full_message = f"\n{message}"

        if solution:
            full_message += f"\n\n💡 Solution:\n{solution}"

        if help_url:
            full_message += f"\n\n📚 Learn more: {help_url}"

        super().__init__(full_message)


class AuthenticationError(AIMError):
    """Raised when authentication with AIM fails"""
    pass


class TokenExpiredError(AuthenticationError):
    """
    Raised when refresh token has been rotated or revoked.

    This is expected behavior after token rotation - a security feature
    that protects against token theft and unauthorized access.
    """

    def __init__(self, portal_url: str = "http://localhost:3000", docs_url: str = None):
        message = "❌ Authentication Failed: Token Expired"

        solution = f"""Your SDK credentials have expired due to token rotation (security policy).

To fix this issue:
  1. Log in to AIM portal: {portal_url}/auth/login
  2. Download fresh SDK: {portal_url}/dashboard/sdk
  3. Copy new credentials to ~/.aim/credentials.json

Why does this happen?
  AIM uses token rotation for enterprise security:
  • When you use a refresh token → backend issues a NEW token
  • OLD token is immediately revoked → prevents token theft
  • This is SOC 2 / HIPAA compliant behavior

This security measure protects your organization from unauthorized access."""

        help_url = docs_url or f"{portal_url}/docs/security/token-rotation"

        super().__init__(message, help_url, solution)


class InvalidCredentialsError(AuthenticationError):
    """
    Raised when agent credentials are invalid or malformed.
    """

    def __init__(self, reason: str = "Invalid credentials format", portal_url: str = "http://localhost:3000"):
        message = f"❌ Authentication Failed: {reason}"

        solution = f"""Your agent credentials appear to be invalid or corrupted.

To fix this issue:
  1. Download fresh SDK from: {portal_url}/dashboard/sdk
  2. Extract the ZIP file
  3. Copy .aim/credentials.json to your project or ~/.aim/

If you're using an existing agent:
  • Check that credentials.json has both OAuth tokens AND agent keys
  • Verify the file hasn't been corrupted or modified
  • Ensure you have the correct agent_id

Need help? Contact support with your agent ID."""

        help_url = f"{portal_url}/docs/troubleshooting/authentication"

        super().__init__(message, help_url, solution)


class VerificationError(AIMError):
    """Raised when action verification fails or is rejected"""
    pass


class ActionDeniedError(AIMError):
    """Raised when AIM denies permission to perform an action"""

    def __init__(self, action_type: str, reason: str = "Permission denied", dashboard_url: str = "http://localhost:3000/dashboard"):
        message = f"❌ Action Denied: {action_type}"

        solution = f"""AIM denied permission to perform this action.

Reason: {reason}

Possible causes:
  • Agent trust score is too low
  • Action risk level exceeds allowed threshold
  • Agent is suspended or inactive
  • Organization policy blocks this action type

To resolve:
  1. Check your agent's trust score: {dashboard_url}
  2. Review security alerts for your agent
  3. Verify agent is active and verified
  4. Contact your AIM administrator

Build trust by:
  • Performing verified actions successfully
  • Avoiding failed or risky actions
  • Maintaining consistent behavior"""

        help_url = f"{dashboard_url}/docs/trust-scoring"

        super().__init__(message, help_url, solution)


class ConfigurationError(AIMError):
    """Raised when SDK is misconfigured"""

    def __init__(self, issue: str, fix: str = None):
        message = f"⚙️  Configuration Error: {issue}"

        if fix:
            solution = f"""Configuration issue detected.

How to fix:
{fix}

Common configuration issues:
  • Missing or invalid aim_url
  • Missing agent_id
  • Missing credentials (API key OR Ed25519 keys)
  • Invalid credential format

For proper SDK setup:
  1. Download SDK from AIM portal
  2. Use secure() function for automatic setup
  3. Or provide all required parameters manually"""
        else:
            solution = None

        help_url = "http://localhost:3000/docs/sdk/configuration"

        super().__init__(message, help_url, solution)
