# AIM Python SDK

Cryptographic identity, capability authorization, and audit trails for Python AI agents. Apache 2.0.

[![PyPI version](https://img.shields.io/pypi/v/aim-sdk.svg)](https://pypi.org/project/aim-sdk/)
[![Python](https://img.shields.io/pypi/pyversions/aim-sdk.svg)](https://pypi.org/project/aim-sdk/)
[![License: Apache-2.0](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](../../LICENSE)

Part of [Agent Identity Management (AIM)](../../README.md). Managed hosting at [aim.opena2a.org/get-started](https://aim.opena2a.org/get-started); self-host via the [main README](../../README.md#install-aim-self-hosted).

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

`jit_access=True` pauses execution and creates an approval request in the AIM dashboard. The function returns only after a human approves, or raises `JITAccessTimeoutError` after `timeout_seconds`.

### Enforcement mode

The organization's enforcement mode (configured in dashboard Settings → Security → Policies) controls what happens on verification failure:

- **Monitoring** (default) — warning logged, function executes anyway. For dev and gradual rollout.
- **Strict** — `PermissionError` raised, function blocked. For production and compliance.

Override locally for testing: `AIM_STRICT_MODE=true python my_agent.py`. Production deployments configure in the dashboard, not via env var.

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
aim-sdk logout                   # Clear ~/.aim/sdk_credentials.json
aim-sdk status                   # Show authentication state
aim-sdk --version                # Show SDK version
```

For SecOps workflows (scanning a codebase, hardening configs, monitoring runtime), see the separate [opena2a CLI](https://github.com/opena2a-org/opena2a).

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

[Semantic Versioning 2.0.0](https://semver.org/). Current: see `VERSION` file. SDK 1.x.x is compatible with backend 1.x.x; SDK 2.x.x requires backend 2.x.x.

```python
import aim_sdk
print(aim_sdk.__version__)
```

See [CHANGELOG.md](CHANGELOG.md) for history, [docs/VERSIONING.md](docs/VERSIONING.md) for the support policy.

## Related

- [Java SDK](../java/README.md) — same API shape, AspectJ-based decoration
- [TypeScript SDK](../typescript/README.md) — local-or-server mode
- [opena2a CLI](https://github.com/opena2a-org/opena2a) — codebase auditing, credential migration, runtime monitoring
- [AIM backend](../../README.md) — server, dashboard, deployment

## License

Apache-2.0. See [LICENSE](../../LICENSE).
