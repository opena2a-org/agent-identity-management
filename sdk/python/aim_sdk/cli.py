"""
AIM SDK Command Line Interface.

Provides commands for authenticating and managing the SDK:
- login: Authenticate with an AIM server (OAuth 2.0 device grant, RFC 8628)
- logout: Revoke credentials and clear local storage
- status: Check current authentication status
- version: Show SDK version
- help: Show the command list

Usage:
    aim-sdk login                    # Login to AIM Cloud (aim.opena2a.org)
    aim-sdk login --url http://localhost:8080  # Login to self-hosted
    aim-sdk logout                   # Clear credentials
    aim-sdk status                   # Check authentication status
    aim-sdk status --json            # ... as one JSON object, for scripts
    aim-sdk version --json           # {"version": "..."}
    aim-sdk help                     # Same as `aim-sdk` with no arguments

Login design (RFC 8628, OAuth 2.0 Device Authorization Grant):
- The CLI asks the server for a device code and a short user code
- It prints the user code and opens the dashboard's /device page
- The user signs in to the dashboard and approves the code there
- The CLI polls the server at the interval it was given, never faster, and
  stores the same access/refresh token pair the dashboard login issues
- No local HTTP server, no redirect URI, no browser callback: the device code
  never leaves this process and the user code alone authorizes nothing
"""

import argparse
import sys
import os
import time
import webbrowser
import base64
import json
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

# Bounds for `aim-sdk login`. The pre-flight probe keeps a dead --url from
# opening a browser at all. The poll loop is bounded by the lifetime the server
# gives the device code (expiresIn), capped locally at LOGIN_MAX_WAIT_SECONDS
# so a server answer cannot make the CLI wait longer than that.
LOGIN_PROBE_TIMEOUT_SECONDS = 5
LOGIN_MAX_WAIT_SECONDS = 900
DEVICE_CLIENT_ID = "aim-sdk"
DEVICE_GRANT_TYPE = "urn:ietf:params:oauth:grant-type:device_code"


def check_server_reachable(aim_url, timeout):
    """
    Pre-flight probe: is anything answering HTTP at aim_url?

    Any HTTP response -- including an error status -- proves the server is
    reachable; only a transport failure (refused, unroutable, timed out)
    counts as unreachable.
    """
    try:
        requests.get(aim_url, timeout=timeout, allow_redirects=False)
        return True
    except requests.RequestException:
        return False


def print_banner():
    """Print AIM SDK banner. Every line is 61 columns wide so the ║ borders align."""
    print("""
╔═══════════════════════════════════════════════════════════╗
║                       AIM SDK Login                       ║
║          Agent Identity Management for AI Agents          ║
╚═══════════════════════════════════════════════════════════╝
""")


def _server_reason(response, fallback):
    """The server's human-readable reason for a non-success answer, bare (the
    caller decides the prefix), or `fallback` when the body carries none."""
    try:
        data = response.json() if response.content else {}
    except ValueError:
        data = {}
    if not isinstance(data, dict):
        data = {}
    return data.get('errorDescription') or data.get('error_description') or data.get('error') or fallback


def request_device_code(aim_url: str, client_id: str = DEVICE_CLIENT_ID):
    """
    Start the device grant: POST /api/v1/oauth/device/code.

    Returns:
        dict: the server's answer (deviceCode, userCode, verificationUri,
        verificationUriComplete, expiresIn, interval), or {'error': reason}
        with a bare reason the caller prefixes.
    """
    url = f"{aim_url}/api/v1/oauth/device/code"
    try:
        response = requests.post(
            url,
            json={'clientId': client_id},
            headers={'Content-Type': 'application/json'},
            timeout=30,
        )
    except requests.RequestException as e:
        # The same rendering the SDK uses everywhere else: the host from the
        # server URL, the failure class, the URL to check; never the urllib3
        # pool chain.
        from .client import _request_failure_message

        return {'error': _request_failure_message(e, aim_url)}

    if response.status_code != 200:
        return {'error': _server_reason(
            response,
            f"HTTP {response.status_code} from {url} — verify the server URL is the AIM API "
            f"base (e.g. https://api.aim.opena2a.org), not the dashboard URL",
        )}
    try:
        data = response.json()
    except ValueError:
        data = None
    if not isinstance(data, dict) or not data.get('deviceCode') or not data.get('verificationUri'):
        return {'error': f"the device authorization answer from {url} carried no device code or "
                         f"verification URI (the SDK requires both)"}
    return data


