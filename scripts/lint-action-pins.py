#!/usr/bin/env python3
"""Every `uses:` reference in the workflows is pinned to a full commit sha that the
action's own repository reaches from its default branch.

A tag or branch on another repository is a mutable pointer: whoever controls that
repository can move `v4` or `master` to new code, and the next run of a job that
holds `contents: write`, `packages: write` or `id-token: write` executes it with
those permissions. A 40-hex commit sha names exact bytes, and the version it was
taken from stays readable in a trailing comment:

    - uses: actions/checkout@11d5960a326750d5838078e36cf38b85af677262 # v4

A sha alone is not enough. The host serves every object of a fork network under
the parent repository's name, so `owner/repo@<sha>` resolves a commit that exists
only in somebody's fork, and a pin taken while a repository is compromised can
name a commit none of its branches carry. So each pinned commit must also be
REACHABLE: an ancestor of the action repository's default branch or, only where
ALLOWLIST below names the fully-qualified owner/repo at the pinned major M, of
that entry's one branch, which must be `releases/vM`. M is read from the trailing
comment (`# v4`, `# v4.4.0`, `# release/v1`); a pin without a version comment is
judged on the default branch alone.

How reachability is decided:

  * `git ls-remote --symref <url> HEAD` names the default branch;
  * `git fetch --filter=tree:0 <url> +refs/heads/<branch>:<ref>` brings that
    branch's commits, without trees or blobs, from the action's own repository
    into a scratch clone under $RUNNER_TEMP (else the system temporary directory);
  * `git merge-base --is-ancestor <sha> <ref>` exiting 0 is the only evidence.

What never counts: object existence, a fetch by sha, and the head sha itself
standing in for an ancestry test. A filtered fetch marks the URL as a promisor,
and git then fetches any missing object BY SHA on first use, which is the lookup
that resolves fork-only commits; so lazy fetching is switched off
(GIT_NO_LAZY_FETCH, and the promisor setting is removed after each fetch) and a
commit outside the fetched history stays absent. A branch's whole history is
fetched rather than a --depth slice: without trees it is small, and a shallow
history can only say "not within N commits", never "not an ancestor". A failed
ls-remote or fetch is a named refusal, never a skip.

What it reads, in one pass, parsing each file as YAML (so comment lines and text
inside `run:` scripts are never `uses:` keys):

  * `jobs.<id>.steps[*].uses` in `.github/workflows/*.yml` (and `*.yaml`): actions;
  * `jobs.<id>.uses`: reusable workflows (`owner/repo/path@ref`), same rules;
  * `docker://image...`: must carry an `@sha256:<64 hex>` digest;
  * `./path` in either position: versioned with this repository, so not pinned,
    but the file it names is read by the same pass: a local reusable workflow's
    jobs, a local action's `runs.steps[*].uses`, and a local docker action's
    `runs.image` when it is a `docker://` reference.

Left out, with the reason: the `uses:` lines inside a REMOTE action (a composite
action that calls other actions). The pinned sha fixes those bytes, but reading
them needs the action's tree at that commit, and this check fetches commits only.
Images a container action builds or pulls for itself are not `uses:` references;
scripts/lint-runtime-image-pins.py covers the images write-scoped jobs name.

Rule ids in the findings:

  mutable-ref        a tag, a branch, a short sha, or no @ref at all;
  unreachable        a pinned commit that no permitted upstream branch reaches;
  docker-digest      a docker:// image without @sha256:<digest>;
  unreadable         a value that is not a string or names no owner/repo;
  local-unreadable   a ./path naming no workflow file or action.yml in this repository;
  allowlist-entry    an ALLOWLIST key that is not owner/repo@M;
  allowlist-branch   an ALLOWLIST entry naming any branch other than releases/vM;
  allowlist-unused   an ALLOWLIST entry that no workflow pin uses.

    python3 scripts/lint-action-pins.py [path ...]

With no arguments it checks the repository's `.github/workflows/`. A path may be a
workflow file or a directory of them. ALLOWLIST use is counted over the
repository's workflows as well as the paths given.

Exit: 0 every reference pinned and reachable; 1 a finding; 2 a named refusal, the
check could not decide (no workflow files, PyYAML missing, git missing, a file that
does not parse, an ls-remote or fetch that failed). 2 is never a pass.
Needs PyYAML (scripts/requirements-ci-lint.txt), git, and HTTPS to github.com.
"""
from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys
import tempfile
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path

