"""User-facing output carries no emoji-class characters (#391, AIM-15).

Enumerates the shipped package from disk rather than checking known sites, so a
NEW emoji added to a NEW file fails this test. Checking only the files that were
once wrong is how the class comes back.

AIM-15 retired the U+2713 CHECK MARK / U+2717 BALLOT X "house style" this
module used to exempt from the package-source walk (and assert PRESENT):
console states are now rendered with the plain-text markers [OK] and [FAIL],
coloured through rich markup where rich is available. The package-source walk
below therefore exempts NOTHING in the measured ranges. The prose surfaces
(README.md, CHANGELOG.md, docs/*.md) still carry the old marks and are outside
AIM-15's scope, so the prose walk tolerates exactly those marks -- and nothing
else.

Every banned or tolerated character is written as an escape, never as a literal
glyph. A bare U+FE0F is an INVISIBLE character, and a file asserting "no
invisible characters here" that contains one is indistinguishable from the
GlassWorm attack it guards against -- our own scanner flagged an earlier draft
of this file as CRITICAL for exactly that. Escapes also keep the file greppable
and reviewable in a plain diff.
"""

import pathlib
import shutil
import unicodedata

import pytest

PKG = pathlib.Path(__file__).resolve().parent.parent / "aim_sdk"

# In scope. Named, not derived from a property lookup, so the test states its
# own contract rather than tracking whatever the local unicodedata says today.
BANNED = {
    "\u23F3": "HOURGLASS WITH FLOWING SAND",
    "\u26A0": "WARNING SIGN",
    "\u2139": "INFORMATION SOURCE",
    "\uFE0F": "VARIATION SELECTOR-16 (forces emoji presentation)",
}

# Tolerated in PROSE ONLY. These were the package's pre-AIM-15 console marks;
# the docs that describe old output still show them, and sweeping prose is a
# separate unit. The package-source walk grants no such tolerance.
PROSE_TOLERATED = {
    "\u2713": "CHECK MARK",
    "\u2717": "BALLOT X",
}

# The plain-text vocabulary that replaced the marks above in console output.
REPLACEMENT_MARKERS = ("[OK]", "[FAIL]")


def _python_files(root=None):
    root = root or PKG
    files = sorted(root.rglob("*.py"))
    assert files, f"found no source under {root} -- the scan is blind"
    return files


def _prose_files():
    """The prose a reader actually meets.

    CHANGELOG.md and README.md are in MANIFEST.in and therefore ship inside the
    distribution; docs/*.md do not ship but are public on GitHub, and the
    standard is about what a human reads, not about the packaging manifest.
    """
    root = PKG.parent
    files = [p for p in (root / "CHANGELOG.md", root / "README.md") if p.exists()]
    files += sorted((root / "docs").rglob("*.md"))
    return files


def _is_emoji(ch: str) -> bool:
    """Property-based, so a pictograph nobody enumerated is still caught.

    An earlier fix here used a hand-written list of emoji to strip, and the list
    was the defect: it removed the ones someone had thought of and left ten
    others (a bug, a plug, a handshake) sitting in the same documents. A rule
    that needs a rule per spelling is not a rule.

    No exemptions: U+2713 and U+2717 are in range and are reported like any
    other hit. (U+25CB WHITE CIRCLE sits outside every measured range, so the
    exemption it used to enjoy here was always a no-op.)
    """
    o = ord(ch)
    return (
        0x1F000 <= o <= 0x1FAFF
        or 0x2600 <= o <= 0x27BF
        or 0x2B00 <= o <= 0x2BFF
        or o in (0xFE0F, 0x2139, 0x23F3, 0x23F8)
    )


def _scan_tree(root):
    """Walk every .py file under ``root``; report each offending file, line
    and code point. Blind-scan-proof: refuses to pass on an empty walk."""
    offenders = []
    for f in _python_files(root):
        for lineno, line in enumerate(f.read_text(encoding="utf-8").splitlines(), 1):
            for ch in {c for c in line if _is_emoji(c)}:
                offenders.append(
                    f"{f.relative_to(root.parent)}:{lineno}: U+{ord(ch):04X} "
                    f"{unicodedata.name(ch, '?')}"
                )
    return offenders


def test_AIM_15_AC1_package_source_scan_returns_zero_lines():
    """No character in the measured ranges anywhere under aim_sdk/**/*.py."""
    offenders = _scan_tree(PKG)
    assert not offenders, "emoji-class characters in package source:\n" + "\n".join(
        offenders[:60]
    )


def test_AIM_15_AC2_walk_is_unexempted_and_non_vacuous():
    """The walk sees the real tree and exempts nothing the ranges cover."""
    assert _is_emoji("\u2713"), "U+2713 CHECK MARK must not be exempted"
    assert _is_emoji("\u2717"), "U+2717 BALLOT X must not be exempted"
    files = _python_files()
    names = {f.name for f in files}
    assert "console.py" in names and "client.py" in names, (
        "the walk did not reach the package's known modules -- looking at the "
        f"wrong tree? saw {len(files)} files under {PKG}"
    )


