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
the recorded walkthrough's typed and install lines to the root README's Quick
start, and the root README's captured register output, in its first 30
lines, to what this SDK prints for that call and the SDK version it names.

Everything here runs offline and reads only files in this repository; the
check of the walkthrough's own extractor also runs it with node, when node
is installed.
"""

import ast
import json
import re
import shutil
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
RENDER = REPO_ROOT / "docs" / "demo" / "render.sh"
EXTRACTOR = REPO_ROOT / "docs" / "demo" / "lib" / "extract-readme.mjs"

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
    assert output[0] == "" and "Agent registered:" in output[1], (
        "the fence after the register call must be its captured output"
    )
    assert output[3] <= FIRST_SCREEN, (
        f"the captured output closes on line {output[3]}; the register call and all of its "
        f"output must fit in the first {FIRST_SCREEN} lines"
    )
    assert re.search(r"\b20\d\d-\d\d-\d\d\b", prose), (
        "the text before the captured output must say when it was captured"
    )


# "commit <hash>" in any case, with or without a backtick before the hash, or a
# bare full-length hash.
NAMED_COMMIT = re.compile(r"\bcommit\s+`?[0-9a-f]{7,40}\b|\b[0-9a-f]{40}\b", re.IGNORECASE)


def test_root_readme_dates_its_captured_output_without_naming_a_commit():
    """A commit hash in the capture sentence chases the branch it was captured on: the
    hash exists only there until the branch lands, and lands under a different hash, so a
    reader following it reaches a commit the repository does not have. The sentence names
    the date and the repository, not a commit."""
    _, _, prose = register_and_output(ROOT_README)
    named = NAMED_COMMIT.findall(prose)
    assert not named, (
        f"the text before the captured output names a commit {named!r}; say when it was "
        "captured and that the server and SDK were built from this repository"
    )


@pytest.mark.parametrize("sentence", [
    "captured at commit abcdef1 on a self-hosted AIM",
    "Commit abcdef1 was the build",
    "captured at commit `abcdef1` on a self-hosted AIM",
    "captured at COMMIT `ABCDEF1`",
    "built from 0123456789abcdef0123456789abcdef01234567",
])
def test_the_named_commit_guard_catches_each_phrasing_of_a_hash(sentence):
    assert NAMED_COMMIT.search(sentence), f"the guard misses a commit named as {sentence!r}"


@pytest.mark.parametrize("sentence", [
    "captured on 2026-10-09 with the server and SDK built from this repository",
    "the SDK reports version 2.0.3",
    "committed abcdef1",
    "the commit message",
])
def test_the_named_commit_guard_passes_a_sentence_that_names_no_commit(sentence):
    assert not NAMED_COMMIT.search(sentence), f"the guard flags {sentence!r}, which names no commit"


def test_root_readme_names_the_sdk_version_its_captured_output_came_from():
    """`pip install` fetches the release on PyPI, which can print the registration
    differently from the SDK in this repository: the aim-sdk 2.0.3 release prints a check
    mark where this repository's SDK prints [OK], under the same version number. The
    capture sentence names the version, so a reader can tell which SDK printed the lines
    below it, and a version bump fails here until the sentence is checked again."""
    _, _, prose = register_and_output(ROOT_README)
    version = (SDK_DIR / "VERSION").read_text(encoding="utf-8").strip()
    assert re.search(rf"(?<![\w.]){re.escape(version)}(?!\.?\d)", prose), (
        f"the text before the captured output must name the SDK version it was captured "
        f"with, {version} (sdk/python/VERSION)"
    )


def test_root_readme_installs_the_rich_extra_its_captured_output_shows():
    # Without rich the SDK prints the registration in an 11-line panel, which
    # does not fit in the first screen with the commands above it.
    installs = [
        line for lang, body in fences(quick_start(ROOT_README)) if lang == "bash"
        for line in body.split("\n") if line.startswith("pip install")
    ]
    assert installs == ['pip install "aim-sdk[rich]"'], (
        f"the Quick start must install the rich extra its captured output shows, got {installs!r}"
    )


class MarkupStrippingConsole:
    """Prints what rich prints for this SDK's console markup, without colour:
    style tags such as [dim] and [/] dropped, an escaped \\[ printed as [. The
    comparison below then holds whether or not rich is installed."""

    def print(self, text=""):
        print(re.sub(r"\\\[|\[/?[a-z0-9 #_]*\]", lambda m: "[" if m.group(0) == "\\[" else "", str(text)))


def printed_line_pattern(module, marker):
    """A regex for the f-string in `module` whose text contains `marker`."""
    for node in ast.walk(ast.parse(Path(module.__file__).read_text(encoding="utf-8"))):
        if isinstance(node, ast.JoinedStr):
            parts = [v.value if isinstance(v, ast.Constant) else None for v in node.values]
            if marker in "".join(p for p in parts if p is not None):
                return "".join(".+?" if p is None else re.escape(p) for p in parts)
    raise AssertionError(f"{module.__name__} has no f-string containing {marker!r}")


def test_root_readme_register_output_is_what_this_sdk_prints_for_that_call(monkeypatch, capsys):
    """Holds the captured lines to the format this SDK prints, and to the call above them:
    the agent name, its capabilities and the pending status. The values the server returns,
    the ID, type, version and trust score, are read from the capture and printed back
    through this SDK's console, so they are not checked: the trust score is computed by the
    server, and changing "Trust: 59%" to "Trust: 95%" keeps this test green by design."""
    from aim_sdk import console as console_module
    from aim_sdk import oauth as oauth_module

    register, output, _ = register_and_output(ROOT_README)
    call = next(
        node for node in ast.walk(ast.parse(register[1]))
        if isinstance(node, ast.Call) and getattr(node.func, "id", None) == "secure"
    )
    name = call.args[0].value
    capabilities = next(ast.literal_eval(k.value) for k in call.keywords if k.arg == "capabilities")

    lines = output[1].split("\n")
    detection = re.fullmatch(r"  ○ Agent Type: using default '(\w+)'", lines[0])
    assert output[0] == "" and detection, (
        "the fence after the register call must be its captured output, "
        "opening with the agent-type line secure() prints"
    )
    # The first call after `aim-sdk login` refreshes the sign-in, and the server
    # rotates the refresh token; the SDK prints one line for that.
    assert re.fullmatch(printed_line_pattern(oauth_module, "Token rotated successfully"), lines[1]), (
        "the captured output's second line must be the token-rotation line this SDK prints"
    )
    shown = re.fullmatch(
        r"\n\[OK\] Agent registered: (?P<name>.+)\n"
        r"  ID: (?P<id>\S+)  Type: (?P<type>\S+)  Version: (?P<version>\S+)\n"
        r"  Status: ○ (?P<status>\S+)  Trust: (?P<trust>\d+)%\n"
        r"  Capabilities: (?P<capabilities>.+?)\n*",
        "\n".join(lines[2:]),
    )
    assert shown, "the captured output must end with the registration lines the rich extra prints"
    assert shown["name"] == name, "the captured output names a different agent than the call"
    assert shown["capabilities"] == ", ".join(capabilities), (
        "the captured output lists different capabilities than the call grants"
    )
    assert shown["status"] == "pending", "a newly registered agent is pending until verified"

    printer = console_module.AIMConsole()
    printer.console = MarkupStrippingConsole()
    monkeypatch.setattr(console_module, "RICH_AVAILABLE", True)
    printer.detection_none("Agent Type", fallback=detection.group(1))
    printer.agent_registered(
        name=name,
        agent_id=shown["id"].replace("...", "-00000000-"),  # shortened back to the captured ID
        agent_type=shown["type"],
        version=shown["version"],
        trust_score=int(shown["trust"]),
        status=shown["status"],
        capabilities=capabilities,
    )
    printed = capsys.readouterr().out.split("\n", 1)
    assert "\n".join([printed[0], lines[1], printed[1]]).rstrip("\n") == output[1].rstrip("\n"), (
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


TYPED_FENCES = ("bash", "python")  # any other fence, or one with no language, is captured output


def readme_code_lines():
    """The lines the walkthrough may type, extracted as docs/demo/lib/extract-readme.mjs does:
    the lines of the Quick start's bash and python fences, without a trailing comment."""
    out = set()
    for lang, body in fences(quick_start(ROOT_README)):
        if lang not in TYPED_FENCES:
            continue
        for raw in body.split("\n"):
            line = re.sub(r"\s+#.*$", "", raw).rstrip()
            if line.strip():
                out.add(line)
    return out


def node_extractor_lines():
    """The lines docs/demo/lib/extract-readme.mjs extracts from the root README."""
    if shutil.which("node") is None:
        pytest.skip("node is not installed")
    run = subprocess.run(
        ["node", str(EXTRACTOR), str(ROOT_README)],
        capture_output=True, text=True, timeout=60, check=True,
    )
    return set(json.loads(run.stdout)["lines"])


def captured_output_lines():
    """Lines of the Quick start's other fences that no bash or python fence also holds."""
    captured = set()
    for lang, body in fences(quick_start(ROOT_README)):
        if lang not in TYPED_FENCES:
            captured |= {line.rstrip() for line in body.split("\n") if line.strip()}
    return captured - readme_code_lines()


@pytest.mark.parametrize("extract", [
    pytest.param(readme_code_lines, id="python-mirror"),
    pytest.param(node_extractor_lines, id="extract-readme.mjs"),
])
def test_walkthrough_may_not_type_the_quick_start_captured_output(extract):
    captured = captured_output_lines()
    assert "[OK] Agent registered: my-first-agent" in captured, (
        "the root README's register output is no longer an unlabelled fence in its Quick start"
    )
    typeable = sorted(captured & extract())
    assert not typeable, f"captured output lines the walkthrough may type: {typeable}"


def test_walkthrough_extractor_and_its_python_mirror_agree():
    assert node_extractor_lines() == readme_code_lines(), (
        "docs/demo/lib/extract-readme.mjs and readme_code_lines() extract different lines"
    )


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


def test_walkthrough_render_types_the_root_readme_install_line():
    # render.sh puts the install step into the tape at render time, inside a
    # shell-quoted string, so the tape test above never sees it.
    typed = re.findall(r"""\\nType (?:'"'"'|")(pip install .*?)(?:'"'"'|")\\n""", RENDER.read_text(encoding="utf-8"))
    assert typed, "render.sh no longer types an install line"
    stray = sorted(set(typed) - readme_code_lines())
    assert not stray, f"render.sh types install lines that are not in the README Quick start: {stray}"
