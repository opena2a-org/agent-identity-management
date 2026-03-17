# Use Case: Track What My AI Agent Does

**Time:** 5 minutes
**Prerequisites:** An AIM identity ([create one first](register-my-agent.md))

## Problem

Your agent performs actions -- reading databases, calling APIs, writing files -- but there is no record of what happened. When something goes wrong, you have no way to investigate.

## Step 1: Log an Event

```bash
opena2a identity log --action "db:read" --target "users"
```

Expected output:

```
Event logged:
  Action:    db:read
  Target:    users
  Timestamp: 2026-03-16T14:23:01.000Z
  Hash:      sha256:a3f8c1...9e2b
  Chain:     sha256:7d4e0a...1f8c (links to previous event)
```

Each event is appended to `~/.opena2a/aim-core/audit.jsonl`. The hash chain means any tampering is detectable -- if someone modifies an earlier entry, every subsequent hash breaks.

## Step 2: View the Audit Log

```bash
opena2a identity audit
```

Expected output:

```
Audit Log: my-agent (aim_7f3a9c2e)
Chain integrity: VALID (12 events, 0 breaks)

#   Timestamp                Action          Target      Outcome
1   2026-03-16T14:20:00Z     identity:create my-agent    allowed
2   2026-03-16T14:20:05Z     tool:attach     claude-code allowed
3   2026-03-16T14:20:05Z     tool:attach     cursor      allowed
4   2026-03-16T14:23:01Z     db:read         users       allowed
...

Showing 4 of 12 events. Use --limit to see more.
```

## Step 3: Understand the Hash Chain

Every event includes two hashes:

- **Event hash**: SHA-256 of the event data (action, target, timestamp, agent ID)
- **Chain hash**: SHA-256 of this event hash combined with the previous chain hash

This forms a linked chain:

```
Event 1: hash(data_1)                          -> chain_1
Event 2: hash(data_2) + hash(chain_1)          -> chain_2
Event 3: hash(data_3) + hash(chain_2)          -> chain_3
```

If anyone modifies Event 1 after the fact, `chain_1` changes, which invalidates `chain_2`, `chain_3`, and every event after it. The `Chain integrity: VALID` line at the top of the audit output confirms no tampering has occurred.

## Step 4: Filter Events

View the last 50 events:

```bash
opena2a identity audit --limit 50
```

Expected output:

```
Audit Log: my-agent (aim_7f3a9c2e)
Chain integrity: VALID (50 events shown of 128 total, 0 breaks)

#    Timestamp                Action          Target         Outcome
79   2026-03-16T10:00:12Z     api:call        weather-svc    allowed
80   2026-03-16T10:01:44Z     db:read         orders         allowed
81   2026-03-16T10:02:03Z     file:write      /tmp/report    denied
...
128  2026-03-16T14:23:01Z     db:read         users          allowed

Showing 50 of 128 events.
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
aim.getOrCreateIdentity();

// Log events
aim.logEvent({ action: 'db:read', target: 'customers', outcome: 'allowed' });
aim.logEvent({ action: 'api:call', target: 'weather-svc', outcome: 'allowed' });
aim.logEvent({ action: 'file:write', target: '/tmp/report', outcome: 'denied' });

// Each event is appended to the local audit log with a SHA-256 hash chain.
// When connected to an AIM Server, events are also sent to the central database.
```

### Python

```bash
pip install aim-sdk
```

```python
from aim_sdk import AIMCore

aim = AIMCore(agent_name="my-agent")
aim.get_or_create_identity()

# Log events
aim.log_event(action="db:read", target="customers", outcome="allowed")
aim.log_event(action="api:call", target="weather-svc", outcome="allowed")
aim.log_event(action="file:write", target="/tmp/report", outcome="denied")

# Each event is appended to the local audit log with a SHA-256 hash chain.
# When connected to an AIM Server, events are also sent to the central database.
```

Events logged via the SDK follow the same hash chain format as CLI-logged events. The audit log at `~/.opena2a/aim-core/audit.jsonl` is compatible with both the CLI `opena2a identity audit` command and SDK-based querying.

## What You Now Have

- An append-only log of every agent action
- Tamper detection via SHA-256 hash chain
- Filterable history for incident investigation

## Audit Log Storage

| Mode | Storage | Query |
|------|---------|-------|
| Local (aim-core) | `~/.opena2a/aim-core/audit.jsonl` | CLI or SDK |
| Server (aim-server) | PostgreSQL | REST API + dashboard |

For centralized audit across a fleet of agents, see [Fleet governance](fleet-governance.md).

## Next Steps

- [Enforce capabilities](enforce-capabilities.md) -- restrict what your agent can do
- [Fleet governance](fleet-governance.md) -- centralized audit across multiple agents
