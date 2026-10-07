#!/usr/bin/env python3
"""Every image a write-scoped job runs, and every Dockerfile base, is pinned by digest.

`lint-action-pins.py` pins the actions. It does not reach the images those jobs
pull at run time: a tag such as `latest` or `buildx-stable-1` is resolved by the
registry when the job runs, so whoever can push that tag chooses the bytes that
execute next to a token holding `packages: write` or `id-token: write`, and the
bytes that end up inside a published image. A `@sha256:` digest names exact
content; the tag stays in front of it so the version is still readable:

    FROM golang:1.25-alpine@sha256:<64 hex> AS builder
    image: docker.io/tonistiigi/binfmt:latest@sha256:<64 hex>

What it checks:

  Workflows (`.github/workflows/*.yml`, `*.yaml`). A job is write-scoped when its
  effective `permissions` grant any scope `write` (`write-all`, or a `write`
  value; the job's own block, else the workflow's). A job with no block at either
  level runs with the repository's default token, which may be write, and is
  treated as write-scoped. In those jobs:

    * `container:` and `services.<id>.image`   must carry `@sha256:<64 hex>`;
    * `uses: docker://...`                      same;
    * `docker/setup-qemu-action`                `with.image` must be set and pinned:
                                                 the default pulls
                                                 tonistiigi/binfmt:latest and runs it
                                                 with --privileged;
    * `docker/setup-buildx-action`              with the docker-container driver
                                                 (its default), `with.driver-opts`
                                                 must carry `image=<pinned>`: the
                                                 default pulls moby/buildkit by tag.

  Dockerfiles (`Dockerfile`, `Dockerfile.*`, `*.Dockerfile`; with no arguments,
  the files git tracks). Every `FROM`, `COPY --from=`, `RUN --mount=...,from=` and
  `# syntax=` image must name an earlier stage, `scratch`, or carry
  `@sha256:<64 hex>`. A `${NAME}` in the image is resolved from the default of a
  global `ARG`; with no default the image is chosen at build time and fails.

LOCAL_BASES lists bases that are built on the machine that runs the build and
are served by no registry, so there is no digest to pin. Each entry names one
file and one reference: a change to either brings the line back under the rule.

Not read: images selected inside a tool rather than named in these files, such
as the SBOM scanner BuildKit runs for `sbom: true`, or the image a container
action pulls for itself.

    python3 scripts/lint-runtime-image-pins.py [path ...]

With no arguments it checks `.github/workflows/` and every tracked Dockerfile. A
path may be a workflow file, a Dockerfile, or a directory: its workflow files,
and the Dockerfiles found under it.
Exit: 0 every image pinned, 1 an unpinned image, 2 nothing to check or a file
that cannot be read (including PyYAML missing: python3 -m pip install pyyaml).
"""
from __future__ import annotations

import os
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
WORKFLOW_DIR = REPO_ROOT / ".github" / "workflows"

DIGEST = re.compile(r"@sha256:[0-9a-f]{64}$")
DOCKERFILE_NAME = re.compile(r"^(Dockerfile(\..+)?|.+\.Dockerfile)$")
VARIABLE = re.compile(
    r"\$\{(?P<braced>[A-Za-z_][A-Za-z0-9_]*)(?P<op>:?[-+])?(?P<word>[^}]*)\}|\$(?P<bare>[A-Za-z_][A-Za-z0-9_]*)"
)
SKIP_DIRS = {".git", "node_modules"}

LOCAL_BASES = {
    # The terminal demo recorder roots on the VHS image built from a hackmyagent
    # checkout (see the file's header); it is never pulled and no workflow builds it.
    "docs/demo/terminal/Dockerfile": "hma-vhs:0.33.0",
}


class Finding:
    def __init__(self, where: str, what: str, why: str) -> None:
        self.where, self.what, self.why = where, what, why

    def __str__(self) -> str:
        return f"{self.where}: {self.what}\n    {self.why}"


def display(path: Path) -> str:
    try:
        return path.resolve().relative_to(REPO_ROOT).as_posix()
    except ValueError:
        return str(path)


