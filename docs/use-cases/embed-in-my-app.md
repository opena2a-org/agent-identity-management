# Use Case: Add Identity to My Own Agent Framework

**Time:** 10 minutes
**Prerequisites:** Node.js 18+ or Python 3.9+

## Problem

You are building an agent framework, tool, or library. You want to embed cryptographic identity, audit logging, and capability enforcement directly in your code -- without requiring users to install a CLI or run a server.

## Option A: TypeScript / Node.js

### Install

```bash
npm install @opena2a/aim-core
```

### Create an Identity

```typescript
import { AIMCore } from '@opena2a/aim-core';

const aim = new AIMCore({ agentName: 'my-assistant' });
const identity = aim.getIdentity();

console.log('Agent ID:', identity.agentId);
console.log('Public Key:', identity.publicKey);
```

Expected output:

```
Agent ID: aim_7f3a9c2e
Public Key: ed25519:x8Kp...mQ4R
```

The identity is created on first run and persisted to `~/.opena2a/aim-core/identities/`. Subsequent calls return the existing identity.

### Log Events

```typescript
aim.logEvent({
  action: 'db:read',
  target: 'customers',
  result: 'allowed',
  plugin: 'my-assistant'
});

aim.logEvent({
  action: 'file:write',
  target: '/tmp/report.csv',
  result: 'denied',
  plugin: 'my-assistant'
});
```

Each event is appended to the local audit log with a SHA-256 hash chain.

### Enforce Capabilities

```typescript
// Save policy with rules (or load from an existing YAML file via aim.loadPolicy())
aim.savePolicy({
  version: '1.0',
  defaultAction: 'deny',
  rules: [
    { capability: 'db:read', action: 'allow' },
    { capability: 'api:call', action: 'allow' },
    { capability: 'db:write', action: 'deny' },
    { capability: 'file:execute', action: 'deny' }
  ]
});

// checkCapability returns a boolean
if (aim.checkCapability('db:read')) {
  // proceed with database read
}

if (!aim.checkCapability('db:write')) {
  console.log('db:write is denied by policy');
}
```

### Calculate Trust Score

```typescript
const score = aim.calculateTrust();

console.log('Overall:', score.overall);
console.log('Calculated at:', score.calculatedAt);
console.log('Factors:', JSON.stringify(score.factors, null, 2));
```

Expected output:

```
Overall: 0.72
Calculated at: 2026-03-16T14:00:00.000Z
Factors: {
  "identity": 1,
  "capabilities": 0.8,
  "auditLog": 0.7,
  "secretsManaged": 0,
  "configSigned": 0,
  "skillsVerified": 0,
  "networkControlled": 0,
  "heartbeatMonitored": 0
}
```

### Full Example

```typescript
import { AIMCore } from '@opena2a/aim-core';

function main() {
  const aim = new AIMCore({ agentName: 'data-processor' });

  // Create or load identity
  const identity = aim.getIdentity();
  console.log(`Agent ${identity.agentId} initialized`);

  // Set capability policy
  aim.savePolicy({
    version: '1.0',
    defaultAction: 'deny',
    rules: [
      { capability: 'db:read', action: 'allow' },
      { capability: 'api:call', action: 'allow' },
      { capability: 'file:read', action: 'allow' },
      { capability: 'db:write', action: 'deny' },
      { capability: 'db:delete', action: 'deny' },
      { capability: 'file:execute', action: 'deny' }
    ]
  });

  // Gate actions on capability checks (checkCapability returns boolean)
  if (aim.checkCapability('db:read')) {
    aim.logEvent({ action: 'db:read', target: 'orders', result: 'allowed', plugin: 'data-processor' });
    // ... perform the read
  }

  if (!aim.checkCapability('db:delete')) {
    aim.logEvent({ action: 'db:delete', target: 'orders', result: 'denied', plugin: 'data-processor' });
    console.log('Delete operation blocked by policy');
  }

  // Report trust (overall is 0-1 float)
  const trust = aim.calculateTrust();
  console.log(`Trust: ${trust.overall}`);
}

main();
```

Expected output:

```
Agent aim_3b8f2c1d initialized
Delete operation blocked by policy
Trust: 0.72
```

## Option B: Python

### Install

```bash
pip install -e sdk/python/
```

### Full Example

```python
from aim_sdk import secure

# Register agent with allowed capabilities
agent = secure(
    name="data-processor",
    capabilities=["db:read", "api:call", "file:read"]
)
print(f"Agent {agent.agent_id} initialized")

# Gate actions using the decorator pattern
@agent.perform_action("db:read", resource="orders")
def read_orders():
    # ... perform the read
    return db.query("SELECT * FROM orders")

@agent.perform_action("db:delete", resource="orders")
def delete_orders():
    return db.execute("DELETE FROM orders")

# Allowed action succeeds
read_orders()
print("db:read succeeded")

# Denied action raises ActionDeniedError
try:
    delete_orders()
except Exception:
    print("Delete operation blocked by policy")
```

Expected output:

```
Agent aim_3b8f2c1d initialized
db:read succeeded
Delete operation blocked by policy
```

## API Reference

### TypeScript: AIMCore Constructor

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `agentName` | string | required | Name for the agent identity |
| `dataDir` | string | `~/.opena2a/aim-core/` | Where keys, logs, and policies are stored |
| `serverUrl` | string | undefined | Optional AIM server URL for fleet reporting |

### TypeScript: Methods

| Method | Returns | Description |
|--------|---------|-------------|
| `getIdentity()` | `AIMIdentity` | Returns existing identity or creates a new Ed25519 keypair |
| `loadPolicy()` | `CapabilityPolicy` | Loads policy from the YAML file in the data directory |
| `savePolicy(policy)` | void | Saves a `CapabilityPolicy` object to YAML |
| `checkCapability(action, plugin?)` | boolean | Returns `true` if allowed, `false` if denied |
| `logEvent(event)` | `AuditEvent` | Appends to the audit log. Requires `action`, `target`, `result`, and `plugin` fields |
| `readAuditLog(options?)` | `AuditEvent[]` | Reads events from the audit log with optional `limit` and `since` filters |
| `calculateTrust()` | `TrustScore` | Returns `overall` (0-1 float), `factors`, and `calculatedAt` |
| `setTrustHints(hints)` | void | Provides plugin hints (secretsManaged, configSigned, etc.) to improve trust accuracy |

### Python: Key Exports

| Function/Class | Description |
|----------------|-------------|
| `register_agent(name, capabilities, agent_type, ...)` | Registers an agent and returns an `AIMClient` instance |
| `secure(name, ...)` | Alias for `register_agent` |
| `AIMClient(agent_id, aim_url, api_key, ...)` | Low-level client for connecting to an AIM server |
| `AgentType` | Enum of agent types: `CLAUDE`, `LANGCHAIN`, `CREWAI`, `GPT`, etc. |
| `@client.perform_action(action, resource)` | Decorator that logs and enforces capabilities on function calls |

## What You Now Have

- Cryptographic identity embedded in your application code
- Capability enforcement without external dependencies
- A tamper-evident audit log for every action
- Trust scores your users or upstream systems can query

## Next Steps

- [Enforce capabilities](enforce-capabilities.md) -- more detail on policy syntax and wildcards
- [Fleet governance](fleet-governance.md) -- centralize identity management when you outgrow local mode
