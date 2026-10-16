#!/usr/bin/env python3
"""No tracked file may be a compiled executable.

Six compiled server binaries were once tracked under apps/backend, mode 100755,
and every clone of the public repository distributed them. Nothing built from
them and nothing ran them, so nothing noticed. A binary carries what the build
machine had: paths, embedded strings, whatever was linked in. The source is
the artifact this repository publishes; binaries come from a build.

What is read: EVERY blob in the tree of the given ref (default HEAD), whatever
its path, extension or mode, through `git ls-tree -r -z --full-tree` and
`git cat-file --batch`. The decision is made on the first 64 bytes of each blob,
and for a blob that starts with `MZ` also on the four bytes its `e_lfanew` field
(offset 0x3C) points at, since a PE header usually sits past byte 64. The rest of
each blob is read off the pipe and discarded. A submodule entry is a commit, not
a blob, and is not read.

What is refused, by magic bytes only:

  * mach-o            thin Mach-O, 32- and 64-bit, either byte order
                      (FE ED FA CE, CE FA ED FE, FE ED FA CF, CF FA ED FE);
  * mach-o-universal  CA FE BA BE (or CA FE BA BF, the 64-bit fat header) with an
                      architecture count below 45 in bytes 4-7;
  * java-class        CA FE BA BE with a larger value there (minor and major
                      version; the first Java class major version is 45);
  * elf               7F 45 4C 46;
  * pe                MZ whose e_lfanew points at "PE\\0\\0".

The universal/Java split only names the family: every blob starting CA FE BA BE
is refused either way. There is no path list, no extension rule and no
allowlist. An `MZ` without a PE header (a text file can start with those two
letters) is not refused.

    python3 scripts/lint-no-executable-blobs.py [ref]

Exit: 0 no executable, 1 one `path: <family>` line on stdout per executable,
2 a usage error, 3 a refusal: git is not on PATH, or a git call failed (not a
repository, an unknown ref, a missing object). A refusal is never a pass.
"""
from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
from typing import BinaryIO

NAME = "lint-no-executable-blobs"
HEAD_BYTES = 64
CHUNK = 1 << 16

MACH_O_THIN = (
    bytes.fromhex("feedface"),
    bytes.fromhex("cefaedfe"),
    bytes.fromhex("feedfacf"),
    bytes.fromhex("cffaedfe"),
)
FAT = (bytes.fromhex("cafebabe"), bytes.fromhex("cafebabf"))
ELF = bytes.fromhex("7f454c46")
MZ = b"MZ"
PE_SIGNATURE = b"PE\0\0"
E_LFANEW = 0x3C
# The first Java class file major version is 45; a fat header's architecture
# count sits in the same four bytes and is never that large in practice.
FIRST_JAVA_MAJOR = 45


class Refusal(Exception):
    """A reason the lint cannot give a verdict."""


def family(head: bytes, pe_signature: bytes) -> str | None:
    """The executable family `head` starts, or None."""
    magic = head[:4]
    if magic in MACH_O_THIN:
        return "mach-o"
    if magic in FAT:
        if magic == FAT[0] and len(head) >= 8 and int.from_bytes(head[4:8], "big") >= FIRST_JAVA_MAJOR:
            return "java-class"
        return "mach-o-universal"
    if magic == ELF:
        return "elf"
    if head[:2] == MZ and pe_signature == PE_SIGNATURE:
        return "pe"
    return None


def read_exact(stream: BinaryIO, size: int) -> bytes:
    data = stream.read(size)
    if len(data) != size:
        raise Refusal("git cat-file --batch ended inside an object")
    return data


def read_blob(stream: BinaryIO, size: int) -> tuple[bytes, bytes]:
    """Consume one blob of `size` bytes; return its first 64 bytes and, for an MZ
    blob, the four bytes at its e_lfanew offset (empty when there are none)."""
    head = read_exact(stream, min(size, HEAD_BYTES))
    window = -1
    if head[:2] == MZ and len(head) >= E_LFANEW + 4:
        window = int.from_bytes(head[E_LFANEW:E_LFANEW + 4], "little")
    signature = bytearray()

    def take(chunk: bytes, pos: int) -> None:
        if window < 0:
            return
        lo, hi = max(window, pos), min(window + 4, pos + len(chunk))
        if lo < hi:
            signature.extend(chunk[lo - pos:hi - pos])

    take(head, 0)
    pos = len(head)
    while pos < size:
        chunk = read_exact(stream, min(CHUNK, size - pos))
        take(chunk, pos)
        pos += len(chunk)
    if read_exact(stream, 1) != b"\n":
        raise Refusal("git cat-file --batch output is not framed as expected")
    return head, bytes(signature)


