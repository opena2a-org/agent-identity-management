#!/usr/bin/env python3
"""Every `uses:` reference in the workflows must be pinned to a full commit sha.

A tag or branch on another repository is a mutable pointer: whoever controls that
repository can move `v4` or `master` to new code, and the next run of a job that
holds `contents: write`, `packages: write` or `id-token: write` executes it with
those permissions. A 40-hex commit sha names exact bytes, and the version it was
taken from stays readable in a trailing comment:

    - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262 # v4

What it checks, line by line over `.github/workflows/*.yml` (and `*.yaml`):

  * `owner/repo[/path]@ref`      ref must be 40 lowercase hex characters;
  * `docker://image...`          must carry an `@sha256:<64 hex>` digest;
  * `./path`                     a local action, versioned with the workflow: allowed.

Comment lines and text inside `run:` scripts are not `uses:` keys and are not read.

    python3 scripts/lint-action-pins.py [path ...]

With no arguments it checks the repository's `.github/workflows/`. A path may be a
workflow file or a directory of them.
Exit: 0 every reference pinned, 1 an unpinned reference, 2 nothing to check.
"""
from __future__ import annotations

import re
import sys
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_DIR = REPO_ROOT / ".github" / "workflows"

USES_KEY = re.compile(r"^\s*(?:-\s+)?uses:\s*(?P<value>.*)$")
SHA = re.compile(r"^[0-9a-f]{40}$")
DIGEST = re.compile(r"@sha256:[0-9a-f]{64}$")


def reference(raw: str) -> str:
    """The `uses:` value without its trailing comment or surrounding quotes."""
    value = raw.strip()
    if value[:1] in ("'", '"'):
        quote = value[0]
        end = value.find(quote, 1)
        return value[1:end] if end > 0 else value[1:]
    return re.split(r"\s+#", value, maxsplit=1)[0].strip()


def problem(ref: str) -> str | None:
    """Why `ref` is not pinned, or None when it is."""
    if not ref:
        return "the value could not be read on this line"
    if ref.startswith("./"):
        return None
    if ref.startswith("docker://"):
        return None if DIGEST.search(ref) else "a docker:// image without an @sha256: digest"
    action, sep, version = ref.rpartition("@")
    if not sep or not action:
        return "no @ref, so it runs the default branch as it is at run time"
    if SHA.match(version):
        return None
    return f"'{version}' is not a full commit sha; a tag or branch can be moved by its repository"


def workflow_files(args: list[str]) -> list[Path]:
    targets = [Path(a) for a in args] or [DEFAULT_DIR]
    files: list[Path] = []
    for target in targets:
        if target.is_dir():
            files.extend(sorted(p for p in target.iterdir() if p.suffix in (".yml", ".yaml")))
        elif target.is_file():
            files.append(target)
    return files


def display(path: Path) -> str:
    try:
        return str(path.resolve().relative_to(REPO_ROOT))
    except ValueError:
        return str(path)


def main(argv: list[str]) -> int:
    files = workflow_files(argv)
    if not files:
        print("lint-action-pins: no workflow files found to check", file=sys.stderr)
        return 2

    checked = 0
    failures: list[str] = []
    for path in files:
        for number, line in enumerate(path.read_text(encoding="utf-8").splitlines(), start=1):
            match = USES_KEY.match(line)
            if not match:
                continue
            checked += 1
            ref = reference(match.group("value"))
            why = problem(ref)
            if why:
                failures.append(f"{display(path)}:{number}: uses: {ref}\n    {why}")

    if failures:
        print(f"lint-action-pins: {len(failures)} of {checked} uses: references are not pinned to a commit sha\n")
        for failure in failures:
            print(failure)
        print(
            "\nFix: resolve the commit the tag names today from the action's own repository,\n"
            "  gh api repos/<owner>/<repo>/git/ref/tags/<tag>\n"
            "  (for an annotated tag, follow .object.sha through gh api repos/<owner>/<repo>/git/tags/<sha>)\n"
            "then write it with the version kept as a comment:\n"
            "  uses: <owner>/<repo>@<40-hex sha> # <tag>\n"
            "Verify: python3 scripts/lint-action-pins.py"
        )
        return 1

    print(f"lint-action-pins: {checked} uses: references across {len(files)} workflow files, all pinned to a commit sha")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
