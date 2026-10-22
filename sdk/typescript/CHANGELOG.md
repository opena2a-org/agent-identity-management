# Changelog

All notable changes to `@opena2a/aim-sdk` are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this package adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed: the README's `init` section says what `init` needs from the server and how a bootstrap token is minted

- "Register a first agent from the terminal" says that `init` needs an AIM server that has onboarding tokens, which
  are in builds of this repository's main branch from 2026-10-08 on and in no release yet, and how `aim-arp init` and
  the Python SDK's `aim-sdk init` report the HTTP 404 a released server answers the token exchange with:
  `has no onboarding endpoint (HTTP 404)` and `HTTP 404 from <url>/api/v1/onboarding/bootstrap-tokens/exchange`.
- The section said that the onboarding screen in the dashboard generates the bootstrap token. The dashboard has no
  screen that mints one; the section now says that a signed-in member, manager or admin mints one with
  `POST /api/v1/onboarding/bootstrap-tokens`, and links to that route in `docs/API.md`. The source-build block and
  the release condition before the npx form are unchanged.

### Fixed — ARP's `loadConfig` names the file and gives the parser's message when a config does not parse (#861)

- Run on the code before this change (Node.js 26.12.0, js-yaml 4.3.2), `loadConfig(path)` from
  `@opena2a/aim-sdk/arp` failed with `Failed to parse config: <path>. Install js-yaml for YAML support.` for a
  `.yaml` file holding `monitors: [unclosed` and for a `.yml` file holding `agentName: [a`, with js-yaml installed.
  A `.json` file holding `{,}` failed with a `SyntaxError` whose message,
  `Expected property name or '}' in JSON at position 1 (line 1 column 2)`, does not name the file.
- A YAML or JSON config that does not parse now fails with `Failed to parse config: <path>: <parser message>`. For a
  YAML config the parser message is js-yaml's reason with the line and column, and the error has no `cause`: the
  `.yaml` file above gives `unexpected end of the stream within a flow collection (2:1)`. js-yaml's own message ends
  with an excerpt of the file, and the lines around a syntax error can hold a value such as
  `intelligence.adapterConfig.apiKey`, so neither that message nor the js-yaml error is kept. For a JSON config the
  parser message is the `SyntaxError`'s, and that error is the `cause`: on Node.js 26.12.0 the `.json` file above
  gives the message quoted above. The install hint appears when js-yaml cannot be loaded; a JSON config still loads
  then.
- Tests: `src/arp/config/loader.parse-errors.test.ts`.

### Fixed — `init` removes only group and other permissions from an existing `~/.aim` or `~/.aim/agents`, and prints each mode it changes (#860)

- Measured with the previous code: `init` set an existing `~/.aim/agents` at 0755, 0555 or 0500 to 0700 and printed
  no line about it, so at 0555 or 0500 it added the owner's write permission and registered the agent. An existing
  `~/.aim` at 0755 stayed 0755.
- An existing `~/.aim` or `~/.aim/agents` with group or other permission bits now loses those bits and keeps its owner
  bits, and one line on stderr names it with its old and new modes:
  `Changed the mode of /home/user/.aim from 0755 to 0700, removing its group and other permissions.` A folder with no
  group or other permission bits keeps its mode. A folder `init` creates has mode 0700, as before.
- `init` still writes and removes a file in `~/.aim/agents` before it sends the token, so a folder at 0500, or at 0555,
  which becomes 0500, is refused with `Error: cannot save the agent's key file in <folder> (EACCES); the token has not
  been used.` and no request is sent.
- Tests: `src/cli/init.test.ts`.

### Added — ARP's `getStatus()` says whether L2 and the runtime twin run (#857)

- `AgentRuntimeProtection.getStatus()` carries an `intelligence` block. Measured on the code before this change,
  `Object.keys(getStatus())` was `running, monitors, budget, pausedPids` with `intelligence: { enabled: false }` and
  with `intelligence: { enabled: true, adapter: 'ollama' }`.
- `intelligence.l2.running` says whether L2 runs. When it runs, `l2.adapter` is the kind of the adapter it built:
  `anthropic`, `openai` or `ollama`. When it does not run, `l2.reason` is `not-enabled` (`intelligence.enabled` is
  not `true`), `no-adapter` (`enabled` is `true` and no `intelligence.adapter` is set) or `adapter-failed` (the named
  adapter could not be built), and with `adapter-failed`, `l2.adapter` is the kind of the configured name, `other`
  for a name outside those three: `adapter: 'agent-proxy'` reports
  `{ running: false, adapter: 'other', reason: 'adapter-failed' }`.
- `intelligence.twin.running` says whether the runtime twin was built, and `intelligence.twin.fleetGradient` whether
  the twin would send its fleet gradient at a flush now.
- The block is read from the objects the instance built: `l2` from the L2 decision the intelligence coordinator
  stores as codes when it is built (`IntelligenceCoordinator.getL2Status()`, which returns a copy), and
  `twin.fleetGradient` from `RuntimeTwin.wouldSendFleetGradient()`, which the twin's send path calls before it posts,
  so an opt-out marker written after the twin was built turns it to `false`. It holds booleans and codes from fixed
  sets only, so it carries no `adapterConfig` value, model name, error message or environment variable's value.
  `l2.running` and `twin.running` are settled when the instance is built, so `start()` does not change them.
- `IntelligenceStatus`, `L2Status`, `L2AdapterKind` and `L2NotRunningReason` are exported from
  `@opena2a/aim-sdk/arp`.
- Test: `src/arp/get-status-intelligence.test.ts`. It plants marker values in `adapterConfig.host`,
  `adapterConfig.model` and an unknown adapter name, and finds none of them in `JSON.stringify(getStatus())` while
  the config holds each of them. It checks `l2.running` against `describeL2Status` in
  `src/arp/intelligence/coordinator.ts`, and `twin.fleetGradient` against the gradient posts a flush makes: with
  fleet gradients on, after an opt-out marker is written, with `OPENA2A_TELEMETRY_OPTOUT` set when the twin is built,
  and with fleet gradients off.

### Fixed — `aim-arp` answers an argument it does not know with a pointer to `--help`, `init` refuses a name whose credentials file name is over 255 bytes before it sends the token, and the telemetry text drops emphasis capitals and em dashes (#852)

- `aim-arp` with an argument it does not know printed `Unknown command: <argument>` on stderr and the full help on
  stdout, and exited 1: `--frob` was called a command, and a bootstrap token given as the argument (`aim_ob_FAKE`,
  `--paste=aim_ob_FAKE`) was printed back as typed. Measured before this change with the built bin and the dispatch
  tests. Now an unknown argument that starts with `-` prints `Unknown option: <argument>`, one that does not prints
  `Unknown command: <argument>`, and each is followed by `Run: aim-arp --help`. Both lines go to stderr, nothing goes
  to stdout, and the exit status is still 1. A bootstrap token in the echoed argument is printed as
  `aim_ob_[REDACTED]`, as `init` prints one.
- `aim-arp init` refuses a name whose credentials file name (`<name>.json` under `~/.aim/agents`) is over 255 bytes in
  UTF-8, before it sends the token and before it writes anything under the home directory: an `Error:` line ending
  `Nothing was sent.` and a line naming `--name`, both on stderr, and exit status 1. Measured before this change
  against a stand-in server on 127.0.0.1, on macOS: `--name` with 251 ASCII letters (a 256-byte file name) sent the
  token, the stand-in answered 201, and saving the key file then failed with `ENAMETOOLONG`. A name of 126 two-byte
  letters (a 257-byte file name of 131 characters) was sent and saved there; it is refused now, as the limit is
  counted in bytes. A name whose file name is exactly 255 bytes is still sent, tested with 250 ASCII letters and with
  125 two-byte letters.
- The telemetry text is lower case where it used capitals for emphasis, and uses plain punctuation where it used an em
  dash; no word is added, removed or moved. Measured before this change: `aim-arp telemetry --help`,
  `aim-arp telemetry disclosure` and `aim-arp telemetry opt-out` printed `ALL`, `SHAPE`, `STRUCTURAL SHAPE`, `NEVER`,
  `BEFORE` and `DISABLED`, and three em dashes, and `aim-arp telemetry opt-in` under an environment opt-out printed
  `NOTE:`. The opt-out error for a telemetry home it cannot write now says `did not take effect`. `ON` and `OFF` as a
  state, and environment variable names, keep their capitals.
- Tests: `src/arp/cli/aim-arp-argv.test.ts` and `src/cli/init.test.ts`. `src/arp/cli/telemetry-polish-412.test.ts`
  pins `did not take effect`, and `src/arp/cli/telemetry-help-noexec.test.ts` checks that `opt-out --help` does not
  print the opt-out's `OpenA2A telemetry disabled` line.

### Added — ARP reports a failed L2 call once and counts it in the budget status

- An L2 call that throws, sent for one event or for a batch, prints one line with `console.warn`, the channel of the
  line ARP prints when L2 is not running: `[ARP] L2 call failed (<adapter>, <endpoint>): <error>. L0 and L1 are
  unaffected and still gate.`, then a next step. For `ollama` the endpoint is the server the adapter connects to and
  the next step is `Check that an Ollama server is listening there: curl -sS <endpoint>/api/tags`; for `anthropic`
  and `openai` it names the key variable, `intelligence.adapterConfig.apiKey` and `curl -sSI` on the vendor endpoint.
  The error is named by its code (`ECONNREFUSED`, `ENOTFOUND`), an HTTP error by its status alone (`HTTP 500`), and
  an answer that is not JSON as `the answer was not valid JSON`, so the line carries no text of the prompt, the event
  or the server's answer. No command is printed for a host that a shell would not hand to curl as written, such as
  one holding `$(`. The line is printed once for each run of failed calls: a call that answers re-arms it, and the
  next call that fails prints it again.
- `BudgetController.getStatus()`, which `AgentRuntimeProtection.getStatus().budget` returns unchanged, carries
  `failedCalls`, the number of L2 calls that threw, and `lastFailure`: `null`, or `{ at, adapter, message }` with the
  time as an ISO string, the adapter's name and the error as the line names it. `totalCalls` still counts the
  answered calls only. `budget.json` already kept `totalCalls` across restarts, and it keeps both new fields with it,
  so a status read in another process sees them; they start again from zero with the monthly period, as
  `totalCalls` does, and a `budget.json` saved before this change reads as `failedCalls: 0` and `lastFailure: null`.
  `BudgetController.recordFailure(adapter, message)` counts one failed call.
- `AgentRuntimeProtection.start()` calls the L2 adapter's `healthCheck()` once, through the new
  `IntelligenceCoordinator.checkL2Health()`, and prints the same line when the check fails, throws or does not answer
  within 2 seconds. `start()` waits for it at most 2 seconds, while the monitors start, and does not fail because of
  it. A failed check is not counted in `failedCalls`, which counts calls. The check runs only when L2 runs:
  `intelligence.enabled` is `true` and the adapter was built. The Ollama adapter's check is a `GET /api/tags` to its
  host; the vendor adapters' check reads whether a key is set and sends nothing.
- The built-in adapters carry `endpoint`, the URL they send to; `LLMAdapter.endpoint` is optional.
- Measured before this change with `intelligence: { enabled: true, adapter: 'ollama' }` and nothing listening at the
  host: two events through `analyze()` printed nothing, `getBudgetStatus()` read `totalCalls: 0` and had no field for
  the failed calls, and each event kept `classifiedBy: 'L0-rules'`. On 1.0.0 through 1.3.1 a failed L2 call left no
  trace: nothing was printed, the budget status did not count it, and the event was logged as L0 and L1 left it.
