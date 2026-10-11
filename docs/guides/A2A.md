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

### Auto-Attestation

When an agent uses an MCP server for the first time, the SDK automatically creates an attestation:

```python
@agent.perform_action(capability="db:query", resource="database-mcp")
def query_database(sql: str):
    # First call creates attestation automatically
    return mcp_client.query(sql)
```

The attestation includes:
- Server identity (name, URL, public key)
- Available tools
- First seen timestamp
- Trust score

### Manual Attestation

Create explicit attestations for known-good configurations:

```python
from aim_sdk import AIMClient

client = AIMClient.from_sdk_token()

# Attest to an MCP server configuration
client.create_attestation(
    agent_id="my-agent-uuid",
    mcp_server_name="database-mcp",
    mcp_server_url="http://localhost:8000",
    tools=["query", "insert"],
    confidence_level="high"
)
```

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

When agents delegate authority to other agents, trust decays through the chain. This prevents deep delegation chains from bypassing trust requirements.

Each delegation specifies:
- `trustAttenuationFactor` (default 0.8): multiplied at each hop
- `minDelegatedTrustScore` (default 0.3): floor below which the chain is invalid

```
Human (trust: 1.0)
  └─ Orchestrator (effective: 0.8)    # 1.0 × 0.8
       └─ Researcher (effective: 0.64) # 0.8 × 0.8
            └─ Sub-agent (effective: 0.51) # 0.64 × 0.8
```

If any hop drops below the minimum, the delegation chain is rejected. Combined with per-capability trust thresholds, this means a deeply-delegated agent cannot execute high-risk capabilities even if it has been granted them.

The TypeScript SDK enforces this in `verifyDelegationChain()`:

```typescript
const result = await verifyDelegationChain(chain, { rootTrustScore: 0.95 });
// result.results[i].effectiveTrust shows trust at each hop
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
