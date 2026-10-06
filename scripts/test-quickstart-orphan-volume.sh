#!/usr/bin/env bash
# test-quickstart-orphan-volume.sh -- quickstart.sh must stop, before writing
# anything, when the database volume of an earlier install is still present.
#
# The postgres image applies POSTGRES_PASSWORD only to an empty data directory.
# A user who deletes the install directory and runs the quickstart again gets a
# freshly generated .env whose password the old, already-initialised volume
# rejects, and the run ends in the health-check timeout with no cause given.
#
# docker and curl are replaced by stubs on PATH, so this runs without a Docker
# daemon or network. The stubs log every invocation; the assertions read that
# log to prove what the script did and did not run.
#
# Refusal cases are paired with controls that MUST proceed: "the script stops"
# alone is also satisfied by a script that always stops.
#
# Usage: ./scripts/test-quickstart-orphan-volume.sh
#        QUICKSTART=/path/to/quickstart.sh ./scripts/test-quickstart-orphan-volume.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QUICKSTART="${QUICKSTART:-$SCRIPT_DIR/quickstart.sh}"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

BIN="$WORK/bin"
mkdir -p "$BIN"

# docker stub: volumes in STUB_VOLUMES exist; STUB_CONTAINERS holds
# <volume>:<container> pairs for containers that mount a volume.
cat > "$BIN/docker" <<'STUB'
#!/usr/bin/env bash
printf 'docker %s\n' "$*" >> "$STUB_LOG"
case "${1:-}" in
  info) exit 0 ;;
  volume)
    [ "${2:-}" = inspect ] || exit 1
    for v in ${STUB_VOLUMES:-}; do [ "$v" = "${3:-}" ] && exit 0; done
    exit 1 ;;
  ps)
    vol=""
    for a in "$@"; do case "$a" in volume=*) vol=${a#volume=} ;; esac; done
    for pair in ${STUB_CONTAINERS:-}; do
      [ "${pair%%:*}" = "$vol" ] && printf '%s\n' "${pair#*:}"
    done
    exit 0 ;;
  compose) exit 0 ;;
esac
exit 1
STUB

# curl stub: a download (-o FILE) writes a placeholder; the health probe
# succeeds unless STUB_HEALTH=down.
cat > "$BIN/curl" <<'STUB'
#!/usr/bin/env bash
printf 'curl %s\n' "$*" >> "$STUB_LOG"
out="" prev=""
for a in "$@"; do [ "$prev" = "-o" ] && out=$a; prev=$a; done
if [ -n "$out" ]; then printf 'services: {}\n' > "$out"; exit 0; fi
[ "${STUB_HEALTH:-up}" = up ]
STUB

# sleep stub: keeps the 30-try health loop instant.
printf '#!/usr/bin/env bash\nexit 0\n' > "$BIN/sleep"
chmod +x "$BIN/docker" "$BIN/curl" "$BIN/sleep"

FAILURES=0
fail() { echo "  FAIL: $*"; FAILURES=$((FAILURES + 1)); }
pass() { echo "  ok:   $*"; }

# run_case NAME [VAR=value ...] -- runs quickstart.sh in a fresh directory
# with a clean environment. Sets CASE_DIR, CASE_OUT, CASE_LOG and CASE_RC.
run_case() {
  local name="$1"; shift
  CASE_DIR="$WORK/$name"
  CASE_OUT="$WORK/$name.out"
  CASE_LOG="$WORK/$name.log"
  mkdir -p "$CASE_DIR"
  : > "$CASE_LOG"
  if [ -n "${PREPARE:-}" ]; then (cd "$CASE_DIR" && eval "$PREPARE"); fi
  set +e
  (cd "$CASE_DIR" && env -i PATH="$BIN:$PATH" HOME="$WORK" STUB_LOG="$CASE_LOG" "$@" \
    bash "$QUICKSTART" > "$CASE_OUT" 2>&1)
  CASE_RC=$?
  set -e
  echo "-- $name (exit $CASE_RC)"
}