- Tests: `tests/arp/intelligence/coordinator-l2-failure.test.ts`, and the failed-call cases in
  `tests/arp/intelligence/budget.test.ts`.

### Fixed — `SecretsClient` refuses a namespace ID that is not a UUID before it sends a request (#832)

- `getNamespace`, `deleteNamespace`, `storeCredential` and `rotateCredential` put the namespace ID into the request
  path as passed. Measured before this change against a server on 127.0.0.1, with `AIMClient`'s request as the host:
  `deleteNamespace('../../agents/<id>')` sent `DELETE /api/v1/agents/<id>`, `getNamespace('../../agents/<id>')` sent
  `GET /api/v1/agents/<id>`, `storeCredential` and `rotateCredential` with the ID `../x` sent
  `POST /api/v1/secrets/x/credentials` and `POST /api/v1/secrets/x/rotate`, `getNamespace('a/b')` sent
  `GET /api/v1/secrets/namespaces/a/b`, and a `?` or `#` in the ID ended the path there.
- The four methods now reject with `ConfigurationError` and make no request when the ID is not a UUID written as the
  server answers a namespace's `id`: 32 hexadecimal digits, of either letter case, in groups of 8, 4, 4, 4 and 12
  joined by hyphens. They percent-encode the ID into the path. The message begins
  `the namespace ID is not a UUID; the request was not sent` and ends with a `Fix:` line.
- Tests: `src/secrets/SecretsClient.namespace-id.test.ts`. The namespace and credential storage tests in
  `src/secrets/SecretsClient.test.ts` pass a UUID.

### Security — ARP's proxy listens on 127.0.0.1 unless its config sets a host (#853)

- `ARPProxy.start()` called `listen` with the port and a callback and no host argument, as a spy on
  `http.Server.prototype.listen` recorded it.
- `ProxyConfig` gains an optional `host`, the address the proxy listens on, which `start()` passes to `listen`.
  When it is unset or an empty string, `start()` passes `127.0.0.1`, exported as `DEFAULT_PROXY_HOST`; a proxy
  started with no host and port 0 reports `127.0.0.1` as its listening address.
- An operator whose clients reach the proxy from another host sets `proxy.host` (for example `0.0.0.0`) in the
  config file.
- Tests: `tests/arp/proxy/listen-address.test.ts`.

### Security

- **Breaking change for code that sets `requireSignedRequests` or `maxTimestampAge`.** `A2ASecuritySettings`
  carries the member names the server reads and answers with: `requireRequestSigning` in place of
  `requireSignedRequests`, and `nonceValiditySeconds`, in seconds, in place of `maxTimestampAge`. It adds
  `minAttestationCount`, `maxRequestsPerMinute`, `maxRequestsPerHour`, `createdBy` and `updatedBy`, and
  `allowedAgentIds` and `blockedAgentIds` are optional. The server reads neither old name, and it replaces
  every setting with the ones a `PUT /api/v1/a2a/security/settings` carries. Run against the server's handler,
  a body with `requireSignedRequests: true` and no `requireRequestSigning` was answered `200` and stored request
  signing off, and stored `nonceValiditySeconds`, `minAttestationCount`, `maxRequestsPerMinute` and
  `maxRequestsPerHour` as 0 where the body left them out. To change some settings, read them with
  `getSecuritySettings()` and pass the whole object, changed, to `updateSecuritySettings()`. The request is
  pinned in `tests/fixtures/sdk_requests/a2a_update_security_settings.json`, which the backend's handler tests
  post to that route and the Java SDK's tests hold its client to (#652).

