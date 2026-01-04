# AIM CLI Specification

**Goal:** Make AIM feel like a real developer tool. CLI-first experience for OSS adoption.

---

## Design Principles

1. **Familiar patterns** — Like git, docker, kubectl
2. **Sensible defaults** — Works without config
3. **Local-first** — No cloud required
4. **Scriptable** — JSON output for automation
5. **Fast** — Sub-100ms for common operations

---

## Installation

```bash
# macOS
brew install opena2a/tap/aim

# Linux
curl -sSL https://get.opena2a.org | sh

# Windows
scoop install aim

# From source
go install github.com/opena2a-org/aim-cli@latest
```

---

## Configuration

```bash
# Config file location
~/.aim/config.yaml

# Environment variables (override config)
AIM_SERVER_URL=http://localhost:8080
AIM_API_KEY=sk-...
AIM_OUTPUT=json  # or "table" or "plain"
```

---

## Commands

### `aim init`

Initialize AIM in current project.

```bash
aim init

# Creates:
# - .aim/config.yaml (local config)
# - Detects agent frameworks (LangChain, CrewAI, etc.)
# - Suggests SDK installation
```

**Output:**
```
Detected: Python project with LangChain
Created: .aim/config.yaml

Next steps:
  pip install aim-sdk
  aim login
```

---

### `aim login`

Authenticate with AIM server (browser-based OAuth flow).

```bash
# Interactive login (opens browser automatically)
aim login

# To specific server
aim login --server https://aim.example.com

# Non-interactive (for CI/scripts)
aim login --api-key sk-...

# Show current auth status
aim login --status

# Logout
aim logout
```

**Flow (browser-based):**
```
$ aim login

Opening browser to authenticate...
If browser doesn't open, visit: https://aim.opena2a.org/cli/auth?code=ABCD-1234

Waiting for authentication...

✓ Authenticated as admin@example.com
  Server: https://aim.opena2a.org
  Token saved to ~/.aim/credentials
```

**How it works:**
1. CLI generates a device code
2. Opens browser to `/cli/auth?code=XXXX`
3. User authenticates (Google, Microsoft, email, etc.)
4. CLI polls for token completion
5. Token saved to `~/.aim/credentials`

This is the same pattern as:
- `gh auth login` (GitHub CLI)
- `vercel login`
- `supabase login`
- `netlify login`

**Backend endpoints needed:**
- `POST /api/v1/cli/device-code` — Generate device code
- `GET /api/v1/cli/device-code/{code}/status` — Poll for completion
- `GET /cli/auth` — Browser page to authenticate with code

---

### `aim status`

Quick overview of your AIM setup.

```bash
aim status
```

**Output:**
```
AIM Status
──────────────────────────────────
Server:     http://localhost:8080 (connected)
User:       admin@example.com
Agents:     12 registered (11 active, 1 suspended)
MCP:        5 servers (4 verified, 1 pending)
Actions:    1,247 today (3 blocked)
Trust:      avg 0.87
```

---

### `aim agents`

Manage agents.

```bash
# List all agents
aim agents list
aim agents list --status active
aim agents list --type langchain
aim agents list --output json

# Get agent details
aim agents get <agent-id>
aim agents get my-agent --output json

# Suspend/revoke
aim agents suspend <agent-id> --reason "investigating anomaly"
aim agents revoke <agent-id>
aim agents activate <agent-id>

# Delete
aim agents delete <agent-id>
```

**Output (list):**
```
AGENT ID          TYPE        STATUS    TRUST   LAST SEEN
────────────────────────────────────────────────────────────
my-agent          langchain   active    0.92    2 minutes ago
data-processor    crewai      active    0.88    1 hour ago
test-agent        claude      suspended 0.45    3 days ago
```

**Output (get):**
```
Agent: my-agent
──────────────────────────────────
ID:           a1b2c3d4-...
Type:         langchain
Status:       active
Trust Score:  0.92

Capabilities:
  - db:read
  - api:call
  - file:read

MCP Connections:
  - postgres-mcp (verified, trust: 0.95)
  - slack-mcp (verified, trust: 0.88)

Recent Actions:
  2 min ago   db:read      users_table     success
  5 min ago   api:call     stripe-api      success
  12 min ago  db:read      orders_table    success
```

---

### `aim logs`

View activity logs.