try:
    import yaml
except ImportError:  # reported as a named refusal, exit 2
    yaml = None

REPO_ROOT = Path(__file__).resolve().parent.parent
WORKFLOWS = Path(".github") / "workflows"
GITHUB = "https://github.com"
GIT_TIMEOUT = 120

# owner/repo@M -> the one branch besides the default branch that may reach a pin
# whose comment names major M. The branch must be releases/vM: these actions cut
# an older major's releases on a maintenance branch after the next major moved to
# the default branch. An entry that names any other branch, or that no workflow
# pin uses, fails the check. Evidence for each entry: the pinned commit is not an
# ancestor of the default branch, and it is one of releases/vM:
#   git ls-remote https://github.com/<owner>/<repo>.git refs/heads/releases/vM
ALLOWLIST: dict[str, str] = {
    "actions/checkout@4": "releases/v4",
    "actions/setup-go@5": "releases/v5",
    "actions/setup-java@4": "releases/v4",
    "dorny/paths-filter@3": "releases/v3",
    "github/codeql-action@3": "releases/v3",
}

SHA = re.compile(r"^[0-9a-f]{40}$")
DIGEST = re.compile(r"@sha256:[0-9a-f]{64}$")
OWNER = re.compile(r"^[A-Za-z0-9][A-Za-z0-9-]{0,38}$")
NAME = re.compile(r"^[A-Za-z0-9._-]{1,100}$")
ENTRY = re.compile(r"^(?P<repo>[^/@\s]+/[^/@\s]+)@(?P<major>\d+)$")
BRANCH = re.compile(r"^[A-Za-z0-9._/-]+$")
SYMREF = re.compile(r"^ref: refs/heads/(?P<branch>\S+)\s+HEAD$")
PROMISOR = r"^(remote\..*\.(promisor|partialclonefilter)|extensions\.partialclone)$"

FIXES = {
    "mutable-ref": (
        "resolve the commit the tag names today from the action's own repository,\n"
        "  git ls-remote https://github.com/<owner>/<repo>.git 'refs/tags/<tag>' 'refs/tags/<tag>^{}'\n"
        "  (the ^{} line, when present, is the commit an annotated tag points at)\n"
        "then write it with the version kept as a comment:\n"
        "  uses: <owner>/<repo>@<40-hex sha> # <tag>"
    ),
    "unreachable": (
        "pin a commit the action's default branch carries, normally a release tag cut there:\n"
        "  git ls-remote --symref https://github.com/<owner>/<repo>.git HEAD 'refs/tags/<tag>' 'refs/tags/<tag>^{}'\n"
        "If the action cuts major M on releases/vM, check that branch reaches the pin,\n"
        "  git ls-remote https://github.com/<owner>/<repo>.git refs/heads/releases/vM\n"
        "and add '<owner>/<repo>@M': 'releases/vM' to ALLOWLIST in scripts/lint-action-pins.py.\n"
        "A commit no upstream branch reaches is not a release of that action."
    ),
    "docker-digest": (
        "resolve the digest the tag names today, docker buildx imagetools inspect <image>:<tag>,\n"
        "then write docker://<image>:<tag>@sha256:<digest>"
    ),
    "unreadable": "write the value as one string, <owner>/<repo>[/path]@<40-hex sha> # <version>",
    "local-unreadable": "point ./path at a workflow file, or at a directory holding action.yml, inside this repository",
    "allowlist-entry": "key each ALLOWLIST entry '<owner>/<repo>@M' in scripts/lint-action-pins.py",
    "allowlist-branch": "an ALLOWLIST entry '<owner>/<repo>@M' may name releases/vM and no other branch",
    "allowlist-unused": "remove the ALLOWLIST entry from scripts/lint-action-pins.py; no workflow pins that action at that major",
}


