# Agent-to-Agent (A2A) Communication

AIM supports secure agent-to-agent (A2A) communication patterns where AI agents collaborate, delegate tasks, or share information with other agents and services. This guide covers how to configure, secure, and monitor A2A interactions.

## Overview

Modern AI applications often involve multiple agents working together:

- **Orchestrator agents** that delegate to specialized agents
- **Multi-agent systems** (CrewAI, AutoGen, LangGraph) with peer communication
- **Agents connecting to MCP servers** that may host other agents
- **Agent pipelines** where outputs flow between agents

AIM provides visibility and control over these interactions through:

1. **MCP Server Registration**: Declare which MCP servers your agent communicates with
2. **Agent Connections**: Track agent-to-agent relationships
3. **Attestation**: Verify the integrity of connected services
4. **Supply Chain Analytics**: Visualize and audit the full agent ecosystem

## Declaring Agent Connections

### Python SDK

Use the `mcp_servers` parameter to declare which services your agent communicates with:

```python
from aim_sdk import secure, AgentType

# Simple declaration
agent = secure(
    "orchestrator-agent",
    agent_type=AgentType.CREWAI,
    capabilities=["task:delegate", "agent:invoke"],
    mcp_servers=["worker-agent-1", "worker-agent-2", "database-mcp"]
)
```

#### Auto-Detection of MCP Servers

The SDK can auto-detect MCP servers from your Claude Desktop configuration:

```python
# Opt-in to MCP auto-detection
agent = secure(
    "my-agent",
    auto_detect_mcp=True  # Scans Claude config for MCP servers
)
```

This scans:
- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`

### Java SDK

```java
import org.opena2a.aim.client.AIMClient;
import org.opena2a.aim.client.AgentType;
import java.util.Arrays;

AIMClient agent = AIMClient.secure(
    "orchestrator-agent",
    Arrays.asList("task:delegate", "agent:invoke"),
    AgentType.CREWAI,
    Arrays.asList("worker-agent-1", "worker-agent-2", "database-mcp"),  // talksTo
    "Orchestrator agent for task delegation",
    Arrays.asList("production"),
    null  // metadata
);
```

### TypeScript SDK

```typescript
import { AIMClient, AgentType } from '@opena2a/aim-sdk';

const client = new AIMClient({
  baseUrl: 'https://aim.example.com',
  apiKey: process.env.AIM_API_KEY,
});

const agent = await client.registerAgent({
  name: 'orchestrator-agent',
  agentType: AgentType.LANGGRAPH,
  capabilities: ['task:delegate', 'agent:invoke'],
  talksTo: ['worker-agent-1', 'worker-agent-2', 'database-mcp'],
  description: 'Orchestrator agent for task delegation',
});
```

## MCP Server Registration

Register MCP servers that agents connect to:

### Via Dashboard

1. Navigate to **MCP Servers** in the dashboard
2. Click **Add MCP Server**
3. Enter server details (name, URL, description)
4. Optionally add public key for attestation

### Via API

```bash
curl -X POST https://aim.example.com/api/v1/mcp-servers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "database-mcp",
    "displayName": "Database MCP Server",
    "url": "http://localhost:8000",
    "publicKey": "BASE64_PUBLIC_KEY",
    "tools": ["query", "insert", "update"],
    "description": "MCP server for database operations"
  }'
```

## Attestation and Trust

### Attesting an MCP Server

An attestation is an agent's signed statement that it has checked an MCP server. AIM records one only when it is submitted to the server. `@agent.perform_action` does not attest, and naming an MCP server as its `resource` creates no attestation.

In the Python SDK the call is `attest_mcp()`. It requests a challenge from `GET /api/v1/mcp-servers/{id}/challenge`, signs the attestation and the challenge with the agent's Ed25519 key, and submits both to `POST /api/v1/mcp-servers/{id}/attest`. An agent registered in API-key mode has no signing key and cannot attest. The Java SDK's `AIMClient.attestMcp()` follows the same flow.

```python
from aim_sdk import secure

agent = secure("my-agent")

# mcp_server_id is the server's ID in AIM
agent.attest_mcp(
    mcp_server_id="550e8400-e29b-41d4-a716-446655440000",
    capabilities=["query", "insert"],
    mcp_url="http://localhost:8000",
    mcp_name="database-mcp",
)
```

The signed attestation carries the agent ID, the server's URL and name, the tools the agent found, whether the connection and the health check succeeded, the connection latency, a timestamp, the SDK version and the challenge.

`use_mcp_tool()` makes the call for you when it records a tool use. Given `mcp_url`, it attests on the agent's first use of the server, on the first use of a tool it has neither used nor attested, when its last attestation is more than 24 hours old or its recorded time cannot be read, and when discovery finds that the server's tools have changed. Pass `auto_attest=False` to turn this off.

```python
agent.use_mcp_tool(
    server_id="550e8400-e29b-41d4-a716-446655440000",
    tool_name="query",
    mcp_url="http://localhost:8000",
    mcp_name="database-mcp",
)
```

A member of the organization can attest without an agent key from the MCP server's page in the dashboard, or with `POST /api/v1/mcp-servers/{id}/manual-attest`.

### Trust Verification

Before connecting to an MCP server, verify its trust status:

```python
# Check if MCP server is trusted
trust_info = client.get_mcp_trust_status("database-mcp")

