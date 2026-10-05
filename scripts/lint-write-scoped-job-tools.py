#!/usr/bin/env python3
"""A job that holds a write-scoped token must not run a tool resolved at run time.

`go install module@latest` and `npx some-package` fetch whatever the module proxy or
the npm registry serves at the moment the job runs. In a job whose GITHUB_TOKEN can
write (`contents: write`, `packages: write`, `id-token: write`, ...) that code runs
with the token in reach, and no reviewed commit names its bytes. Either the tool is
fixed by the commit (an exact module version, or a package the committed lockfile
resolves) or it runs in a job that holds no write scope and hands its output on.

What it checks, per job of each `.github/workflows/*.yml` (and `*.yaml`):

  * effective permissions: the job's `permissions`, else the workflow's. A scope set
    to `write`, or `write-all`, makes the job write-scoped. A workflow with no
    `permissions` at either level gets the repository default, which can be
    read-write, so it is treated as write-scoped.
  * in every `run:` script of a write-scoped job:
      - any `<something>@latest` argument (go install, go run, npm install, npx ...);
      - `npx <package>`, `npm exec <package>` or `npm x <package>` where <package> is
        neither a top-level package nor a declared bin of one in the committed
        `package-lock.json` of the step's working directory (nearest one going up),
        or where an explicit `@version` differs from the locked version.

Jobs without a write scope are not read: that is where a run-time tool belongs.

    python3 scripts/lint-write-scoped-job-tools.py [--root DIR] [path ...]
    python3 scripts/lint-write-scoped-job-tools.py --self-test

With no path it checks the repository's `.github/workflows/`. A path may be a workflow
file or a directory of them. `--root` is the directory a step's `working-directory` is
relative to (default: the repository root). `--self-test` runs every fixture under
`scripts/testdata/write-scoped-job-tools/`: each `red-*.yml` must fail and each
`green-*.yml` must pass.

KNOWN_SITES lists the sites that predate this check. They are printed on every run,
they do not fail it, and an entry that no longer matches anything fails it, so the
list can only shrink.
Exit: 0 clean, 1 a refused tool (or a stale known site, or a failed self-test),
2 nothing to check or PyYAML missing.
"""
from __future__ import annotations

import json
import re
import shlex
import subprocess
import sys
from pathlib import Path

try:
    import yaml
except ImportError:  # pragma: no cover - reported as exit 2
    yaml = None

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_DIR = REPO_ROOT / ".github" / "workflows"
FIXTURE_DIR = REPO_ROOT / "scripts" / "testdata" / "write-scoped-job-tools"

# (workflow, job, site) present before this check existed. security.yml grants
# `security-events: write` to every job; these five run-time tools run under it.
# Fix: pin each tool to an exact version (or move it to a job with
# `permissions: contents: read`) and delete its entry here.
KNOWN_SITES = [
    (".github/workflows/security.yml", "secret-scan", "@latest hackmyagent@latest"),
    (".github/workflows/security.yml", "secret-scan", "npx hackmyagent"),
    (".github/workflows/security.yml", "go-security-scan", "@latest github.com/securego/gosec/v2/cmd/gosec@latest"),
    (".github/workflows/security.yml", "go-security-scan", "@latest golang.org/x/vuln/cmd/govulncheck@latest"),
    (".github/workflows/security.yml", "license-check", "npx license-checker"),
]

LATEST = re.compile(r"^(?P<what>[^\s@][^\s]*)@latest$")
# npx / npm exec options that consume the following word.
NPX_VALUE_FLAGS = {"-c", "--call", "--node-arg", "-n", "--shell", "-w", "--workspace", "--prefix", "--cache", "--registry"}
NPX_PACKAGE_FLAGS = {"-p", "--package"}
COMMAND_BREAK = {"&&", "||", ";", "|", "&", "(", ")", "{", "}", "then", "do", "else"}


def write_scopes(job: dict, workflow: dict) -> list[str]:
    """The write scopes a job's token holds; empty when it holds none."""
    if "permissions" in job:
        perms = job["permissions"]
    elif "permissions" in workflow:
        perms = workflow["permissions"]
    else:
        return ["repository default (no permissions block)"]
    if isinstance(perms, str):
        return ["write-all"] if perms.strip() == "write-all" else []
    if isinstance(perms, dict):
        return sorted(f"{scope}: write" for scope, level in perms.items() if str(level).strip() == "write")
    return []


def working_directory(step: dict, job: dict, workflow: dict) -> str:
    if step.get("working-directory"):
        return str(step["working-directory"])
    for holder in (job, workflow):
        run_defaults = ((holder.get("defaults") or {}).get("run") or {})
        if run_defaults.get("working-directory"):
            return str(run_defaults["working-directory"])
    return "."


