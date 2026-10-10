"""
The Quick start in the root README and in this SDK's README is one Python
session a reader types top to bottom: sign in, register, verify, one allowed
call, the strict-mode step, one refused call.

These tests run that example as written: the Python fences, in order, as one
script in a fresh interpreter (`python -I`), with `aim_sdk` replaced by a
stand-in module that enforces exactly the capabilities the example declares.
A name the example uses without defining it raises NameError; a call the
example never makes is counted as missing; the refused call ends the script
as it would end a reader's, so it has to be the last line. They also hold
the minimal dev stack command to the services docker-compose.yml defines,
the recorded walkthrough's typed lines to the root README's Quick start, and
the root README's captured register output, in its first 30 lines, to what
this SDK prints for that call.

Everything here runs offline and reads only files in this repository.
"""

import ast
import json
import re
import subprocess
import sys
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
    start = next(i for i, line in enumerate(lines) if re.match(r"^## quick start\s*$", line, re.IGNORECASE))
    end = next((i for i, line in enumerate(lines) if i > start and line.startswith("## ")), len(lines))
    return "\n".join(lines[start:end])


def fences(section):
    """(language, body) for every fenced block in the section, in order."""
    return [
        (m.group(1), m.group(2))
        for m in re.finditer(r"^```(\w*)\n(.*?)^```\s*$", section, re.MULTILINE | re.DOTALL)
    ]


# The stand-in `aim_sdk` the example imports. `secure` records each agent;
# `perform_action` refuses a capability the agent does not hold with
# ActionDeniedError, a PermissionError like the SDK's, and never runs the
# function body for it. At interpreter exit, which also follows an uncaught
# exception, it writes the agents, their calls (with the script line that made
# each) and the script's JSON-serialisable module names to result.json next to
# itself. JSON turns the example's integer dictionary keys into strings.
STUB_AIM_SDK = '''\
import atexit
import json
import sys
from pathlib import Path

_agents = []


class ActionDeniedError(PermissionError):
    pass


class _Agent:
    def __init__(self, name, capabilities):
        self.name = name
        self.capabilities = list(capabilities or [])
        self.calls = []

    def perform_action(self, capability, **_):
        def decorate(fn):
            def wrapper(*args, **kwargs):
                line = sys._getframe(1).f_lineno
                if capability not in self.capabilities:
                    self.calls.append(["refused", capability, None, line])
                    raise ActionDeniedError(f"AIM denied {capability!r}")
                result = fn(*args, **kwargs)
                self.calls.append(["allowed", capability, result, line])
                return result

            return wrapper

        return decorate


def secure(name, capabilities=None, **_):
    agent = _Agent(name, capabilities)
    _agents.append(agent)
    return agent


def _record():
    names = {}
    for key, value in vars(sys.modules["__main__"]).items():
        if key.startswith("_"):
            continue
        try:
            names[key] = json.loads(json.dumps(value))
        except (TypeError, ValueError):
            pass
    Path(__file__).with_name("result.json").write_text(
        json.dumps({
            "agents": [
                {"name": a.name, "capabilities": a.capabilities, "calls": a.calls}
                for a in _agents
            ],
            "main": names,
        }),
        encoding="utf-8",
    )


atexit.register(_record)
'''

# `python -I` puts neither the script's directory nor any environment path on
# sys.path, so the script itself points at the stand-in before the example's
# first line. The three names are deleted again: the example must define
# every name it uses.
PRELUDE = [
    "import os as _os, sys as _sys",
    "_sys.path.insert(0, _os.path.dirname(_os.path.abspath(__file__)))",
    "del _os, _sys",
]


def run_example(path, tmp_path):
    """Run the Quick start's Python fences, in order, as one script in a fresh interpreter.

    Returns the recorded agents and the script's module names. A script that
    ends in an error is accepted only when the error is the stand-in's
    ActionDeniedError raised from the script's last statement: the example's
    refused call.
    """
    blocks = [body for lang, body in fences(quick_start(path)) if lang == "python"]
    assert blocks, f"{path.name}: the Quick start has no Python fence"

    (tmp_path / "aim_sdk.py").write_text(STUB_AIM_SDK, encoding="utf-8")
    lines = list(PRELUDE)
    first_line = []                      # 1-based script line each fence starts on
    for body in blocks:
        first_line.append(len(lines) + 1)
        lines.extend(body.split("\n"))
    script = tmp_path / "quickstart.py"
    script.write_text("\n".join(lines) + "\n", encoding="utf-8")

    done = subprocess.run(
        [sys.executable, "-I", "-B", str(script)],
        cwd=tmp_path, capture_output=True, text=True, timeout=60, check=False,
    )
    if done.returncode != 0:
        last = done.stderr.strip().split("\n")[-1]
        assert last.startswith("aim_sdk.ActionDeniedError:"), (
            f"{path.name}: the Quick start does not run as written:\n{done.stderr}"
        )
        raised_on = int(re.search(r'quickstart\.py", line (\d+)', done.stderr).group(1))
        fence = sum(1 for start in first_line if start <= raised_on)
        assert fence == len(blocks), (
            f"{path.name}: the refused call in Python fence {fence} ends the session, "
            f"so fences {fence + 1} to {len(blocks)} never run"
        )
        last = ast.parse(script.read_text(encoding="utf-8")).body[-1]
        assert last.lineno <= raised_on <= last.end_lineno, (
            f"{path.name}: the refused call ends the session, so the code after it "
            f"in the last Python fence never runs"
        )
    result = json.loads((tmp_path / "result.json").read_text(encoding="utf-8"))
    return result["agents"], result["main"]


