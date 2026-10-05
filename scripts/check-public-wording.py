#!/usr/bin/env python3
"""Fail when wording that has been ruled out appears in the public documents.

The documents a reader opens first make statements about audit and evidence. Some wording
overstates what the code does ("tamper proof" for a table that is not append-only at the
database layer), and once such wording is removed nothing stops it from coming back. This
script reads the barred terms and the allowlisted sentences from
`scripts/public-wording/barred.json` and scans:

    README.md  SECURITY.md  HARDENING.md  docs/**/*.md  examples/**/*.md  sdk/**/*.md

    python3 scripts/check-public-wording.py            # scan the documents above
    python3 scripts/check-public-wording.py FILE...    # scan only the named files

Each hit prints as `file:line: term` with the reason the term is barred. A term matches
case-insensitively on word boundaries; a space or hyphen inside a term matches any run of
spaces, hyphens and line breaks, so "tamper proof" also finds "Tamper-proof" and a phrase
wrapped over two lines. A hit inside an allowlisted sentence (an accurate statement of what
the code does not do) is not counted.

Exit status: 0 no hits, 1 at least one hit, 2 the list file or the arguments are unusable.
"""
from __future__ import annotations

import argparse
import json
import os
import re
import sys
from bisect import bisect_right
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
DEFAULT_LIST = SCRIPT_DIR / "public-wording" / "barred.json"
DEFAULT_ROOT = SCRIPT_DIR.parent

ROOT_FILES = ("README.md", "SECURITY.md", "HARDENING.md")
ROOT_DIRS = ("docs", "examples", "sdk")
# Installed dependencies carry their own Markdown; it is not ours to check.
PRUNED_DIRS = {"node_modules", "site-packages", "venv", "__pycache__"}

SEPARATOR = r"[\s\-‐‑]"


class ListError(Exception):
    pass


def load_list(path: Path) -> tuple[list[dict], list[dict]]:
    try:
        data = json.loads(path.read_text(encoding="utf-8"))
    except OSError as err:
        raise ListError(f"cannot read {path}: {err.strerror}") from err
    except json.JSONDecodeError as err:
        raise ListError(f"{path} is not valid JSON: {err}") from err
    if not isinstance(data, dict):
        raise ListError(f"{path}: the top level must be an object")

    def entries(key: str, text_field: str) -> list[dict]:
        items = data.get(key)
        if not isinstance(items, list):
            raise ListError(f"{path}: '{key}' must be a list")
        seen: set[str] = set()
        for index, item in enumerate(items):
            where = f"{path}: {key}[{index}]"
            if not isinstance(item, dict):
                raise ListError(f"{where} must be an object")
            for field in (text_field, "reason"):
                value = item.get(field)
                if not isinstance(value, str) or not value.strip():
                    raise ListError(f"{where} needs a non-empty '{field}'")
            folded = " ".join(item[text_field].split()).lower()
            if folded in seen:
                raise ListError(f"{where} repeats '{item[text_field]}'")
            seen.add(folded)
        return items

    barred = entries("barred", "term")
    if not barred:
        raise ListError(f"{path}: 'barred' is empty, so the check would pass on anything")
    for item in barred:
        if not re.search(r"\w", item["term"]):
            raise ListError(f"{path}: term '{item['term']}' has no letter or digit to match")
    return barred, entries("allowlist", "sentence")


def term_pattern(term: str) -> re.Pattern[str]:
    words = [re.escape(word) for word in re.split(SEPARATOR + "+", term.strip()) if word]
    return re.compile(r"(?<!\w)" + (SEPARATOR + "*").join(words) + r"(?!\w)", re.IGNORECASE)


def sentence_pattern(sentence: str) -> re.Pattern[str]:
    return re.compile(r"\s+".join(re.escape(word) for word in sentence.split()))


def default_files(root: Path) -> list[Path]:
    files = [root / name for name in ROOT_FILES if (root / name).is_file()]
    for top in ROOT_DIRS:
        for current, dirs, names in os.walk(root / top):
            dirs[:] = sorted(d for d in dirs if d not in PRUNED_DIRS and not d.startswith("."))
            files.extend(Path(current) / name for name in sorted(names) if name.endswith(".md"))
    return files


def display(path: Path, root: Path) -> str:
    try:
        return path.resolve().relative_to(root.resolve()).as_posix()
    except ValueError:
        return path.as_posix()


def scan(text: str, terms: list[tuple[dict, re.Pattern[str]]], allowed: list[re.Pattern[str]]):
    """Return (hits, allowlisted) as lists of (line, term, reason), in document order."""
    line_starts = [0] + [match.end() for match in re.finditer(r"\n", text)]
    spans = [match.span() for pattern in allowed for match in pattern.finditer(text)]
    hits, allowlisted = [], []
    for entry, pattern in terms:
        for match in pattern.finditer(text):
            line = bisect_right(line_starts, match.start())
            inside = any(start <= match.start() and match.end() <= end for start, end in spans)
            (allowlisted if inside else hits).append((line, entry["term"], entry["reason"]))
    return sorted(hits), sorted(allowlisted)


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(
        description="Fail when wording that has been ruled out appears in the public documents.",
        epilog="Exit status: 0 no hits, 1 at least one hit, 2 unusable list file or arguments.",
    )
    parser.add_argument("files", nargs="*", type=Path, help="scan only these files")
    parser.add_argument("--list", type=Path, default=DEFAULT_LIST, dest="list_file",
                        help="barred terms and allowlisted sentences (default: %(default)s)")
    parser.add_argument("--root", type=Path, default=DEFAULT_ROOT,
                        help="repository root to scan and to print paths against")
    args = parser.parse_args(argv)

    try:
        barred, allowlist = load_list(args.list_file)
    except ListError as err:
        print(f"check-public-wording: {err}", file=sys.stderr)
        return 2
    terms = [(entry, term_pattern(entry["term"])) for entry in barred]
    allowed = [sentence_pattern(entry["sentence"]) for entry in allowlist]

    files = args.files or default_files(args.root)
    if not files:
        print(f"check-public-wording: no documents to scan under {args.root}", file=sys.stderr)
        return 2

    per_file: list[tuple[str, int]] = []
    allowlisted_total = 0
    for path in files:
        name = display(path, args.root)
        try:
            text = path.read_text(encoding="utf-8", errors="replace")
        except OSError as err:
            print(f"check-public-wording: cannot read {name}: {err.strerror}", file=sys.stderr)
            return 2
        hits, allowlisted = scan(text, terms, allowed)
        allowlisted_total += len(allowlisted)
        for line, term, reason in hits:
            print(f"{name}:{line}: {term}")
            print(f"    reason: {reason}")
        if hits:
            per_file.append((name, len(hits)))

    total = sum(count for _, count in per_file)
    print()
    for name, count in per_file:
        print(f"{count:4d}  {name}")
    print(f"{total} hit(s) in {len(per_file)} of {len(files)} file(s) scanned; "
          f"{allowlisted_total} more inside allowlisted sentences, not counted.")
    if total:
        print("Reword each line above, or, where the sentence accurately says what the code "
              f"does not do, add it to the allowlist in {display(args.list_file, args.root)}.")
    return 1 if total else 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
