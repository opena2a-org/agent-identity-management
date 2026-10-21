#!/usr/bin/env python3
"""The fixture administrator of docs/demo/quickstart.tape.

The README's Quick start has three steps an administrator performs in the
dashboard. The terminal recording stays in the terminal, so this process
performs them off screen, through the API the dashboard pages call, and the
sidecar lists them under `edits`:

1. Before the tape: Global Enforcement Mode set to strict ("The refusal below
   needs strict mode"). The process writes <out>/cue-ready when it is set.
2. While the tape runs: each device code that a watched file shows is approved
   ("a code you approve on your dashboard's /device page"), two seconds after
   it appears, and my-first-agent is verified once it is registered and
   pending ("The agent starts pending until an administrator verifies it").
   Each verification writes <out>/cue-verified.
3. Between the recording and the text capture, on <out>/req-reset:
   my-first-agent is deleted, so the capture registers it again as a first run.
   The process writes <out>/cue-reset when it is gone, and exits on
   <out>/req-stop.

It reads the administrator's email and password from stdin, one per line,
never from an argument or the environment, and prints neither; its log names
actions, never a code, a token or an ID. Standard library only.

    printf '%s\\n%s\\n' "$email" "$password" | \\
      quickstart-admin.py --api http://localhost:8080 --out /work/out --watch FILE [--watch FILE ...]
"""
import argparse
import json
import os
import re
import sys
import time
import urllib.error
import urllib.request

AGENT_NAME = "my-first-agent"
CODE = re.compile(r"Code:\s+([A-Z0-9]{4}-[A-Z0-9]{4})")
APPROVE_AFTER_SECONDS = 2.0


def log(message):
    print(time.strftime("%H:%M:%SZ", time.gmtime()) + " admin: " + message, file=sys.stderr, flush=True)


class Api:
    def __init__(self, base):
        self.base = base.rstrip("/") + "/api/v1"
        self.token = None

    def call(self, method, path, body=None):
        data = None if body is None else json.dumps(body).encode()
        request = urllib.request.Request(self.base + path, data=data, method=method)
        request.add_header("Accept", "application/json")
        if data is not None:
            request.add_header("Content-Type", "application/json")
        if self.token:
            request.add_header("Authorization", "Bearer " + self.token)
        try:
            with urllib.request.urlopen(request, timeout=15) as response:
                raw = response.read()
                return response.status, (json.loads(raw) if raw else None)
        except urllib.error.HTTPError as err:
            return err.code, None

    def sign_in(self, email, password):
        status, body = self.call("POST", "/auth/login/local", {"email": email, "password": password})
        if status != 200 or not body or not body.get("accessToken"):
            raise SystemExit(f"admin: the fixture administrator could not sign in (HTTP {status})")
        self.token = body["accessToken"]

    def agent(self):
        status, body = self.call("GET", "/agents?limit=100")
        if status != 200 or not body:
            return None
        return next((a for a in body.get("agents") or [] if a.get("name") == AGENT_NAME), None)


def cue(out, name):
    with open(os.path.join(out, "cue-" + name), "w") as f:
        f.write(repr(time.time()))


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--api", required=True)
    parser.add_argument("--out", required=True)
    parser.add_argument("--watch", action="append", default=[])
    args = parser.parse_args()

    email = sys.stdin.readline().strip()
    password = sys.stdin.readline().rstrip("\n")
    if not email or not password:
        raise SystemExit("admin: expected the administrator's email and password on stdin")
    api = Api(args.api)
    api.sign_in(email, password)
    del password

    status, _ = api.call("PUT", "/admin/enforcement-settings", {"enforcementMode": "strict"})
    if status not in (200, 204):
        raise SystemExit(f"admin: setting strict mode failed (HTTP {status})")
    log("organization set to strict mode")
    cue(args.out, "ready")

    seen = {}  # device code -> time first seen
    approved = set()
    verified = set()
    req = lambda name: os.path.join(args.out, "req-" + name)
    while not os.path.exists(req("stop")):
        for path in args.watch:
            try:
                text = open(path, encoding="utf-8", errors="replace").read()
            except OSError:
                continue
            for code in CODE.findall(text):
                seen.setdefault(code, time.time())
        for code, first in seen.items():
            if code in approved or time.time() - first < APPROVE_AFTER_SECONDS:
                continue
            status, _ = api.call("POST", "/oauth/device/approve", {"userCode": code})
            approved.add(code)
            log(f"device code approved (HTTP {status})")

        if os.path.exists(req("reset")):
            agent = api.agent()
            if agent:
                status, _ = api.call("DELETE", f"/agents/{agent['id']}")
                log(f"{AGENT_NAME} deleted for the text capture (HTTP {status})")
            os.remove(req("reset"))
            cue(args.out, "reset")
        elif not os.path.exists(os.path.join(args.out, "cue-verified")):
            agent = api.agent()
            if agent and agent.get("status") == "pending" and agent["id"] not in verified:
                status, _ = api.call("POST", f"/agents/{agent['id']}/verify")
                if status == 200:
                    verified.add(agent["id"])
                    cue(args.out, "verified")
                log(f"{AGENT_NAME} verified (HTTP {status})")
        time.sleep(0.5)
    log("stopped")


if __name__ == "__main__":
    main()