if trust_info["trust_score"] < 0.7:
    raise SecurityError("MCP server trust score too low")

if trust_info["drift_detected"]:
    raise SecurityError("MCP server configuration has drifted")
```

## Multi-Agent Patterns

### Pattern 1: Orchestrator with Workers

```python
from aim_sdk import secure, AgentType

# Orchestrator agent
orchestrator = secure(
    "orchestrator",
    agent_type=AgentType.LANGGRAPH,
    capabilities=["task:plan", "agent:invoke"],
    mcp_servers=["researcher", "writer", "reviewer"]
)

# Worker agents
researcher = secure(
    "researcher",
    agent_type=AgentType.LANGCHAIN,
    capabilities=["web:search", "doc:read"]
)

writer = secure(
    "writer",
    agent_type=AgentType.LANGCHAIN,
    capabilities=["content:generate"]
)
```

### Pattern 2: Peer-to-Peer Agents

```python
# Agent A can talk to Agent B and vice versa
agent_a = secure(
    "agent-a",
    capabilities=["data:process"],
    mcp_servers=["agent-b"]
)

agent_b = secure(
    "agent-b",
    capabilities=["data:validate"],
    mcp_servers=["agent-a"]
)
```

### Pattern 3: Hub and Spoke

```python
# Central hub agent
hub = secure(
    "central-hub",
    capabilities=["route:message", "aggregate:results"],
    mcp_servers=["spoke-1", "spoke-2", "spoke-3", "spoke-4"]
)

# Spoke agents (don't talk to each other, only to hub)
for i in range(1, 5):
    spoke = secure(
        f"spoke-{i}",
        capabilities=["task:execute"],
        mcp_servers=["central-hub"]
    )
```

### Delegation Trust Attenuation

Trust decays along a delegation chain only where the TypeScript SDK's `verifyDelegationChain()` checks the chain. The AIM server does not apply attenuation: it stores a trust attenuation factor (default 0.8) and a minimum delegated trust score (default 0.3) for each delegation in its `a2a_delegation` table, and no server code reads them. Neither the Python SDK nor the Java SDK applies attenuation.

`verifyDelegationChain()` takes two optional settings from its caller, the verifier, in its options:
- `trustAttenuation` (default 0.8): the factor that turns one delegation's trust into the next one's
- `minDelegatedTrust` (default 0.3): the floor a later delegation's trust must not fall below

Each must be a number from 0 to 1; any other value fails the chain before a delegation is checked. The first delegation keeps the trust passed as `rootTrustScore` (default 1.0). Each later delegation's effective trust is the previous one's multiplied by `trustAttenuation`. A delegation whose effective trust falls below `minDelegatedTrust` carries an error, and the chain is not valid. With the defaults:

```
root               effectiveTrust 1.0
  subdelegation-1  effectiveTrust 0.8     # 1.0 × 0.8
  subdelegation-2  effectiveTrust 0.64    # 0.8 × 0.8
  subdelegation-3  effectiveTrust 0.512   # 0.64 × 0.8
```

A delegation can also carry fields named `trustAttenuation` and `minDelegatedTrust`. `createDelegation()` sets neither field, and the SDK leaves both out of the signed payload, so whoever passes a chain on can change them without failing verification. `verifyDelegationChain()` does not read them: a chain cannot raise the trust it is given or lower the floor it is held to.

```typescript
const result = await verifyDelegationChain(chain, {
  rootTrustScore: 0.95,
  trustAttenuation: 0.8,
  minDelegatedTrust: 0.3,
});
// result.valid is false when any delegation fails a check, the trust floor included
// result.results[i].effectiveTrust is the trust at delegation i
```

## Supply Chain Analytics

AIM provides comprehensive visibility into your agent ecosystem:

### Supply Chain Data

View your organization's agent-to-MCP-server connections and attestations:

```bash
# Supply chain analytics via API
curl -X GET "https://aim.example.com/api/v1/supply-chain/analytics?days=30" \
  -H "Authorization: Bearer $TOKEN" \
  -o supply-chain.json
```

The answer has three members:
- `stats`: supply chain totals
- `attestationTrend`: attestations over the period
- `connections`: agent-to-MCP-server connections, with details

`GET /api/v1/supply-chain/drift-alerts` lists MCP capabilities that were added, removed or not verified recently. The server serves no agent bill of materials (ABOM) export.

### Dependency Graph

Visualize agent relationships in the dashboard:

1. Navigate to **Supply Chain** → **Dependency Graph**
2. View agent-to-MCP connections
3. Identify critical paths and single points of failure
4. Detect orphaned or unused agents

### Drift Detection

AIM automatically detects when agent configurations change:

```python
# Get drift alerts
drift_alerts = client.get_drift_alerts(agent_id="my-agent-uuid")

