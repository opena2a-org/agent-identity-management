# aim-backend.sh — start a disposable AIM backend for an SDK integration run.
#
# Sourced by scripts/test-sdk-python-framework-integration.sh and
# scripts/test-sdk-java-helpers-integration.sh; not run on its own. Before
# sourcing, the caller sets:
#   aim_backend_name  the prefix of every message (the calling script's name)
#   repo_root         this repository's root
# and then calls, in this order:
#   aim_backend_refuse_stored_sign_in DIR SDK_SUBDIR
#       stops the run when DIR or a directory above it holds a .aim entry;
#   aim_backend_parse_database_url
#       reads DATABASE_URL into pg_user, pg_password, pg_host, pg_port, pg_db
#       and pg_sslmode, the user, the password and the database name
#       percent-decoded;
#   aim_backend_start WORK_PREFIX [NAME=VALUE ...]
#       generates JWT_SECRET, KEYVAULT_MASTER_KEY and the admin password
#       (printing none of them), builds cmd/server, cmd/migrate and
#       cmd/bootstrap, runs the migrations, seeds the admin, starts the server
#       on a free port, waits for /health/ready and completes the admin's
#       required password change (aim_backend_complete_password_change,
#       below). Each NAME=VALUE is added to the variables the backend
#       processes get; a setting such as ENVIRONMENT is the caller's to
#       choose, never this file's.
# After aim_backend_start: aim_url, admin_email, admin_password (the password
# the account holds after the change, which the tests sign in with), work (a
# fresh directory under TMPDIR), logs ($work/logs) and $work/home exist. The
# EXIT trap stops the server whatever the outcome and keeps $logs unless the
# caller sets status=0 before it exits.

backend_dir="$repo_root/apps/backend"
admin_email="admin@opena2a.org"
server_pid=""
status=1

fail() {
  echo "$aim_backend_name: $*" >&2
  exit 1
}

aim_backend_refuse_stored_sign_in() {
  local dir="$1" sdk_subdir="$2"
  while :; do
    if [ -e "$dir/.aim" ]; then
      fail "refusing to run: $dir holds a .aim entry, and the SDK under test reads a stored sign-in from a .aim entry in its working directory or any directory above it.
  Verify: ls -d '$dir/.aim'
  Fix: run the tests from a clone outside it: d=\"\$(mktemp -d)\" && git clone -q '$repo_root' \"\$d/aim\" && '$repo_root/scripts/$aim_backend_name.sh' \"\$d/aim/$sdk_subdir\""
    fi
    [ "$dir" = "/" ] && break
    dir="$(dirname "$dir")"
  done
}

# aim_backend_percent_decode NAME PART VALUE sets the variable NAME to VALUE
# with each %XX replaced by the byte it encodes. A % that does not begin a %XX
# stops the run, as it makes lib/pq refuse the URL, and so does %00, a NUL
# byte, which no environment variable can carry. PART names the value in the
# message, which holds none of it.
aim_backend_percent_decode() {
  local name="$1" part="$2" rest="$3" decoded="" byte
  while [[ "$rest" == *%* ]]; do
    decoded+="${rest%%\%*}"
    rest="${rest#*%}"
    [[ "$rest" =~ ^[0-9A-Fa-f]{2} ]] \
      || fail "the $part in DATABASE_URL holds a % that does not begin a percent-encoded byte (%XX); write a % in it as %25"
    [ "${rest:0:2}" != "00" ] \
      || fail "the $part in DATABASE_URL holds %00, and an environment variable cannot carry a NUL byte"
    printf -v byte '%b' "\\x${rest:0:2}"
    decoded+="$byte"
    rest="${rest:2}"
  done
  printf -v "$name" '%s' "$decoded$rest"
}

aim_backend_parse_database_url() {
  [ -n "${DATABASE_URL:-}" ] || fail "DATABASE_URL is not set"
  local url_pattern='^postgres(ql)?://([^:@/]+)(:([^@]*))?@([^:/?]+)(:([0-9]+))?/([^?]+)(\?(.*))?$'
  [[ "$DATABASE_URL" =~ $url_pattern ]] || fail "DATABASE_URL is not of the form postgres://<user>[:<password>]@<host>[:<port>]/<db>[?<options>]"
  pg_user="${BASH_REMATCH[2]}"
  pg_password="${BASH_REMATCH[4]}"
  pg_host="${BASH_REMATCH[5]}"
  pg_port="${BASH_REMATCH[7]:-5432}"
  pg_db="${BASH_REMATCH[8]}"
  pg_options="${BASH_REMATCH[10]}"
  # cmd/migrate opens DATABASE_URL itself, and lib/pq percent-decodes its user,
  # password and database name. aim-bootstrap and the server read them from
  # POSTGRES_USER, POSTGRES_PASSWORD and POSTGRES_DB as they are, so those get
  # the decoded values.
  aim_backend_percent_decode pg_user user "$pg_user"
  aim_backend_percent_decode pg_password password "$pg_password"
  aim_backend_percent_decode pg_db "database name" "$pg_db"
  pg_sslmode="disable"
  if [[ "&$pg_options" =~ \&sslmode=([^&]+) ]]; then
    pg_sslmode="${BASH_REMATCH[1]}"
  fi
}

