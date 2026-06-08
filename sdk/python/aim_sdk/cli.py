"""
AIM SDK Command Line Interface.

Provides commands for authenticating and managing the SDK:
- login: Authenticate with AIM server via browser (OAuth 2.0 + PKCE)
- logout: Revoke credentials and clear local storage
- status: Check current authentication status
- version: Show SDK version

Usage:
    aim-sdk login                    # Login to AIM Cloud (aim.opena2a.org)
    aim-sdk login --url http://localhost:8080  # Login to self-hosted
    aim-sdk logout                   # Clear credentials
    aim-sdk status                   # Check authentication status

Security Design (RFC 8252 - OAuth for Native Apps):
- Uses Authorization Code flow with PKCE (Proof Key for Code Exchange)
- Browser redirects directly to localhost (no cross-origin requests)
- Short-lived authorization code exchanged for tokens server-side
- State parameter prevents CSRF attacks
"""

import argparse
import sys
import os
import webbrowser
import socket
import secrets
import hashlib
import base64
import json
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


def generate_pkce_pair():
    """
    Generate PKCE code_verifier and code_challenge.

    Returns:
        tuple: (code_verifier, code_challenge)
    """
    # Generate a cryptographically random code_verifier (43-128 chars)
    code_verifier = secrets.token_urlsafe(64)

    # Generate code_challenge = BASE64URL(SHA256(code_verifier))
    code_challenge = base64.urlsafe_b64encode(
        hashlib.sha256(code_verifier.encode('ascii')).digest()
    ).decode('ascii').rstrip('=')

    return code_verifier, code_challenge


# --- OAuth callback page (matches the AIM dashboard design language) ---------
# Blue-600 primary (#2563eb), white card on gray-50, Inter, gray-900/gray-600 text,
# the real OpenA2A logo, lucide-style inline SVG status icons. No emoji, no purple
# gradient (org UI standards). Fully self-contained: the logo is an inlined data URI
# and the Inter webfont falls back to the system stack offline.

from ._branding import LOGO_DATA_URI

_LOGO = f'<img src="{LOGO_DATA_URI}" alt="OpenA2A" width="56" height="56">'

_ICON_CHECK = (
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" '
    'stroke-linecap="round" stroke-linejoin="round" width="28" height="28">'
    '<path d="M20 6 9 17l-5-5"/></svg>'
)

_ICON_ALERT = (
    '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" '
    'stroke-linecap="round" stroke-linejoin="round" width="28" height="28">'
    '<circle cx="12" cy="12" r="10"/><path d="M12 8v4"/><path d="M12 16h.01"/></svg>'
)

_CALLBACK_PAGE = """<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>AIM SDK - {title}</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&display=swap" rel="stylesheet">
<style>
  :root {{ --blue: #2563eb; --ink: #111827; --muted: #4b5563; --border: #e5e7eb; }}
  * {{ box-sizing: border-box; }}
  body {{ font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
          margin: 0; min-height: 100vh; display: flex; align-items: center; justify-content: center;
          background: #f9fafb; color: var(--ink); padding: 24px; }}
  .card {{ width: 100%; max-width: 420px; background: #fff; border: 1px solid var(--border);
           border-radius: 16px; box-shadow: 0 10px 30px rgba(17,24,39,0.08); padding: 40px 36px;
           text-align: center; }}
  .brand {{ display: flex; align-items: center; justify-content: center; margin-bottom: 28px; }}
  .brand img {{ display: block; }}
  .status {{ width: 56px; height: 56px; border-radius: 999px; display: inline-flex;
             align-items: center; justify-content: center; margin-bottom: 18px;
             background: {accent_soft}; border: 1px solid {accent_border}; color: {accent}; }}
  h1 {{ font-size: 20px; font-weight: 600; margin: 0 0 8px; color: var(--ink); }}
  p {{ font-size: 14px; line-height: 1.55; color: var(--muted); margin: 6px 0; }}
  .hint {{ font-size: 13px; color: #6b7280; margin-top: 18px; }}
  code {{ font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 12.5px;
          background: #f3f4f6; border: 1px solid var(--border); border-radius: 6px; padding: 2px 7px; color: var(--ink); }}
</style>
</head>
<body>
  <div class="card">
    <div class="brand">{logo}</div>
    <div class="status">{icon}</div>
    <h1>{heading}</h1>
    <p>{message}</p>
    <p class="hint">{hint}</p>
  </div>
</body>
</html>"""


