# MCP Server Demo — Ed25519 + AIM verification

A minimal MCP (Model Context Protocol) server that holds an Ed25519 keypair, signs the challenge AIM sends to `/mcp/.well-known/mcp/verify`, and lists its tools, resources and prompts at `/.well-known/mcp/capabilities`. Use it to exercise AIM's MCP-server verification flow without depending on a third-party MCP server.

## What this demonstrates

- Capability discovery: `/.well-known/mcp/capabilities` lists three tools (`echo`, `calculate`, `timestamp`), two resources (`server://status`, `server://config`) and one prompt (`greeting`)
- Challenge-response verification: AIM sends a nonce, the server signs it with its Ed25519 private key, and AIM checks the signature against the public key registered for the server
- Capability detection: after a successful verification, AIM reads the capabilities endpoint and records the tools, resources and prompts it lists

This is the server side of the MCP attestation story shown on Talk 1, Slide 12 (May 2026 LF Open Source Summit). On the other side, HackMyAgent scans this server's capabilities and prompts as part of its 209-static-check + 29-semantic-check suite.

## Prerequisites

- Python 3.11+
- AIM dashboard running at `http://localhost:3000` (so you can register the server)

## Setup

```bash
python3 -m venv .venv
. .venv/bin/activate
pip install -r requirements.txt
python3 mcp-server.py
```

The server listens on port 5151 on all interfaces. On startup it prints its public key (the `Public Key:` line). Copy that value.

Call a tool to see it answer:

```bash
curl -s -X POST http://localhost:5151/mcp/tools/calculate \
  -H 'Content-Type: application/json' -d '{"expression": "2 + 2"}'
```

```
{"content":[{"text":"2 + 2 = 4","type":"text"}]}
```

## Check the server against this README

```bash
python3 -m unittest -v test_readme.py
```

The test starts its own copy of the server on port 5151 (stop any copy that is already running), sends every route in the Endpoints table its documented method, signs a nonce through the verification URL AIM builds from the registration below, and checks the signature against the public key the server printed.

## Register with AIM

1. Open `http://localhost:3000/dashboard/mcp`
2. Click **Register MCP server**
3. Fill in:
   - **Server name**: `test-mcp-local`
   - **Server URL**: `http://localhost:5151/mcp`
   - **Public key (optional)**: paste the value the server printed. Left empty, AIM generates a keypair of its own, which this server does not hold.
4. Click **Register server**, open the server's page, and click **Verify**

The registered URL ends in `/mcp` because of how AIM builds its two requests:

- Verification: AIM appends `/.well-known/mcp/verify` to the registered URL, so it posts its challenge to `http://localhost:5151/mcp/.well-known/mcp/verify`, and reads `signedChallenge` from the reply.
- Capability detection: AIM drops the path and reads `http://localhost:5151/.well-known/mcp/capabilities`.

AIM sends these requests only to addresses that resolve to a public IP. Against `http://localhost:5151/mcp` the **Verify** step reports `Verification failed: server is unreachable`, and the audit log entry for it records `URL hostname "localhost" is not allowed`. To complete verification, register the URL at which the server is reachable from a public address. The signing side is the same either way, and `test_readme.py` checks it locally.

## Key rotation

The server generates a fresh Ed25519 keypair every time it starts. After a restart, update the public key on the server's AIM registration; until then, verification against the old key fails.

## Endpoints

| Endpoint | Method | Purpose |
|---|---|---|
| `/.well-known/mcp/capabilities` | GET | Lists the tools, resources and prompts (read by AIM's capability detection) |
| `/mcp/.well-known/mcp/verify` | POST | Signs the `challenge` string from the JSON body; replies with `signedChallenge`, `publicKey` and `algorithm` |
| `/mcp/health` | GET | Health status and the server's public key |
| `/mcp/tools/echo` | POST | Echoes `message` from the JSON body |
| `/mcp/tools/calculate` | POST | Evaluates the arithmetic `expression` from the JSON body (numbers, `+ - * / % **` and unary minus) |
| `/mcp/tools/timestamp` | POST | Returns the current UTC timestamp |
| `/mcp/resources/server/status` | GET | The `server://status` resource |
| `/mcp/prompts/greeting` | GET | The `greeting` prompt; optional `name` query parameter |

The `server://config` resource is listed in the capabilities but has no route. Only the verification reply is signed; every other response is plain JSON.

## Related demos

- [`flight-search-agent`](../flight-search-agent/) — the agent side of the AIM story. Pair it with this MCP server if you want a full agent-to-MCP demo locally.
- [`a2a-multi-agent-demo`](../a2a-multi-agent-demo/) — A2A protocol between two agents (sibling protocol to MCP).