for alert in drift_alerts:
    print(f"Drift detected: {alert['type']}")
    print(f"  Before: {alert['previous_value']}")
    print(f"  After: {alert['current_value']}")
    print(f"  Severity: {alert['severity']}")
```

## Security Best Practices

### 1. Principle of Least Privilege

Only declare connections that are actually needed:

```python
# Good: Specific connections
agent = secure(
    "my-agent",
    mcp_servers=["database-mcp"]  # Only what's needed
)

# Avoid: Overly broad connections
agent = secure(
    "my-agent",
    mcp_servers=["*"]  # Don't do this
)
```

### 2. Verify Before Connect

Check MCP server trust before sensitive operations:

```python
@agent.perform_action(capability="payment:process")
def process_payment(amount: float):
    # Verify MCP server trust first
    trust = client.get_mcp_trust_status("payment-mcp")
    if trust["trust_score"] < 0.9:
        raise SecurityError("Payment MCP server trust insufficient")

    return payment_mcp.process(amount)
```

### 3. Monitor Trust Drops

Webhooks carry no event for an MCP connection or for capability drift. A webhook subscribed to
`trust_score.low` and `agent.suspended` is told when an agent's trust score falls below 70% and when the
trust score policy suspends the agent. Event delivery is not switched on in the server yet; see
[Webhook Events](../API.md#webhook-events) for what is sent and when.

```bash
curl -X POST http://localhost:8080/api/v1/webhooks \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "trust-drops", "url": "https://your-app.com/webhooks/aim", "events": ["trust_score.low", "agent.suspended"]}'
```

### 4. Regular Attestation Review

Periodically review and re-attest MCP servers:

```python
# Get all attestations for an agent
attestations = client.get_agent_attestations(agent_id="my-agent-uuid")

for att in attestations:
    if att["age_days"] > 30:
        print(f"Stale attestation: {att['mcp_server_name']}")
        # Consider re-attesting or removing
```

## Compliance and Audit

### Audit Trail

All A2A interactions are logged:

```python
# Query agent communication history
audit_logs = client.get_audit_logs(
    agent_id="my-agent-uuid",
    event_types=["mcp.connection", "attestation.created"],
    start_date="2025-01-01",
    end_date="2025-01-31"
)
```

### Compliance Reports

Download your organization's compliance report (an administrator's session; `format` is `csv` or `json`):

```bash
# Export compliance report
curl -X GET "https://aim.example.com/api/v1/compliance/export?format=json" \
  -H "Authorization: Bearer $TOKEN" \
  -o compliance-report.json
```

## Framework Integration

### CrewAI

```python
from crewai import Crew, Agent
from aim_sdk import secure

# Wrap CrewAI agents with AIM
aim_agent = secure(
    "crewai-researcher",
    agent_type="crewai",
    capabilities=["research:web", "research:docs"],
    mcp_servers=["web-search-mcp", "document-mcp"]
)

researcher = Agent(
    role="Researcher",
    goal="Research topics thoroughly",
    backstory="Expert researcher"
)
```

### AutoGen

```python
from autogen import AssistantAgent
from aim_sdk import secure

aim_agent = secure(
    "autogen-assistant",
    agent_type="autogen",
    capabilities=["code:generate", "code:review"],
    mcp_servers=["code-execution-mcp"]
)

assistant = AssistantAgent(
    name="assistant",
    llm_config={"model": "gpt-4"}
)
```

### LangGraph

```python
from langgraph.graph import StateGraph
from aim_sdk import secure

aim_agent = secure(
    "langgraph-orchestrator",
    agent_type="langgraph",
    capabilities=["workflow:orchestrate"],
    mcp_servers=["task-mcp", "storage-mcp"]
)

graph = StateGraph(...)
```

## Troubleshooting

### Connection Not Registered

If MCP connections aren't appearing:

```python
# Force MCP discovery
agent = secure(
    "my-agent",
    auto_detect_mcp=True,  # Enable auto-detection
    mcp_servers=["known-server"]  # Also add known servers
)
```

### Attestation Failures

If attestation is failing:

```python
# Check MCP server reachability
try:
    response = client.verify_mcp_server("database-mcp")
    print(f"Server status: {response['status']}")
except Exception as e:
    print(f"Server unreachable: {e}")
```

### Trust Score Too Low

If trust scores are unexpectedly low:

```python
# Get detailed trust breakdown
trust_details = client.get_mcp_trust_details("database-mcp")

for factor in trust_details["factors"]:
    print(f"{factor['name']}: {factor['score']} (weight: {factor['weight']})")
```

## Related Documentation

- [MCP Server Registration Tutorial](../tutorials/mcp-registration.md)
- [Trust Scoring Algorithm](../sdk/trust-scoring.md)
- [Supply Chain Dashboard Guide](../guides/SECURITY.md)
