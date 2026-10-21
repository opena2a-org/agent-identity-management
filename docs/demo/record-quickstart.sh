#!/usr/bin/env bash
# Renders docs/demo/quickstart.tape, the README's Quick start as one terminal recording, on a local
# AIM stack with fixture data only, and writes the recording's files beside the tape.
#
#   docs/demo/record-quickstart.sh             render, check, and write the outputs
#   docs/demo/record-quickstart.sh --dry-run   print what a render would use and write; change nothing
#
# A render:
# 1. Starts the API from docs/demo/stack/compose.yml (the postgres, redis and backend services, images
#    pinned by digest in docs/demo/stack/images.env) as its own compose project aimdemo-<utc-stamp>,
#    with secrets generated for the run in a directory mktemp creates, and nothing published on the
#    host. The dashboard is not started: no step of this recording opens it.
# 2. Creates the fixture administrator (admin@example.com) with the server's bootstrap binary.
# 3. In the terminal container (docs/demo/terminal/Dockerfile on the VHS base image), on the stack's
#    network and with no variable from this environment except the SDK version: forwards
#    localhost:8080 to the API (docs/demo/lib/gw.mjs) so the typed address is the README's; installs
#    aim-sdk[rich] into a virtual environment from wheels built from this checkout's sdk/python;
#    starts docs/demo/quickstart-admin.py, which sets strict mode, approves the device code and
#    verifies the agent off screen; renders the tape with HOME=/home/demo; then restores that home
#    directory to its state before the tape and writes the text version with
#    docs/demo/quickstart-capture.py (NO_COLOR=1, TERM=dumb, COLUMNS=110).
# 4. On this machine: checks that the recorded session and the text version show the outcomes the
#    README promises; runs a census of both for the run's secrets, credential shapes, local paths,
#    emails outside example.com, hosts outside the allow-list and this machine's user and host
#    names; and checks the pace rule (typing speed, the hold before each Enter, each step's hold
#    against its narration and its lines on screen, the length, the GIF's size and width). Only when
#    every check passes are the GIF, the step screenshots, the text version and the sidecar copied
#    into docs/demo; the MP4 goes to docs/demo/out/, which is not committed.
# 5. Removes the stack with its volumes and network, and deletes the run's secrets.
#
# The SDK is this checkout's build, so the sidecar says prerelease: true unless HEAD carries the
# sdk-py-v<version> tag. The published aim-sdk 2.0.3 cannot complete the device sign-in against a
# self-hosted server, so a recording of the published package waits for its next release.
#
# Prerequisites: Docker with Compose v2 (`docker compose` or `docker-compose`), Node 22 or later on
# this machine for the checks, and the VHS base image docs/demo/README.md names.
#
# Exit codes: 0 rendered; 2 a precondition failed (Docker, the base image, the stack, the install);
# 3 the recording or a check failed, and nothing was written to docs/demo; 4 stack-safety refusal.
# The last stdout line is one JSON object.
set -euo pipefail

NAME=quickstart

# ---- inside the terminal container ---------------------------------------------------------------

# wait_until <seconds> <command...>: runs the command every half second until it succeeds.
wait_until() {
  local deadline=$(( $(date +%s) + $1 )); shift
  until "$@" >/dev/null 2>&1; do
    [ "$(date +%s)" -lt "$deadline" ] || return 1
    sleep 0.5
  done
}