class Refusal(Exception):
    """The check cannot decide. Reported by name with exit 2, never a pass."""


@dataclass(frozen=True)
class Use:
    file: Path
    line: int
    key: str             # "uses", or "image" for a local docker action
    value: str | None    # None when the value is not a string
    comment: str


@dataclass(frozen=True)
class Finding:
    file: str
    line: int
    rule: str
    what: str
    why: str

    def __str__(self) -> str:
        return f"{self.file}:{self.line}: [{self.rule}] {self.what}\n    {self.why}"


def display(path: Path, root: Path) -> str:
    try:
        return path.resolve().relative_to(root).as_posix()
    except ValueError:
        return str(path)


def count(n: int, noun: str) -> str:
    return f"{n} {noun}" + ("" if n == 1 else "s")


def last_line(text: str) -> str:
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    return lines[-1][:300] if lines else "no message"


# ---------------------------------------------------------------------------
# Reading uses: keys
# ---------------------------------------------------------------------------


def child(node, key: str):
    if isinstance(node, yaml.MappingNode):
        for k, v in node.value:
            if isinstance(k, yaml.ScalarNode) and k.value == key:
                return v
    return None


def entries(node) -> list:
    return list(node.value) if isinstance(node, yaml.SequenceNode) else []


def values(node) -> list:
    return [v for _, v in node.value] if isinstance(node, yaml.MappingNode) else []


def use_at(path: Path, lines: list[str], node, key: str) -> Use:
    line = node.start_mark.line + 1
    if not isinstance(node, yaml.ScalarNode):
        return Use(path, line, key, None, "")
    comment = ""
    end = node.end_mark
    if node.start_mark.line == end.line and end.line < len(lines):
        found = re.match(r"\s*#\s*(.*?)\s*$", lines[end.line][end.column:])
        comment = found.group(1) if found else ""
    return Use(path, line, key, node.value.strip(), comment)


def read_uses(path: Path, kind: str) -> list[Use]:
    """Every uses: key of a workflow file, or of an action metadata file (kind 'action')."""
    text = path.read_text(encoding="utf-8")
    root = yaml.compose(text, Loader=yaml.SafeLoader)
    lines = text.splitlines()
    found: list[Use] = []

    def steps(container) -> None:
        for step in entries(child(container, "steps")):
            node = child(step, "uses")
            if node is not None:
                found.append(use_at(path, lines, node, "uses"))

    if kind == "workflow":
        for job in values(child(root, "jobs")):
            node = child(job, "uses")
            if node is not None:
                found.append(use_at(path, lines, node, "uses"))
            steps(job)
    else:
        runs = child(root, "runs")
        steps(runs)
        image = child(runs, "image")
        if isinstance(image, yaml.ScalarNode) and image.value.strip().startswith("docker://"):
            found.append(use_at(path, lines, image, "image"))
    return found


def local_file(root: Path, value: str) -> tuple[Path, str] | None:
    """The workflow file or action metadata file a ./path names, with its kind."""
    target = (root / value[2:]).resolve()
    if target != root and root not in target.parents:
        return None
    if target.is_file() and target.suffix in (".yml", ".yaml"):
        return target, "workflow"
    for name in ("action.yml", "action.yaml"):
        if (target / name).is_file():
            return target / name, "action"
    return None


class Reader:
    """Parses each file once and follows ./ references from the files it starts at."""

    def __init__(self, root: Path) -> None:
        self.root = root
        self.parsed: dict[Path, list[Use]] = {}
        self.refusals: list[str] = []

    def uses(self, path: Path, kind: str) -> list[Use]:
        key = path.resolve()
        if key not in self.parsed:
            try:
                self.parsed[key] = read_uses(path, kind)
            except (OSError, UnicodeDecodeError, yaml.YAMLError) as err:
                self.refusals.append(f"cannot parse {display(path, self.root)}: {' '.join(str(err).split())}")
                self.parsed[key] = []
        return self.parsed[key]

    def walk(self, start: list[Path]) -> tuple[list[Use], int]:
        """The uses of each start file and of every ./ file they reach, and the file count."""
        queue = [(path, "workflow") for path in start]
        seen: set[Path] = set()
        found: list[Use] = []
        while queue:
            path, kind = queue.pop(0)
            if path.resolve() in seen:
                continue
            seen.add(path.resolve())
            for use in self.uses(path, kind):
                found.append(use)
                if use.value and use.value.startswith("./"):
                    target = local_file(self.root, use.value)
                    if target:
                        queue.append(target)
        return found, len(seen)


