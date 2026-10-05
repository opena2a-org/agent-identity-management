#!/usr/bin/env python3
"""A job that can run for a release tag or a publish must not use a run-written cache.

A cache entry is written by whichever run got there first: a pull request build, a
merge to main, a manual dispatch. A later job that restores it builds from bytes no
reviewed commit names, and the release it produces carries them. So a job on the
release path starts from the checkout and the registries only.

Which jobs are on the release path is computed, never listed by hand:

  * the workflow's `on:` gives the runs that carry a tag or publish something:
      - `push` with `tags` (one run per pattern), with `tags-ignore`, or with no
        branch or tag filter at all (any tag);
      - `release` (the ref is the release's tag);
      - `workflow_dispatch` (the ref may be a tag).
  * a job is on the path when, for one of those runs, its `if:` is not provably false
    and every job it `needs` is on the path too (a job whose `if:` calls `always()`,
    `failure()` or `cancelled()` does not depend on its needs). `if:` is evaluated
    against `github.event_name`, `github.ref`, `github.ref_type` and `github.ref_name`
    with `==`, `!=`, `!`, `&&`, `||`, parentheses, `startsWith` and `contains`.
    Anything else is unknown, and unknown counts as "can run".
  * inside such a job a step is read unless its own `if:` is provably false for every
    run that reaches the job.

What is refused in a step of such a job:

  * `cache-from` or `cache-to` in `with:` (docker/build-push-action, bake), or a
    `--cache-from` / `--cache-to` argument in a `run:` script;
  * `uses: actions/cache` or `uses: actions/cache/restore`;
  * an `actions/setup-*` step with a `cache:` input that is not false or empty;
  * `actions/setup-go` without `cache: false` (it caches by default);
  * `docker/setup-qemu-action` without `cache-image: false` (by default it restores
    the binfmt image from the Actions cache, loads it, and falls back to it when the
    pull fails).

    python3 scripts/lint-release-path-cache.py [path ...]
    python3 scripts/lint-release-path-cache.py --self-test

With no path it checks `.github/workflows/release.yml` and
`.github/workflows/docker-publish.yml`. A path may be a workflow file or a directory
of them. `--self-test` runs every fixture under `scripts/testdata/release-path-cache/`:
each `red-*.yml` must fail and each `green-*.yml` must pass.

Not covered: a cache an action restores without declaring it in the workflow (for
example a newer setup-node's automatic package-manager cache, or setup-buildx-action's
`cache-binary`, which applies only when the action downloads or builds buildx), and
reusable workflows called with `uses:` at job level, which are reported as unread.
Exit: 0 clean, 1 a refused cache (or a failed self-test), 2 nothing to check or
PyYAML missing.
"""
from __future__ import annotations

import io
import re
import sys
from pathlib import Path

try:
    import yaml
except ImportError:  # pragma: no cover - reported as exit 2
    yaml = None

REPO_ROOT = Path(__file__).resolve().parent.parent
WORKFLOW_DIR = REPO_ROOT / ".github" / "workflows"
DEFAULT_FILES = [WORKFLOW_DIR / "release.yml", WORKFLOW_DIR / "docker-publish.yml"]
FIXTURE_DIR = REPO_ROOT / "scripts" / "testdata" / "release-path-cache"

UNKNOWN = object()  # a value the lint cannot decide; it never proves a job cannot run
STATUS_FUNCTIONS = {"always", "failure", "cancelled"}
GLOB = re.compile(r"[*?+\[\]!]")


class Run:
    """One kind of run on the release path: an event and what is known of its tag."""

    def __init__(self, event: str, pattern: str):
        self.event = event
        self.pattern = pattern
        found = GLOB.search(pattern)
        self.exact = found is None
        self.name_prefix = pattern if found is None else pattern[: found.start()]

    def label(self) -> str:
        return f"{self.event} of tag {self.pattern}"

    def context(self, name: str):
        """A `github.*` value: ("exact", text), ("prefix", text) or UNKNOWN."""
        kind = "exact" if self.exact else "prefix"
        if name == "github.event_name":
            return ("exact", self.event)
        if name == "github.ref_type":
            return ("exact", "tag")
        if name == "github.ref":
            return (kind, "refs/tags/" + self.name_prefix)
        if name == "github.ref_name":
            return (kind, self.name_prefix)
        return UNKNOWN


