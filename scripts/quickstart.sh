#!/usr/bin/env bash
set -euo pipefail

# AIM Quickstart — one command to run the full stack
# Usage: curl -sSL https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/scripts/quickstart.sh | bash

COMPOSE_URL="https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/docker-compose.quickstart.yml"
INSTALL_DIR="${AIM_DIR:-aim}"

info()  { printf '\033[1;34m[AIM]\033[0m %s\n' "$*"; }
error() { printf '\033[1;31m[AIM]\033[0m %s\n' "$*" >&2; exit 1; }

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
  (umask 077 && printf 'POSTGRES_PASSWORD=%s\nREDIS_PASSWORD=%s\nJWT_SECRET=%s\nKEYVAULT_MASTER_KEY=%s\nADMIN_EMAIL=admin@localhost\nADMIN_PASSWORD=%s\n' \
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
    error "Backend did not become healthy. Check logs: docker compose -f docker-compose.quickstart.yml logs backend"
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
  printf '  Login:      admin@localhost / %s\n' "$admin_pass"
  printf '              (change this password on first login)\n'
else
  printf '  Login:      credentials from initial setup (see .env)\n'
fi
printf '\n'
printf '  Stop:       cd %s && docker compose -f docker-compose.quickstart.yml stop\n' "$INSTALL_DIR"
printf '  Logs:       cd %s && docker compose -f docker-compose.quickstart.yml logs -f\n' "$INSTALL_DIR"
printf '\n'
