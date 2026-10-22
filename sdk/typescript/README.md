# [AIM SDK for TypeScript/Node.js](https://opena2a.org/agent-identity-management)

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

Requires Node.js 20.19 or a later 20.x release, or Node.js 22.12 or later.

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

### Register a first agent from the terminal

`init` is not in 1.3.1, the published version: the 1.3.1 package answers `init` with `Unknown command: init`. While 1.3.1 is the published version, run `init` from a source build of this repository, which reports its version as `1.3.1+source`.

`init` needs an AIM server that has onboarding tokens, which are in builds of this repository's main branch from 2026-10-08 on and in no release yet. A released server answers the token exchange with HTTP 404, which `aim-arp init` reports as `has no onboarding endpoint (HTTP 404)` and the Python SDK's `aim-sdk init` as `HTTP 404 from <url>/api/v1/onboarding/bootstrap-tokens/exchange`.

The dashboard has no screen that mints a bootstrap token yet: a signed-in member, manager or admin mints one with `POST /api/v1/onboarding/bootstrap-tokens` ([API reference](https://github.com/opena2a-org/agent-identity-management/blob/main/docs/API.md#post-apiv1onboardingbootstrap-tokens)). The token lasts 15 minutes and registers one agent. `init` exchanges it for an agent and saves the agent's credentials:

```bash
git clone https://github.com/opena2a-org/agent-identity-management.git
cd agent-identity-management/sdk/typescript
npm ci && npm run build
AIM_BOOTSTRAP_TOKEN=<token> node dist/arp/cli/aim-arp.js init --url https://aim.example.com
```

```
Registering agent "my-first-agent" with https://aim.example.com ...

Agent "my-first-agent" is registered.
  Agent ID:     550e8400-e29b-41d4-a716-446655440000
  Status:       pending
  Credentials:  /home/you/.aim/agents/my-first-agent.json (owner read/write only)
  Dashboard:    https://aim.example.com/dashboard/agents/550e8400-e29b-41d4-a716-446655440000

The private key was generated on this machine and has not been sent anywhere.
```

The Ed25519 key pair is generated locally and only the public key is sent. The credentials file uses the layout the Python SDK reads, so either SDK can load the agent. `--name <name>` names the agent (default `my-first-agent`); `--url` defaults to `AIM_URL`, then `https://aim.opena2a.org`. The token is read from `AIM_BOOTSTRAP_TOKEN` only; a flag that carries it is refused, because a flag value is visible in the process list and shell history. A name that holds `/` or `\`, or is `.` or `..`, is refused before the token is sent. Run `node dist/arp/cli/aim-arp.js init --help` from the same directory for every option.

Once a release after 1.3.1 is published, the same command runs from npm without the clone and the build, and prints the same output:

```bash
AIM_BOOTSTRAP_TOKEN=<token> npx @opena2a/aim-sdk init --url https://aim.example.com
```

## Features

- **Full TypeScript Support**: Complete type definitions for all APIs
- **Ed25519 Signatures**: Cryptographic signing for secure verification
- **OAuth 2.0**: Automatic token management with the JWT-bearer grant (RFC 7523): the SDK signs a short-lived assertion with the agent's Ed25519 key, and the server verifies it against the agent's registered public key
- **Express Middleware**: Easy integration with Express.js applications
- **Fastify Plugin**: First-class support for Fastify applications
- **Typed HTTP Errors**: Failures surface as a typed error family (`RateLimitError` carries the server's `Retry-After`); the SDK does not retry automatically: retry policy stays with the caller
- **Local Credential Verification**: Verify signed ATX credentials offline against cached trust anchors, no per-action call to a central service
- **Delegation Chains**: Create and verify Ed25519 delegation chains (cross-engine interop), with signature, identity, scope-narrowing, and expiry all enforced at verification time

## Express Integration

```typescript
import express from 'express';
import { loadCredentialsFromFile } from '@opena2a/aim-sdk';
import { createAIMMiddleware, verifyAction, aimErrorHandler } from '@opena2a/aim-sdk/express';

const app = express();

// Add AIM middleware globally. `credentials` is the registered agent the
// middleware verifies as; omit it to read the four AIM_* variables below.
app.use(createAIMMiddleware({
  baseUrl: 'https://aim.example.com',
  credentials: await loadCredentialsFromFile('./agent-credentials.json'),
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

// Optional: answer SDK errors as JSON (ActionDeniedError -> 403,
// AuthenticationError -> 401, any other SDK error -> its status or 500).
// aimErrorHandler IS the four-argument handler — pass it to app.use, do not call it.
app.use(aimErrorHandler);
```

The middleware authenticates to AIM as a registered agent, and `apiKey` alone does not give it an identity. Pass the agent's credentials as `credentials` (an `AgentCredentials` object, for example from `loadCredentialsFromFile`), pass a ready `client`, or set all four of `AIM_AGENT_ID`, `AIM_PRIVATE_KEY`, `AIM_PUBLIC_KEY` and `AIM_ORGANIZATION_ID` in the environment. Without an identity every verified route answers 401 and AIM is never contacted; when the environment is one variable short, the 401 names the missing variable ([#449](https://github.com/opena2a-org/agent-identity-management/issues/449)). `aimErrorHandler` answers every SDK error as JSON. Denials and authentication failures get the bodies `verifyAction` sends. Any other SDK error, including an upstream 5xx or a network failure, gets the shape the Fastify plugin gives it, for example `{"statusCode":500,"code":"API_ERROR","error":"Internal Server Error","message":"..."}`, with no stack trace ([#450](https://github.com/opena2a-org/agent-identity-management/issues/450)). Errors that did not come from the SDK are passed on to `next(error)` unchanged.

## Fastify Integration

```typescript
import Fastify from 'fastify';
import { loadCredentialsFromFile } from '@opena2a/aim-sdk';
import { aimPlugin, verifyAction } from '@opena2a/aim-sdk/fastify';

const fastify = Fastify();

// Register the AIM plugin with the registered agent's credentials
await fastify.register(aimPlugin, {
  baseUrl: 'https://aim.example.com',
  credentials: await loadCredentialsFromFile('./agent-credentials.json'),
});

// Verify actions with preHandler hook
fastify.post('/api/data', {
  preHandler: verifyAction('data:write'),
}, async (request, reply) => {
  return { success: true };
});
```

The plugin takes the same `credentials` or `client` options as the Express middleware, or reads the same four environment variables; with `{ baseUrl, apiKey }` alone every verified route answers 401 ([#449](https://github.com/opena2a-org/agent-identity-management/issues/449)).

## Local Credential Verification (offline)

An ATX (Agent Trust eXtension) credential is a signed, portable credential
designed to be verified *locally*: the signature is checked against the issuer's
cached public key in roughly a millisecond, the issuing node is never on the
verification path, and revocation rides on an asynchronously-refreshed,
short-lived cached list. When you configure `localVerification` with cached trust
anchors, the client verifies a resolved credential offline and decides
authorization from the credential's own signed claims, with no per-action call to a
central service. The remote `verifyAction` POST is retained as the fallback for
when no local credential is available.

The verifier is the shared, conformance-locked `@opena2a/atx-verify`, byte-for-byte
interoperable with the Go (`opena2a-registry/pkg/atcverify`) and Python reference
verifiers. The SDK adds the checks that package leaves out, as the Java SDK's
verifier makes them: every ML-DSA-65 signature a credential declares must verify
against an `ML-DSA-65` key in `publicKeys` (an issuer that signs with both suites
needs both keys there), a signature algorithm other than `Ed25519` and `ML-DSA-65`
rejects the credential, and a v1.1 `issuerChain` DID lends its keys only when it
is also in `trustedIssuers`.

```typescript
import { AIMClient } from '@opena2a/aim-sdk';

const client = new AIMClient({
  // Cached once from AIM/the Registry; refresh the CRL off the hot path.
  localVerification: {
    trustedIssuers: ['did:opena2a:issuer-1'],
    publicKeys: [
      { algorithm: 'Ed25519', publicKeyHex: '<issuer raw ed25519 pubkey hex>' },
      // { algorithm: 'ML-DSA-65', publicKeyHex: '<issuer raw ML-DSA-65 pubkey hex>' }, // issuer signs with both suites
    ],
    // crl: { entries: [{ agentId, reason }] }, // optional cached revocation list
  },
});

// Resolve the agent's ATX once (the AAP broker / network step), then cache it.
// `resolvedAtx` is the signed ATX credential returned by that broker/network step.
const resolvedAtx = await fetchAtxForAgent(); // your resolution step
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
but then authorization runs on holder-forgeable capabilities. Only do this for a
closed, trusted v1.0 deployment, never across a trust boundary.

> **Multi-issuer anchor sets.** Give each key a DID-URL `keyId` (e.g.
> `did:opena2a:authority:opena2a.org#key-1`). `@opena2a/atx-verify` binds a key
> to its controller DID, so a key may only verify credentials issued by that DID
> (or, for v1.1, a signed `issuerChain` authority): one trusted issuer cannot
> impersonate another. A key with no `keyId` fragment is unbound and eligible for
> any issuer: fine for a single-issuer anchor set, unsafe for a multi-issuer one.

For credential verification without the action-authorization adaptation, use the
verifier directly:

```typescript
const verifier = client.getLocalVerifier();
const { valid, context, rejectCategory } = await verifier!.verifyCredential(atx);
```

When you hold the credential as it arrived on the wire, pass its JSON text (or
bytes) instead of a parsed object. `verifyCredential` and `authorize` both accept
it, and strict-parse it before reading any field: a credential with a duplicate
member at any depth, including two names that differ only in case, is rejected
as `MALFORMED`. A parsed object cannot get that check, because `JSON.parse` has
already kept one of the duplicates and dropped the other.

```typescript
const result = await verifier!.verifyCredential(credentialJson); // string or Uint8Array
```

> Network is reserved for credential *resolution* (the AAP broker hands the agent
> its ATX) and the periodic CRL refresh, never for a per-action decision.

## Delegation Chains

A delegation is a signed statement that one key (the delegator) grants a set of
scopes to another key (the delegate), optionally chained so authority passes from
a root through intermediaries to a leaf. Delegations use `did:key` (Ed25519) and a
canonical signing form for cross-engine interop.

```typescript
import {
  generateKeyPair,
  createDelegation,
  verifyDelegation,
  verifyDelegationChain,
} from '@opena2a/aim-sdk';

const root = await generateKeyPair();
const coordinator = await generateKeyPair();
const worker = await generateKeyPair();

// Root delegates to a coordinator, which sub-delegates a narrower scope set.
const d1 = await createDelegation({
  delegatorKeyPair: root,
  delegatePublicKey: coordinator.publicKey,
  scopes: ['search', 'memory.read', 'memory.write'],
  // expiresAt defaults to 7 days from now; pass an ISO-8601 string to set it.
});

const d2 = await createDelegation({
  delegatorKeyPair: coordinator,
  delegatePublicKey: worker.publicKey,
  scopes: ['search', 'memory.read'], // must be a subset of the parent's scopes
  parentDelegation: 'd1',
  parentExpiresAt: d1.expiresAt,     // a child must not outlive its parent
});

const { valid, results } = await verifyDelegationChain([d1, d2]);
// valid === true only if every hop passes: signature, delegator identity,
// scope narrowing, chain linkage, trust attenuation, AND temporal validity.
```

`verifyDelegation` and `verifyDelegationChain` enforce BOTH bounds of the
delegation's signed `createdAt`/`expiresAt` window. An expired delegation is
rejected, a delegation evaluated before its signed `createdAt` is rejected as
not yet valid, and verification fails closed on a missing, unparseable, or
inverted (`createdAt` after `expiresAt`) timestamp. A chain is evaluated against a single instant so every hop is judged by
the same clock, and a child that outlives its parent is rejected even while the
parent is still live (a delegate cannot hold authority in time beyond its
delegator). When creating a sub-delegation, pass `parentExpiresAt` so the child's
default expiry is capped at the parent's and an over-long child expiry is refused
at creation.

Pass an explicit evaluation time for deterministic tests or offline / as-of
verification:

```typescript
// Verify as the chain would have stood at a specific instant.
await verifyDelegation(d1, { verifyAt: '2026-07-15T00:00:00.000Z' });
await verifyDelegationChain([d1, d2], { verifyAt: new Date('2026-07-15T00:00:00Z') });
```

For the raw signature check without temporal evaluation (archival or audit
inspection where authenticity matters independent of time), use
`verifyDelegationSignature`; for the standalone temporal check and its reason
string, use `checkDelegationTemporalValidity`:

```typescript
import { verifyDelegationSignature, checkDelegationTemporalValidity } from '@opena2a/aim-sdk';

await verifyDelegationSignature(d1);                       // true if the signature is authentic
checkDelegationTemporalValidity(d1);                       // { valid, error? } at the current time
checkDelegationTemporalValidity(d1, '2026-07-15T00:00:00.000Z');
```

## Causal-Denial Telemetry (opt-in)

The SDK can correlate *why* a blocked agent action happened by joining three
signals around one verified action: the authorization outcome (an observed
fact), the classified intent, and the injection cause (both inferences). The
full **correlated record** is authoritative and stays on the machine; only an
anonymized **shared indicator** is ever uploaded. This section covers the
causal-denial channel only. The runtime-protection module ships a separate
structural-signature channel that is also **off by default**; see the Runtime
Protection section for its scope and how to turn it on.

The causal-denial channel is **off by default** and gated by two independent
opt-ins:

1. **Capture** (`telemetry.enabled`): mints a correlation ID per `verifyAction`,
   assembles records, and appends them to a local log at
   `~/.opena2a/correlated-events.jsonl`. Nothing leaves the machine.
2. **Share** (`telemetry.relay.enabled`): a best-effort relay reduces local
   records to anonymized indicators and uploads only `denied_injection_attempt`
   events to the public, count-only Registry endpoint
   (`POST /api/v1/telemetry/runtime`).

   The shared indicator carries **no** payloads, paths, credentials,
   resource/capability names, correlation ID, agent ID, or denial reason text.
   It carries only: a validated Threat Matrix `techniqueId` (`T-NNNN`, dropped
   if malformed) and its source, a detection confidence, the coarse enforcement
   outcome (`deny`/`allow`), the integrator's self-declared `packageName`/
   `agentCategory`, `daySinceInstall`, `runtimeEnv`, `triggeredAt`, and a
   `sensorToken`, a stable per-device pseudonym (`sha256(host+user+local salt)`)
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
It observes an agent process from the inside with monitors and interceptors for
process, network, filesystem, prompt, MCP, and A2A activity; a rule-based
event engine (L0); a behavioral anomaly twin (L1); and an intelligence
coordinator (L2). It produces the detection inputs shown in the telemetry
section above.

The boundary with the `hackmyagent` scanner: scan-time analysis (static
scanning, hardening rules, artifact classification) lives in hackmyagent;
runtime protection lives here, inside the agent process. This module is the
canonical home of the ARP engine; hackmyagent's `./arp` export is being
converted to a thin re-export of it.

### Default rules

The event engine (L0) checks each event against a list of rules and sends the
action of every rule that matches to enforcement, so one event can trigger more
than one action. When the config object given to
`new AgentRuntimeProtection(config)` or `new EventEngine(config)` has no `rules`
field, the list is:

| Rule | Condition | Action | `requireLlmConfirmation` |
| --- | --- | --- | --- |
| `critical-threat` | `{ category: 'threat', minSeverity: 'critical' }` | `kill` | `true` |
| `high-violation` | `{ category: 'violation', minSeverity: 'high' }` | `alert` | |
| `anomaly-burst` | `{ category: 'anomaly', threshold: { count: 10, windowMs: 60000 } }` | `alert` | `true` |
| `prompt-injection-detected` | `{ source: 'prompt', category: 'threat', minSeverity: 'high' }` | `alert` | |
| `credential-leak-detected` | `{ source: 'prompt', category: 'threat', fieldMatch: { 'data.patternId': 'OL-001' } }` | `alert` | |
| `mcp-exploitation-detected` | `{ source: 'mcp-protocol', category: 'threat', minSeverity: 'high' }` | `alert` | |
| `a2a-spoofing-detected` | `{ source: 'a2a-protocol', category: 'threat', minSeverity: 'high' }` | `alert` | |

A `threshold` counts every event in the window from the rule's `source` (from
all sources when `source` is not set), not only events that meet the rest of the
condition: an anomaly that arrives after nine other events within 60 seconds
triggers `anomaly-burst`.

With L2 off, which is the default, an event classified as a critical `threat`
(an L0 pattern match, for one) ends in `kill` from `critical-threat` as soon as
it is emitted. `requireLlmConfirmation` does not hold that back: the rule's
action runs at once with or without the flag, which only records
`_llmReviewRequested`, `_initialAction` and `_matchedRule` on `event.data`. With
L2 on, L2 assesses such an event before the rules are evaluated, unless its
budget is spent, so the action waits for its answer, and that answer can change
the event's category and severity and with them the rules that match: an L2
answer of `PAUSE` turns a critical threat into a high violation, which
`high-violation` answers with `alert` instead of `kill`.

`rules` in the ARP config replaces this list; it is not merged with it, so copy
any default rule you want to keep, and `rules: []` means no rule fires.
`loadConfig()`, which `new AgentRuntimeProtection()` uses when given no argument
or a config file path, starts from `defaultConfig()` with `rules: []`: ARP built
that way, from a config file without `rules` or with no config file at all, has
no rules and sends no action to enforcement until the config lists some.

```typescript
import { AgentRuntimeProtection } from '@opena2a/aim-sdk/arp';

const arp = new AgentRuntimeProtection({
  agentName: 'my-agent',
  rules: [
    // critical-threat, with alert in place of kill
    { name: 'critical-threat', condition: { category: 'threat', minSeverity: 'critical' }, action: 'alert' },
    { name: 'high-violation', condition: { category: 'violation', minSeverity: 'high' }, action: 'alert' },
  ],
});
```

### L2 sends nothing unless you configure it

L0 (rules) and L1 (behavioral twin) run entirely in-process and make no
outbound connections. **L2 is off by default**, and turning it on takes two
deliberate settings, not one:

```typescript
intelligence: {
  enabled: true,        // absent or false means L2 does not run
  adapter: 'ollama',    // required: there is no automatic provider selection
}
```

Setting `enabled: false` also switches off the in-process behavioral twin, which
reads the same flag; leave it absent if you only want L2 off.

`adapter` chooses where inference runs. `'ollama'` keeps it on the machine.
`'anthropic'` and `'openai'` send event content to that vendor, authenticated
with your own key and billed to your own account. Choose them only if you
intend that.

If you do choose a remote adapter, this is what leaves and what does not.
`event.data` is allowlisted: identifiers and structural facts (pattern id,
category, direction, tool name, peer agent ids, protocol, port, pid) travel,
and everything else (matched text, command lines, arguments) is replaced by a
descriptor that buckets the length rather than reporting it: `<withheld: 9-16
chars, lower+digit>`. At eight characters or fewer the descriptor is
`<withheld: up to 8 chars>` and names no character classes, because an exact
length and a class set do not summarise a short value, they reconstruct it. An
empty value renders `<withheld: empty>`. The event's `description` is **not**
redacted and is sent as written; it is normally tool-authored text plus an
identifier such as a hostname or file path, but process-monitor descriptions
embed up to 100 characters of the child command line.

If `enabled` is true but no `adapter` is set, L2 does not run and ARP says so
once on startup; L0 and L1 are unaffected and still gate.

To read the same from code, `arp.getStatus().intelligence.l2` holds `running`
and, when L2 does not run, a `reason` of `not-enabled`, `no-adapter` or
`adapter-failed`. `intelligence.twin.running` says whether the behavioral twin
was built, and `intelligence.twin.fleetGradient` whether it would send its
fleet gradient at a flush now. The block holds booleans and codes from fixed
sets only, so no value from `adapterConfig` appears in it.

A destination that cannot be reached, such as no Ollama server listening at
`adapterConfig.host` (default `http://localhost:11434`), is reported once with
an `[ARP] L2 call failed` line on stderr, and each call that fails is counted in
`getStatus().budget.failedCalls`, with the latest in `lastFailure`.

> **Changed after 1.3.1.** Versions 1.0.0 through 1.3.1 defaulted to
> `enabled: true` with `adapter: 'agent-proxy'`, which picked a destination from
> whichever model key was exported in the environment, so a default
> installation on a machine with `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` set
> sent event content to that vendor. `'agent-proxy'` now throws rather than
> guessing. See the CHANGELOG for the upgrade note.

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
them: ESM bindings are resolved at link time and cannot be patched. The
monitors (which poll rather than intercept) are unaffected. Start interceptors
as early as possible in the process, and treat them as one observation layer,
not a sandbox.

**The pattern analyzers are not interchangeable.** Each one runs its own
pattern set, and its `detected: false` covers that set only. Every result
names the set in `scope.set` and lists the id of every check that ran, matched
or not, in `scope.patternIds`; a pattern whose id is absent was not evaluated.

| Analyzer | `scope.set` | Checks that run | `detected: false` means |
|---|---|---|---|
| `PromptInterceptor.scanInput(content)` | `input` | `PI-*`, `JB-*`, `DE-*`, `CM-*` (`PATTERN_SETS.inputPatterns`) | no prompt-injection, jailbreak, data-exfiltration or context-manipulation pattern matched |
| `PromptInterceptor.scanOutput(content)` | `output` | `OL-*` (`PATTERN_SETS.outputPatterns`) | no API-key, PII or system-prompt pattern matched |
| `MCPProtocolInterceptor.scanToolCall(name, args)` | `mcp` | `MCP-ALLOWLIST` when an allowlist is set, then `MCP-*` (`PATTERN_SETS.mcpPatterns`) over the flattened argument values | the tool is on the allowlist, if one is set, and no path-traversal, command-injection, SSRF or tool-registration pattern matched |
| `A2AProtocolInterceptor.scanMessage(from, to, content)` | `a2a` | `A2A-TRUST` when a trusted-agent list is set, then `A2A-*` (`PATTERN_SETS.a2aPatterns`) and `PI-*` (`PATTERN_SETS.promptInjection`) | the sender is on the trusted-agent list, if one is set, and no spoofing, delegation-abuse or prompt-injection pattern matched |
| `scanText(text, patterns)` | `custom` | the ids of `patterns` | none of those patterns matched |

For example, `scanInput` does not run the MCP command-injection pattern
(`MCP-002`), so a clean `scanInput` result on a file with shell one-liners is
not a clean verdict on the file. After an allowlist refusal, `scanToolCall`
lists only `MCP-ALLOWLIST`, because it did not scan the arguments. A result
with `truncated: true` covers only the first 64 KB of the text. An analyzer
that is not started returns `scanned: false` and an empty `patternIds`: it
evaluated nothing, and its `detected: false` is not a verdict.

Classification of runtime events is supplied through the injectable
`ClassificationProvider` seam. The default `NanoMindGuardClassificationProvider`
talks to the local NanoMind-Guard daemon over a Unix socket and only accepts
signed classification results; when the daemon is absent the annotator degrades
to "no classification". It never fabricates a label and never blocks.

**Invariant:** the runtime-protection module is a telemetry producer. Its
detection outputs flow through the `telemetry.detection` seam into the
correlated record; they never enter `verifyAction`'s allow/deny decision.

**Structural signature telemetry (off by default, opt-in).** Like the
causal-denial channel above, the ARP engine shares nothing unless you turn it
on. When enabled, and when a detection fires, the structural shape of the
event sequence (technique identifier, event-type sequence, severity, and a
one-way hash of structural tokens) is signed and reported to the OpenA2A
Registry (`https://api.oa2a.org`). A report is stored there; today nothing is
sent back to any install from it, and the patterns ARP matches are the ones
bundled in the package. Prompts, model responses, tool arguments, file
contents or paths, command lines, environment values, secrets, IP addresses,
hostnames, and account, tool, agent, and model names never appear in a report.
A report does carry a random per-install sensor id and a monthly-rotating org
pseudonym alongside the structural fields. A one-time disclosure is printed
before first collection, and every payload is appended to a local audit log
(`~/.opena2a/telemetry-audit.log`, JSONL) before it is sent. Review it with
`aim-arp telemetry log` (or `npx @opena2a/aim-sdk telemetry log`).

Turn it on with either of:

- `AIM_TELEMETRY=1` in the environment, or
- `signatureTelemetry: { enabled: true }` in your ARP config.

Turn it off again (an opt-out always wins over an opt-in) with any one of:

- `OPENA2A_TELEMETRY=off` in the environment. This is the ecosystem-wide switch
  documented at [opena2a.org/privacy](https://opena2a.org/privacy), and it works
  here the same way it works on the CLIs (`off`, `0`, `false` and `no` are all
  accepted). It is read in the off direction only: `OPENA2A_TELEMETRY=on` will
  not turn this channel on, because a library running inside your process should
  not start a network channel on the strength of an ecosystem-wide CLI setting.
- `OPENA2A_TELEMETRY_OPTOUT=1` (or `ARP_TELEMETRY_DISABLED=1`) in the
  environment,
- `signatureTelemetry: { enabled: false }` in your ARP config,
- `aim-arp telemetry opt-out`, the consent CLI this package installs (also asks
  the registry to delete already-sent signatures), or
- `writeOptOutMarker()` from `@opena2a/aim-sdk/arp`, which persists
  `~/.opena2a/telemetry-optout` across processes.

Any one of these is the runtime-protection module's master switch: it disables
every telemetry channel the module can produce (structural signatures, the
opt-in legacy GTIN runtime channel, and fleet behavioral gradients). The causal-denial channel above is
controlled solely by its own `telemetry` client config and is off unless you
enabled it. To also delete signatures this sensor already shared, run
`aim-arp telemetry purge` (or call `purgeRemoteSignatures()`): right-to-delete;
best-effort, never blocks the local opt-out.

## Configuration

### Environment Variables

The SDK automatically reads from these environment variables:

| Variable | Description |
|----------|-------------|
| `AIM_BASE_URL` | AIM server base URL |
| `AIM_API_KEY` | API key for authentication |
| `AIM_ORGANIZATION_ID` | Organization ID (required with the three agent variables below) |
| `AIM_AGENT_ID` | Pre-registered agent ID |
| `AIM_PRIVATE_KEY` | Ed25519 private key: standard padded base64 of 32 bytes (the seed) or of the 64 bytes AIM issues (the seed, then its public key). Hex is refused |
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

> **Class identity is per module format.** The entry points (`.`, `/arp`,
> `/express`, `/fastify`) share one copy of `AIMClient` and the error classes,
> so an `ActionDeniedError` raised through `@opena2a/aim-sdk/express` is
> `instanceof` the class imported from `@opena2a/aim-sdk`. The ESM and CommonJS
> builds are still two copies: if one process loads the package both through
> `import` and through `require`, `instanceof` does not match across that
> boundary. For checks that must hold there, match on `error.code`
> (e.g. `'ACTION_DENIED'`) instead.

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
- `getAgent()` - Get current agent info. Returns `null` (without making any
  request) when no credentials are loaded, and `null` (rather than throwing)
  when the lookup fails
- `updateAgent(updates)` - Update agent metadata
- `reportCapabilities(capabilities)` - Report agent capabilities
- `getTrustScore()` - Get current trust score. Returns `0` when the agent
  cannot be resolved (no credentials, or the lookup failed), so treat `0` as
  "unknown", not as a measured score of zero
- `getCredentials()` - Get stored credentials
- `setCredentials(credentials)` - Set credentials

### Other exported clients

The package exports more than `AIMClient`. The ones you are most likely to
reach for:

- `A2AClient` / `createA2AClient` - Agent-to-agent protocol client for
  caller-initiated messages to a peer agent
- `SecretsClient` - Identity-native secrets client; usually reached through
  `client.secrets` rather than constructed directly
- `OAuthTokenManager` - The token acquisition and caching `AIMClient` uses for
  its authenticated requests, reusable standalone
- `CrlCache` - Background-refreshed revocation-list cache that feeds
  `LocalVerifier` for offline credential verification

### Types

See
[src/types/index.ts](https://github.com/opena2a-org/agent-identity-management/blob/main/sdk/typescript/src/types/index.ts)
for complete type definitions (source is not shipped in the npm package; the
bundled `.d.ts` files carry the same types).

## Local mode with `@opena2a/aim-core`

```typescript
import { AIMCore } from "@opena2a/aim-core";

const agent = new AIMCore({ agentName: "my-first-agent" });
agent.getIdentity(); // creates the Ed25519 keypair on first use

agent.loadPolicy({ allow: ["db:read"], default: "deny" });
const allowed = agent.checkCapability("db:read"); // true; "db:write" is false
agent.logEvent({
  plugin: "my-first-agent",
  action: "db:read",
  target: "users_table",
  result: allowed ? "allowed" : "denied",
});
```

The only SDK that runs without a server today. Backs local mode: the example writes `identity.json` and one `audit.jsonl` line under `~/.opena2a/aim-core/`. It runs as written with `@opena2a/aim-core` 0.2.0 (measured 2026-10-04). Source: [`packages/aim-core`](https://github.com/opena2a-org/opena2a/tree/main/packages/aim-core).

The client for an AIM server is a separate package, `@opena2a/aim-sdk`, documented in this README. In 1.3.1, the published version, `verifyAction` does not complete against a self-hosted server: it requests `/oauth/token` and then `/api/v1/verify` (measured 2026-10-04 on the published package), and the server registers neither path (it serves tokens at `/api/v1/oauth/token`). The source in this repository posts the action to `/api/v1/sdk-api/verifications`, signed with the agent's key, and makes no token request for it; it ships in the next release ([#662](https://github.com/opena2a-org/agent-identity-management/issues/662)).

## License

Apache-2.0

## Contributing

See
[CONTRIBUTING.md](https://github.com/opena2a-org/agent-identity-management/blob/main/CONTRIBUTING.md)
for guidelines.
