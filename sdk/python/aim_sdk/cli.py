"""
AIM SDK Command Line Interface.

Provides commands for authenticating and managing the SDK:
- login: Authenticate with an AIM server (OAuth 2.0 device grant, RFC 8628)
- init: Register a first agent with a bootstrap token from the onboarding screen
- logout: Revoke credentials and clear local storage
- status: Check the SDK sign-in and list the agents saved on this machine
- version: Show SDK version
- help: Show the command list

Usage:
    aim-sdk login                    # Login to AIM Cloud (aim.opena2a.org)
    aim-sdk login --url http://localhost:8080  # Login to self-hosted
    AIM_BOOTSTRAP_TOKEN=aim_ob_... aim-sdk init   # Register my-first-agent
    aim-sdk init --url http://localhost:8080 --name my-agent  # ... on self-hosted
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
- An unavailable server (HTTP 502, 503 or 504, or no HTTP answer) ends the
  login at that request with exit status 75: nothing is retried, slept on or
  stored, so the same command can be run again later
"""

import argparse
import re
import stat
import sys
import os
import time
import webbrowser
import base64
import json
from datetime import datetime, timezone
from pathlib import Path
from urllib.parse import urlsplit

# requests is imported inside the functions that call it, not here: it is the
# heaviest import on the CLI's path, and `aim-sdk --help`, `version` and `status`
# never touch the network.
def __getattr__(name):
    """Keep ``aim_sdk.cli.requests`` resolvable for callers that patch it."""
    if name == "requests":
        import requests
        return requests
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")


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

# Exit status of `aim-sdk login` when the AIM server was unavailable: it
# answered 502, 503 or 504, or gave no HTTP answer. This is EX_TEMPFAIL from
# sysexits.h. It is kept apart from 1 so a script can tell "run the same
# command again later" from a login that was denied, expired or refused.
EXIT_SERVER_UNAVAILABLE = 75
SERVER_UNAVAILABLE_STATUSES = (502, 503, 504)

# `aim-sdk init`: the bootstrap token minted on the dashboard's onboarding
# screen registers exactly one agent. The environment variable is the only
# channel because an argument is visible in the process list.
BOOTSTRAP_TOKEN_ENV = "AIM_BOOTSTRAP_TOKEN"
# Why `aim-sdk init --token` is refused; the value is never read. The same
# sentence as `npx @opena2a/aim-sdk init`'s refusal.
TOKEN_FLAG_REFUSED = (f"the bootstrap token is read from {BOOTSTRAP_TOKEN_ENV}, not from a flag "
                      "(a flag value is visible in the process list and shell history)")
BOOTSTRAP_TOKEN_PREFIX = "aim_ob_"
BOOTSTRAP_TOKEN_HEADER = "X-AIM-Bootstrap-Token"
DEFAULT_INIT_AGENT_NAME = "my-first-agent"
INIT_REQUEST_TIMEOUT_SECONDS = 30
# The hosts `aim-sdk init` and `aim-sdk login` reach over http:, as
# urlsplit().hostname gives them. The host is compared as written in the URL
# and is never resolved; any other host needs an https: address, or
# `aim-sdk login --allow-http`.
LOOPBACK_HOSTS = frozenset({"localhost", "127.0.0.1", "::1"})

# An agent or organization ID `init` saves and prints. The server sends a UUID;
# the pattern admits that and other short IDs, and no character that changes
# how a terminal line or a dashboard path reads.
_INIT_ANSWER_ID_PATTERN = re.compile(r'[A-Za-z0-9][A-Za-z0-9_-]{0,127}')
# An agent status `init` saves and prints; any other value is 'unknown'.
_INIT_ANSWER_STATUS_PATTERN = re.compile(r'[a-z][a-z_-]{0,31}')

# Server text printed inside one line of the CLI's own text: every C0
# character (newline, carriage return and tab included), DEL, C1, the
# bidirectional formatting characters and a lone surrogate (which has no UTF-8
# form) are each replaced with U+FFFD, and text over the cap is cut with a
# marker. The TypeScript SDK's `sanitizeTerminalLine` replaces the same set.
TERMINAL_LINE_MAX_CHARS = 500
_NOT_IN_A_LINE = re.compile(r'[\x00-\x1f\x7f-\x9f\u061c\u200e\u200f\u202a-\u202e\u2066-\u2069\ud800-\udfff]')


def _terminal_line(text, max_chars=TERMINAL_LINE_MAX_CHARS):
    """`text` as one terminal line: see _NOT_IN_A_LINE."""
    text = _NOT_IN_A_LINE.sub('�', str(text))
    if len(text) > max_chars:
        text = text[:max_chars] + ' [truncated]'
    return text


def _json_type_name(value):
    """The JSON type of a decoded value, named as JSON names it."""
    if isinstance(value, bool):
        return 'boolean'
    if isinstance(value, (int, float)):
        return 'number'
    if isinstance(value, list):
        return 'array'
    if isinstance(value, dict):
        return 'object'
    return 'string' if isinstance(value, str) else 'null'


def invalid_server_url_reason(aim_url, option="--url"):
    """
    Why `aim_url` cannot be an AIM server address, or None when it can. The
    reason names the address by `option`: `--url` for the commands, `aim_url`
    for secure().

    Checked before the reachability probe (#408): an empty `--url` or a string
    that is not an http(s) URL used to reach the probe, which reported "could
    not reach the AIM server ... no HTTP response within 5s" -- the wrong
    problem, since no request was ever sent.
    """
    url = (aim_url or "").strip()
    if not url:
        return f"{option} is empty"
    try:
        parts = urlsplit(url)
    except ValueError:
        return f"{option} {url!r} is not a URL"
    if parts.scheme not in ("http", "https"):
        return f"{option} {url!r} is not an http(s) URL"
    if not parts.hostname:
        return f"{option} {url!r} has no host"
    return None


# Whitespace, C0, DEL and C1: refused in `aim-sdk init --url` and
# `aim-sdk login --url`, see _url_text_refusal.
_URL_SPACE_OR_CONTROL = re.compile(r'[\s\x00-\x1f\x7f-\x9f]')


