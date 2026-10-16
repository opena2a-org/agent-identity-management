#!/usr/bin/env bash
# test-lint-no-executable-blobs.sh -- lint-no-executable-blobs.py must refuse a
# tracked blob that starts with a compiled executable's magic bytes, in every
# family, whatever the file is called, and must refuse to run without git.
#
# Six compiled server binaries were once tracked under apps/backend and every
# clone distributed them. The lint keys on the bytes, so each synthetic file
# here sits under a name that says nothing about what it is (a Mach-O at
# assets/logo.png), and a PNG header sits at bin/server with mode 100755. The
# files are written from hex by this script and hold a header only, never a
# real binary. A PNG, an "MZ" text without a PE header, an empty file and a
# text file are decoys: "the lint fails" alone is also satisfied by a lint that
# rejects every binary-looking file.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LINT="$SCRIPT_DIR/lint-no-executable-blobs.py"
PYTHON="$(command -v python3)"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

# The throwaway repositories must not read the caller's git configuration:
# a global hook, a signing requirement or a default branch name would change
# what these commits do.
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_NOSYSTEM=1

FAILURES=0

fail() {
  echo "  FAIL: $*"
  FAILURES=$((FAILURES + 1))
}

pass() {
  echo "  ok:   $*"
}

# zeros <n>: n zero bytes, as hex.
zeros() {
  local i out=""
  for ((i = 0; i < $1; i++)); do out+="00"; done
  printf '%s' "$out"
}