class PKCECallbackHandler(BaseHTTPRequestHandler):
    """
    HTTP handler for OAuth PKCE callback.

    Receives authorization code via redirect (not POST).
    """

    authorization_code = None
    error = None
    expected_state = None

    def log_message(self, format, *args):
        """Suppress HTTP logs."""
        pass

    def do_GET(self):
        """Handle OAuth callback redirect."""
        parsed = urllib.parse.urlparse(self.path)
        params = urllib.parse.parse_qs(parsed.query)

        if parsed.path == '/callback':
            # Check for error
            if 'error' in params:
                PKCECallbackHandler.error = params.get('error', ['Unknown error'])[0]
                self._send_error_page(PKCECallbackHandler.error)
                return

            # Validate state (CSRF protection)
            received_state = params.get('state', [None])[0]
            if not received_state or received_state != PKCECallbackHandler.expected_state:
                PKCECallbackHandler.error = "Invalid state parameter - possible CSRF attack"
                self._send_error_page("Security validation failed")
                return

            # Extract authorization code
            code = params.get('code', [None])[0]
            if not code:
                PKCECallbackHandler.error = "No authorization code received"
                self._send_error_page("No authorization code received")
                return

            # Store code for exchange
            PKCECallbackHandler.authorization_code = code
            self._send_success_page()
        else:
            self.send_response(404)
            self.end_headers()

    def _send_success_page(self):
        """Send success HTML page (matches the AIM dashboard design language)."""
        html = _CALLBACK_PAGE.format(
            title="Login successful",
            logo=_LOGO,
            accent="#10b981",
            accent_soft="#ecfdf5",
            accent_border="#a7f3d0",
            icon=_ICON_CHECK,
            heading="Login successful",
            message="You can close this tab and return to your terminal.",
            hint="Credentials saved. Run <code>aim-sdk status</code> to verify.",
        )
        self.send_response(200)
        self.send_header('Content-Type', 'text/html; charset=utf-8')
        self.end_headers()
        self.wfile.write(html.encode())

    def _send_error_page(self, error: str):
        """Send error HTML page (matches the AIM dashboard design language)."""
        safe_error = error.replace('&', '&amp;').replace('<', '&lt;').replace('>', '&gt;').replace('"', '&quot;')
        html = _CALLBACK_PAGE.format(
            title="Login failed",
            logo=_LOGO,
            accent="#dc2626",
            accent_soft="#fef2f2",
            accent_border="#fecaca",
            icon=_ICON_ALERT,
            heading="Login failed",
            message=safe_error,
            hint="Please try again with <code>aim-sdk login</code>",
        )
        self.send_response(200)
        self.send_header('Content-Type', 'text/html; charset=utf-8')
        self.end_headers()
        self.wfile.write(html.encode())


