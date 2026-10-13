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


# Sentences the short form keeps from the long form, so a reader meets no state the
# README does not name: what pending means, the setting the refusal needs, where
# secure() writes, how the release on PyPI prints the captures, where the self-hosted
# login comes from, and that the install is unpinned.
KEPT_SENTENCES = [
    "until an administrator verifies it under Agents",
    "needs strict mode",
    "stores credentials at `~/.aim/`",
    "the release on PyPI prints",
    "Login credentials print at the end of the run",
    "nothing on this path is pinned or verified",
]

# Text that other tests read from this README: the token-rotation line and its Redis
# condition, the policy a quickstart-default stack names, and when the trust score is
# recalculated. The last marker is held as a whole paragraph.
PINNED_SENTENCES = [
    "Token rotated successfully",
    "`default_policy`",
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
# the file here changes. Keep the heading, or repoint every copy in the same change.
LINKED_FROM_OUTSIDE = ["install-aim-self-hosted"]


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


def test_every_job_in_the_list_links_where_it_is_done():
    lines = text().split("\n")
    start = lines.index("## What you can do")
    end = next(i for i, line in enumerate(lines) if i > start and line.startswith("## "))
    jobs = [line for line in lines[start:end] if line.startswith("- ")]
    assert len(jobs) >= 5, "the job list names the jobs AIM does"
    without_link = [job for job in jobs if not re.search(r"\]\([^)]+\)", job)]
    assert not without_link, f"job-list entries with nowhere to go: {without_link}"