def image_problem(ref: str) -> str | None:
    """Why `ref` is not pinned, or None when it is."""
    if not ref:
        return "no image reference could be read"
    if "${{" in ref:
        return "an expression: the image is chosen at run time and cannot be checked here"
    if DIGEST.search(ref):
        return None
    return "resolved by tag when the job runs; pin it with @sha256:<digest>"


# ---------------------------------------------------------------------------
# Workflows
# ---------------------------------------------------------------------------


def child(node, key: str):
    if not isinstance(node, yaml.MappingNode):
        return None
    for k, v in node.value:
        if isinstance(k, yaml.ScalarNode) and k.value == key:
            return v
    return None


def pairs(node):
    if not isinstance(node, yaml.MappingNode):
        return []
    return [(k.value, v) for k, v in node.value if isinstance(k, yaml.ScalarNode)]


def scalar(node) -> str | None:
    return node.value.strip() if isinstance(node, yaml.ScalarNode) else None


def line_of(node) -> int:
    return node.start_mark.line + 1


def grants_write(permissions) -> bool:
    if isinstance(permissions, yaml.ScalarNode):
        return permissions.value.strip() == "write-all"
    return any(scalar(v) == "write" for _, v in pairs(permissions))


def write_scope(job, workflow) -> str | None:
    """Where the job's write permission comes from, or None when it has none."""
    own = child(job, "permissions")
    if own is not None:
        return "job permissions grant write" if grants_write(own) else None
    inherited = child(workflow, "permissions")
    if inherited is not None:
        return "workflow permissions grant write" if grants_write(inherited) else None
    return "no permissions block, so the repository's default token applies"


def action_name(uses: str) -> str:
    return uses.split("@", 1)[0].strip().lower()


def check_workflow(path: Path, findings: list[Finding]) -> tuple[int, int]:
    """Returns (write-scoped jobs, image references checked)."""
    with path.open(encoding="utf-8") as fh:
        root = yaml.compose(fh, Loader=yaml.SafeLoader)
    jobs_scoped = 0
    checked = 0
    name = display(path)

    def check(node, what: str, ref: str | None, why_missing: str | None = None) -> None:
        nonlocal checked
        checked += 1
        if ref is None and why_missing:
            findings.append(Finding(f"{name}:{line_of(node)}", what, why_missing))
            return
        why = image_problem(ref or "")
        if why:
            findings.append(Finding(f"{name}:{line_of(node)}", f"{what}: {ref}", why))

    for job_id, job in pairs(child(root, "jobs")):
        scope = write_scope(job, root)
        if scope is None:
            continue
        jobs_scoped += 1
        label = f"job {job_id} ({scope})"

        container = child(job, "container")
        if container is not None:
            image = scalar(container) if isinstance(container, yaml.ScalarNode) else scalar(child(container, "image"))
            check(container, f"{label} container", image)

        for service_id, service in pairs(child(job, "services")):
            image_node = child(service, "image")
            if image_node is not None:
                check(image_node, f"{label} service {service_id}", scalar(image_node))

        steps = child(job, "steps")
        for step in steps.value if isinstance(steps, yaml.SequenceNode) else []:
            uses_node = child(step, "uses")
            uses = scalar(uses_node) or ""
            if not uses:
                continue
            with_node = child(step, "with")
            action = action_name(uses)
            if uses.startswith("docker://"):
                check(uses_node, f"{label} uses", uses)
            elif action == "docker/setup-qemu-action":
                image_node = child(with_node, "image")
                check(
                    image_node if image_node is not None else uses_node,
                    f"{label} setup-qemu-action with.image",
                    scalar(image_node),
                    "not set, so the action pulls docker.io/tonistiigi/binfmt:latest and runs it with --privileged",
                )
            elif action == "docker/setup-buildx-action":
                driver = scalar(child(with_node, "driver")) or "docker-container"
                if driver in ("docker", "remote"):
                    continue
                opts_node = child(with_node, "driver-opts")
                images = [
                    entry.strip()[len("image="):].strip()
                    for entry in (scalar(opts_node) or "").splitlines()
                    if entry.strip().startswith("image=")
                ]
                check(
                    opts_node if opts_node is not None else uses_node,
                    f"{label} setup-buildx-action driver-opts image=",
                    images[-1] if images else None,
                    f"no image= in driver-opts, so the {driver} driver pulls a BuildKit image by tag",
                )
    return jobs_scoped, checked