def exchange_code_for_tokens(aim_url: str, code: str, code_verifier: str, redirect_uri: str):
    """
    Exchange authorization code for tokens using PKCE.

    Args:
        aim_url: AIM server URL
        code: Authorization code from callback
        code_verifier: PKCE code verifier
        redirect_uri: The redirect URI used in authorization

    Returns:
        dict: Token response or None on error
    """
    token_url = f"{aim_url}/api/v1/auth/token"

    try:
        response = requests.post(
            token_url,
            json={
                'grant_type': 'authorization_code',
                'code': code,
                'code_verifier': code_verifier,
                'redirect_uri': redirect_uri,
            },
            headers={'Content-Type': 'application/json'},
            timeout=30,
        )

        if response.status_code == 200:
            return response.json()
        else:
            # Return a BARE reason — the caller prefixes "Token exchange failed: ".
            # (Previously this returned a pre-prefixed string, so login() printed
            # "Token exchange failed: Token exchange failed: 405".) Prefer the
            # server's human-readable error_description, then error, then a status
            # line that hints at the most common misconfiguration: pointing the SDK
            # at the dashboard/frontend URL instead of the AIM API base.
            try:
                error_data = response.json() if response.content else {}
            except ValueError:
                error_data = {}
            reason = error_data.get('error_description') or error_data.get('error')
            if not reason:
                reason = (
                    f"HTTP {response.status_code} from {token_url} — verify the "
                    f"server URL is the AIM API base (e.g. https://api.aim.opena2a.org), "
                    f"not the dashboard URL"
                )
            return {'error': reason}
    except requests.RequestException as e:
        return {'error': f'Network error: {str(e)}'}


def login(args):
    """Login to AIM server via browser with OAuth 2.0 + PKCE."""
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

    # Generate PKCE pair
    code_verifier, code_challenge = generate_pkce_pair()

    # Generate state for CSRF protection
    state = secrets.token_urlsafe(32)

    # Start local callback server
    port = find_free_port()
    redirect_uri = f"http://localhost:{port}/callback"

    # Reset handler state
    PKCECallbackHandler.authorization_code = None
    PKCECallbackHandler.error = None
    PKCECallbackHandler.expected_state = state

    server = HTTPServer(('localhost', port), PKCECallbackHandler)
    server.timeout = 120  # 2 minute timeout

    # Build authorization URL with PKCE parameters
    params = urllib.parse.urlencode({
        'response_type': 'code',
        'redirect_uri': redirect_uri,
        'state': state,
        'code_challenge': code_challenge,
        'code_challenge_method': 'S256',
    })
    login_url = f"{aim_url}/auth/login?{params}"

    print("Opening browser for authentication...")
    print()
    print(f"If the browser doesn't open, visit:")
    print(f"  {login_url}")
    print()
    print("Waiting for authentication... (Ctrl+C to cancel)")

    # Open browser
    webbrowser.open(login_url)

    # Wait for callback (with timeout)
    try:
        while PKCECallbackHandler.authorization_code is None and PKCECallbackHandler.error is None:
            server.handle_request()
    except KeyboardInterrupt:
        print("\n\nLogin cancelled.")
        return 1
    finally:
        server.server_close()

    if PKCECallbackHandler.error:
        print(f"\nAuthentication failed: {PKCECallbackHandler.error}")
        return 1

    if not PKCECallbackHandler.authorization_code:
        print("\nAuthentication failed: No authorization code received")
        return 1

    # Exchange authorization code for tokens
    print("\nExchanging authorization code for tokens...")
    token_response = exchange_code_for_tokens(
        aim_url,
        PKCECallbackHandler.authorization_code,
        code_verifier,
        redirect_uri,
    )

    if 'error' in token_response:
        print(f"\nToken exchange failed: {token_response['error']}")
        return 1

    # Save credentials
    credentials = {
        'aimUrl': aim_url,
        'refreshToken': token_response.get('refresh_token'),
        'accessToken': token_response.get('access_token'),
        'userId': token_response.get('user_id'),
        'userEmail': token_response.get('email'),
        'organizationId': token_response.get('organization_id'),
    }

    if save_sdk_credentials(credentials):
        print()
        print("Successfully authenticated.")
        print()
        print(f"   User: {credentials.get('userEmail', 'Unknown')}")
        print(f"   Server: {aim_url}")
        print(f"   Credentials saved to: {AIM_DIR}/sdk_credentials.json")
        print()
        print("You can now use the AIM SDK. Register an agent, then protect any")
        print("function with a capability grant:")
        print()
        print("   from aim_sdk import secure")
        print()
        print("   agent = secure('my-agent', capabilities=['db:read'])")
        print()
        print("   @agent.perform_action(capability='db:read')")
        print("   def get_customer(customer_id):")
        print("       return db.query('SELECT * FROM customers WHERE id = ?', customer_id)")
        print()
        print("Every @perform_action call is signed, authorized by the server's 5-step")
        print("Fine-Grained Authorization, and recorded in the audit log. Calls outside")
        print("the granted capabilities are denied at the tool-call boundary.")
        print()
        print("   Verify auth:   aim-sdk status")
        print("   Quickstart:    https://opena2a.org/docs/tutorials/sdk-quickstart")
        print("   Full docs:     https://opena2a.org/docs/aim")
        print()
        return 0
    else:
        print("Failed to save credentials.")
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
        pass  # Ignore revocation errors

    # Delete local credentials
    creds_file = Path(AIM_DIR) / "sdk_credentials.json"
    if creds_file.exists():
        creds_file.unlink()
        print("Credentials cleared.")
    else:
        print("No credentials to clear")

    return 0


