# Use Case: Track What My AI Agent Does

**Time:** 5 minutes
**Prerequisites:** An AIM identity ([create one first](register-my-agent.md))

## Problem

Your agent performs actions (reading databases, calling APIs, writing files), but there is no record of what happened. When something goes wrong, you have no way to investigate.

## Step 1: Log an Event

```bash
opena2a identity log --action "db:read" --target "users"
```

What `opena2a identity log --action "db:read" --target "users"` prints (captured from opena2a-cli 0.10.13, 2026-11-10, on an empty HOME):

```
Event logged
  Action:  db:read
  Target:  users
  Result:  allowed
  Time:    2026-11-10T10:04:49.402Z
```

Each event is appended to `~/.opena2a/aim-core/audit.jsonl` as one JSON line. The file has no hash chain or signature, so it does not show whether an earlier line was changed. When the file passes 50 MB it is rotated, and the five most recent rotated files are kept.

## Step 2: View the Audit Log

```bash
opena2a identity audit
```

What `opena2a identity audit` prints with one `db:read` event in the log (captured from opena2a-cli 0.10.13, 2026-11-10):

```
Audit Log (last 1)
----------------------------------------------------------------------
  2026-11-10 19:24:13  db:read          users            allowed [cli]
----------------------------------------------------------------------
```

The row gives the event's time in UTC, its action, its target, its result and, in brackets, the plugin name it was logged with (`cli` for `opena2a identity log`).

## Step 3: Limit the Number of Events

View the last 50 events:

```bash
opena2a identity audit --limit 50
```

`--limit` sets how many of the most recent events are listed; without it, `opena2a identity audit` lists the last 10. With one event in the log, `--limit 50` prints the same list as Step 2 (captured from opena2a-cli 0.10.13, 2026-11-10):

```
Audit Log (last 1)
----------------------------------------------------------------------
  2026-11-10 19:24:13  db:read          users            allowed [cli]
----------------------------------------------------------------------
```

## Using the SDK

You can log and query audit events programmatically using the TypeScript or Python SDKs.

### TypeScript

```bash
npm install @opena2a/aim-core
```

```typescript
import { AIMCore } from '@opena2a/aim-core';

const aim = new AIMCore({ agentName: 'my-agent' });
aim.getIdentity();

// Log events (plugin and result are required fields)
aim.logEvent({ action: 'db:read', target: 'customers', result: 'allowed', plugin: 'my-agent' });
aim.logEvent({ action: 'db:write', target: 'customers', result: 'allowed', plugin: 'my-agent' });
aim.logEvent({ action: 'file:write', target: '/tmp/report', result: 'denied', plugin: 'my-agent' });

// Each event is appended to the local audit log as one JSON line.
// These events stay on this machine, separate from the AIM server's audit log.
```

### Python

```bash
pip install -e sdk/python/
```

```python
from aim_sdk import secure

# Register the agent (creates identity automatically)
agent = secure("my-agent")

# Log events using the decorator pattern
@agent.perform_action("db:read", resource="customers")
def read_customers():
    return db.query("SELECT * FROM customers")

# AIM records the decorated calls it verifies, with the agent's identity.
# When connected to an AIM Server (via aim_url), events are also sent to the central database.
```

Events logged through the TypeScript SDK and through the CLI go to the same file in the same format, unless a `dataDir` is set. The audit log at `~/.opena2a/aim-core/audit.jsonl` is compatible with both the CLI `opena2a identity audit` command and SDK-based querying.

## What You Now Have

- A local log of the events your agent and tools record
- A history you can read back with `opena2a identity audit`

## Audit Log Storage

| Mode | Storage | Query |
|------|---------|-------|
| Local (aim-core) | `~/.opena2a/aim-core/audit.jsonl` | CLI or SDK |
| Server (aim-server) | PostgreSQL | REST API + dashboard |

For centralized audit across a fleet of agents, see [Fleet governance](fleet-governance.md).

## Next Steps

- [Enforce capabilities](enforce-capabilities.md): restrict what your agent can do
- [Fleet governance](fleet-governance.md): centralized audit across multiple agents