def tracked(path: Path) -> bool:
    """True when git has `path` in its index: a lockfile that exists only locally is not committed."""
    try:
        result = subprocess.run(
            ["git", "-C", str(path.parent), "ls-files", "--error-unmatch", path.name],
            capture_output=True,
            check=False,
        )
    except OSError:
        return False
    return result.returncode == 0


_LOCK_CACHE: dict[Path, tuple[dict[str, str], dict[str, str]] | None] = {}


def lockfile_for(root: Path, workdir: str) -> tuple[Path, dict[str, str], dict[str, str]] | None:
    """The committed package-lock.json nearest to `workdir`, as (path, package versions, bin owners)."""
    root = root.resolve()
    current = (root / workdir).resolve()
    if root != current and root not in current.parents:
        return None
    while True:
        candidate = current / "package-lock.json"
        if candidate not in _LOCK_CACHE:
            _LOCK_CACHE[candidate] = None
            if candidate.is_file() and tracked(candidate):
                try:
                    packages = json.loads(candidate.read_text(encoding="utf-8")).get("packages") or {}
                except (OSError, ValueError):
                    packages = {}
                versions: dict[str, str] = {}
                bins: dict[str, str] = {}
                for key, entry in packages.items():
                    # Top-level installs only: a nested node_modules copy is not what npx runs.
                    if not key.startswith("node_modules/") or "/node_modules/" in key[len("node_modules/"):]:
                        continue
                    name = key[len("node_modules/"):]
                    versions[name] = str(entry.get("version", ""))
                    for bin_name in (entry.get("bin") or {}):
                        bins.setdefault(bin_name, name)
                _LOCK_CACHE[candidate] = (versions, bins)
        found = _LOCK_CACHE[candidate]
        if found is not None:
            return candidate, found[0], found[1]
        if current == root:
            return None
        current = current.parent


def split_spec(spec: str) -> tuple[str, str | None]:
    """`@scope/name@1.2.3` -> (`@scope/name`, `1.2.3`); `name` -> (`name`, None)."""
    at = spec.rfind("@")
    if at > 0:
        return spec[:at], spec[at + 1:]
    return spec, None


def words(line: str) -> list[str]:
    try:
        return shlex.split(line, comments=True)
    except ValueError:
        return re.split(r"\s+", line.split(" #", 1)[0].strip())


def npx_specs(tokens: list[str], start: int) -> list[str]:
    """The package specs an npx / npm exec invocation resolves, from the words after the command."""
    explicit: list[str] = []
    index = start
    while index < len(tokens):
        token = tokens[index]
        if token in COMMAND_BREAK:
            break
        if token == "--":
            index += 1
            continue
        if token in NPX_PACKAGE_FLAGS:
            if index + 1 < len(tokens):
                explicit.append(tokens[index + 1])
            index += 2
            continue
        if token.startswith("--package="):
            explicit.append(token.split("=", 1)[1])
            index += 1
            continue
        if token in NPX_VALUE_FLAGS:
            index += 2
            continue
        if token.startswith("-"):
            index += 1
            continue
        return explicit or [token]
    return explicit


def run_sites(script: str) -> list[tuple[str, str]]:
    """(kind, subject) for every run-time-resolved tool in a `run:` script."""
    sites: list[tuple[str, str]] = []
    for line in script.splitlines():
        tokens = words(line)
        for token in tokens:
            match = LATEST.match(token)
            if match:
                sites.append(("@latest", token))
        for index, token in enumerate(tokens):
            start = None
            if token == "npx":
                start = index + 1
            elif token == "npm" and index + 1 < len(tokens) and tokens[index + 1] in ("exec", "x"):
                start = index + 2
            if start is None:
                continue
            specs = npx_specs(tokens, start)
            if not specs:
                sites.append(("npx", "<no package could be read on this line>"))
            for spec in specs:
                sites.append(("npx", spec))
    return sites


def npx_problem(spec: str, root: Path, workdir: str) -> str | None:
    """Why this npx / npm exec package is not fixed by a committed lockfile, or None when it is."""
    name, version = split_spec(spec)
    if not name or any(ch in name for ch in "$`<>"):
        return "the package name is not a literal, so no lockfile can be shown to resolve it"
    lock = lockfile_for(root, workdir)
    if lock is None:
        return f"no committed package-lock.json at or above working directory '{workdir}'"
    path, versions, bins = lock
    try:
        shown = str(path.relative_to(root.resolve()))
    except ValueError:
        shown = str(path)
    package = name if name in versions else bins.get(name)
    if package is None:
        return f"'{name}' is not a top-level package or bin in {shown}; the registry decides what runs"
    if version is not None and version != versions[package]:
        return f"'{spec}' asks for {version} but {shown} locks {package} at {versions[package]}; the registry copy runs"
    return None


