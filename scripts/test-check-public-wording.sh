#!/bin/sh
# test-check-public-wording.sh -- check-public-wording.py must fail on every barred
# term and pass on every allowlisted sentence.
#
# The fixtures live in scripts/public-wording/testdata/:
#
#   fail-<term>.md   one per barred term in barred.json (a term with no fixture is a
#                    failure here, so a new term cannot land untested). Lines carrying
#                    the comment "<!-- hit -->" must be reported, and no other line.
#   fail-*.md        any other failing case, read the same way.
#   pass-*.md        must exit 0 with nothing reported.
#   invalid-*.json   a list file the check must refuse with exit 2.
#
# "Exits 1" alone is also satisfied by a check that reports the wrong line or the wrong
# term, so each failing case compares the reported lines with the marked lines, and each
# per-term fixture also compares the reported term.

set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CHECK="$SCRIPT_DIR/check-public-wording.py"
LIST="$SCRIPT_DIR/public-wording/barred.json"
DATA="$SCRIPT_DIR/public-wording/testdata"

FAILURES=0
FAIL_CASES=0
PASS_CASES=0
REFUSE_CASES=0

fail() {
  echo "  FAIL: $*"
  FAILURES=$((FAILURES + 1))
}

pass() {
  echo "  ok:   $*"
}

# reported_lines OUTPUT NAME -> the line numbers the check reported for fixture NAME
reported_lines() {
  printf '%s\n' "$1" | sed -n "s|^.*/$2:\([0-9][0-9]*\): .*\$|\1|p" | sort -n | tr '\n' ' '
}

# reported_terms OUTPUT NAME -> the distinct terms the check reported for fixture NAME
reported_terms() {
  printf '%s\n' "$1" | sed -n "s|^.*/$2:[0-9][0-9]*: \(.*\)\$|\1|p" | sort -u | tr '\n' ';'
}

# check_failing FIXTURE [TERM] -- exit 1, the marked lines and only those, and TERM if given
check_failing() {
  fixture="$1"
  name="$(basename "$fixture")"
  FAIL_CASES=$((FAIL_CASES + 1))
  output="$(python3 "$CHECK" "$fixture" 2>&1)"
  rc=$?
  want="$(grep -n -e '<!-- hit -->' "$fixture" | cut -d: -f1 | sort -n | tr '\n' ' ')"
  got="$(reported_lines "$output" "$name")"
  if [ "$rc" -ne 1 ]; then
    fail "$name -- exit $rc, want 1"
  elif [ -z "$want" ]; then
    fail "$name -- the fixture marks no line with <!-- hit -->"
  elif [ "$got" != "$want" ]; then
    fail "$name -- reported lines [ $got], want [ $want]"
  elif [ $# -eq 2 ] && [ "$(reported_terms "$output" "$name")" != "$2;" ]; then
    fail "$name -- reported terms [$(reported_terms "$output" "$name")], want [$2;]"
  elif ! printf '%s\n' "$output" | grep -q '^    reason: .'; then
    fail "$name -- a hit was printed without its reason"
  else
    pass "$name fails on lines $want"
  fi
}

echo "barred terms: one failing fixture each"
TERMS="$(python3 -c '
import json, re, sys
for entry in json.load(open(sys.argv[1], encoding="utf-8"))["barred"]:
    term = entry["term"]
    print(re.sub(r"[^a-z0-9]+", "-", term.lower()).strip("-") + "\t" + term)
' "$LIST")" || { echo "  FAIL: cannot read the terms from $LIST"; exit 1; }
[ -n "$TERMS" ] || { echo "  FAIL: $LIST holds no barred term"; exit 1; }

TERM_FIXTURES=""
TAB="$(printf '\t')"
while IFS="$TAB" read -r slug term; do
  fixture="$DATA/fail-$slug.md"
  TERM_FIXTURES="$TERM_FIXTURES $fixture"
  if [ -f "$fixture" ]; then
    check_failing "$fixture" "$term"
  else
    FAIL_CASES=$((FAIL_CASES + 1))
    fail "'$term' has no fixture: add $fixture"
  fi
done <<EOF
$TERMS
EOF

echo "other failing cases"
for fixture in "$DATA"/fail-*.md; do
  case " $TERM_FIXTURES " in
    *" $fixture "*) ;;
    *) check_failing "$fixture" ;;
  esac
done

echo "allowlisted sentences and near misses: nothing reported"
for fixture in "$DATA"/pass-*.md; do
  name="$(basename "$fixture")"
  PASS_CASES=$((PASS_CASES + 1))
  output="$(python3 "$CHECK" "$fixture" 2>&1)"
  rc=$?
  got="$(reported_lines "$output" "$name")"
  if [ "$rc" -ne 0 ]; then
    fail "$name -- exit $rc, want 0; reported lines [ $got]"
  elif [ -n "$got" ]; then
    fail "$name -- exit 0 but reported lines [ $got]"
  else
    pass "$name passes"
  fi
done

# The allowlist fixture must be passing because of the allowlist, not because the check
# missed the terms: with the allowlist emptied, the same file has to fail.
PASS_CASES=$((PASS_CASES + 1))
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
python3 -c '
import json, sys
data = json.load(open(sys.argv[1], encoding="utf-8"))
data["allowlist"] = []
json.dump(data, open(sys.argv[2], "w", encoding="utf-8"))
' "$LIST" "$WORK/no-allowlist.json"
python3 "$CHECK" --list "$WORK/no-allowlist.json" "$DATA/pass-allowlisted.md" >/dev/null 2>&1
rc=$?
if [ "$rc" -eq 1 ]; then
  pass "pass-allowlisted.md fails once the allowlist is emptied"
else
  fail "pass-allowlisted.md -- exit $rc with an empty allowlist, want 1"
fi

echo "unusable list files: refused"
for bad in "$DATA"/invalid-*.json; do
  name="$(basename "$bad")"
  REFUSE_CASES=$((REFUSE_CASES + 1))
  python3 "$CHECK" --list "$bad" "$DATA/pass-near-misses.md" >/dev/null 2>&1
  rc=$?
  if [ "$rc" -eq 2 ]; then
    pass "$name is refused"
  else
    fail "$name -- exit $rc, want 2"
  fi
done

echo
echo "failing-fixture cases: $FAIL_CASES  passing-fixture cases: $PASS_CASES  refused-list cases: $REFUSE_CASES"
if [ "$FAILURES" -ne 0 ]; then
  echo "$FAILURES case(s) FAILED"
  exit 1
fi
echo "all cases passed"