def run_git(git: str, args: list[str]) -> bytes:
    try:
        result = subprocess.run([git, *args], capture_output=True, check=False)
    except OSError as err:
        raise Refusal(f"git {args[0]} could not be started: {err}") from err
    if result.returncode != 0:
        detail = result.stderr.decode("utf-8", "replace").strip().splitlines()
        reason = detail[0] if detail else f"exit {result.returncode}"
        raise Refusal(f"git {' '.join(args)} failed: {reason}")
    return result.stdout


def tracked_blobs(git: str, ref: str) -> list[tuple[bytes, str]]:
    """(path, object id) for every blob in the tree of `ref`."""
    listing = run_git(git, ["ls-tree", "-r", "-z", "--full-tree", ref])
    blobs: list[tuple[bytes, str]] = []
    for entry in listing.split(b"\0"):
        if not entry:
            continue
        meta, sep, path = entry.partition(b"\t")
        fields = meta.split(b" ")
        if not sep or len(fields) != 3:
            raise Refusal(f"git ls-tree printed an entry this lint cannot read: {entry[:80]!r}")
        if fields[1] == b"blob":
            blobs.append((path, fields[2].decode("ascii")))
    return blobs


def classify(git: str, oids: list[str]) -> dict[str, str | None]:
    """The family of each object id, read through one `git cat-file --batch`."""
    families: dict[str, str | None] = {}
    with tempfile.TemporaryFile() as errors:
        try:
            proc = subprocess.Popen([git, "cat-file", "--batch"], stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=errors)
        except OSError as err:
            raise Refusal(f"git cat-file could not be started: {err}") from err
        assert proc.stdin is not None and proc.stdout is not None
        try:
            for oid in oids:
                # One request at a time: without --buffer git flushes each answer,
                # so neither pipe can fill while the other side waits.
                proc.stdin.write(oid.encode("ascii") + b"\n")
                proc.stdin.flush()
                header = proc.stdout.readline().rstrip(b"\n").split(b" ")
                if len(header) != 3 or header[1] != b"blob" or not header[2].isdigit():
                    raise Refusal(f"git cat-file --batch could not read {oid}: {b' '.join(header).decode('utf-8', 'replace')}")
                head, signature = read_blob(proc.stdout, int(header[2]))
                families[oid] = family(head, signature)
            proc.stdin.close()
        except OSError as err:
            raise Refusal(f"git cat-file --batch failed: {err}") from err
        finally:
            if not proc.stdin.closed:
                proc.stdin.close()
            proc.stdout.close()
            returncode = proc.wait()
        if returncode != 0:
            errors.seek(0)
            detail = errors.read().decode("utf-8", "replace").strip().splitlines()
            raise Refusal(f"git cat-file --batch failed: {detail[0] if detail else f'exit {returncode}'}")
    return families


def display(path: bytes) -> str:
    """The path as one printable line."""
    text = path.decode("utf-8", "backslashreplace")
    return "".join(c if c.isprintable() else f"\\x{ord(c):02x}" for c in text)


def main(argv: list[str]) -> int:
    if argv[:1] in (["-h"], ["--help"]):
        print(__doc__)
        return 0
    if len(argv) > 1 or (argv and argv[0].startswith("-")):
        print(f"usage: python3 scripts/{NAME}.py [ref]", file=sys.stderr)
        return 2
    ref = argv[0] if argv else "HEAD"

    git = shutil.which("git")
    try:
        if git is None:
            raise Refusal("git is not on PATH, so no tracked file could be read")
        # Resolve once and list that tree, so the id printed is the tree read.
        tree = run_git(git, ["rev-parse", "--verify", f"{ref}^{{tree}}"]).decode("ascii").strip()
        blobs = tracked_blobs(git, tree)
        families = classify(git, list(dict.fromkeys(oid for _, oid in blobs)))
    except Refusal as refusal:
        print(f"{NAME}: refused: {refusal}", file=sys.stderr)
        print(f"{NAME}: no verdict for {ref}; this is not a pass.", file=sys.stderr)
        return 3

    hits = [(path, families[oid]) for path, oid in blobs if families[oid]]
    if hits:
        for path, kind in hits:
            print(f"{display(path)}: {kind}")
        print(
            f"\n{NAME}: {len(hits)} of {len(blobs)} tracked files at {ref} are compiled executables.\n"
            "Fix: remove each from the index (git rm --cached <path>), add an exact-path rule\n"
            "for it to .gitignore, and build it in CI or a Dockerfile stage instead.\n"
            f"Verify: python3 scripts/{NAME}.py",
            file=sys.stderr,
        )
        return 1

    print(f"{NAME}: {len(blobs)} tracked files at {ref} (tree {tree[:12]}), none is a compiled executable")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
