# Dashboard Walkthrough

**Time: 3 minutes** | **Difficulty: Beginner**

A quick tour of the AIM dashboard. Learn where everything is and how to manage your agents, MCP servers, and security.

## Prerequisites

- AIM running at http://localhost:3000

---

## Step 1: Login

Open http://localhost:3000 and login with:

- **Email:** `admin@opena2a.org`
- **Password:** `AIM2025!Secure`

> **First login?** You'll be prompted to change your password for security.

---

## Step 2: Dashboard Overview

The main dashboard gives you a bird's eye view of your AI agent security:

| Card | Description |
|------|-------------|
| **Total Agents** | Number of registered AI agents in your organization |
| **MCP Servers** | Connected and attested MCP servers |
| **Average Trust Score** | Overall health of your agent fleet |
| **Active Alerts** | Security issues requiring attention |

---

## Step 3: Managing Agents

Navigate to **Agents** in the sidebar to:

- **View all agents** — See name, status, trust score, and last active time
- **Register new agent** — Click "Register Agent" to add one manually
- **Agent details** — Click an agent to see full details, activity, and credentials
- **Verify/Revoke** — Change agent status from pending → verified or revoke compromised agents

> **Pro tip:** Agents registered via SDK appear here automatically with status "pending" until verified.

---

## Step 4: MCP Server Management

Navigate to **MCP Servers** to manage Model Context Protocol servers:

- **Register MCP Server** — Add servers your agents connect to
- **Attestation Status** — See which servers are cryptographically verified
- **Connected Agents** — View which agents are using each MCP server
- **Capabilities** — Manage what capabilities each MCP server provides

---

## Step 5: Security Dashboard

Navigate to **Security** for threat monitoring:

- **Threats** — Active security threats detected by AIM
- **Anomalies** — Behavioral anomalies in agent activity
- **Alerts** — All security alerts with severity levels
- **Bulk Acknowledge** — Acknowledge all alerts at once when resolved

---

## Step 6: Settings

Navigate to **Settings** for configuration:

| Setting | Description |
|---------|-------------|
| **SDK Download** | Download the pre-configured Python SDK with your credentials embedded |
| **API Keys** | Generate and manage API keys for direct API access |
| **Security Policies** | Configure automated security rules and thresholds |
| **User Management** | Add team members and manage roles (Admin only) |

---

## Quick Actions Reference

| I want to... | Go to... |
|--------------|----------|
| Register a new agent | Agents → Register Agent |
| Verify a pending agent | Agents → Click agent → Verify |
| Add an MCP server | MCP Servers → Register Server |
| View security alerts | Security → Alerts |
| Download the SDK | Settings → SDK Download |
| Generate an API key | Settings → API Keys |
| Configure security policies | Settings → Security Policies |
| View audit logs | Admin → Audit Logs |
| Approve capability requests | Admin → Capability Requests |

---

## Key Metrics to Monitor

### Trust Score

- **0.8 - 1.0** — Excellent (green)
- **0.6 - 0.8** — Good (yellow)
- **0.4 - 0.6** — Needs attention (orange)
- **Below 0.4** — Critical (red)

### Alert Severity

- **Critical** — Immediate action required
- **High** — Address within hours
- **Medium** — Address within days
- **Low** — Informational

---

## What's Next?

- **[SDK Quickstart](./sdk-quickstart.md)** - Secure your first agent in 2 minutes
- **[Register MCP Servers](./mcp-registration.md)** - Connect and attest MCP servers
- **[Security Policies](../guides/SECURITY.md)** - Configure automated rules

---

<div align="center">

[← API Quickstart](./api-quickstart.md) | [Back to Tutorials](./README.md) | [MCP Registration →](./mcp-registration.md)

</div>
