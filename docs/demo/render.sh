#!/usr/bin/env bash
# Render one demo video from the README, on a throwaway stack of released
# images, with fixture data only.
#
#   docs/demo/render.sh <slug> [--rehearsal]
#
# A rehearsal installs this checkout's sdk/python instead of the published
# package, may type the lines listed in <slug>/rehearsal-lines.txt, and burns
# a REHEARSAL watermark on every frame; it is never uploaded. A counted render
# requires the three preconditions below (exit 2 names the one that fails).
#
# Exit codes: 0 rendered; 2 precondition failed; 3 lint or census failed;
# 4 stack-safety refusal (a project name already in use, or a host port asked for).
# The last stdout line is one JSON object: result, mp4, sha256, durationSeconds,
# sdkVersion, imageRevision, readmeCommit, rehearsal.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
SLUG="${1:?usage: render.sh <slug> [--rehearsal]}"
REHEARSAL=0; [ "${2:-}" = "--rehearsal" ] && REHEARSAL=1
DOCKER="${DOCKER:-docker}"; command -v "$DOCKER" >/dev/null 2>&1 || DOCKER="$HOME/.docker/bin/docker"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
PROJECT="aimdemo-${STAMP}"
RUN="${TMPDIR:-/tmp}/${PROJECT}"
OUT="$HERE/out/$SLUG"
SCENE="$HERE/$SLUG"
T0=$(date +%s)
log() { printf '%s render: %s\n' "$(date -u +%H:%M:%SZ)" "$*" >&2; }
fail() {
  local code=$1; shift; log "$*"
  if [ -f "$RUN/compose.env" ]; then
    log "last log lines of the stack services (fixture stack; no secrets are logged by the services)"
    compose logs --no-log-prefix --tail 30 backend frontend 2>/dev/null | sed 's/^/  | /' >&2 || true
    compose logs --no-log-prefix --tail 20 browser 2>/dev/null | sed 's/^/  |browser| /' >&2 || true
  fi
  echo "{\"result\":\"failed\",\"exit\":$code,\"reason\":\"$*\"}"; teardown || true; exit "$code"
}

# shellcheck disable=SC1091
. "$HERE/stack/images.env"

compose() { "$DOCKER" compose -p "$PROJECT" -f "$HERE/stack/compose.yml" --env-file "$RUN/compose.env" "$@"; }

teardown() {
  [ -f "$RUN/compose.env" ] || return 0
  compose stop >/dev/null 2>&1 || true
  compose rm -f -v >/dev/null 2>&1 || true
  for v in $("$DOCKER" volume ls -q --filter "name=^${PROJECT}_" 2>/dev/null); do "$DOCKER" volume rm "$v" >/dev/null 2>&1 || true; done
  for n in $("$DOCKER" network ls -q --filter "name=^${PROJECT}_" 2>/dev/null); do "$DOCKER" network rm "$n" >/dev/null 2>&1 || true; done
  rm -rf "$RUN"
  log "stack $PROJECT removed; run secrets deleted"
}
trap 'teardown' EXIT

# ---- stack safety -----------------------------------------------------------
if "$DOCKER" ps -a --format '{{.Names}}' | grep -q "^${PROJECT}[-_]"; then fail 4 "project name $PROJECT is already in use"; fi
if grep -qE '^\s*ports:' "$HERE/stack/compose.yml"; then fail 4 "stack/compose.yml must not publish host ports"; fi
if grep -qE 'container_name:' "$HERE/stack/compose.yml"; then fail 4 "stack/compose.yml must not fix container names"; fi

# ---- run secrets ------------------------------------------------------------
umask 077
mkdir -p "$RUN"
{
  echo "POSTGRES_PASSWORD=$(openssl rand -hex 16)"
  echo "REDIS_PASSWORD=$(openssl rand -hex 16)"
  echo "JWT_SECRET=$(openssl rand -hex 32)"
  echo "KEYVAULT_MASTER_KEY=$(openssl rand -base64 32)"
  echo "ADMIN_PASSWORD=$(openssl rand -base64 18 | tr '/+' 'Aa')Aa1!"
  echo "ADMIN_EMAIL=admin@example.com"
} > "$RUN/.env"
{
  cat "$RUN/.env"
  grep -v '^#' "$HERE/stack/images.env"
  echo "DEMO_SLUG=$SLUG"
  echo "DEMO_OUT=$OUT"
  echo "DEMO_RUN=$RUN"
  echo "DEMO_REHEARSAL=$REHEARSAL"
} > "$RUN/compose.env"
umask 022
rm -rf "$OUT"; mkdir -p "$OUT/cards" "$OUT/browser"