inside() {
  local email password env=/work/env out=/work/out
  IFS= read -r email
  IFS= read -r password
  cd /work
  mkdir -p "$env" docs/demo/out

  # localhost:8080 inside this container is the API, as the README types it.
  GW_ROUTES="8080=backend:8080" node /work/src/lib/gw.mjs > "$out/gw.log" 2>&1 &
  local gw=$!
  wait_until 90 python3 -c 'import urllib.request; urllib.request.urlopen("http://localhost:8080/health", timeout=3)' \
    || { echo "record-quickstart.sh: the API did not answer on localhost:8080" >&2; exit 2; }

  # aim-sdk[rich] from this checkout, installed before the tape: wheels built from a copy of the
  # read-only sdk/python, then the README's requirement against those wheels only.
  cp -r /work/sdk "$env/src"
  python3 -m venv "$env/venv"
  export PIP_NO_CACHE_DIR=1 PIP_DISABLE_PIP_VERSION_CHECK=1
  "$env/venv/bin/pip" wheel -q -w "$env/wheels" "$env/src[rich]" >/dev/null \
    || { echo "record-quickstart.sh: building the SDK wheels failed" >&2; exit 2; }
  PIP_NO_INDEX=1 PIP_FIND_LINKS="$env/wheels" "$env/venv/bin/pip" install -q "aim-sdk[rich]" \
    || { echo "record-quickstart.sh: installing aim-sdk[rich] failed" >&2; exit 2; }
  local installed
  installed="$(HOME="$env" "$env/venv/bin/aim-sdk" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)"
  if [ "$installed" != "$SDK_VERSION" ]; then
    echo "record-quickstart.sh: the container runs aim-sdk ${installed:-none}, not $SDK_VERSION; nothing was rendered." >&2
    exit 2
  fi

  # The fixture administrator, off screen. Its password travels on a pipe, never in an argument.
  printf '%s\n%s\n' "$email" "$password" | python3 /work/src/quickstart-admin.py \
    --api http://localhost:8080 --out "$out" --watch "$out/session.txt" --watch "$out/capture.txt" \
    2> "$out/admin.log" &
  local admin=$!
  password=""
  wait_until 60 test -e "$out/cue-ready" \
    || { echo "record-quickstart.sh: the fixture administrator did not set strict mode:" >&2; sed 's/^/  | /' "$out/admin.log" >&2; exit 2; }

  # The home directory the recorded commands see, as it is before the tape; the text version
  # starts from the same state, so both are first runs and print the same paths.
  tar -C /home -cf "$env/home.tar" demo

  # The recording. Wall clock: from the start of vhs to its first "Creating" line, which it prints
  # when the tape's last command has run and the encode starts.
  local ran_at started wall
  ran_at="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  started="$(date +%s%N)"
  HOME=/home/demo SHELL=/bin/bash BROWSER=/bin/true PATH="$env/venv/bin:$PATH" \
    vhs /work/src/quickstart.tape < /dev/null 2>&1 | while IFS= read -r line; do
      case "$line" in Creating*) [ -s "$env/recorded-at" ] || date +%s%N > "$env/recorded-at" ;; esac
      printf '%s\n' "$line"
    done
  if [ ! -s "$env/recorded-at" ] || [ ! -s docs/demo/$NAME.gif ]; then
    echo "record-quickstart.sh: vhs wrote no recording; the session log is docs/demo/out/$NAME/session.txt" >&2
    exit 3
  fi
  wall="$(( ($(cat "$env/recorded-at") - started) / 100000000 ))"

  # The text version: the agent is deleted on the server and the home directory restored, then the
  # same lines run once more.
  rm -f "$out/cue-verified"
  : > "$out/req-reset"
  wait_until 60 test -e "$out/cue-reset" \
    || { echo "record-quickstart.sh: the agent was not removed for the text version" >&2; exit 3; }
  find /home/demo -mindepth 1 -delete
  tar -C /home -xf "$env/home.tar"
  HOME=/home/demo NO_COLOR=1 TERM=dumb COLUMNS=110 LINES=28 BROWSER=/bin/true PATH="$env/venv/bin:$PATH" \
    python3 -u /work/src/quickstart-capture.py /work/src/quickstart.tape "$out/capture-steps.json" \
    < /dev/null > "$out/capture.txt" 2>&1 \
    || { echo "record-quickstart.sh: the text version failed; see docs/demo/out/$NAME/capture.txt" >&2; exit 3; }
  : > "$out/req-stop"
  wait "$admin" || true
  kill "$gw" 2>/dev/null || true

  gifsicle -O3 --lossy=40 --batch "docs/demo/$NAME.gif"
  RAN_AT="$ran_at" WALL_TENTHS="$wall" INSTALLED="$installed" \
  FRAMES="$(gifsicle --info "docs/demo/$NAME.gif" | head -1 | grep -oE '[0-9]+ images' | grep -oE '[0-9]+')" \
  GIF_PX="$(ffprobe -v error -select_streams v:0 -show_entries stream=width,height -of csv=p=0 "docs/demo/$NAME.gif")" \
  DURATION="$(ffprobe -v error -show_entries format=duration -of csv=p=0 "docs/demo/out/$NAME.mp4")" \
  VHS_VERSION="$(vhs --version 2>/dev/null | head -1 | sed 's/^vhs version //')" \
  OS_NAME="$(. /etc/os-release && echo "$PRETTY_NAME")" \
  PYTHON_VERSION="$("$env/venv/bin/python3" --version 2>&1)" \
  python3 -c '
