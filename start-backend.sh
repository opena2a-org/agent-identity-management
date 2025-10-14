#!/bin/bash
set -a
source .env
set +a
exec ./bin/server
