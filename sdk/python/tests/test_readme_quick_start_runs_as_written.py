"""
The Quick start in the root README and in this SDK's README is one Python
session a reader types top to bottom: sign in, register, verify, one allowed
call, the strict-mode step, one refused call.

These tests run that example as written, fence by fence in one namespace, with
`aim_sdk` replaced by a stand-in that enforces exactly the capabilities the
example declares. A name the example uses without defining it raises
NameError; a call the example never makes is counted as missing. They also
hold the minimal dev stack command to the services docker-compose.yml defines,
and the recorded walkthrough's typed lines to the root README's Quick start.

Everything here runs offline and reads only files in this repository.
"""

import re
import sys
import types
from pathlib import Path

import pytest

SDK_DIR = Path(__file__).resolve().parent.parent          # .../sdk/python
REPO_ROOT = SDK_DIR.parent.parent                         # repo root
ROOT_README = REPO_ROOT / "README.md"
SDK_README = SDK_DIR / "README.md"
COMPOSE = REPO_ROOT / "docker-compose.yml"
TAPE = REPO_ROOT / "docs" / "demo" / "quickstart-selfhosted" / "tape.tape"

READMES = [
    pytest.param(ROOT_README, id="root-README"),
    pytest.param(SDK_README, id="sdk-python-README"),
]


def quick_start(path):
    """The text under `## Quick start` up to the next `## ` heading."""
    lines = path.read_text(encoding="utf-8").split("\n")
    start = next(i for i, l in enumerate(lines) if re.match(r"^## quick start\s*$", l, re.I))
    end = next((i for i, l in enumerate(lines) if i > start and l.startswith("## ")), len(lines))
    return "\n".join(lines[start:end])


def fences(section):
    """(language, body) for every fenced block in the section, in order."""
    return [
        (m.group(1), m.group(2))
        for m in re.finditer(r"^```(\w*)\n(.*?)^```\s*$", section, re.M | re.S)
    ]


class StubDenied(PermissionError):
    """Stands in for ActionDeniedError, which is a PermissionError."""


class StubAgent:
    def __init__(self, name, capabilities):
        self.name = name
        self.capabilities = list(capabilities or [])
        self.calls = []

    def perform_action(self, capability, **_):
        def decorate(fn):
            def wrapper(*args, **kwargs):
                if capability not in self.capabilities:
                    self.calls.append(("refused", capability, None))
                    raise StubDenied(f"AIM denied {capability!r}")
                result = fn(*args, **kwargs)
                self.calls.append(("allowed", capability, result))
                return result

            return wrapper

        return decorate


def run_example(path, monkeypatch):
    """Execute the Quick start's Python fences in order in one namespace."""
    agents = []

    def secure(name, capabilities=None, **_):
        agent = StubAgent(name, capabilities)
        agents.append(agent)
        return agent

    stub = types.ModuleType("aim_sdk")
    stub.secure = secure
    monkeypatch.setitem(sys.modules, "aim_sdk", stub)

    namespace = {"__name__": "__quickstart__"}
    blocks = [body for lang, body in fences(quick_start(path)) if lang == "python"]
    assert blocks, f"{path.name}: the Quick start has no Python fence"
    for index, body in enumerate(blocks):
        code = compile(body, f"{path.name} Quick start fence {index + 1}", "exec")
        try:
            exec(code, namespace)
        except StubDenied:
            pass
    return agents, namespace


@pytest.mark.parametrize("path", READMES)
def test_example_runs_as_written_with_one_allowed_and_one_refused_call(path, monkeypatch):
    agents, namespace = run_example(path, monkeypatch)

    assert len(agents) == 1, f"{path.name}: the example must register exactly one agent"
    agent = agents[0]
    assert agent.capabilities == ["db:read"], (
        f"{path.name}: the code must show that the agent holds db:read, "
        f"got capabilities={agent.capabilities!r}"
    )
    allowed = [c for c in agent.calls if c[0] == "allowed"]
    refused = [c for c in agent.calls if c[0] == "refused"]
    assert [c[1] for c in allowed] == ["db:read"], (
        f"{path.name}: the example must make exactly one allowed db:read call, got {agent.calls!r}"
    )
    assert [c[1] for c in refused] == ["db:write"], (
        f"{path.name}: the example must make exactly one refused db:write call, got {agent.calls!r}"
    )
    assert allowed[0][2] == namespace["customers"][42], (
        f"{path.name}: the allowed call must return the record it reads"
    )
    assert 42 in namespace["customers"], (
        f"{path.name}: the refused call's body must not have run"
    )