def poll_device_token(aim_url: str, device_code: str, interval: int, deadline: float):
    """
    Poll POST /api/v1/oauth/device/token until the user approves, the code
    expires or is denied, or the deadline passes.

    Never polls faster than `interval`; a `slow_down` answer or an HTTP 429
    adds five seconds to the wait (RFC 8628 section 3.5).

    Returns:
        dict: the token pair on success, or {'error': <one of 'expired_token',
        'access_denied', 'timeout', or a bare reason>}.
    """
    url = f"{aim_url}/api/v1/oauth/device/token"
    wait = max(int(interval or 5), 1)
    while True:
        if time.monotonic() >= deadline:
            return {'error': 'timeout'}
        time.sleep(wait)
        try:
            response = requests.post(
                url,
                json={'deviceCode': device_code, 'grantType': DEVICE_GRANT_TYPE},
                headers={'Content-Type': 'application/json'},
                timeout=30,
            )
        except requests.RequestException as e:
            from .client import _request_failure_message

            return {'error': _request_failure_message(e, aim_url)}

        if response.status_code == 200:
            try:
                data = response.json()
            except ValueError:
                data = None
            if not isinstance(data, dict) or not data.get('accessToken'):
                return {'error': f"the token answer from {url} carried no access token"}
            return data

        code = _server_reason(response, '')
        if response.status_code == 429 or code == 'slow_down':
            wait += 5
            continue
        if code == 'authorization_pending':
            continue
        if code in ('expired_token', 'access_denied'):
            return {'error': code}
        return {'error': code or f"HTTP {response.status_code} from {url}"}