def workflow_files(args: list[str], root: Path) -> list[Path]:
    targets = [Path(a) for a in args] or [root / WORKFLOWS]
    files: list[Path] = []
    for target in targets:
        if target.is_dir():
            files.extend(sorted(p for p in target.iterdir() if p.is_file() and p.suffix in (".yml", ".yaml")))
        elif target.is_file():
            files.append(target)
    return files


# ---------------------------------------------------------------------------
# The rules
# ---------------------------------------------------------------------------


def repo_of(action: str) -> str | None:
    """owner/repo, lower-cased, from `owner/repo[/path]`."""
    parts = action.split("/")
    if len(parts) < 2 or not OWNER.match(parts[0]) or not NAME.match(parts[1]) or parts[1] in (".", ".."):
        return None
    return f"{parts[0]}/{parts[1]}".lower()


def pinned_major(comment: str) -> int | None:
    """The major version a trailing comment names: v4, v4.4.0, 4.2.2, release/v1."""
    found = re.match(r"v?(\d+)(?:\.\d+)*(?:\s|$)", comment) or re.search(r"(?:^|[\s/])v(\d+)(?:\.\d+)*(?:\s|$)", comment)
    return int(found.group(1)) if found else None


def static_problem(value: str) -> tuple[str, str] | None:
    """(rule, why) when a non-local value is not a pin, else None."""
    if value.startswith("docker://"):
        if DIGEST.search(value):
            return None
        return "docker-digest", "a docker:// image without an @sha256: digest; the registry resolves the tag at run time"
    action, sep, version = value.rpartition("@")
    if not sep or not action:
        return "mutable-ref", "no @ref, so it runs the default branch as it is at run time"
    if not SHA.match(version):
        return "mutable-ref", f"'{version}' is not a full commit sha; a tag or branch can be moved by its repository"
    if repo_of(action) is None:
        return "unreadable", f"'{action}' does not name an owner/repo"
    return None


def why_unreachable(oracle, repo: str, sha: str, major: int | None, honoured: dict) -> str | None:
    """None when `sha` is an ancestor of a branch it may be reached from, else why it is not."""
    default = oracle.default_branch(repo)
    if oracle.reachable(repo, sha, default):
        return None
    head = oracle.fetched.get((repo, default), "")
    not_default = (
        f"{sha[:12]} is not an ancestor of {default}, the default branch of github.com/{repo}"
        + (f" (head {head[:12]} as fetched)" if head else "")
    )
    branch = honoured.get((repo, major)) if major is not None else None
    if branch is None:
        if major is None:
            return f"{not_default}; with no version in its trailing comment no ALLOWLIST entry can apply"
        return f"{not_default}, and ALLOWLIST names no release branch for {repo}@{major}"
    if oracle.branch_head(repo, branch) is None:
        return f"{not_default}, and {branch}, which ALLOWLIST names for {repo}@{major}, does not exist upstream"
    if oracle.reachable(repo, sha, branch):
        return None
    head = oracle.fetched.get((repo, branch), "")
    return (
        f"{not_default}, nor of {branch}, the branch ALLOWLIST names for {repo}@{major}"
        + (f" (head {head[:12]} as fetched)" if head else "")
    )


def allowlist_line(key: str) -> int:
    try:
        for number, line in enumerate(Path(__file__).read_text(encoding="utf-8").splitlines(), start=1):
            if f'"{key}":' in line:
                return number
    except OSError:
        pass
    return 0


