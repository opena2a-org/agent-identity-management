"""
docs/demo/quickstart.json describes the committed terminal recording of the
README's Quick start, and each of its claims is checked here against the files
beside it.

docs/demo/record-quickstart.sh renders docs/demo/quickstart.tape on a local
stack and writes the GIF, the step screenshots, the text version and the
sidecar. Nothing here renders: the tests read the committed files. They fail
when the tape changes without a new render (the sidecar's scriptSha256,
commands and holds no longer match it), when a committed output is replaced by
hand (its sha256 no longer matches), or when the pace leaves the rule the
recording is made to: 100-140 ms per typed character and at most 72 characters
a line, 600-1000 ms before each Enter, each step held at least as long as its
narration and its lines on screen need and at most 12 s, the length within its
declared limit, and a GIF of at most 3,000,000 bytes and 1200 px wide.
"""

import hashlib
import json
import re
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parents[3]
DEMO = REPO_ROOT / "docs" / "demo"
SIDECAR = DEMO / "quickstart.json"
TAPE = DEMO / "quickstart.tape"
NARRATION = DEMO / "quickstart.narration.json"


def _sha256(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def _seconds(value):
    """A VHS duration (`700ms`, `5s`, `0.5`) in seconds."""
    match = re.fullmatch(r"(\d+(?:\.\d+)?)(ms|s)?", value)
    assert match, f"not a VHS duration: {value}"
    return float(match.group(1)) / (1000 if match.group(2) == "ms" else 1)


def _read_tape():
    """The tape as the recorder plays it: its settings, the lines it shows, the
    pause before each shown Enter, and each step's hold after its last shown Enter."""
    text = TAPE.read_text(encoding="utf-8")
    settings = dict(re.findall(r"^Set (\w+) (\S+)", text, flags=re.M))
    hidden, last, typed = False, None, ""
    shown, pre_enter, steps = [], [], []
    for raw in text.splitlines():
        line = raw.strip()
        step = re.match(r"#\s*step:\s*(\S+)\s*(.*)$", line)
        if step:
            steps.append({"id": step.group(1), "title": step.group(2), "hold": 0.0, "counting": False, "shots": []})
            last = None
            continue
        if not line or line.startswith("#"):
            continue
        current = steps[-1] if steps else None
        typed_match = re.match(r"^Type(@\S+)?\s+([\"'`])(.*)\2$", line)
        if line in ("Hide", "Show"):
            hidden = line == "Hide"
        elif typed_match:
            if not hidden:
                assert typed_match.group(1) is None, f"a shown Type sets its own speed: {line}"
                typed += typed_match.group(3)
            if current:
                current["counting"] = False
        elif line.startswith("Sleep "):
            if current and current["counting"] and not hidden:
                current["hold"] += _seconds(line.split()[1])
        elif line == "Enter":
            if not hidden:
                pre_enter.append(_seconds(last.split()[1]) if last and last.startswith("Sleep ") else 0.0)
                shown.append(typed)
                typed = ""
                if current:
                    current["hold"], current["counting"] = 0.0, True
        elif line.startswith("Screenshot ") and current:
            current["shots"].append(line.split()[1])
        last = line
    return settings, shown, pre_enter, steps


def _sidecar():
    return json.loads(SIDECAR.read_text(encoding="utf-8"))


def test_the_sidecar_names_its_script_and_what_it_ran():
    sidecar = _sidecar()
    narration = json.loads(NARRATION.read_text(encoding="utf-8"))
    assert sidecar["schema"] == "demo-sidecar/2"
    assert sidecar["surfaceClass"] == "terminal"
    assert sidecar["tool"] == "aim-sdk"
    assert sidecar["script"] == "docs/demo/quickstart.tape"
    assert sidecar["scriptSha256"] == _sha256(TAPE), "the tape changed after the render: run docs/demo/record-quickstart.sh"
    assert sidecar["narration"] == "docs/demo/quickstart.narration.json"
    assert narration["schema"] == "demo-narration/1"
    assert narration["version"] == sidecar["version"]
    assert isinstance(sidecar["prerelease"], bool)
    assert sidecar["meteredCostUsd"] == 0
    assert (REPO_ROOT / sidecar["capture"]).is_file()


def test_the_commands_are_the_lines_the_tape_shows():
    _, shown, _, _ = _read_tape()
    assert _sidecar()["commands"] == [line for line in shown if line]


def test_each_committed_output_matches_its_recorded_hash():
    outputs = _sidecar()["outputs"]
    for entry in [outputs["gif"], *outputs["frames"]]:
        path = REPO_ROOT / entry["path"]
        assert path.stat().st_size == entry["bytes"], entry["path"]
        assert _sha256(path) == entry["sha256"], entry["path"]
    assert outputs["gif"]["bytes"] <= 3_000_000
    assert outputs["gif"]["width"] <= 1200


def test_each_step_is_held_as_long_as_its_narration_and_lines_on_screen_need():
    _, _, _, steps = _read_tape()
    narration = {s["id"]: s["text"] for s in json.loads(NARRATION.read_text(encoding="utf-8"))["steps"]}
    recorded = _sidecar()["steps"]
    assert [s["id"] for s in recorded] == [s["id"] for s in steps] == list(narration)
    for tape_step, step in zip(steps, recorded):
        words = len(narration[step["id"]].split())
        needed = max(3.0, step["linesOnScreen"] / 4, words / 2.5 + 0.5)
        assert step["narrationWords"] == words, step["id"]
        assert step["anchorFrame"] == tape_step["shots"][0], step["id"]
        assert step["holdSeconds"] == tape_step["hold"], step["id"]
        assert needed - 0.01 <= step["requiredHoldSeconds"] <= step["holdSeconds"] <= 12, step["id"]


def test_typing_framing_and_length_keep_to_the_pace_rule():
    settings, shown, pre_enter, _ = _read_tape()
    sidecar = _sidecar()
    assert 0.1 <= _seconds(settings["TypingSpeed"]) <= 0.14
    assert max(len(line) for line in shown) <= 72
    assert pre_enter and all(0.6 <= pause <= 1.0 for pause in pre_enter), pre_enter
    assert settings["CursorBlink"] == "false"
    assert 100 <= int(settings["Columns"]) <= 120 and 24 <= int(settings["Rows"]) <= 30
    assert 14 <= int(settings["FontSize"]) <= 16
    limit = {"front-door": 45, "multi-step": 90}[sidecar["lengthClass"]]
    assert sidecar["lengthLimitSeconds"] == limit
    assert sidecar["durationSeconds"] <= limit


def test_the_text_version_shows_what_the_readme_promises():
    capture = (REPO_ROOT / _sidecar()["capture"]).read_text(encoding="utf-8")
    assert "Signed in as admin@example.com on http://localhost:8080." in capture
    assert "Agent registered: my-first-agent" in capture
    assert "{'name': 'Example Co', 'plan': 'team'}" in capture
    assert "ActionDeniedError: AIM denied 'db:write'" in capture
    assert "Agent not verified" not in capture