def check_file(path: Path, root: Path) -> tuple[int, list[tuple[str, str, str]]]:
    """(write-scoped jobs read, [(job, site, message)]) for one workflow file."""
    workflow = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    jobs = workflow.get("jobs") or {}
    scoped = 0
    findings: list[tuple[str, str, str]] = []
    for job_id, job in jobs.items():
        if not isinstance(job, dict):
            continue
        scopes = write_scopes(job, workflow)
        if not scopes:
            continue
        scoped += 1
        held = ", ".join(scopes)
        for step in job.get("steps") or []:
            if not isinstance(step, dict) or not isinstance(step.get("run"), str):
                continue
            workdir = working_directory(step, job, workflow)
            for kind, subject in run_sites(step["run"]):
                if kind == "@latest":
                    why = "resolved when the job runs, not by the commit"
                    site = f"@latest {subject}"
                else:
                    why = npx_problem(subject, root, workdir)
                    site = f"npx {split_spec(subject)[0]}"
                    subject = f"npx {subject}"
                if why:
                    findings.append((str(job_id), site, f"job '{job_id}' holds {held} and runs {subject}\n    {why}"))
    return scoped, findings


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


def lint(files: list[Path], root: Path, out=sys.stdout) -> int:
    known = list(KNOWN_SITES)
    scoped_jobs = 0
    failures: list[str] = []
    tolerated: list[str] = []
    seen: set[str] = set()
    for path in files:
        shown = display(path)
        seen.add(shown)
        scoped, findings = check_file(path, root)
        scoped_jobs += scoped
        for job_id, site, message in findings:
            key = (shown, job_id, site)
            if key in known:
                known.remove(key)
                tolerated.append(f"{shown}: {message.splitlines()[0]}")
            else:
                failures.append(f"{shown}: {message}")
    stale = [entry for entry in known if entry[0] in seen]

    for line in tolerated:
        print(f"known, not yet fixed: {line}", file=out)
    if failures or stale:
        print(
            f"lint-write-scoped-job-tools: {len(failures)} run-time-resolved tools in write-scoped jobs"
            + (f", {len(stale)} stale known sites" if stale else "")
            + "\n",
            file=out,
        )
        for failure in failures:
            print(failure, file=out)
        for workflow, job_id, site in stale:
            print(f"{workflow}: known site no longer present: job '{job_id}', {site}\n    remove it from KNOWN_SITES", file=out)
        print(
            "\nFix, either:\n"
            "  fix the tool by the commit: `go install <module>@vX.Y.Z` (an exact version the Go checksum\n"
            "  database verifies), or add the npm package to the package.json and package-lock.json of the\n"
            "  step's working directory and run it after `npm ci`;\n"
            "or run it without a write scope: move the step to a job with `permissions: contents: read`,\n"
            "  upload its output as an artifact, and let a separate job that runs only sha-pinned actions\n"
            "  hold the write scope.\n"
            "Verify: python3 scripts/lint-write-scoped-job-tools.py",
            file=out,
        )
        return 1
    print(
        f"lint-write-scoped-job-tools: {scoped_jobs} write-scoped jobs across {len(files)} workflow files, "
        f"no run-time-resolved tool outside the {len(tolerated)} known sites",
        file=out,
    )
    return 0


def self_test() -> int:
    import io

    fixtures = sorted(FIXTURE_DIR.glob("*.yml"))
    red = [p for p in fixtures if p.name.startswith("red-")]
    green = [p for p in fixtures if p.name.startswith("green-")]
    if not red or not green:
        print(f"lint-write-scoped-job-tools: no red-*.yml or no green-*.yml under {display(FIXTURE_DIR)}", file=sys.stderr)
        return 2
    wrong = 0
    for path in red + green:
        want = 1 if path in red else 0
        got = lint([path], FIXTURE_DIR / "root", out=io.StringIO())
        verdict = "ok" if got == want else "WRONG"
        wrong += got != want
        print(f"{verdict}: {path.name} exit {got} (want {want})")
    if wrong:
        print(f"lint-write-scoped-job-tools: self-test failed on {wrong} of {len(red) + len(green)} fixtures")
        return 1
    print(f"lint-write-scoped-job-tools: self-test passed, {len(red)} refused and {len(green)} accepted fixtures")
    return 0


def main(argv: list[str]) -> int:
    if yaml is None:
        print("lint-write-scoped-job-tools: PyYAML is required (pip install pyyaml)", file=sys.stderr)
        return 2
    if argv == ["--self-test"]:
        return self_test()
    root = REPO_ROOT
    if argv[:1] == ["--root"]:
        if len(argv) < 2:
            print("lint-write-scoped-job-tools: --root needs a directory", file=sys.stderr)
            return 2
        root, argv = Path(argv[1]), argv[2:]
    files = workflow_files(argv)
    if not files:
        print("lint-write-scoped-job-tools: no workflow files found to check", file=sys.stderr)
        return 2
    return lint(files, root)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
