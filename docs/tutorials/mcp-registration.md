# Register MCP Servers

**Time: 3 minutes** | **Difficulty: Beginner**

Register and attest MCP (Model Context Protocol) servers that your AI agents connect to. This enables drift detection and security monitoring for agent-to-MCP communications.

## Why Register MCP Servers?

- **Drift Detection** — Get alerts if agents connect to unregistered servers
- **Trust Scoring** — Attested MCP connections improve agent trust scores
- **Audit Trail** — Complete visibility into agent-MCP interactions
- **Capability Control** — Define what capabilities each MCP server provides

---

## Method 1: Via Dashboard (Recommended)

### Step 1: Navigate to MCP Servers

Open the AIM dashboard and click **MCP Servers** in the sidebar.

### Step 2: Click "Register Server"

Fill in the registration form:

- **Name:** Unique identifier (e.g., `filesystem-server`)
- **Display Name:** Human-readable name (e.g., "Filesystem MCP Server")
- **Server URL:** Connection URL (e.g., `stdio://filesystem-server`)
- **Capabilities:** What this server can do (e.g., read_file, write_file)
- **Description:** Optional description for documentation

### Step 3: Link to Agents

After registering, go to an agent's detail page and connect it to the MCP server. This tells AIM which servers each agent is allowed to use.

---

## Method 2: Via Python SDK

Register MCP servers programmatically when your agent starts:

```python
from aim_sdk import secure

# Initialize your agent
agent = secure("my-agent")

# Register an MCP server
mcp_result = agent.register_mcp(
    server_name="filesystem-server",
    server_url="stdio://filesystem-server",
    capabilities=["read_file", "write_file", "list_directory"]
)

print(f"MCP Server registered: {mcp_result['id']}")

# The agent is now linked to this MCP server
# AIM will detect if this agent connects to any OTHER servers
```

> **Auto-Detection:** If you're using Claude Desktop, AIM can automatically detect MCP servers from your `claude_desktop_config.json`. Just run `agent = secure("my-agent")` and they'll be registered automatically!

---

## Method 3: Via REST API

```bash
# Register an MCP server
curl -X POST "http://localhost:8080/api/v1/mcp-servers" \
  -H "Authorization: Bearer $AIM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github-server",
    "display_name": "GitHub MCP Server",
    "server_url": "stdio://github-server",
    "capabilities": ["read_repo", "create_issue", "create_pr"],
    "description": "GitHub integration for code operations"
  }'
```

```bash
# Link agent to MCP server
curl -X POST "http://localhost:8080/api/v1/agents/$AGENT_ID/mcp-servers/$MCP_ID" \
  -H "Authorization: Bearer $AIM_API_KEY"
```

---

## Common MCP Server Examples

| Server | URL Pattern | Typical Capabilities |
|--------|-------------|---------------------|
| Filesystem | `stdio://filesystem-server` | read_file, write_file, list_directory |
| GitHub | `stdio://github-server` | read_repo, create_issue, create_pr |
| Database | `stdio://postgres-server` | query, insert, update, delete |
| Slack | `stdio://slack-server` | send_message, read_channel |
| Browser | `stdio://puppeteer-server` | navigate, screenshot, click |

---

## Configuration Drift Detection

Once you register MCP servers for an agent, AIM monitors for drift:

### What Gets Detected

- Agent connects to an **unregistered** MCP server
- Agent uses capabilities not declared for that server
- MCP server URL changes unexpectedly

### What Happens

- Security alert created with HIGH severity
- Agent trust score reduced
- Admin notified via dashboard
- Event logged to audit trail

---

## MCP Attestation (Advanced)

For maximum security, you can cryptographically attest MCP servers:

```bash
# Attest an MCP server (generates cryptographic proof)
curl -X POST "http://localhost:8080/api/v1/mcp-servers/$MCP_ID/attest" \
  -H "Authorization: Bearer $AIM_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "public_key": "ed25519:your_mcp_server_public_key",
    "attestation_data": {
      "version": "1.0.0",
      "checksum": "sha256:abc123..."
    }
  }'
```

Attested servers:
- Have a verified cryptographic identity
- Cannot be spoofed or impersonated
- Improve connected agents' trust scores

---

## What's Next?

- **[MCP Attestation Guide](../guides/SECURITY.md#mcp-attestation)** - Cryptographically verify MCP servers
- **[Trust Scoring](../quick-start.md#step-5-see-it-work-instant-feedback)** - How MCP attestation affects trust scores
- **[Security Policies](../guides/SECURITY.md)** - Configure drift detection policies
- **[Alerts](./dashboard-walkthrough.md#step-5-security-dashboard)** - Manage security alerts

---

<div align="center">

[← Dashboard Walkthrough](./dashboard-walkthrough.md) | [Back to Tutorials](./README.md)

</div>