# write_hex <file> <hex>: write the bytes the hex string spells.
write_hex() {
  local file="$1" hex="$2" out="" i
  mkdir -p "$(dirname "$file")"
  for ((i = 0; i < ${#hex}; i += 2)); do out+="\\x${hex:i:2}"; done
  printf '%b' "$out" > "$file"
}

# new_repo <dir>: an empty repository on branch main.
new_repo() {
  mkdir -p "$1"
  git -C "$1" -c init.defaultBranch=main init -q
}

# commit_all <dir> <message>: commit the working tree of <dir>.
commit_all() {
  git -C "$1" add -A
  git -C "$1" -c user.name=test -c user.email=test@example.invalid commit -q -m "$2"
}

# run_lint <dir> [args...]: run the lint from inside <dir>; sets RC and leaves
# stdout in $WORK/out and stderr in $WORK/err.
run_lint() {
  local dir="$1"
  shift
  RC=0
  (cd "$dir" && "$PYTHON" -I "$LINT" "$@") > "$WORK/out" 2> "$WORK/err" || RC=$?
}

show_output() {
  sed 's/^/        out: /' "$WORK/out"
  sed 's/^/        err: /' "$WORK/err"
}

# expect_lines <want-file> <label>: stdout holds exactly the lines of <want-file>,
# in any order.
expect_lines() {
  if diff <(sort "$1") <(sort "$WORK/out") > "$WORK/diff"; then
    pass "$2"
  else
    fail "$2"
    sed 's/^/        /' "$WORK/diff"
  fi
}

# --- the synthetic headers -------------------------------------------------
# Mach-O thin, 64-bit, little-endian (MH_CIGAM_64 as the bytes read), arm64.
MACHO_64_LE="cffaedfe""0c000001""00000000""02000000"
# Mach-O universal: 0xCAFEBABE, then nfat_arch = 2, then the first fat_arch.
UNIVERSAL="cafebabe""00000002""01000007""00000003"
# Java class: 0xCAFEBABE, minor 0, major 65.
JAVA_CLASS="cafebabe""0000""0041""003d0a00"
# ELF, 64-bit, little-endian, version 1.
ELF="7f454c46""020101""00""$(zeros 8)""0200""3e00"
# PE: "MZ", e_lfanew = 0x80 at offset 0x3C, "PE\0\0" and machine x86-64 at 0x80.
PE="4d5a""$(zeros 58)""80000000""$(zeros 64)""50450000""6486"
# PNG signature and the start of an IHDR chunk.
PNG="89504e470d0a1a0a""0000000d""49484452"
# An MZ that is not a PE: e_lfanew points inside the file at bytes that are not
# "PE\0\0".
MZ_NOT_PE="4d5a""$(zeros 58)""40000000""4e450000""$(zeros 12)"

echo "one file per family is refused and named; the decoys are not"
REPO="$WORK/families"
new_repo "$REPO"
write_hex "$REPO/assets/logo.png" "$MACHO_64_LE"
write_hex "$REPO/docs/guide.md" "$UNIVERSAL"
write_hex "$REPO/lib/Main.txt" "$JAVA_CLASS"
write_hex "$REPO/README" "$ELF"
write_hex "$REPO/web/app.js" "$PE"
write_hex "$REPO/bin/server" "$PNG"
chmod 755 "$REPO/bin/server"
write_hex "$REPO/notes/mz-header.bin" "$MZ_NOT_PE"
printf 'MZ is the two-letter prefix of a DOS header; this file is text.\n' > "$REPO/notes/mz.txt"
: > "$REPO/notes/empty"
printf 'plain text\n' > "$REPO/notes/readme.txt"
commit_all "$REPO" "five executable headers and four decoys"

cat > "$WORK/want-families" <<'EOF'
README: elf
assets/logo.png: mach-o
docs/guide.md: mach-o-universal
lib/Main.txt: java-class
web/app.js: pe
EOF

run_lint "$REPO"
if [[ $RC -eq 1 ]]; then
  pass "exit 1 at HEAD"
else
  fail "exit $RC at HEAD, want 1"
  show_output
fi
expect_lines "$WORK/want-families" "exactly the five expected lines"

echo "every Mach-O thin width and byte order is refused"
THIN="$WORK/thin"
new_repo "$THIN"
write_hex "$THIN/a" "feedface""00000007"
write_hex "$THIN/b" "cefaedfe""07000000"
write_hex "$THIN/c" "feedfacf""01000007"
write_hex "$THIN/d" "cffaedfe""07000001"
commit_all "$THIN" "four thin Mach-O headers"
printf '%s: mach-o\n' a b c d > "$WORK/want-thin"
run_lint "$THIN"
if [[ $RC -eq 1 ]]; then
  pass "exit 1"
else
  fail "exit $RC, want 1"
  show_output
fi
expect_lines "$WORK/want-thin" "32-bit and 64-bit, big- and little-endian"

echo "a clean commit passes, and the ref argument is the tree that is read"
git -C "$REPO" rm -q assets/logo.png docs/guide.md lib/Main.txt README web/app.js
commit_all "$REPO" "remove the executable headers"
run_lint "$REPO"
if [[ $RC -eq 0 ]]; then
  pass "exit 0 at HEAD: $(cat "$WORK/out")"
else
  fail "exit $RC at the clean HEAD, want 0"
  show_output
fi
run_lint "$REPO" HEAD~1
if [[ $RC -eq 1 ]]; then
  pass "exit 1 at HEAD~1"
else
  fail "exit $RC at HEAD~1, want 1"
  show_output
fi
expect_lines "$WORK/want-families" "HEAD~1 still names the five"

echo "no git, or a failed git call, is a named refusal and never a pass"
RC=0
(cd "$REPO" && PATH=/nonexistent "$PYTHON" -I "$LINT") > "$WORK/out" 2> "$WORK/err" || RC=$?
if [[ $RC -eq 3 ]] && grep -q 'refused: git is not on PATH' "$WORK/err"; then
  pass "PATH=/nonexistent: exit 3, $(head -n 1 "$WORK/err")"
else
  fail "PATH=/nonexistent: exit $RC, want 3 with 'refused: git is not on PATH'"
  show_output
fi
run_lint "$REPO" no-such-ref
if [[ $RC -eq 3 ]] && grep -q 'refused: git .*no-such-ref.* failed: ' "$WORK/err"; then
  pass "unknown ref: exit 3, $(head -n 1 "$WORK/err")"
else
  fail "unknown ref: exit $RC, want 3 naming the failed git call"
  show_output
fi
mkdir -p "$WORK/not-a-repo"
GIT_CEILING_DIRECTORIES="$WORK" run_lint "$WORK/not-a-repo"
if [[ $RC -eq 3 ]] && grep -q 'refused:' "$WORK/err"; then
  pass "outside a repository: exit 3, $(head -n 1 "$WORK/err")"
else
  fail "outside a repository: exit $RC, want 3 with a refusal"
  show_output
fi

if [[ $FAILURES -ne 0 ]]; then
  echo "test-lint-no-executable-blobs: $FAILURES failure(s)"
  exit 1
fi
echo "test-lint-no-executable-blobs: all checks passed"