def check_allowlist(allowlist: dict[str, str], used: set, findings: list[Finding]) -> dict:
    """Validates every entry; returns the honoured ones as {(repo, major): branch}."""
    where = display(Path(__file__), REPO_ROOT)
    honoured: dict[tuple[str, int], str] = {}
    for key, branch in allowlist.items():
        what = f"{key} -> {branch}"
        line = allowlist_line(key)
        found = ENTRY.match(key)
        repo = repo_of(found.group("repo")) if found else None
        if repo is None:
            findings.append(Finding(where, line, "allowlist-entry", what, "an entry is keyed owner/repo@M, M the pinned major version"))
            continue
        major = int(found.group("major"))
        if branch != f"releases/v{major}":
            findings.append(Finding(
                where, line, "allowlist-branch", what,
                f"the one branch an entry for major {major} may name is releases/v{major}; this entry is not honoured",
            ))
            continue
        if (repo, major) not in used:
            findings.append(Finding(
                where, line, "allowlist-unused", what,
                f"no workflow pins {repo} with a v{major} version comment, so the entry only widens what is accepted",
            ))
            continue
        honoured[(repo, major)] = branch
    return honoured


# ---------------------------------------------------------------------------
# The oracle: git against each action's own repository
# ---------------------------------------------------------------------------


