# LangChain CRUD Agent — AIM SDK integration

A LangChain agent that performs CRUD operations on a todo list, with each operation secured by AIM's `@perform_action` decorator. Demonstrates that AIM integrates with the LangChain agent runtime without rewriting the agent's tools or prompts.

## What this demonstrates

- LangChain agent with custom `@tool`-decorated functions for create / read / update / delete
- AIM SDK `secure()` + `@perform_action(capability, resource)` decorator on each CRUD operation
- Real-time capability verification against AIM (every tool call goes through `verify_capability`)
- Audit trail visible in the AIM dashboard
- Trust-score evolution as operations succeed or fail

## Prerequisites

- AIM backend running locally (`docker compose up -d postgres redis backend` from the repo root)
- Python 3.11+
- A Google API key for Gemini (`GOOGLE_API_KEY`) and an AIM API key (`AIM_API_KEY`)

```bash
export AIM_API_KEY='your-aim-api-key'
export GOOGLE_API_KEY='your-google-api-key'
```

The agent and `run-langchain-crud.sh` reach the stack through the host ports the compose files read, so they follow a stack started on other ports:

| Variable | Default | Used for |
|---|---|---|
| `AIM_BACKEND_PORT` | `8080` | the backend, `http://localhost:$AIM_BACKEND_PORT`, unless `AIM_API_URL` is set |
| `AIM_FRONTEND_PORT` | `3000` | the dashboard links, `http://localhost:$AIM_FRONTEND_PORT` |

Each comes from the environment first, then from the repository's `.env`, then the default (`stack_env.py` reads them for the agent, `run-langchain-crud.sh` for itself). `AIM_API_URL` in the environment (or, for `run-langchain-crud.sh`, in this directory's `.env`) wins over `AIM_BACKEND_PORT`. For a stack started with `AIM_BACKEND_PORT=18080 AIM_FRONTEND_PORT=13000`:

```bash
AIM_BACKEND_PORT=18080 AIM_FRONTEND_PORT=13000 ./run-langchain-crud.sh
```

## Setup

Two supported SDK layouts — `flight_agent.py:34-39` prepends both paths so either works:

**Option 1 — Repo checkout (recommended for contributors).** The SDK at `../../sdk/python` is already on `sys.path`.

```bash
pip install -r requirements.txt
python3 langchain_crud_agent.py
```

**Option 2 — Dashboard bundle (AIM Cloud users).** Download the Python SDK from your AIM dashboard → Settings → SDK Downloads, extract over `./aim-sdk-python/`, then run the same command.

## Usage

The agent presents a LangChain REPL. Sample prompts:

- `add a todo: buy milk`
- `list all todos`
- `mark todo 1 as done`
- `delete todo 2`

Each tool call routes through AIM. If you've configured a tool with a capability the agent doesn't hold, the call denies before the tool runs. Watch the AIM dashboard at `http://localhost:3000/dashboard/agents/<agent-id>` (the agent prints this link with the port from `AIM_FRONTEND_PORT`): verification requests, audit events, and trust-score changes appear as the agent runs.

## Files in this directory

- `langchain_crud_agent.py` — the agent (~500 lines)
- `requirements.txt` — pinned dependencies
- `run-langchain-crud.sh` — convenience launcher
- `stack_env.py`: the backend and dashboard URLs, from `AIM_BACKEND_PORT` and `AIM_FRONTEND_PORT`
- `COMPLETE_CHANGES_DOCUMENTATION.md` — internal change-log from the October 2025 implementation; kept for historical context but not required reading to run the demo

## Related demos

- [`flight-search-agent`](../flight-search-agent/) — same AIM verification flow without LangChain, plus deterministic `inject` injection scenarios for the LF stage demo.
- [`a2a-multi-agent-demo`](../a2a-multi-agent-demo/) — Agent-to-Agent collaboration across two agents.
