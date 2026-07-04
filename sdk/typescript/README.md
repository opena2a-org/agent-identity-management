# AIM SDK for TypeScript/Node.js

Official TypeScript SDK for Agent Identity Management (AIM) - secure identity verification for AI agents.

Managed hosting available at [aim.opena2a.org/get-started](https://aim.opena2a.org/get-started). Self-host instructions below.

## Installation

```bash
npm install @opena2a/aim-sdk
# or
yarn add @opena2a/aim-sdk
# or
pnpm add @opena2a/aim-sdk
```

## Quick Start

```typescript
import { AIMClient, AgentType } from '@opena2a/aim-sdk';

// Create client
const client = new AIMClient({
  baseUrl: 'https://aim.example.com',
  apiKey: process.env.AIM_API_KEY,
});

// Register an agent
const agent = await client.registerAgent({
  name: 'my-ai-agent',
  displayName: 'My AI Agent',
  agentType: AgentType.LANGCHAIN,
  capabilities: ['file:read', 'api:call'],
});

// Verify an action before execution
const result = await client.verifyAction({
  action: 'file:read',
  resource: '/data/config.json',
});

console.log(`Action allowed: ${result.actionAllowed}`);
console.log(`Trust score: ${result.trustScore}`);
```

## Features

- **Full TypeScript Support**: Complete type definitions for all APIs
- **Ed25519 Signatures**: Cryptographic signing for secure verification
- **OAuth 2.0**: Automatic token management with client credentials flow
- **Express Middleware**: Easy integration with Express.js applications
- **Fastify Plugin**: First-class support for Fastify applications
- **Automatic Retries**: Built-in retry logic with exponential backoff
- **Local Credential Verification**: Verify signed ATX credentials offline against cached trust anchors, no per-action call to a central service

## Express Integration

```typescript
import express from 'express';
import { createAIMMiddleware, verifyAction, aimErrorHandler } from '@opena2a/aim-sdk/express';

const app = express();

// Add AIM middleware globally
app.use(createAIMMiddleware({
  baseUrl: 'https://aim.example.com',
  apiKey: process.env.AIM_API_KEY,
}));

// Verify specific actions on routes
app.post('/api/data',
  verifyAction('data:write'),
  (req, res) => {
    // Action has been verified
    res.json({ success: true });
  }
);

// Access AIM context in handlers
app.get('/api/profile', (req, res) => {
  const { agentId, trustScore } = req.aim ?? {};
  res.json({ agentId, trustScore });
});

// Optional: map SDK errors thrown in later handlers to HTTP responses
// (ActionDeniedError -> 403, AuthenticationError -> 401). aimErrorHandler IS
// the four-argument handler — pass it to app.use, do not call it.
app.use(aimErrorHandler);
```

## Fastify Integration

```typescript
import Fastify from 'fastify';
import { aimPlugin, verifyAction } from '@opena2a/aim-sdk/fastify';

const fastify = Fastify();

// Register the AIM plugin
await fastify.register(aimPlugin, {
  baseUrl: 'https://aim.example.com',
  apiKey: process.env.AIM_API_KEY,
});

// Verify actions with preHandler hook
fastify.post('/api/data', {
  preHandler: verifyAction('data:write'),
}, async (request, reply) => {
  return { success: true };
});
```

## Local Credential Verification (offline)

An ATX (Agent Trust eXtension) credential is a signed, portable credential
designed to be verified *locally*: the signature is checked against the issuer's
cached public key in roughly a millisecond, the issuing node is never on the
verification path, and revocation rides on an asynchronously-refreshed,
short-lived cached list. When you configure `localVerification` with cached trust
anchors, the client verifies a resolved credential offline and decides
authorization from the credential's own signed claims — no per-action call to a
central service. The remote `verifyAction` POST is retained as the fallback for
when no local credential is available.

The verifier is the shared, conformance-locked `@opena2a/atx-verify` — byte-for-byte
interoperable with the Go (`opena2a-registry/pkg/atcverify`) and Python reference
verifiers.

```typescript
import { AIMClient } from '@opena2a/aim-sdk';

const client = new AIMClient({
  // Cached once from AIM/the Registry; refresh the CRL off the hot path.
  localVerification: {
    trustedIssuers: ['did:opena2a:issuer-1'],
    publicKeys: [{ algorithm: 'Ed25519', publicKeyHex: '<issuer raw ed25519 pubkey hex>' }],
    // crl: { entries: [{ agentId, reason }] }, // optional cached revocation list
  },
});

// Resolve the agent's ATX once (the AAP broker / network step), then cache it.
client.setLocalCredential(resolvedAtx);

// Per action: verified offline, sub-millisecond, no network.
const result = await client.verifyActionLocally({ action: 'file:read' });
// or just call verifyAction(): it takes the local path automatically when a
// credential is cached, and falls back to the remote POST otherwise.
```

Authorization is gated on *signed* capabilities. ATX v1.1 credentials carry
capabilities under the signature, so they are trusted; v1.0 capabilities are
forgeable by the holder and are refused by default. Passing
`requireSignedCapabilities: false` to `LocalVerifier.authorize` overrides this,
but then authorization runs on holder-forgeable capabilities — only do this for a
closed, trusted v1.0 deployment, never across a trust boundary.

> **Multi-issuer anchor sets.** Give each key a DID-URL `keyId` (e.g.
> `did:opena2a:authority:opena2a.org#key-1`). `@opena2a/atx-verify` binds a key
> to its controller DID, so a key may only verify credentials issued by that DID
> (or, for v1.1, a signed `issuerChain` authority) — one trusted issuer cannot
> impersonate another. A key with no `keyId` fragment is unbound and eligible for
> any issuer: fine for a single-issuer anchor set, unsafe for a multi-issuer one.

For credential verification without the action-authorization adaptation, use the
verifier directly:

```typescript
const verifier = client.getLocalVerifier();
const { valid, context, rejectCategory } = await verifier!.verifyCredential(atx);
```

> Network is reserved for credential *resolution* (the AAP broker hands the agent
> its ATX) and the periodic CRL refresh — never for a per-action decision.

## Causal-Denial Telemetry (opt-in)

The SDK can correlate *why* a blocked agent action happened by joining three
signals around one verified action: the authorization outcome (an observed
fact), the classified intent, and the injection cause (both inferences). The
full **correlated record** is authoritative and stays on the machine; only an
anonymized **shared indicator** is ever uploaded. This section covers the
causal-denial channel only — the runtime-protection module ships a separate
structural-signature channel that is **on by default**; see the Runtime
Protection section for its scope and opt-out.

The causal-denial channel is **off by default** and gated by two independent
opt-ins:

1. **Capture** (`telemetry.enabled`) — mints a correlation ID per `verifyAction`,
   assembles records, and appends them to a local log at
   `~/.opena2a/correlated-events.jsonl`. Nothing leaves the machine.
2. **Share** (`telemetry.relay.enabled`) — a best-effort relay reduces local
   records to anonymized indicators and uploads only `denied_injection_attempt`
   events to the public, count-only Registry endpoint
   (`POST /api/v1/telemetry/runtime`).

   The shared indicator carries **no** payloads, paths, credentials,
   resource/capability names, correlation ID, agent ID, or denial reason text.
   It carries only: a validated Threat Matrix `techniqueId` (`T-NNNN`, dropped
   if malformed) and its source, a detection confidence, the coarse enforcement
   outcome (`deny`/`allow`), the integrator's self-declared `packageName`/
   `agentCategory`, `daySinceInstall`, `runtimeEnv`, `triggeredAt`, and a
   `sensorToken` — a stable per-device pseudonym (`sha256(host+user+local salt)`)
   that lets the Registry de-duplicate without identifying you.

```typescript
const client = new AIMClient({
  baseUrl: 'https://aim.example.com',
  apiKey: process.env.AIM_API_KEY,
  telemetry: {
    enabled: true,                 // stage 1: capture records locally
    relay: {
      enabled: true,               // stage 2: upload anonymized indicators
      packageName: 'acme/support-agent', // your app's public sensor label
      packageVersion: '2.1.0',
      // registryUrl defaults to https://api.oa2a.org
      // intervalMs defaults to 60000, batchSize to 50
    },
  },
});

// Per-action inputs (populated by the runtime-protection module, or supplied
// directly). Ignored unless telemetry is enabled.
await client.verifyAction({
  action: 'file:read',
  resource: '/data/sensitive.json',
  telemetry: {
    intent: { intentClass: 'exfiltration', confidence: 0.7, blocked: true, source: 'nanomind-intent' },
    detection: {
      injectionDetected: true,
      techniqueId: 'T-2002',
      techniqueSource: 'interim-mapping',
      confidence: 0.84,
      detector: 'nanomind-guard',
      detectedAt: new Date().toISOString(),
    },
  },
});

// Stop the internally managed flush timers when shutting down.
client.closeTelemetry();
```

**Guarantees.** Telemetry runs off the enforcement path and is best-effort: a
capture, join, or upload failure is swallowed and never changes an action's
verification result. When both opt-ins are off, no correlation ID is minted, no
header is attached, and nothing is written or sent.

`TelemetryConfig` fields: `enabled` (capture switch), `enforcementSource`
(stamped on the enforcement fact, default `aim-pdp`), `joiner` (supply your own
to control the sink/lifecycle), and `relay` (`RelayConfig`: `enabled`,
`registryUrl`, `packageName`, `packageVersion`, `agentCategory`, `dataDir`,
`batchSize`, `intervalMs`, `timeoutMs`). The `CorrelatedRelay` class is also
exported for standalone use (e.g. draining the local log from a CLI).

## Runtime Protection (`@opena2a/aim-sdk/arp`)

The SDK ships the ARP (Agent Runtime Protection) engine as a subpath module.
It observes an agent process from the inside — monitors and interceptors for
process, network, filesystem, prompt, MCP, and A2A activity; a rule-based
event engine (L0); a behavioral anomaly twin (L1); and an intelligence
coordinator (L2) — and produces the detection inputs shown in the telemetry
section above.

The boundary with the `hackmyagent` scanner: scan-time analysis (static
scanning, hardening rules, artifact classification) lives in hackmyagent;
runtime protection lives here, inside the agent process. This module is the
canonical home of the ARP engine; hackmyagent's `./arp` export is being
converted to a thin re-export of it.

```typescript
import { EventEngine, FilesystemMonitor, EnforcementEngine } from '@opena2a/aim-sdk/arp';

const engine = new EventEngine(config);
const monitor = new FilesystemMonitor(engine);
await monitor.start();
```

**Interception scope.** The filesystem and process interceptors patch the CJS
module registry (`require('fs')`, `require('child_process')`). Code that loads
those builtins via `require` or an ESM namespace/default import
(`import fs from 'fs'`) is observed. Code that captured **named** ESM bindings
before the interceptor started (`import { readFileSync } from 'fs'`) bypasses
them — ESM bindings are resolved at link time and cannot be patched. The
monitors (which poll rather than intercept) are unaffected. Start interceptors
as early as possible in the process, and treat them as one observation layer,
not a sandbox.

Classification of runtime events is supplied through the injectable
`ClassificationProvider` seam. The default `NanoMindGuardClassificationProvider`
talks to the local NanoMind-Guard daemon over a Unix socket and only accepts
signed classification results; when the daemon is absent the annotator degrades
to "no classification" — it never fabricates a label and never blocks.

**Invariant:** the runtime-protection module is a telemetry producer. Its
detection outputs flow through the `telemetry.detection` seam into the
correlated record; they never enter `verifyAction`'s allow/deny decision.

**Structural signature telemetry (on by default, opt-out).** Unlike the
opt-in causal-denial channel above, the ARP engine shares **structural attack
signatures** by default: when a detection fires, the structural shape of the
event sequence — technique identifier, event-type sequence, severity, and a
one-way hash of structural tokens — is signed and reported to the OpenA2A
registry (`https://api.oa2a.org`), so an attack shape first seen at one
deployment can protect others. Prompts, model responses, tool arguments, file
contents or paths, command lines, environment values, secrets, IP addresses,
hostnames, and account, tool, agent, and model names are never shared. A
one-time disclosure is printed before first collection, and every payload is
appended to a local audit log (`~/.opena2a/telemetry-audit.log`, JSONL) before
it is sent. Opt out with any one of:

- `OPENA2A_TELEMETRY_OPTOUT=1` (or `ARP_TELEMETRY_DISABLED=1`) in the
  environment,
- `signatureTelemetry: { enabled: false }` in your ARP config, or
- `writeOptOutMarker()` from `@opena2a/aim-sdk/arp`, which persists
  `~/.opena2a/telemetry-optout` across processes.

Any one of these is the runtime-protection module's master switch: it disables
every telemetry channel the module can produce (structural signatures and the
opt-in legacy GTIN runtime channel). The causal-denial channel above is
controlled solely by its own `telemetry` client config and is off unless you
enabled it. To also delete signatures this sensor already shared, call
`purgeRemoteSignatures()` (right-to-delete; best-effort, never blocks the
local opt-out).

## Configuration

### Environment Variables

The SDK automatically reads from these environment variables:

| Variable | Description |
|----------|-------------|
| `AIM_BASE_URL` | AIM server base URL |
| `AIM_API_KEY` | API key for authentication |
| `AIM_ORGANIZATION_ID` | Organization ID |
| `AIM_AGENT_ID` | Pre-registered agent ID |
| `AIM_PRIVATE_KEY` | Ed25519 private key (base64) |
| `AIM_PUBLIC_KEY` | Ed25519 public key (base64) |
| `AIM_DEBUG` | Enable debug logging (`true`/`false`) |

### Client Options

```typescript
const client = new AIMClient({
  baseUrl: 'https://aim.example.com',  // AIM server URL
  apiKey: 'your-api-key',              // API key
  organizationId: 'org-uuid',          // Organization ID
  autoRegister: true,                  // Auto-register if not registered
  timeout: 30000,                      // Request timeout in ms
  debug: false,                        // Debug logging
  headers: {},                         // Custom headers
});
```

## Agent Types

The SDK supports various agent types:

```typescript
import { AgentType } from '@opena2a/aim-sdk';

// LLM Providers
AgentType.CLAUDE
AgentType.GPT
AgentType.GEMINI

// Frameworks
AgentType.LANGCHAIN
AgentType.CREWAI
AgentType.AUTOGEN

// Assistants
AgentType.COPILOT
AgentType.ASSISTANT
```

## Error Handling

```typescript
import {
  ActionDeniedError,
  AuthenticationError,
  RateLimitError,
} from '@opena2a/aim-sdk';

try {
  await client.verifyAction({ action: 'file:delete' });
} catch (error) {
  if (error instanceof ActionDeniedError) {
    console.log(`Denied: ${error.reason}`);
    console.log(`Trust score: ${error.trustScore}`);
  } else if (error instanceof RateLimitError) {
    console.log(`Retry after: ${error.retryAfter} seconds`);
  }
}
```

> **Class identity is per entry point.** Each entry point (`.`, `/arp`,
> `/express`, `/fastify`) bundles its own copy of the error classes, so
> `instanceof` only matches errors raised by the same entry point you imported
> from. When you use an integration, import the error classes from that same
> integration (`@opena2a/aim-sdk/express` and `/fastify` re-export the full
> error family); for checks that must work across entry points, match on
> `error.code` (e.g. `'ACTION_DENIED'`) instead.

## Credential Management

```typescript
import {
  loadCredentialsFromFile,
  saveCredentialsToFile,
} from '@opena2a/aim-sdk';

// Save credentials after registration
const credentials = client.getCredentials();
await saveCredentialsToFile(credentials, '.aim/credentials.json');

// Load credentials on startup
const saved = await loadCredentialsFromFile('.aim/credentials.json');
client.setCredentials(saved);
```

## API Reference

### AIMClient

- `registerAgent(options)` - Register a new agent
- `verifyAction(options, atx?)` - Verify an action (local path when a credential is cached, remote POST fallback otherwise)
- `verifyActionLocally(options, atx?)` - Verify an action against a locally-held ATX credential, fully offline
- `setLocalCredential(atx)` - Cache the resolved ATX credential for offline verification (pass `null` to clear)
- `getLocalVerifier()` - The configured `LocalVerifier`, or `null` when local verification is not enabled
- `getAgent()` - Get current agent info
- `updateAgent(updates)` - Update agent metadata
- `reportCapabilities(capabilities)` - Report agent capabilities
- `getTrustScore()` - Get current trust score
- `getCredentials()` - Get stored credentials
- `setCredentials(credentials)` - Set credentials

### Types

See
[src/types/index.ts](https://github.com/opena2a-org/agent-identity-management/blob/main/sdk/typescript/src/types/index.ts)
for complete type definitions (source is not shipped in the npm package; the
bundled `.d.ts` files carry the same types).

## License

Apache-2.0

## Contributing

See
[CONTRIBUTING.md](https://github.com/opena2a-org/agent-identity-management/blob/main/CONTRIBUTING.md)
for guidelines.
