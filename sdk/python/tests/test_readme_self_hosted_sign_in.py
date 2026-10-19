"""
The root README's Quick start installs the SDK from PyPI, then signs in with
`aim-sdk login`, which in this repository's SDK prints a code you approve on
the dashboard's `/device` page. The release on PyPI, aim-sdk 2.0.3, has no
device sign-in: its `aim-sdk login --url http://localhost:8080` prints an
`http://localhost:8080/auth/login?response_type=code&...` address and waits,
and the API registers no `/auth/login` route. So the README sentence that
describes the code says the PyPI release does not yet, and links the issue
that tracks the publish and the self-hosted install in docs/quick-start.md:
the SDK archive the dashboard's SDK & docs page downloads, which carries the
user's sign-in.

The install steps there are held to what the server and the dashboard send:
the page and its button, the archive's file name, its top folder and the path
of the sign-in inside it, each read from the source that produces it.

Everything here runs offline and reads only files in this repository.
"""

import json
import re
from pathlib import Path

from aim_sdk import credentials

# The autouse isolated_home fixture replaces this function for every test; the
# test of the lookup itself keeps the one the module defines.
FIND_PACKAGE_CREDENTIALS = credentials._find_sdk_package_credentials

SDK_DIR = Path(__file__).resolve().parent.parent          # .../sdk/python
REPO_ROOT = SDK_DIR.parent.parent                         # repo root
ROOT_README = REPO_ROOT / "README.md"
QUICK_START_DOC = REPO_ROOT / "docs" / "quick-start.md"
WEB_APP = REPO_ROOT / "apps" / "web" / "app"
SDK_PAGE = WEB_APP / "dashboard" / "sdk" / "page.tsx"
SDK_HANDLER = REPO_ROOT / "apps" / "backend" / "internal" / "interfaces" / "http" / "handlers" / "sdk_handler.go"
SERVER_DIR = REPO_ROOT / "apps" / "backend" / "cmd" / "server"
BACKEND_DOCKERFILE = REPO_ROOT / "infrastructure" / "docker" / "Dockerfile.backend"

TRACKING_ISSUE = "https://github.com/opena2a-org/agent-identity-management/issues/789"
SELF_HOSTED_HEADING = "Install the SDK from your dashboard"


def github_anchor(heading):
    """The fragment GitHub gives a markdown heading."""
    return re.sub(r"[^\w\- ]", "", heading.strip().lower()).replace(" ", "-")


def without_fences(text):
    return re.sub(r"^```\w*\n.*?^```\s*$", "", text, flags=re.MULTILINE | re.DOTALL)


def device_sentence():
    """The one sentence of the root README's Quick start prose that names `/device`."""
    lines = ROOT_README.read_text(encoding="utf-8").split("\n")
    start = lines.index("## Quick start")
    end = next(i for i, line in enumerate(lines) if i > start and line.startswith("## "))
    paragraphs = re.split(r"\n\s*\n", without_fences("\n".join(lines[start:end])))
    found = [s for p in paragraphs for s in re.split(r"(?<=[.!?])\s+", p.strip()) if "`/device`" in s]
    assert len(found) == 1, f"the Quick start must name `/device` in one sentence: {found}"
    return found[0]


def doc_section(path, heading):
    """The lines under a markdown heading, up to the next heading of the same or a higher
    level; lines inside fenced code are never headings."""
    lines = path.read_text(encoding="utf-8").split("\n")
    in_fence, start, level = False, None, None
    for number, line in enumerate(lines):
        if re.match(r"^\s*```", line):
            in_fence = not in_fence
            continue
        m = None if in_fence else re.match(r"^(#{1,6})\s+(.*\S)\s*$", line)
        if not m:
            continue
        if start is None:
            if m.group(2) == heading:
                start, level = number, len(m.group(1))
        elif len(m.group(1)) <= level:
            return lines[start + 1:number]
    assert start is not None, f"{path.name} has no heading {heading!r}"
    return lines[start + 1:]


def sprintf_format(source, variable):
    """The format string of `variable := fmt.Sprintf("...", ...)` in Go source."""
    m = re.search(rf'\b{variable} := fmt\.Sprintf\("([^"]+)"', source)
    assert m, f"{SDK_HANDLER.name} no longer builds {variable} with fmt.Sprintf"
    return m.group(1)


def test_the_device_sign_in_sentence_says_the_pypi_release_does_not_yet_and_links_the_install():
    sentence = device_sentence()
    assert re.search(r"\bPyPI\b.*\bnot yet\b", sentence), (
        f"the sentence that says `aim-sdk login` prints a code must say the PyPI release does not yet: {sentence!r}"
    )
    link = f"](docs/quick-start.md#{github_anchor(SELF_HOSTED_HEADING)})"
    assert link in sentence, f"the sentence must link the self-hosted install, {link!r}: {sentence!r}"
    assert f"]({TRACKING_ISSUE})" in sentence, f"the sentence must link {TRACKING_ISSUE}: {sentence!r}"


