#!/usr/bin/env bash
set -euo pipefail

# AIM Quickstart — one command to run the full stack
#
# Recommended (download, inspect, then run — no pipe to shell):
#   curl -sSLO https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/scripts/quickstart.sh
#   bash quickstart.sh
#
# We deliberately do NOT recommend piping a network download straight
# into a shell interpreter: a compromised or MITM-ed mirror could inject
# anything between the download and the interpreter, and there is no
# opportunity to inspect or checksum the payload first.

COMPOSE_URL="https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/docker-compose.quickstart.yml"
INSTALL_DIR="${AIM_DIR:-aim}"

info()  { printf '\033[1;34m[AIM]\033[0m %s\n' "$*"; }
error() { printf '\033[1;31m[AIM]\033[0m %s\n' "$*" >&2; exit 1; }

# Compose names each volume <project>_<volume>. The project is
# COMPOSE_PROJECT_NAME when set, otherwise the install directory's base name,
# lower-cased, with every character outside [a-z0-9_-] dropped.
compose_project() {
  local name="${COMPOSE_PROJECT_NAME:-}"
  if [ -z "$name" ]; then
    if [ -d "$INSTALL_DIR" ]; then
      name=$(basename "$(cd "$INSTALL_DIR" && pwd)")
    else
      name=$(basename "$INSTALL_DIR")
    fi
  fi
  printf '%s' "$name" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9_-' | sed 's/^[-_]*//'
}

# --- Validate install directory ---
case "$INSTALL_DIR" in
  /*|*..*)  error "AIM_DIR must be a relative path without '..': $INSTALL_DIR" ;;
esac

# --- Pre-flight checks ---
for cmd in docker openssl curl; do
  command -v "$cmd" >/dev/null 2>&1 || error "Required command not found: $cmd"
done

docker info >/dev/null 2>&1 || error "Docker daemon is not running"

# --- Setup directory ---
if [ -d "$INSTALL_DIR" ] && [ -f "$INSTALL_DIR/docker-compose.quickstart.yml" ]; then
  info "Existing installation found in ./$INSTALL_DIR"
  info "Starting services..."
  cd "$INSTALL_DIR"
  docker compose -f docker-compose.quickstart.yml up -d
else
  # --- Stop before writing anything if an earlier install's database remains ---
  # The postgres image applies POSTGRES_PASSWORD only to an empty data
  # directory, so the .env this run would generate cannot connect to a volume
  # an earlier install initialised. The volume is the user's data: never remove it.
  project=$(compose_project)
  pg_volume="${project}_postgres_data"
  if [ -n "$project" ] && docker volume inspect "$pg_volume" >/dev/null 2>&1; then
    old_volumes="$pg_volume"
    if docker volume inspect "${project}_redis_data" >/dev/null 2>&1; then
      old_volumes="$old_volumes ${project}_redis_data"
    fi
    old_containers=$(for v in $old_volumes; do
      docker ps -a --filter "volume=$v" --format '{{.Names}}' 2>/dev/null || true
    done | sort -u | tr '\n' ' ' | sed 's/ *$//')

    info "Found Docker volume $pg_volume, the database of an earlier AIM install."
    info "It accepts only the POSTGRES_PASSWORD from the .env of the install that created it,"
    info "so the new .env this run would generate cannot connect to it. Nothing has been changed."
    printf '\n'
    printf '  Keep the data:  put the earlier .env and docker-compose.quickstart.yml back in ./%s,\n' "$INSTALL_DIR"
    printf '                  then run this script again.\n'
    printf '\n'
    if [ -n "$old_containers" ]; then
      printf '  Start over:     docker rm -f %s\n' "$old_containers"
      printf '                  docker volume rm %s\n' "$old_volumes"
    else
      printf '  Start over:     docker volume rm %s\n' "$old_volumes"
    fi
    printf '                  then run this script again. This deletes every agent, user and event\n'
    printf '                  in that database.\n'
    printf '\n'
    error "Stopped before setting up ./$INSTALL_DIR."
  fi

  info "Setting up AIM in ./$INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
  cd "$INSTALL_DIR"

  # Download compose file
  curl -sSL "$COMPOSE_URL" -o docker-compose.quickstart.yml

  # Generate secrets into variables (avoids exposing values in process args)
  pg_pass=$(openssl rand -hex 16)
  redis_pass=$(openssl rand -hex 16)
  jwt_secret=$(openssl rand -hex 32)
  kv_key=$(openssl rand -base64 32)
  admin_pass=$(openssl rand -base64 18)

  # Write .env from variables (restricted permissions)
  (umask 077 && printf 'POSTGRES_PASSWORD=%s\nREDIS_PASSWORD=%s\nJWT_SECRET=%s\nKEYVAULT_MASTER_KEY=%s\nADMIN_EMAIL=admin@opena2a.org\nADMIN_PASSWORD=%s\n' \
    "$pg_pass" "$redis_pass" "$jwt_secret" "$kv_key" "$admin_pass" > .env)

  info "Generated .env with random secrets"
  info "Pulling images and starting services..."
  docker compose -f docker-compose.quickstart.yml up -d
fi

# --- Wait for backend health ---
info "Waiting for backend to be ready..."
retries=0
max_retries=30
until curl -sf http://localhost:8080/health >/dev/null 2>&1; do
  retries=$((retries + 1))
  if [ "$retries" -ge "$max_retries" ]; then
    printf '\033[1;31m[AIM]\033[0m %s\n' "Backend did not become healthy. Check logs: docker compose -f docker-compose.quickstart.yml logs backend" >&2
    printf '      %s\n' \
      'If the log shows "password authentication failed", the database volume was created' \
      'by an earlier install with a different POSTGRES_PASSWORD. List volumes: docker volume ls' >&2
    exit 1
  fi
  sleep 2
done

# --- Done ---
printf '\n'
info "AIM is running!"
printf '\n'
printf '  Dashboard:  \033[1;32mhttp://localhost:3000\033[0m\n'
printf '  API:        \033[1;32mhttp://localhost:8080\033[0m\n'
if [ -n "${admin_pass:-}" ]; then
  printf '  Login:      admin@opena2a.org / %s\n' "$admin_pass"
  printf '              (change this password on first login)\n'
else
  printf '  Login:      credentials from initial setup (see .env)\n'
fi
printf '\n'
printf '  Stop:       cd %s && docker compose -f docker-compose.quickstart.yml stop\n' "$INSTALL_DIR"
printf '  Logs:       cd %s && docker compose -f docker-compose.quickstart.yml logs -f\n' "$INSTALL_DIR"
printf '\n'