def _url_text_refusal(aim_url):
    """
    Why `aim-sdk init` and `aim-sdk login` refuse `aim_url`, an address that
    passed invalid_server_url_reason, before the loopback check, or None.

    A backslash is refused because the loopback check reads the host with
    urlsplit while requests connects with urllib3, which ends the host at a
    backslash where urlsplit does not: for 'http://aim.example.test\\@127.0.0.1'
    the check would read 127.0.0.1 and requests would connect to
    aim.example.test. Whitespace and control characters, which no server
    address holds, are refused too. The reason does not quote the address,
    which may hold a password or the token.
    """
    url = (aim_url or "").strip()
    if "\\" in url:
        return "--url has a backslash"
    if _URL_SPACE_OR_CONTROL.search(url):
        return "--url has whitespace or a control character"
    return None


class _UrlRefusal:
    """
    Why `aim-sdk init` or `aim-sdk login` refuses an address: the lines it
    prints, and, when the refusal is the http: host rule, the address as those
    lines show it. `aim-sdk login --allow-http` lifts that rule only; nothing
    lifts a backslash, whitespace or a control character.
    """

    __slots__ = ('lines', 'cleartext_address')

    def __init__(self, lines, cleartext_address=None):
        self.lines = lines
        self.cleartext_address = cleartext_address


def _cleartext_url_refusal(aim_url, verb, token=None):
    """
    Why `aim-sdk <verb>` ('init' or 'login') refuses `aim_url`, an address
    that passed invalid_server_url_reason, as a _UrlRefusal, or None.

    The one rule both verbs apply before any request: _url_text_refusal, then
    an http: address only on a host in LOOPBACK_HOSTS. The lines name the host
    as urlsplit reads it, which leaves out a user:password part, with `token`
    masked in it, and the example commands keep the port the address names.
    """
    problem = _url_text_refusal(aim_url)
    if problem:
        return _UrlRefusal([
            f"Error: {problem}.",
            "Pass the server's http(s) address, for example:",
            f"  aim-sdk {verb} --url https://aim.example.com",
            f"or omit --url to use {DEFAULT_AIM_URL}.",
        ])

    url_parts = urlsplit(aim_url.strip())
    if url_parts.scheme != 'http' or url_parts.hostname in LOOPBACK_HOSTS:
        return None
    host = url_parts.hostname
    shown = _redact_token(f"[{host}]" if ':' in host else host, token)
    try:
        port = url_parts.port
    except ValueError:
        port = None
    host_port = shown if port is None else f"{shown}:{port}"

    if verb == 'login':
        rule = "aim-sdk login receives your access and refresh tokens over http: only from this machine"
    else:
        rule = "aim-sdk init sends the bootstrap token over http: only to this machine"
    lines = [
        f"Error: {rule} (localhost, 127.0.0.1 or [::1]), and {shown} is not this machine. "
        "Nothing was sent or saved.",
        "Pass the server's https address, for example:",
        f"  aim-sdk {verb} --url https://{host_port}",
    ]
    if verb == 'login':
        lines += [
            "or, on a network you trust, accept that the tokens cross it unencrypted:",
            f"  aim-sdk login --url http://{host_port} --allow-http",
            "Self-hosted: put a TLS-terminating proxy in front of the dashboard and the API "
            "(docs/DEPLOYMENT.md, \"Enable HTTPS\").",
        ]
    else:
        lines.append(f"or omit --url to use {DEFAULT_AIM_URL}.")
    return _UrlRefusal(lines, cleartext_address=f"http://{host_port}")


# The outcomes poll_device_token reports by name. A grant state counts only on
# a 400 or 403 answer (RFC 8628 section 3.5; AIM sends 403), and 'timeout' only
# comes from the local deadline: on any other answer these are just body text.
_POLL_OUTCOMES = ('authorization_pending', 'slow_down', 'expired_token', 'access_denied', 'timeout')

LOGIN_BROWSER_AND_WAIT_HELP = f"""\
browser and wait:
  A browser opens on the approval page only when standard output is a
  terminal; when output goes to a pipe, a file or a CI log, the page and the
  code are printed and no browser opens.
  --no-browser prints the page and the code without opening a browser, at a
  terminal too.
  The command waits for the approval for up to the lesser of the code's
  lifetime the server states and {LOGIN_MAX_WAIT_SECONDS} seconds, and exits 1
  if the code is not approved in that time.
"""

LOGIN_EXIT_CODES_HELP = """\
exit codes:
  0   signed in, or an existing sign-in was kept
  1   the login did not complete: the code was denied or expired, the login
      was cancelled, the server refused the request, or the address was
      refused before any request. An http: address is accepted only on this
      machine (localhost, 127.0.0.1 or [::1]); --allow-http accepts one on
      another host, and the tokens then cross the network unencrypted
  2   the command line was not valid
  75  the AIM server was unavailable: it answered HTTP 502, 503 or 504, or
      gave no HTTP answer. Nothing was stored; run the same command again later
"""


def _no_browser_reason(args):
    """
    Why `aim-sdk login` opens no browser, or None when it opens one.

    The browser is for a person at a terminal: a standard output that is a
    pipe, a file or a CI log has nobody in front of it, and --no-browser asks
    for none. The page and the code are printed either way.
    """
    if getattr(args, 'no_browser', False):
        return '--no-browser was passed'
    try:
        at_terminal = sys.stdout.isatty()
    except (AttributeError, ValueError):
        at_terminal = False
    if not at_terminal:
        return 'standard output is not a terminal'
    return None


def _probe_failure(aim_url, timeout):
    """
    Pre-flight probe: the `requests` failure of one GET to aim_url, or None.

    Any HTTP response -- including an error status -- proves the server is
    reachable; only a transport failure (refused, unroutable, timed out)
    counts as unreachable.
    """
    import requests
    try:
        requests.get(aim_url, timeout=timeout, allow_redirects=False)
        return None
    except requests.RequestException as e:
        return e


def check_server_reachable(aim_url, timeout):
    """Pre-flight probe: is anything answering HTTP at aim_url?"""
    return _probe_failure(aim_url, timeout) is None


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


