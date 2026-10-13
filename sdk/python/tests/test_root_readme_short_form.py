"""
The root README is the short form: a start block a visitor reads in one screen,
a job list, the usage sections, Install, Documentation and License, under a word
budget and a section budget, with no version or date in its text. Reference
passages live in docs/ and the SDK READMEs, and the captured output's provenance
lives in docs/captures/quick-start.json.

Everything here runs offline and reads only files in this repository.
"""

import json
import re
import subprocess
from pathlib import Path

import pytest

REPO_ROOT = Path(__file__).resolve().parent.parent.parent.parent
README = REPO_ROOT / "README.md"
SIDECAR = REPO_ROOT / "docs" / "captures" / "quick-start.json"

WORD_BUDGET = 1716          # the short-form ceiling for this README
WORD_TARGET = 885           # the short form's target, before the sentences below
SECTION_BUDGET = 7          # H2 headings
PARAGRAPH_CAP = 69          # words in the longest paragraph
PARAGRAPHS_OVER_60 = 1      # paragraphs allowed over 60 words
KEPT_SENTENCE_CAP = 25      # words in each Quick start sentence kept from the long form

VERSION_OR_DATE = re.compile(r"v?[0-9]+\.[0-9]+\.[0-9]+|20[0-9]{2}-[0-9]{2}-[0-9]{2}")


def text():
    return README.read_text(encoding="utf-8")


def h2_headings():
    return [line[3:].strip() for line in text().split("\n") if line.startswith("## ")]


def heading_anchors(path):
    """The anchors GitHub gives a markdown file's headings: the heading's text with link
    targets and punctuation dropped, lower-cased, spaces as hyphens, and -1, -2 after a
    repeat. Lines inside fenced code are not headings."""
    anchors, seen, in_fence = set(), {}, False
    for line in path.read_text(encoding="utf-8").split("\n"):
        if re.match(r"^\s*```", line):
            in_fence = not in_fence
            continue
        m = None if in_fence else re.match(r"^#{1,6}\s+(.*\S)\s*$", line)
        if not m:
            continue
        title = re.sub(r"!?\[([^\]]*)\]\([^)]*\)", r"\1", m.group(1))
        slug = re.sub(r"[^\w\- ]", "", title.lower()).replace(" ", "-")
        repeat = seen.get(slug, 0)
        seen[slug] = repeat + 1
        anchors.add(slug if repeat == 0 else f"{slug}-{repeat}")
    return anchors


def paragraphs():
    """Word counts of the README's paragraphs: runs of non-blank lines outside fenced
    code, excluding headings, table rows, HTML and image lines; each list item starts
    a new paragraph."""
    counts, current, in_fence = [], [], False

    def flush():
        if current:
            counts.append((" ".join(current), len(" ".join(current).split())))
            current.clear()

    for line in text().split("\n"):
        if re.match(r"^\s*```", line):
            flush()
            in_fence = not in_fence
            continue
        if in_fence:
            continue
        stripped = line.strip()
        if not stripped or re.match(r"^(#{1,6}\s|\||<|!\[|\[!\[|-{3,}$)", stripped):
            flush()
            continue
        if re.match(r"^([-*+]|\d+\.)\s", stripped):
            flush()
        current.append(stripped)
    flush()
    return counts


def test_readme_is_under_the_word_budget():
    words = len(text().split())
    assert words <= WORD_BUDGET, f"README.md has {words} words; the budget is {WORD_BUDGET}"


# Sentences the short form keeps, so a reader meets no state the README does not name:
# what pending means, the setting the refusal needs, where secure() writes, how the
# release on PyPI prints the captures, what the self-hosted install needs and writes,
# where its login comes from, and that it is unpinned.
KEPT_SENTENCES = [
    "until an administrator verifies it under Agents",
    "needs strict mode",
    "stores credentials at `~/.aim/`",
    "the release on PyPI prints",
    "of generated secrets to `./aim`",
    "Login credentials print at the end of the run",
    "nothing on this path is pinned or verified",
]

# Text that other tests read from this README: the token-rotation line and its Redis
# condition, the policy a quickstart-default stack names, what a pending agent's call
# raises in monitoring mode and is refused with in strict mode, and when the trust
# score is recalculated. The last marker is held as a whole paragraph.
PINNED_SENTENCES = [
    "Token rotated successfully",
    "`default_policy`",
    "`Unverified Agent Action (Monitoring Mode)`",
    "`Agent not verified - all actions denied`",
    "trust-score/recalculate",
]
PINNED_PARAGRAPH = "A verification request (`POST /api/v1/verifications`)"


