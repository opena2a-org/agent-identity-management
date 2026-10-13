"""How the A2A demo's Python scripts reach the local AIM stack.

Each value comes from the environment first, then from the repository's .env
(the file docker compose reads for docker-compose.yml), then from the default
the compose files use:

    AIM_BACKEND_PORT=8080    the backend's host port
    AIM_FRONTEND_PORT=3000   the dashboard's host port

Sets BACKEND_URL and FRONTEND_URL. run_demo.py takes the backend from
--aim-url or AIM_URL first, when one is given.
"""

import os
from pathlib import Path

_DOTENV = Path(__file__).resolve().parent.parent.parent.parent / ".env"


def _dotenv_value(name):
    """The last value the repository's .env assigns to name, or ""."""
    try:
        lines = _DOTENV.read_text().splitlines()
    except OSError:
        return ""
    prefix = name + "="
    value = ""
    for line in lines:
        if line.startswith(prefix):
            value = line[len(prefix):]
    return value


def stack_value(name, default):
    """name from the environment, then the repository's .env, then default."""
    return os.environ.get(name) or _dotenv_value(name) or default


BACKEND_URL = f"http://localhost:{stack_value('AIM_BACKEND_PORT', '8080')}"
FRONTEND_URL = f"http://localhost:{stack_value('AIM_FRONTEND_PORT', '3000')}"
