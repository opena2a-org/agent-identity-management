# OpenA2A AIM (Agent Identity Management)

[![Status: stable](https://img.shields.io/badge/status-stable-green)](STATUS.md)
[![Docker](https://img.shields.io/docker/pulls/opena2a/aim-server?label=docker%20pulls)](https://hub.docker.com/r/opena2a/aim-server)
[![CI](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/ci.yml)
[![Security](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml/badge.svg)](https://github.com/opena2a-org/agent-identity-management/actions/workflows/security.yml)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

> **[OpenA2A](https://github.com/opena2a-org)**: [CLI](https://github.com/opena2a-org/opena2a) · [HackMyAgent](https://github.com/opena2a-org/hackmyagent) · [Secretless](https://github.com/opena2a-org/secretless-ai) · **AIM** · [Browser Guard](https://github.com/opena2a-org/ai-browserguard) · [DVAA](https://github.com/opena2a-org/damn-vulnerable-ai-agent)

[Website](https://opena2a.org/agent-identity-management) · [AIM Cloud](https://aim.opena2a.org/get-started) · [Discord](https://discord.gg/uRZa3KXgEn)

Cryptographic identity, capability authorization, and audit trails for AI agents.

## Quick start
```bash
pip install "aim-sdk[rich]"
aim-sdk login                              # OAuth to aim.opena2a.org
aim-sdk login --url http://localhost:8080  # or to your self-hosted AIM
```

The `[rich]` extra is optional and formats the output shown below; plain `pip install aim-sdk` works and prints the same fields in a panel. Then register an agent, in a Python session, with the capability it holds. What `secure()` prints, trimmed ([full output](docs/captures/quick-start.txt)):

```python
from aim_sdk import secure
agent = secure("my-first-agent", capabilities=["db:read"])
```
```
  ○ Agent Type: using default 'ai_agent'

[OK] Agent registered: my-first-agent
  ...
  Status: ○ pending  Trust: 59%
  Capabilities: db:read
```

`aim-sdk login` prints a code you approve on your dashboard's `/device` page; the PyPI release does not yet ([self-hosted sign-in](docs/quick-start.md#sign-in-from-your-dashboard), [#789](https://github.com/opena2a-org/agent-identity-management/issues/789)). `secure()` generates an Ed25519 keypair and stores credentials at `~/.aim/`.

The agent starts `pending` until an administrator verifies it under Agents in the dashboard. On a new organization, which starts in monitoring mode, a pending agent's call runs and raises an `Unverified Agent Action (Monitoring Mode)` alert. On a strict-mode organization every call is refused with `Agent not verified - all actions denied` until then.

Once it is verified, protect a function with that capability and call it:

```python
customers = {42: {"name": "Ada Lovelace", "plan": "team"}}

@agent.perform_action(capability="db:read")
def get_customer(customer_id):
    return customers[customer_id]

get_customer(42)
```

`@perform_action` signs each call, has the server check it against the agent's granted capabilities and the organization's security policies, and logs the outcome.

The refusal below needs strict mode (dashboard: Security, then Policies, Global Enforcement Mode: Strict); in the default monitoring mode, `db:write` runs and deletes the record.

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

The decorator files a request for `db:write`, and the agent's audit log records the denial with the same reason. On a stack started from the quickstart defaults, the reason names the built-in `default_policy` instead of 'Capability Violation Detection'.

The SDK built from this repository printed the registration capture, and the release on PyPI printed the refusal; the release on PyPI prints `✓` where this repository's build prints `[OK]`, and adds `✓ Registered db:write (medium)` after the refusal's first line.

The trust score is calculated at registration and recalculated when a manager verifies, suspends or reactivates the agent, grants or revokes a capability, or calls `POST /api/v1/agents/:id/trust-score/recalculate`, and every hour by default by the trust re-evaluation job (`TRUST_REEVALUATION_INTERVAL`). What a verification request does to it: [Trust-gated capabilities](docs/server-features.md#trust-gated-capabilities).

The same one-line shape works in [Java](#java); TypeScript has no `secure()` call ([local mode](sdk/typescript/README.md#local-mode-with-opena2aaim-core)).

## What you can do

- Protect an agent's calls in Python: [Quick start](#quick-start).
- Protect them in Java: [Java quick start](sdk/java/README.md#quick-start).
- Check capabilities and keep an audit log with no server: [local mode in TypeScript](sdk/typescript/README.md#local-mode-with-opena2aaim-core).
- Register an agent from an app or the terminal: [Register an agent](docs/use-cases/register-my-agent.md).
- Track what an agent does: [Audit agent actions](docs/use-cases/audit-agent-actions.md).
- Restrict what an agent can do: [Enforce capabilities](docs/use-cases/enforce-capabilities.md).
- Add identity to your agent framework: [Embed in an app](docs/use-cases/embed-in-my-app.md).
- Govern many agents: [Fleet governance](docs/use-cases/fleet-governance.md).
- Gate privileged actions, attest MCP servers and forward audit logs to Splunk: [Server features](docs/server-features.md).

All 11 jobs: [docs/USE-CASES.md](docs/USE-CASES.md#what-you-can-do).

## Deployment

| Mode | When | Includes |
|---|---|---|
| **AIM Cloud** | Managed | Hosted at [aim.opena2a.org/get-started](https://aim.opena2a.org/get-started). Python and Java SDKs work out of the box. |
| [**Self-hosted**](#install-aim-self-hosted) | Your infrastructure | PostgreSQL audit, REST API, dashboard, OAuth, 5-step FGA, 9-factor trust score, MCP attestation, PAM, audit log forwarding to Splunk. |
| [**Local-only**](sdk/typescript/README.md#local-mode-with-opena2aaim-core) | One machine, no server | TypeScript SDK + opena2a CLI. 8-factor local trust score. Local mode is in the TypeScript SDK only. |

Local only mode keeps its own event format in `~/.opena2a/aim-core/audit.jsonl`, separate from the server's audit log. It includes an Ed25519 keypair and YAML capability policies.

## SDKs

| SDK | Install | Mode | API |
|---|---|---|---|
| Python | `pip install aim-sdk` | Server | `secure("name")` + `@perform_action` |
| Java | `cd sdk/java && mvn install` (from source) | Server | `AIMClient.secure("name")` + `@SecureAction` |
| TypeScript | `npm install @opena2a/aim-core` | Local | `new AIMCore({ agentName })` |

Each SDK's README: [Python](sdk/python/README.md), [Java](sdk/java/README.md), [TypeScript](sdk/typescript/README.md); examples for all three are in [`examples/`](examples/). The SDKs send requests only to the AIM server you signed in to; the causal-denial telemetry the Python and TypeScript SDK READMEs describe is opt-in and off until you configure it; the Java SDK has none.

### Java

```java
AIMClient agent = AIMClient.secure("my-first-agent");

@SecureAction(capability = "db:read", resource = "users_table")
public User getCustomer(String customerId) {
    return userRepository.findById(customerId);
}
```

AspectJ wraps each `@SecureAction` invocation with the same Ed25519 signing and audit trail.

## Install AIM (self-hosted)

```bash
curl -sSLO https://raw.githubusercontent.com/opena2a-org/agent-identity-management/main/scripts/quickstart.sh
bash quickstart.sh
```

It needs Docker running, `openssl` and `curl`, and writes the compose file and a `.env` of generated secrets to `./aim`. Brings up `aim-server`, `aim-dashboard`, PostgreSQL, and Redis. Dashboard at `localhost:3000`, API at `localhost:8080`. Login credentials print at the end of the run.

The script runs from `main` and starts the `aim-server` and `aim-dashboard` images at their `latest` tags; nothing on this path is pinned or verified, so read [the script](scripts/quickstart.sh) and [the compose file](docker-compose.quickstart.yml) first.

Image tags, building from source, observability and the first administrator on other setups: [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md#install-from-this-repository). Production on Azure, GCP or AWS: [infrastructure/DEPLOYMENT.md](infrastructure/DEPLOYMENT.md).

## Documentation

- [See it work](docs/quick-start.md#see-it-work): the audit log behind the two calls above, where to grant `db:write`, and the Quick start's notes in full.
- [Server features](docs/server-features.md), [Deployment](docs/DEPLOYMENT.md), [Verifying the `@opena2a/aim-core` package](docs/verifying-packages.md), [Use cases](docs/USE-CASES.md), [Demos](docs/DOCUMENTATION_INDEX.md#demos) and the [A/B recording](docs/images/aim-ab-demo.gif).
- [opena2a CLI commands](https://github.com/opena2a-org/opena2a#commands): harden configs and monitor runtime events, with no server.
- [SDK quickstart tutorial](https://opena2a.org/docs/tutorials/sdk-quickstart), [MCP registration](https://opena2a.org/docs/tutorials/mcp-registration), the [documentation site](https://opena2a.org/docs), [Research](https://research.opena2a.org).
- [Roadmap](ROADMAP.md)
- [Contributing](CONTRIBUTING.md): the dev loop and pull requests. Report security issues to `info@opena2a.org`; [SECURITY.md](SECURITY.md) says what to expect.

## License

Apache-2.0. See [LICENSE](LICENSE).