import json, os
e = os.environ
width, height = (int(v) for v in e["GIF_PX"].split(","))
print(json.dumps({
    "ranAtUtc": e["RAN_AT"], "wallClockSeconds": int(e["WALL_TENTHS"]) / 10,
    "durationSeconds": round(float(e["DURATION"]), 1), "gifFrames": int(e["FRAMES"]),
    "gifWidth": width, "gifHeight": height, "vhsVersion": e["VHS_VERSION"], "os": e["OS_NAME"],
    "installed": e["INSTALLED"], "python": e["PYTHON_VERSION"],
}, indent=2))' > "$out/render.json"
}

if [ "${1:-}" = "--inside" ]; then
  inside
  exit 0
fi

# ---- on this machine -------------------------------------------------------------------------------

HERE="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$HERE/../.." && pwd)"
DRY_RUN=0
[ "${1:-}" = "--dry-run" ] && DRY_RUN=1
DOCKER="${DOCKER:-docker}"
STAMP="$(date -u +%Y%m%d-%H%M%S)"
PROJECT="aimdemo-${STAMP}"
OUT="$HERE/out/$NAME"
STAGE="$OUT/stage"
RUN=""
COMPOSE=()
log() { printf '%s record-quickstart: %s\n' "$(date -u +%H:%M:%SZ)" "$*" >&2; }
fail() {
  local code=$1; shift
  log "$*"
  if [ -n "$RUN" ] && [ -f "$RUN/compose.env" ]; then
    log "last log lines of the API (fixture stack; the services log no secrets)"
    compose logs --no-log-prefix --tail 20 backend 2>/dev/null | sed 's/^/  | /' >&2 || true
  fi
  echo "{\"result\":\"failed\",\"exit\":$code,\"reason\":\"$*\"}"
  exit "$code"
}

# shellcheck disable=SC1091
. "$HERE/stack/images.env"
SDK_VERSION="$(tr -d '[:space:]' < "$ROOT/sdk/python/VERSION")"
HEAD_SHA="$(git -C "$ROOT" rev-parse HEAD)"
PRERELEASE=true
if git -C "$ROOT" tag --points-at HEAD | grep -qx "sdk-py-v$SDK_VERSION"; then PRERELEASE=false; fi
BASE_ID="$("$DOCKER" image inspect -f '{{.Id}}' "$VHS_BASE_IMAGE" 2>/dev/null || true)"
TERMINAL_IMAGE="aim-demo-terminal:$( { echo "$BASE_ID"; cat "$HERE/terminal/Dockerfile"; } | git hash-object --stdin | cut -c1-12)"
FIXTURE_FILES=(docs/demo/stack/compose.yml docs/demo/stack/images.env docs/demo/lib/gw.mjs docs/demo/quickstart-admin.py)
# Changes to the fixture that the next commit would not carry: an unstaged edit or an untracked
# file counts, a fully staged one does not.
FIXTURE_DIRTY="$(git -C "$ROOT" status --porcelain -- "${FIXTURE_FILES[@]}" | grep -cv '^[MA]  ' || true)"

if [ "$DRY_RUN" = 1 ]; then
  echo "record-quickstart.sh --dry-run: nothing was built, started, rendered or written."
  echo "tape:      docs/demo/$NAME.tape"
  echo "sdk:       aim-sdk $SDK_VERSION built from sdk/python at $HEAD_SHA (prerelease: $PRERELEASE)"
  echo "api:       $AIM_SERVER_IMAGE (revision $AIM_SERVER_REVISION)"
  if [ -n "$BASE_ID" ]; then echo "base:      $VHS_BASE_IMAGE, present"; else echo "base:      $VHS_BASE_IMAGE, NOT FOUND: build it as docs/demo/README.md says"; fi
  if "$DOCKER" image inspect "$TERMINAL_IMAGE" >/dev/null 2>&1; then echo "terminal:  $TERMINAL_IMAGE, present"; else echo "terminal:  $TERMINAL_IMAGE, built from docs/demo/terminal on the first render"; fi
  echo "fixture:   ${FIXTURE_FILES[*]}, $FIXTURE_DIRTY uncommitted changes"
  echo "writes:"
  for f in "$NAME.gif" "$NAME-s1.png" "$NAME-s2.png" "$NAME-s3.png" "$NAME-s4.png" "$NAME-capture.txt" "$NAME.json" "out/$NAME.mp4"; do
    echo "  docs/demo/$f"
  done
  exit 0
fi

if "$DOCKER" compose version >/dev/null 2>&1; then
  COMPOSE=("$DOCKER" compose)
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE=(docker-compose)
else
  fail 2 "neither docker compose nor docker-compose is available"
fi
compose() { "${COMPOSE[@]}" -p "$PROJECT" -f "$HERE/stack/compose.yml" --env-file "$RUN/compose.env" "$@"; }

