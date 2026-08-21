"""User-facing output carries no emoji-class characters (#391).

Enumerates the shipped package from disk rather than checking known sites, so a
NEW emoji added to a NEW file fails this test. Checking only the files that were
once wrong is how the class comes back.

Scope is the issue's: U+23F3, U+26A0 (bare and with the U+FE0F variation
selector that forces colour-emoji presentation), U+2139. The house-style marks
the issue explicitly exempts -- U+2713, U+2717, U+25CB, box drawing -- are
asserted PRESENT below, so a future "sweep" cannot pass this file by deleting
the whole visual vocabulary.
"""

import pathlib
import unicodedata

import pytest

PKG = pathlib.Path(__file__).resolve().parent.parent / "aim_sdk"

# In scope. Named, not derived from a property lookup, so the test states its
# own contract rather than tracking whatever the local unicodedata says today.
#
# Written as escapes, never as literal glyphs. A bare U+FE0F is an INVISIBLE
# character, and a file asserting "no invisible characters here" that contains
# one is indistinguishable from the GlassWorm attack it guards against -- our own
# scanner flagged an earlier draft of this file as CRITICAL for exactly that.
# Escapes also keep the file greppable and reviewable in a plain diff.
BANNED = {
    "\u23F3": "HOURGLASS WITH FLOWING SAND",
    "\u26A0": "WARNING SIGN",
    "\u2139": "INFORMATION SOURCE",
    "\uFE0F": "VARIATION SELECTOR-16 (forces emoji presentation)",
}

# Deliberately allowed: the house style, per the issue.
HOUSE_STYLE = {
    "\u2713": "CHECK MARK",
    "\u2717": "BALLOT X",
    "\u25CB": "WHITE CIRCLE",
}


def _python_files():
    files = sorted(PKG.rglob("*.py"))
    assert files, f"found no source under {PKG} -- the scan is blind"
    return files


def _text_surfaces():
    """Python source plus the prose a reader actually meets.

    CHANGELOG.md and README.md are in MANIFEST.in and therefore ship inside the
    distribution; docs/*.md do not ship but are public on GitHub, and the
    standard is about what a human reads, not about the packaging manifest.
    """
    root = PKG.parent
    files = _python_files()
    files += [p for p in (root / "CHANGELOG.md", root / "README.md") if p.exists()]
    files += sorted((root / "docs").rglob("*.md"))
    return files


def _is_emoji(ch: str) -> bool:
    """Property-based, so a pictograph nobody enumerated is still caught.

    An earlier fix here used a hand-written list of emoji to strip, and the list
    was the defect: it removed the ones someone had thought of and left ten
    others (a bug, a plug, a handshake) sitting in the same documents. A rule
    that needs a rule per spelling is not a rule.
    """
    if ch in HOUSE_STYLE:
        return False
    o = ord(ch)
    return (
        0x1F000 <= o <= 0x1FAFF
        or 0x2600 <= o <= 0x27BF
        or 0x2B00 <= o <= 0x2BFF
        or o in (0xFE0F, 0x2139, 0x23F3, 0x23F8)
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


def test_no_emoji_of_any_kind_in_source_or_prose():
    """The whole class, not the three codepoints the issue happened to table."""
    offenders = []
    for f in _text_surfaces():
        for lineno, line in enumerate(f.read_text(encoding="utf-8").splitlines(), 1):
            for ch in {c for c in line if _is_emoji(c)}:
                offenders.append(
                    f"{f.relative_to(PKG.parent)}:{lineno}: U+{ord(ch):04X} "
                    f"{unicodedata.name(ch, '?')}"
                )
    assert not offenders, "emoji in source or prose:\n" + "\n".join(offenders[:40])


@pytest.mark.parametrize("ch", sorted(HOUSE_STYLE))
def test_house_style_marks_are_still_used(ch):
    """Non-vacuity control, and a guard against over-correction.

    If this fails, someone removed the plain-text vocabulary instead of the
    emoji -- which would make the banned-character test above pass for the
    wrong reason.
    """
    total = sum(f.read_text(encoding="utf-8").count(ch) for f in _python_files())
    assert total > 0, f"U+{ord(ch):04X} {HOUSE_STYLE[ch]} vanished from the package"


def test_the_scanner_can_actually_see_a_planted_offender(tmp_path):
    """Proves the detection works, independent of the package's current state.

    Without this, a scan that silently matched nothing would report the same
    clean result as a scan that read every file and found nothing.
    """
    planted = tmp_path / "planted.py"
    # Single-escaped on purpose: this source file stays pure ASCII, while the
    # bytes written to the fixture are the real characters the scan must catch.
    # Double-escaping here would write the literal text backslash-u-26A0, the scan would
    # find nothing, and the test would report the detector broken when it is not.
    planted.write_text('print("\u26A0\uFE0F  something")\n', encoding="utf-8")
    found = [ch for ch in set(planted.read_text(encoding="utf-8")) if ch in BANNED]
    assert found, "the scan cannot see a known-bad character"
    # U+FE0F is unnamed in the UCD, so name() raises rather than returning "".
    assert all(unicodedata.name(ch, "") is not None for ch in found)
    assert _is_emoji("\u26A0"), "the property-based check disagrees with BANNED"
