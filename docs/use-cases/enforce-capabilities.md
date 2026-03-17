# Use Case: Restrict What My Agent Can Do

**Time:** 5 minutes
**Prerequisites:** An AIM identity ([create one first](register-my-agent.md))

## Problem

Your agent has access to tools it should not use. You want to declare allowed and denied actions, and have the runtime enforce them.

## Step 1: Create a Policy File

Create `policy.yaml`:

```yaml
version: "1.0"
agent: my-agent
capabilities:
  allow:
    - db:read
    - api:call
    - file:read
  deny:
    - db:write
    - db:delete
    - network:fetch
    - file:execute
```

The policy uses a deny-by-default model: anything not explicitly listed in `allow` is denied. The `deny` list makes intent explicit and adds entries to the audit log when a denied action is attempted.

## Step 2: Load the Policy

```bash
opena2a identity policy load --file policy.yaml
```

Expected output:

```
Policy loaded:
  Agent:    my-agent (aim_7f3a9c2e)
  Allowed:  4 capabilities (db:read, api:call, file:read)
  Denied:   4 capabilities (db:write, db:delete, network:fetch, file:execute)
  Stored:   ~/.opena2a/aim-core/policies/my-agent.yaml
```

## Step 3: Check an Allowed Capability

```bash
opena2a identity check file:read
```

Expected output:

```
ALLOWED: file:read

  Agent:   my-agent (aim_7f3a9c2e)
  Policy:  explicitly allowed
  Logged:  capability:check -> file:read (allowed)
```

## Step 4: Check a Denied Capability

```bash
opena2a identity check network:fetch
```

Expected output:

```
DENIED: network:fetch

  Agent:   my-agent (aim_7f3a9c2e)
  Policy:  explicitly denied
  Logged:  capability:check -> network:fetch (denied)
```

The check returns a non-zero exit code on denial, so you can use it in scripts:

```bash
if opena2a identity check db:read; then
  echo "Proceeding with database read"
else
  echo "Agent is not allowed to read the database"
fi
```

## Step 5: Verify Trust Score Improvement

Loading a policy improves your trust score:

```bash
opena2a identity trust
```

Expected output:

```
Trust Score: 85/100 (B)

Factors:
  Identity Strength:      10/15  (Ed25519 key present)
  Capability Compliance:  15/15  (policy loaded, enforced)
  Audit Completeness:     10/15  (log active, events present)
  MCP Attestation:        10/10  (2 tools verified)
  Policy Adherence:       10/10  (no violations)
  Lifecycle Status:       10/10  (active)
  Ownership Verification: 10/15  (key + tool attestation)
  Behavioral Analysis:    10/10  (consistent behavior)
```

## Using the SDK

You can load policies and enforce capabilities programmatically using the TypeScript or Python SDKs.

### TypeScript

```bash
npm install @opena2a/aim-core
```

```typescript
import { AIMCore } from '@opena2a/aim-core';

const aim = new AIMCore({ agentName: 'my-agent' });
aim.getOrCreateIdentity();

// Load policy (equivalent to the YAML policy file above)
aim.loadPolicy({
  allow: ['db:read', 'api:call', 'file:read'],
  deny: ['db:write', 'db:delete', 'network:fetch', 'file:execute']
});

// Check with exception on denial
try {
  aim.checkCapability('db:read');
  console.log('db:read is allowed');
} catch (err) {
  console.log('db:read is denied');
}

// Check with boolean
if (aim.isAllowed('network:fetch')) {
  // proceed with network call
} else {
  console.log('network:fetch is denied by policy');
}

// Trust score reflects loaded policy
const trust = aim.calculateTrust();
console.log(`Trust: ${trust.score}/100 (${trust.grade})`);
```

Expected output:

```
db:read is allowed
network:fetch is denied by policy
Trust: 85/100 (B)
```

### Python

```bash
pip install aim-sdk
```

```python
from aim_sdk import AIMCore

aim = AIMCore(agent_name="my-agent")
aim.get_or_create_identity()

# Load policy (equivalent to the YAML policy file above)
aim.load_policy(
    allow=["db:read", "api:call", "file:read"],
    deny=["db:write", "db:delete", "network:fetch", "file:execute"]
)

# Check with exception on denial
try:
    aim.check_capability("db:read")
    print("db:read is allowed")
except Exception:
    print("db:read is denied")

# Check with boolean
if aim.is_allowed("network:fetch"):
    pass  # proceed with network call
else:
    print("network:fetch is denied by policy")

# Trust score reflects loaded policy
trust = aim.calculate_trust()
print(f"Trust: {trust.score}/100 ({trust.grade})")
```

Expected output:

```
db:read is allowed
network:fetch is denied by policy
Trust: 85/100 (B)
```

The SDK policy enforcement uses the same deny-by-default model as the CLI. Actions not listed in `allow` are denied, and `checkCapability()` / `check_capability()` raises an exception on denial so you can integrate it into your application's control flow.

## Policy Format Reference

```yaml
version: "1.0"
agent: my-agent
capabilities:
  allow:
    - "db:read"           # exact match
    - "api:*"             # wildcard: any api action
    - "file:read"
  deny:
    - "db:write"
    - "network:*"         # wildcard: all network actions
    - "file:execute"
```

Wildcards (`*`) match any suffix after the colon. `api:*` allows `api:call`, `api:list`, `api:delete`, etc.

## What You Now Have

- A declarative policy controlling what your agent can do
- Runtime enforcement with non-zero exit codes on denial
- Every capability check recorded in the audit log

## Next Steps

- [Audit agent actions](audit-agent-actions.md) -- review what your agent has done
- [Embed in my app](embed-in-my-app.md) -- enforce capabilities programmatically in your code
- [Fleet governance](fleet-governance.md) -- manage policies across multiple agents