# ---------------------------------------------------------------------------
# Dockerfiles
# ---------------------------------------------------------------------------


def instructions(text: str) -> list[tuple[int, str, str]]:
    """(line, KEYWORD, arguments) with continuation lines joined."""
    out: list[tuple[int, str, str]] = []
    start, buf = None, ""
    for number, raw in enumerate(text.splitlines(), start=1):
        stripped = raw.strip()
        if stripped.startswith("#") or (start is None and not stripped):
            continue
        if start is None:
            start, buf = number, ""
        if stripped.endswith("\\"):
            buf += stripped[:-1] + " "
            continue
        buf += stripped
        keyword, _, args = buf.strip().partition(" ")
        out.append((start, keyword.upper(), args.strip()))
        start = None
    if start is not None:
        keyword, _, args = buf.strip().partition(" ")
        out.append((start, keyword.upper(), args.strip()))
    return out


def substitute(value: str, args: dict[str, str]) -> tuple[str, list[str]]:
    """`value` with ARG defaults expanded, and the names that had no value."""
    missing: list[str] = []

    def expand(match: re.Match) -> str:
        name = match.group("braced") or match.group("bare")
        op, word = match.group("op"), match.group("word") or ""
        current = args.get(name, "")
        if op and op.endswith("-"):
            return current or word
        if op and op.endswith("+"):
            return word if current else ""
        if not current:
            missing.append(name)
        return current

    return VARIABLE.sub(expand, value), missing


def parse_args(args: str) -> dict[str, str]:
    out: dict[str, str] = {}
    try:
        tokens = shlex.split(args)
    except ValueError:
        tokens = args.split()
    for token in tokens:
        key, sep, default = token.partition("=")
        out[key] = default if sep else ""
    return out


def check_dockerfile(path: Path, findings: list[Finding], allowed: list[str]) -> int:
    """Returns the number of image references checked."""
    text = path.read_text(encoding="utf-8")
    name = display(path)
    stages: set[str] = set()
    global_args: dict[str, str] = {}
    seen_from = False
    checked = 0

    def check(number: int, what: str, raw: str) -> None:
        nonlocal checked
        checked += 1
        ref, missing = substitute(raw, global_args)
        if missing:
            findings.append(Finding(
                f"{name}:{number}", f"{what} {raw}",
                f"build arg {', '.join(missing)} has no default here, so the image is chosen at build time",
            ))
            return
        if ref.lower() in stages or ref.lower() == "scratch" or ref.isdigit():
            return
        if LOCAL_BASES.get(name) == ref:
            allowed.append(f"{name}:{number}: {ref} (local build, listed in LOCAL_BASES)")
            return
        why = image_problem(ref)
        if why:
            findings.append(Finding(f"{name}:{number}", f"{what} {ref}", why.replace("the job runs", "the image is built")))

    for number, raw in enumerate(text.splitlines(), start=1):
        directive = re.match(r"^#\s*syntax\s*=\s*(\S+)", raw.strip(), re.IGNORECASE)
        if directive:
            check(number, "# syntax=", directive.group(1))
        if raw.strip() and not raw.strip().startswith("#"):
            break

    for number, keyword, args in instructions(text):
        if keyword == "ARG" and not seen_from:
            global_args.update(parse_args(args))
        elif keyword == "FROM":
            seen_from = True
            tokens = [t for t in args.split() if not t.startswith("--")]
            if not tokens:
                findings.append(Finding(f"{name}:{number}", "FROM", "no image could be read"))
                continue
            check(number, "FROM", tokens[0])
            if len(tokens) >= 3 and tokens[1].lower() == "as":
                stages.add(tokens[2].lower())
        elif keyword in ("COPY", "RUN"):
            for flag in re.findall(r"--from=(\S+)", args) if keyword == "COPY" else []:
                check(number, "COPY --from=", flag.strip("\"'"))
            for mount in re.findall(r"--mount=(\S+)", args) if keyword == "RUN" else []:
                for part in mount.split(","):
                    key, _, value = part.partition("=")
                    if key == "from":
                        check(number, "RUN --mount from=", value.strip("\"'"))
    return checked