# ---- preconditions for a counted render ---------------------------------------
README_COMMIT="$(git -C "$ROOT" rev-parse --short HEAD)"
SDK_VERSION="$(tr -d '[:space:]' < "$ROOT/sdk/python/VERSION")"
if [ "$REHEARSAL" = 0 ]; then
  grep -q 'login --url' "$ROOT/README.md" || fail 2 "R1: README.md Quick start does not show the self-hosted login (aim-sdk login --url)"
  ! grep -q 'do not yet complete against a self-hosted backend' "$ROOT/README.md" || fail 2 "R1: README.md still says the self-hosted login does not complete"
  PUBLISHED="$(curl -fsS https://pypi.org/pypi/aim-sdk/json | python3 -c 'import json,sys; print(json.load(sys.stdin)["info"]["version"])')"
  [ "$PUBLISHED" != "2.0.3" ] || fail 2 "R2: PyPI aim-sdk is still 2.0.3 (its login predates the device grant)"
  SDK_VERSION="$PUBLISHED"
  for rev in "$AIM_SERVER_REVISION" "$AIM_DASHBOARD_REVISION"; do
    git -C "$ROOT" merge-base --is-ancestor 30ca4857e "$rev" 2>/dev/null || fail 2 "R3: image revision $rev is not at or after 30ca485"
  done
fi

# ---- tape and lint ------------------------------------------------------------
if [ "$REHEARSAL" = 1 ]; then
  # A rehearsal installs the checkout's SDK, but through the README's own line:
  # the wheel is built from the (read-only) source mount under Hide and pip is
  # pointed at it, so `pip install aim-sdk` on screen installs that wheel.
  INSTALL='Hide\nType '"'"'cp -r /work/sdk /tmp/aim-sdk-src && pip wheel -q -w /tmp/dist /tmp/aim-sdk-src 2>&1 | tail -1; export PIP_NO_INDEX=1 PIP_FIND_LINKS=/tmp/dist; clear'"'"'\nEnter\nWait+Line@600s /^\\$\\s*$/\nSleep 300ms\nShow\nSleep 1s\nType "pip install aim-sdk"\nEnter\nWait+Line@600s /^\\$\\s*$/\nSleep 3s'
else
  INSTALL='Type "pip install aim-sdk"\nEnter\nWait+Line@600s /^\\$\\s*$/\nSleep 3s'
fi
python3 - "$SCENE/tape.tape" "$OUT/tape.rendered.tape" "$INSTALL" <<'PY'
import sys
src, dst, install = sys.argv[1], sys.argv[2], sys.argv[3].encode().decode("unicode_escape")
text = open(src).read()
assert text.count("# @install") == 1
open(dst, "w").write(text.replace("# @install", install, 1))
PY
LINT_FLAGS=(); [ "$REHEARSAL" = 1 ] && LINT_FLAGS=(--rehearsal); [ -n "${DEMO_FORBIDDEN_FILE:-}" ] && LINT_FLAGS+=(--forbidden "$DEMO_FORBIDDEN_FILE")
node "$HERE/lib/lint.mjs" "$SLUG" --tape "$OUT/tape.rendered.tape" "${LINT_FLAGS[@]}" >&2 || fail 3 "lint failed"

# ---- images and stack -----------------------------------------------------------
"$DOCKER" image inspect "$VHS_BASE_IMAGE" >/dev/null 2>&1 || fail 2 "base image $VHS_BASE_IMAGE missing: build it from a hackmyagent checkout (docs/vhs/docker) first"
compose build --quiet gw terminal browser >&2
log "images built"
compose up -d --wait postgres redis backend frontend gw >&2 || fail 2 "the stack did not come up healthy"
log "stack $PROJECT up (backend $AIM_SERVER_REVISION, dashboard $AIM_DASHBOARD_REVISION)"
compose exec -T gw sh -c 'curl -fsS http://localhost:8080/health >/dev/null && curl -fsS -o /dev/null -w "%{http_code}" http://localhost:3000/device' | grep -q 200 || fail 2 "stack check: the API or the device page did not answer through the forwarder"
log "stack check passed: localhost:8080/health and localhost:3000/device answer inside the recording namespace"

# The released server seeds no administrator; its bootstrap binary does. The
# secrets travel to it on stdin, never as arguments, and its output is filtered.
(
  # shellcheck disable=SC1091
  . "$RUN/.env"
  printf 'export DEFAULT_ADMIN_PASSWORD=%q DATABASE_URL=%q; /app/aim-bootstrap --default --admin-email %q --org-name %q </dev/null 2>&1 | grep -vi password | tail -1\n' \
    "$ADMIN_PASSWORD" "postgres://postgres:${POSTGRES_PASSWORD}@postgres:5432/identity?sslmode=disable" "$ADMIN_EMAIL" "Default Organization"
) | compose exec -T backend sh >&2 || fail 2 "the administrator bootstrap failed"
# The bootstrap marks its password as temporary (force_password_change), which
# sends the first dashboard sign-in to a change-password page. The fixture
# administrator is one who has already completed that first sign-in.
compose exec -T postgres psql -q -U postgres -d identity -c "UPDATE users SET force_password_change = FALSE WHERE email = '$(grep '^ADMIN_EMAIL=' "$RUN/.env" | cut -d= -f2-)'" >&2 || fail 2 "could not clear the temporary-password flag"
log "fixture administrator $(grep '^ADMIN_EMAIL=' "$RUN/.env" | cut -d= -f2-) created (password generated for this run, never printed)"

