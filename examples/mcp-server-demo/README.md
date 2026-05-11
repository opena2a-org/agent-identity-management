# MCP Server Demo — Ed25519 + AIM verification

A minimal MCP (Model Context Protocol) server that ships with an Ed25519 keypair and a `/.well-known/mcp/capabilities` endpoint. Use it to exercise AIM's MCP-server-attestation flow without depending on a third-party MCP server.

## What this demonstrates

- Standard MCP protocol surface: `/.well-known/mcp/capabilities`, tools (`echo`, `calculate`, `timestamp`), resources, and a prompt
- Per-request Ed25519 signature on the response — AIM verifies the signature against the public key registered in the dashboard
- Challenge-response authentication: AIM issues a nonce, the server signs it, AIM checks the signature
- Auto-detectable capabilities (AIM's MCP scanner reads the capabilities endpoint and registers tools/resources/prompts automatically)

This is the server side of the MCP attestation story shown on Talk 1, Slide 12 (May 2026 LF Open Source Summit). On the other side, HackMyAgent scans this server's capabilities and prompts as part of its 209-static-check + 29-semantic-check suite.

## Prerequisites

- Python 3.11+
- AIM dashboard running at `http://localhost:3000` (so you can register the server)

## Setup

```bash
pip install -r requirements.txt
python3 mcp-server.py
```

On startup the server prints its public key. Copy that value.

## Register with AIM

1. Open `http://localhost:3000/dashboard/mcp`
2. Click **Register MCP Server**
3. Fill in:
   - **Name**: `test-mcp-local`
   - **URL**: `http://localhost:5555`
   - **Public Key**: (paste the value the server printed)
4. Click **Save**, then **Verify**

Once verified, AIM auto-detects the server's tools / resources / prompts from the capabilities endpoint and registers them.

## Key rotation

The server generates a fresh Ed25519 keypair on every restart. After restarting, re-paste the new public key in the AIM dashboard (the old registration's verification will start failing — that's expected).

To keep a stable keypair across restarts, set `MCP_SERVER_PRIVATE_KEY` to a base64-encoded Ed25519 secret key before launching.

## Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/.well-known/mcp/capabilities` | GET | Returns capabilities + public key (signed) |
| `/tools/echo` | POST | Echoes input back |
| `/tools/calculate` | POST | Evaluates a math expression |
| `/tools/timestamp` | POST | Returns current ISO timestamp |
| `/resources/status` | GET | Server uptime + key fingerprint |
| `/resources/config` | GET | Server configuration snapshot |
| `/prompts/greeting` | POST | Returns a greeting given a name parameter |
| `/auth/challenge` | POST | Signs a nonce with the server's private key |

All responses include an Ed25519 signature header so AIM can verify the response wasn't tampered with in transit.

## Related demos

- [`flight-search-agent`](../flight-search-agent/) — the agent side of the AIM story. Pair it with this MCP server if you want a full agent-to-MCP demo locally.
- [`a2a-multi-agent-demo`](../a2a-multi-agent-demo/) — A2A protocol between two agents (sibling protocol to MCP).
