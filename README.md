# [OpenA2A AIM (Agent Identity Management)](https://opena2a.org/agent-identity-management)

[![Status: stable](https://img.shields.io/badge/status-stable-brightgreen)](./STATUS.md) [![CI](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml) [![Security](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml) [![Docker](https://img.shields.io/docker/pulls/opena2a/aim-server?label=docker%20pulls)](https://hub.docker.com/r/opena2a/aim-server) [![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

> **[OpenA2A](https://github.com/opena2a-org/opena2a)**: [CLI](https://github.com/opena2a-org/opena2a) · [HackMyAgent](https://github.com/opena2a-org/hackmyagent) · [Secretless](https://github.com/opena2a-org/secretless-ai) · [AIM](https://github.com/opena2a-org/agent-identity-management) · [Browser Guard](https://github.com/opena2a-org/AI-BrowserGuard) · [DVAA](https://github.com/opena2a-org/damn-vulnerable-ai-agent)

Cryptographic identity, capability authorization, and audit trails for AI agents. Apache 2.0. [Website](https://opena2a.org) · [AIM Cloud](https://aim.opena2a.org/get-started) · [Discord](https://discord.gg/uRZa3KXgEn)

## Quick start
```bash
pip install "aim-sdk[rich]"
aim-sdk login                              # OAuth to aim.opena2a.org
aim-sdk login --url http://localhost:8080  # or to your self-hosted AIM
```

Then register an agent, in a Python session, with the capability it holds. What `secure()` prints, trimmed ([full output](docs/captures/quick-start.txt)):

```python
from aim_sdk import secure
agent = secure("my-first-agent", capabilities=["db:read"])
```
```
  ○ Agent Type: using default 'ai_agent'

[OK] Agent registered: my-first-agent
  ID: 14466d98...cd30  Type: ai_agent  Version: 1.0.0
  Status: ○ pending  Trust: 59%
  Capabilities: db:read
```

The first request after sign-in presents the refresh token `aim-sdk login` saved. A server with Redis records it as revoked, answers with a new one, and the SDK prints a line: `Token rotated successfully — old refresh token revoked (new id: 461d130d...)`. Without Redis the server logs `Token revocation disabled (no Redis configured)` and the SDK prints no such line.

`aim-sdk login` uses the OAuth 2.0 device grant (RFC 8628): the CLI prints a code and you approve it on your dashboard's `/device` page. `secure()` generates an Ed25519 keypair, registers the agent with the AIM backend, and stores credentials at `~/.aim/`.

The SDK built from this repository printed `[OK]` above where the release on PyPI prints `✓`. Without the `rich` extra (`pip install aim-sdk`), the same fields print in a panel.

The agent starts `pending` until an administrator verifies it under Agents in the dashboard; on a strict-mode organization every call is refused until then. Once it is verified, protect a function with the capability the agent holds and call it:

```python
customers = {42: {"name": "Ada Lovelace", "plan": "team"}}

@agent.perform_action(capability="db:read")
def get_customer(customer_id):
    return customers[customer_id]

get_customer(42)
```

`@perform_action` signs every invocation, has the server check it against the capabilities granted to the agent and the organization's security policies, and records the outcome in the audit log.

The refusal below needs strict mode, one setting an administrator turns on per organization: in the dashboard, Security, then Policies, then set Global Enforcement Mode to Strict (the API is `PUT /api/v1/admin/enforcement-settings` with `{"enforcementMode": "strict"}`). On a new organization's default monitoring mode the `db:write` call runs and deletes the record.

Now call something the agent was not granted. `my-first-agent` holds `db:read` only:

```python
@agent.perform_action(capability="db:write")
def delete_customer(customer_id):
    del customers[customer_id]

delete_customer(42)
```

AIM refuses it before the function body runs. What the call prints, trimmed ([full output](docs/captures/quick-start.txt)):

```
Capability 'db:write' pending admin approval (strict mode)
aim_sdk.exceptions.ActionDeniedError: AIM denied 'db:write': Capability violation blocked by security policy 'Capability Violation Detection': Agent does not have permission for capability 'db:write' (allowed: [db:read]). The action was blocked and not executed. AIM denies an action after applying your organization's enforcement mode (currently strict), so a denial blocks in every mode -- monitoring mode governs verifications AIM could not answer, not ones it refused. To permit this capability, grant it to this agent in the AIM dashboard under Agents. If the agent should not be denied at all, check its status there: an agent marked compromised, suspended or unverified is refused regardless of its capabilities.
```

The decorator files a capability request for `db:write`, the server refuses the call, and the agent's audit log records the denial with the same reason. The release on PyPI also prints `✓ Registered db:write (medium)` after the first line; the SDK built from this repository does not.

A stack started from the quickstart defaults has no security policy configured, so there the reason names AIM's built-in `default_policy` where the output above reads 'Capability Violation Detection'. An `ActionDeniedError` is a `PermissionError`, so an existing `except PermissionError` handler catches it.

The 9-factor trust score is calculated when an agent registers. It is recalculated when a manager verifies, suspends or reactivates the agent, grants or revokes one of its capabilities, or calls `POST /api/v1/agents/:id/trust-score/recalculate`.

A verification request (`POST /api/v1/verifications`) does not recalculate the score. From an SDK the same request goes to `POST /api/v1/sdk-api/verifications`; the two routes share a handler. A request for a capability the agent was granted leaves the score unchanged. When a verified agent that holds at least one granted capability, and is not marked compromised, requests a capability outside its grants, the score drops by a fixed 0.10 if the request is blocked and 0.05 if it is let through, never below 0.

New to AIM? The [SDK quickstart tutorial](https://opena2a.org/docs/tutorials/sdk-quickstart) walks through this end to end. The same one-line shape works in [Java](#java). TypeScript has no `secure()` call: `@opena2a/aim-core` checks a capability against a local policy with no server, and the server client `@opena2a/aim-sdk` registers an agent and verifies each action with separate calls ([TypeScript](sdk/typescript/README.md#local-mode-with-opena2aaim-core)).

## What you can do

- Register an agent and protect its calls from Python: [Quick start](#quick-start).
- Do the same from Java: [Java quick start](sdk/java/README.md#quick-start).
- Check capabilities and keep an audit log on one machine, with no server: [local mode in TypeScript](sdk/typescript/README.md#local-mode-with-opena2aaim-core).
- Audit an existing codebase with no server: the [opena2a CLI's commands](https://github.com/opena2a-org/opena2a#commands).
- Register an agent from an application or the terminal: [Register an agent](docs/use-cases/register-my-agent.md).
- Track what an agent does: [Audit agent actions](docs/use-cases/audit-agent-actions.md).
- Restrict what an agent can do: [Enforce capabilities](docs/use-cases/enforce-capabilities.md).
- Add identity to your own agent framework: [Embed in an app](docs/use-cases/embed-in-my-app.md).
- Manage identity across many agents: [Fleet governance](docs/use-cases/fleet-governance.md).
- Gate privileged actions, attest MCP servers and forward audit logs to Splunk from the server: [Server features](docs/server-features.md).
- Run AIM on your own machines: [Install AIM (self-hosted)](#install-aim-self-hosted).
- Watch the same boundary stop a prompt injection, with and without AIM: the [A/B demo](https://github.com/opena2a-org/damn-vulnerable-ai-agent#aim-protected-agent).

## Three deployment modes

| Mode | When | Includes |
|---|---|---|
| **AIM Cloud** | Managed, fastest path | Production-managed at [aim.opena2a.org/get-started](https://aim.opena2a.org/get-started). Python and Java SDKs work out of the box. |
| **Self-hosted** | Team or fleet, your infrastructure | All AIM Cloud features. PostgreSQL audit, REST API, dashboard, OAuth, 5-step FGA, 9-factor trust score, MCP attestation, PAM, SIEM adapters. |
| **Local-only** | Solo developer, single machine, no server | TypeScript SDK + opena2a CLI. Ed25519 keypair, `audit.jsonl`, YAML capability policies, 8-factor local trust score, cross-tool event bridges. Local mode is in the TypeScript SDK only. |

All three share the same audit-event schema. Local agents can push history to a server via `AIMCore.enableReporting()`.

## SDKs

| SDK | Install | Mode | API |
|---|---|---|---|
| Python | `pip install aim-sdk` | Server (today) | `secure("name")` + `@perform_action` |
| Java | `cd sdk/java && mvn install` (from source) | Server | `AIMClient.secure("name")` + `@SecureAction` |
| TypeScript | `npm install @opena2a/aim-core` | Local or server | `new AIMCore({ agentName })` |

Working examples for all three live in [`examples/`](examples/). The SDKs send requests only to the AIM server you signed in to; the causal-denial telemetry each SDK README describes is opt-in and off until you configure it. Framework auto-detection and risk levels for Python: [`sdk/python/README.md`](sdk/python/README.md). Local mode and the server client for TypeScript: [`sdk/typescript/README.md`](sdk/typescript/README.md).

### Java

```java
AIMClient agent = AIMClient.secure("my-first-agent");

@SecureAction(capability = "db:read", resource = "users_table")
public User getCustomer(String customerId) {
    return userRepository.findById(customerId);
}
```

Built from source. Same Ed25519 signing, same audit trail; AspectJ wraps `@SecureAction` invocations. See [`sdk/java/README.md`](sdk/java/README.md).

## Install AIM (self-hosted)

```bash
curl -sSLO https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/scripts/quickstart.sh
bash quickstart.sh
```

Brings up `aim-server`, `aim-dashboard`, PostgreSQL, and Redis. Dashboard at `localhost:3000`, API at `localhost:8080`. Login credentials print at the end of the run.

The script is fetched from `main` and starts the `aim-server` and `aim-dashboard` images at their `latest` tags. Nothing on this path is pinned or verified: read [the script](scripts/quickstart.sh) and [the compose file](docker-compose.quickstart.yml) it fetches before running it. New accounts wait for an administrator's approval; set `AIM_PLATFORM_ADMINS` before starting the backend to bootstrap the first one.

Image tags and withdrawn tags, building from source, and the OpenTelemetry demo stack: [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md#install-from-this-repository). Production deployment (Azure, GCP, AWS): [infrastructure/DEPLOYMENT.md](infrastructure/DEPLOYMENT.md).

## Documentation

- [See it work](docs/quick-start.md#see-it-work): the audit log behind the two calls above, and where to grant `db:write`.
- [Server features](docs/server-features.md): 5-step Fine-Grained Authorization, trust scoring, MCP attestation, Privileged Access Management, CyberArk, SIEM adapters, the web dashboard.
- [Deployment](docs/DEPLOYMENT.md), [Verifying the `@opena2a/aim-core` package](docs/verifying-packages.md), [Use cases](docs/USE-CASES.md), [Demos](docs/DOCUMENTATION_INDEX.md#demos) and the [A/B recording](docs/images/aim-ab-demo.gif).
- [opena2a CLI commands](https://github.com/opena2a-org/opena2a#commands): auditing a codebase, hardening configs, monitoring runtime events, with no server required.
- [SDK quickstart tutorial](https://opena2a.org/docs/tutorials/sdk-quickstart), [MCP registration](https://opena2a.org/docs/tutorials/mcp-registration), the [documentation site](https://opena2a.org/docs), [Roadmap](ROADMAP.md), [Research](https://research.opena2a.org).
- [Contributing](CONTRIBUTING.md) has the dev loop and the pull request process. Security issues go to `info@opena2a.org` under coordinated disclosure; [SECURITY.md](SECURITY.md) says what to expect and when.

## License

Apache-2.0. See [LICENSE](LICENSE).