def test_AIM_15_AC3_scan_fails_on_a_planted_check_mark(tmp_path):
    """Proves the detection works, independent of the package's current state.

    Without this, a scan that silently matched nothing would report the same
    clean result as a scan that read every file and found nothing. The planted
    character is U+2713 deliberately: the character the pre-AIM-15 predicate
    exempted, so this also fails if the exemption ever creeps back.
    """
    scratch = tmp_path / "aim_sdk"
    shutil.copytree(PKG, scratch, ignore=shutil.ignore_patterns("__pycache__"))
    planted = scratch / "planted.py"
    # Single-escaped on purpose: this source file stays pure ASCII, while the
    # bytes written to the fixture are the real character the scan must catch.
    planted.write_text('print("\u2713 done")\n', encoding="utf-8")

    with pytest.raises(AssertionError) as excinfo:
        offenders = _scan_tree(scratch)
        assert not offenders, "emoji-class characters:\n" + "\n".join(offenders)

    message = str(excinfo.value)
    assert "planted.py" in message, "the failure must name the offending file"
    assert "U+2713" in message, "the failure must name the offending code point"


@pytest.mark.parametrize("marker", REPLACEMENT_MARKERS)
def test_AIM_15_AC4_replacement_markers_are_still_used(marker):
    """Non-vacuity control, and a guard against over-correction.

    If this fails, someone removed the plain-text vocabulary instead of the
    emoji -- which would make the scans above pass for the wrong reason.
    Replaces the retired assertion that U+2713/U+2717/U+25CB were present.
    """
    total = sum(f.read_text(encoding="utf-8").count(marker) for f in _python_files())
    assert total > 0, f"{marker} vanished from the package"


def test_AIM_15_AC5_changelog_documents_the_marker_replacement():
    """CHANGELOG.md carries the change in a section newer than 2.0.2."""
    changelog = (PKG.parent / "CHANGELOG.md").read_text(encoding="utf-8")
    headings = [l for l in changelog.splitlines() if l.startswith("## ")]
    assert headings and "2.0.2" not in headings[0], (
        "no CHANGELOG section newer than 2.0.2 -- the marker replacement is "
        "undocumented"
    )
    newest = changelog.split(headings[0], 1)[1].split("## [2.0.2]", 1)[0]
    assert "[OK]" in newest and "U+2713" in newest, (
        "the newest CHANGELOG section does not name the console-marker "
        "replacement"
    )


def test_no_banned_emoji_class_characters_anywhere_in_the_package():
    offenders = []
    for f in _python_files():
        for lineno, line in enumerate(f.read_text(encoding="utf-8").splitlines(), 1):
            for ch in set(line) & set(BANNED):
                offenders.append(
                    f"{f.relative_to(PKG.parent)}:{lineno}: U+{ord(ch):04X} {BANNED[ch]}"
                )
    assert not offenders, "emoji-class characters in shipped output:\n" + "\n".join(
        offenders
    )


def test_no_emoji_in_prose_beyond_the_tolerated_marks():
    """The whole class over the prose surfaces, minus exactly the pre-AIM-15
    marks the docs still legitimately show. Package source is covered -- with
    no tolerance at all -- by the AC1 walk above."""
    offenders = []
    for f in _prose_files():
        for lineno, line in enumerate(f.read_text(encoding="utf-8").splitlines(), 1):
            for ch in {c for c in line if _is_emoji(c) and c not in PROSE_TOLERATED}:
                offenders.append(
                    f"{f.relative_to(PKG.parent)}:{lineno}: U+{ord(ch):04X} "
                    f"{unicodedata.name(ch, '?')}"
                )
    assert not offenders, "emoji in prose:\n" + "\n".join(offenders[:40])


def test_the_scanner_can_actually_see_a_planted_banned_offender(tmp_path):
    """The BANNED-list twin of the AC3 planted-fault test."""
    planted = tmp_path / "planted.py"
    # Single-escaped on purpose, as in the AC3 test above. Double-escaping here
    # would write the literal text backslash-u-26A0, the scan would find
    # nothing, and the test would report the detector broken when it is not.
    planted.write_text('print("\u26A0\uFE0F  something")\n', encoding="utf-8")
    found = [ch for ch in set(planted.read_text(encoding="utf-8")) if ch in BANNED]
    assert found, "the scan cannot see a known-bad character"
    # U+FE0F is unnamed in the UCD, so name() raises rather than returning "".
    assert all(unicodedata.name(ch, "") is not None for ch in found)
    assert _is_emoji("\u26A0"), "the property-based check disagrees with BANNED"
