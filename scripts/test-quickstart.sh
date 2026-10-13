#!/usr/bin/env bash
set -euo pipefail

# Test the quickstart flow with locally built backend
# Usage: ./scripts/test-quickstart.sh
#
# Each run works in its own directory from mktemp and under its own Compose
# project, so two runs on one host do not share containers, volumes or ports,
# and a run never tears down anything it did not create. Host ports come from
# the environment with the quickstart's defaults:
#   AIM_POSTGRES_PORT=5432  AIM_REDIS_PORT=6379  AIM_BACKEND_PORT=8080  AIM_FRONTEND_PORT=3000
# The run prints the teardown command for the stack it started.

# Docker Desktop on macOS
export PATH="$HOME/.docker/bin:$PATH"

REPO="$(cd "$(dirname "$0")/.." && pwd)"

BACKEND_PORT="${AIM_BACKEND_PORT:-8080}"
FRONTEND_PORT="${AIM_FRONTEND_PORT:-3000}"
POSTGRES_PORT="${AIM_POSTGRES_PORT:-5432}"
REDIS_PORT="${AIM_REDIS_PORT:-6379}"

DIR=""
teardown_hint() {
  if [ -n "$DIR" ] && [ -f "$DIR/.env" ]; then
    printf '  Teardown:   docker compose -f %s/docker-compose.quickstart.yml --env-file %s/.env down -v\n' "$DIR" "$DIR"
  fi
}

info()  { printf '\033[1;34m[TEST]\033[0m %s\n' "$*"; }
error() { printf '\033[1;31m[TEST]\033[0m %s\n' "$*" >&2; teardown_hint >&2; exit 1; }

# Pre-flight
command -v docker >/dev/null 2>&1 || error "Docker not found"
docker info >/dev/null 2>&1 || error "Docker daemon not running"

# Build backend with local code changes
info "Building backend image..."
docker build -q -t aim-server:test \
  -f "$REPO/infrastructure/docker/Dockerfile.backend" \
  "$REPO" >/dev/null

# Setup test directory: a fresh one for this run, never a path a previous run used
DIR="$(mktemp -d "${TMPDIR:-/tmp}/aim-qs.XXXXXX")"
project="aim-test-$(basename "$DIR" | tr '[:upper:]' '[:lower:]' | tr -cd 'a-z0-9' | tail -c 6)"
cp "$REPO/docker-compose.quickstart.yml" "$DIR/"

# Swap in local backend image
sed -i.bak 's|ghcr.io/opena2a-org/aim-server:latest|aim-server:test|' "$DIR/docker-compose.quickstart.yml"
rm -f "$DIR/docker-compose.quickstart.yml.bak"

# Generate secrets; the project name and host ports go in the same .env so the
# teardown command below addresses exactly this run's containers and volumes.
admin_pass=$(openssl rand -base64 18)
(umask 077 && printf 'POSTGRES_PASSWORD=%s\nREDIS_PASSWORD=%s\nJWT_SECRET=%s\nKEYVAULT_MASTER_KEY=%s\nADMIN_EMAIL=admin@opena2a.org\nADMIN_PASSWORD=%s\nCOMPOSE_PROJECT_NAME=%s\nAIM_PROJECT=%s\nAIM_POSTGRES_PORT=%s\nAIM_REDIS_PORT=%s\nAIM_BACKEND_PORT=%s\nAIM_FRONTEND_PORT=%s\n' \
  "$(openssl rand -hex 16)" "$(openssl rand -hex 16)" "$(openssl rand -hex 32)" "$(openssl rand -base64 32)" "$admin_pass" \
  "$project" "$project" "$POSTGRES_PORT" "$REDIS_PORT" "$BACKEND_PORT" "$FRONTEND_PORT" > "$DIR/.env")

# Start
info "Starting services (project $project)..."
docker compose -f "$DIR/docker-compose.quickstart.yml" --env-file "$DIR/.env" up -d

# Wait for backend
info "Waiting for backend..."
retries=0
until curl -sf "http://localhost:$BACKEND_PORT/health" >/dev/null 2>&1; do
  retries=$((retries + 1))
  [ "$retries" -ge 30 ] && error "Backend timeout"
  sleep 2
done

# Done
printf '\n'
info "Ready!"
printf '\n'
printf '  Dashboard:  \033[1;32mhttp://localhost:%s\033[0m\n' "$FRONTEND_PORT"
printf '  API:        \033[1;32mhttp://localhost:%s\033[0m\n' "$BACKEND_PORT"
printf '  Login:      admin@opena2a.org / %s\n' "$admin_pass"
printf '              (change this password on first login)\n'
printf '\n'
teardown_hint
printf '\n'

# Open browser
if command -v open >/dev/null 2>&1; then
  open "http://localhost:$FRONTEND_PORT"
fi
