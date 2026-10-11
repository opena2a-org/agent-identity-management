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
Trust Score: 0.85 (strong)

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
aim.getIdentity();

// Save policy (equivalent to the YAML policy file above)
aim.savePolicy({
  version: '1.0',
  defaultAction: 'deny',
  rules: [
    { capability: 'db:read', action: 'allow' },
    { capability: 'api:call', action: 'allow' },
    { capability: 'file:read', action: 'allow' },
    { capability: 'db:write', action: 'deny' },
    { capability: 'db:delete', action: 'deny' },
    { capability: 'network:fetch', action: 'deny' },
    { capability: 'file:execute', action: 'deny' }
  ]
});

// checkCapability returns a boolean (does not throw)
if (aim.checkCapability('db:read')) {
  console.log('db:read is allowed');
} else {
  console.log('db:read is denied');
}

if (aim.checkCapability('network:fetch')) {
  // proceed with network call
} else {
  console.log('network:fetch is denied by policy');
}

// Trust score reflects loaded policy (overall is 0-1, not a grade)
const trust = aim.calculateTrust();
console.log(`Trust: ${trust.overall}`);
```

Expected output:

```
db:read is allowed
network:fetch is denied by policy
Trust: 0.72
```

### Python

The Python SDK asks the AIM server about each call, so what happens to a call outside the agent's capabilities depends
on the organization's enforcement mode. A new organization starts in monitoring mode: the decorator registers the
capability the first time the function runs, AIM grants it, and the call runs and is recorded. AIM refuses the call
only in strict mode. An administrator turns strict mode on per organization: Global Enforcement Mode on
`/dashboard/admin/security-policies`, or `PUT /api/v1/admin/enforcement-settings` with
`{"enforcementMode": "strict"}`. Strict mode also refuses every action of an agent that is not verified, so verify
`my-agent` (**Verify agent** on its page in the dashboard) before you run the example.

```bash
pip install -e sdk/python/
```

```python
from aim_sdk import register_agent, AgentType

# Register agent with allowed capabilities
agent = register_agent(
    name="my-agent",
    capabilities=["db:read", "api:call", "file:read"],
    agent_type=AgentType.CLAUDE
)

# Use the decorator pattern to enforce capabilities at runtime
@agent.perform_action("db:read", resource="users_table")
def read_users():
    return db.query("SELECT * FROM users")

# In strict mode, an action not in the capabilities list is denied
@agent.perform_action("network:fetch", resource="external-api")
def fetch_external():
    return requests.get("https://external.example.com")

# In strict mode, read_users() succeeds and fetch_external() raises ActionDeniedError
try:
    read_users()
    print("db:read is allowed")
except Exception:
    print("db:read is denied")

try:
    fetch_external()
    print("network:fetch is allowed")
except Exception:
    print("network:fetch is denied by policy")
```

Expected output in strict mode:

```
db:read is allowed
network:fetch is denied by policy
```

In monitoring mode AIM allows `fetch_external()`, and its body runs.

The TypeScript SDK uses `checkCapability()` which returns a boolean, from the policy saved on the machine. The Python
SDK uses the `@agent.perform_action()` decorator, which raises `ActionDeniedError` when AIM refuses the call; in strict
mode AIM refuses a capability the agent was not granted.

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

## Per-Capability Trust Thresholds

Each capability definition on the server carries a minimum trust score (`minTrustScore`), set from its risk level:

| Risk Level | Min Trust Score | Example Capabilities |
|------------|----------------|---------------------|
| low        | 0% (no gate)   | `file:read`, `api:call` |
| medium     | 30%            | `db:write`, `file:write` |
| high       | 50%            | `file:delete`, `network:listen` |
| critical   | 70%            | `system:admin`, `user:impersonate` |

The server checks this minimum on every SDK verification (`POST /api/v1/sdk-api/verifications`) and on `POST /api/v1/agents/:id/verify-capability`, after it finds the capability among the agent's grants. A wildcard grant such as `file:*` is checked against the definition of the action requested, such as `file:delete`. In an organization in strict enforcement mode, an agent whose trust score is below the minimum is denied the action even though the capability is granted. In monitoring mode the action is allowed, as it is for every other check; the `reason` in the answer of `verify-capability` says the agent is below the minimum.

The definition used is the organization's own for that capability, else the core one. A capability outside the core set is registered as a medium-risk definition, with a 30% minimum, when it is first granted through `POST /api/v1/agents/:id/capabilities`. A capability with no definition has no minimum. `GET /api/v1/capabilities` lists the definitions with their minimums; no route changes a minimum.

## Execution Modes

Each granted capability has an execution mode that controls how actions are processed:

| Mode     | Behavior | Default For |
|----------|----------|-------------|
| `auto`   | Execute immediately | low, medium risk |
| `notify` | Execute + create alert | high risk |
| `review` | Queue for human approval | critical risk |

Set the execution mode when granting a capability via API:

```json
{
  "capabilityType": "payment:process",
  "executionMode": "review"
}
```

When `review` mode is active, `VerifyAction` returns `isAuthorized: false` with `denialReason: "pending_human_review"`. A capability request is created for admin approval.

## What You Now Have

- A declarative policy controlling what your agent can do
- Per-capability trust thresholds gating high-risk actions
- Execution modes for granular control (auto, notify, review)
- Runtime enforcement with non-zero exit codes on denial
- Every capability check recorded in the audit log

## Next Steps

- [Audit agent actions](audit-agent-actions.md) -- review what your agent has done
- [Embed in my app](embed-in-my-app.md) -- enforce capabilities programmatically in your code
- [Fleet governance](fleet-governance.md) -- manage policies across multiple agents