aim_backend_cleanup() {
  if [ -n "$server_pid" ] && kill -0 "$server_pid" 2>/dev/null; then
    kill "$server_pid" 2>/dev/null || true
    wait "$server_pid" 2>/dev/null || true
  fi
  rm -rf "${work:?}/bin" "${work:?}/home" "${work:?}/auth"
  if [ "$status" -eq 0 ]; then
    rm -rf "${work:?}"
  else
    echo "$aim_backend_name: logs kept in $logs" >&2
  fi
}

aim_backend_start() {
  local work_prefix="$1" assignment
  shift
  for assignment in "$@"; do
    [[ "$assignment" =~ ^[A-Za-z_][A-Za-z0-9_]*= ]] || fail "aim_backend_start: $assignment is not NAME=VALUE"
  done

  # --- Ephemeral secrets (never printed) -----------------------------------

  # JWT_SECRET: at least 32 characters. KEYVAULT_MASTER_KEY: base64 of exactly
  # 32 bytes. The admin password must hold upper, lower, digit and special.
  local jwt_secret keyvault_master_key
  jwt_secret="$(openssl rand -hex 32)"
  keyvault_master_key="$(openssl rand -base64 32)"
  admin_password="$(openssl rand -base64 18 | tr -d '/+=' | head -c 24)Aa1!"

  work="$(mktemp -d "${TMPDIR:-/tmp}/$work_prefix.XXXXXX")"
  logs="$work/logs"
  mkdir -p "$logs" "$work/bin" "$work/home"
  trap aim_backend_cleanup EXIT

  # The backend processes get exactly these variables and the caller's,
  # nothing inherited.
  local backend_env=(
    env -i
    PATH="$PATH"
    HOME="$work/home"
    DATABASE_URL="$DATABASE_URL"
    POSTGRES_HOST="$pg_host"
    POSTGRES_PORT="$pg_port"
    POSTGRES_USER="$pg_user"
    POSTGRES_PASSWORD="$pg_password"
    POSTGRES_DB="$pg_db"
    POSTGRES_SSL_MODE="$pg_sslmode"
    JWT_SECRET="$jwt_secret"
    KEYVAULT_MASTER_KEY="$keyvault_master_key"
    DEFAULT_ADMIN_PASSWORD="$admin_password"
    "$@"
  )

  # --- Backend -------------------------------------------------------------

  echo "Building the backend"
  (cd "$backend_dir" && go build -o "$work/bin/aim-server" ./cmd/server \
    && go build -o "$work/bin/aim-migrate" ./cmd/migrate \
    && go build -o "$work/bin/aim-bootstrap" ./cmd/bootstrap) \
    || fail "the backend did not build"

  # cmd/migrate reads the migrations directory relative to apps/backend.
  echo "Running migrations"
  (cd "$backend_dir" && "${backend_env[@]}" "$work/bin/aim-migrate" >"$logs/migrate.log" 2>&1) \
    || fail "migrations failed (see $logs/migrate.log)"

  echo "Seeding the admin"
  (cd "$backend_dir" && "${backend_env[@]}" "$work/bin/aim-bootstrap" --default --admin-email "$admin_email" \
    >"$logs/bootstrap.log" 2>&1) \
    || fail "aim-bootstrap --default failed (see $logs/bootstrap.log)"

  local port="" candidate
  for _ in $(seq 1 50); do
    candidate=$((20000 + RANDOM % 10000))
    if ! (exec 3<>"/dev/tcp/127.0.0.1/$candidate") 2>/dev/null; then
      port="$candidate"
      break
    fi
  done
  [ -n "$port" ] || fail "found no free port in 20000-29999"
  aim_url="http://127.0.0.1:$port"

  echo "Starting the backend on port $port"
  cd "$backend_dir"
  "${backend_env[@]}" APP_PORT="$port" "$work/bin/aim-server" >"$logs/server.log" 2>&1 &
  server_pid=$!
  cd "$repo_root"

  local ready=""
  for _ in $(seq 1 90); do
    kill -0 "$server_pid" 2>/dev/null || fail "the backend exited before it was ready (see $logs/server.log)"
    if curl -fsS "$aim_url/health/ready" >/dev/null 2>&1; then
      ready=1
      break
    fi
    sleep 1
  done
  [ -n "$ready" ] || fail "the backend was not ready within 90s (see $logs/server.log)"
  echo "Backend ready at $aim_url"

  # --- The admin's required password change --------------------------------

  aim_backend_complete_password_change
}