```bash
# All logs
aim logs

# Filter by agent
aim logs --agent my-agent

# Filter by time
aim logs --since 1h
aim logs --since 2024-01-01

# Filter by capability
aim logs --capability db:write

# Filter by status
aim logs --status blocked

# Follow mode (like tail -f)
aim logs -f
aim logs -f --agent my-agent

# Output as JSON (for piping)
aim logs --output json | jq '.[] | select(.risk == "high")'
```

**Output:**
```
TIME                AGENT           ACTION      RESOURCE        STATUS
─────────────────────────────────────────────────────────────────────────
2 min ago           my-agent        db:read     users_table     success
5 min ago           my-agent        api:call    stripe-api      success
8 min ago           data-proc       file:write  report.csv      success
15 min ago          test-agent      db:delete   prod_table      BLOCKED
```

**Output (follow mode):**
```
Watching for new actions... (Ctrl+C to stop)

14:32:01  my-agent        db:read     users_table     success
14:32:15  my-agent        api:call    openai-api      success
14:33:02  data-proc       file:read   config.yaml     success
```

---

### `aim mcp`

Manage MCP servers.

```bash
# List MCP servers
aim mcp list
aim mcp list --status verified

# Get MCP details
aim mcp get <mcp-name>

# Verify an MCP server
aim mcp verify <mcp-url>

# Check for drift
aim mcp drift-check
aim mcp drift-check --mcp postgres-mcp
```

**Output (list):**
```
MCP SERVER        URL                         STATUS      TRUST   AGENTS
──────────────────────────────────────────────────────────────────────────
postgres-mcp      http://localhost:5432       verified    0.95    3
slack-mcp         http://localhost:3001       verified    0.88    2
github-mcp        http://localhost:3002       pending     -       0
untrusted-mcp     http://evil.com:8080        unverified  0.12    1
```

**Output (drift-check):**
```
Checking MCP servers for capability drift...

postgres-mcp:  OK (no changes)
slack-mcp:     OK (no changes)
github-mcp:    DRIFT DETECTED
  Added:   issues:delete, repo:admin
  Removed: (none)

1 server has capability drift. Review with: aim mcp get github-mcp
```

---

### `aim alerts`

View security alerts.

```bash
# List alerts
aim alerts list
aim alerts list --severity critical
aim alerts list --unacknowledged

# Acknowledge alert
aim alerts ack <alert-id>
aim alerts ack <alert-id> --note "false positive"

# Get alert details
aim alerts get <alert-id>
```

**Output:**
```
SEVERITY   TIME          AGENT         MESSAGE
──────────────────────────────────────────────────────────────
CRITICAL   10 min ago    test-agent    Capability violation: db:delete
HIGH       1 hour ago    data-proc     Trust score dropped below 0.5
MEDIUM     2 hours ago   my-agent      Unusual action pattern detected
```

---

### `aim server`

Run AIM server locally (for development).

```bash
# Start with SQLite (zero config)
aim server start

# Start with PostgreSQL
aim server start --db postgres://localhost/aim

# Specify port
aim server start --port 8080

# Run in background
aim server start --daemon

# Stop background server
aim server stop

# Check server status
aim server status
```

**Output:**
```
Starting AIM server...

Server:    http://localhost:8080
Database:  SQLite (~/.aim/aim.db)
Dashboard: http://localhost:8080

Ready. Press Ctrl+C to stop.
```

---

### `aim export`

Export data for compliance/backup.

```bash
# Export audit logs
aim export logs --since 30d --output audit-logs.json

# Export ABOM (Agent Bill of Materials)
aim export abom --output abom.json
aim export abom --format cyclonedx

# Export agents
aim export agents --output agents.json
```

---

### `aim verify`

Verify agent identity (useful in CI).

```bash
# Verify an agent is registered and active
aim verify <agent-id>

# Verify with specific trust threshold
aim verify <agent-id> --min-trust 0.8

# Exit codes for CI:
# 0 = verified
# 1 = not found
# 2 = suspended/revoked
# 3 = trust below threshold
```

**Output:**
```
Agent: my-agent
Status: VERIFIED

Trust Score: 0.92 (threshold: 0.8)
Last Verified: 2 minutes ago
```

---

## Global Flags

```bash
--server, -s      AIM server URL
--output, -o      Output format: table, json, plain
--quiet, -q       Suppress non-essential output
--verbose, -v     Show debug information
--help, -h        Show help
--version         Show version
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Authentication error |
| 3 | Resource not found |
| 4 | Verification failed |
| 5 | Server connection failed |

---

## Examples

### Quick Start

```bash
# Install and start local server
brew install opena2a/tap/aim
aim server start --daemon