def release_runs(workflow: dict) -> list[Run]:
    """The runs of this workflow that carry a tag or publish, from its `on:`."""
    # PyYAML reads the bare key `on` as the boolean True.
    on = workflow.get("on", workflow.get(True))
    if isinstance(on, str):
        on = {on: None}
    elif isinstance(on, list):
        on = {event: None for event in on}
    elif not isinstance(on, dict):
        return []
    runs: list[Run] = []
    if "push" in on:
        push = on["push"] or {}
        if "tags" in push:
            patterns = push["tags"] or []
            if isinstance(patterns, str):
                patterns = [patterns]
            runs.extend(Run("push", str(p)) for p in patterns if not str(p).startswith("!"))
        elif "tags-ignore" in push or not ("branches" in push or "branches-ignore" in push):
            runs.append(Run("push", "*"))
    for event in ("release", "workflow_dispatch"):
        if event in on:
            runs.append(Run(event, "*"))
    return runs


TOKEN = re.compile(
    r"\s*(?:(?P<str>'(?:[^']|'')*')|(?P<op>==|!=|&&|\|\||[!(),])|(?P<word>[A-Za-z_][\w.\-]*(?:\[[^\]]*\])?[\w.\-]*)"
    r"|(?P<num>-?\d+(?:\.\d+)?))"
)


def tokenize(text: str) -> list[tuple[str, str]] | None:
    tokens, pos = [], 0
    text = text.strip()
    while pos < len(text):
        found = TOKEN.match(text, pos)
        if not found or found.end() == pos:
            return None
        pos = found.end()
        for kind in ("str", "op", "word", "num"):
            if found.group(kind) is not None:
                value = found.group(kind)
                if kind == "str":
                    value = value[1:-1].replace("''", "'")
                tokens.append((kind, value))
                break
    return tokens


class Expression:
    """Three-valued evaluation of a job or step `if:` for one run.

    Values are True, False, ("exact", text), ("prefix", text) or UNKNOWN. Comparisons
    of text ignore case, as the workflow expression language does.
    """

    def __init__(self, tokens: list[tuple[str, str]], run: Run):
        self.tokens = tokens
        self.pos = 0
        self.run = run
        self.status_call = False

    def peek(self):
        return self.tokens[self.pos] if self.pos < len(self.tokens) else (None, None)

    def take(self):
        token = self.peek()
        self.pos += 1
        return token

    def parse(self):
        value = self.parse_or()
        if self.pos != len(self.tokens):
            raise ValueError("trailing tokens")
        return value

    def parse_or(self):
        value = truth(self.parse_and())
        while self.peek() == ("op", "||"):
            self.take()
            right = truth(self.parse_and())
            if value is True or right is True:
                value = True
            elif value is False and right is False:
                value = False
            else:
                value = UNKNOWN
        return value

    def parse_and(self):
        value = self.parse_equality()
        while self.peek() == ("op", "&&"):
            self.take()
            left, right = truth(value), truth(self.parse_equality())
            if left is False or right is False:
                value = False
            elif left is True and right is True:
                value = True
            else:
                value = UNKNOWN
        return value

    def parse_equality(self):
        value = self.parse_unary()
        while self.peek() in (("op", "=="), ("op", "!=")):
            operator = self.take()[1]
            same = equal(value, self.parse_unary())
            if same is UNKNOWN:
                value = UNKNOWN
            else:
                value = same if operator == "==" else not same
        return value

    def parse_unary(self):
        if self.peek() == ("op", "!"):
            self.take()
            value = truth(self.parse_unary())
            return UNKNOWN if value is UNKNOWN else not value
        return self.parse_primary()

    def parse_primary(self):
        kind, value = self.take()
        if kind == "op" and value == "(":
            inner = self.parse_or_raw()
            if self.take() != ("op", ")"):
                raise ValueError("unbalanced parenthesis")
            return inner
        if kind == "str":
            return ("exact", value)
        if kind == "num":
            return UNKNOWN
        if kind == "word":
            if self.peek() == ("op", "("):
                return self.call(value)
            if value in ("true", "false"):
                return value == "true"
            return self.run.context(value)
        raise ValueError("unexpected token")

    def parse_or_raw(self):
        """`(a)` keeps a text value so `(github.ref) == 'x'` still compares."""
        start = self.pos
        value = self.parse_and()
        if self.peek() == ("op", "||"):
            self.pos = start
            return self.parse_or()
        return value

    def call(self, name: str):
        self.take()  # (
        arguments = []
        if self.peek() != ("op", ")"):
            arguments.append(self.parse_or_raw())
            while self.peek() == ("op", ","):
                self.take()
                arguments.append(self.parse_or_raw())
        if self.take() != ("op", ")"):
            raise ValueError("unbalanced call")
        lowered = name.lower()
        if lowered in STATUS_FUNCTIONS:
            self.status_call = True
            return UNKNOWN
        if lowered == "startswith" and len(arguments) == 2:
            return starts_with(arguments[0], arguments[1])
        if lowered == "contains" and len(arguments) == 2:
            return contains(arguments[0], arguments[1])
        return UNKNOWN