def only(found, marker):
    assert len(found) == 1, f"README.md must say {marker!r} once, in one sentence or paragraph: {found}"
    return len(found[0].split())


def test_readme_is_under_its_target_plus_the_sentences_it_keeps():
    """The target counts the short form alone; each kept or pinned sentence, and the
    pinned paragraph, adds its own words and nothing more. Text added anywhere else, or
    a kept sentence repeated, fails here."""
    kept = sum(only([s for s in sentences() if m.lower() in s.lower()], m) for m in KEPT_SENTENCES)
    pinned = sum(only([s for s in sentences() if m in s], m) for m in PINNED_SENTENCES)
    pinned += only([p for p, _ in paragraphs() if PINNED_PARAGRAPH in p], PINNED_PARAGRAPH)
    budget = WORD_TARGET + kept + pinned
    words = len(text().split())
    assert words <= budget, (
        f"README.md has {words} words; the target is {WORD_TARGET} plus {kept} in kept sentences "
        f"and {pinned} in pinned ones, {budget}; move the rest to docs/"
    )


def test_readme_has_at_most_seven_sections_in_the_short_form_order():
    headings = h2_headings()
    assert len(headings) <= SECTION_BUDGET, f"README.md has {len(headings)} H2 sections: {headings}"
    assert headings[0] == "Quick start", "the start block comes first"
    assert headings[-1] == "License", "License comes last"
    for earlier, later in [("Quick start", "What you can do"), ("Install AIM (self-hosted)", "Documentation"),
                           ("Documentation", "License")]:
        assert headings.index(earlier) < headings.index(later), f"{earlier!r} must come before {later!r}"


def test_readme_names_no_version_or_date():
    """Anywhere in the file, captured output included: a capture line that carries a
    version or a date is elided in place, never edited, and the untrimmed text and the
    capture's provenance live in the sidecar."""
    hits = [f"{n}: {line}" for n, line in enumerate(text().split("\n"), 1) if VERSION_OR_DATE.search(line)]
    assert not hits, "versions and dates belong in docs/ and the capture sidecar, not the README:\n" + "\n".join(hits)


def test_readme_paragraphs_stay_under_the_cap():
    long = [(p[:60], n) for p, n in paragraphs() if n > PARAGRAPH_CAP]
    assert not long, f"paragraphs over {PARAGRAPH_CAP} words: {long}"
    over_60 = [(p[:60], n) for p, n in paragraphs() if n > 60]
    assert len(over_60) <= PARAGRAPHS_OVER_60, f"paragraphs over 60 words: {over_60}"


def sentences():
    """The sentences of the README's paragraphs, split after '.', '!' or '?' and a space."""
    return [s for p, _ in paragraphs() for s in re.split(r"(?<=[.!?])\s+", p)]


@pytest.mark.parametrize("marker", [
    "until an administrator verifies it under Agents",  # the pending state and what clears it
    "needs strict mode",                                 # the setting the refusal depends on
])
def test_each_kept_quick_start_sentence_is_short(marker):
    found = [s for s in sentences() if marker in s]
    assert len(found) == 1, f"the Quick start must say once, in one sentence, {marker!r}: {found}"
    words = len(found[0].split())
    assert words <= KEPT_SENTENCE_CAP, (
        f"{words} words, over {KEPT_SENTENCE_CAP}: {found[0]!r}; the rest belongs in sdk/python/README.md"
    )


AGENT_SERVICE = REPO_ROOT / "apps" / "backend" / "internal" / "application" / "agent_service.go"
PENDING_MONITORING_ALERT = "Unverified Agent Action (Monitoring Mode)"
PENDING_STRICT_REFUSAL = "Agent not verified - all actions denied"


def test_the_pending_paragraph_says_what_a_pending_agents_call_does_in_each_mode():
    """A reader who calls the agent before it is verified meets one of two states: on a
    new organization, in monitoring mode, the call runs and raises an alert; on a
    strict-mode organization it is refused. The paragraph names each in the words the
    server uses."""
    server = AGENT_SERVICE.read_text(encoding="utf-8")
    assert f'"{PENDING_MONITORING_ALERT}: %s"' in server, "the server no longer titles the alert this way"
    assert f'"{PENDING_STRICT_REFUSAL}"' in server, "the server no longer refuses with this reason"

    found = [p for p, _ in paragraphs() if p.startswith("The agent starts `pending`")]
    assert len(found) == 1, f"the Quick start must say once that the agent starts pending: {found}"
    said = re.split(r"(?<=[.!?])\s+", found[0])
    monitoring = [s for s in said if f"`{PENDING_MONITORING_ALERT}`" in s]
    assert len(monitoring) == 1, f"the paragraph must name the monitoring-mode alert: {found[0]!r}"
    assert "monitoring mode" in monitoring[0] and "runs" in monitoring[0], (
        f"the sentence naming the alert must say the call runs in monitoring mode: {monitoring[0]!r}"
    )
    strict = [s for s in said if f"`{PENDING_STRICT_REFUSAL}`" in s]
    assert len(strict) == 1, f"the paragraph must quote the strict-mode refusal: {found[0]!r}"
    assert "strict-mode organization" in strict[0], (
        f"the sentence quoting the refusal must say it needs a strict-mode organization: {strict[0]!r}"
    )


