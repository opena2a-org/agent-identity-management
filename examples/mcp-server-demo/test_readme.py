"""Start mcp-server.py and check it against README.md.

Run from this directory, with requirements.txt installed:

    python3 -m unittest -v test_readme.py

The test starts its own copy of the server on port 5151, so stop any copy
that is already running first.
"""

import base64
import json
import os
import re
import socket
import subprocess
import sys
import threading
import time
import unittest
import urllib.error
import urllib.request
from pathlib import Path
from urllib.parse import urlsplit, urlunsplit

from nacl.encoding import Base64Encoder
from nacl.signing import VerifyKey

HERE = Path(__file__).resolve().parent
README = (HERE / "README.md").read_text(encoding="utf-8")
SERVER = HERE / "mcp-server.py"
PORT = 5151

# Request bodies for the POST routes the README documents. A documented POST
# route that is missing here is sent an empty JSON object.
POST_BODIES = {
    "/mcp/.well-known/mcp/verify": {"challenge": "bm9uY2U=", "server_id": "readme-check"},
    "/mcp/tools/echo": {"message": "hello"},
    "/mcp/tools/calculate": {"expression": "2 + 2"},
    "/mcp/tools/timestamp": {},
}

# Paths an earlier README documented. The server never served them.
OLD_PATHS = [
    ("POST", "/tools/echo"),
    ("POST", "/tools/calculate"),
    ("POST", "/tools/timestamp"),
    ("GET", "/resources/status"),
    ("GET", "/resources/config"),
    ("POST", "/prompts/greeting"),
    ("POST", "/auth/challenge"),
]


def request(method, url, body=None):
    data = None
    headers = {}
    if method == "POST":
        data = json.dumps(body if body is not None else {}).encode("utf-8")
        headers["Content-Type"] = "application/json"
    req = urllib.request.Request(url, data=data, headers=headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=5) as resp:
            return resp.status, resp.read()
    except urllib.error.HTTPError as err:
        with err:
            return err.code, err.read()


def on_local_port(url):
    """The same URL, sent to 127.0.0.1 (the server listens on IPv4 only)."""
    parts = urlsplit(url)
    return urlunsplit(parts._replace(netloc=f"127.0.0.1:{parts.port}"))


def documented_routes():
    """(path, method) rows of the README's Endpoints table."""
    return re.findall(r"^\| `(/[^`]*)` \| (GET|POST) \|", README, re.MULTILINE)


def registered_url():
    """The Server URL the README tells the reader to register in AIM."""
    match = re.search(r"\*\*Server URL\*\*: `([^`]+)`", README)
    return match.group(1) if match else None


class ReadmeMatchesServer(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        with socket.socket() as probe:
            if probe.connect_ex(("127.0.0.1", PORT)) == 0:
                raise RuntimeError(f"port {PORT} is already in use; stop the server running there")
        env = dict(os.environ, PYTHONUNBUFFERED="1")
        cls.proc = subprocess.Popen(
            [sys.executable, str(SERVER)],
            cwd=HERE,
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
        )
        cls.output = []

        def collect():
            for line in cls.proc.stdout:
                cls.output.append(line)

        threading.Thread(target=collect, daemon=True).start()
        deadline = time.monotonic() + 20
        while time.monotonic() < deadline:
            if cls.proc.poll() is not None:
                break
            with socket.socket() as probe:
                if probe.connect_ex(("127.0.0.1", PORT)) == 0:
                    return
            time.sleep(0.1)
        cls.proc.kill()
        raise RuntimeError("server did not start on port %d:\n%s" % (PORT, "".join(cls.output)))

    @classmethod
    def tearDownClass(cls):
        cls.proc.terminate()
        cls.proc.wait(timeout=10)
        cls.proc.stdout.close()

    def banner_value(self, label):
        """The value after `label` on the first server output line that starts with it."""
        deadline = time.monotonic() + 5
        while time.monotonic() < deadline:
            for line in list(self.output):
                if line.strip().startswith(label):
                    return line.split(label, 1)[1].strip()
            time.sleep(0.1)
        self.fail(f"server output has no '{label}' line")

    def test_server_listens_on_the_documented_port(self):
        self.assertTrue(f"port {PORT}" in README, f"README does not say the server listens on port {PORT}")
        self.assertFalse("5555" in README, "README still names port 5555")
        self.assertEqual(registered_url(), f"http://localhost:{PORT}/mcp")

    def test_every_documented_route_answers_its_method(self):
        routes = documented_routes()
        self.assertGreaterEqual(len(routes), 8, "README Endpoints table not found")
        for path, method in routes:
            with self.subTest(route=f"{method} {path}"):
                status, body = request(method, f"http://127.0.0.1:{PORT}{path}", POST_BODIES.get(path))
                self.assertTrue(200 <= status < 300, f"{method} {path} -> {status}: {body[:200]!r}")

    def test_old_unprefixed_paths_are_not_served(self):
        for method, path in OLD_PATHS:
            with self.subTest(route=f"{method} {path}"):
                status, _ = request(method, f"http://127.0.0.1:{PORT}{path}", {})
                self.assertEqual(status, 404)

    def test_verify_url_aim_builds_returns_a_valid_signature(self):
        # AIM appends /.well-known/mcp/verify to the registered URL, posts a
        # base64 nonce and reads `signedChallenge` from the JSON reply.
        verify_url = on_local_port(registered_url() + "/.well-known/mcp/verify")
        nonce = base64.b64encode(os.urandom(32)).decode("ascii")
        status, body = request("POST", verify_url, {"challenge": nonce, "server_id": "readme-check"})
        self.assertEqual(status, 200, body[:200])
        reply = json.loads(body)
        self.assertIn("signedChallenge", reply)
        public_key = VerifyKey(self.banner_value("Public Key:").encode("ascii"), encoder=Base64Encoder)
        public_key.verify(nonce.encode("ascii"), base64.b64decode(reply["signedChallenge"]))

    def test_capabilities_url_aim_derives_lists_the_tools(self):
        # AIM drops the registered URL's path and reads the well-known
        # capabilities document from the host and port.
        parts = urlsplit(registered_url())
        status, body = request("GET", f"http://127.0.0.1:{parts.port}/.well-known/mcp/capabilities")
        self.assertEqual(status, 200)
        tools = {tool["name"] for tool in json.loads(body)["tools"]}
        self.assertEqual(tools, {"echo", "calculate", "timestamp"})

    def test_endpoints_the_server_prints_answer(self):
        status, _ = request("GET", on_local_port(self.banner_value("Capabilities Endpoint:")))
        self.assertEqual(status, 200)
        status, _ = request("POST", on_local_port(self.banner_value("Verification Endpoint:")), POST_BODIES["/mcp/.well-known/mcp/verify"])
        self.assertEqual(status, 200)

    def test_readme_names_only_environment_variables_the_server_reads(self):
        source = SERVER.read_text(encoding="utf-8")
        for name in re.findall(r"`([A-Z][A-Z0-9]*_[A-Z0-9_]+)`", README):
            with self.subTest(variable=name):
                self.assertTrue(name in source, f"README names {name}, which mcp-server.py never reads")


if __name__ == "__main__":
    unittest.main()