def truth(value):
    if value is True or value is False or value is UNKNOWN:
        return value
    kind, text = value
    if kind == "exact":
        return text != ""
    return True if text else UNKNOWN


def is_text(value) -> bool:
    return isinstance(value, tuple)


def equal(left, right):
    if isinstance(left, bool) and isinstance(right, bool):
        return left == right
    if not (is_text(left) and is_text(right)):
        return UNKNOWN
    (lkind, ltext), (rkind, rtext) = left, right
    ltext, rtext = ltext.lower(), rtext.lower()
    if lkind == "exact" and rkind == "exact":
        return ltext == rtext
    if lkind == "prefix" and rkind == "exact":
        return UNKNOWN if rtext.startswith(ltext) else False
    if lkind == "exact" and rkind == "prefix":
        return UNKNOWN if ltext.startswith(rtext) else False
    return UNKNOWN if ltext.startswith(rtext) or rtext.startswith(ltext) else False


def starts_with(value, prefix):
    if not (is_text(value) and is_text(prefix)) or prefix[0] != "exact":
        return UNKNOWN
    kind, text = value[0], value[1].lower()
    wanted = prefix[1].lower()
    if text.startswith(wanted):
        return True
    if kind == "prefix" and wanted.startswith(text):
        return UNKNOWN
    return False


def contains(value, needle):
    if not (is_text(value) and is_text(needle)) or needle[0] != "exact":
        return UNKNOWN
    kind, text = value[0], value[1].lower()
    wanted = needle[1].lower()
    if wanted in text:
        return True
    return False if kind == "exact" else UNKNOWN


def evaluate(condition, run: Run) -> tuple[object, bool]:
    """(True | False | UNKNOWN, whether the condition calls a status function)."""
    if condition is None:
        return True, False
    if isinstance(condition, bool):
        return condition, False
    text = str(condition).strip()
    if text.startswith("${{") and text.endswith("}}") and text.count("${{") == 1:
        text = text[3:-2]
    elif "${{" in text:
        return UNKNOWN, True
    tokens = tokenize(text)
    if not tokens:
        return UNKNOWN, True
    expression = Expression(tokens, run)
    try:
        return truth(expression.parse()), expression.status_call
    except (ValueError, IndexError, TypeError):
        return UNKNOWN, True


def needs_of(job: dict) -> list[str]:
    needs = job.get("needs") or []
    return [needs] if isinstance(needs, str) else [str(n) for n in needs]


def jobs_on_path(workflow: dict) -> dict[str, list[Run]]:
    """job id -> the release-path runs it can run for (absent when there is none)."""
    jobs = workflow.get("jobs") or {}
    on_path: dict[str, list[Run]] = {}
    for run in release_runs(workflow):
        decided: dict[str, bool] = {}

        def can_run(job_id: str, seen: tuple[str, ...] = ()) -> bool:
            if job_id in decided:
                return decided[job_id]
            job = jobs.get(job_id)
            if not isinstance(job, dict) or job_id in seen:
                return True
            verdict, status_call = evaluate(job.get("if"), run)
            result = verdict is not False
            if result and not status_call:
                result = all(can_run(need, seen + (job_id,)) for need in needs_of(job))
            decided[job_id] = result
            return result

        for job_id in jobs:
            if can_run(job_id):
                on_path.setdefault(job_id, []).append(run)
    return on_path


def enabled(value) -> bool:
    """Whether a `with:` value switches something on. An expression counts as on."""
    if value is None or value is False:
        return False
    return str(value).strip().lower() not in ("", "false")


RUN_CACHE_FLAG = re.compile(r"(?<![\w-])--(cache-from|cache-to)\b")


def step_problems(step: dict) -> list[str]:
    problems = []
    uses = str(step.get("uses") or "")
    action = uses.split("@", 1)[0].strip().lower().rstrip("/")
    inputs = step.get("with") if isinstance(step.get("with"), dict) else {}
    if enabled(inputs.get("cache-from")):
        problems.append(f"`cache-from: {inputs['cache-from']}` imports a cache another run wrote")
    if enabled(inputs.get("cache-to")):
        problems.append(f"`cache-to: {inputs['cache-to']}` exports a cache other runs import")
    if action in ("actions/cache", "actions/cache/restore"):
        problems.append(f"`uses: {uses}` restores a cache another run wrote")
    if action.startswith("actions/setup-"):
        if "cache" in inputs:
            if enabled(inputs["cache"]):
                problems.append(f"`{action}` with `cache: {inputs['cache']}` restores a cache another run wrote")
        elif action == "actions/setup-go":
            problems.append("`actions/setup-go` caches by default; set `cache: false`")
    if action == "docker/setup-qemu-action":
        if "cache-image" not in inputs:
            problems.append(
                "`docker/setup-qemu-action` restores the binfmt image from the Actions cache by default; "
                "set `cache-image: false`"
            )
        elif str(inputs["cache-image"]).strip().lower() != "false":
            value = inputs["cache-image"]
            shown = str(value).lower() if isinstance(value, bool) else value
            problems.append(
                f"`docker/setup-qemu-action` with `cache-image: {shown}` restores a binfmt image "
                "another run cached; set `cache-image: false`"
            )
    script = step.get("run")
    if isinstance(script, str):
        for flag in sorted({found.group(1) for found in RUN_CACHE_FLAG.finditer(script)}):
            problems.append(f"`--{flag}` in a run script uses a cache shared between runs")
    return problems


