"""
Every docs page that installs the Python SDK must name the published package.

The SDK is published as `aim-sdk` (the `name=` in setup.py), and the README
installs it with `pip install aim-sdk`. Several pages under `docs/` said the
opposite -- "There is NO pip package", "NO pip install available" -- and sent
readers to a dashboard ZIP as the only way in. A reader who followed the README
and then opened one of those pages was told the command they had just run could
not exist.

The same pages linked to a quick-start anchor that no heading produced, so the
"instructions" link landed at the top of the page.

NOT marked `integration` -- `pytest.ini` deselects that marker by default.
"""

import ast
import os
import re

import pytest

SDK_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
REPO_ROOT = os.path.normpath(os.path.join(SDK_ROOT, "..", ".."))
DOCS_ROOT = os.path.join(REPO_ROOT, "docs")

# The pages that walk a reader through installing the Python SDK.
INSTALL_PAGES = [
    "quick-start.md",
    "sdk/python.md",
    "tutorials/sdk-quickstart.md",
    "examples/weather-agent.md",
    "examples/flight-tracker.md",
    "examples/database-agent.md",
    "integrations/crewai.md",
    "integrations/langchain.md",
]

NO_PIP_CLAIM = re.compile(r"\bno\s+pip\s+(install|package)\b", re.IGNORECASE)

pytestmark = pytest.mark.skipif(
    not os.path.isdir(DOCS_ROOT),
    reason=f"{DOCS_ROOT} not present (sdk distributed standalone)",
)


def _published_name():
    """The distribution name, read from the `setup()` call in setup.py."""
    source = open(os.path.join(SDK_ROOT, "setup.py"), encoding="utf-8").read()
    for node in ast.walk(ast.parse(source)):
        if isinstance(node, ast.Call) and getattr(node.func, "id", None) == "setup":
            for keyword in node.keywords:
                if keyword.arg == "name" and isinstance(keyword.value, ast.Constant):
                    return keyword.value.value
    raise AssertionError("setup.py has no literal name= in its setup() call")


def _read(relative):
    return open(os.path.join(DOCS_ROOT, relative), encoding="utf-8").read()


def _docs_markdown():
    for directory, _, files in os.walk(DOCS_ROOT):
        for name in sorted(files):
            if name.endswith(".md"):
                path = os.path.join(directory, name)
                yield os.path.relpath(path, DOCS_ROOT), path


def _github_anchor(heading):
    """The fragment GitHub generates for a Markdown heading."""
    slug = heading.strip().lower()
    slug = re.sub(r"[^\w\- ]", "", slug)
    return slug.replace(" ", "-")


def test_the_package_name_is_aim_sdk():
    assert _published_name() == "aim-sdk"


def test_no_docs_page_says_there_is_no_pip_package():
    offenders = []
    for relative, path in _docs_markdown():
        for number, line in enumerate(open(path, encoding="utf-8"), start=1):
            if NO_PIP_CLAIM.search(line):
                offenders.append(f"docs/{relative}:{number}: {line.strip()}")
    assert not offenders, (
        "these lines say the SDK has no pip package, but it is published as "
        f"{_published_name()!r}:\n" + "\n".join(offenders)
    )


@pytest.mark.parametrize("relative", INSTALL_PAGES)
def test_install_page_names_the_pip_install_command(relative):
    name = re.escape(_published_name())
    # Not followed by more of a package name, so `aim-sdk-python` does not count.
    command = re.compile(rf"pip install [\"']?{name}(\[[a-z,]+\])?(?![\w.-])")
    assert command.search(_read(relative)), (
        f"docs/{relative} walks through installing the SDK but never gives "
        f"`pip install {_published_name()}`, the README's install command"
    )


def test_links_into_the_quick_start_land_on_a_heading():
    quick_start = _read("quick-start.md")
    anchors = {
        _github_anchor(match.group(1))
        for match in re.finditer(r"^#{1,6}\s+(.+)$", quick_start, re.MULTILINE)
    }
    broken = []
    for relative, path in _docs_markdown():
        text = open(path, encoding="utf-8").read()
        for anchor in re.findall(r"quick-start\.md#([^)\s]+)", text):
            if anchor not in anchors:
                broken.append(f"docs/{relative}: quick-start.md#{anchor}")
    assert not broken, "links to quick-start.md anchors no heading produces:\n" + "\n".join(broken)