QUICKSTART_SCRIPT = REPO_ROOT / "scripts" / "quickstart.sh"


def test_the_install_section_says_what_the_quickstart_script_needs_and_writes():
    """The script stops with exit 1 when a command its pre-flight loop checks is missing or
    the Docker daemon is not running, and on a first run writes its compose file and a
    .env of generated secrets, the login among them, to its install directory. A reader
    meets each before the dashboard, so the Install section says so in one sentence."""
    script = QUICKSTART_SCRIPT.read_text(encoding="utf-8")
    required = re.search(r"^for cmd in ([\w ]+); do\n\s+command -v ", script, re.MULTILINE)
    assert required, "scripts/quickstart.sh no longer checks its required commands in one loop"
    assert re.search(r'^docker info .*\|\| error "Docker daemon is not running"', script, re.MULTILINE), (
        "scripts/quickstart.sh no longer stops when the Docker daemon is not running"
    )
    install_dir = re.search(r'^INSTALL_DIR="\$\{AIM_DIR:-([\w.-]+)\}"', script, re.MULTILINE)
    assert install_dir, "scripts/quickstart.sh no longer names its default install directory"
    assert '-o docker-compose.quickstart.yml' in script and "> .env)" in script, (
        "scripts/quickstart.sh no longer writes the compose file and .env this test describes"
    )

    lines = text().split("\n")
    start = lines.index("## Install AIM (self-hosted)")
    end = next(i for i, line in enumerate(lines) if i > start and line.startswith("## "))
    section = " ".join(lines[start:end])
    said = [s for s in sentences() if f"`./{install_dir.group(1)}`" in s and s in section]
    assert len(said) == 1, (
        f"the Install section must say once, in one sentence, what the script writes to "
        f"`./{install_dir.group(1)}`: {said}"
    )
    missing = [cmd for cmd in required.group(1).split() if cmd.lower() not in said[0].lower()]
    assert not missing, f"the sentence must name each command the script needs, missing {missing}: {said[0]!r}"
    assert "Docker running" in said[0], f"the sentence must say Docker has to be running: {said[0]!r}"
    assert "compose file" in said[0] and "`.env`" in said[0], (
        f"the sentence must name the compose file and the `.env` the script writes: {said[0]!r}"
    )


def test_capture_provenance_lives_in_the_sidecar_the_readme_links():
    assert "](docs/captures/quick-start.txt)" in text(), "the README links the full captured output"
    sidecar = json.loads(SIDECAR.read_text(encoding="utf-8"))
    assert sidecar["text"] == "docs/captures/quick-start.txt"
    assert (REPO_ROOT / sidecar["text"]).is_file()
    for capture in sidecar["captures"]:
        assert re.fullmatch(r"20\d\d-\d\d-\d\d", capture["capturedAt"]), capture["command"]
        assert capture["sdk"]["version"], capture["command"]
        assert capture["outputs"]["capture"].strip(), capture["command"]


@pytest.mark.parametrize("heading, destination", [
    ("See it work", "docs/quick-start.md"),
    ("Notes on the root README's Quick start", "docs/quick-start.md"),
    ("Python", "sdk/python/README.md"),
    ("Java", "sdk/java/README.md"),
    ("TypeScript", "sdk/typescript/README.md"),
    ("5-step Fine-Grained Authorization", "docs/server-features.md"),
    ("Trust-gated capabilities", "docs/server-features.md"),
    ("MCP attestation", "docs/server-features.md"),
    ("Privileged Access Management", "docs/server-features.md"),
    ("CyberArk integration", "docs/server-features.md"),
    ("SIEM adapters", "docs/server-features.md"),
    ("Web dashboard", "docs/server-features.md"),
    ("Docker", "docs/DEPLOYMENT.md"),
    ("From source", "docs/DEPLOYMENT.md"),
    ("Verifying the `@opena2a/aim-core` package", "docs/verifying-packages.md"),
    ("Trust scoring", "docs/server-features.md"),
    ("Observability", "docs/DEPLOYMENT.md"),
    ("Demos", "docs/DOCUMENTATION_INDEX.md"),
    ("Use cases", "docs/USE-CASES.md"),
    ("Contributing", "CONTRIBUTING.md"),
])
def test_each_moved_passage_is_at_its_destination(heading, destination):
    body = (REPO_ROOT / destination).read_text(encoding="utf-8")
    assert heading in body, f"{destination} does not carry {heading!r}"


