"""
AIM SDK Command Line Interface.

Provides commands for authenticating and managing the SDK:
- login: Authenticate with AIM server and save credentials
- logout: Revoke credentials and clear local storage
- status: Check current authentication status
- version: Show SDK version

Usage:
    aim-sdk login                    # Login to AIM Cloud (aim.opena2a.org)
    aim-sdk login --url http://localhost:8080  # Login to self-hosted
    aim-sdk logout                   # Clear credentials
    aim-sdk status                   # Check authentication status
"""

import argparse
import getpass
import sys
import os
from pathlib import Path

import requests

# Version - avoid circular import
def _get_version():
    """Get version from VERSION file or fallback."""
    version_file = os.path.join(os.path.dirname(__file__), "..", "VERSION")
    if os.path.exists(version_file):
        with open(version_file, "r", encoding="utf-8") as vf:
            return vf.read().strip()
    try:
        from importlib.metadata import version
        return version("aim-sdk")
    except Exception:
        return "0.0.0"

__version__ = _get_version()

# Default AIM Cloud URL
DEFAULT_AIM_URL = "https://aim.opena2a.org"


def print_banner():
    """Print AIM SDK banner."""
    print("""
╔═══════════════════════════════════════════════════════════╗
║                     AIM SDK Login                          ║
║         Agent Identity Management for AI Agents            ║
╚═══════════════════════════════════════════════════════════╝
""")


def login(args):
    """Login to AIM server and save credentials."""
    from .credentials import save_sdk_credentials, load_sdk_credentials, AIM_DIR

    aim_url = args.url.rstrip('/')

    print_banner()
    print(f"Connecting to: {aim_url}")
    print()

    # Check if already logged in
    existing_creds = load_sdk_credentials()
    if existing_creds:
        existing_url = existing_creds.get('aimUrl') or existing_creds.get('aim_url', '')
        if existing_url and not args.force:
            print(f"Already authenticated to: {existing_url}")
            print()
            response = input("Do you want to re-authenticate? [y/N]: ").strip().lower()
            if response != 'y':
                print("Login cancelled.")
                return 0
            print()

    # Get credentials
    print("Enter your AIM credentials:")
    email = input("  Email: ").strip()
    if not email:
        print("Error: Email is required")
        return 1

    password = getpass.getpass("  Password: ")
    if not password:
        print("Error: Password is required")
        return 1

    print()
    print("Authenticating...")

    # Call login endpoint
    try:
        response = requests.post(
            f"{aim_url}/api/v1/public/login",
            json={"email": email, "password": password},
            timeout=30
        )

        if response.status_code == 401:
            print("❌ Authentication failed: Invalid email or password")
            return 1

        if response.status_code != 200:
            error_msg = "Unknown error"
            try:
                error_data = response.json()
                error_msg = error_data.get('error', error_msg)
            except Exception:
                error_msg = response.text
            print(f"❌ Authentication failed: {error_msg}")
            return 1

        data = response.json()

        if not data.get('success'):
            if not data.get('isApproved'):
                print("❌ Your account is pending admin approval.")
                print("   Please wait for an administrator to approve your registration.")
                return 1
            print(f"❌ Authentication failed: {data.get('message', 'Unknown error')}")
            return 1

        # Check if password change is required
        if data.get('requiresPasswordChange'):
            print("⚠️  Password change required.")
            print(f"   Please visit {aim_url} to change your password, then try again.")
            return 1

        access_token = data.get('accessToken')
        refresh_token = data.get('refreshToken')
        user = data.get('user', {})

        if not access_token or not refresh_token:
            print("❌ Authentication failed: No tokens received")
            return 1

        # Save credentials
        credentials = {
            'aimUrl': aim_url,
            'refreshToken': refresh_token,
            'accessToken': access_token,
            'userId': user.get('id'),
            'userEmail': user.get('email'),
            'organizationId': user.get('organizationId'),
        }

        if save_sdk_credentials(credentials):
            print()
            print("✅ Successfully authenticated!")
            print()
            print(f"   User: {user.get('email', 'Unknown')}")
            print(f"   Server: {aim_url}")
            print(f"   Credentials saved to: {AIM_DIR}/sdk_credentials.json")
            print()
            print("You can now use the AIM SDK:")
            print()
            print("   from aim_sdk import secure")
            print("   agent = secure('my-agent', capabilities=['db:read'])")
            print()
            return 0
        else:
            print("❌ Failed to save credentials")
            return 1

    except requests.exceptions.ConnectionError:
        print(f"❌ Connection failed: Could not connect to {aim_url}")
        print()
        print("   Make sure the AIM server is running and accessible.")
        if aim_url == DEFAULT_AIM_URL:
            print("   For self-hosted AIM, use: aim-sdk login --url http://localhost:8080")
        return 1
    except requests.exceptions.Timeout:
        print(f"❌ Connection timeout: Server at {aim_url} did not respond")
        return 1
    except Exception as e:
        print(f"❌ Error: {e}")
        return 1