# Check status
aim status

# Watch logs in real-time
aim logs -f
```

### CI/CD Integration

```bash
#!/bin/bash
# verify-agents.sh

# Verify all production agents have high trust
for agent in $(aim agents list --tag production --output json | jq -r '.[].id'); do
  if ! aim verify "$agent" --min-trust 0.8; then
    echo "Agent $agent failed verification"
    exit 1
  fi
done

echo "All agents verified"
```

### Security Monitoring

```bash
# Check for any blocked actions in last hour
aim logs --status blocked --since 1h

# Check for capability drift
aim mcp drift-check

# Review critical alerts
aim alerts list --severity critical --unacknowledged
```

---

## Implementation Notes

### Tech Stack
- **Language:** Go (single binary, cross-platform)
- **CLI Framework:** Cobra + Viper
- **Output:** tablewriter for tables, encoding/json for JSON
- **Config:** YAML in ~/.aim/

### API Endpoints Needed

Most commands map to existing API endpoints:

| Command | Endpoint |
|---------|----------|
| `aim agents list` | `GET /api/v1/agents` |
| `aim agents get` | `GET /api/v1/agents/{id}` |
| `aim logs` | `GET /api/v1/audit-logs` |
| `aim mcp list` | `GET /api/v1/mcp-servers` |
| `aim alerts list` | `GET /api/v1/security-alerts` |
| `aim status` | `GET /api/v1/stats` (new) |

### New Endpoints Needed

1. `GET /api/v1/stats` — Aggregate stats for `aim status`
2. `POST /api/v1/agents/{id}/suspend` — Suspend agent
3. `POST /api/v1/agents/{id}/activate` — Reactivate agent
4. `GET /api/v1/mcp-servers/drift-check` — Check all for drift

---

## Release Plan

### v0.1.0 (MVP)
- [ ] `aim login`
- [ ] `aim status`
- [ ] `aim agents list|get`
- [ ] `aim logs` (basic)
- [ ] `aim server start` (SQLite)

### v0.2.0
- [ ] `aim mcp list|get|drift-check`
- [ ] `aim alerts list|ack`
- [ ] `aim logs -f` (follow mode)
- [ ] `aim verify`

### v0.3.0
- [ ] `aim init`
- [ ] `aim export`
- [ ] `aim agents suspend|revoke|activate`
- [ ] Homebrew formula
- [ ] GitHub Action

---

## SDK Authentication Updates

The SDKs should support the same browser-based auth flow for developer experience consistency.

### Current State
```python
# Today: manual token
agent = secure("my-agent", api_key="sk-...")

# Or environment variable
# AIM_SDK_TOKEN=sk-...
agent = secure("my-agent")
```

### Proposed: Browser-Based Auth

**Python SDK:**
```python
from aim_sdk import login, secure

# Interactive login (opens browser)
login()  # Opens browser, saves token to ~/.aim/credentials

# Then use normally
agent = secure("my-agent")  # Uses saved credentials
```

**Java SDK:**
```java
import org.opena2a.aim.AIMAuth;
import org.opena2a.aim.AIMClient;

// Interactive login
AIMAuth.login();  // Opens browser

// Then use normally
AIMClient client = AIMClient.create();  // Uses saved credentials
```

**TypeScript SDK:**
```typescript
import { login, secure } from '@opena2a/aim-sdk';

// Interactive login
await login();  // Opens browser

// Then use normally
const agent = await secure('my-agent');
```

### Credential Storage

All SDKs and CLI share the same credential file:

```yaml
# ~/.aim/credentials
default:
  server: https://aim.opena2a.org
  token: eyJ...
  email: user@example.com
  expires: 2024-02-01T00:00:00Z

# Support multiple profiles
work:
  server: https://aim.company.com
  token: eyJ...
```

### Environment Variable Override

For CI/CD, environment variables always take precedence:

```bash
AIM_SERVER_URL=https://aim.company.com
AIM_API_KEY=sk-...
```

### Benefits

1. **Consistent DX** — Same auth flow across CLI and all SDKs
2. **No manual token copying** — Browser handles OAuth
3. **Shared credentials** — Login once, use everywhere
4. **Secure** — No tokens in code or config files

---

## Success Metrics

- Downloads/installs per week
- `aim status` commands per user (engagement)
- GitHub stars on CLI repo
- Mentions in dev communities (Twitter, HN, Reddit)