@pytest.mark.parametrize("path", READMES)
def test_example_uses_no_undefined_database_object(path):
    section = quick_start(path)
    assert not re.search(r"\bdb\.(query|execute)\b", section), (
        f"{path.name}: the Quick start calls a `db` object it never defines"
    )


@pytest.mark.parametrize("path", READMES)
def test_steps_come_in_the_order_a_reader_can_follow(path):
    section = quick_start(path)
    register = section.index("secure(")
    verify = section.index("until an administrator verifies it under Agents")
    allowed = section.index("get_customer(42)")
    strict = section.index('{"enforcementMode": "strict"}')
    refused = section.index("delete_customer(42)")
    assert register < verify < allowed < strict < refused, (
        f"{path.name}: the Quick start must read register, verify, allowed call, "
        f"strict mode, refused call"
    )


def test_both_readmes_sign_in_with_the_same_lines_including_self_hosted():
    root_bash = [b for lang, b in fences(quick_start(ROOT_README)) if lang == "bash"]
    sdk_bash = [b for lang, b in fences(quick_start(SDK_README)) if lang == "bash"]
    assert root_bash and root_bash[0] == sdk_bash[0], (
        "the root README's sign-in fence must match the SDK README's, byte for byte"
    )
    assert "aim-sdk login --url http://localhost:8080" in root_bash[0]
    assert "`/device`" in quick_start(ROOT_README)


def test_root_readme_drops_the_stale_self_hosted_sentence():
    assert "do not yet complete" not in ROOT_README.read_text(encoding="utf-8")


def compose_services():
    lines = COMPOSE.read_text(encoding="utf-8").split("\n")
    start = lines.index("services:")
    services = set()
    for line in lines[start + 1:]:
        if re.match(r"^\S", line):
            break
        m = re.match(r"^  ([A-Za-z0-9_.-]+):\s*$", line)
        if m:
            services.add(m.group(1))
    return services


def test_minimal_dev_stack_command_names_services_the_compose_file_defines():
    commands = re.findall(
        r"^docker compose up -d (\S.*)$", ROOT_README.read_text(encoding="utf-8"), re.M
    )
    named = [s for c in commands for s in c.split()]
    assert named, "the README must keep its minimal dev stack command"
    services = compose_services()
    missing = [s for s in named if s not in services]
    assert not missing, (
        f"docker compose would fail with 'no such service' for {missing}; "
        f"docker-compose.yml defines {sorted(services)}"
    )


def readme_code_lines():
    """The lines the walkthrough may type, extracted as docs/demo/lib/extract-readme.mjs does."""
    out = set()
    for _, body in fences(quick_start(ROOT_README)):
        for raw in body.split("\n"):
            line = re.sub(r"\s+#.*$", "", raw).rstrip()
            if line.strip():
                out.add(line)
    return out


def test_walkthrough_types_only_root_readme_quick_start_lines():
    allowed = readme_code_lines()
    hidden = False
    stray = []
    for number, line in enumerate(TAPE.read_text(encoding="utf-8").split("\n"), 1):
        t = line.strip()
        if re.match(r"^Hide\b", t):
            hidden = True
        if re.match(r"^Show\b", t):
            hidden = False
        m = re.match(r"""^Type\s+(["'])(.*)\1\s*$""", t)
        if m and not hidden and m.group(2) not in allowed:
            stray.append(f"tape.tape:{number}: {m.group(2)}")
    assert not stray, "typed lines that are not in the README Quick start:\n" + "\n".join(stray)