class _Outage:
    """
    Why the AIM server could not be used for a device-flow request: the 502,
    503 or 504 it answered with, or the class of failure that left the request
    with no HTTP answer.

    A class rather than a dict key so that no server body can produce one.
    """

    __slots__ = ('status', 'retry_after', 'reason_code', 'failure')

    def __init__(self, status=None, retry_after=None, reason_code=None, failure=None):
        self.status = status
        self.retry_after = retry_after
        self.reason_code = reason_code
        self.failure = failure


def _retry_after_seconds(response):
    """`Retry-After` as a non-negative whole number of seconds, or None. A
    date, a fraction or a missing header is None: no default is made up."""
    raw = response.headers.get('Retry-After')
    if not isinstance(raw, str) or not re.fullmatch(r'[0-9]{1,9}', raw.strip()):
        return None
    return int(raw.strip())


def _machine_code(response):
    """The server's machine code for an answer: the body's `reasonCode` when
    it is a string, otherwise `code` when it is a string, otherwise None. Read
    only after the status has set the class of the answer."""
    try:
        data = response.json() if response.content else None
    except ValueError:
        data = None
    if not isinstance(data, dict):
        return None
    for member in ('reasonCode', 'code'):
        if isinstance(data.get(member), str):
            return data[member]
    return None


def _unavailable_answer(response, url):
    """The result of a device-flow request the server answered 502, 503 or 504."""
    return {
        'error': f"HTTP {response.status_code} from {url}",
        'outage': _Outage(
            status=response.status_code,
            retry_after=_retry_after_seconds(response),
            reason_code=_machine_code(response),
        ),
    }


def _outage_from_failure(exc):
    """
    The outage a `requests` failure stands for, or None when it is not one.

    No HTTP answer is an outage: refused, reset, unresolvable, unanswered. A
    rejected TLS certificate is not (running the command again later would not
    help), and neither is a URL `requests` would not send to.
    """
    import requests
    from .client import _is_transport_failure, _transport_failure_class

    if not _is_transport_failure(exc):
        return None
    if isinstance(exc, getattr(requests.exceptions, 'SSLError', ())):
        return None
    return _Outage(failure=_transport_failure_class(exc))


def _no_answer(exc, aim_url):
    """The result of a device-flow request that raised instead of answering."""
    # The same rendering the SDK uses everywhere else: the host from the
    # server URL, the failure class, the URL to check; never the urllib3
    # pool chain.
    from .client import _request_failure_message

    result = {'error': _request_failure_message(exc, aim_url)}
    outage = _outage_from_failure(exc)
    if outage is not None:
        result['outage'] = outage
    return result


def _print_server_unavailable(aim_url, outage, blank_line=True):
    """
    The last output of a login the server could not serve: the lines of
    _server_unavailable_lines. `blank_line` is False where the output so far
    already ends with one.
    """
    if blank_line:
        print()
    for line in _server_unavailable_lines(aim_url, outage):
        print(line)


def _server_unavailable_lines(aim_url, outage):
    """
    What happened and where, that nothing was stored, and when to run the
    command again, for a sign-in the server could not serve.

    The wait is printed, never slept, and comes from `Retry-After` alone.
    """
    if outage.status is not None:
        what = f"it answered HTTP {outage.status}"
        # A machine code is a short token; anything else in that member is
        # server-chosen text and is left out of the terminal.
        if outage.reason_code and re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9_.:-]{0,63}', outage.reason_code):
            what += f", reason code {outage.reason_code}"
    elif outage.failure == 'timed out':
        what = "it gave no HTTP answer in time"
    else:
        what = f"it gave no HTTP answer ({outage.failure})"

    if outage.retry_after is None:
        when = "later"
    elif outage.retry_after == 0:
        when = "now"
    else:
        when = f"in {outage.retry_after} second{'' if outage.retry_after == 1 else 's'}"

    return [
        f"The AIM server at {aim_url} is unavailable: {what}.",
        f"No sign-in was completed and nothing was stored. Run the same command again {when}.",
    ]