# aim_backend_post_json PATH BODY_FILE OUT [HEADER_FILE] posts the JSON in
# BODY_FILE to the backend at PATH, writes the answer's body to OUT and prints
# the HTTP status. HEADER_FILE, when given, holds request headers, one per
# line, which curl reads itself (-H @file), so a bearer token is on no command
# line. A request that gets no answer fails the call.
aim_backend_post_json() {
  local path="$1" body_file="$2" out="$3" header_file="${4:-}"
  local -a args=(-sS -o "$out" -w '%{http_code}' -X POST -H 'Content-Type: application/json' --data-binary "@$body_file")
  if [ -n "$header_file" ]; then
    args+=(-H "@$header_file")
  fi
  curl "${args[@]}" "$aim_url$path"
}

# aim_backend_json_string NAME KEY FILE sets NAME to the first string member
# KEY of the JSON in FILE, or to the empty string when there is none. The
# member read here, accessToken, holds no quote.
aim_backend_json_string() {
  local name="$1" key="$2" file="$3" value
  value="$(sed -nE 's/.*"'"$key"'"[[:space:]]*:[[:space:]]*"([^"]*)".*/\1/p' "$file" | head -n 1)"
  printf -v "$name" '%s' "$value"
}

# aim_backend_complete_password_change completes the required password change
# of the admin aim-bootstrap --default seeds: while users.force_password_change
# is set, the API admits the account's session only to GET /api/v1/auth/me and
# POST /api/v1/auth/change-password, and every other route answers 403
# passwordChangeRequired. It signs in with the seeded password, changes the
# password through POST /api/v1/auth/change-password to a second generated
# one, which clears the requirement, sets admin_password to it, waits for the
# next whole second (token times are whole seconds, and a token minted in the
# second of a change is refused), and proves that a sign-in with it is
# admitted and requires no change. Request bodies and the bearer token go
# through files under $work/auth, mode 0700, removed when the function returns
# and by the EXIT trap; no password or token is on a command line or printed.
aim_backend_complete_password_change() {
  local auth_dir="$work/auth" new_password status token
  echo "Completing the admin's required password change"
  mkdir -m 700 "$auth_dir"
  # The same alphabet as the seeded password: neither holds a character JSON
  # escapes, and both satisfy the server's rule (upper, lower, digit, special).
  new_password="$(openssl rand -base64 18 | tr -d '/+=' | head -c 24)Bb2!"

  (umask 077 && printf '{"email":"%s","password":"%s"}' "$admin_email" "$admin_password" >"$auth_dir/sign-in.json")
  status="$(aim_backend_post_json /api/v1/auth/login/local "$auth_dir/sign-in.json" "$auth_dir/sign-in-answer.json")" \
    || fail "POST /api/v1/auth/login/local got no answer"
  [ "$status" = 200 ] \
    || fail "the seeded admin's sign-in, POST /api/v1/auth/login/local, answered $status: $(head -c 300 "$auth_dir/sign-in-answer.json")"
  aim_backend_json_string token accessToken "$auth_dir/sign-in-answer.json"
  [ -n "$token" ] || fail "the seeded admin's sign-in answered 200 without accessToken"

  (umask 077 && printf 'Authorization: Bearer %s\n' "$token" >"$auth_dir/authorization.header" \
    && printf '{"currentPassword":"%s","newPassword":"%s"}' "$admin_password" "$new_password" >"$auth_dir/change.json")
  status="$(aim_backend_post_json /api/v1/auth/change-password "$auth_dir/change.json" "$auth_dir/change-answer.json" "$auth_dir/authorization.header")" \
    || fail "POST /api/v1/auth/change-password got no answer"
  [ "$status" = 200 ] \
    || fail "POST /api/v1/auth/change-password answered $status: $(head -c 300 "$auth_dir/change-answer.json")"
  admin_password="$new_password"

  # The next whole second; 2 where sleep takes whole seconds only.
  sleep 2
  (umask 077 && printf '{"email":"%s","password":"%s"}' "$admin_email" "$admin_password" >"$auth_dir/sign-in.json")
  status="$(aim_backend_post_json /api/v1/auth/login/local "$auth_dir/sign-in.json" "$auth_dir/sign-in-answer.json")" \
    || fail "POST /api/v1/auth/login/local got no answer"
  [ "$status" = 200 ] \
    || fail "the admin's sign-in with the changed password answered $status: $(head -c 300 "$auth_dir/sign-in-answer.json")"
  if grep -Eq '"forcePasswordChange"[[:space:]]*:[[:space:]]*true' "$auth_dir/sign-in-answer.json"; then
    fail "the admin's account still requires a password change after POST /api/v1/auth/change-password answered 200"
  fi
  rm -rf "$auth_dir"
  echo "The admin's password change is complete"
}
