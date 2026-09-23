#!/usr/bin/env python3
"""Pin the OpenA2A Registry's HTTP route table for the backend's sender/receiver drift test.

The AIM backend builds a handful of Registry URLs by hand (the contribution bridge, the
ATC issuer). Nothing checks that the path a sender targets is a route the Registry
registers, and one sender (the community-intelligence push) targeted a route that never
existed: every run was a silent 404. This script reads the Registry's server main file,
resolves Fiber group prefixes statically, and writes the full route table to
`apps/backend/testdata/registry-routes.txt`; `registry_routes_drift_test.go` asserts every
path the backend targets is in it.

    python3 scripts/pin-registry-routes.py /path/to/opena2a-registry [--write]

Without --write the table is printed. The header records the Registry commit the pin was
taken from, so a reader knows what the check is measured against.
"""
from __future__ import annotations

import re
import subprocess
import sys
from pathlib import Path

PIN = Path(__file__).resolve().parent.parent / "apps" / "backend" / "testdata" / "registry-routes.txt"


def resolve(src: str) -> tuple[list[tuple[str, str]], int]:
    groups = {"app": ""}
    assignments = re.findall(r'(\w+)\s*:?=\s*(\w+)\.Group\("([^"]*)"', src)
    for _ in range(6):  # groups nest a few levels; iterate until every parent is known
        for var, parent, prefix in assignments:
            if parent in groups:
                groups[var] = groups[parent] + prefix
    routes: set[tuple[str, str]] = set()
    unresolved = 0
    for var, method, path in re.findall(r'(\w+)\.(Get|Post|Put|Patch|Delete|All)\("([^"]*)"', src):
        if var in groups:
            routes.add((method.upper(), groups[var] + path))
        else:
            unresolved += 1
    return sorted(routes), unresolved


def main(argv: list[str]) -> int:
    if not argv:
        print(__doc__)
        return 2
    repo = Path(argv[0]).expanduser().resolve()
    main_go = repo / "cmd" / "server" / "main.go"
    src = main_go.read_text(encoding="utf-8")
    sha = subprocess.run(["git", "-C", str(repo), "rev-parse", "HEAD"], capture_output=True, text=True).stdout.strip()
    routes, unresolved = resolve(src)
    lines = [
        "# OpenA2A Registry route table, resolved from cmd/server/main.go by scripts/pin-registry-routes.py.",
        f"# registry commit {sha or 'unknown'}; {len(routes)} routes; {unresolved} registrations on receivers this resolver does not follow.",
        "# METHOD path, one per line. Regenerate after the Registry changes its routes.",
    ] + [f"{m} {p}" for m, p in routes]
    text = "\n".join(lines) + "\n"
    if "--write" in argv:
        PIN.parent.mkdir(parents=True, exist_ok=True)
        PIN.write_text(text, encoding="utf-8")
        print(f"wrote {PIN} ({len(routes)} routes at {sha[:8] if sha else 'unknown'})")
    else:
        sys.stdout.write(text)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