def login(args):
    """Login to an AIM server with the OAuth 2.0 device grant (RFC 8628)."""
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
            try:
                response = input("Re-authenticate? [y/N]: ").strip().lower()
            except (EOFError, KeyboardInterrupt):
                # Non-interactive stdin (CI / piped) or Ctrl-D/Ctrl-C: don't
                # crash with a raw traceback — keep the existing credentials and
                # tell the user how to force a re-auth non-interactively.
                print("\nNo input received; keeping existing credentials. "
                      "Pass --force to re-authenticate non-interactively.")
                return 0
            if response != 'y':
                print("Login cancelled.")
                return 0
            print()

    # Fail fast on an unreachable server: probing before the browser opens is
    # what keeps a mistyped or dead --url from parking the user on a login
    # page that will never call back.
    if not check_server_reachable(aim_url, LOGIN_PROBE_TIMEOUT_SECONDS):
        print(f"Error: could not reach the AIM server at {aim_url}")
        print(f"(no HTTP response within {LOGIN_PROBE_TIMEOUT_SECONDS}s).")
        print("Check the URL and your network, then retry. For a self-hosted")
        print("server, pass it explicitly: aim-sdk login --url <your-aim-url>")
        return 1

    # Start the device grant. The device code stays in this process; only the
    # short user code is shown, and it authorizes nothing by itself.
    device = request_device_code(aim_url)
    if 'error' in device:
        print(f"\nCould not start the device login: {device['error']}")
        return 1

    user_code = device.get('userCode', '')
    verification_uri = device.get('verificationUri', '')
    verification_uri_complete = device.get('verificationUriComplete') or verification_uri
    if not (verification_uri.startswith('http://') or verification_uri.startswith('https://')):
        print(f"\nCould not start the device login: the server sent a verification URI that is not "
              f"an http(s) URL ({verification_uri!r}); check the server's FRONTEND_URL.")
        return 1
    interval = device.get('interval') or 5
    try:
        expires_in = int(device.get('expiresIn') or LOGIN_MAX_WAIT_SECONDS)
    except (TypeError, ValueError):
        expires_in = LOGIN_MAX_WAIT_SECONDS
    max_wait = max(1, min(expires_in, LOGIN_MAX_WAIT_SECONDS))

    print("To sign in, open this page in your browser and approve the code:")
    print()
    print(f"  {verification_uri}")
    print()
    print(f"  Code: {user_code}")
    print()
    print(f"Waiting for approval... (the code expires in {max_wait}s; Ctrl+C to cancel)")

    # Open the browser on the page with the code filled in; the page still
    # requires a signed-in user and one explicit click.
    try:
        webbrowser.open(verification_uri_complete)
    except Exception:
        pass

    deadline = time.monotonic() + max_wait
    try:
        token_response = poll_device_token(aim_url, device['deviceCode'], interval, deadline)
    except KeyboardInterrupt:
        print("\n\nLogin cancelled.")
        return 1

    if 'error' in token_response:
        reason = token_response['error']
        if reason in ('expired_token', 'timeout'):
            print(f"\nThe code expired before it was approved (waited {max_wait}s; login timed out). "
                  f"Run aim-sdk login to get a new code.")
        elif reason == 'access_denied':
            print("\nThe login was denied in the dashboard. Nothing was stored.")
        else:
            print(f"\nLogin failed: {reason}")
        return 1

    # The pair is the one the dashboard login issues; the user fields come
    # from the access token's claims, read without verifying the signature
    # (the server verifies on every call).
    from .oauth import decode_jwt_claims

    claims = decode_jwt_claims(token_response.get('accessToken', '')) or {}
    credentials = {
        'aimUrl': aim_url,
        'refreshToken': token_response.get('refreshToken'),
        'accessToken': token_response.get('accessToken'),
        'userId': claims.get('user_id'),
        'userEmail': claims.get('email'),
        'organizationId': claims.get('organization_id'),
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


def _token_state(access_token) -> str:
    """
    Classify the stored access token without verifying its signature.

    One classifier for both renderings, so `--json` and the human output can
    never disagree about the same credentials file.

    Returns one of: "absent" (no token stored), "valid", "expired", "unknown"
    (a token is stored but its expiry could not be read).
    """
    if not access_token:
        return "absent"
    try:
        parts = access_token.split('.')
        if len(parts) != 3:
            return "unknown"
        payload = parts[1]
        payload += '=' * (-len(payload) % 4)
        exp = json.loads(base64.urlsafe_b64decode(payload)).get('exp')
        if not exp:
            return "unknown"
        return "valid" if exp > time.time() else "expired"
    except Exception:
        return "unknown"


def status(args):
    """Check authentication status."""
    from .credentials import load_sdk_credentials, AIM_DIR

    creds = load_sdk_credentials()
    creds_file = Path(AIM_DIR) / "sdk_credentials.json"

    if not creds:
        if getattr(args, 'json', False):
            # Exactly one JSON object on stdout and nothing else -- this is the
            # output a wrapper script parses, so a stray banner line would make
            # `aim-sdk status --json | jq` fail on a working install.
            print(json.dumps({
                "authenticated": False,
                "server": None,
                "user": None,
                "credentialsPath": str(creds_file),
                "tokenState": "absent",
            }))
            return 1
        print("Checking authentication status...")
        print()
        print("Not authenticated.")
        print()
        print("Run 'aim-sdk login' to authenticate")
        return 1

    aim_url = creds.get('aimUrl') or creds.get('aim_url', 'Unknown')
    user_email = creds.get('userEmail', 'Unknown')
    token_state = _token_state(creds.get('accessToken'))

    if getattr(args, 'json', False):
        print(json.dumps({
            "authenticated": True,
            "server": aim_url,
            "user": user_email,
            "credentialsPath": str(creds_file),
            "tokenState": token_state,
        }))
        return 0

    print("Checking authentication status...")
    print()
    print(f"   Server: {aim_url}")
    print(f"   User: {user_email}")
    print(f"   Credentials: {creds_file}")
    print()

    if token_state == "valid":
        print("Token is valid.")
    elif token_state == "expired":
        print("Token may be expired; it will refresh on next SDK use.")
    elif token_state == "unknown":
        print("Could not verify token status.")

    return 0


def version_cmd(args):
    """Show SDK version."""
    if getattr(args, 'json', False):
        print(json.dumps({"version": __version__}))
        return 0
    print(f"aim-sdk {__version__}")
    return 0


def demo(args):
    """Run the demo agent (see aim_sdk.demo)."""
    from .demo import run as run_demo

    return run_demo(
        interactive=args.interactive,
        ci=args.ci,
        cleanup=args.cleanup,
        url=args.url,
    )


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
    status_parser.add_argument(
        '--json',
        action='store_true',
        help='Print one JSON object instead of human-readable text',
    )
    status_parser.set_defaults(func=status)

    # Version command
    version_parser = subparsers.add_parser('version', help='Show SDK version')
    version_parser.add_argument(
        '--json',
        action='store_true',
        help='Print one JSON object instead of human-readable text',
    )
    version_parser.set_defaults(func=version_cmd)

    # Help command. `aim-sdk help` is what people type; argparse rejected it as
    # an invalid choice and printed the usage line to stderr with exit 2.
    subparsers.add_parser('help', help='Show this help message')

    # Demo command
    demo_parser = subparsers.add_parser(
        'demo',
        help='Register a demo agent and watch your dashboard come alive',
    )
    demo_parser.add_argument(
        '--interactive',
        action='store_true',
        help='Open the full interactive menu (security demos, JIT, MCP)',
    )
    demo_parser.add_argument(
        '--ci',
        action='store_true',
        help='Non-interactive, no delays (for scripted runs)',
    )
    demo_parser.add_argument(
        '--cleanup',
        action='store_true',
        help='Delete the demo agent again',
    )
    demo_parser.add_argument(
        '--url',
        default=None,
        help='AIM server URL (default: the URL you logged in to)',
    )
    demo_parser.set_defaults(func=demo)

    args = parser.parse_args()

    # Asking for help, by either spelling, is a successful invocation. Exiting
    # 1 after printing the help made `aim-sdk` fail every CI step that ran it
    # to check the tool was installed, and made `aim-sdk help` (rejected by
    # argparse) exit 2 with only a usage line.
    if not args.command or args.command == 'help':
        parser.print_help()
        return 0

    return args.func(args)


if __name__ == '__main__':
    sys.exit(main())