def test_the_quick_start_doc_names_the_issue_that_tracks_the_publish():
    assert f"]({TRACKING_ISSUE})" in QUICK_START_DOC.read_text(encoding="utf-8"), (
        f"docs/quick-start.md must link {TRACKING_ISSUE}, the issue that tracks the publish"
    )


def test_the_self_hosted_install_names_the_page_button_archive_folder_and_sign_in_the_server_sends():
    section = doc_section(QUICK_START_DOC, SELF_HOSTED_HEADING)
    prose = without_fences("\n".join(section))
    commands = [
        line.strip()
        for block in re.findall(r"^```bash\n(.*?)^```\s*$", "\n".join(section), re.MULTILINE | re.DOTALL)
        for line in block.split("\n")
    ]

    page = "/" + SDK_PAGE.parent.relative_to(WEB_APP).as_posix()
    assert SDK_PAGE.is_file() and f"`{page}`" in prose, f"the install must name the dashboard page {page}"
    button = re.search(
        r"selectedSDK === 'python' \? 'Downloading\.\.\.' : '([^']+)'", SDK_PAGE.read_text(encoding="utf-8")
    )
    assert button, f"{SDK_PAGE.name} no longer labels its Python download button this way"
    assert f'"{button.group(1)}"' in prose, f"the install must name the button {button.group(1)!r}"

    handler = SDK_HANDLER.read_text(encoding="utf-8")
    archive = sprintf_format(handler, "filename").replace("%s", "python", 1).replace("%s", "*", 1)
    folder = sprintf_format(handler, "zipPrefix").replace("%s", "python")
    sign_in = re.search(r'credPath := filepath\.Join\(zipPrefix, "([^"]+)", "([^"]+)"\)', handler)
    assert sign_in, f"{SDK_HANDLER.name} no longer places the sign-in this way"
    # A browser saves a second download of the archive as "<name> (1).zip", and given two
    # archives unzip extracts nothing, so the install unzips the newest one.
    newest = f'unzip "$(ls -t ~/Downloads/{archive} | head -n 1)"'
    assert newest in commands, f"the install must unzip the newest {archive}: {commands}"
    assert f"unzip ~/Downloads/{archive}" not in commands, f"the install must not unzip every {archive}: {commands}"
    assert f"cd {folder}" in commands, f"the install must enter the archive's folder, {folder}: {commands}"
    assert any(re.fullmatch(r'pip install -e "?\.(\[[a-z,]+\])?"?', c) for c in commands), (
        f"the install must be editable, so the SDK reads the archive's sign-in from any directory: {commands}"
    )
    assert f"`{folder}/{sign_in.group(1)}/{sign_in.group(2)}`" in prose, "the install must name where the sign-in is"


def test_the_archive_is_the_sdk_tree_the_server_image_copies():
    assert re.search(r"^COPY sdk \./sdk$", BACKEND_DOCKERFILE.read_text(encoding="utf-8"), re.MULTILINE), (
        "the server image no longer copies the sdk/ tree the download is built from"
    )
    handler = SDK_HANDLER.read_text(encoding="utf-8")
    assert 'sdkBaseDir = filepath.Join(".", "sdk")' in handler
    assert "sdkRoot := filepath.Join(sdkBaseDir, sdkType)" in handler


def test_the_api_registers_no_auth_login_route_at_its_root():
    """The page the release on PyPI opens is <url>/auth/login; the API's sign-in routes
    are under /api/v1."""
    sources = [p for p in sorted(SERVER_DIR.glob("*.go")) if not p.name.endswith("_test.go")]
    assert sources
    root_auth = re.compile(r'\bapp\.(Get|Post|Put|Patch|Delete|All|Group|Use)\("/auth\b')
    found = [
        f"{p.name}: {line.strip()}"
        for p in sources
        for line in p.read_text(encoding="utf-8").split("\n")
        if root_auth.search(line) or '"/auth/login"' in line
    ]
    assert not found, f"the API registers an /auth route at its root: {found}"


def test_the_sdk_reads_the_archives_sign_in_from_the_folder_that_holds_the_package(tmp_path, monkeypatch):
    """After `pip install -e` from the unzipped archive the package is that folder's aim_sdk,
    so the lookup finds the folder's sign-in from any working directory."""
    archive = tmp_path / "aim-sdk-python"
    (archive / "aim_sdk").mkdir(parents=True)
    (archive / ".aim").mkdir()
    sign_in = archive / ".aim" / "sdk_credentials.json"
    sign_in.write_text(json.dumps({"type": "sdk_oauth", "refreshToken": "not-a-real-token"}), encoding="utf-8")
    elsewhere = tmp_path / "elsewhere"
    elsewhere.mkdir()
    monkeypatch.chdir(elsewhere)

    import aim_sdk

    monkeypatch.setattr(aim_sdk, "__file__", str(archive / "aim_sdk" / "__init__.py"))
    found = FIND_PACKAGE_CREDENTIALS()
    assert found is not None, "the SDK does not read the sign-in in the folder that holds its package"
    data, path = found
    assert path == sign_in
    assert data["refreshToken"] == "not-a-real-token"
