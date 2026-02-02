"""
AIM SDK Command Line Interface.

Provides commands for authenticating and managing the SDK:
- login: Authenticate with AIM server via browser (opens automatically)
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
import sys
import os
import webbrowser
import threading
import socket
import urllib.parse
from http.server import HTTPServer, BaseHTTPRequestHandler
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


def find_free_port():
    """Find a free port on localhost."""
    with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
        s.bind(('', 0))
        s.listen(1)
        port = s.getsockname()[1]
    return port


class CallbackHandler(BaseHTTPRequestHandler):
    """HTTP handler for OAuth callback."""

    credentials = None
    error = None

    def log_message(self, format, *args):
        """Suppress HTTP logs."""
        pass

    def do_GET(self):
        """Handle callback with tokens."""
        parsed = urllib.parse.urlparse(self.path)
        params = urllib.parse.parse_qs(parsed.query)

        if parsed.path == '/callback':
            # Extract tokens from query params
            access_token = params.get('accessToken', [None])[0]
            refresh_token = params.get('refreshToken', [None])[0]
            user_email = params.get('email', [None])[0]
            user_id = params.get('userId', [None])[0]
            org_id = params.get('organizationId', [None])[0]
            error = params.get('error', [None])[0]

            if error:
                CallbackHandler.error = error
                self._send_error_page(error)
            elif access_token and refresh_token:
                CallbackHandler.credentials = {
                    'accessToken': access_token,
                    'refreshToken': refresh_token,
                    'userEmail': user_email,
                    'userId': user_id,
                    'organizationId': org_id,
                }
                self._send_success_page()
            else:
                CallbackHandler.error = "Missing tokens in callback"
                self._send_error_page("Authentication failed - missing tokens")
        else:
            self.send_response(404)
            self.end_headers()

    def _send_success_page(self):
        """Send success HTML page."""
        html = """<!DOCTYPE html>
<html>
<head>
    <title>AIM SDK - Login Successful</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
               display: flex; justify-content: center; align-items: center; height: 100vh;
               margin: 0; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); }
        .card { background: white; padding: 40px 60px; border-radius: 16px; text-align: center;
                box-shadow: 0 20px 60px rgba(0,0,0,0.3); }
        h1 { color: #22c55e; margin-bottom: 10px; }
        p { color: #666; margin: 10px 0; }
        .check { font-size: 64px; margin-bottom: 20px; }
        code { background: #f3f4f6; padding: 2px 8px; border-radius: 4px; }
    </style>
</head>
<body>
    <div class="card">
        <div class="check">✅</div>
        <h1>Login Successful!</h1>
        <p>You can close this tab and return to your terminal.</p>
        <p style="margin-top: 20px; color: #888; font-size: 14px;">
            Credentials saved. Run <code>aim-sdk status</code> to verify.
        </p>
    </div>
</body>
</html>"""
        self.send_response(200)
        self.send_header('Content-Type', 'text/html')
        self.end_headers()
        self.wfile.write(html.encode())

    def _send_error_page(self, error):
        """Send error HTML page."""
        html = f"""<!DOCTYPE html>
<html>
<head>
    <title>AIM SDK - Login Failed</title>
    <style>
        body {{ font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
               display: flex; justify-content: center; align-items: center; height: 100vh;
               margin: 0; background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); }}
        .card {{ background: white; padding: 40px 60px; border-radius: 16px; text-align: center;
                box-shadow: 0 20px 60px rgba(0,0,0,0.3); }}
        h1 {{ color: #ef4444; margin-bottom: 10px; }}
        p {{ color: #666; margin: 10px 0; }}
        .icon {{ font-size: 64px; margin-bottom: 20px; }}
    </style>
</head>
<body>
    <div class="card">
        <div class="icon">❌</div>
        <h1>Login Failed</h1>
        <p>{error}</p>
        <p style="margin-top: 20px; color: #888; font-size: 14px;">
            Please try again with <code>aim-sdk login</code>
        </p>
    </div>
</body>
</html>"""
        self.send_response(200)
        self.send_header('Content-Type', 'text/html')
        self.end_headers()
        self.wfile.write(html.encode())


def login(args):
    """Login to AIM server via browser."""
    from .credentials import save_sdk_credentials, load_sdk_credentials, AIM_DIR

    aim_url = args.url.rstrip('/')

    print_banner()
    print(f"Server: {aim_url}")
    print()

    # Check if already logged in
    existing_creds = load_sdk_credentials()
    if existing_creds and not args.force:
        existing_url = existing_creds.get('aimUrl') or existing_creds.get('aim_url', '')
        user_email = existing_creds.get('userEmail', 'Unknown')
        if existing_url:
            print(f"Already authenticated as: {user_email}")
            print(f"Server: {existing_url}")
            print()
            response = input("Re-authenticate? [y/N]: ").strip().lower()
            if response != 'y':
                print("Login cancelled.")
                return 0
            print()

    # Start local callback server
    port = find_free_port()
    callback_url = f"http://localhost:{port}/callback"

    # Reset handler state
    CallbackHandler.credentials = None
    CallbackHandler.error = None

    server = HTTPServer(('localhost', port), CallbackHandler)
    server.timeout = 120  # 2 minute timeout

    # Build login URL with callback
    login_url = f"{aim_url}/cli-auth?callback={urllib.parse.quote(callback_url)}"

    print("Opening browser for authentication...")
    print()
    print(f"If the browser doesn't open, visit:")
    print(f"  {login_url}")
    print()
    print("Waiting for authentication...")

    # Open browser
    webbrowser.open(login_url)

    # Wait for callback (with timeout)
    try:
        while CallbackHandler.credentials is None and CallbackHandler.error is None:
            server.handle_request()
    except KeyboardInterrupt:
        print("\n\nLogin cancelled.")
        return 1
    finally:
        server.server_close()

    if CallbackHandler.error:
        print(f"\n❌ Authentication failed: {CallbackHandler.error}")
        return 1

    if not CallbackHandler.credentials:
        print("\n❌ Authentication failed: No credentials received")
        return 1

    # Save credentials
    credentials = {
        'aimUrl': aim_url,
        'refreshToken': CallbackHandler.credentials.get('refreshToken'),
        'accessToken': CallbackHandler.credentials.get('accessToken'),
        'userId': CallbackHandler.credentials.get('userId'),
        'userEmail': CallbackHandler.credentials.get('userEmail'),
        'organizationId': CallbackHandler.credentials.get('organizationId'),
    }

    if save_sdk_credentials(credentials):
        print()
        print("✅ Successfully authenticated!")
        print()
        print(f"   User: {credentials.get('userEmail', 'Unknown')}")
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