def test_readme_links_resolve_inside_the_repository():
    missing = []
    for target in sorted(set(re.findall(r"\]\(([^)#\s]+)(?:#[^)\s]*)?\)", text()))):
        if re.match(r"^(https?|mailto):", target):
            continue
        if not (REPO_ROOT / target).exists():
            missing.append(target)
    assert not missing, f"README links to paths the repository does not have: {missing}"
    anchors = heading_anchors(README)
    dead = [a for a in set(re.findall(r"\]\(#([^)]+)\)", text())) if a not in anchors]
    assert not dead, f"README links to headings it does not have: {dead}"


def test_readme_links_to_other_files_land_on_their_headings():
    """A link such as docs/quick-start.md#see-it-work names a heading in that file; when
    the heading there is renamed, the README's link lands at the top of the page."""
    dead = [
        f"{target}#{anchor}"
        for target, anchor in sorted(set(re.findall(r"\]\(([^)#\s:]+\.md)#([^)\s]+)\)", text())))
        if anchor not in heading_anchors(REPO_ROOT / target)
    ]
    assert not dead, f"README links to headings those files do not have: {dead}"


# The README as GitHub serves it, by absolute URL, at the repository page or at the file.
README_URL_ANCHOR = re.compile(
    r"github\.com/opena2a-org/agent-identity-management(?:/blob/[^/\s#)]+/README\.md)?/?#([\w-]+)"
)
# GitHub gives the README on a repository page the anchor #readme; it is no heading.
REPOSITORY_PAGE_ANCHORS = {"readme"}
# Headings linked from outside this repository, where no test here can see the link:
# copies of sdk/python/README.md elsewhere keep linking the self-hosted install after
# the file here changes, and the AIM page on opena2a.org links #deployment as the
# deployment documentation's setup instructions. Keep the heading, or repoint every
# copy in the same change.
LINKED_FROM_OUTSIDE = ["install-aim-self-hosted", "deployment"]


def links_to_readme_headings():
    """(file, anchor) for each link in a tracked file to a heading of the root README: a
    relative link from markdown, or the README's URL on GitHub from any text file."""
    tracked = subprocess.run(
        ["git", "ls-files", "-z"], cwd=REPO_ROOT, capture_output=True, check=True,
    ).stdout.decode().split("\0")
    links = []
    for name in filter(None, tracked):
        path = REPO_ROOT / name
        if name == "README.md" or not path.is_file():
            continue
        try:
            body = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue
        links += [(name, anchor) for anchor in README_URL_ANCHOR.findall(body)]
        if name.endswith(".md"):
            links += [
                (name, anchor)
                for target, anchor in re.findall(r"\]\(([^)#\s:]*README\.md)#([^)\s]+)\)", body)
                if (path.parent / target).resolve() == README.resolve()
            ]
    return links


def test_links_to_readme_headings_land_on_a_heading_it_has():
    """The short form moved most of the long README's sections to docs/. A heading the
    README removes or renames breaks every link to it, in this repository and outside it."""
    links = links_to_readme_headings()
    for found in [("sdk/python/README.md", "install-aim-self-hosted"),   # by URL
                  ("sdk/java/README.md", "install-aim-self-hosted")]:    # by relative path
        assert found in links, f"the scan no longer finds {found[0]}'s link to #{found[1]}"
    anchors = heading_anchors(README)
    dead = sorted({f"{name} -> #{anchor}" for name, anchor in links
                   if anchor not in anchors and anchor not in REPOSITORY_PAGE_ANCHORS})
    dead += [f"outside this repository -> #{anchor}" for anchor in LINKED_FROM_OUTSIDE if anchor not in anchors]
    assert not dead, "links to README headings it does not have:\n" + "\n".join(dead)


