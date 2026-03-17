# Use Case: Register My AI Agent with a Cryptographic Identity

**Time:** 2 minutes
**Prerequisites:** Node.js 18+, npm

## Problem

Your AI agent has no verifiable identity. Other systems cannot authenticate it, and there is no record of what it does.

## Step 1: Create an Identity

```bash
npx opena2a-cli identity create --name my-agent
```

Expected output:

```
Agent created:
  ID:         aim_7f3a9c2e
  Name:       my-agent
  Public Key: ed25519:x8Kp...mQ4R
  Stored:     ~/.opena2a/aim-core/identities/my-agent.json
  Audit Log:  ~/.opena2a/aim-core/audit.jsonl
```

This generates an Ed25519 keypair. The private key stays on your machine at `~/.opena2a/aim-core/identities/my-agent.json`. The audit log is created automatically.

## Step 2: Check Initial Trust Score

```bash
opena2a identity trust
```

Expected output:

```
Trust Score: 35/100 (D)

Factors:
  Identity Strength:      10/15  (Ed25519 key present)
  Capability Compliance:   0/15  (no policy loaded)
  Audit Completeness:      5/15  (log exists, no events)
  MCP Attestation:         0/10  (no tools attached)
  Policy Adherence:        0/10  (no policy)
  Lifecycle Status:       10/10  (active)
  Ownership Verification:  5/15  (key only, no attestation)
  Behavioral Analysis:     5/10  (insufficient data)

Recoverable: +50 by attaching tools and loading a policy
```

The score is low because the agent has no tool connections and no capability policy yet.

## Step 3: Attach to Detected Tools

```bash
opena2a identity attach --all
```

Expected output:

```
Scanning for tools...

Attached:
  claude-code    MCP server   ~/.claude/config.json
  cursor         IDE plugin   ~/.cursor/mcp.json

2 tools attached. Run 'opena2a identity trust' to see updated score.
```

This scans your environment for MCP servers, IDE plugins, and other agent tools, then links them to your identity.

## Step 4: Verify Improved Trust Score

```bash
opena2a identity trust
```

Expected output:

```
Trust Score: 55/100 (C)

Factors:
  Identity Strength:      10/15  (Ed25519 key present)
  Capability Compliance:   0/15  (no policy loaded)
  Audit Completeness:      5/15  (log exists, 2 events)
  MCP Attestation:        10/10  (2 tools verified)
  Policy Adherence:        0/10  (no policy)
  Lifecycle Status:       10/10  (active)
  Ownership Verification: 10/15  (key + tool attestation)
  Behavioral Analysis:    10/10  (consistent behavior)

Recoverable: +30 by loading a capability policy
```

## Using the SDK

You can perform all the steps above programmatically using the TypeScript or Python SDKs.

### TypeScript

```bash
npm install @opena2a/aim-core
```

```typescript
import { AIMCore } from '@opena2a/aim-core';

const aim = new AIMCore({ agentName: 'my-agent' });

// Step 1: Create an identity
const identity = aim.getIdentity();
console.log('Agent ID:', identity.agentId);
console.log('Public Key:', identity.publicKey);

// Step 2: Calculate trust score
const trust = aim.calculateTrust();
console.log(`Trust: ${trust.overall}`);
console.log('Factors:', JSON.stringify(trust.factors, null, 2));
```

Expected output:

```
Agent ID: aim_7f3a9c2e
Public Key: ed25519:x8Kp...mQ4R
Trust: 0.35
Factors: {
  "identity": 1,
  "capabilities": 0,
  "auditLog": 0.5,
  "secretsManaged": 0,
  "configSigned": 0,
  "skillsVerified": 0,
  "networkControlled": 0,
  "heartbeatMonitored": 0
}
```

### Python

```bash
pip install aim-sdk
```

```python
from aim_sdk import register_agent, AgentType

# Step 1: Register an agent (creates identity automatically)
agent = register_agent(
    name="my-agent",
    capabilities=["db:read", "api:call"],
    agent_type=AgentType.CLAUDE
)
print(f"Agent ID: {agent.agent_id}")
```

Expected output:

```
Agent ID: aim_7f3a9c2e
```

For a full SDK walkthrough including policies and event logging, see [Embed in my app](embed-in-my-app.md).

## What You Now Have

- An Ed25519 keypair stored locally
- A tamper-evident audit log recording identity events
- Tool attestations linking your agent to its runtime environment
- A trust score that other systems can query

## Next Steps

- [Audit agent actions](audit-agent-actions.md) -- track what your agent does
- [Enforce capabilities](enforce-capabilities.md) -- restrict what your agent can do
- [Embed in my app](embed-in-my-app.md) -- use the SDK directly in your code