class GitOracle:
    """Decides reachability from upstream branch heads with git alone."""

    def __init__(self, scratch: Path, base_url: str = GITHUB, git: str | None = None) -> None:
        found = git or shutil.which("git")
        if not found:
            raise Refusal(
                "git is not on PATH. Reachability is decided with git ls-remote, git fetch and "
                "git merge-base; without git no pin can be judged, and an unjudged pin is not a pass."
            )
        self.git = found
        self.scratch = scratch
        self.base_url = base_url.rstrip("/")
        self.fetched: dict[tuple[str, str], str] = {}
        self._defaults: dict[str, str] = {}
        self._clones: dict[str, Path] = {}
        self._refs: dict[tuple[str, str], str] = {}
        self._failed: dict[tuple[str, ...], Refusal] = {}  # a failed lookup is not retried per pin
        # Inherited GIT_* variables could redirect these commands to another repository
        # (GIT_DIR) or switch lazy fetching back on; none are kept.
        self.env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_") and k != "SSH_ASKPASS"}
        self.env.update({
            "GIT_TERMINAL_PROMPT": "0",       # an auth challenge fails instead of waiting
            "GIT_NO_LAZY_FETCH": "1",         # a missing object stays missing
            "GIT_CONFIG_NOSYSTEM": "1",
            "GIT_CONFIG_GLOBAL": os.devnull,  # no insteadOf rewrite, credential helper or extra header
        })

    def url(self, repo: str) -> str:
        return f"{self.base_url}/{repo}.git"

    def run(self, *args: str, where: Path | None = None) -> subprocess.CompletedProcess:
        command = [self.git]
        if where is not None:
            command += ["-C", str(where)]
        command += ["-c", "credential.helper=", "-c", "http.extraHeader=", *args]
        try:
            return subprocess.run(
                command, env=self.env, capture_output=True, text=True, errors="replace",
                timeout=GIT_TIMEOUT, check=False,
            )
        except subprocess.TimeoutExpired:
            raise Refusal(f"git {' '.join(args)} did not finish within {GIT_TIMEOUT}s") from None

    def default_branch(self, repo: str) -> str:
        if ("default", repo) in self._failed:
            raise self._failed[("default", repo)]
        try:
            return self._default_branch(repo)
        except Refusal as err:
            self._failed[("default", repo)] = err
            raise

    def _default_branch(self, repo: str) -> str:
        if repo not in self._defaults:
            url = self.url(repo)
            done = self.run("ls-remote", "--symref", url, "HEAD")
            if done.returncode != 0:
                raise Refusal(f"git ls-remote --symref {url} HEAD failed (exit {done.returncode}): {last_line(done.stderr)}")
            branches = [m.group("branch") for m in map(SYMREF.match, done.stdout.splitlines()) if m]
            if len(branches) != 1 or not BRANCH.match(branches[0]):
                raise Refusal(f"git ls-remote --symref {url} HEAD named no default branch")
            self._defaults[repo] = branches[0]
        return self._defaults[repo]

    def branch_head(self, repo: str, branch: str) -> str | None:
        """The branch's head as the repository advertises it, or None when there is no such branch."""
        url = self.url(repo)
        done = self.run("ls-remote", url, f"refs/heads/{branch}")
        if done.returncode != 0:
            raise Refusal(f"git ls-remote {url} refs/heads/{branch} failed (exit {done.returncode}): {last_line(done.stderr)}")
        for line in done.stdout.splitlines():
            sha, _, ref = line.partition("\t")
            if ref.strip() == f"refs/heads/{branch}":
                return sha.strip()
        return None

    def clone(self, repo: str) -> Path:
        if repo not in self._clones:
            path = self.scratch / f"{len(self._clones)}.git"
            done = self.run("init", "--quiet", "--bare", str(path))
            if done.returncode != 0:
                raise Refusal(f"git init of a scratch clone failed (exit {done.returncode}): {last_line(done.stderr)}")
            self._clones[repo] = path
        return self._clones[repo]

    def fetch(self, repo: str, branch: str) -> str:
        """Fetches the branch's commits and returns the local ref holding its head."""
        key = (repo, branch)
        if key in self._refs:
            return self._refs[key]
        if ("fetch", repo, branch) in self._failed:
            raise self._failed[("fetch", repo, branch)]
        try:
            return self._fetch(repo, branch)
        except Refusal as err:
            self._failed[("fetch", repo, branch)] = err
            raise

    def _fetch(self, repo: str, branch: str) -> str:
        key = (repo, branch)
        url = self.url(repo)
        if not BRANCH.match(branch):
            raise Refusal(f"{url} names a branch this check will not fetch: {branch!r}")
        clone = self.clone(repo)
        local = f"refs/upstream/{len(self._refs)}"
        done = self.run(
            "fetch", "--quiet", "--no-tags", "--no-recurse-submodules", "--filter=tree:0",
            url, f"+refs/heads/{branch}:{local}", where=clone,
        )
        if done.returncode != 0:
            raise Refusal(f"git fetch of {branch} from {url} failed (exit {done.returncode}): {last_line(done.stderr)}")
        listed = self.run("config", "--get-regexp", PROMISOR, where=clone)
        for line in listed.stdout.splitlines():
            self.run("config", "--unset-all", line.split(" ", 1)[0], where=clone)
        if self.run("config", "--get-regexp", PROMISOR, where=clone).stdout.strip():
            raise Refusal(f"could not switch lazy fetching off in the scratch clone of {url}")
        head = self.run("rev-parse", "--verify", "--quiet", f"{local}^{{commit}}", where=clone)
        if head.returncode != 0:
            raise Refusal(f"the fetch of {branch} from {url} stored no commit")
        self._refs[key] = local
        self.fetched[key] = head.stdout.strip()
        return local

    def reachable(self, repo: str, sha: str, branch: str) -> bool:
        local = self.fetch(repo, branch)
        clone = self._clones[repo]
        # Nothing here fetches by sha, so a commit outside the fetched history is absent,
        # and an absent commit is no ancestor. Presence alone proves nothing.
        if self.run("cat-file", "-e", f"{sha}^{{commit}}", where=clone).returncode != 0:
            return False
        done = self.run("merge-base", "--is-ancestor", sha, local, where=clone)
        if done.returncode in (0, 1):
            return done.returncode == 0
        raise Refusal(
            f"git merge-base --is-ancestor {sha} {branch} failed in the scratch clone of "
            f"{self.url(repo)} (exit {done.returncode}): {last_line(done.stderr)}"
        )


# ---------------------------------------------------------------------------


