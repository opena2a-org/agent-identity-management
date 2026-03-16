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
const identity = aim.getOrCreateIdentity();

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
  outcome: 'allowed'
});

aim.logEvent({
  action: 'file:write',
  target: '/tmp/report.csv',
  outcome: 'denied'
});
```

Each event is appended to the local audit log with a SHA-256 hash chain.

### Enforce Capabilities

```typescript
// Load policy from object
aim.loadPolicy({
  allow: ['db:read', 'api:call'],
  deny: ['db:write', 'file:execute']
});

// Check before performing an action
try {
  aim.checkCapability('db:read');
  // proceed with database read
} catch (err) {
  // CapabilityDenied: db:read is not allowed
}

// Or use the boolean form
if (aim.isAllowed('api:call')) {
  // proceed
}
```

### Calculate Trust Score

```typescript
const score = aim.calculateTrust();

console.log('Score:', score.score);
console.log('Grade:', score.grade);
console.log('Factors:', JSON.stringify(score.factors, null, 2));
```

Expected output:

```
Score: 85
Grade: B
Factors: {
  "identityStrength": 10,
  "capabilityCompliance": 15,
  "auditCompleteness": 10,
  "mcpAttestation": 10,
  "policyAdherence": 10,
  "lifecycleStatus": 10,
  "ownershipVerification": 10,
  "behavioralAnalysis": 10
}
```

### Full Example

```typescript
import { AIMCore } from '@opena2a/aim-core';

async function main() {
  const aim = new AIMCore({ agentName: 'data-processor' });

  // Create or load identity
  const identity = aim.getOrCreateIdentity();
  console.log(`Agent ${identity.agentId} initialized`);

  // Set capability policy
  aim.loadPolicy({
    allow: ['db:read', 'api:call', 'file:read'],
    deny: ['db:write', 'db:delete', 'file:execute']
  });

  // Gate actions on capability checks
  if (aim.isAllowed('db:read')) {
    aim.logEvent({ action: 'db:read', target: 'orders', outcome: 'allowed' });
    // ... perform the read
  }

  if (!aim.isAllowed('db:delete')) {
    aim.logEvent({ action: 'db:delete', target: 'orders', outcome: 'denied' });
    console.log('Delete operation blocked by policy');
  }

  // Report trust
  const trust = aim.calculateTrust();
  console.log(`Trust: ${trust.score}/100 (${trust.grade})`);
}

main();
```

Expected output:

```
Agent aim_3b8f2c1d initialized
Delete operation blocked by policy
Trust: 85/100 (B)
```

## Option B: Python

### Install

```bash
pip install aim-sdk
```

### Full Example

```python
from aim_sdk import AIMCore

aim = AIMCore(agent_name="data-processor")

# Create or load identity
identity = aim.get_or_create_identity()
print(f"Agent {identity.agent_id} initialized")

# Set capability policy
aim.load_policy(
    allow=["db:read", "api:call", "file:read"],
    deny=["db:write", "db:delete", "file:execute"]
)

# Gate actions on capability checks
if aim.is_allowed("db:read"):
    aim.log_event(action="db:read", target="orders", outcome="allowed")
    # ... perform the read

if not aim.is_allowed("db:delete"):
    aim.log_event(action="db:delete", target="orders", outcome="denied")
    print("Delete operation blocked by policy")

# Report trust
trust = aim.calculate_trust()
print(f"Trust: {trust.score}/100 ({trust.grade})")
```

Expected output:

```
Agent aim_3b8f2c1d initialized
Delete operation blocked by policy
Trust: 85/100 (B)
```

## API Reference

### AIMCore Constructor

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `agentName` | string | required | Name for the agent identity |
| `storagePath` | string | `~/.opena2a/aim-core/` | Where keys and logs are stored |

### Methods

| Method | Description |
|--------|-------------|
| `getOrCreateIdentity()` | Returns existing identity or creates a new Ed25519 keypair |
| `loadPolicy(policy)` | Sets allow/deny capability rules |
| `checkCapability(action)` | Throws if action is denied |
| `isAllowed(action)` | Returns boolean |
| `logEvent(event)` | Appends to the tamper-evident audit log |
| `calculateTrust()` | Returns score (0-100), grade (A-F), and factor breakdown |

## What You Now Have

- Cryptographic identity embedded in your application code
- Capability enforcement without external dependencies
- A tamper-evident audit log for every action
- Trust scores your users or upstream systems can query

## Next Steps

- [Enforce capabilities](enforce-capabilities.md) -- more detail on policy syntax and wildcards
- [Fleet governance](fleet-governance.md) -- centralize identity management when you outgrow local mode
