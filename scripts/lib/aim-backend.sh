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
#       and pg_sslmode;
#   aim_backend_start WORK_PREFIX [NAME=VALUE ...]
#       generates JWT_SECRET, KEYVAULT_MASTER_KEY and the admin password
#       (printing none of them), builds cmd/server, cmd/migrate and
#       cmd/bootstrap, runs the migrations, seeds the admin, starts the server
#       on a free port and waits for /health/ready. Each NAME=VALUE is added
#       to the variables the backend processes get; a setting such as
#       ENVIRONMENT is the caller's to choose, never this file's.
# After aim_backend_start: aim_url, admin_email, admin_password, work (a fresh
# directory under TMPDIR), logs ($work/logs) and $work/home exist. The EXIT
# trap stops the server whatever the outcome and keeps $logs unless the caller
# sets status=0 before it exits.

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
  rm -rf "${work:?}/bin" "${work:?}/home"
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
}