teardown() {
  local ids
  ids="$("$DOCKER" ps -aq --filter "label=aimdemo.project=$PROJECT" 2>/dev/null || true)"
  [ -z "$ids" ] || "$DOCKER" rm -f $ids >/dev/null 2>&1 || true
  [ -n "$RUN" ] && [ -d "$RUN" ] || return 0
  if [ -f "$RUN/compose.env" ]; then
    compose stop >/dev/null 2>&1 || true
    compose rm -f -v >/dev/null 2>&1 || true
    for v in $("$DOCKER" volume ls -q --filter "name=^${PROJECT}_" 2>/dev/null); do "$DOCKER" volume rm "$v" >/dev/null 2>&1 || true; done
    for n in $("$DOCKER" network ls -q --filter "name=^${PROJECT}_" 2>/dev/null); do "$DOCKER" network rm "$n" >/dev/null 2>&1 || true; done
  fi
  rm -rf "$RUN"
  log "stack $PROJECT removed; run secrets deleted"
}
trap teardown EXIT

# ---- stack safety -----------------------------------------------------------------------------------
if "$DOCKER" ps -a --format '{{.Names}}' | grep -q "^${PROJECT}[-_]"; then fail 4 "project name $PROJECT is already in use"; fi
if grep -qE '^\s*ports:' "$HERE/stack/compose.yml"; then fail 4 "stack/compose.yml must not publish host ports"; fi
if grep -qE 'container_name:' "$HERE/stack/compose.yml"; then fail 4 "stack/compose.yml must not fix container names"; fi
[ -n "$BASE_ID" ] || fail 2 "base image $VHS_BASE_IMAGE missing: build it as docs/demo/README.md says"
command -v node >/dev/null 2>&1 || fail 2 "node is not on PATH; the checks run on this machine"

# ---- run secrets -----------------------------------------------------------------------------------
umask 077
RUN="$(mktemp -d "${TMPDIR:-/tmp}/${PROJECT}.XXXXXX")"
{
  echo "POSTGRES_PASSWORD=$(openssl rand -hex 16)"
  echo "REDIS_PASSWORD=$(openssl rand -hex 16)"
  echo "JWT_SECRET=$(openssl rand -hex 32)"
  echo "KEYVAULT_MASTER_KEY=$(openssl rand -base64 32)"
  echo "ADMIN_PASSWORD=$(openssl rand -base64 18 | tr '/+' 'Aa')Aa1!"
  echo "ADMIN_EMAIL=admin@example.com"
} > "$RUN/.env"
# The file's terminal and browser services need these three to parse; this script starts neither.
{
  cat "$RUN/.env"
  grep -v '^#' "$HERE/stack/images.env"
  echo "DEMO_SLUG=quickstart-selfhosted"
  echo "DEMO_OUT=$OUT"
  echo "DEMO_RUN=$RUN"
} > "$RUN/compose.env"
umask 022
rm -rf "$OUT"
mkdir -p "$STAGE/out"

# ---- images and stack ---------------------------------------------------------------------------------
if ! "$DOCKER" image inspect "$TERMINAL_IMAGE" >/dev/null 2>&1; then
  "$DOCKER" build -q --build-arg "BASE_IMAGE=$VHS_BASE_IMAGE" -t "$TERMINAL_IMAGE" "$HERE/terminal" >&2 \
    || fail 2 "building $TERMINAL_IMAGE from docs/demo/terminal failed"
fi
compose up -d --wait postgres redis backend >&2 || fail 2 "the API did not come up healthy"
log "stack $PROJECT up (API $AIM_SERVER_REVISION)"
# The released server seeds no administrator; its bootstrap binary does. The secrets travel to it
# on stdin, never as arguments, and its output is filtered.
(
  # shellcheck disable=SC1091
  . "$RUN/.env"
  printf 'export DEFAULT_ADMIN_PASSWORD=%q DATABASE_URL=%q; /app/aim-bootstrap --default --admin-email %q --org-name %q </dev/null 2>&1 | grep -vi password | tail -1\n' \
    "$ADMIN_PASSWORD" "postgres://postgres:${POSTGRES_PASSWORD}@postgres:5432/identity?sslmode=disable" "$ADMIN_EMAIL" "Default Organization"
) | compose exec -T backend sh >&2 || fail 2 "the administrator bootstrap failed"
# The bootstrap marks its password as temporary; the fixture administrator has already changed it.
compose exec -T postgres psql -q -U postgres -d identity -c "UPDATE users SET force_password_change = FALSE WHERE email = 'admin@example.com'" >&2 \
  || fail 2 "could not clear the temporary-password flag"