# ---- record -----------------------------------------------------------------------
compose up -d browser >&2
BROWSER_ID="$(compose ps -q browser)"
compose run --rm -T terminal /work/out/tape.rendered.tape >&2 || fail 3 "the terminal recording failed"
log "terminal recorded"
"$DOCKER" wait "$BROWSER_ID" >/dev/null
BROWSER_RC="$("$DOCKER" inspect -f '{{.State.ExitCode}}' "$BROWSER_ID")"
compose logs --no-log-prefix browser 2>/dev/null | tail -3 >&2 || true
[ "$BROWSER_RC" = 0 ] || fail 3 "the browser walkthrough failed (exit $BROWSER_RC); see $OUT/browser"
log "browser recorded"

# ---- cards, edit, census ------------------------------------------------------------
REV7="$(printf '%s' "$AIM_SERVER_REVISION" | cut -c1-7)"
if [ "$REHEARSAL" = 1 ]; then
  STAMPLINE="rehearsal: aim-sdk built from source $README_COMMIT, AIM images $REV7, $(date -u +%F)"
  NAME="aim-${SLUG}_REHEARSAL-${README_COMMIT}_$(date -u +%F).mp4"
else
  STAMPLINE="aim-sdk quick start on a local fixture stack, recorded with aim-sdk $SDK_VERSION and AIM images $REV7, $(date -u +%F), hard cuts and labelled skips"
  NAME="aim-${SLUG}_aim-sdk-${SDK_VERSION}_$(date -u +%F).mp4"
fi
compose run --rm -T browser node /work/lib/render-card.mjs /work/scene/narration.md /work/out/cards "$STAMPLINE" >&2
compose run --rm -T --entrypoint node terminal /work/lib/edit.mjs "--output=/work/out/$NAME" "--rehearsal=$REHEARSAL" "--sha7=$README_COMMIT" "--stamp=$STAMPLINE" >&2 || fail 3 "edit failed"
node "$HERE/lib/census.mjs" "--out=$OUT" "--scene=$SCENE" "--env=$RUN/.env" ${DEMO_FORBIDDEN_FILE:+"--forbidden=$DEMO_FORBIDDEN_FILE"} >&2 || fail 3 "census failed"

# ---- sidecar ----------------------------------------------------------------------------
python3 - "$OUT/recording.json" "$NAME" "$SDK_VERSION" "$README_COMMIT" "$REHEARSAL" "$AIM_SERVER_IMAGE" "$AIM_SERVER_REVISION" "$AIM_DASHBOARD_IMAGE" "$AIM_DASHBOARD_REVISION" "$POSTGRES_IMAGE" "$REDIS_IMAGE" "$PLAYWRIGHT_IMAGE" "$VHS_BASE_SOURCE" "$ROOT" "$STAMPLINE" "$T0" <<'PY'
import hashlib, json, subprocess, sys, time
(f, name, sdk, commit, rehearsal, srv, srvrev, dash, dashrev, pg, redis, pw, vhs, root, stamp, t0) = sys.argv[1:]
d = json.load(open(f))
readme = open(root + "/README.md", encoding="utf8").read()
sec = readme.split("\n## Quick start", 1)[1].split("\n## ", 1)[0] if "\n## Quick start" in readme else ""
d.update({
  "video": name, "rehearsal": rehearsal == "1", "stamp": stamp,
  "sdk": {"version": sdk, "source": "checkout sdk/python" if rehearsal == "1" else "PyPI", "cliSha256": hashlib.sha256(open(root + "/sdk/python/aim_sdk/cli.py", "rb").read()).hexdigest()},
  "images": {"aimServer": {"ref": srv, "revision": srvrev}, "aimDashboard": {"ref": dash, "revision": dashrev}, "postgres": pg, "redis": redis, "playwright": pw, "vhsBase": vhs},
  "readme": {"commit": commit, "quickStartSha256": hashlib.sha256(sec.encode()).hexdigest()},
  "wallClockSeconds": int(time.time()) - int(t0),
})
json.dump(d, open(f, "w"), indent=2)
print(json.dumps({"result": "rendered", "mp4": "docs/demo/out/%s/%s" % (f.split("/out/")[1].split("/")[0], name), "sha256": d["sha256"], "durationSeconds": d["format"]["durationSeconds"], "sdkVersion": sdk, "imageRevision": srvrev[:7], "readmeCommit": commit, "rehearsal": rehearsal == "1", "wallClockSeconds": d["wallClockSeconds"]}))
PY