def logout(args):
    """Logout and clear credentials."""
    from .credentials import load_sdk_credentials, AIM_DIR, SDK_CREDENTIALS_FILE
    from .oauth import OAuthTokenManager

    print("Logging out...")

    # Try to revoke token on server
    try:
        token_manager = OAuthTokenManager()
        if token_manager.has_credentials():
            token_manager.revoke_token()
    except Exception:
        pass

    # Delete local credentials
    creds_file = SDK_CREDENTIALS_FILE
    if creds_file.exists():
        try:
            creds_file.unlink()
            print("✅ Credentials removed successfully")
        except Exception as e:
            print(f"⚠️  Failed to remove credentials file: {e}")
            return 1
    else:
        print("No credentials found - already logged out")

    return 0


def status(args):
    """Check current authentication status."""
    from .credentials import load_sdk_credentials, AIM_DIR
    from .oauth import OAuthTokenManager

    print("Checking authentication status...")
    print()

    creds = load_sdk_credentials()
    if not creds:
        print("❌ Not authenticated")
        print()
        print("   Run 'aim-sdk login' to authenticate")
        return 1

    aim_url = creds.get('aimUrl') or creds.get('aim_url', 'Unknown')
    user_email = creds.get('userEmail', 'Unknown')

    print(f"   Server: {aim_url}")
    print(f"   User: {user_email}")
    print(f"   Credentials: {AIM_DIR}/sdk_credentials.json")
    print()

    # Try to validate token
    try:
        token_manager = OAuthTokenManager()
        token = token_manager.get_access_token(suppress_errors=True)
        if token:
            print("✅ Authenticated (token valid)")
        else:
            print("⚠️  Token may be expired - will refresh on next SDK use")
    except Exception:
        print("⚠️  Could not validate token")

    return 0


def version_cmd(args):
    """Show SDK version."""
    print(f"AIM SDK version {__version__}")
    return 0


def main():
    """Main CLI entry point."""
    parser = argparse.ArgumentParser(
        prog='aim-sdk',
        description='AIM SDK - Agent Identity Management for AI Agents',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  aim-sdk login                           Login to AIM Cloud (aim.opena2a.org)
  aim-sdk login --url http://localhost:8080   Login to self-hosted AIM
  aim-sdk logout                          Clear saved credentials
  aim-sdk status                          Check authentication status

For more information, visit: https://opena2a.org/docs
        """
    )

    parser.add_argument(
        '--version', '-v',
        action='version',
        version=f'%(prog)s {__version__}'
    )

    subparsers = parser.add_subparsers(dest='command', help='Available commands')

    # Login command
    login_parser = subparsers.add_parser(
        'login',
        help='Authenticate with AIM server'
    )
    login_parser.add_argument(
        '--url', '-u',
        default=DEFAULT_AIM_URL,
        help=f'AIM server URL (default: {DEFAULT_AIM_URL})'
    )
    login_parser.add_argument(
        '--force', '-f',
        action='store_true',
        help='Force re-authentication without prompting'
    )
    login_parser.set_defaults(func=login)

    # Logout command
    logout_parser = subparsers.add_parser(
        'logout',
        help='Logout and clear credentials'
    )
    logout_parser.set_defaults(func=logout)

    # Status command
    status_parser = subparsers.add_parser(
        'status',
        help='Check authentication status'
    )
    status_parser.set_defaults(func=status)

    # Version command (alternative to --version)
    version_parser = subparsers.add_parser(
        'version',
        help='Show SDK version'
    )
    version_parser.set_defaults(func=version_cmd)

    args = parser.parse_args()

    if not args.command:
        parser.print_help()
        return 0

    return args.func(args)


if __name__ == '__main__':
    sys.exit(main())
