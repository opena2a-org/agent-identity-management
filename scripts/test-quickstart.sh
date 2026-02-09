#!/usr/bin/env bash
set -euo pipefail

# Test the quickstart flow with locally built backend
# Usage: ./scripts/test-quickstart.sh

# Docker Desktop on macOS
export PATH="$HOME/.docker/bin:$PATH"

DIR="/tmp/aim-quickstart-test"
REPO="$(cd "$(dirname "$0")/.." && pwd)"

info()  { printf '\033[1;34m[TEST]\033[0m %s\n' "$*"; }
error() { printf '\033[1;31m[TEST]\033[0m %s\n' "$*" >&2; exit 1; }

# Pre-flight
command -v docker >/dev/null 2>&1 || error "Docker not found"
docker info >/dev/null 2>&1 || error "Docker daemon not running"

# Clean previous test
if [ -d "$DIR" ]; then
  info "Cleaning previous test..."
  docker compose -f "$DIR/docker-compose.quickstart.yml" --env-file "$DIR/.env" down -v 2>/dev/null || true
  rm -rf "$DIR"
fi

# Build backend with local code changes
info "Building backend image..."
docker build -q -t aim-server:test \
  -f "$REPO/apps/backend/infrastructure/docker/Dockerfile.backend" \
  "$REPO" >/dev/null

# Setup test directory
mkdir -p "$DIR"
cp "$REPO/docker-compose.quickstart.yml" "$DIR/"

# Swap in local backend image
sed -i.bak 's|ghcr.io/opena2a-org/aim-server:latest|aim-server:test|' "$DIR/docker-compose.quickstart.yml"
rm -f "$DIR/docker-compose.quickstart.yml.bak"

# Generate secrets
admin_pass=$(openssl rand -base64 18)
(umask 077 && printf 'POSTGRES_PASSWORD=%s\nREDIS_PASSWORD=%s\nJWT_SECRET=%s\nKEYVAULT_MASTER_KEY=%s\nADMIN_EMAIL=admin@opena2a.org\nADMIN_PASSWORD=%s\n' \
  "$(openssl rand -hex 16)" "$(openssl rand -hex 16)" "$(openssl rand -hex 32)" "$(openssl rand -base64 32)" "$admin_pass" > "$DIR/.env")

# Start
info "Starting services..."
docker compose -f "$DIR/docker-compose.quickstart.yml" --env-file "$DIR/.env" up -d

# Wait for backend
info "Waiting for backend..."
retries=0
until curl -sf http://localhost:8080/health >/dev/null 2>&1; do
  retries=$((retries + 1))
  [ "$retries" -ge 30 ] && error "Backend timeout"
  sleep 2
done

# Done
printf '\n'
info "Ready!"
printf '\n'
printf '  Dashboard:  \033[1;32mhttp://localhost:3000\033[0m\n'
printf '  API:        \033[1;32mhttp://localhost:8080\033[0m\n'
printf '  Login:      admin@opena2a.org / %s\n' "$admin_pass"
printf '              (change this password on first login)\n'
printf '\n'
printf '  Teardown:   docker compose -f %s/docker-compose.quickstart.yml --env-file %s/.env down -v\n' "$DIR" "$DIR"
printf '\n'

# Open browser
if command -v open >/dev/null 2>&1; then
  open http://localhost:3000
fi