@pytest.mark.parametrize("path", READMES)
def test_example_runs_as_written_with_one_allowed_and_one_refused_call(path, tmp_path):
    agents, names = run_example(path, tmp_path)

    assert len(agents) == 1, f"{path.name}: the example must register exactly one agent"
    agent = agents[0]
    assert agent["capabilities"] == ["db:read"], (
        f"{path.name}: the code must show that the agent holds db:read, "
        f"got capabilities={agent['capabilities']!r}"
    )
    allowed = [c for c in agent["calls"] if c[0] == "allowed"]
    refused = [c for c in agent["calls"] if c[0] == "refused"]
    assert [c[1] for c in allowed] == ["db:read"], (
        f"{path.name}: the example must make exactly one allowed db:read call, got {agent['calls']!r}"
    )
    assert [c[1] for c in refused] == ["db:write"], (
        f"{path.name}: the example must make exactly one refused db:write call, got {agent['calls']!r}"
    )
    assert allowed[0][2] == names["customers"]["42"], (
        f"{path.name}: the allowed call must return the record it reads"
    )
    assert "42" in names["customers"], (
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


FIRST_SCREEN = 30  # README lines a visitor reads before scrolling


def numbered_fences(path):
    """(language, body, opening line, closing line) for every fenced block, lines 1-based."""
    blocks = []
    opened = None
    for number, line in enumerate(path.read_text(encoding="utf-8").split("\n"), 1):
        if opened is None:
            m = re.match(r"^```(\w*)\s*$", line)
            if m:
                lang, body, opened = m.group(1), [], number
        elif re.match(r"^```\s*$", line):
            blocks.append((lang, "\n".join(body), opened, number))
            opened = None
        else:
            body.append(line)
    return blocks


def register_and_output(path):
    """The Python fence that calls secure(), the fence after it, and the prose before the second."""
    blocks = numbered_fences(path)
    i = next((i for i, b in enumerate(blocks) if b[0] == "python" and "secure(" in b[1]), None)
    assert i is not None, f"{path.name}: no Python fence calls secure()"
    assert i + 1 < len(blocks), f"{path.name}: nothing follows the register call"
    lines = path.read_text(encoding="utf-8").split("\n")
    prose_from = blocks[i - 1][3] if i > 0 else 0
    prose = "\n".join(
        line for number, line in enumerate(lines, 1)
        if prose_from < number < blocks[i + 1][2] and not blocks[i][2] <= number <= blocks[i][3]
    )
    return blocks[i], blocks[i + 1], prose


def test_root_readme_shows_the_register_call_and_its_dated_output_in_its_first_30_lines():
    register, output, prose = register_and_output(ROOT_README)
    assert register[2] <= FIRST_SCREEN, (
        f"the register call opens on line {register[2]}; it must open within the first {FIRST_SCREEN}"
    )
    assert output[0] == "" and "Agent Registered" in output[1], (
        "the fence after the register call must be its captured output"
    )
    assert output[2] < FIRST_SCREEN, (
        f"the captured output opens on line {output[2]}; it must start within the first {FIRST_SCREEN}"
    )
    assert re.search(r"\b20\d\d-\d\d-\d\d\b", prose), (
        "the text before the captured output must say when it was captured"
    )


def test_root_readme_register_output_is_what_this_sdk_prints_for_that_call(monkeypatch, capsys):
    from aim_sdk import console as console_module

    register, output, _ = register_and_output(ROOT_README)
    call = next(
        node for node in ast.walk(ast.parse(register[1]))
        if isinstance(node, ast.Call) and getattr(node.func, "id", None) == "secure"
    )
    name = call.args[0].value
    capabilities = next(ast.literal_eval(k.value) for k in call.keywords if k.arg == "capabilities")

    detection = re.fullmatch(r"  ○ Agent Type: using default '(\w+)'", output[1].split("\n", 1)[0])
    assert output[0] == "" and detection, (
        "the fence after the register call must be its captured output, "
        "opening with the agent-type line secure() prints"
    )
    rows = dict(re.findall(r"^│  ([A-Za-z ]+):\s+(.*?)\s*│$", output[1], re.MULTILINE))
    assert rows["Agent"] == name, "the captured output names a different agent than the call"
    assert rows["Capabilities"] == ", ".join(capabilities), (
        "the captured output lists different capabilities than the call grants"
    )
    assert rows["Status"] == "pending", "a newly registered agent is pending until verified"

    # The plain-text panel is what `pip install aim-sdk` prints: rich is not a dependency.
    monkeypatch.setattr(console_module, "RICH_AVAILABLE", False)
    printer = console_module.AIMConsole()
    printer.detection_none("Agent Type", fallback=detection.group(1))
    printer.agent_registered(
        name=name,
        agent_id=rows["ID"].replace("...", "-00000000-"),  # shortened back to the captured ID
        agent_type=rows["Type"],
        version=rows["Version"],
        trust_score=int(rows["Trust Score"].rstrip("%")),
        status=rows["Status"],
        capabilities=capabilities,
    )
    assert capsys.readouterr().out.rstrip("\n") == output[1].rstrip("\n"), (
        "the README's captured register output no longer matches what this SDK prints; "
        "capture it again"
    )


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
        r"^docker compose up -d (\S.*)$", ROOT_README.read_text(encoding="utf-8"), re.MULTILINE
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