def report(findings: list[Finding], refusals: list[str], fetched: dict, uses: int, files: int) -> None:
    if findings:
        print(f"lint-action-pins: {count(len(findings), 'finding')} in {count(uses, 'uses: reference')} across {count(files, 'file')}\n")
        for finding in sorted(findings, key=lambda f: (f.file, f.line, f.rule)):
            print(finding)
        print()
        for rule in sorted({f.rule for f in findings}):
            print(f"Fix ({rule}): {FIXES[rule]}")
        print("Verify: python3 scripts/lint-action-pins.py")
    if fetched:
        when = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
        print(f"\nReachability decided {when} by git merge-base against {count(len(fetched), 'fetched branch head')}:")
        repos: dict[str, list[str]] = {}
        for (repo, branch), head in fetched.items():
            repos.setdefault(repo, []).append(f"{branch}@{head[:12]}")
        for repo in sorted(repos):
            print(f"  {repo}: {' '.join(repos[repo])}")
    if refusals:
        print(f"lint-action-pins: refused, {count(len(refusals), 'question')} could not be decided (exit 2, not a pass):", file=sys.stderr)
        for refusal in refusals:
            print(f"  {refusal}", file=sys.stderr)


def main(argv: list[str], *, root: Path = REPO_ROOT, oracle=None, allowlist: dict[str, str] | None = None) -> int:
    if yaml is None:
        print(
            "lint-action-pins: refused (exit 2, not a pass): PyYAML is not installed, and without a YAML "
            "parser the workflow files cannot be read. Install it: python3 -m pip install "
            "--require-hashes --only-binary :all: --no-deps -r scripts/requirements-ci-lint.txt",
            file=sys.stderr,
        )
        return 2
    allowlist = ALLOWLIST if allowlist is None else allowlist
    root = root.resolve()
    targets = workflow_files(argv, root)
    if not targets:
        print("lint-action-pins: refused (exit 2, not a pass): no workflow files found to check", file=sys.stderr)
        return 2

    reader = Reader(root)
    checked, files = reader.walk(targets)
    usage = checked + reader.walk(workflow_files([], root))[0]

    findings: list[Finding] = []
    pins: dict[tuple[str, str, int | None], list[Use]] = {}
    for use in checked:
        where, what = display(use.file, root), f"{use.key}: {use.value}"
        if use.value is None:
            findings.append(Finding(where, use.line, "unreadable", f"{use.key}:", "the value is not a string"))
            continue
        if use.value.startswith("./"):
            if local_file(root, use.value) is None:
                findings.append(Finding(
                    where, use.line, "local-unreadable", what,
                    "names no workflow file and no action.yml or action.yaml inside this repository, "
                    "so what it runs cannot be read",
                ))
            continue
        problem = static_problem(use.value)
        if problem:
            findings.append(Finding(where, use.line, problem[0], what, problem[1]))
        elif not use.value.startswith("docker://"):
            action, _, sha = use.value.rpartition("@")
            pins.setdefault((repo_of(action), sha, pinned_major(use.comment)), []).append(use)

    used = set()
    for use in usage:
        if use.value and not use.value.startswith(("./", "docker://")) and not static_problem(use.value):
            used.add((repo_of(use.value.rpartition("@")[0]), pinned_major(use.comment)))
    honoured = check_allowlist(allowlist, used, findings)

    refusals = list(reader.refusals)
    fetched: dict = {}
    if pins:
        scratch = None
        try:
            if oracle is None:
                scratch = tempfile.TemporaryDirectory(prefix="lint-action-pins-", dir=os.environ.get("RUNNER_TEMP") or None)
                oracle = GitOracle(Path(scratch.name))
            for (repo, sha, major), lines in pins.items():
                try:
                    why = why_unreachable(oracle, repo, sha, major, honoured)
                except Refusal as err:
                    refusals.append(str(err))
                    continue
                for use in lines if why else []:
                    shown = f"{use.key}: {use.value}" + (f" # {use.comment}" if use.comment else "")
                    findings.append(Finding(display(use.file, root), use.line, "unreachable", shown, why))
            fetched = dict(oracle.fetched)
        except Refusal as err:
            refusals.append(str(err))
        finally:
            if scratch is not None:
                scratch.cleanup()

    if not findings and not refusals:
        print(
            f"lint-action-pins: {count(len(checked), 'uses: reference')} across {count(files, 'file')}, all pinned to a commit sha"
            + (f"; all {count(len(pins), 'pinned commit')} reachable from a branch their repository permits" if pins else "")
        )
    refusals = list(dict.fromkeys(refusals))
    report(findings, refusals, fetched, len(checked), files)
    if refusals:
        return 2
    return 1 if findings else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
