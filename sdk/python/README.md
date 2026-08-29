# AIM Python SDK

Cryptographic identity, capability authorization, and audit trails for Python AI agents. Apache 2.0.

[![PyPI version](https://img.shields.io/pypi/v/aim-sdk.svg)](https://pypi.org/project/aim-sdk/)
[![Python](https://img.shields.io/pypi/pyversions/aim-sdk.svg)](https://pypi.org/project/aim-sdk/)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../../LICENSE)

Part of [Agent Identity Management (AIM)](../../README.md). Managed hosting at [aim.opena2a.org/get-started](https://aim.opena2a.org/get-started); self-host via the [main README](../../README.md#install-aim-self-hosted).

## Upgrading to 2.0.0: a denied action now stops

In every published version before 2.0.0, an action AIM **denied** executed anyway. The denial was raised, fell through a `except Exception` handler, and the wrapped function ran. From 2.0.0 it raises `ActionDeniedError` and the function does not run — **in every enforcement mode, including monitoring, which is the default and which every existing organization was backfilled to.**

Monitoring mode governs what happens when AIM cannot give an answer, not what happens when AIM says no.

Before upgrading a running agent, check whether anything it does is currently being denied and executing regardless (dashboard: Agents, and Settings → Security → Policies). Those calls will start raising. Full detail, including two smaller behaviour changes, is in the 2.0.0 entry of [CHANGELOG.md](https://github.com/opena2a-org/agent-identity-management/blob/main/sdk/python/CHANGELOG.md).

## Quick start

```python
from aim_sdk import secure

agent = secure("my-first-agent")

@agent.perform_action(capability="db:read")
def get_customer(customer_id):
    return db.query("SELECT * FROM customers WHERE id = ?", customer_id)
```

`secure()` generates an Ed25519 keypair, registers the agent with the AIM backend, and stores credentials at `~/.aim/`. `@perform_action` signs every invocation, runs it through 5-step Fine-Grained Authorization on the server, and records the outcome in the audit log.

Install:

```bash
pip install aim-sdk
aim-sdk login                              # OAuth to aim.opena2a.org
aim-sdk login --url http://localhost:8080  # or to your self-hosted AIM
```

Login uses OAuth 2.0 with PKCE. Credentials save to `~/.aim/sdk_credentials.json` (mode 0600).

## Framework auto-detection

`secure()` reads `sys.modules` to detect the agent's framework or LLM provider from imports:

| Category | Detected from | Mapped agent_type |
|---|---|---|
| Frameworks | `langchain`, `crewai`, `autogen`, `llama_index`, `haystack`, `semantic_kernel`, `langgraph` | `langchain`, `crewai`, etc. |
| LLM providers | `anthropic`, `openai`, `google.generativeai`, `mistralai`, `cohere` | `claude`, `gpt`, `gemini`, `mistral`, `cohere` |

Frameworks take priority over LLM providers. If both `langchain` and `anthropic` are imported, the agent type is `langchain`.

Override explicitly:

```python
from aim_sdk import secure, AgentType

agent = secure("my-agent", agent_type=AgentType.CREWAI)
```

## Auto-instrumentation

After `secure()` registers the agent, the SDK installs no-op-on-failure hooks for any of these libraries that are present:

- LangChain
- CrewAI
- OpenAI
- Anthropic

Each model call (chat completion, embedding, tool call) is recorded to the audit trail. The hooks never raise — a hook failure logs a warning and the call proceeds.

Disable: `secure("my-agent", auto_hooks=False)`.

## Capability decorators

`@agent.perform_action` signs each invocation, runs it through FGA on the server, and records the outcome. Risk level auto-detects from the capability string using two lookup tables in [`aim_sdk/risk_detector.py`](aim_sdk/risk_detector.py):

- **Namespace prefix** maps `payment:`, `admin:`, `system:`, `billing:`, `finance:` to critical; `email:`, `notification:`, `sms:`, `user:`, `auth:`, `secret:`, `credential:` to high; `db:`, `database:`, `file:`, `storage:`, `cache:` to medium; `api:`, `weather:`, `search:`, `geocode:`, `translate:`, `time:`, `math:`, `util:` to low.
- **Action suffix** maps `:read`, `:fetch`, `:get`, `:list`, `:query`, `:view`, `:check`, `:validate` to low; `:write`, `:update`, `:create`, `:modify`, `:save`, `:upload` to medium; `:delete`, `:send`, `:execute`, `:run`, `:invoke`, `:export`, `:transfer` to high; `:process`, `:refund`, `:charge`, `:approve`, `:drop`, `:truncate`, `:wipe`, `:terminate` to critical.

When namespace and action disagree the higher risk wins. `SPECIFIC_CAPABILITY_MAP` overrides both for known patterns (for example `user:delete` escalates to critical).

```python
@agent.perform_action(capability="db:read")               # medium (db: medium, :read low → max = medium)
def get_customer(customer_id): ...

@agent.perform_action(capability="db:delete")             # high (SPECIFIC_CAPABILITY_MAP override)
def delete_customer(customer_id): ...

@agent.perform_action(capability="payment:refund",
                      risk_level="critical",
                      jit_access=True,
                      timeout_seconds=300)
def process_refund(order_id, amount): ...                 # waits for admin approval
```

### JIT access

`jit_access=True` pauses execution and creates an approval request in the AIM dashboard. The function returns only after a human approves. There is no dedicated timeout exception — a request that times out, cannot be verified, or is denied raises through the same enforcement rule as every other verification path: `VerificationUnavailableError` if AIM could not be reached or answer within `timeout_seconds`, `ActionDeniedError` if explicitly denied. Both subclass `AIMError`.

### Enforcement mode

The organization's enforcement mode (configured in dashboard Settings → Security → Policies) controls what happens when a verification **could not be completed**.

> Monitoring mode governs what happens when AIM cannot give an answer, not what happens when AIM says no. A verification that could not be completed is logged and the action proceeds; an explicit denial blocks in every mode, because it is a decision AIM already made with your organization's enforcement mode in hand.

**An explicit denial raises `ActionDeniedError` and blocks the function, in every mode.** `ActionDeniedError` subclasses `PermissionError`, so `except PermissionError` catches it. AIM applies your enforcement mode server-side before it answers — under monitoring a policy refusal is converted to an approval and never reaches your process as a denial — so a denial that does reach the SDK is one the server declined to override while holding your organization's row.

Where the mode does apply is the third case: AIM answered with no decision, or could not be reached at all. That raises `VerificationUnavailableError`, which deliberately does **not** subclass `VerificationError` or `PermissionError` — a handler written for "AIM said no" must not silently absorb "AIM was never asked".

- **Monitoring** (default) — an unanswered verification logs a warning and the function executes anyway. For dev and gradual rollout.
- **Strict** — an unanswered verification blocks. For production and compliance.

If the mode cannot be resolved either, a server that answered still blocks, while an unreachable AIM currently runs and warns — that last case becomes a block in 3.0.0.

`AIM_STRICT_MODE` is a **ratchet**: it can raise enforcement to strict but can never lower it. `AIM_STRICT_MODE=true|1|yes|on` forces strict mode locally, whatever the dashboard says; a false value is ignored with a warning. Use the dashboard to configure production, and the variable only to make a specific process stricter than its organization.

> Before 2.0.0 this section was inverted, and so was the behaviour: the dashboard setting never reached the Python SDK, and the environment variable was the only lever that produced any enforcement at all. If you are upgrading from an earlier version, read [the 2.0.0 entry in CHANGELOG.md](https://github.com/opena2a-org/agent-identity-management/blob/main/sdk/python/CHANGELOG.md) before deploying.

Everything the SDK enforces happens inside the agent's own process, so it is advisory with respect to that agent. It blocks denied actions for an honest operator and for a compromised agent that still routes through the SDK. It is not a control against a hostile operator.

## Capability declaration

The SDK supports three ways to declare capabilities. Decorators are preferred.

1. **Decorators** — `@agent.perform_action(capability="...")` in code. Most accurate.
2. **Config file** — `~/.aim/capabilities.json` with `{"capabilities": ["db:read", ...]}`. For static declarations.
3. **Explicit at registration** — `secure("my-agent", capabilities=["api:call", "db:read"])`.

Auto-detection from `sys.modules` is disabled by default — it's noisy and misleading (almost every Python agent imports the same generic packages).

### Request additional capabilities

After registration, new capabilities require admin approval (prevents privilege escalation per CVE-2025-32711):

```python
result = agent.request_capability(
    capability_type="db:write",
    reason="Need to update user preferences"
)

if result["status"] == "pending":
    print(f"Request {result['id']} submitted - awaiting admin approval")
elif result["status"] == "approved":
    print("Capability granted")
```

## MCP server registration

`secure()` auto-discovers MCP servers via Claude Desktop config and queries each server using the MCP protocol (`tools/list`):

```python
agent = secure("my-agent", mcp_servers=["filesystem", "github"])
# SDK queries each server, discovers actual capabilities, auto-attests.
```

Manual registration:

```python
agent.register_mcp(
    server_name="my-database-server",
    server_url="http://localhost:3001",
    capabilities=["db:read", "db:write", "data:delete"]
)
```

Discover capabilities without attesting:

```python
from aim_sdk.detection import discover_mcp_capabilities

caps = discover_mcp_capabilities(["filesystem", "github"])
# {"filesystem": ["read_file", "write_file", ...], "github": [...]}
```

## Credential storage

| Path | Contents | Mode |
|---|---|---|
| `~/.aim/sdk_credentials.json` | OAuth tokens (from `aim-sdk login`) | 0600 |
| `~/.aim/agents/<name>.json` | Per-agent Ed25519 keypair + metadata | 0600 |
| `~/.aim/credentials.json` | Legacy combined-store; auto-migrated | 0600 |
| `~/.aim/capabilities.json` | Explicit capability declarations (optional) | 0644 |

The private key is returned **once** at registration. The SDK saves it locally. Losing the private key means rotating credentials via the dashboard.

To force a fresh registration that bypasses the local cache and reconnects to the backend:

```python
agent = secure("my-agent", force_new=True)
```

`force_new=True` is for credential rotation, debugging, or post-database-reset recovery. To create an entirely new agent, use a different name.

## CLI commands

The SDK ships with a small CLI for authentication and status:

```bash
aim-sdk login                    # OAuth to AIM Cloud
aim-sdk login --url <URL>        # OAuth to self-hosted instance
aim-sdk demo                     # Register a demo agent, watch your dashboard come alive
aim-sdk demo --interactive       # Full menu: security demos, JIT approval, MCP
aim-sdk demo --cleanup           # Delete the demo agent again
aim-sdk logout                   # Clear ~/.aim/sdk_credentials.json
aim-sdk status                   # Show authentication state
aim-sdk --version                # Show SDK version
```

The demo agent registers in your own organization under agent type `demo`:
visibly a demo in every list, excluded from adoption and trust analytics,
and re-runs reconnect to the same agent instead of creating new ones.

For SecOps workflows (scanning a codebase, hardening configs, monitoring runtime), see the separate [opena2a CLI](https://github.com/opena2a-org/opena2a).

## Causal-denial telemetry (opt-in)

When a verification is denied, the SDK can join the injection cause, the
classified intent, and the authorization outcome into one local correlated
record so you can see **why** an action was blocked. It is OFF by default and
best-effort -- it never changes a verdict and never adds latency to the
enforcement path. Two independent opt-ins gate it:

```python
from aim_sdk import AIMClient
from aim_sdk.telemetry import IntentInput, DetectionInput

client = AIMClient(
    agent_id="...", api_key="aim_abc123", aim_url="https://aim.opena2a.org",
    telemetry={
        "enabled": True,                       # stage 1: capture records locally
        "relay": {                             # stage 2: share anonymized indicators
            "enabled": True,                   #   (separate, explicit opt-in)
            "package_name": "my-service",      #   self-declared sensor label
        },
    },
)

# The injection detector / intent classifier populate the optional seam:
client.verify_capability(
    "net:connect", resource="https://example/data",
    telemetry={
        "intent": IntentInput(intent_class="exfiltration", confidence=0.7,
                              blocked=True, source="nanomind-intent"),
        "detection": DetectionInput(injection_detected=True, confidence=0.84,
                                    detector="nanomind-guard",
                                    technique_source="interim-mapping",
                                    technique_id="T-2002"),
    },
)

client.close()  # stops the managed joiner/relay threads (or use a `with` block)
```

**Two tiers, by design.** The full correlated record is authoritative and stays
on the machine (`~/.opena2a/correlated-events.jsonl`). Only an anonymized
**indicator** -- event type, Threat-Matrix technique ID, confidence, runtime,
and an anonymous per-device sensor token -- may leave, and only when relay
sharing is opted in. Identifiers (agent ID, resource, capability, credential
references, payloads, the correlation key) are never shared, and the relay
egress-validates technique fields before transmission. Only
`denied_injection_attempt` indicators are uploaded, to the Registry's public,
count-only endpoint.

**Producer only, never on the deny path.** Detection output is an inference the
seam records; it never enters `verify_action` or any policy-decision deny path,
and the modules that decide allow/deny (`aim_sdk.enforcement`,
`aim_sdk.decision`) do not import the seam.

**Port stage.** `aim_sdk.telemetry` is stage 1 of 3 of the ARP runtime-protection
port (roadmap `arp-python-sdk-parity`), ported schema-identical to the TypeScript
reference in `sdk/typescript/src/telemetry` so both SDKs write the same
`correlated-events.jsonl`. Stage 2 (the guard-socket client) and stage 3 (the
engine port: event engine, runtime twin, coordinator, monitors, interceptors)
are not part of this package -- see the module docstring for the boundary.

## Manual mode (no OAuth)

For CI environments or pre-configured credentials, skip `aim-sdk login` and pass an API key:

```python
agent = secure("my-agent", api_key="aim_abc123")
```

Or supply full credentials:

```python
from aim_sdk import AIMClient

client = AIMClient(
    agent_id="550e8400-e29b-41d4-a716-446655440000",
    public_key="<base64-Ed25519>",
    private_key="<base64-Ed25519>",
    aim_url="https://aim.opena2a.org"
)

@client.perform_action(capability="db:read")
def get_customer(customer_id): ...
```

## Examples

Working examples in [`examples/`](examples/):

| Example | Shows |
|---|---|
| [`example.py`](examples/example.py) | Decorator-based verification, manual mode |
| [`example_auto_detection.py`](examples/example_auto_detection.py) | Framework + MCP auto-discovery (no backend required) |
| [`example_one_line_setup.py`](examples/example_one_line_setup.py) | Zero-config `secure()` flow (requires backend) |

Framework integration guides:
- [LangChain](docs/LANGCHAIN_INTEGRATION.md)
- [CrewAI](docs/CREWAI_INTEGRATION.md)
- [MCP servers](docs/MCP_INTEGRATION.md)

## Requirements

- Python 3.8+
- `requests` (HTTP)
- `pynacl` (Ed25519)
- `cryptography` (TLS, secure storage)
- `keyring` (OS keychain for OAuth tokens)

All install via `pip install aim-sdk`.

## Versioning

[Semantic Versioning 2.0.0](https://semver.org/). Current: see `VERSION` file. The SDK and the backend platform are versioned and released independently — see [docs/VERSIONING.md](docs/VERSIONING.md#backend-api-compatibility).

```python
import aim_sdk
print(aim_sdk.__version__)
```

See [CHANGELOG.md](https://github.com/opena2a-org/agent-identity-management/blob/main/sdk/python/CHANGELOG.md) for history, [docs/VERSIONING.md](docs/VERSIONING.md) for the support policy.

## Related

- [Java SDK](../java/README.md) — same API shape, AspectJ-based decoration
- [TypeScript SDK](../typescript/README.md) — local-or-server mode
- [opena2a CLI](https://github.com/opena2a-org/opena2a) — codebase auditing, credential migration, runtime monitoring
- [AIM backend](../../README.md) — server, dashboard, deployment
- [aicomply](https://github.com/opena2a-org/aicomply) — content-compliance companion (`pip install aicomply`); `@guard_io`/`@guard_output` scan the PII, credentials, and regulated data an agent reads and emits, complementing AIM's `@agent.perform_action` capability checks (AIM authorizes the action; aicomply inspects the content)

## License

Apache-2.0. See [LICENSE](../../LICENSE).