log "fixture administrator admin@example.com created (password generated for this run, never printed)"

# ---- record ----------------------------------------------------------------------------------------------
(
  # shellcheck disable=SC1091
  . "$RUN/.env"
  printf '%s\n%s\n' "$ADMIN_EMAIL" "$ADMIN_PASSWORD"
) | "$DOCKER" run --rm -i --network "${PROJECT}_default" --label "aimdemo.project=$PROJECT" \
    --shm-size=1g -w /work -e SDK_VERSION="$SDK_VERSION" \
    -v "$HERE:/work/src:ro" -v "$ROOT/sdk/python:/work/sdk:ro" -v "$STAGE:/work/docs/demo" -v "$OUT:/work/out" \
    --entrypoint bash "$TERMINAL_IMAGE" /work/src/record-quickstart.sh --inside >&2 \
  || fail 3 "the recording failed; the session log and the administrator's log are in docs/demo/out/$NAME"
log "recorded"

# ---- checks and sidecar ---------------------------------------------------------------------------------
HERE="$HERE" ROOT="$ROOT" OUT="$OUT" STAGE="$STAGE" RUN_ENV="$RUN/.env" SDK_VERSION="$SDK_VERSION" \
PRERELEASE="$PRERELEASE" HEAD_SHA="$HEAD_SHA" FIXTURE_DIRTY="$FIXTURE_DIRTY" FIXTURE_FILES="${FIXTURE_FILES[*]}" \
VHS_BASE_IMAGE="$VHS_BASE_IMAGE" BASE_ID="$BASE_ID" AIM_SERVER_IMAGE="$AIM_SERVER_IMAGE" \
AIM_SERVER_REVISION="$AIM_SERVER_REVISION" POSTGRES_IMAGE="$POSTGRES_IMAGE" REDIS_IMAGE="$REDIS_IMAGE" \
HOST_USER="$(id -un)" HOST_NAME="$(hostname -s)" \
node - <<'NODE' || fail 3 "a check failed; nothing was written to docs/demo (the run's files are in docs/demo/out/quickstart)"
const fs = require("fs"), path = require("path"), crypto = require("crypto");
const e = process.env;
const NAME = "quickstart";
const sha = (f) => crypto.createHash("sha256").update(fs.readFileSync(f)).digest("hex");
const file = (abs, rel) => ({ path: rel, bytes: fs.statSync(abs).size, sha256: sha(abs) });
const read = (f) => fs.readFileSync(f, "utf8");
// Escape sequences out, carriage returns out: the text a viewer reads in the session log.
const plain = (s) => s
  .replace(/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, "")
  .replace(/\x1b\[[0-9;?]*[ -\/]*[@-~]/g, "")
  .replace(/\x1b[()][0-9A-Za-z]/g, "")
  .replace(/\x1b[=>]/g, "")
  .replace(/\r/g, "");
const problems = [];

const tapeFile = path.join(e.HERE, `${NAME}.tape`);
const tape = read(tapeFile);
const narration = JSON.parse(read(path.join(e.HERE, `${NAME}.narration.json`)));
const render = JSON.parse(read(path.join(e.OUT, "render.json")));
const shown = JSON.parse(read(path.join(e.OUT, "capture-steps.json")));
const capture = read(path.join(e.OUT, "capture.txt"));
const session = plain(read(path.join(e.OUT, "session.txt")));

// ---- the tape, read the way vhs plays it
const setting = (name) => (tape.match(new RegExp(`^Set ${name} (\\S+)`, "m")) || [])[1];
const ms = (v) => { const m = /^(\d+(?:\.\d+)?)(ms|s)?$/.exec(v); return m ? Number(m[1]) * (m[2] === "ms" ? 1 : 1000) : NaN; };
let hidden = false, last = null, typedLine = "";
const steps = [], typed = [], preEnter = [];
for (const raw of tape.split("\n")) {
  const line = raw.trim();
  let m;
  if ((m = /^#\s*step:\s*(\S+)\s*(.*)$/.exec(line))) { steps.push({ id: m[1], title: m[2], holdMs: 0, counting: false, shots: [] }); last = null; continue; }
  if (!line || line.startsWith("#")) continue;
  const step = steps[steps.length - 1];
  if (line === "Hide" || line === "Show") hidden = line === "Hide";
  else if ((m = /^Type(@\S+)?\s+(["'`])(.*)\2$/.exec(line))) {
    if (!hidden) { typedLine += m[3]; if (m[1]) problems.push(`P1: a shown Type sets its own speed (${m[1]})`); }
    if (step) step.counting = false;
  } else if ((m = /^Sleep\s+(\S+)$/.exec(line))) {
    if (step && step.counting && !hidden) step.holdMs += ms(m[1]);
  } else if (line === "Enter") {
    if (!hidden) {
      preEnter.push(last && last.startsWith("Sleep ") ? ms(last.slice(6).trim()) : 0);
      typed.push(typedLine);
      typedLine = "";
      if (step) { step.counting = true; step.holdMs = 0; }
    }
  } else if ((m = /^Screenshot\s+(\S+)$/.exec(line)) && step) step.shots.push(m[1]);
  last = line;
}
const commands = typed.filter((t) => t !== "");
const cols = Number(setting("Columns")), rows = Number(setting("Rows"));
const typingMs = ms(setting("TypingSpeed"));

// ---- outcomes the README promises, in the recorded session and in the text version
const expected = [
  ["the sign-in", "Signed in as admin@example.com on http://localhost:8080."],
  ["the registration", "Agent registered: my-first-agent"],
  ["the allowed call's result", "{'name': 'Example Co', 'plan': 'team'}"],
  ["the refusal", "ActionDeniedError: AIM denied 'db:write'"],
];
for (const [what, text] of expected)
  for (const [where, body] of [["recorded session", session], ["text version", capture]])
    if (!body.includes(text)) problems.push(`outcome: the ${where} does not show ${what}: ${text}`);
for (const [where, body] of [["recorded session", session], ["text version", capture]])
  if (/Agent not verified/.test(body)) problems.push(`outcome: the ${where} shows the agent unverified at a call`);
const replayed = shown.map((s) => s.line).filter((t) => t !== "");
if (JSON.stringify(replayed) !== JSON.stringify(commands)) problems.push("capture: the text version did not replay the tape's shown lines in order");

// ---- pace (ruling rows P1, P2, P3, P5, P6, P7, P8)
const words = Object.fromEntries(narration.steps.map((s) => [s.id, s.text.split(/\s+/).filter(Boolean).length]));
const rowsOf = (text) => text === "" ? 0 : text.replace(/\n$/, "").split("\n").reduce((n, l) => n + Math.max(1, Math.ceil(l.length / cols)), 0);
const stepFacts = steps.map((s) => {
  const mine = shown.filter((x) => x.step === s.id);
  const onScreen = Math.min(rows, mine.reduce((n, x) => n + rowsOf(">>> " + x.line) + rowsOf(x.printed || ""), 0));
  const required = Math.max(3, onScreen / 4, (words[s.id] || 0) / 2.5 + 0.5);
  if (!(s.id in words)) problems.push(`narration: no text for step ${s.id}`);
  if (s.holdMs / 1000 + 1e-9 < required) problems.push(`P3: step ${s.id} holds ${s.holdMs / 1000} s, needs ${required.toFixed(2)} s`);
  if (s.holdMs / 1000 > 12) problems.push(`P3: step ${s.id} holds ${s.holdMs / 1000} s, more than 12 s`);
  return { id: s.id, title: s.title, anchorFrame: s.shots[0], holdSeconds: s.holdMs / 1000, requiredHoldSeconds: Number(required.toFixed(2)), linesOnScreen: onScreen, narrationWords: words[s.id] || 0 };
});
const maxLine = Math.max(...commands.map((c) => c.length));
if (!(typingMs >= 100 && typingMs <= 140)) problems.push(`P1: TypingSpeed ${typingMs} ms per character`);
if (maxLine > 72) problems.push(`P1: a shown line has ${maxLine} characters`);
preEnter.forEach((v, i) => { if (!(v >= 600 && v <= 1000)) problems.push(`P2: Enter ${i + 1} is held ${v} ms`); });
if (setting("CursorBlink") !== "false") problems.push("P5: the cursor blinks");
const LIMIT = 90; // a documented multi-step path
if (render.durationSeconds > LIMIT) problems.push(`P6: ${render.durationSeconds} s, more than ${LIMIT} s`);
if (!(cols >= 100 && cols <= 120 && rows >= 24 && rows <= 30)) problems.push(`P7: ${cols} columns by ${rows} rows`);
const font = Number(setting("FontSize"));
if (!(font >= 14 && font <= 16) || setting("Padding") !== "20" || setting("Framerate") !== "10") problems.push("P7: font size, padding or frame rate");
const gifAbs = path.join(e.STAGE, `${NAME}.gif`);
const gifBytes = fs.statSync(gifAbs).size;
if (gifBytes > 3000000) problems.push(`P8: the GIF has ${gifBytes} bytes`);
if (render.gifWidth > 1200) problems.push(`P8: the GIF is ${render.gifWidth} px wide`);

// ---- the sidecar (schema demo-sidecar/2, its keys in order, then this recording's own)
const fixtureFiles = e.FIXTURE_FILES.split(" ");
const fixtureHash = crypto.createHash("sha256");
for (const f of fixtureFiles) fixtureHash.update(fs.readFileSync(path.join(e.ROOT, f)));
const captureRel = `docs/demo/${NAME}-capture.txt`;
const sidecar = {
  schema: "demo-sidecar/2",
  surfaceClass: "terminal",
  tool: "aim-sdk",
  version: e.SDK_VERSION,
  prerelease: e.PRERELEASE === "true",
  script: `docs/demo/${NAME}.tape`,
  scriptSha256: sha(tapeFile),
  recorder: { name: "vhs", version: render.vhsVersion, container: `docs/demo/terminal/Dockerfile on ${e.VHS_BASE_IMAGE} (${render.os})`, baseImageId: e.BASE_ID },
  commands,
  steps: stepFacts,
  fixture: "a local AIM API from docs/demo/stack/compose.yml (its postgres, redis and backend services, images pinned in docs/demo/stack/images.env); administrator admin@example.com with a password generated for the run; the organization set to strict mode",
  fixtureSha: fixtureHash.digest("hex"),
  fixtureFiles,
  fixtureUncommittedChanges: Number(e.FIXTURE_DIRTY),
  ranAtUtc: render.ranAtUtc,
  durationSeconds: render.durationSeconds,
  outputs: {
    gif: { ...file(gifAbs, `docs/demo/${NAME}.gif`), width: render.gifWidth, height: render.gifHeight, frames: render.gifFrames },
    mp4: file(path.join(e.STAGE, "out", `${NAME}.mp4`), `docs/demo/out/${NAME}.mp4`),
    frames: steps.flatMap((s) => s.shots).map((p) => file(path.join(e.STAGE, path.basename(p)), p)),
  },
  capture: captureRel,
  narration: `docs/demo/${NAME}.narration.json`,
  pace: { typingSpeedMs: typingMs, preEnterHoldMs: Math.min(...preEnter), minHoldSeconds: Math.min(...stepFacts.map((s) => s.holdSeconds)), maxLineChars: maxLine },
  exitCodes: shown.filter((s) => "exit" in s).map((s) => s.exit),
  meteredCostUsd: 0,
  lengthClass: "multi-step",
  lengthLimitSeconds: LIMIT,
  build: { package: "aim-sdk[rich]", source: "sdk/python", commit: e.HEAD_SHA, installedVersion: render.installed, python: render.python },
  stack: { compose: "docs/demo/stack/compose.yml", services: ["postgres", "redis", "backend"], aimServer: { image: e.AIM_SERVER_IMAGE, revision: e.AIM_SERVER_REVISION }, postgres: e.POSTGRES_IMAGE, redis: e.REDIS_IMAGE },
  homePath: "/home/demo",
  columns: cols,
  rows,
  fontSize: font,
  framerate: Number(setting("Framerate")),
  gifsicle: "-O3 --lossy=40 --batch",
  edits: [
    "aim-sdk[rich] is installed before the tape from wheels built from this checkout's sdk/python; the README's pip install line is not shown",
    "before the tape, docs/demo/quickstart-admin.py sets the organization's enforcement mode to strict through the API (PUT /api/v1/admin/enforcement-settings), the README's precondition for the refusal",
    "off screen, docs/demo/quickstart-admin.py approves the device code (POST /api/v1/oauth/device/approve, the call the dashboard's /device page makes) and verifies my-first-agent once it is registered (POST /api/v1/agents/:id/verify)",
    "the sample record customers = {42: ...} is defined off screen, with a fictional name in place of the README's",
    "the screen is cleared at each step boundary",
  ],
  wallClockSeconds: render.wallClockSeconds,
};

// ---- census of everything a viewer reads: the session log, the text version, the narration, the sidecar
const secrets = [];
for (const line of read(e.RUN_ENV).split("\n")) {
  const m = line.match(/^([A-Z0-9_]+)=(.*)$/);
  if (m && /PASSWORD|SECRET|KEY|TOKEN/.test(m[1]) && m[2].length >= 8) secrets.push(m[2]);
}
const escape = (s) => s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
const classes = [
  ["jwt", /eyJ[A-Za-z0-9_-]{10,}/, "eyJhbGciOiJIUzI1NiJ9.canary"],
  ["api-key", /aim_live_[A-Za-z0-9]{6,}/, "aim_live_canary0000"],
  ["bearer", /Bearer\s+[A-Za-z0-9._-]{8,}/, "Bearer canarytoken00"],
  ["token-field", /"(refreshToken|accessToken)"\s*:\s*"[^"]{6,}"/, '"refreshToken": "canary000000"'],
  ["local-path", /\/Users\/|\/home\/(?!demo\b)[a-z]|~\/workspace|\/private\/|\/var\/folders\//, ["", "Users", "canary"].join("/")],
  ["email", /[A-Za-z0-9._%+-]+@(?!example\.com\b)[A-Za-z0-9.-]+\.[a-z]{2,}/, "canary@canary.org"],
  // localhost with or without a port, ended by a path, a space, a quote (a JSON string), the end of
  // a line, or a sentence's punctuation (the SDK prints "Signed in as ... on http://localhost:8080.")
  ["host", /https?:\/\/(?!localhost(?::\d+)?(?:[\/\s"'`]|[.,;:)](?:[\s"'`]|$)|$)|github\.com\/opena2a-org|opena2a\.org[\/\s]|pypi\.org|files\.pythonhosted\.org)[a-z0-9.-]+/im, "https://canary.example.net/x"],
  ["public-ip", /\b(?!10\.|127\.|192\.168\.|172\.(?:1[6-9]|2\d|3[01])\.|0\.0\.0\.0)(?:\d{1,3}\.){3}\d{1,3}\b/, "203.0.113.7"],
];
for (const [cls, value] of [["user-name", e.HOST_USER], ["host-name", e.HOST_NAME]])
  if (value && value.length >= 3) classes.push([cls, new RegExp(`\\b${escape(value)}\\b`, "i"), value]);
const scan = (text) => {
  const hits = [];
  if (secrets.some((s) => text.includes(s))) hits.push("run-secret");
  for (const [cls, re] of classes) if (re.test(text)) hits.push(cls);
  return hits;
};
const planted = classes.map((c) => c[2]).join("\n") + (secrets[0] ? "\n" + secrets[0] : "");
const control = scan(planted);
const missed = classes.map((c) => c[0]).filter((c) => !control.includes(c));
if (secrets.length && !control.includes("run-secret")) missed.push("run-secret");
if (missed.length) problems.push(`census: inconclusive, the planted control was not reported for ${missed.join(", ")}`);
const inputs = [["recorded session", session], ["text version", capture], ["narration", JSON.stringify(narration)], ["sidecar", JSON.stringify(sidecar)]];
for (const [where, text] of inputs) for (const cls of scan(text)) problems.push(`census: ${cls} in the ${where}`);
sidecar.census = { inputs: inputs.map((i) => i[0]), classes: classes.length, runSecretsChecked: secrets.length, controlReportedAll: missed.length === 0, hits: problems.filter((p) => p.startsWith("census:")).length };

fs.writeFileSync(path.join(e.STAGE, `${NAME}.json`), JSON.stringify(sidecar, null, 2) + "\n");
if (problems.length) {
  for (const p of problems) console.error("  " + p);
  console.error(`${problems.length} check(s) failed`);
  process.exit(3);
}
console.error(`checks passed: outcomes ${expected.length}, census ${inputs.length} inputs and ${classes.length} classes plus ${secrets.length} run secrets, pace P1 P2 P3 P5 P6 P7 P8`);
NODE

# ---- outputs -------------------------------------------------------------------------------------------------
cp "$OUT/capture.txt" "$HERE/$NAME-capture.txt"
for f in "$NAME.gif" "$NAME-s1.png" "$NAME-s2.png" "$NAME-s3.png" "$NAME-s4.png"; do cp "$STAGE/$f" "$HERE/$f"; done
mkdir -p "$HERE/out"
cp "$STAGE/out/$NAME.mp4" "$HERE/out/$NAME.mp4"
cp "$STAGE/$NAME.json" "$HERE/$NAME.json"
node -e '
const s = JSON.parse(require("fs").readFileSync(process.argv[1], "utf8"));
console.log(JSON.stringify({ result: "rendered", sidecar: "docs/demo/quickstart.json", version: s.version, prerelease: s.prerelease,
  durationSeconds: s.durationSeconds, gifBytes: s.outputs.gif.bytes, gifSha256: s.outputs.gif.sha256, wallClockSeconds: s.wallClockSeconds }));
' "$HERE/$NAME.json"
