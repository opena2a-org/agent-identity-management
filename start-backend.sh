#!/bin/bash
set -a
[ -f .env ] && source .env
set +a
cd apps/backend
exec go run cmd/server/main.go
