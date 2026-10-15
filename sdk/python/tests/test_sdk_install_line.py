"""
The Python SDK's install line is `pip install aim-sdk` (#627).

setup.py's install_requires carries the distributions the SDK imports; rich is an
extra, and aim_sdk/console.py prints plain text when it is not installed. The root
README's Quick start installs "aim-sdk[rich]" because the output it captures is the
rich form, and says the plain line works (tests/test_readme_quick_start_runs_as_written.py
holds it to both). These tests hold the other places a reader is told how to install
the SDK to the plain line, and the SDK README's Requirements to naming what each of
the two install lines installs.

Everything here runs offline and reads only files in this repository.
"""

import ast
import re
from pathlib import Path

import pytest

SDK_DIR = Path(__file__).resolve().parent.parent          # .../sdk/python
REPO_ROOT = SDK_DIR.parent.parent                         # repo root
ROOT_README = REPO_ROOT / "README.md"
SDK_README = SDK_DIR / "README.md"
DASHBOARD_SDK_PAGE = REPO_ROOT / "apps" / "web" / "app" / "dashboard" / "sdk" / "page.tsx"

INSTALL_LINE = "pip install aim-sdk"
INSTALLS_THE_SDK = r"pip install\s+['\"]?aim-sdk\b"


def section(path, heading):
    """The text under `## <heading>` up to the next `## ` heading."""
    lines = path.read_text(encoding="utf-8").split("\n")
    start = lines.index(f"## {heading}")
    end = next((i for i, line in enumerate(lines) if i > start and line.startswith("## ")), len(lines))
    return "\n".join(lines[start:end])


def bash_install_lines(path):
    """The lines of a page's bash fences that install the SDK, trailing shell comment dropped."""
    text = path.read_text(encoding="utf-8")
    return [
        re.sub(r"\s+#.*$", "", line).strip()
        for body in re.findall(r"^```bash\n(.*?)^```\s*$", text, re.MULTILINE | re.DOTALL)
        for line in body.split("\n") if re.match(INSTALLS_THE_SDK, line)
    ]


def root_readme_sdk_table_install():
    row = next(line for line in ROOT_README.read_text(encoding="utf-8").split("\n") if line.startswith("| Python |"))
    return [row.split("|")[2].strip().strip("`")]


def sdk_readme_requirements_install():
    return re.findall(rf"`({INSTALLS_THE_SDK}[^`]*)`", section(SDK_README, "Requirements"))


def dashboard_sdk_page_install():
    return re.findall(rf"<code[^>]*>({INSTALLS_THE_SDK}[^<]*)</code>", DASHBOARD_SDK_PAGE.read_text(encoding="utf-8"))


@pytest.mark.parametrize("install_lines", [
    pytest.param(root_readme_sdk_table_install, id="root-README-SDKs-table"),
    pytest.param(lambda: bash_install_lines(REPO_ROOT / "docs" / "use-cases" / "register-my-agent.md"),
                 id="register-my-agent-use-case"),
    pytest.param(lambda: bash_install_lines(REPO_ROOT / "docs" / "sdk" / "python.md"), id="python-SDK-guide"),
    pytest.param(sdk_readme_requirements_install, id="sdk-README-Requirements"),
    pytest.param(dashboard_sdk_page_install, id="dashboard-SDK-page"),
])
def test_each_copy_outside_the_quick_starts_gives_the_plain_install_line(install_lines):
    assert install_lines() == [INSTALL_LINE], (
        f"the SDK's install line is {INSTALL_LINE!r}; this copy gives {install_lines()!r}"
    )


def distribution(requirement):
    return re.split(r"[\s<>=!~;\[]", requirement, maxsplit=1)[0].lower()


def setup_keyword(name):
    tree = ast.parse((SDK_DIR / "setup.py").read_text(encoding="utf-8"))
    call = next(
        node for node in ast.walk(tree)
        if isinstance(node, ast.Call) and getattr(node.func, "id", None) == "setup"
    )
    return ast.literal_eval(next(k.value for k in call.keywords if k.arg == name))


def test_sdk_readme_requirements_name_what_each_install_line_installs():
    """`pip install aim-sdk` installs setup.py's install_requires, and the Quick start's
    "aim-sdk[rich]" adds the rich extra. The Requirements list names each distribution,
    and marks the extra optional, so a reader of either line finds what it installed."""
    requirements = section(SDK_README, "Requirements")
    items = {
        m.group(1).lower(): m.group(0)
        for m in re.finditer(r"^- `([^`]+)`.*$", requirements, re.MULTILINE)
    }
    required = {distribution(r) for r in setup_keyword("install_requires")}
    missing = sorted(required - items.keys())
    assert not missing, f"the SDK README's Requirements omit {missing}, which setup.py's install_requires declares"
    for name in sorted(distribution(r) for r in setup_keyword("extras_require")["rich"]):
        assert name in items, f"the SDK README's Requirements omit {name}, which the [rich] extra installs"
        assert "optional" in items[name], f"the SDK README's Requirements must mark {name} optional: {items[name]!r}"
