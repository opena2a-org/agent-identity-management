"""
The root ROADMAP.md is a public status page: what AIM provides today, each
item linked to the release that carries it, and what may come next, with no
dates. These tests hold its shape and wording so an edit cannot quietly turn
it back into a list of promises.

They check the fixed header and closing section byte for byte (the date on
line 3 excepted), the heading outline, that every byte is printable ASCII,
where digits may appear, the form of each Shipped line's release link, the
wording rules for open work, and the lines in README.md, HARDENING.md,
docs/DOCUMENTATION_INDEX.md, docs/sdk/trust-scoring.md and
apps/backend/docs/OBSERVABILITY.md that point at the roadmap or used to.

Whether each Shipped line is true, and whether its release page and month
match what the registries answer, is checked by hand on the edit date; these
tests run offline and read only files in this repository.
"""

import datetime
import re
from pathlib import Path

SDK_DIR = Path(__file__).resolve().parent.parent          # .../sdk/python
REPO_ROOT = SDK_DIR.parent.parent                         # repo root
ROADMAP = REPO_ROOT / "ROADMAP.md"

REPO_URL = "https://github.com/opena2a-org/agent-identity-management"

HEADER = [
    "# AIM roadmap",
    "",
    None,  # "Last updated: YYYY-MM-DD", the date of the commit that changes the file
    "",
    "This file lists what AIM provides today and what may come next. Only Shipped "
    "lines carry versions and dates, each taken from the linked release or image. "
    "Requests and bug reports go through GitHub issues.",
    "",
    "- **Shipped:** available now in the release or image that the line links.",
    "- **In progress:** a linked pull request is open, or merged but not yet released.",
    "- **Planned:** intended work with no open pull request; scope and order may change.",
    "- **Under consideration:** not scheduled; listed because earlier versions of "
    "this file named it.",
    "",
    f"[Releases]({REPO_URL}/releases) | [Changelog](CHANGELOG.md) | "
    f"[Issues]({REPO_URL}/issues) | [Security policy](SECURITY.md)",
]

SECTIONS = [
    "Shipped",
    "In progress",
    "Planned",
    "Under consideration",
    "How to follow and request changes",
]
CLASS_SECTIONS = SECTIONS[:4]

FOLLOW = [
    f"- Watch [releases]({REPO_URL}/releases); each release page lists the changes "
    "in that version. The [changelog](CHANGELOG.md) also records platform changes "
    "that are merged but not yet released.",
    f"- Open an [issue]({REPO_URL}/issues) to request a capability or report a bug.",
    "- Report a vulnerability as described in [SECURITY.md](SECURITY.md), not in a "
    "public issue.",
]

LEADS = ["SDKs", "Registering agents", "Dashboard", "API", "Deployment", "Security"]
ITEM = re.compile(r"^- \*\*(?P<lead>[^*]+):\*\* (?P<body>.+)$")

MONTHS = (
    "January|February|March|April|May|June|July|August|September|October|"
    "November|December"
)
VERSION = r"\d+\.\d+\.\d+"
URL = re.escape(REPO_URL)
ANCHORS = [
    re.compile(
        rf"\[AIM platform (?P<v>{VERSION})\]\({URL}/releases/tag/platform-v(?P<t>{VERSION})\), "
        rf"(?:{MONTHS}) \d{{4}}$"
    ),
    re.compile(
        rf"\[aim-sdk (?P<v>{VERSION}) for Python\]\({URL}/releases/tag/sdk-py-v(?P<t>{VERSION})\), "
        rf"on PyPI, (?:{MONTHS}) \d{{4}}$"
    ),
    re.compile(
        rf"\[@opena2a/aim-sdk (?P<v>{VERSION}) for TypeScript\]\({URL}/releases/tag/sdk-ts-v(?P<t>{VERSION})\), "
        rf"on npm, (?:{MONTHS}) \d{{4}}$"
    ),
    re.compile(
        r"\[aim-server image sha256:(?P<v>[0-9a-f]{12})\]"
        r"\((?P<t>https://github\.com/[^)\s]*/aim-server[^)\s]*)\)$"
    ),
]
TRAILING_PR = re.compile(r" \(#\d+\)$")

BANNED_WORDS = [
    "product", "seamless", "powerful", "robust", "enterprise-grade", "best-in-class",
    "cutting-edge", "next-generation", "revolutionary", "world-class", "effortless",
    "simply", "easily",
]
FIRST_PERSON_PLURAL = ["we", "our", "ours", "us", "ourselves"]


def _lines():
    return ROADMAP.read_text(encoding="utf-8").split("\n")


def _sections():
    """Map each H2 title to the non-empty lines under it."""
    sections = {}
    current = None
    for line in _lines():
        if line.startswith("## "):
            current = line[3:]
            sections[current] = []
        elif current is not None and line.strip():
            sections[current].append(line)
    return sections


def _anchor(bullet):
    for pattern in ANCHORS:
        match = pattern.search(bullet)
        if match:
            return match
    return None


def _without_link_targets(text):
    return re.sub(r"\]\([^)]*\)", "]", text)