def check_file(path: Path) -> tuple[dict[str, list[Run]], list[tuple[str, str, str]], list[str]]:
    """(jobs on the release path, [(job, step, problem)], notes)."""
    workflow = yaml.safe_load(path.read_text(encoding="utf-8")) or {}
    if not isinstance(workflow, dict):
        return {}, [], []
    on_path = jobs_on_path(workflow)
    findings: list[tuple[str, str, str]] = []
    notes: list[str] = []
    for job_id, runs in on_path.items():
        job = (workflow.get("jobs") or {}).get(job_id)
        if not isinstance(job, dict):
            continue
        if "uses" in job:
            notes.append(f"job {job_id}: calls {job['uses']}, which is not read")
        for index, step in enumerate(job.get("steps") or [], start=1):
            if not isinstance(step, dict):
                continue
            if all(evaluate(step.get("if"), run)[0] is False for run in runs):
                continue
            name = str(step.get("name") or step.get("id") or step.get("uses") or "run")
            for problem in step_problems(step):
                findings.append((job_id, f'step {index} "{name}"', problem))
    return on_path, findings, notes


def workflow_files(args: list[str]) -> list[Path]:
    if not args:
        return [p for p in DEFAULT_FILES if p.is_file()]
    files: list[Path] = []
    for arg in args:
        target = Path(arg)
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


def lint(files: list[Path], out=sys.stdout) -> int:
    total = 0
    for path in files:
        on_path, findings, notes = check_file(path)
        shown = display(path)
        if not on_path:
            print(f"{shown}: no job can run for a tag or a publish", file=out)
            continue
        for job_id, runs in on_path.items():
            print(f"{shown}: job {job_id} is on the release path ({'; '.join(r.label() for r in runs)})", file=out)
        for note in notes:
            print(f"{shown}: note: {note}", file=out)
        for job_id, step, problem in findings:
            print(f"{shown}: REFUSED job {job_id}, {step}: {problem}", file=out)
        total += len(findings)
    if total:
        print(
            f"lint-release-path-cache: {total} run-written cache use(s) in a job that can run for a tag "
            "or a publish. Remove the cache input (for setup-go set `cache: false`, for setup-qemu-action "
            "`cache-image: false`), or move the step to "
            "a job whose `if:` keeps it off tag and publish runs.",
            file=out,
        )
        return 1
    print(f"lint-release-path-cache: ok, {len(files)} workflow(s), no run-written cache on the release path", file=out)
    return 0


def self_test() -> int:
    fixtures = sorted(FIXTURE_DIR.glob("*.yml"))
    if not fixtures:
        print(f"lint-release-path-cache: no fixture under {display(FIXTURE_DIR)}", file=sys.stderr)
        return 2
    failed = 0
    for fixture in fixtures:
        want_red = fixture.name.startswith("red-")
        buffer = io.StringIO()
        got_red = lint([fixture], out=buffer) != 0
        verdict = "ok" if got_red == want_red else "WRONG"
        if got_red != want_red:
            failed += 1
            sys.stdout.write(buffer.getvalue())
        print(f"self-test: {verdict}: {fixture.name} is {'red' if got_red else 'green'}")
    if failed:
        print(f"lint-release-path-cache: self-test failed on {failed} fixture(s)")
        return 1
    print(f"lint-release-path-cache: self-test ok, {len(fixtures)} fixture(s)")
    return 0


def main(argv: list[str]) -> int:
    if yaml is None:
        print("lint-release-path-cache: PyYAML is required (pip install pyyaml)", file=sys.stderr)
        return 2
    if argv and argv[0] == "--self-test":
        return self_test()
    files = workflow_files(argv)
    if not files:
        print("lint-release-path-cache: no workflow file to check", file=sys.stderr)
        return 2
    return lint(files)


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