def test_the_deployment_section_links_where_each_mode_is_set_up():
    """The AIM page on opena2a.org sends a reader to #deployment for setup instructions;
    each mode in the section's table links the place that mode is set up."""
    lines = text().split("\n")
    start = lines.index("## Deployment")
    end = next(i for i, line in enumerate(lines) if i > start and line.startswith("## "))
    rows = {re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", row.split("|")[1]).strip(" *"): row
            for row in lines[start:end] if row.startswith("| ")}
    for mode, setup in [("AIM Cloud", "https://aim.opena2a.org/get-started"),
                        ("Self-hosted", "#install-aim-self-hosted"),
                        ("Local-only", "sdk/typescript/README.md#local-mode-with-opena2aaim-core")]:
        assert mode in rows, f"the Deployment table has no {mode!r} row: {list(rows)}"
        assert f"]({setup})" in rows[mode], f"the {mode!r} row does not link {setup}: {rows[mode]}"


def test_every_job_in_the_list_links_where_it_is_done():
    lines = text().split("\n")
    start = lines.index("## What you can do")
    end = next(i for i, line in enumerate(lines) if i > start and line.startswith("## "))
    jobs = [line for line in lines[start:end] if line.startswith("- ")]
    assert len(jobs) >= 5, "the job list names the jobs AIM does"
    without_link = [job for job in jobs if not re.search(r"\]\([^)]+\)", job)]
    assert not without_link, f"job-list entries with nowhere to go: {without_link}"


UNBACKED_CLAIMS = [
    # The server registers no route under /v1/, so no local history reaches it this way.
    "enableReporting",
    "local-to-server reporting",
    "Both can run for the same agent",
    "share the same audit-event schema",
    # Nothing writes the TME evaluations the trust calculator reads, so the factor is never modulated.
    "NanoMind-modulated",
]


def test_the_deployment_section_says_local_only_keeps_its_own_event_format():
    """Local-only mode writes its own audit file; the server's audit log is a different
    store, and nothing in the TypeScript SDK pushes one into the other."""
    lines = text().split("\n")
    start = lines.index("## Deployment")
    end = next(i for i, line in enumerate(lines) if i > start and line.startswith("## "))
    section = "\n".join(lines[start:end])
    assert ("Local only mode keeps its own event format in `~/.opena2a/aim-core/audit.jsonl`, "
            "separate from the server's audit log.") in section


@pytest.mark.parametrize("path", ["README.md", "docs/server-features.md"])
def test_the_readme_and_the_server_features_guide_claim_no_integration_that_does_not_exist(path):
    body = (REPO_ROOT / path).read_text(encoding="utf-8")
    found = [claim for claim in UNBACKED_CLAIMS if claim in body]
    assert not found, f"{path} still claims: {found}"


def test_the_security_alerts_trust_factor_row_says_what_feeds_it():
    """The server's security-alerts factor is scored from the agent's unacknowledged alerts."""
    rows = [line for line in (REPO_ROOT / "docs" / "server-features.md").read_text(encoding="utf-8").split("\n")
            if line.startswith("| Security alerts |")]
    assert rows == ["| Security alerts | 15% | Observed |"]


AIM_CORE_GUIDES = [
    "docs/use-cases/audit-agent-actions.md",
    "docs/use-cases/embed-in-my-app.md",
    "docs/use-cases/fleet-governance.md",
]

# `@opena2a/aim-core` keeps identities, policies and events on the machine it runs on. Its one way
# out, the reporter its `serverUrl` option starts, posts to /v1/audit/events, and the server
# registers no route under /v1/.
AIM_CORE_SERVER_CLAIMS = [
    "serverUrl",
    "fleet reporting",
    "registered on the server",
    "sent to the central server",
    "server-side with history",
]


@pytest.mark.parametrize("path", AIM_CORE_GUIDES)
def test_no_use_case_guide_says_aim_core_reaches_an_aim_server(path):
    body = (REPO_ROOT / path).read_text(encoding="utf-8")
    found = [claim for claim in AIM_CORE_SERVER_CLAIMS if claim in body]
    assert not found, f"{path} still says @opena2a/aim-core reaches an AIM server: {found}"


def test_the_sdk_table_gives_aim_core_the_one_mode_it_has():
    rows = [row for row in text().split("\n") if row.startswith("| TypeScript |")]
    assert len(rows) == 1, f"the SDK table has {len(rows)} TypeScript rows"
    cells = [cell.strip() for cell in rows[0].split("|")[1:-1]]
    assert cells[1] == "`npm install @opena2a/aim-core`", rows[0]
    assert cells[2] == "Local", f"the @opena2a/aim-core row names a mode it does not have: {cells[2]!r}"
