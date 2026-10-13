# shellcheck shell=bash disable=SC2034  # sourced; its callers read the values
# Sourced by reset-demo.sh, prep-stage.sh and check_sdk_token.sh: how the demo
# reaches the local AIM stack (stack_env.py does the same for the Python
# scripts). Each value comes from the environment first, then from the
# repository's .env (the file docker compose reads for docker-compose.yml),
# then from the default the compose files use:
#
#   AIM_PROJECT=aim         prefix of the container names (PostgreSQL runs as
#                           $AIM_PROJECT-postgres)
#   AIM_BACKEND_PORT=8080   the backend's host port
#   AIM_POSTGRES_PORT=5432  PostgreSQL's host port
#
# Sets PG_CONTAINER and AIM_BACKEND_URL.

_stack_dotenv="$(dirname "${BASH_SOURCE[0]}")/../../.env"

# _stack_dotenv_value NAME prints the last value the repository's .env assigns
# to NAME, or nothing.
_stack_dotenv_value() {
  [ -f "$_stack_dotenv" ] || return 0
  sed -n "s/^$1=//p" "$_stack_dotenv" | tail -n 1
}

AIM_PROJECT="${AIM_PROJECT:-$(_stack_dotenv_value AIM_PROJECT)}"
AIM_PROJECT="${AIM_PROJECT:-aim}"
AIM_BACKEND_PORT="${AIM_BACKEND_PORT:-$(_stack_dotenv_value AIM_BACKEND_PORT)}"
AIM_BACKEND_PORT="${AIM_BACKEND_PORT:-8080}"
AIM_POSTGRES_PORT="${AIM_POSTGRES_PORT:-$(_stack_dotenv_value AIM_POSTGRES_PORT)}"
AIM_POSTGRES_PORT="${AIM_POSTGRES_PORT:-5432}"

PG_CONTAINER="${AIM_PROJECT}-postgres"
AIM_BACKEND_URL="http://localhost:${AIM_BACKEND_PORT}"