def status(args):
    """Check authentication status."""
    from .credentials import load_sdk_credentials, AIM_DIR

    print("Checking authentication status...")
    print()

    creds = load_sdk_credentials()
    if not creds:
        print("Not authenticated.")
        print()
        print("Run 'aim-sdk login' to authenticate")
        return 1

    aim_url = creds.get('aimUrl') or creds.get('aim_url', 'Unknown')
    user_email = creds.get('userEmail', 'Unknown')
    creds_file = Path(AIM_DIR) / "sdk_credentials.json"

    print(f"   Server: {aim_url}")
    print(f"   User: {user_email}")
    print(f"   Credentials: {creds_file}")
    print()

    # Check token validity
    access_token = creds.get('accessToken')
    if access_token:
        # Try to decode JWT to check expiry (without verification)
        try:
            import base64
            parts = access_token.split('.')
            if len(parts) == 3:
                # Decode payload
                payload = parts[1]
                # Add padding if needed
                payload += '=' * (4 - len(payload) % 4)
                decoded = base64.urlsafe_b64decode(payload)
                import json
                data = json.loads(decoded)
                exp = data.get('exp')
                if exp:
                    import time
                    if exp > time.time():
                        print("Token is valid.")
                    else:
                        print("Token may be expired; it will refresh on next SDK use.")
        except Exception:
            print("Could not verify token status.")

    return 0


def version_cmd(args):
    """Show SDK version."""
    print(f"aim-sdk {__version__}")
    return 0


def main():
    """Main CLI entry point."""
    parser = argparse.ArgumentParser(
        prog='aim-sdk',
        description='AIM SDK - Agent Identity Management CLI',
    )
    parser.add_argument(
        '--version', '-V',
        action='version',
        version=f'aim-sdk {__version__}',
        help='Show SDK version and exit',
    )
    subparsers = parser.add_subparsers(dest='command', help='Commands')

    # Login command
    login_parser = subparsers.add_parser('login', help='Login to AIM server')
    login_parser.add_argument(
        '--url',
        default=DEFAULT_AIM_URL,
        help=f'AIM server URL (default: {DEFAULT_AIM_URL})',
    )
    login_parser.add_argument(
        '--force', '-f',
        action='store_true',
        help='Force re-authentication even if already logged in',
    )
    login_parser.set_defaults(func=login)

    # Logout command
    logout_parser = subparsers.add_parser('logout', help='Logout and clear credentials')
    logout_parser.set_defaults(func=logout)

    # Status command
    status_parser = subparsers.add_parser('status', help='Check authentication status')
    status_parser.set_defaults(func=status)

    # Version command
    version_parser = subparsers.add_parser('version', help='Show SDK version')
    version_parser.set_defaults(func=version_cmd)

    args = parser.parse_args()

    if not args.command:
        parser.print_help()
        return 1

    return args.func(args)


if __name__ == '__main__':
    sys.exit(main())