expect_rc()       { if [ "$CASE_RC" -eq "$1" ]; then pass "exits $1"; else fail "exits $CASE_RC, want $1"; fi; }
expect_out()      { if grep -q -F -- "$1" "$CASE_OUT"; then pass "prints: $1"; else fail "output lacks: $1"; fi; }
expect_no_out()   { if grep -q -F -- "$1" "$CASE_OUT"; then fail "output has: $1"; else pass "does not print: $1"; fi; }
expect_ran()      { if grep -q -F -- "$1" "$CASE_LOG"; then pass "ran: $1"; else fail "did not run: $1"; fi; }
expect_not_ran()  { if grep -q -F -- "$1" "$CASE_LOG"; then fail "ran: $1"; else pass "did not run: $1"; fi; }
expect_no_path()  { if [ ! -e "$CASE_DIR/$1" ]; then pass "no $1 created"; else fail "$1 was created"; fi; }
dump()            { [ "$CASE_RC" -eq "$1" ] || sed 's/^/    | /' "$CASE_OUT"; }

echo "== quickstart.sh and a database volume left by an earlier install =="

# 1. Install directory deleted, both volumes remain.
PREPARE="" run_case deleted-dir STUB_VOLUMES="aim_postgres_data aim_redis_data"
expect_rc 1
expect_no_path aim
expect_not_ran "docker compose"
expect_not_ran "curl"
expect_not_ran "docker volume rm"
expect_out "aim_postgres_data"
expect_out "put the earlier .env and docker-compose.quickstart.yml back in ./aim"
expect_out "docker volume rm aim_postgres_data aim_redis_data"
expect_out "deletes every agent, user and event"
expect_no_out "docker rm -f"

# 2. Control: no earlier volume, so a first run proceeds.
PREPARE="" run_case fresh STUB_VOLUMES=""
expect_rc 0; dump 0
expect_ran "docker volume inspect aim_postgres_data"
expect_ran "docker compose -f docker-compose.quickstart.yml up -d"
if [ -f "$CASE_DIR/aim/.env" ]; then pass ".env written"; else fail ".env not written"; fi

# 3. Control: an existing install owns its volume and must start as before.
PREPARE="mkdir aim && printf 'services: {}\n' > aim/docker-compose.quickstart.yml" \
  run_case existing STUB_VOLUMES="aim_postgres_data aim_redis_data"
expect_rc 0; dump 0
expect_not_ran "docker volume inspect"
expect_ran "docker compose -f docker-compose.quickstart.yml up -d"

# 4. Compose file deleted, earlier .env kept: the .env must not be overwritten.
PREPARE="mkdir aim && printf 'POSTGRES_PASSWORD=earlier\n' > aim/.env" \
  run_case compose-file-deleted STUB_VOLUMES="aim_postgres_data"
expect_rc 1
if [ "$(cat "$CASE_DIR/aim/.env")" = "POSTGRES_PASSWORD=earlier" ]; then pass ".env unchanged"; else fail ".env was rewritten"; fi
expect_no_path aim/docker-compose.quickstart.yml
expect_not_ran "docker compose"

# 5. The probed name follows Compose's default project name for AIM_DIR.
PREPARE="" run_case custom-dir AIM_DIR="Lab/My.AIM" STUB_VOLUMES="myaim_postgres_data"
expect_rc 1
expect_out "myaim_postgres_data"
expect_no_path Lab
PREPARE="" run_case custom-dir-control AIM_DIR="Lab/My.AIM" STUB_VOLUMES="aim_postgres_data"
expect_rc 0; dump 0
expect_ran "docker volume inspect myaim_postgres_data"

# 6. COMPOSE_PROJECT_NAME, when set, names the project instead.
PREPARE="" run_case project-env COMPOSE_PROJECT_NAME="stack" STUB_VOLUMES="stack_postgres_data"
expect_rc 1
expect_out "stack_postgres_data"

# 7. Only the database volume remains, and an earlier container still mounts it:
#    the start-over commands name exactly what exists, container first.
PREPARE="" run_case container-attached STUB_VOLUMES="aim_postgres_data" STUB_CONTAINERS="aim_postgres_data:aim-postgres"
expect_rc 1
expect_out "docker rm -f aim-postgres"
expect_out "docker volume rm aim_postgres_data"
expect_no_out "aim_redis_data"
expect_not_ran "docker rm"

# 8. The health timeout names the inspect step for a volume the probe missed.
PREPARE="" run_case health-timeout STUB_VOLUMES="" STUB_HEALTH="down"
expect_rc 1
expect_out "Backend did not become healthy"
expect_out "docker volume ls"

echo
if (( FAILURES > 0 )); then
  echo "FAILED: $FAILURES assertion(s)."
  exit 1
fi
echo "PASSED: quickstart.sh stops before writing over an earlier install's database."