# ---------------------------------------------------------------------------
# Targets
# ---------------------------------------------------------------------------


def is_workflow(path: Path) -> bool:
    return path.suffix in (".yml", ".yaml")


def is_dockerfile(path: Path) -> bool:
    return bool(DOCKERFILE_NAME.match(path.name))


def tracked_dockerfiles() -> list[Path]:
    try:
        out = subprocess.run(
            ["git", "-C", str(REPO_ROOT), "ls-files", "-z"],
            check=True, capture_output=True, text=True,
        ).stdout
        return sorted(REPO_ROOT / p for p in out.split("\0") if p and is_dockerfile(Path(p)))
    except (OSError, subprocess.CalledProcessError):
        return walk_dockerfiles(REPO_ROOT)


def walk_dockerfiles(top: Path) -> list[Path]:
    found: list[Path] = []
    for dirpath, dirnames, filenames in os.walk(top):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        found.extend(Path(dirpath) / f for f in filenames if DOCKERFILE_NAME.match(f))
    return sorted(found)


def targets(argv: list[str]) -> tuple[list[Path], list[Path]]:
    if not argv:
        workflows = sorted(p for p in WORKFLOW_DIR.iterdir() if is_workflow(p)) if WORKFLOW_DIR.is_dir() else []
        return workflows, tracked_dockerfiles()
    workflows: list[Path] = []
    dockerfiles: list[Path] = []
    for arg in argv:
        target = Path(arg)
        if target.is_dir():
            workflows.extend(sorted(p for p in target.iterdir() if p.is_file() and is_workflow(p)))
            dockerfiles.extend(walk_dockerfiles(target))
        elif target.is_file() and is_dockerfile(target):
            dockerfiles.append(target)
        elif target.is_file() and is_workflow(target):
            workflows.append(target)
    return workflows, dockerfiles


def main(argv: list[str]) -> int:
    if yaml is None:
        print("lint-runtime-image-pins: PyYAML is required (python3 -m pip install pyyaml)", file=sys.stderr)
        return 2
    workflows, dockerfiles = targets(argv)
    if not workflows and not dockerfiles:
        print("lint-runtime-image-pins: no workflow files or Dockerfiles found to check", file=sys.stderr)
        return 2

    findings: list[Finding] = []
    allowed: list[str] = []
    jobs = images = 0
    for path in workflows:
        try:
            scoped, checked = check_workflow(path, findings)
        except (OSError, yaml.YAMLError) as err:
            print(f"lint-runtime-image-pins: cannot read {display(path)}: {err}", file=sys.stderr)
            return 2
        jobs += scoped
        images += checked
    for path in dockerfiles:
        try:
            images += check_dockerfile(path, findings, allowed)
        except (OSError, UnicodeDecodeError) as err:
            print(f"lint-runtime-image-pins: cannot read {display(path)}: {err}", file=sys.stderr)
            return 2

    scope = (
        f"{jobs} write-scoped jobs in {len(workflows)} workflow files, "
        f"{len(dockerfiles)} Dockerfiles"
    )
    if findings:
        print(f"lint-runtime-image-pins: {len(findings)} of {images} image references are not pinned by digest ({scope})\n")
        for finding in findings:
            print(finding)
        print(
            "\nFix: resolve the digest the tag names today from the registry (the top-level Digest:,\n"
            "which is the multi-platform index),\n"
            "  docker buildx imagetools inspect <image>:<tag>\n"
            "then write the tag and the digest together:\n"
            "  FROM <image>:<tag>@sha256:<digest>\n"
            "  setup-qemu-action   with: image: docker.io/tonistiigi/binfmt:<tag>@sha256:<digest>\n"
            "  setup-buildx-action with: driver-opts: image=moby/buildkit:<tag>@sha256:<digest>\n"
            "Verify: python3 scripts/lint-runtime-image-pins.py"
        )
        return 1

    print(f"lint-runtime-image-pins: {images} image references ({scope}), all pinned by digest")
    for entry in allowed:
        print(f"  allowed {entry}")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