def test_header_is_the_fixed_text_with_a_real_date():
    lines = _lines()
    assert len(lines) > len(HEADER)
    for number, (expected, actual) in enumerate(zip(HEADER, lines), start=1):
        if expected is None:
            match = re.fullmatch(r"Last updated: (\d{4}-\d{2}-\d{2})", actual)
            assert match, f"line {number} is not 'Last updated: YYYY-MM-DD': {actual!r}"
            datetime.date.fromisoformat(match.group(1))
        else:
            assert actual == expected, f"line {number} differs from the fixed header"


def test_one_title_five_sections_in_order_and_nothing_deeper():
    lines = _lines()
    assert [l for l in lines if l.startswith("# ")] == ["# AIM roadmap"]
    assert [l[3:] for l in lines if l.startswith("## ")] == SECTIONS
    deeper = [l for l in lines if l.startswith("###")]
    assert deeper == [], f"headings below H2: {deeper}"


def test_every_byte_is_printable_ascii_or_a_newline():
    data = ROADMAP.read_bytes()
    bad = [
        (offset, byte) for offset, byte in enumerate(data)
        if byte != 0x0A and not 0x20 <= byte <= 0x7E
    ]
    assert bad == [], f"bytes outside 0x20-0x7E and newline at offsets {bad[:10]}"


def test_digits_only_in_the_date_release_links_and_pull_request_numbers():
    sections = _sections()
    shipped = set(sections["Shipped"])
    open_work = set(sections["In progress"]) | set(sections["Planned"])
    offenders = []
    for number, line in enumerate(_lines(), start=1):
        if number == 3:
            rest = re.sub(r"\d{4}-\d{2}-\d{2}$", "", line)
        elif line in shipped:
            match = _anchor(line)
            rest = line[: match.start()] if match else line
        elif line in open_work:
            rest = TRAILING_PR.sub("", line)
        else:
            rest = line
        if re.search(r"\d", _without_link_targets(rest)):
            offenders.append(f"{number}: {line}")
    assert offenders == [], "digits outside the date, release links and (#N):\n" + "\n".join(offenders)


def test_class_sections_hold_only_lead_word_items():
    sections = _sections()
    for title in CLASS_SECTIONS:
        assert sections[title], f"section {title!r} is empty"
        for line in sections[title]:
            match = ITEM.match(line)
            assert match, f"not a '- **Lead:** text' item under {title!r}: {line}"
            assert match.group("lead") in LEADS, f"lead word {match.group('lead')!r} is not one of {LEADS}"


def test_every_shipped_line_ends_with_one_release_link():
    for line in _sections()["Shipped"]:
        assert line.count("](") == 1, f"a Shipped line carries one link, its release: {line}"
        match = _anchor(line)
        assert match, f"Shipped line does not end with a release link in a known form: {line}"
        if "aim-server image" not in match.group(0):
            assert match.group("v") == match.group("t"), f"link text and tag differ: {line}"


def test_open_work_starts_with_the_work_and_promises_nothing():
    sections = _sections()
    for title in CLASS_SECTIONS:
        for line in sections[title]:
            first = ITEM.match(line).group("body").split()[0]
            if title == "Shipped":
                assert not first.endswith("ing"), f"a Shipped line states what exists: {line}"
            else:
                assert first.endswith("ing"), f"open work starts with an -ing word: {line}"
                assert not re.search(r"\bwill\b", line, re.IGNORECASE), f"'will' in open work: {line}"


def test_wording():
    text = ROADMAP.read_text(encoding="utf-8")
    assert "!" not in text
    assert "```" not in text and "~~~" not in text
    for word in FIRST_PERSON_PLURAL + BANNED_WORDS:
        assert not re.search(rf"\b{re.escape(word)}\b", text, re.IGNORECASE), f"{word!r} in ROADMAP.md"


def test_how_to_follow_is_the_fixed_text_and_ends_with_the_reporting_path():
    assert _sections()["How to follow and request changes"] == FOLLOW


def test_entry_points_read_as_written():
    readme = (REPO_ROOT / "README.md").read_text(encoding="utf-8").split("\n")
    links = readme[readme.index("## Links"):]
    links = links[: next((i for i, l in enumerate(links[1:], 1) if l.startswith("## ")), len(links))]
    assert "- [Roadmap](ROADMAP.md)" in links
    assert any(l.endswith("Local mode is in the TypeScript SDK only. |") for l in readme)

    hardening = (REPO_ROOT / "HARDENING.md").read_text(encoding="utf-8").split("\n")
    assert hardening[2].endswith(
        " This page records the hardening work behind 1.0; open feature work is listed "
        "in [ROADMAP.md](ROADMAP.md)."
    )

    index = (REPO_ROOT / "docs" / "DOCUMENTATION_INDEX.md").read_text(encoding="utf-8")
    assert "\n15. `/ROADMAP.md` - What has shipped and what may come next\n" in index

    observability = (REPO_ROOT / "apps" / "backend" / "docs" / "OBSERVABILITY.md").read_text(encoding="utf-8")
    assert "Producer is expected to write this from a real scanner; no shipped integration performs that write yet." in observability

    scoring = (REPO_ROOT / "docs" / "sdk" / "trust-scoring.md").read_text(encoding="utf-8")
    assert "and will be until an independent verification source ships. " in scoring