def request_device_code(aim_url: str, client_id: str = DEVICE_CLIENT_ID):
    """
    Start the device grant: POST /api/v1/oauth/device/code.

    Returns:
        dict: the server's answer (deviceCode, userCode, verificationUri,
        verificationUriComplete, expiresIn, interval), or {'error': reason}
        with a bare reason the caller prefixes. When the server was
        unavailable (502, 503, 504 or no HTTP answer) the result also carries
        'outage'.
    """
    import requests
    url = f"{aim_url}/api/v1/oauth/device/code"
    try:
        response = requests.post(
            url,
            json={'clientId': client_id},
            headers={'Content-Type': 'application/json'},
            timeout=30,
        )
    except requests.RequestException as e:
        return _no_answer(e, aim_url)

    if response.status_code in SERVER_UNAVAILABLE_STATUSES:
        return _unavailable_answer(response, url)
    if response.status_code != 200:
        return {'error': _server_reason(
            response,
            f"HTTP {response.status_code} from {url}. Verify the server URL is the AIM API "
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

    The status sets the class of an answer before any body field is read: a
    502, 503 or 504 (or no HTTP answer) ends the poll at once, with no retry
    and no sleep, whatever the body says; the grant states count only on a 400
    or 403.

    Returns:
        dict: the token pair on success, or {'error': <one of 'expired_token',
        'access_denied', 'timeout', or a bare reason>}. When the server was
        unavailable the result also carries 'outage'.
    """
    import requests
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
            return _no_answer(e, aim_url)

        status = response.status_code
        if status == 200:
            try:
                data = response.json()
            except ValueError:
                data = None
            if not isinstance(data, dict) or not data.get('accessToken'):
                return {'error': f"the token answer from {url} carried no access token"}
            return data

        if status in SERVER_UNAVAILABLE_STATUSES:
            return _unavailable_answer(response, url)
        if status == 429:
            wait += 5
            continue

        code = _server_reason(response, '')
        if status in (400, 403):
            if code == 'slow_down':
                wait += 5
                continue
            if code == 'authorization_pending':
                continue
            if code in ('expired_token', 'access_denied'):
                return {'error': code}
        if code in _POLL_OUTCOMES:
            code = ''
        return {'error': code or f"HTTP {status} from {url}"}


def _device_wait(device):
    """
    The poll interval and the longest wait, in seconds, for a device-code
    answer: its interval (5 when it states none or no number), and the code's
    lifetime it states in expiresIn (LOGIN_MAX_WAIT_SECONDS when it states
    none or no number), capped at LOGIN_MAX_WAIT_SECONDS and at least 1.
    """
    try:
        interval = int(device.get('interval') or 5)
    except (TypeError, ValueError):
        interval = 5
    try:
        expires_in = int(device.get('expiresIn') or LOGIN_MAX_WAIT_SECONDS)
    except (TypeError, ValueError):
        expires_in = LOGIN_MAX_WAIT_SECONDS
    return interval, max(1, min(expires_in, LOGIN_MAX_WAIT_SECONDS))


def _verification_uri_problem(device):
    """
    Why a device-code answer's verification URIs cannot be used, or None.

    Both URLs are server-chosen strings: the one printed and the one handed
    to the browser. Only http(s) is opened; anything else stops the sign-in.
    """
    verification_uri = device.get('verificationUri', '')
    for candidate in (verification_uri, device.get('verificationUriComplete') or verification_uri):
        if not (str(candidate).startswith('http://') or str(candidate).startswith('https://')):
            return (f"the server sent a verification URI that is not an http(s) URL ({candidate!r}); "
                    f"check the server's FRONTEND_URL.")
    return None


def _sign_in_failure(reason, max_wait):
    """The sentence that ends a device grant that answered `reason` instead of a token pair."""
    if reason in ('expired_token', 'timeout'):
        return (f"The code expired before it was approved (waited {max_wait}s; login timed out). "
                f"Run aim-sdk login to get a new code.")
    if reason == 'access_denied':
        return "The login was denied in the dashboard. Nothing was stored."
    return f"Login failed: {reason}"


def login(args):
    """Login to an AIM server with the OAuth 2.0 device grant (RFC 8628)."""
    from .credentials import save_sdk_credentials, load_sdk_credentials, AIM_DIR

    url_problem = invalid_server_url_reason(args.url)
    if url_problem:
        print(f"Error: {url_problem}.")
        print("Pass the server's http(s) address, for example:")
        print("  aim-sdk login --url https://aim.example.com")
        print(f"or omit --url to use {DEFAULT_AIM_URL}.")
        return 1
    # Before any request: the whole sign-in goes to this address.
    refusal = _cleartext_url_refusal(args.url, 'login')
    if refusal and not (refusal.cleartext_address and getattr(args, 'allow_http', False)):
        print("\n".join(refusal.lines))
        return 1
    if refusal:
        print(f"Warning: {refusal.cleartext_address} is not this machine and the address is http:. "
              "The access and refresh tokens this sign-in receives will cross the network unencrypted, "
              "and anyone on the path can read and reuse them. --allow-http accepted that.", flush=True)

    aim_url = args.url.strip().rstrip('/')

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
    probe_failure = _probe_failure(aim_url, LOGIN_PROBE_TIMEOUT_SECONDS)
    if probe_failure is not None:
        outage = _outage_from_failure(probe_failure)
        if outage is not None:
            _print_server_unavailable(aim_url, outage, blank_line=False)
            return EXIT_SERVER_UNAVAILABLE
        print(f"Error: could not reach the AIM server at {aim_url}")
        print(f"(no HTTP response within {LOGIN_PROBE_TIMEOUT_SECONDS}s).")
        print("Check the URL and your network, then retry. For a self-hosted")
        print("server, pass it explicitly: aim-sdk login --url <your-aim-url>")
        return 1

    # Start the device grant. The device code stays in this process; only the
    # short user code is shown, and it authorizes nothing by itself.
    device = request_device_code(aim_url)
    if isinstance(device.get('outage'), _Outage):
        _print_server_unavailable(aim_url, device['outage'], blank_line=False)
        return EXIT_SERVER_UNAVAILABLE
    if 'error' in device:
        print(f"\nCould not start the device login: {device['error']}")
        return 1

    user_code = device.get('userCode', '')
    verification_uri = device.get('verificationUri', '')
    verification_uri_complete = device.get('verificationUriComplete') or verification_uri
    uri_problem = _verification_uri_problem(device)
    if uri_problem:
        print(f"\nCould not start the device login: {uri_problem}")
        return 1
    interval, max_wait = _device_wait(device)

    print("To sign in, open this page in your browser and approve the code:")
    print()
    print(f"  {verification_uri}")
    print()
    print(f"  Code: {user_code}")
    print()
    print(f"Waiting for approval... (the code expires in {max_wait}s; Ctrl+C to cancel)")
    no_browser_reason = _no_browser_reason(args)
    if no_browser_reason:
        print(f"No browser was opened ({no_browser_reason}): open the page above and approve the code. "
              f"This command waits up to {max_wait} seconds for the approval and exits 1 if the code "
              f"is not approved in that time.")
    # A pipe or a log file is block-buffered: without this flush the page and
    # the code would stay in the buffer for the whole wait.
    sys.stdout.flush()

    if no_browser_reason is None:
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

    if isinstance(token_response.get('outage'), _Outage):
        _print_server_unavailable(aim_url, token_response['outage'])
        return EXIT_SERVER_UNAVAILABLE
    if 'error' in token_response:
        print(f"\n{_sign_in_failure(token_response['error'], max_wait)}")
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
        print("Every @perform_action call is signed, checked by the server against the")
        print("agent's granted capabilities and your organization's security policies,")
        print("and recorded in the audit log. In strict enforcement mode, calls outside")
        print("the granted capabilities are denied unless a security policy allows")
        print("them; in monitoring mode, the default, they run and are recorded. An")
        print("administrator turns strict mode on with Global Enforcement Mode on the")
        print("dashboard page /dashboard/admin/security-policies.")
        print()
        print("   Verify auth:   aim-sdk status")
        print("   Quickstart:    https://opena2a.org/docs/tutorials/sdk-quickstart")
        print("   Full docs:     https://opena2a.org/docs/aim")
        print()
        return 0
    else:
        print("Failed to save credentials.")
        return 1


# The first line secure() prints when it signs this machine in; `{agent}` is
# the name the call registers.
INLINE_SIGN_IN_LEAD = "{agent} needs an AIM sign-in on this machine. Sign in once to continue."


def sign_in_block(lead, aim_url, verification_uri, user_code, max_wait):
    """
    The lines a device sign-in prints before it waits: `lead`, the server, the
    page to open, the code, and the wait line, which gives `max_wait` seconds
    in whole minutes, rounded up. Each value is printed as one terminal line
    (_terminal_line). No line carries styling, so the bytes are the same at a
    terminal, in a pipe and with NO_COLOR set.
    """
    minutes = -(-max_wait // 60)
    return [
        lead,
        f"  Server: {_terminal_line(aim_url)}",
        f"  Open:   {_terminal_line(verification_uri)}",
        f"  Code:   {_terminal_line(user_code)}",
        f"Waiting for your approval (up to {minutes} min). Ctrl+C stops the wait. "
        "Nothing is stored until you approve.",
    ]


def _home_relative(path):
    """`path` with the home directory written as `~`, or in full when it is elsewhere."""
    try:
        return "~/" + Path(path).relative_to(Path.home()).as_posix()
    except ValueError:
        return str(path)


def sign_in_inline(aim_url, agent_name, out=None):
    """
    Sign this machine in to the AIM server at `aim_url` for secure(), which
    calls this when it finds no SDK credentials and no api_key and a person
    can approve a code. The device grant and the stored credentials are the
    ones `aim-sdk login` uses; the lines go to `out`, standard error by default.

    Returns:
        dict: the stored SDK credentials.

    Raises:
        ConfigurationError: the address was refused before any request, the
            server was unavailable or did not start the grant, or the
            credentials could not be saved.
        AuthenticationError: the code was denied or expired, or the wait for
            the token pair ended with another error.
        KeyboardInterrupt: Ctrl+C during the wait.

    Credentials are saved only after the server answers with a token pair.
    """
    from . import credentials as stored
    from .exceptions import AuthenticationError, ConfigurationError

    out = sys.stderr if out is None else out

    url_problem = invalid_server_url_reason(aim_url, option="aim_url")
    if url_problem:
        raise ConfigurationError(
            f"{url_problem}. Pass the server's http(s) address, for example "
            f"aim_url=\"https://aim.example.com\", or omit aim_url to use {DEFAULT_AIM_URL}."
        )
    # Before any request: the whole sign-in goes to this address.
    refusal = _cleartext_url_refusal(aim_url, 'login')
    if refusal:
        raise ConfigurationError("\n".join(refusal.lines))
    aim_url = aim_url.strip().rstrip('/')

    device = request_device_code(aim_url)
    if isinstance(device.get('outage'), _Outage):
        raise ConfigurationError("\n".join(_server_unavailable_lines(aim_url, device['outage'])))
    if 'error' in device:
        raise ConfigurationError(_terminal_line(f"Could not start the device login: {device['error']}"))
    uri_problem = _verification_uri_problem(device)
    if uri_problem:
        raise ConfigurationError(_terminal_line(f"Could not start the device login: {uri_problem}"))
    interval, max_wait = _device_wait(device)
    verification_uri = device['verificationUri']

    lead = INLINE_SIGN_IN_LEAD.format(agent=_terminal_line(agent_name))
    for line in sign_in_block(lead, aim_url, verification_uri, device.get('userCode', ''), max_wait):
        print(line, file=out)
    out.flush()

    # The rule `aim-sdk login` applies: a browser opens only when standard
    # output is a terminal, on the page with the code filled in.
    if _no_browser_reason(None) is None:
        try:
            webbrowser.open(device.get('verificationUriComplete') or verification_uri)
        except Exception:
            pass

    token_response = poll_device_token(aim_url, device['deviceCode'], interval, time.monotonic() + max_wait)
    if isinstance(token_response.get('outage'), _Outage):
        raise ConfigurationError("\n".join(_server_unavailable_lines(aim_url, token_response['outage'])))
    if 'error' in token_response:
        raise AuthenticationError(_terminal_line(_sign_in_failure(token_response['error'], max_wait)))

    # The credentials login() stores, key for key: accessToken and
    # organizationId mark them as a sign-in's (credentials.detect_credential_origin).
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
    if not stored.save_sdk_credentials(credentials):
        raise ConfigurationError("Failed to save credentials.")

    path = stored.SDK_CREDENTIALS_FILE
    try:
        private = stat.S_IMODE(os.stat(path).st_mode) == 0o600
    except OSError:
        private = False
    who = credentials.get('userEmail') or credentials.get('userId') or 'Unknown'
    print(f"Signed in as {_terminal_line(who)} on {aim_url}.", file=out)
    print(f"Credentials: {_home_relative(path)}" + (" (owner read/write only)" if private else ""), file=out)
    out.flush()
    return credentials


_SDK_TOKEN_KEYS = ("refreshToken", "refresh_token", "accessToken", "access_token")


def _clear_legacy_sdk_credentials() -> None:
    """Remove SDK OAuth tokens from the legacy ~/.aim/credentials.json.

    load_sdk_credentials() copies that file to sdk_credentials.json and leaves
    it in place, so a logout that deleted only the copy was undone by the next
    command: `aim-sdk status` migrated the legacy file again and read as signed
    in. The file is deleted when the tokens are all it holds; agent credentials
    sharing it are kept and only the token fields are dropped.
    """
    from .credentials import LEGACY_CREDENTIALS_FILE, CredentialType, detect_credential_type

    legacy = Path(LEGACY_CREDENTIALS_FILE)
    try:
        data = json.loads(legacy.read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return
    if not isinstance(data, dict) or detect_credential_type(data) != CredentialType.SDK_OAUTH:
        return
    remaining = {k: v for k, v in data.items() if k not in _SDK_TOKEN_KEYS}
    try:
        if detect_credential_type(remaining) == CredentialType.AGENT:
            legacy.write_text(json.dumps(remaining, indent=2), encoding="utf-8")
            os.chmod(legacy, 0o600)
        else:
            legacy.unlink()
    except OSError:
        pass


def logout(args):
    """Revoke the stored pair on the server and clear the local credentials."""
    from .credentials import load_sdk_credentials, remove_sdk_credentials_lock, AIM_DIR
    from .oauth import OAuthTokenManager

    print("Logging out...")

    creds_file = Path(AIM_DIR) / "sdk_credentials.json"
    had_file = creds_file.exists()
    if not had_file and not load_sdk_credentials():
        remove_sdk_credentials_lock()
        print("No credentials to clear")
        return 0

    # revoke_token posts the server's logout route, prints why when the server
    # did not confirm the revocation, and deletes the local file on every path.
    confirmed = False
    try:
        token_manager = OAuthTokenManager()
        if token_manager.has_credentials():
            confirmed = token_manager.revoke_token()
    except Exception:
        confirmed = False

    # Belt and braces: the file must be gone whatever the token manager did.
    if creds_file.exists():
        creds_file.unlink()
    _clear_legacy_sdk_credentials()
    # The lock file guards the credentials file removed above.
    remove_sdk_credentials_lock()

    if confirmed:
        print("Signed out: the server revoked the session and the local credentials were cleared.")
        return 0
    print("Signed out locally only: the local credentials were cleared, but the server did not "
          "confirm the revocation (see above). Verify with: aim-sdk status")
    return 1


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


def _status_verdict(creds, token_state):
    """
    (authenticated, sentence) for `aim-sdk status`, from the stored
    credentials alone. Every branch returns a sentence, so the command always
    states a verdict (#409): a legacy credentials file holding only a refresh
    token used to print Server/User/Credentials and then nothing, with exit 0.
    """
    has_refresh = bool(creds.get('refreshToken') or creds.get('refresh_token'))
    if token_state == "valid":
        return True, "Authenticated. The access token is valid."
    if not has_refresh:
        return False, ("Not authenticated: the stored credentials hold no usable token. "
                       "Run 'aim-sdk login' to authenticate.")
    if token_state == "expired":
        return True, "Authenticated. The access token has expired; the SDK refreshes it on next use."
    if token_state == "absent":
        return True, ("Authenticated with a stored refresh token; the SDK requests an "
                      "access token on first use.")
    return True, ("Authenticated. The access token's expiry could not be read; the SDK "
                  "refreshes it if the server rejects it.")


def _credentials_source():
    """The credentials file status should cite: one that exists, or None."""
    from . import credentials as creds_module

    for path in (creds_module.SDK_CREDENTIALS_FILE, creds_module.LEGACY_CREDENTIALS_FILE):
        if Path(path).exists():
            return Path(path)
    return None


def _unreadable_credentials():
    """
    (path, why) for the first file _credentials_source checks that exists but
    cannot be read as a JSON object, or None.

    Asked only when no sign-in was loaded: load_sdk_credentials returns None for
    a file it cannot read as it does for no file, and status reported a file
    holding text that is not JSON as `tokenState` "absent".
    """
    from . import credentials as creds_module

    for path in (creds_module.SDK_CREDENTIALS_FILE, creds_module.LEGACY_CREDENTIALS_FILE):
        path = Path(path)
        if not path.exists():
            continue
        try:
            with open(path, 'r') as f:
                data = json.load(f)
        except OSError as e:
            return path, e.strerror or type(e).__name__
        except ValueError:
            return path, "it is not JSON"
        if not isinstance(data, dict):
            return path, "it holds JSON that is not an object"
    return None


def _print_saved_agents(signed_in):
    """
    List the agents saved under ~/.aim/agents after the sign-in verdict, which
    is about the SDK sign-in alone. `aim-sdk init` saves an agent there and
    stores no sign-in, so right after it the verdict read "Not authenticated"
    with nothing to say that the agent was saved.
    """
    from . import credentials as creds_module

    agents = sorted(creds_module.list_agent_credentials(), key=lambda agent: str(agent.get('name')))
    if not agents:
        return
    print()
    print(f"Agents saved on this machine, in {creds_module.AGENTS_DIR}:")
    for agent in agents:
        agent_id = agent.get('agent_id')
        label = f"agent ID {_terminal_line(agent_id, 128)}" if isinstance(agent_id, str) and agent_id else "no agent ID saved"
        print(f"   {_terminal_line(agent.get('name'), 128)} ({label})")
    if not signed_in:
        print("The verdict above is about the SDK sign-in that aim-sdk login stores; "
              "aim-sdk init saves an agent and no sign-in.")


def status(args):
    """Check the SDK sign-in and list the agents saved on this machine."""
    from .credentials import load_sdk_credentials, AIM_DIR

    creds = load_sdk_credentials()

    if not creds:
        unreadable = _unreadable_credentials()
        if getattr(args, 'json', False):
            # Exactly one JSON object on stdout and nothing else -- this is the
            # output a wrapper script parses, so a stray banner line would make
            # `aim-sdk status --json | jq` fail on a working install.
            # Cite a file only if it exists: with nothing stored there is no
            # sdk_credentials.json to send the user to.
            source = unreadable[0] if unreadable else _credentials_source()
            print(json.dumps({
                "authenticated": False,
                "server": None,
                "user": None,
                "credentialsPath": str(source) if source else None,
                "tokenState": "unreadable" if unreadable else "absent",
            }))
            return 1
        print("Checking authentication status...")
        print()
        print("Not authenticated.")
        if unreadable:
            print(f"{unreadable[0]} exists but could not be read: {unreadable[1]}.")
        print()
        print("Run 'aim-sdk login' to authenticate")
        _print_saved_agents(signed_in=False)
        return 1

    # Legacy files use snake_case keys; read both so an upgraded install does
    # not show "User: Unknown" for a user the file names.
    aim_url = creds.get('aimUrl') or creds.get('aim_url')
    user_email = creds.get('userEmail') or creds.get('user_email') or 'Unknown'
    token_state = _token_state(creds.get('accessToken') or creds.get('access_token'))
    authenticated, verdict = _status_verdict(creds, token_state)
    # Cite a file only if it exists: when adopting a legacy file fails, the
    # new location was never written and naming it sent users to nothing.
    source = _credentials_source()

    if getattr(args, 'json', False):
        print(json.dumps({
            "authenticated": authenticated,
            "server": aim_url,
            "user": user_email,
            "credentialsPath": str(source) if source else None,
            "tokenState": token_state,
        }))
        return 0 if authenticated else 1

    print("Checking authentication status...")
    print()
    print(f"   Server: {aim_url or f'not recorded (login uses {DEFAULT_AIM_URL} by default)'}")
    print(f"   User: {user_email}")
    print(f"   Credentials: {source if source else f'none saved under {AIM_DIR}'}")
    print()
    print(verdict)
    print("(Checked from the stored credentials; the server was not contacted.)")
    _print_saved_agents(signed_in=authenticated)

    return 0 if authenticated else 1


def _generate_agent_keypair():
    """
    A fresh Ed25519 key pair as (private_key_b64, public_key_b64).

    The private key is the 64-byte seed + public key form the client loads
    (and Go's ed25519 package uses); it never leaves this machine.
    """
    from nacl.signing import SigningKey

    signing_key = SigningKey.generate()
    seed = bytes(signing_key)
    public = signing_key.verify_key.encode()
    return (
        base64.b64encode(seed + public).decode('ascii'),
        base64.b64encode(public).decode('ascii'),
    )


def _redact_token(text, token):
    """`text` with every occurrence of the bootstrap token masked."""
    text = str(text)
    if token:
        text = text.replace(token, f"{BOOTSTRAP_TOKEN_PREFIX}[REDACTED]")
    return text


def _request_not_sent(exc):
    """
    Whether a `requests` failure is known to have come before the request was
    sent: no connection was made (refused, host not found, unroutable), or
    none within the timeout. Any other failure may have come after the server
    received the request.
    """
    import requests
    from urllib3.exceptions import NewConnectionError

    if isinstance(exc, requests.exceptions.ConnectTimeout):
        return True
    if isinstance(exc, requests.exceptions.ConnectionError) and exc.args:
        return isinstance(getattr(exc.args[0], 'reason', None), NewConnectionError)
    return False


def exchange_bootstrap_token(aim_url, token, name, public_key):
    """
    Register one agent with a bootstrap token:
    POST /api/v1/onboarding/bootstrap-tokens/exchange.

    The token travels only in the X-AIM-Bootstrap-Token header, never in the
    URL. Redirects are not followed: requests drops Authorization on a
    cross-host redirect but keeps custom headers, so following one would hand
    the token to whatever host the redirect names.

    Returns:
        dict: the server's answer (agentId, organizationId, name, status,
        publicKey, aimUrl), or {'error': reason} with a bare reason the caller
        prefixes. No returned string contains the token, and server text in a
        reason (its error message, a redirect address) is one terminal line.
        A request that got no HTTP answer also carries 'token_state':
        'unused' when the request was not sent, 'unknown' otherwise.
    """
    import requests
    url = f"{aim_url}/api/v1/onboarding/bootstrap-tokens/exchange"
    try:
        response = requests.post(
            url,
            json={'name': name, 'agentType': 'custom', 'publicKey': public_key},
            headers={'Content-Type': 'application/json', BOOTSTRAP_TOKEN_HEADER: token},
            timeout=INIT_REQUEST_TIMEOUT_SECONDS,
            allow_redirects=False,
        )
    except requests.RequestException as e:
        from .client import _request_failure_message

        return {
            'error': _redact_token(_request_failure_message(e, aim_url), token),
            'token_state': 'unused' if _request_not_sent(e) else 'unknown',
        }

    status_code = response.status_code
    if status_code in (200, 201):
        try:
            data = response.json()
        except ValueError:
            data = None
        if not isinstance(data, dict) or not data.get('agentId'):
            return {'error': f"the answer from {url} carried no agent ID"}
        return data

    if 300 <= status_code < 400:
        location = response.headers.get('Location') or 'another address'
        return {'error': (f"{url} answered with a redirect to {_terminal_line(_redact_token(location, token))}. "
                          f"The token was not sent on; pass the address the server redirects to as --url.")}

    reason = _terminal_line(_redact_token(_server_reason(response, ''), token))
    if status_code == 401:
        return {'error': reason or ("The bootstrap token is not valid. "
                                    "Generate a new one from the onboarding screen.")}
    if status_code == 409:
        conflict = reason.rstrip('.') or f"An agent named {name!r} already exists in your organization"
        return {'error': (f"{conflict}. "
                          f"The token was not used: run aim-sdk init again with --name <another-name>.")}
    if status_code == 429:
        return {'error': "Too many requests to the onboarding endpoint. Wait a minute, then run aim-sdk init again."}
    if status_code == 404:
        return {'error': (f"HTTP 404 from {url}. Check that --url is the AIM API address and that the "
                          f"server runs an AIM version with onboarding tokens.")}
    return {'error': reason or f"HTTP {status_code} from {url}"}


def init(args):
    """Register a first agent with a bootstrap token from the onboarding screen."""
    from .credentials import get_agent_credentials_path, save_agent_credentials

    if getattr(args, 'token', None) is not None:
        # Refused whatever it holds, even when the environment has a token
        # too; the value is never read or printed.
        print(f"Error: {TOKEN_FLAG_REFUSED}.")
        print("Set the token in the environment instead:")
        print()
        print(f"  {BOOTSTRAP_TOKEN_ENV}=<token> aim-sdk init")
        return 1

    token = (os.environ.get(BOOTSTRAP_TOKEN_ENV) or '').strip()
    if not token:
        print("Error: no bootstrap token.")
        print("Copy the command from the onboarding screen in the AIM dashboard, or set the token")
        print("in the environment, which keeps it out of the process list and shell history:")
        print()
        print(f"  {BOOTSTRAP_TOKEN_ENV}=<token> aim-sdk init")
        return 1
    if not token.startswith(BOOTSTRAP_TOKEN_PREFIX):
        # Never echo it: a value pasted into the wrong variable may be some
        # other secret.
        print(f"Error: the token given is not a bootstrap token (those start with {BOOTSTRAP_TOKEN_PREFIX}).")
        print("Generate one from the onboarding screen in the AIM dashboard.")
        return 1

    url_problem = invalid_server_url_reason(args.url)
    if url_problem:
        print(f"Error: {url_problem}.")
        print("Pass the server's http(s) address, for example:")
        print("  aim-sdk init --url https://aim.example.com")
        print(f"or omit --url to use {DEFAULT_AIM_URL}.")
        return 1
    refusal = _cleartext_url_refusal(args.url, 'init', token)
    if refusal:
        print("\n".join(refusal.lines))
        return 1
    aim_url = args.url.strip().rstrip('/')

    name = (args.name or '').strip()
    if not name:
        print("Error: --name is empty.")
        return 1
    # The name is the agent's name on the dashboard; the server takes any
    # text, so a path is refused here, before the token is sent.
    if '/' in name or '\\' in name or name in ('.', '..'):
        print("Error: an agent name cannot hold '/' or '\\', or be '.' or '..'. Nothing was sent.")
        print("Pick another name; it is the agent's name on the dashboard:")
        print()
        print("  aim-sdk init --name <name>")
        return 1

    # An existing file holds the private key of an agent registered earlier.
    # Refusing here, before the token is sent, keeps that key and the token.
    creds_path = get_agent_credentials_path(name)
    if creds_path.exists():
        print(f"Error: credentials for an agent named {name!r} already exist at {creds_path}.")
        print("Pick another name, which leaves that agent's key in place:")
        print()
        print("  aim-sdk init --name <another-name>")
        return 1

    private_key, public_key = _generate_agent_keypair()

    print(f"Registering agent {name!r} with {aim_url} ...")
    result = exchange_bootstrap_token(aim_url, token, name, public_key)
    if 'error' in result:
        print(f"\nCould not register the agent: {result['error']}")
        if result.get('token_state') == 'unused':
            print("The request was not sent, so the token has not been used.")
        elif result.get('token_state') == 'unknown':
            print("The server may have received the request before it failed, so the token may have been used. "
                  "If the agent is listed in the dashboard, delete it, then run aim-sdk init again with a new token.")
        return 1

    agent_id = result['agentId']
    if not isinstance(agent_id, str) or not _INIT_ANSWER_ID_PATTERN.fullmatch(agent_id):
        if isinstance(agent_id, str):
            shown = json.dumps(_terminal_line(agent_id, 128), ensure_ascii=False)
        else:
            shown = f"a JSON {_json_type_name(agent_id)}"
        print(f"\nError: the server answered with an agent ID this tool cannot use ({shown}). Nothing was saved.")
        print("The agent may exist on the server: delete it in the dashboard, then run aim-sdk init again with a new token.")
        return 1
    returned_key = result.get('publicKey')
    if returned_key and returned_key != public_key:
        print(f"\nError: the server registered agent {agent_id} with a public key this machine did not")
        print("generate, so the local private key cannot sign for it. Nothing was saved.")
        return 1

    organization_id = result.get('organizationId')
    if not isinstance(organization_id, str) or not _INIT_ANSWER_ID_PATTERN.fullmatch(organization_id):
        organization_id = None
    agent_status = result.get('status')
    if not isinstance(agent_status, str) or not _INIT_ANSWER_STATUS_PATTERN.fullmatch(agent_status):
        agent_status = 'unknown'

    saved = save_agent_credentials(name, {
        'agent_id': agent_id,
        'organization_id': organization_id,
        'public_key': public_key,
        'private_key': private_key,
        'aim_url': aim_url,
        'status': agent_status,
        'registered_at': datetime.now(timezone.utc).isoformat(),
    })
    if not saved:
        print(f"\nThe agent {agent_id} was registered, but its credentials could not be saved to {creds_path}.")
        print("Its private key existed only in this process and is gone; delete the agent in the")
        print("dashboard and run aim-sdk init again with a new token.")
        return 1

    from ._dashboard import dashboard_url_for

    dashboard_url = dashboard_url_for(aim_url)
    print()
    print("Agent registered.")
    print()
    print(f"   Name:        {name}")
    print(f"   Agent ID:    {agent_id}")
    print(f"   Status:      {agent_status}")
    print(f"   Credentials: {creds_path} (owner read/write only)")
    print()
    print(f"   Dashboard:   {dashboard_url}/dashboard/agents/{agent_id}")
    print()
    print("The private key was generated on this machine and was not sent to the server.")
    print("Use the agent from Python:")
    print()
    print("   from aim_sdk import secure")
    print()
    print(f"   agent = secure({name!r})")
    print()
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
    login_parser = subparsers.add_parser(
        'login',
        help='Login to AIM server',
        epilog=LOGIN_BROWSER_AND_WAIT_HELP + "\n" + LOGIN_EXIT_CODES_HELP,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
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
    login_parser.add_argument(
        '--no-browser',
        action='store_true',
        help='Print the approval page and the code without opening a browser',
    )
    login_parser.add_argument(
        '--allow-http',
        action='store_true',
        help=('Accept an http: address on a host other than this machine (localhost, 127.0.0.1 or [::1]); '
              'the access and refresh tokens then cross the network unencrypted'),
    )
    login_parser.set_defaults(func=login)

    # Init command
    init_parser = subparsers.add_parser(
        'init',
        help='Register a first agent with a bootstrap token from the onboarding screen',
        description=(f'Register a first agent with a bootstrap token from the onboarding screen. '
                     f'The token is read from {BOOTSTRAP_TOKEN_ENV} only: a flag value is visible in '
                     f'the process list and shell history.'),
    )
    # Defined so that init() refuses it: an option argparse does not know
    # would be printed back, value and all, in its "unrecognized arguments" error.
    init_parser.add_argument('--token', default=None, help=argparse.SUPPRESS)
    init_parser.add_argument(
        '--url',
        default=DEFAULT_AIM_URL,
        help=f'AIM server URL, for a self-hosted server (default: {DEFAULT_AIM_URL})',
    )
    init_parser.add_argument(
        '--name',
        default=DEFAULT_INIT_AGENT_NAME,
        help=f'Agent name, without "/" or "\\" (default: {DEFAULT_INIT_AGENT_NAME})',
    )
    init_parser.set_defaults(func=init)

    # Logout command
    logout_parser = subparsers.add_parser('logout', help='Logout and clear credentials')
    logout_parser.set_defaults(func=logout)

    # Status command
    status_parser = subparsers.add_parser(
        'status',
        help='Check the SDK sign-in and list the agents saved on this machine',
    )
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