- **Breaking change for a verifier that sets `trustAttenuation` or `minDelegatedTrust` on each delegation.**
  `verifyDelegationChain()` takes the trust attenuation factor and the trust floor from its options,
  `trustAttenuation` (default 0.8) and `minDelegatedTrust` (default 0.3), which the verifier sets. It no longer
  applies the fields of those names on a delegation: a delegation's signed payload leaves them out, so whoever passes
  a chain on can set them and every signature still verifies. Run against the code before this change, a chain of
  three delegations made by `createDelegation()` and verified with no options was not valid, with an effective trust
  of 1, 0.5 and 0.25, when each delegation was given `trustAttenuation: 0.5` and `minDelegatedTrust: 0.6`, and was
  valid with an effective trust of 1 at all three when each was given `trustAttenuation: 1`. A chain in which any
  delegation carries either field now fails unless the verifier passes the matching option, with an error naming
  the field, for example `trustAttenuation on a delegation is outside its signature and never applied: pass
  trustAttenuation in the options to verify this chain`. A verifier that set the fields on each delegation passes
  the same values in the options instead: with `trustAttenuation: 0.5` and `minDelegatedTrust: 0.6` in the options,
  the first chain above is again not valid, at 1, 0.5 and 0.25. A factor or floor that is not a number from 0 to 1
  fails the chain with `trustAttenuation and minDelegatedTrust must be numbers from 0 to 1` before any delegation is
  checked (#690, #695).

- `verifyDelegationChain()` fails a chain with `rootTrustScore must be a number from 0 to 1` when the
  `rootTrustScore` option is not a number from 0 to 1, before any delegation is checked. It took the value as given:
  on a chain of three delegations made by `createDelegation()`, `rootTrustScore: NaN` gave a valid chain with an
  effective trust of NaN at each delegation, which no floor fails, and `rootTrustScore: 5` a valid chain at 5, 4
  and 3.2 (#695).

- ARP's classification annotator sends the NanoMind-Guard socket the same allowlisted `event.data` the L2 prompt
  sends: `pid`, `patternId` and the other allowlisted fields verbatim, and `command`, `matchedText` and every other
  field as a withheld length-and-character-class descriptor. It sent `JSON.stringify(event.data)`, so a command line
  or matched text that L2 withholds from a vendor travelled in full, up to the 4,096-character cap, over the local
  socket. `event.description` is sent as before on both channels.

- **One rule for an agent private key, on every path that signs with it.** `AIMClient` (credentials from the
  environment or from `setCredentials`), `createAgentRequestHeaders`, `SecretsClient.resolve`, `createDelegation`,
  `sign` and `createRequestSignature` decode the key with `normalizePrivateKey`, now exported. It accepts 32 bytes, or
  the 64 bytes AIM issues (the seed, then its public key), given as bytes or as standard padded base64 that
  re-encodes to itself. In the 64-byte form the last 32 bytes must be the public key derived from the first 32. Only
  the 32-byte seed reaches the signer. Any other key throws `PrivateKeyError`, a `ConfigurationError` whose `fix`
  says how to convert the key; the error carries no part of the key. Before this change `SecretsClient.resolve`
  decoded the key leniently, so it signed with an unpadded or whitespace-wrapped key, and it threw
  `Uint8Array expected` on the 64-byte form AIM issues. `sign` and `createDelegation` signed with a hex string passed
  as the private key, and threw `Uint8Array expected` on the 64-byte form. Hex is accepted nowhere: convert a hex key
  with `Buffer.from(key, 'hex').toString('base64')`.

### Changed

- The npm "Homepage" link and the first link in this README go to the project page,
  https://opena2a.org/agent-identity-management. They pointed at the repository README and at the hosted sign-up
  page. The repository stays linked as the package's `repository`.
- `aim-arp --version`, `aim-arp telemetry --version` and the first line of `aim-arp --help` print the version
  followed by `+source (built from source; not the published <version>)` unless the package was built by the
  release workflow on its `sdk-ts-v<version>` tag, which prints the bare version. A build of this repository printed
  `aim-arp v1.3.1`, the line the published 1.3.1 prints, while it carried `init`, which 1.3.1 does not have. The
  help header no longer has an em dash: the CLI's name is on its own line. `VERSION`, the package `version` and the
  User-Agent are unchanged.
- The README's `init` section says that 1.3.1 answers `npx @opena2a/aim-sdk init` with `Unknown command: init` and
  shows how to run `init` from a source build. Its note on the L2 default named an unpublished version; it now reads
  "Changed after 1.3.1", and a test holds every "Changed in X" or "Added in X" marker in the README to a release
  heading in this file.
- `new AgentRuntimeProtection()` checks the shape of the `monitors`, `interceptors` and `aiLayer` blocks before it
  builds or writes anything, for a config passed as an object, as a file path or found as `arp.yaml`, `arp.yml` or
  `arp.json`, and throws a `TypeError` that names the key path and the shape it takes. Each entry is an object with
  a boolean `enabled`. A key that is not a documented entry is refused by name: `monitors` takes `process`,
  `network`, `filesystem`, `skill` and `heartbeat`; `interceptors` takes `process`, `network` and `filesystem`;
  `aiLayer` takes `prompt`, `mcp` and `a2a`. `intervalMs` is a finite number of at least 1000; `watchPaths`,
  `allowedPaths`, `allowedHosts`, `allowedTools` and `trustedAgents` are arrays of strings; `maxStaleMs` is a finite
  number above 0. A config that used to be read past now throws at construction. On 1.3.1,
  `monitors: { filesystem: false, network: false }` and `monitors: { fileSystem: { enabled: false } }` left all three
  monitors running, `aiLayer: { prompt: true }` built no prompt interceptor, and an `intervalMs` of `'abc'` or `-5`
  polled `ps` every millisecond; the shape that turns a monitor off is `{ filesystem: { enabled: false } }`. A
  top-level key `ARPConfig` does not have is not refused: it is named in one `console.warn` and construction goes on,
  so a config file that carries a key from another version still starts the agent. `dataDir`, `rules`,
  `declaredCapabilities`, `gtin`, `signatureTelemetry`, `proxy` and the `intelligence` keys other than the three
  `enabled` switches described in the L2 Security entry below are not checked.

### Changed — the README's `init` section opens with the source build, and the npx form follows a release condition

- "Register a first agent from the terminal" in the README opens with the statement that 1.3.1, the published
  version, does not have `init`, and its first command block is the source build
  (`node dist/arp/cli/aim-arp.js init`). The `npx @opena2a/aim-sdk init` block follows, under the sentence "Once a
  release after 1.3.1 is published". The section led with the npx block, which 1.3.1 answers with
  `Unknown command: init`, and its option paragraph pointed to `npx @opena2a/aim-sdk init --help`, which 1.3.1
  answers the same way; that paragraph now points to `node dist/arp/cli/aim-arp.js init --help`.
- Tests: `tests/readme-init-quickstart-published-surface.test.ts` holds that order while `init` is named only under
  `## [Unreleased]` headings in this file, and lifts it once a release heading's section names `init`.

### Deprecated

- `createHybridRequestHeaders`. AIM's request verification does not accept these headers: it verifies a signature
  over the method, path, timestamp and body joined by newlines, and these headers sign
  `timestamp:METHOD:path:sha256(body)`. Its documentation said the headers were compatible with the AIM backend; it
  now says they are not accepted, in the package's type declarations too. The function itself is unchanged.

### Added

- An ARP analyzer's result says what it evaluated. `ScanResult` gains two optional fields: `scope`, which names the
  pattern set (`input` for `PromptInterceptor.scanInput`, `output` for `scanOutput`, `mcp` for
  `MCPProtocolInterceptor.scanToolCall`, `a2a` for `A2AProtocolInterceptor.scanMessage`, `custom` for `scanText`)
  and lists in `patternIds` the id of every check that ran, matched or not; and `scanned`, which is `false` when the
  analyzer is not started and evaluated nothing, so that result is no longer the value a clean scan returns. A
  configured allowlist or trusted-agent list adds `MCP-ALLOWLIST` or `A2A-TRUST` ahead of the set's ids, and after an
  allowlist refusal `scanToolCall` lists only `MCP-ALLOWLIST`, because it does not scan the arguments then.
  `scanText` takes an optional third argument naming the set (default `custom`). `scanMessage` lists the ids of both
  sets it runs (`A2A-*` and `PI-*`) and now sets `truncated`, which it left out before this change, also on content
  longer than 64 KB. `detected`, `matches` and `truncated` are otherwise unchanged. The README's Runtime Protection
  section has a table of each analyzer, the checks it runs and what its `detected: false` covers: `scanInput` does not
  run the MCP command-injection pattern, so a clean `scanInput` result on a file with shell one-liners is not a clean
  verdict on the file.

- `createAgentRequestHeaders` signs a request as an agent with `agent-request-v1`, the Ed25519 request signature AIM
  verifies, and returns the four headers to send with it: `X-Agent-ID`, `X-Timestamp`, `X-Signature` and
  `X-Public-Key`. The signed bytes are the method, the path and query string of the URL, `X-Timestamp`, and, when the
  body is not empty, the body exactly as sent; the URL's host is not signed. `X-Timestamp` is this host's clock in
  Unix seconds, and AIM accepts the request only within 30 seconds of its own clock, earlier or later. There is no
  nonce: anyone who obtains the request can send it to AIM again until AIM's clock is more than 30 seconds past
  `X-Timestamp`. The method must be GET, POST, PUT, PATCH or DELETE in upper case, and the URL absolute http or https
  with no username or password and no empty query (a `?` with nothing after it, which Node 20's fetch drops from the
  request and Node 24's sends). The private key must be standard base64 of 32 bytes (a seed) or 64 bytes (the seed,
  then its public key, as AIM issues it), and its public key must equal the configured one. Anything else throws
  `ConfigurationError`, and nothing is signed. This change does not alter how `AIMClient`, `SecretsClient` and
  `A2AClient` send requests.

- `npx @opena2a/aim-sdk init` registers a first agent with a bootstrap token from the onboarding screen. The token
  is read from `AIM_BOOTSTRAP_TOKEN` only (a flag that carries it is refused before any request and its value is not
  echoed, because a flag value is visible in the process list and shell history) and
  sent only in the `X-AIM-Bootstrap-Token` header to `POST /api/v1/onboarding/bootstrap-tokens/exchange`. The
  agent's Ed25519 key pair is generated locally and only the public key is sent. Credentials are written to
  `~/.aim/agents/<name>.json` (file 0600, directory 0700) in the layout the Python SDK reads, with the file name
  sanitized as the Python SDK does it (letters and digits of any script, `-` and `_` are kept, so `café` is
  `café.json` from either SDK), and the agent's dashboard URL is printed. The dashboard address is the one the
  server reports in the exchange answer (`dashboardUrl`); for a server that reports none it is derived from
  `--url`, mapping port 8080 to 3000 only on a loopback host and dropping only a trailing `/api` path segment.
  `--url` (default `AIM_URL`, then `https://aim.opena2a.org`) points it at a self-hosted server and `--name`
  (default `my-first-agent`) names the agent. The token is never printed
  or saved, server text is scrubbed of token-shaped strings, redirects are not followed (so the token header cannot
  be forwarded to another host), and existing credentials for the name are refused before the single-use token is
  spent. A home directory that cannot hold the key file is refused before the token is spent as well: before any
  request, `init` creates `~/.aim/agents` (0700) and creates and removes an owner-only file in it, and when that
  fails it names the directory and the error code (`ENOTDIR`, `EACCES`), says the token has not been used, and sends
  nothing. The README describes the command under Quick Start.

### Fixed — `SecretsClient.createNamespace` refuses a namespace that `resolve` refuses to sign (#830)

- `createNamespace(agentId, namespace, operations, urlPatterns)` now throws `ConfigurationError` before anything is
  sent when the namespace contains a line feed, a carriage return or a vertical bar (`|`), or begins with the seven
  characters `DSSEv1 `: the namespaces `resolve` refuses to sign.
- The message is `the namespace contains a line feed, a carriage return or a vertical bar (|), which resolve refuses
  to sign; the request was not sent` and `Fix: Pass a namespace that contains no line feed, carriage return or
  vertical bar (|).`, or for the prefix `the namespace begins with "DSSEv1 ", which resolve refuses to sign; the
  request was not sent` and `Fix: Pass a namespace that does not begin with "DSSEv1 ".`
- Before this change, `createNamespace` sent each of `team|prod`, `team`, a line feed and `prod`, `team`, a carriage
  return and `prod`, and `DSSEv1 46 application/vnd.opena2a.action-request.v1+json 2 {}`. A namespace that holds
  `DSSEv1` without the space, in lower case, or after its first character is sent as before.
- Tests: `src/secrets/SecretsClient.test.ts`, one refusal for each of the four, and five namespaces that are sent once.

### Fixed: `SecretsClient.resolve` refuses a namespace or operation that holds a line feed, a carriage return or `|`, and a namespace that begins with `DSSEv1 `

- `resolve(namespace, operation)` signs the text `namespace|operation|nonce`. It now throws `ConfigurationError`
  before anything is signed or sent when the namespace or the operation contains a line feed, a carriage return or a
  vertical bar (`|`), which would make the signed text read as other fields, and when the namespace begins with the
  seven characters `DSSEv1 `. The signed text starts with the namespace, and a DSSE statement signed with an agent
  key starts with `DSSEv1 ` (the DSSE v1 pre-authentication encoding): the refusal is there so that the signed text
  does not start as such a statement does. With no line feed in the namespace, the signed text does not start with
  an HTTP method and a line feed either, as the text of an `agent-request-v1` signature does.
- The message names the field, then `the request was not sent`, then a `Fix:` line: `Fix: Pass a namespace and an
  operation that contain no line feed, carriage return or vertical bar (|).`, or for the prefix `Fix: Pass a
  namespace that does not begin with "DSSEv1 ".`
- Run on the previous code, each of the 15 refused cases of the new tests was signed and sent: the six of field and
  character, four namespaces beginning with `DSSEv1 `, and five beginning with `GET`, `POST`, `PUT`, `PATCH` or
  `DELETE` and a line feed. A namespace that holds `DSSEv1` without the space, in lower case, or after its first
  character is signed and sent as before.
- Tests: `src/secrets/SecretsClient.resolve-refusals.test.ts`.

### Added: an `agent-request-v2` signer, which the package does not export, checked against the conformance vectors

- `src/crypto/agent-request-v2.ts` signs a request as an `agent-request-v2` statement and returns the two headers
  that carry it, `X-Agent-Request` and `X-Agent-Request-Signature`. The statement names the method, the path and
  query string of the URL, the URL's origin as the audience, the SHA-256 of the body when the body is not empty, the
  agent ID, this host's clock in whole seconds and a random 16-byte nonce drawn for each call; each signature covers
  its DSSE v1 pre-authentication encoding. The Ed25519 key of the credentials signs every statement. Given an ML-DSA
  key, for an agent in hybrid mode, that key signs it too, and `X-Agent-Request-Signature` carries both signatures
  separated by a comma.
- The request and the credentials are checked by one function that `createAgentRequestHeaders` now calls for the
  same checks, so the two signers refuse the same inputs with the same messages. An ML-DSA algorithm other than
  ML-DSA-44, ML-DSA-65 or ML-DSA-87, a key that is not bytes, and a key whose length is not its algorithm's are
  refused as well. Nothing is signed after a refusal. The 34 tests of `createAgentRequestHeaders` pass unchanged.
- The package entry point does not export the signer and no SDK code calls it, so no request the SDK sends changes.
- Tests: `src/crypto/agent-request-v2.test.ts`. Of the eight requests in `docs/specs/agent-request-v2-vectors.json`
  that AIM's verifier accepts, the signer reproduces six byte for byte, the statement and the signature line, ML-DSA-65
  signatures included; the other two are forms it does not write, an ML-DSA-65 signature alone and a statement with
  line breaks and its members in another order. The tests also send signed requests with fetch to a loopback server
  and check each statement against the request received, verifying the Ed25519 signature with `node:crypto`.

### Fixed — ARP log rotation rotates the file a symbolic link points to and counts only rotated copies, and `telemetryLog('0')` is refused (#762)

- `rotateLogIfNeeded` resolves a symbolic link before it measures and renames, so the file the link points to is
  rotated beside itself and the link stays in place. Run on the previous code with `events.jsonl` a link to a
  17-byte file and a 16-byte cap, the rotation renamed the link, the target kept its 17 bytes, and the next append
  created a regular file at `events.jsonl`.
- The prune after a rotation counts and deletes only names of the form `<name>.<13 digits>.<digits>.<8 hex
  digits>`. Run on the previous code with five rotated copies and an `events.jsonl.bak` beside them, one rotation
  left four rotated copies and the `.bak` file.
- `telemetryLog`, exported from `src/arp/cli/telemetry.ts`, refuses a count that is not a positive integer and
  returns 1, as `aim-arp telemetry log 0` already did. Called directly on the previous code with `'0'` on a log of
  25 records, it printed 20 records and returned 0.
- Tests: `src/arp/reporting/log-rotation.test.ts`, `src/arp/cli/telemetry-audit-log-corrupt.test.ts`.

### Fixed — `SecretsClient` encodes its query values, and sends the store and rotate members the server reads (#751)

- `getAuditLog` and `listNamespaces` encode each query value. `getAuditLog` put `since` in the path as passed, so
  `since: '2026-10-16T10:00:00+02:00'` was sent as `since=2026-10-16T10:00:00+02:00`. Run against the backend's
  `GET /api/v1/secrets/audit` handler on the previous code, that query read `since` as
  `2026-10-16T10:00:00 02:00`, which its RFC 3339 parse refuses, and the route answered `200` and listed the agent's
  entries with no time filter. The request now carries `since=2026-10-16T10%3A00%3A00%2B02%3A00`, which the handler
  reads as the timestamp passed and filters by. A `since` in UTC (`Z`) or with a negative offset has no `+` and was
  read as passed before this change too.
- `storeCredential` and `rotateCredential` send `encryptedBlob` and `encryptionAlg`, the members the server's store
  and rotate routes read. The `ephemeralPublicKey` option, which neither route reads, is deprecated and no longer
  sent.
- Tests: `src/secrets/SecretsClient.test.ts` decodes the request path as a form is decoded and gets back each value
  passed, and holds the store and rotate bodies to the members the backend's `storeCredentialRequest` and
  `rotateCredentialRequest` declare. The backend's `TestSDKCallsMatchTheRoutesTheyCall` no longer allow-lists the
  `ephemeralPublicKey` member of either call, and its allow-list ceiling drops from 30 to 28.

### Fixed — `init` checks the agent ID, organization ID and status the server answers with, and prints server text as one line

- `init` saves and prints the agent ID from the exchange answer only when it is a string of 1 to 128 letters, digits,
  `-` and `_` that starts with a letter or digit, a form that admits the UUID the server sends. Any other agent ID,
  a value that is not a string included, ends the command with exit status 1 and nothing saved, and the message says
  that the agent may exist on the server and is to be removed there before running the command again with a new
  token. An organization ID outside the same form is saved as `null`, and a status that is not 1 to 32 lowercase
  letters, `-` and `_` starting with a letter is saved and printed as `unknown`.
- The server's error message and a redirect's `Location` are printed as one line: each C0 character (newline,
  carriage return and tab included), DEL, each C1 character, the bidirectional formatting characters U+061C, U+200E,
  U+200F, U+202A-U+202E and U+2066-U+2069, and a lone surrogate are replaced with U+FFFD, and text over 500
  characters is cut with ` [truncated]`. The dashboard address printed after registration passes through the same
  replacement. The replacement is `sanitizeTerminalLine` in `src/arp/telemetry/signature/sanitize.ts`;
  `sanitizeTerminalText` is unchanged.
- Tests: `src/cli/init.test.ts`, `src/arp/telemetry/signature/sanitize.test.ts`.

### Fixed — `aim-arp telemetry log` and `status` exit 1 on an audit log they cannot read, `log N` counts records, and `init` names an unexpected argument and why it could not connect

- `aim-arp telemetry log` and `aim-arp telemetry status` exit with status 1 when the audit log exists but cannot be
  read. On a build of the code before this change, with the audit log at mode 000, `log` printed `Could not read the
  audit log (EACCES), so what was sent cannot be shown.`, `status` showed the Sent count as `unknown (audit log could
  not be read: EACCES)`, and both exited 0: `telemetryLog` set `process.exitCode` to 1, and the bin then set it to
  the 0 the dispatch returned. `telemetryLog` and `telemetryStatus` now return the exit code, and the dispatch passes it
  on (#755).
- `aim-arp telemetry log N` shows the last N audit records. It showed the records among the last N lines, so a line
  that is not a record took a record's place: on a log of five records followed by a line that is not JSON and a
  record with an unknown phase, `log 2` printed `2 line(s) could not be read as audit records; showing 0 record(s).`
  and `log 1` printed the same with `1 line(s)`. `log 2` now shows the last two records, after `2 line(s) could not
  be read as audit records; showing 2 record(s).` The count of unreadable lines covers the lines after the oldest
  record shown, or every line of a log that holds fewer than N records. `telemetry status` counts that log as it
  did before: Sent 2, Failed 1, Dropped 1 and Unreadable 2 (#755).
- `readAuditRecords(limit)`, exported from `@opena2a/aim-sdk/arp`, reads the log the same way and returns the last
  `limit` records, as its documentation says. On a log of three records followed by two lines that are not
  records, it returned no record for a `limit` of 1 or 2 and one record for 3; it now returns one, two and three.
  `readAuditRecords(0)` returns no records; on that log it returned all three before this change (#755).
- `aim-arp init` names an unexpected positional argument: `aim-arp init --url http://127.0.0.1:9 extra-positional`
  printed `Error: Unknown option: a positional argument` and now prints `Error: unexpected argument:
  extra-positional`. A bootstrap token given as a positional is shown as `aim_ob_[REDACTED]` and an empty one as
  `""`, and the argument is printed as one line through `sanitizeTerminalLine`, cut at 128 characters with
  ` [truncated]` (#755).
- When `init` cannot connect, the reason in parentheses says why. Where `localhost` resolves to `::1` and
  `127.0.0.1`, `aim-arp init --url http://localhost:59321` with nothing listening on that port printed `Error: could
  not reach the AIM server at http://localhost:59321 ().`, and `--url http://localhost:/` printed the same with
  `http://localhost:`: fetch fails there with an `AggregateError` cause whose own message is empty. `init` now
  prints the per-address messages, `(connect ECONNREFUSED ::1:59321; connect ECONNREFUSED 127.0.0.1:59321)`, and
  names a cause with no message and no per-address errors by its error code. The reason passes through
  `sanitizeTerminalLine`, as the server's error message does (#755).
- Tests: `src/arp/cli/telemetry-audit-log-corrupt.test.ts`, `src/cli/init.test.ts`.

### Fixed — `aim-arp -V` prints the version, and `init` refuses an agent name that is a path

- `aim-arp -V` and `aim-arp telemetry -V` print the version and exit 0, as `-v` and `--version` do and as
  `aim-sdk -V` does. On a build of the code before this change, `aim-arp -V` printed `Unknown command: -V` and the
  help and exited 1, and `aim-arp telemetry -V` printed `Unknown telemetry subcommand: -V` and exited 1 (#755).
- `init` refuses an agent name that holds `/` or `\`, or is `.` or `..`, before the token is sent: it prints
  `Error: an agent name cannot hold "/" or "\", or be "." or "..". Nothing was sent.` and exits 1. Against a local
  server answering 201, `aim-arp init --name '../../evil'` sent the name `../../evil`, printed `Agent "../../evil" is
  registered.` and saved the key to `~/.aim/agents/______evil.json`. A name with spaces or dots that is none of these,
  such as `ops agent` or `bot.v2`, is sent as before. `init --help` describes `--name` as an agent name without `/`
  or `\`, and `aim-sdk init` in the Python SDK applies the same rule (#755).
- The README's `init` paragraph states the name rule, in the words the Python SDK's README uses; both READMEs say
  that the token is read from `AIM_BOOTSTRAP_TOKEN` only and a flag that carries it is refused (#755).
- Tests: `src/arp/cli/aim-arp-argv.test.ts`, `src/cli/init.test.ts`.

### Tests

- The signed-field enforcement harness now proves that the verifier compares each signed instant to the clock, not
  only that the signature covers it. Its violations change a field after signing, so the signature check alone
  rejected every one of them: a verifier that ignored `createdAt` or `expiresAt` (the 1.0.2 `expiresAt` incident, and
  #417) still passed. Each signed instant (derived from the signed payload) must now also have a clock violation, a
  validly signed delegation that `verifyDelegation`, `checkDelegationTemporalValidity` and `verifyDelegationChain`
  must reject at the evaluation time: before `createdAt`, after `expiresAt`, and exactly at `expiresAt`. The
  `createdAt` lower bound itself was enforced in #467 (#417).
### Added

- `LocalVerifier.verifyCredential` and `LocalVerifier.authorize` accept the credential as raw JSON text or
  bytes as well as a parsed object. Raw input goes through the strict parse in `@opena2a/atx-verify` before
  any field is read, so a credential carrying a duplicate member at any depth, including two names that
  differ only in case, is rejected as `MALFORMED` with a reason naming the member. A parsed object cannot
  get that check because `JSON.parse` has already dropped one of the duplicates. The two strict-parse ATX
  conformance fixtures now replay through the SDK and are rejected; a new test replays every vendored
  fixture through the raw-text entry.

### Fixed

- The ARP runtime twin's logs under the home directory, `~/.opena2a/arp/events.jsonl` (one line per processed
  event) and `~/.opena2a/arp/l1-escalations.jsonl` (one line per escalation), are capped. A file above 50 MiB at
  the next append is renamed to `<name>.<timestamp>.<pid>.<random>` and the append starts a new file, and the five
  newest renamed copies are kept while older ones are deleted. Run on the code before this change with an
  `events.jsonl` of 50 MiB plus one byte, one processed event appended its line to that file and renamed nothing,
  and with six renamed copies beside it, all six stayed. Test:
  `src/arp/intelligence/runtime-twin.log-rotation.test.ts` (#756).

- The ARP process monitor (`ProcessMonitor`, which `AgentRuntimeProtection` starts unless
  `monitors.process.enabled` is `false`) reads the process table with one call per poll,
  `ps -ax -o pid=,ppid=,user=,%cpu=,%mem=,command=` run without a shell, and takes each child's command, user, CPU
  and memory from that listing, with no `ps` call per process. The row of that `ps` itself, a child of the agent
  while it prints, is left out by its pid, parent and command line, so it is never reported as a new or terminated
  child process. In 1.3.1 it was reported as `Child process terminated: PID <n>`, a pid no spawn event had named. A
  `Child process terminated` event is emitted only for a child that was running at `start()` or had a
  `New child process` event, and a poll whose `ps` call fails emits no event. A child that starts and exits between
  two polls still produces no event. Test: `tests/arp/monitors/process-monitor.test.ts`.

- `SecretsClient.storeCredential` and `SecretsClient.rotateCredential` send the algorithm as `encryptionAlg`, the
  member `POST /api/v1/secrets/namespaces/{id}/credentials` and `POST /api/v1/secrets/namespaces/{id}/rotate` read,
  so the server is given the algorithm passed in the `encryptionAlgorithm` option (default
  `X25519-ChaCha20-Poly1305`). They sent it as `encryptionAlgorithm`, a member those routes do not read. The option
  keeps its name.

- `npx @opena2a/aim-sdk init` sends the bootstrap token over `http:` only when the server address's host is
  `localhost`, `127.0.0.1` or `[::1]` as `new URL()` reads it, so `http://127.1`, `http://0x7f000001` and
  `http://2130706433` count as `127.0.0.1`; the host is never resolved. For an `http:` address on
  any other host, from `--url` or `AIM_URL`, it exits 1 before the exchange request: no request is made, nothing is
  saved and the token is not printed. The error names the host and prints `--url` with the `https:` address of that
  host. `https:` addresses are accepted on any host. `invalidServerUrlReason`, which also checks the dashboard
  address a server reports, is unchanged. Test: `src/cli/init-cleartext-url.test.ts`.

- `AIMClient.verifyAction` posts the action to `POST /api/v1/sdk-api/verifications`, the route AIM serves action
  verification on, in the body that route reads (`agentId`, `capability`, `resource`, `context`, `timestamp`,
  `riskLevel`, `signature`, `publicKey`), signed with the agent's Ed25519 key over the message the server rebuilds
  from that body. It makes no token request. An `approved` or `auto-approved` answer is returned, with the server's
  `status` and `enforcementMode` on the result; `denied` and `pending` throw `ActionDeniedError` with the reason; a
  403 with no decision in its body stays `AuthorizationError`; an answer with no known decision throws
  `VerificationError`. It posted to `/api/v1/verify`, a path the server does not register (#662).
- `reportCapabilities` registers each distinct capability through
  `POST /api/v1/sdk-api/agents/:id/capabilities/register` and returns a `CapabilityReport`: how many were granted or
  already held, how many await approval, and the outcome for each. `reportCapabilities` and `attestIsolation` sign
  their requests as the agent (`X-Agent-ID`, `X-Timestamp`, `X-Signature`, `X-Public-Key`) and send no OAuth token.
  `updateAgent` sends only `displayName`, `description`, `agentType`, `version`, `capabilities`, `talksTo` and
  `metadata`, the fields the server applies; `name` and `communityIntelligenceOptIn` are not sent. A token request
  that cannot reach AIM names `/api/v1/oauth/token`, the path it is sent to (#662).

- `LocalVerifier` verifies every ML-DSA-65 signature a credential declares. `@opena2a/atx-verify` 0.3.0 checks
  Ed25519 only and recorded an ML-DSA-65 entry without checking it, so a credential whose ML-DSA-65 signature was
  forged verified as long as its Ed25519 signature was intact. Each declared ML-DSA-65 signature must now verify, over
  the same canonical payload, against an `ML-DSA-65` key in `publicKeys` that may sign for the credential's issuer;
  with no such key the credential rejects as `SIGNATURE_INVALID`. An issuer that signs with both suites therefore needs
  both of its keys in `publicKeys`. A signature whose algorithm is not `Ed25519` or `ML-DSA-65` now rejects instead
  of being passed over. A v1.1 `issuerChain` DID now lends its DID-URL-bound keys only when it is also in
  `trustedIssuers`, as the Java SDK's verifier does, so a signer can no longer name itself in the chain and sign for
  a trusted issuer. The SDK now returns the expected verdict on every vendored ATX conformance fixture, including
  `v1_1-hybrid-mldsa-tampered.json` and `v1_1-untrusted-chain-authority.json`, which it accepted; the conformance test
  now checks each fixture's verdict and rejection category, not only that raw and parsed input agree.

- `new LocalVerifier` throws `ConfigurationError` when an `ML-DSA-65` entry in `publicKeys` is not 3904 hex
  characters (a 1952-byte raw key). Such a key was dropped silently, so a mistyped key made every hybrid credential
  reject with `no eligible ML-DSA-65 trust anchor`, a reason that pointed at the credential rather than the
  configuration. The error names the key's `keyId` and the length it has (#621).

- `OAuthTokenManager` now obtains a token the AIM server issues. It posted to `/oauth/token`, which the server does
  not serve (404), with `grant_type=client_credentials`, which the token endpoint answers with 400
  `unsupported_grant_type`; its client assertion was padded standard base64 and its signature covered a
  request-signature string rather than the JWT, so the server could not verify it either. The SDK now posts to
  `/api/v1/oauth/token` with the RFC 7523 JWT-bearer grant
  (`grant_type=urn:ietf:params:oauth:grant-type:jwt-bearer`) and a compact JWS: unpadded base64url segments, the
  Ed25519 signature taken over `header.payload` with the agent's key, and `sub`, `aud` and a five-minute `exp`.
  The README's OAuth line describes this grant instead of a client credentials flow.
- `OAuthTokenManager` signs the assertion with the 32-byte seed of the configured private key, so the 64-byte form
  AIM issues and `npx @opena2a/aim-sdk init` writes (the seed, then its public key) obtains a token. Signing with
  those 64 bytes as given threw `Uint8Array expected` from the Ed25519 signer, and no request was sent; only a
  32-byte seed worked. The key is decoded as `createAgentRequestHeaders` decodes it: standard base64 of 32 or 64
  bytes, with the last 32 bytes of a 64-byte key equal to the seed's public key. Any other key throws
  `ConfigurationError` before a request is sent.
- `AIMClient` signs the requests it sends with agent credentials (`X-AIM-Signature`) with the 32-byte seed of the
  configured private key, so `verifyAction` completes with the 64-byte form AIM issues and
  `npx @opena2a/aim-sdk init` writes. With that form the client obtained a token and then threw `Uint8Array expected`
  from the Ed25519 signer, and the request was not sent. The key is decoded as `OAuthTokenManager` decodes it; any
  other key throws `ConfigurationError` before a request is sent.
- The Express middleware and the Fastify plugin accept the agent identity in their options. `createAIMMiddleware`
  and `aimPlugin` take `credentials` (an `AgentCredentials` object) or a ready `client`; the README examples, which
  passed only `{ baseUrl, apiKey }`, answered 401 on every verified route without contacting AIM. Passing both
  options is refused. When the environment holds three of the four `AIM_*` agent variables, the "No credentials
  available" error now names the missing one (for example `AIM_ORGANIZATION_ID`), which reaches the 401 body. A test
  drives both README examples with a real client against a fake AIM server and asserts the action is verified
  there (#449).

- ARP enforcement handlers receive the result of each action after it ran. On an `AgentRuntimeProtection`, a handler
  registered with `arp.onEnforcement` or `arp.getEngine().onEnforcement` is given the same result the ARP writes to
  `enforcement.jsonl` for that decision, and the action runs once per matched rule. `EnforcementResult` carries
  `outcome` (`executed`, `failed`, `no-target` or `not-executed`), and `success` is true only when `outcome` is
  `executed`: a log written to `enforcement.jsonl`, an alert raised with the alert callback, when one is set, returning
  without throwing, or a pause or kill signal delivered by `process.kill` without throwing. `targetPid` is the PID a pause or kill signalled
  or tried to. A pause or kill with no PID still reaches every handler, as `success: false`, `outcome: 'no-target'`,
  `reason: 'No PID to kill'` (or `'No PID to pause'`) and the event, so a handler can contain it another way. An alert
  whose callback throws is `failed`, with a `reason` that names the callback's error. `enforcement.jsonl` lines carry
  `outcome` too. `new EventEngine(config, executor)` takes the executor that carries out each action
  (`EnforcementEngine` is one; the type is `EnforcementExecutor`, exported from `@opena2a/aim-sdk/arp` with
  `EnforcementOutcome`); an `EventEngine` with no executor reports a matched action as `success: false`,
  `outcome: 'not-executed'`. Run against the code before this change, under the default rules, a handler was given
  `success: true` and no `targetPid` for a kill of a live child process, a kill of a PID no process holds, and a kill
  whose event had no PID; a handler on an `EventEngine` with no executor, and one for an alert whose callback threw,
  were given `success: true` as well.

- An `EventEngine` built with a custom executor gives enforcement handlers a result whose `success` agrees with its
  `outcome`. When the executor's result sets `outcome`, the engine sets `success` to `outcome === 'executed'`; when it
  leaves `outcome` unset, the engine sets `outcome` from `success`, as before. Run against the code before this change,
  a result with `success: true` and `outcome` `failed`, `no-target` or `not-executed` reached the handlers with
  `success: true`, and a result with `success: false` and `outcome: 'executed'` reached them with `success: false`.
  The executor's other fields reach the handlers unchanged (#753).

- `pause` and `kill` call `process.kill` only for a PID that is a positive integer. Any other value (absent, 0,
  negative, fractional, or a string) is `outcome: 'no-target'` and no signal is sent. Unchanged and known: a kill
  sends SIGKILL to the same PID 5 s after SIGTERM when that PID is still in use, without checking that it still names
  the process it signalled.

- `aim-arp telemetry log` and `telemetry status` no longer read a corrupt or unreadable audit log as an empty one.
  A log holding only unparseable lines printed "No telemetry has been sent yet" and "Sent: 0"; `log` now reports
  how many lines could not be read and how many records it shows, and `status` adds an `Unreadable:` line beside
  the counts. A log that exists but cannot be read (for example `EACCES`) is reported as unreadable, with `Sent:
  unknown`, and `log` exits 1. A JSON line without the audit record's fields is counted as unreadable instead of
  throwing a raw `TypeError` from the renderer. `readAuditRecords` keeps its return shape; the new `readAuditLog`
  in the signature module returns the records with the unreadable count and read error (#416).
- `A2AClient.getTaskHistory` and `A2AClient.listSkills` now reach routes the AIM server registers. Both requested
  paths the server does not serve (`GET /api/v1/a2a/tasks/{agentId}` and `GET /api/v1/a2a/skills`), so every call
  failed. `getTaskHistory` now lists through `GET /api/v1/a2a/tasks?agentId=...&limit=...` and returns the tasks
  the target agent took part in as client or remote agent, mapping the server's `externalTaskId`, `contextId` and
  `state` to the method's existing `taskId`, `taskType` and `status` fields. `listSkills` now returns the current
  agent's skills from `GET /api/v1/a2a/agents/{agentId}/skills`, the route `getSkills` uses. Both keep their
  return types.
- `aimErrorHandler` answers every SDK error as JSON. It handled only denials and
  authentication failures and passed every other SDK error to `next(error)`, so an
  AIM server answering 500 reached Express's default handler, which outside
  `NODE_ENV=production` rendered an HTML page with the stack trace and local file
  paths and printed the stack to stderr. Those errors now get the body the Fastify
  plugin gives them, `{ statusCode, code, error, message }`, keeping a 4xx or 5xx
  status from the SDK error and using 500 otherwise. Errors that did not come from the
  SDK still go to `next(error)`, and so does any error once the response has started.
  A new test drives the README middleware stack against a fake AIM server on loopback
  (#450).
- An unreachable AIM server with no cached token now rejects with `NetworkError`, naming
  the token endpoint and the errno, as every other request does. The token exchange
  wrapped only a timeout, so the raw `TypeError: fetch failed` from `fetch` reached the
  caller, and `aimErrorHandler` passed it to Express's default handler, which answered
  500 with an HTML page carrying the stack trace (#450).
- The type comment on the ARP `AlertRule.requireLlmConfirmation` says what the flag does. It read "Escalate to L2
  LLM for confirmation before enforcement?". It now says that the rule's action is sent to enforcement as soon as
  the rule matches, with or without the flag; that the flag only writes `_llmReviewRequested`, `_initialAction` and
  `_matchedRule` onto `event.data` and starts no L2 assessment; and that L2, when switched on, assesses an event
  before the rules are evaluated and can change its category and severity. The README's Runtime Protection section
  gains a "Default rules" table with each default rule's name, condition, action and flag. It states that with L2
  off, the default, a critical `threat` ends in `kill` from `critical-threat` at once; that with L2 on, an L2 answer
  of `PAUSE` turns a critical threat into a high violation, which the defaults answer with `alert`; and that `rules`
  in the ARP config replaces the list. It also states that a config from `loadConfig()`, which
  `new AgentRuntimeProtection()` reads when given no argument or a config file path, carries `rules: []`, so ARP
  built that way applies no rule until the config lists some. A test holds the table equal to the rules a
  default-configured `EventEngine` applies.

### Changed

- `A2AClient.updateTrustScore` is deprecated: the server refuses the write with 405
  because the A2A trust score is measured from recorded interactions, not asserted.
  A routed agent's `trustScore` is `null` while its composite is unscored.
- `A2AClient.revokeAttestation` is deprecated. The AIM server registers no route that revokes an attestation (it
  serves only `POST /api/v1/a2a/attestations` and `GET /api/v1/a2a/attestations/{agentId}/{skillId}`), so the method
  posted to a path the server does not serve. It now rejects with an error that says so, without sending a request,
  and never resolves. It is kept so existing code still compiles.

### Security — ARP's L2 no longer sends anything to a vendor by default

ARP shipped `intelligence: { enabled: true, adapter: 'agent-proxy' }` as its
default, and `agent-proxy` chose a destination by reading whichever model key
happened to be exported: `ANTHROPIC_API_KEY`, else `OPENAI_API_KEY`, else a
local Ollama. On any machine with one of those set — a developer laptop, a CI
runner for an agent project — a default-configured ARP therefore sent
qualifying events to that vendor, authenticated with the operator's own key and
billed to their own account. Nothing in the README said so.

Measured against a real install of published 1.3.1, with the HTTP layer stubbed
so no socket is opened and with a synthetic key: one `critical` event under the
shipped default produced an outbound attempt to `api.anthropic.com/v1/messages`,
headers `x-api-key, anthropic-version, content-type`, and the body carried the
value planted in the event verbatim. A `critical` event bypasses batching, and
the network monitor emits `critical` and is on by default, so this is the
default path rather than an edge case.

Affected: **1.0.0 through 1.3.1**, every published version — verified by reading
`dist/arp/index.js` out of each tarball. `arp-guard@0.3.0` carries none of this
code in its own tarball and still runs it: line 17 of `dist/index.js` in
`arp-guard-0.3.0.tgz` requires `hackmyagent/arp`, the package depends on
`hackmyagent >=0.11.0`, and hackmyagent's `exports["./arp"]` re-exports this
engine at the version hackmyagent pins (1.3.1 at hackmyagent 0.33.2), so a fresh
install of `arp-guard@0.3.0` runs the affected engine until hackmyagent moves its
pin.

**What changes**

- `intelligence.enabled` now defaults to off, and every gate that can start L2
  requires an explicit `true`. Absent means off. Previously the gates tested
  `!== false`, so an absent value meant ON — and because config is merged
  shallowly, an operator who set only `intelligence.budgetUsd` replaced the whole
  object and armed the vendor channel. **Narrowing the budget, a hardening
  action, used to switch L2 on.**
- On 1.0.0 through 1.3.1 a non-boolean `intelligence.enabled` (a string such as
  `"false"` or the number `0`) was read as on, both for L2 and for the
  behavioral twin; from this release such a value is refused at construction
  with a `TypeError` (so is a non-boolean `intelligence.runtimeTwin.enabled` or
  `intelligence.guardAnomaly.enabled`, and so is `enabled: no` in an `arp.yaml`,
  which js-yaml 4.3.2 reads as the string `"no"`).
- `intelligence.adapter` has no default and must be named explicitly. There is no
  automatic provider selection any more.
- `createAdapter('agent-proxy')` and `autoDetectAdapter()` now throw. The
  `'agent-proxy'` union member is kept so existing builds still compile; it
  refuses at runtime rather than guessing a destination. **This is the breaking
  change**: code that set `adapter: 'agent-proxy'` explicitly must now choose
  `'ollama'`, `'anthropic'` or `'openai'`.
- When L2 is switched on but no adapter is named, ARP says so once on startup and
  runs without L2. L0 and L1 are unaffected and still gate.
- Even with a remote adapter configured deliberately, `event.data` no longer
  travels raw. It is rendered through an allowlist, and withheld values are
  replaced by a length-and-character-class descriptor. The pattern id and the fact
  of a match still travel, so assessment still works. This closes the sharpest
  case, where check `OL-001` ("API key in output") rendered the matched credential
  into the prompt and posted it to the vendor.

  **Scope of that redaction, stated exactly:** it covers `event.data`. It does
  **not** cover `event.description`, which the prompt still carries verbatim. Most
  descriptions are tool-authored text plus an identifier (a pattern id, a hostname,
  a file path, a tool name), but the process monitor composes descriptions that
  embed up to 100 characters of the child command line. So an operator who
  deliberately configures a remote adapter still sends that much command line to
  their chosen vendor. This is pinned by a test rather than left to be discovered
  (`l2-egress-default.test.ts`, T6) and is tracked for a follow-up; it does not
  affect the default configuration, under which L2 makes no outbound call at all.

**If you were relying on the old behaviour**, set both fields explicitly:

```typescript
intelligence: { enabled: true, adapter: 'ollama' }   // inference stays local
```

**Rotation.** Keys sent as `x-api-key` went to their own intended vendor and are
not an exposure. What may be exposed is material carried in event content:
operators who ran with the AI layer (`aiLayer.prompt.enabled`, opt-in) should
treat credentials their agent surfaced in model output as disclosed to the
configured vendor and rotate those.

### The L2 status line reports what is running, and withheld values are coarser

Two follow-ups to the L2 change above, both found by an independent review of it.

The CLI's `Intelligence:` line restated the L2 gate instead of asking it. The first
version tested `enabled !== false` and kept printing `3-Layer (L0+L1+L2)` after the
default changed; the replacement tested `enabled === true && adapter`, which is still
wrong for `adapter: 'agent-proxy'` — that passes both checks and then throws on
construction, so the line announced a layer that was not running. Neither version had
a test. The line now calls `describeL2Status()`, which settles it by attempting the
construction the coordinator attempts, and reports the reason when L2 is not running.

`<withheld: N chars, classes>` reported an exact length. For a small value that pair
is not a summary, it is the value: `true` rendered as `4 chars, lower` and `false` as
`5 chars, lower`, so a boolean was recoverable, and short enums from a known set the
same way. Lengths are now bucketed (`up to 8 chars`, `9-16`, `17-32`, `33-64`,
`65-128`, `over 128`), and values of 8 characters or fewer carry no class information
at all.

### The egress census asks what leaves the process, not what leaves for the registry

`telemetry-egress-census.test.ts` claimed to pin the whole channel set and did
not: it filtered on `api.oa2a.org`, so the L2 vendor channel was invisible
because it addresses `api.anthropic.com` — and invisible a second time because it
sends via `mod.request(...)` through an aliased module handle the matcher did not
recognise. Widening the host list alone would not have found it.

The census now enumerates every module that transmits off-process, matches
aliased sends, and carries named positive controls so a matcher that stops seeing
a known channel fails instead of reporting clean. Modules that transmit because
the caller asked them to — the AIM client, A2A, OAuth, secrets, CRL fetch, the
ARP proxy forwarder — are listed with the reason rather than filtered out, so
they stay in scope if one ever grows an observational side-channel.

### Fixed — ARP's L2 budget and hourly call limit count the calls in flight

- `BudgetController.reserve(estimatedCostUsd)` counts an L2 call and holds its estimated cost before the adapter is
  called, so calls in flight count against `budgetUsd` and `maxCallsPerHour`. `record(reservation, costUsd, tokens)`
  replaces the estimate with the cost of the answer, `release(reservation)` gives back the slot and the estimate of a
  call that throws, and `budget.json` is saved at each of the three steps; a process that stops while a call is in
  flight leaves that call's estimate in `totalSpentUsd`. A batch takes its reservation when it is sent, not when its
  events are queued. Before, `canAfford()` checked the totals and `record()` counted a call only after it returned.
  Measured with a stub adapter before this change, 50 critical events passed to `analyze()` together made 50 adapter
  calls with `maxCallsPerHour: 20`, whether `budgetUsd` held 10 estimates or 20,000; they now make 10 and 20 calls
  respectively. With `maxCallsPerHour: 2`, a batch queued before two critical events used the hour's two calls was
  sent by `stop()`; it is now not sent. `getStatus().spent` and `totalSpentUsd` in `budget.json` count a call in
  flight at its estimate. `reserve()` refuses an estimate that is NaN or negative, and an answered call is charged
  at its estimate when its cost is NaN, infinite or negative, or when the adapter's `estimateCost` throws on it.
- Breaking change for code that calls `BudgetController` directly: `canAfford()` is removed, and `record()` takes the
  reservation `reserve()` returned as its first argument. Called as `record(costUsd, tokens)`, it throws.

### Fixed — ARP's `getStatus()` counts the L2 calls of the current hour (#798)

- `BudgetController.getStatus()`, which `AgentRuntimeProtection.getStatus().budget` returns, starts `callsThisHour`
  again from zero once more than an hour has passed since the count began, as `reserve()` does. Measured before this
  change: with one call reserved and recorded at 10:00 and the clock at 11:01, the controller that made the call
  reported `callsThisHour: 1`; with a `budget.json` saved 61 minutes earlier with one call in the hour, a new
  controller and `AgentRuntimeProtection.getStatus().budget` reported 1 as well. Each now reports 0, the count a
  `reserve()` at that moment starts from.

### Fixed — ARP applies a batched L2 answer to each event of the batch (#801)

- With `enableBatching` on, the answer to a batched L2 call is read, and each event of the batch receives its own
  assessment through the same step as an event assessed alone: `llmAssessment` is set and `classifiedBy` becomes
  `L2-llm`; KILL sets the event's severity and category to `critical` and `threat`, and PAUSE to `high` and
  `violation`. `analyze()` returns null for an event it queues, as before, and the assessment is written to that
  event when the batch's answer arrives. Measured with a stub adapter before this change, a batch of three `high`
  events flushed by `stop()` was sent as one call and recorded in `budget.json` at its cost, and all three events
  kept `classifiedBy: 'L0-rules'`, their severity and category, and no `llmAssessment`, both when the answer gave
  each event a verdict in the batch format and when it was in the single-event format.
- The batch prompt asks for one line for every event, `EVENT_NUM: ALLOW, ALERT, PAUSE, or KILL — one sentence
  reason`, in place of "Flag only genuinely suspicious items". An event the answer gives none of the four actions,
  because it leaves the event out or answers it with another word such as `APPROVE` or `ALLOW/KILL`, gets the
  deny-safe default a single-event answer without an ACTION line gets: `alert`, `consistent: false`, and no change
  to its severity or category. An event given two actions keeps the stricter. A batch line carries no confidence,
  so a batched assessment has `confidence: 0.5`, and `consistent` is true only for ALLOW. Each event's
  `estimatedCost` is an equal share of the call's cost, and its `tokensUsed` a whole-number share of the call's
  tokens that adds up to them: 200 tokens over three events are 67, 67 and 66.
- A batch the budget refuses, or whose call throws, leaves its events as they were, as before.

### Fixed — ARP queues a batched event only while the L2 budget can pay for its batch (#802)

- With `enableBatching` on, an event that L2 would assess in a batch is queued only when the budget could pay for the
  batch's call at that moment: the call's estimate fits in what is left of `budgetUsd` once the calls in flight are
  counted, and `maxCallsPerHour` has a call left. Otherwise the event is not queued and stays at its L1
  classification. The batch still takes its reservation when it is sent. Measured with a stub adapter before this
  change, with `budgetUsd: 0.001`, less than the stub's estimate for one batch call, 2000 `threat` events of severity
  `high` passed to `analyze()` were all queued and a send timer was set, and `stop()` made no adapter call and
  emptied the queue; now none of them is queued and no timer is set. With `maxCallsPerHour: 2` used by two critical
  events, a `high` event analyzed next was queued, and `stop()`, called once the hour had started again, sent it in
  one batch with an event analyzed in the new hour; now it is not queued, and that batch holds the new hour's event
  alone. An adapter whose `estimateCost` throws on the batch's estimate leaves the event unqueued, and `analyze()`
  resolves to null.
- `BudgetController.canReserve(estimatedCostUsd)` returns whether `reserve()` would grant that estimate now, and
  reserves nothing.

### Fixed — ARP logs a batched event again once its L2 batch is answered (#805)

- With `enableBatching` on, `AgentRuntimeProtection` writes an event queued for an L2 batch to `events.jsonl` when
  `analyze()` returns, before the batch is sent, and writes it again once the batch's answer has been applied to it,
  with its `llmAssessment`, `classifiedBy: 'L2-llm'` and the severity and category the answer gave it.
  `getEvents()`, `LocalLogger.readEvents()` and `LocalLogger.tail()` return one record per event ID: an event logged
  twice is returned as its later record, at the place of its first, a limit counts events rather than lines, and
  each record without an ID is returned. Measured with a stub adapter before this change: an event of severity `high`
  emitted with batching on and flushed by `stop()`, whose batch was answered `1: KILL - exfiltration`, was returned by
  `emit()` with `classifiedBy: 'L2-llm'`, severity `critical` and a `kill` recommendation, while `getEvents()`
  returned it with `classifiedBy: 'L0-rules'`, severity `high` and no `llmAssessment`, and `events.jsonl` held one
  line for it; with batching off, `getEvents()` returned it with `L2-llm`, `critical` and `kill`. A batch the budget
  refuses, or whose call throws, leaves its events logged once, as before.
- `IntelligenceCoordinator.onBatchAssessed(handler)` registers a handler that is called with each event of a batch
  once the batch's answer has been applied to it, in the order the events were queued, whether `stop()` or the end
  of the batch window sent the batch. A batch the budget refuses, or whose call throws, calls no handler, and a
  handler that throws or rejects does not stop the other handlers or events.

### Fixed — ARP's runtime twin clips its noised fleet gradient to an L2 norm of 1.0 (#835)

- With fleet gradients on (`intelligence.runtimeTwin.fleetEnabled`), the runtime twin divides the noised gradient by
  its L2 norm when that norm is above 1.0, in float64, before it posts the gradient to
  `https://api.oa2a.org/api/v1/telemetry/behavioral-gradient`. It clipped the gradient to an L2 norm of 1.0, added
  Gaussian noise with a standard deviation of about 4.84 to each of the nine coordinates, and posted the noised
  vector as it was. Measured on the code before this change, in one run of the new test, all 200 gradients posted by
  200 flushes had an L2 norm above 1.0, from 5.97 to 23.58. #835 reports that the endpoint answers HTTP 400 to a
  gradient whose L2 norm exceeds 1.0 by more than 1e-9; in the new test each posted gradient has an L2 norm of at
  most 1.0 + 1e-9. Clipping after the noise is post-processing, so the (epsilon, delta) guarantee, epsilon 1.0 and
  delta 1e-5, is unchanged.
- The noise's Box-Muller transform takes the logarithm of `1 - Math.random()`, which is in (0, 1], in place of
  `Math.random()`, which is in [0, 1). Measured on the code before this change with `Math.random()` returning 0, the
  nine coordinates of the posted gradient were `null`; they are now finite. Test:
  `src/arp/intelligence/runtime-twin.gradient-norm.test.ts`.

### Fixed — aim-arp telemetry CLI polish (#412)

- `aim-arp telemetry` reports an unreadable OpenA2A home as unknown state (exit 1) instead of printing defaults. `status` used to show "OFF (not turned on)" with an opt-out marker hidden behind a permissions error, and `log` said nothing had been sent. The opt-out marker check now treats a marker it cannot stat as present, so consent that cannot be read counts as refused.
- A filesystem failure (EACCES, EROFS, ENOSPC, ...) names the path and the permissions check instead of printing Node's one-line error. When `opt-out` cannot write its marker it says the opt-out did not take effect and names `OPENA2A_TELEMETRY=off`.
- `<subcommand> --help` prints that subcommand's usage and arguments; `telemetry --no-purge opt-out` names the order that works; `telemetry disclosure` uses the same indent and rule as the other commands; the help says "telemetry disclosure" rather than "install-time disclosure" (nothing prints at install); `telemetry status` names `npx @opena2a/aim-sdk telemetry <subcommand>` for an install whose bin is not on PATH.
- The internal CLI's `register` guard (help and stray arguments never run it) moves into a module with its own test.

### Fixed — 1.3.1 release-test P2 findings

- **Token response shape**: `OAuthTokenManager` now reads both the RFC 6749
  snake_case token response (`access_token`/`token_type`/`expires_in`) and the
  historical camelCase form. Previously an RFC-shaped response produced
  `Authorization: Bearer undefined`. A missing or unparseable expiry no longer
  poisons the token cache into `NaN` (which silently forced a refetch on every
  request); it falls back to a conservative 5 minutes, and a response with no
  access token in either spelling raises `AuthenticationError` instead of
  caching `undefined`.
- **Typed `/oauth/token` errors**: a non-2xx from the token endpoint now
  surfaces as the SDK's typed error family (429 → `RateLimitError`, 401 →
  `AuthenticationError`, others → `AIMError` with `code` and `statusCode`)
  instead of a bare `Error`, and the embedded response-body excerpt is capped
  at 500 characters (a proxy's HTML error page ran to 2095).
- **Retries claim removed**: the README advertised "Automatic Retries:
  Built-in retry logic with exponential backoff", but no retry logic exists in
  the SDK. The Features line now describes what the code does (typed errors,
  `RateLimitError.retryAfter`); retry policy stays with the caller. Pinned by
  `src/readme-claims.test.ts` so claim and code cannot diverge again.
- **`Retry-After` honored**: `RateLimitError.retryAfter` now reflects the
  server's `Retry-After` response header (seconds or HTTP-date) when present,
  falling back to the body field then the 60s default only when absent.
  Previously the header was never read, so `Retry-After: 7` yielded 60.
- **Enforcement telemetry on wire 403**: a 403 from `POST /api/v1/verify` with
  telemetry enabled now produces a correlated deny enforcement record, exactly
  as the 200 `{actionAllowed: false}` path does. Previously the
  `AuthorizationError` propagated before the recording hook could run, so the
  403 path recorded nothing. (The allow-path half of this finding — an allowed
  verify carrying a detection produces a record — was already fixed by the
  post-1.3.1 correlation-joiner rework; it is now pinned by a test.)
- **Network-error context**: a connection failure now names the request URL
  and the underlying errno (e.g. `ECONNREFUSED`), unwrapped from the fetch
  error's cause chain, with an explicit `baseUrl` too — not only when the
  localhost default was in play. Previously an explicit-baseUrl failure read
  just "Network error: fetch failed".
- **Delegation lower bound**: `verifyDelegation`, `verifyDelegationChain` and
  `checkDelegationTemporalValidity` now enforce BOTH bounds of the signed
  window: an evaluation instant before `createdAt` is rejected as not yet
  valid (lower bound inclusive; expiry stays exclusive). Previously
  verify-at-epoch against a delegation created in 2026 returned `true`.
- **GTIN registry destination**: `submitGTINEvent` resolves its registry URL
  through the same chain as every other telemetry transport — explicit
  `registryUrl`, else `OPENA2A_REGISTRY_URL`, else the default — instead of
  hardcoding the production registry, which is how the release test's dummy
  event reached production.
- **Env-credential diagnostics**: `loadCredentialsFromEnv()` still returns
  `null` when any of `AIM_AGENT_ID`, `AIM_PRIVATE_KEY`, `AIM_PUBLIC_KEY`,
  `AIM_ORGANIZATION_ID` is missing, but a PARTIAL environment now warns naming
  the missing variable(s) instead of presenting exactly like a clean
  environment. (The README half of this finding was already fixed by #451.)

### Fixed — 1.3.1 release-test P3 findings

- **The npm package no longer ships the TypeScript source inside its source
  maps.** `sourcemap: true` embedded ~2.6 MB of full source (doc comments
  included) as `sourcesContent` in every published tarball while the README
  promised "source is not shipped in the npm package". Maps still ship for
  readable stack traces; `sourcesContent` is stripped, and a packaging test
  now packs the real tarball and pins the invariant.
- `aim-arp help` and `aim-arp version` (the bare words) now work like
  `--help`/`--version`, and `aim-arp telemetry --version` answers with the
  version instead of "Unknown telemetry subcommand". The internal `arp-guard`
  CLI accepts the bare words too.
- `aim-arp telemetry log abc` is now an error instead of silently showing 20
  records, `log 0` is refused (the count must be a positive integer), and
  extra positional arguments (`log 5 5`, `status extra`) are errors — matching
  the strictness the option side already had.
- Every ARP telemetry request identifies its build: sensor enrollment and
  purge (right-to-delete) requests now send `User-Agent: OpenA2A-ARP/<version>`
  like the signature emitter (they previously sent none, logging as "node"),
  and the causal-denial relay's User-Agent now carries the SDK version.
- The express and fastify `verifyAction`/`verifyRoute` hooks set
  `req.aim.agentId` from the verification result, so handlers behind the hook
  can attribute the request to the verified agent without a separate
  `getAgent()` round trip.
- `AIMClient` refuses a nonsensical `timeout`/`enforcementTimeout` (negative,
  zero, NaN, Infinity) with a `ConfigurationError` at construction, and
  `registerAgent` refuses an empty or non-string `name` before any request is
  made.
- `parseAPIError` caps a server-supplied error message at 512 characters and
  falls back to "Unknown error" when the body's `message` is not a string —
  the message lands in logs and terminals verbatim and was previously passed
  through whole.
- The local correlated-events telemetry log is created `0600` (owner-only),
  matching the credential writer; it was `0644`.
- `LocalVerifier` validates the shape of its trust anchors at construction
  (`trustedIssuers` an array of strings, `publicKeys` an array of key
  objects) and throws `ConfigurationError` instead of storing a mis-shaped
  config that would later fail closed with a misleading denial reason.
- README: documented that `getAgent()` returns `null` and `getTrustScore()`
  returns `0` when no credentials are loaded (no request is made), and added
  the previously undocumented exported clients (`A2AClient`, `SecretsClient`,
  `OAuthTokenManager`, `CrlCache`) to the API reference.

### Fixed — entry points share one class identity

- The entry points (`.`, `/arp`, `/express`, `/fastify`) share one copy of `AIMClient` and the error classes (#334). Each bundled its own copy, so an `ActionDeniedError` raised through `@opena2a/aim-sdk/express` was not `instanceof` the class imported from `@opena2a/aim-sdk`. The build now emits shared chunks in both formats. The ESM and CommonJS builds remain two copies; match on `error.code` where one process loads both.

### Fixed — `engines.node` admits only the Node.js releases on which the CommonJS build loads (#836)

- `package.json` declares `"node": "^20.19.0 || >=22.12.0"` in place of `">=18.0.0"`. The CommonJS build loads
  `@noble/ed25519`, which ships as an ES module only, with `require()`. Measured in `node:<version>` container images
  with 1.3.1 from the registry and with the package packed from the source before the change: npm installed it on
  18.20.8 and 22.11.0 with no `EBADENGINE` warning, and `require('@opena2a/aim-sdk')` and
  `require('@opena2a/aim-sdk/arp')` then failed with `ERR_REQUIRE_ESM`. The packed package did the same on 20.18.3
  and 21.7.3, and loaded on 20.19.0, 22.12.0 and 23.0.0. Packed after the change, it installs with an
  `EBADENGINE Unsupported engine` warning on 18.20.8, 20.18.3, 21.7.3 and 22.11.0, and with none on 20.19.0, 22.12.0
  and 23.0.0. On 18.20.8 the ES module build imports, and `generateKeyPair()` fails with `Cannot read properties of
  undefined (reading 'getRandomValues')`: `@noble/ed25519` reads `globalThis.crypto`, which is undefined in a module
  there. The README's Installation section states the same range.

### Fixed — `signatureTelemetry: { enabled: false }` in the ARP config stops the runtime twin's fleet gradients (#862)

- `AgentRuntimeProtection` passes the config's `signatureTelemetry` section to the runtime twin it builds, through a
  new `signatureTelemetry` option of `RuntimeTwin`. The twin reads `enabled: false` there with the same opt-out
  check it applies to the environment variables and the opt-out marker, when it is constructed and again before it
  sends a gradient. Measured on the code before this change: an `AgentRuntimeProtection` with
  `intelligence.runtimeTwin.fleetEnabled: true` and `signatureTelemetry: { enabled: false }` whose engine received
  300 events posted one gradient to `https://api.oa2a.org/api/v1/telemetry/behavioral-gradient`. After the change it
  posts none, and a `RuntimeTwin` given `fleetEnabled: true` and `signatureTelemetry: { enabled: false }` posts none
  at `shutdown()` after five events. The README's "Turn it off again" section lists this setting among the switches
  that disable fleet behavioral gradients. The setting leaves the twin in place: `getRuntimeTwin()` still returns it.
- Tests: `src/arp/intelligence/runtime-twin.optout.test.ts`, beside the tests of the environment variables and the
  opt-out marker, with a positive control in which an `AgentRuntimeProtection` with no opt-out posts one gradient
  after 300 events.

## [1.3.1] - 2026-09-02

### Added

- `AgentType.DEMO`, the agent type the one-command demo registers (#431).

### Fixed

- **NetworkMonitor ss parser read the columns of `ss -tpn` while running
  `ss -tpn state established`, so the Linux monitor never reported an
  established connection.** Under the `state established` filter ss omits the
  `State` column, shifting every field left by one: the peer address was read
  as the local address and the `users:(...)` process field as the peer, so
  `remotePort` parsed to `NaN` and no reported connection ever carried the
  real destination — and because the garbage list was non-empty,
  `getConnections()` never fell through to the correct `/proc/net/tcp`
  fallback either. The parser now picks the address columns from the header
  line (`State` present or not) instead of a fixed index, and no longer drops
  the non-root shape where ss cannot name the owning process and the data
  lines end at the peer column.

### Known issues

Both reproduce on 1.3.0 and are unchanged by this release; scheduled for 1.3.2.

- The README's Express and Fastify examples answer 401 and never contact AIM unless
  `AIM_AGENT_ID`, `AIM_PRIVATE_KEY`, `AIM_PUBLIC_KEY` and `AIM_ORGANIZATION_ID` are all set;
  the middleware options carry no way to pass credentials
  ([#449](https://github.com/opena2a-org/agent-identity-management/issues/449)).
- `aimErrorHandler` passes an upstream 5xx to Express's default handler, which renders a
  stack trace with local paths outside `NODE_ENV=production`; the Fastify plugin answers the
  same case as JSON ([#450](https://github.com/opena2a-org/agent-identity-management/issues/450)).


## [1.3.0] - 2026-08-24

### Added

- **The telemetry consent CLI now actually ships** as the `aim-arp` bin
  ([#400](https://github.com/opena2a-org/agent-identity-management/issues/400)).
  The install-time disclosure has cited `arp telemetry log` / `status` /
  `opt-out` since 1.0.0, attributing them to the hackmyagent CLI — but no
  installed package registered any such command. The audit-log read path,
  status, opt-out, opt-in and the right-to-delete purge were designed and
  implemented, and unreachable. They are now real:

  ```
  aim-arp telemetry status|log|disclosure|opt-out|opt-in|purge
  npx @opena2a/aim-sdk telemetry <subcommand>   # same bin, no global install
  ```

  The bin is named `aim-arp`, not `arp`: `arp` is a system utility on macOS
  and Linux and an unrelated npm package. Sensor enrollment (`register`) is
  deliberately not part of the shipped surface. A test now walks every command
  citation in the shipped TypeScript sources and fails when one names a command the
  package does not register, so this class cannot silently return.

### Fixed

- **Asking a subcommand for help no longer runs it.** `aim-arp telemetry
  opt-out --help` performed a real opt-out — the marker was written by the
  question — `opt-in --help` removed one, and on a machine that had sent,
  `purge --help` would have fired the right-to-delete from a help query.
  The dispatcher chose what to run from the subcommand name alone and never
  validated the arguments, so `--help` never stopped execution and unknown
  options were silently swallowed. `--help` / `-h` anywhere in the arguments
  now print usage and change nothing, and an unrecognized option is an error
  naming the option instead of a no-op.
  A mistyped subcommand is still reported as the error even when `--help`
  rides along, so scripts keep the typo signal. The internal CLI's
  `register` subcommand had the same defect — its help query would have
  built and sent a real enrollment — and is guarded the same way.

- **`telemetry status` names every environment variable it attributes an
  opt-out to, and only when that variable is what opted you out.**
  `OPENA2A_TELEMETRY_OPTOUT` and `ARP_TELEMETRY_DISABLED` were reported as a
  bare "environment variable" with no clearing instruction, while
  `OPENA2A_TELEMETRY` already got "unset it to clear". All three now name
  the variable in effect, and attribution reads the same strict truthiness
  as the opt-out decision — `OPENA2A_TELEMETRY_OPTOUT=0` alongside a local
  marker names the marker, not a variable whose unsetting would clear
  nothing.

- **`telemetry status` no longer creates identity state.** Reading status on a
  clean machine minted three files — the sensor id, the org id, and the org
  root secret — as a side effect of looking. A read command now reads: identity
  fields report `none yet (created on first send)` until a send mints them.

- **Server-supplied text is sanitized before it can reach your terminal.**
  Registry error and message strings printed by `opt-out`, `purge` and
  enrollment failures are stripped of C0/C1 control bytes (ANSI escapes,
  carriage-return overwrite) and capped at 500 chars with an explicit
  truncation marker — the TypeScript half of the class filed as
  [#384](https://github.com/opena2a-org/agent-identity-management/issues/384).
  The manual-retry `curl` commands also single-quote-escape their interpolated
  values.

- **A registry-assigned sensor id is shape-validated before it is persisted.**
  Enrollment adopted the server's `sensorId` verbatim into a local file that
  `telemetry status` later prints; a hostile registry could place
  terminal-driving bytes there. Ids are now accepted only when they match
  `^[A-Za-z0-9._-]{1,128}$`, and status strips control bytes from anything it
  reads off disk regardless.

- **`opt-out` and `purge` no longer create a sensor identity on a machine that
  never sent.** Building the purge proof minted the identity it was about to
  purge. Both commands now check first and report that there is nothing to
  purge.

- **`@opena2a/aim-sdk/arp` now reports the version npm published.** Its
  `VERSION` export was a second hand-maintained literal that read `0.2.0` from
  the day the module landed and was never bumped, so 1.0.0 through 1.2.0 all
  reported the same ARP version while the root export reported the real one.
  Both subpaths now export one value.

  This constant goes on the wire. The two telemetry channels send it as
  `User-Agent: OpenA2A-ARP/<VERSION>`, which is the only build signal the
  registry records for a submission, so every ARP request it has ever logged
  looks like it came from the same sensor build. If you parse that header,
  expect the package version from this release on and `0.2.0` before it.

## [1.2.0] - 2026-08-21

### Changed — read this before upgrading

**Structural signature telemetry is now OFF by default.** It previously ran
unless you opted out, sending the structural shape of anomalous runtime events
to `https://api.oa2a.org` from inside your process. It now sends nothing until
you turn it on:

- `AIM_TELEMETRY=1` in the environment, or
- `signatureTelemetry: { enabled: true }` in your ARP config.

If you were relying on this channel, it stops on upgrade and you must opt in to
restore it. If you had already opted out, nothing changes for you.

**`OPENA2A_TELEMETRY=off` now works in this SDK.** That is the spelling
[opena2a.org/privacy](https://opena2a.org/privacy) documents and the one it
tells you to put in your shell profile, but no released version of this package
ever read it: 1.0.0 through 1.1.0 read only `OPENA2A_TELEMETRY_OPTOUT` and
`ARP_TELEMETRY_DISABLED`, neither of which appears in the policy. A user who did
exactly what the policy said was still emitting. `off`, `0`, `false` and `no`
are accepted, matching the CLIs. It is honored in the off direction only —
`OPENA2A_TELEMETRY=on` does not turn the channel on, because an ecosystem-wide
CLI setting should not start a network channel inside a library embedded in your
process.

An opt-out still beats an opt-in, so `OPENA2A_TELEMETRY_OPTOUT=1`,
`ARP_TELEMETRY_DISABLED=1`, `signatureTelemetry.enabled: false` and the
`~/.opena2a/telemetry-optout` marker all continue to work and now take
precedence over either opt-in above. Clearing an opt-out no longer starts the
channel by itself — refusing and never choosing are now distinct states, and
only an explicit opt-in starts it.

This SDK ships as a library inside other people's production processes, and
AIM's claim is that identity and audit stay on disk. A channel that phones home
unless the operator finds the switch is inconsistent with both.

### Fixed

- **The default config no longer enables the channel.** `defaultConfig()`
  carried `signatureTelemetry: { enabled: true }`, so every `AgentRuntimeProtection`
  built from defaults — which is every one that does not pass an explicit config
  — turned the channel on. That value is now absent rather than `false`:
  an explicit `false` is an opt-out, and because an opt-out beats an opt-in it
  would have made `AIM_TELEMETRY=1` silently do nothing.
- **`arp telemetry status` no longer reports "OFF (opted out)" when nobody opted
  out.** Off-because-refused and off-because-never-enabled are different states
  and only one of them has a reason to report; the status output and
  `arp telemetry opt-in` now distinguish them.
- The first-run disclosure text no longer says "ON by default" and now names how
  to turn the channel on as well as off.

- **A server-side denial is now reported as a denial, not as an opaque 500.**
  `parseAPIError` maps HTTP 403 to `AuthorizationError`, but the express
  middleware, the fastify preHandler, fastify's `verifyRoute` and
  `aimErrorHandler` each checked only for `ActionDeniedError` and
  `AuthenticationError`. An `AuthorizationError` matched no branch and fell
  through to `next(error)` / `throw`, so the ordinary case of AIM refusing an
  action over the wire surfaced to the caller with no status and no denial
  reason. All four now answer `403 Action denied`.

  Verification has always been fail-closed in these integrations -- a failed
  verification never called `next()` and never reached the route handler -- so
  this is a reporting defect, not a bypass. The Python SDK's fail-open denial
  defect has no analogue here.

### Changed

- `verifyRoute` (fastify) now delegates to `verifyAction` instead of repeating
  its body. Two copies of the same catch ladder in one file is how they came to
  disagree; express's `verifyRouter` already delegated this way.
- The error-to-response mapping lives in one place,
  `src/integrations/verification-outcome.ts`, so the integrations agree by
  construction rather than by three files being edited together.

## [1.1.0] - 2026-07-16

### Security

- **Delegation chains now enforce temporal narrowing: a child may not outlive its
  parent.** `verifyDelegationChain` rejects any chain in which a child's
  `expiresAt` is later than its parent's, evaluated independently of the current
  time (the per-hop expiry check added in 1.0.3 already fails the chain once a
  parent has actually expired; this closes the window before that and covers
  partial / as-of verification). This completes the delegation-expiry hardening
  from 1.0.3.

### Added

- `createDelegation` accepts an optional `parentExpiresAt`. When creating a
  sub-delegation, pass the parent's `expiresAt`: the child's default expiry is
  capped at the parent's, and an explicit `expiresAt` beyond the parent's is
  rejected (fails closed on an unparseable value). This prevents the default
  seven-day expiry from silently producing a child that outlives its parent.

### Note

- Stricter verification: a chain whose child outlives its parent — previously
  accepted — is now rejected. Chains built with `createDelegation` (using
  `parentExpiresAt` for sub-delegations) are unaffected. Cross-engine parity for
  this rule (Go / Java verifiers, kanoniv interop spec) is tracked as a follow-up;
  the TypeScript verifier being stricter is fail-closed-safe in the interim.

## [1.0.3] - 2026-07-15

### Security

- **Delegation verification now enforces temporal validity.** `verifyDelegation`
  and `verifyDelegationChain` previously checked signature, delegator identity and
  scope narrowing but never evaluated the signed `createdAt`/`expiresAt` window, so
  an expired delegation — and a child that outlived its parent — still verified as
  valid. Verification now rejects delegations that are expired, have an unparseable
  or missing timestamp, or have an inverted window (`createdAt` after `expiresAt`),
  and fails closed in every one of those cases. `verifyDelegationChain` evaluates
  every hop against a single shared instant.

### Added

- `verifyDelegation(delegation, { verifyAt })` and
  `verifyDelegationChain(chain, { verifyAt })` accept an explicit evaluation time
  (a `Date` or ISO-8601 string) for deterministic tests and offline / as-of
  verification. Defaults to the current time.
- `checkDelegationTemporalValidity(delegation, verifyAt?)` — the standalone
  temporal check, exported for callers that want the reason a delegation is
  temporally invalid.
- `verifyDelegationSignature(delegation)` — the raw Ed25519 signature check with
  no temporal evaluation, for archival/audit inspection where authenticity matters
  independent of time. `verifyDelegation` now composes signature + temporal checks.
- `DelegationChainResult` gains a `temporalValid` field.

### Notes

- The production ATX/ATC credential verifiers (Go backend, `LocalVerifier`, the
  Java SDK) already enforce credential expiry and are unaffected. This change
  concerns the cross-engine delegation-chain primitive in this package.
- Reported privately by Tymofii Pidlisnyi (Agent Passport System,
  agent-passport.org).
