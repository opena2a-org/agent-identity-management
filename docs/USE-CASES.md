# AIM Use cases

Practical guides organized by what you are trying to accomplish. Each guide includes exact commands, expected output, and estimated time.

| Use Case | Audience | Time |
|----------|----------|------|
| [Register my agent](use-cases/register-my-agent.md) | Any developer | 2 min |
| [Audit agent actions](use-cases/audit-agent-actions.md) | Any developer | 5 min |
| [Enforce capabilities](use-cases/enforce-capabilities.md) | Any developer | 5 min |
| [Embed in my app](use-cases/embed-in-my-app.md) | Library/framework developers | 10 min |
| [Fleet governance](use-cases/fleet-governance.md) | Teams managing multiple agents | 30 min |

## Where to Start

**Single agent, no infrastructure:** Start with [Register my agent](use-cases/register-my-agent.md). Everything runs locally using Ed25519 keys and file-based storage.

**Adding identity to your own framework:** Go to [Embed in my app](use-cases/embed-in-my-app.md) for programmatic access via `@opena2a/aim-core` or the Python SDK.

**Team with multiple agents:** Jump to [Fleet governance](use-cases/fleet-governance.md) for centralized audit, service tokens for agents, and dashboard-based management.

## What you can do

- Protect an agent's calls in Python: [Quick start](../README.md#quick-start).
- Protect them in Java: [Java quick start](../sdk/java/README.md#quick-start).
- Check capabilities and keep an audit log with no server: [local mode in TypeScript](../sdk/typescript/README.md#local-mode-with-opena2aaim-core).
- Audit a codebase: the [opena2a CLI's commands](https://github.com/opena2a-org/opena2a#commands).
- Register an agent from an app or the terminal: [Register an agent](use-cases/register-my-agent.md).
- Track what an agent does: [Audit agent actions](use-cases/audit-agent-actions.md).
- Restrict what an agent can do: [Enforce capabilities](use-cases/enforce-capabilities.md).
- Add identity to your agent framework: [Embed in an app](use-cases/embed-in-my-app.md).
- Govern many agents: [Fleet governance](use-cases/fleet-governance.md).
- Gate privileged actions, attest MCP servers and forward audit logs to Splunk: [Server features](server-features.md).
- Run AIM on your own machines: [Install AIM (self-hosted)](../README.md#install-aim-self-hosted).
