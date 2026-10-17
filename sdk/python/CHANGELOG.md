# Changelog

All notable changes to the AIM Python SDK will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed — `aim-sdk demo` names no enforcement mode the server has not reported, and says in one sentence that it cannot reach the server

- `aim-sdk demo` sends one GET to the server address before it prints anything about the server. When that request
  gets no HTTP answer, the demo prints `Could not register the demo agent: ` followed by the SDK's sentence for the
  failure, the one `aim-sdk init` prints for the same address, and exits 1. Measured with a fresh HOME and a stored
  sign-in naming `http://127.0.0.1:9`: `aim-sdk demo --ci` printed its banner with `Strict Mode: DISABLED - Actions
  are logged but not blocked` before any request, then `Warning: Token refresh failed:
  HTTPConnectionPool(host='127.0.0.1', port=9): Max retries exceeded with url: /api/v1/auth/refresh (Caused by NewConnectionError(...))`,
  then `Could not register the demo agent: Cannot register new agent 'demo-agent': OAuth access token failed.` and
  a line saying the token had expired or been revoked. It now prints only `Could not register the demo agent: Could
  not reach AIM at 127.0.0.1:9: connection refused (ConnectionError). Check that the AIM server is running and that
  the aim_url http://127.0.0.1:9 is correct and reachable from this host.` (#755)
- The banner's mode line reads `Enforcement: strict, forced by AIM_STRICT_MODE` when `AIM_STRICT_MODE` forces strict
  mode and `Enforcement: your organization's enforcement mode, set on the dashboard` otherwise. It read `Strict Mode:
  DISABLED - Actions are logged but not blocked` whenever the variable did not force strict mode, a mode the
  organization sets and the demo does not know. The interactive menu and its agent status, JIT and CBAC screens
  print the same `Enforcement` text, and the CBAC summary says that the server allowed the actions under the
  organization's enforcement mode (#755).
- When the refresh request raises a `requests` exception, the text after `Warning: Token refresh failed: ` is the
  SDK's rendering of that exception: for a refused connection, `Could not reach AIM at 127.0.0.1:9: connection
  refused (ConnectionError). Check that ...`, where it was the exception's own text, urllib3's `HTTPConnectionPool`
  chain (#755).
- Tests: `tests/test_demo_command.py`, `tests/test_refresh_cross_process_lock.py`. The demo tests that do not test
  the first request stand in for a server that answers it.

### Fixed — `aim-sdk status` reports a credentials file it cannot read, and lists the agents saved on this machine

- When no sign-in can be loaded and `~/.aim/sdk_credentials.json` or `~/.aim/credentials.json` exists but cannot be
  read as a JSON object, `aim-sdk status --json` answers `"tokenState": "unreadable"` with `credentialsPath` naming
  that file, and the human output adds `<path> exists but could not be read: <why>.`, where `<why>` is `it is not
  JSON`, `it holds JSON that is not an object`, or the operating system's reason, `Permission denied` for a file its
  owner cannot read. Measured with `~/.aim/credentials.json` holding `this is not json {{{`: `status --json`
  answered `"tokenState": "absent"`, and `status` printed `Not authenticated.` and nothing about the file. The exit
  code stays 1 and `--json` keeps its five members (#755).
- `aim-sdk status` lists the agents saved under `~/.aim/agents` after its verdict, one line each with the name and
  agent ID, in name order. When the verdict is not authenticated it adds `The verdict above is about the SDK sign-in
  that aim-sdk login stores; aim-sdk init saves an agent and no sign-in.` Measured right after a successful
  `aim-sdk init` against a local server answering 201: `status` printed `Not authenticated.` and `Run 'aim-sdk login'
  to authenticate`, exited 1, and said nothing about the agent. With no agent saved the output is unchanged, and
  `--json` is unchanged (#755).
- Tests: `tests/test_cli_status_verdict.py`.

### Changed — `aim-sdk init` refuses `--token` as `npx @opena2a/aim-sdk init` does, refuses an agent name that is a path, and says whether the token was used

- `aim-sdk init` reads the bootstrap token from `AIM_BOOTSTRAP_TOKEN` only. `--token` is refused before any
  request, also when the variable is set, in the forms `--token <value>`, `--token=<value>` and an abbreviation such
  as `--tok`: the command prints `Error: the bootstrap token is read from AIM_BOOTSTRAP_TOKEN, not from a flag (a
  flag value is visible in the process list and shell history).` and the command with the variable, exits 1, and
  does not print the value. `init --help` no longer lists the flag and says the token is read from
  `AIM_BOOTSTRAP_TOKEN` only. Measured before: `aim-sdk init --token <value> --url http://127.0.0.1:9` used the
  value and sent the exchange request. No release has carried `aim-sdk init`; the README now says what the
  TypeScript SDK's README says (#755).
- An agent name that holds `/` or `\`, or is `.` or `..`, is refused before the token is sent: `Error: an agent name
  cannot hold '/' or '\', or be '.' or '..'. Nothing was sent.`, exit 1. Measured against a local server answering
  201: `aim-sdk init --name '../../evil-py'` sent the name `../../evil-py`, saved the key to
  `~/.aim/agents/______evil-py.json` and printed `agent = secure('../../evil-py')`. A name with spaces or dots that
  is none of these, such as `ops agent` or `bot.v2`, is sent as before. `npx @opena2a/aim-sdk init` applies the same
  rule (#755).
- When the exchange request raises instead of answering, a second line says what became of the token: `The request
  was not sent, so the token has not been used.` when `requests` raised `ConnectTimeout`, or a `ConnectionError`
  whose cause is urllib3's `NewConnectionError`, as for a refused connection; otherwise `The server may have
  received the request before it failed, so the token may have been used. If the agent is listed in the dashboard,
  delete it, then run aim-sdk init again with a new token.`, as for a connection the server closed after reading
  the request. Measured before on a closed port: the output ended with the `Could not reach AIM` sentence and said
  nothing about the token (#755).
- Tests: `tests/test_cli_init_bootstrap_token.py`. The helpers of `tests/test_cli_init_bootstrap_token.py` and
  `tests/test_cli_init_cleartext_url.py`, and one test of `tests/test_cli_login_cleartext_url.py`, passed the token
  as `--token` would; they now set `AIM_BOOTSTRAP_TOKEN`, and their assertions are unchanged.

### Fixed — `aim-sdk logout` removes `sdk_credentials.json.lock` with the credentials it guards

- `aim-sdk logout` removes `~/.aim/sdk_credentials.json.lock` after the credentials, and removes one found without
  credentials, where it prints `No credentials to clear`. It takes the lock without waiting and removes the file
  while it holds the lock, and only when no credentials file exists by then; the file stays while another process
  holds the lock. Measured before, with a fresh HOME: after a token refresh against an unreachable server had
  created the lock file, `logout` deleted `sdk_credentials.json` and left the 0-byte lock file (#755).
- A process of this SDK that opened the lock file before `logout` removed it, and then took the lock, lets that lock
  go and opens the path again. The Java SDK locks the same file and makes no such check (#755).
- Tests: `tests/test_cli_logout_revocation.py`, `tests/test_refresh_cross_process_lock.py`.

### Fixed: a refused refresh of a downloaded SDK's credential names the download's maximum age, and the token is presented to the refresh route only

- The server ends a downloaded SDK's credential `SDK_TOKEN_FAMILY_MAX_AGE` after the download (365 days by default),
  however often it is refreshed, and answers that refresh `401` `{"error":"Invalid or expired refresh token"}`. The
  `SDK REFRESH TOKEN REJECTED` banner printed for a downloaded credential now lists that cause: "The SDK download
  reached the server's maximum age (SDK_TOKEN_FAMILY_MAX_AGE, 365 days by default). Refreshing does not extend it; a
  fresh download starts a new one." Its opening no longer says the local token is "no longer the latest", which did
  not hold for a revoked or expired token. The banner for an `aim-sdk login` credential is unchanged.
- A refresh the server answers `Invalid or expired refresh token` or `Token has been revoked or is invalid` is no
  longer followed by `Token was revoked - attempting automatic recovery...` and a `POST /api/v1/auth/sdk/recover`
  carrying the refused token. The SDK sent that request without an `Authorization` header, and the route's auth
  middleware reads the session from that header only; the route also mints only for an access token that records a
  sign-in, which the SDK does not hold. Unless errors are suppressed, the banner is all the refusal prints, and the
  security log records one `TOKEN_EXPIRED` event, error `Refresh token refused`, where a `TOKEN_REVOKED` event came
  first.
- Test: in `tests/test_refresh_rejected_recovery.py`, the download banner names the maximum age and the default it
  cites (`8760h` and `365 * 24 * time.Hour` in the backend's `jwt.go`); the login banner does not name it; the
  server's maximum-age refusal body is `Invalid or expired refresh token`; and for both refusal bodies the token is
  posted once, to `/api/v1/auth/refresh`, one `TOKEN_EXPIRED` event is logged and the output is the banner alone.

### Changed — `aim-sdk login` accepts an `http:` address only on this machine, unless `--allow-http` is passed

- `aim-sdk login` applies the rule `aim-sdk init` applies, from one shared check: an `http:` server address is
  accepted when its host is `localhost`, `127.0.0.1` or `[::1]`, compared as written in the URL and never resolved.
  For an `http:` address on any other host it exits 1 before the reachability probe: no request is made and nothing
  is saved. The error names the host, never a user:password part of the address, and prints `aim-sdk login --url`
  with the `https:` address of that host and port, the same command with `--allow-http`, and the deployment guide's
  section on a TLS-terminating proxy.
- The new `--allow-http` flag accepts such an address. Before the first request the command then prints one line
  naming the address and saying that the access and refresh tokens the sign-in receives cross the network
  unencrypted. With an `https:` address, or an `http:` address on this machine, the flag changes nothing and
  nothing more is printed.
- `aim-sdk login` also exits 1 before any request, with or without `--allow-http`, when `--url`, once the whitespace
  around it is trimmed, has a backslash, whitespace or a control character, as `aim-sdk init` does.
- The `https:` example in `aim-sdk init`'s refusal of an `http:` address keeps the port the address names; its
  other text, its exit code and the requests it makes are unchanged.
- `aim-sdk login --help` names the rule and `--allow-http`, and its exit code 1 also covers an address refused
  before any request.
- Earlier releases sent the sign-in's token exchange and received the token pair over the scheme the address named,
  with no refusal and no warning.
- Test: `tests/test_cli_login_cleartext_url.py`; `tests/test_cli_init_cleartext_url.py` passes unchanged.

### Fixed — `aim-sdk login` opens no browser when its output is not a terminal, and prints the page and the code before it waits

- `aim-sdk login` opens a browser on the approval page only when standard output is a terminal and the new
  `--no-browser` flag is not passed. Otherwise it prints one more line after the page and the code: `No browser was
  opened (standard output is not a terminal): open the page above and approve the code. This command waits up to
  <n> seconds for the approval and exits 1 if the code is not approved in that time.`, where the reason reads
  `--no-browser was passed` for the flag and `<n>` is the lesser of the code's lifetime the server states and 900.
  Before, it handed the page to `webbrowser.open` with a terminal and without one.
- The page, the code and the waiting line are flushed to standard output before the first token poll. Against a
  loopback server that answers every poll `authorization_pending`,
  `aim-sdk login --url http://127.0.0.1:<port> </dev/null | cat` printed nothing in 20 seconds while the server
  received the device-code request and three token polls; it now prints the page, the code and both lines at once.
- A login without a terminal still waits for the approval: the wait stays bounded as before, and the exit codes 0, 1,
  2 and 75 are unchanged. `aim-sdk login --help` names `--no-browser`, the terminal rule and the maximum wait.
- Test: `tests/test_cli_login_no_tty_no_browser.py`. The login tests in `tests/test_cli_device_login.py`,
  `tests/test_cli_login_server_unavailable.py` and `tests/test_release_202_p2_findings.py` run with an output that
  reports a terminal, so their browser assertions still test the device grant and the outage paths.

### Fixed — the README's Requirements name PyJWT and the optional rich extra (#627)

- The Requirements section of `README.md` (the PyPI description) listed `requests`, `pynacl`, `cryptography` and
  `keyring`, and said all install via `pip install aim-sdk`. It now also names `PyJWT`, which setup.py's
  `install_requires` declares, and `rich`, marked optional, which the Quick start's `"aim-sdk[rich]"` installs and
  `pip install aim-sdk` does not.
- `pip install aim-sdk` stays the SDK's install line. The Quick start keeps `"aim-sdk[rich]"` for compact console
  output, as its install sentence says.
- Test: `tests/test_sdk_install_line.py` holds the Requirements to setup.py, and holds the root README's SDKs table,
  `docs/use-cases/register-my-agent.md`, `docs/sdk/python.md`, the Requirements and the dashboard's SDK page to
  `pip install aim-sdk`.

### Fixed — `report_capabilities` reads a 409 whose body is not JSON as an answer without a known outcome (#612)

- The capability registration route answers a repeat with 409 and a JSON body that names the outcome. A 409 whose
  body is not JSON now raises `VerificationError: AIM answered the registration of capability '<type>' without a
  known outcome (status None)`, the error a JSON answer without a known `status` raises, and the capabilities after
  it are not sent. With requests 2.34.2 it raised `VerificationError: Request failed: Expecting value: line 1
  column 1 (char 0)`, which names neither the status nor the capability. `_make_request` returns `None` for a
  status in `accept_status` whose body is not JSON, and leaves the outcome check to the caller.
- Test: `test_a_409_whose_body_is_not_json_raises_without_a_known_outcome` in
  `tests/test_report_capabilities_register_route.py`.

### Fixed — the agents directory test reads the mode the directory is created with (#612)

- `test_agents_dir_is_created_0700` records the directory's mode as `Path.mkdir` returns, before the `chmod` in
  `_ensure_private_dir` that narrows a directory an earlier version left wide. With `mode=0o700` taken out of that
  `mkdir` call, the test fails (the directory is created 0755 under umask 022); before, it passed, because the
  `chmod` narrowed the directory before the test read it.

### Changed — `client.py` and `credentials.py` pass ruff's F, E4, E7 and E9 rules, and a test runs ruff on them (#612)

- `ruff check --isolated --select E4,E7,E9,F aim_sdk/client.py aim_sdk/credentials.py` reported 31 findings with
  ruff 0.17.1 (29 in `client.py`, 2 in `credentials.py`) and reports none now. The imports `aim_sdk.client` did
  not use are removed (`hashlib`, `os`, `pathlib`, `VerifyKey`, `AuthnEventType`, `SecurityEventType`,
  `EventSeverity`, `get_agent_credentials_path`, `list_agent_credentials` and `AGENTS_DIR`); code that imported
  one of them from `aim_sdk.client` has to import it from where it is defined (`aim_sdk.credentials`,
  `aim_sdk.security_logging`, `nacl.signing` or the standard library). The imports that sat
  below module code are at the top of the module. The three bare `except:` clauses around reading an error body
  catch `Exception`, so a `KeyboardInterrupt` or `SystemExit` raised there propagates. `credentials.py` drops its
  unused `shutil` and `datetime` imports.
- `tests/test_lint_ruff.py` runs that command and is skipped when ruff is not installed; the `dev` extra and
  `requirements-dev.txt` list `ruff>=0.5.0`. The rules are named in the test, so the installed ruff's default rule
  set does not decide what is checked: ruff 0.17.1 with no rule selection reports 375 findings in the two files.

### Fixed — SDK requests carry only the members their routes read (#623)

- `attest_isolation()` sends `sandbox`, `network`, `filesystem` and `process`, the members AIM reads. It also
  sent `signature` and `signedMessage`, which AIM does not read; the request is authenticated by the agent's
  signature headers, as before.
- Automatic MCP registration (`secure(..., mcp_servers=[...])`) no longer sends `verificationMethod`, a member
  AIM's MCP server creation does not read.
- When the saved sign-in yields no token, the `AuthenticationError` of `register_mcp()` says so and names
  `aim-sdk login`. It said the client held an API key or the agent's signing keys only.
- The tag sync of `secure()` and `register_agent()` reads `urn:uuid:<id>` (in either letter case) and `{<id>}` as
  tag IDs, as AIM does. It treated them as tag names, did not send them and warned that a name was not sent.
- The MCP registration tutorial and `docs/MCP_INTEGRATION.md` describe `register_mcp()` as it is: it takes the
  server's ID or name, returns `added_servers`, and does not attest the server.

### Added — `get_trust_score()` and `get_trust_score_history()`, and the SDK pages name only what the SDK has (#666)

- `AIMClient.get_trust_score(agent_id=None)` reads `GET /api/v1/agents/{id}/trust-score` and returns the
  server's answer: `agentId`, `agentName`, `score`, `factors`, `calculatedAt`, and `excludedFactors` when one or
  more factors were excluded from the score.
- `AIMClient.get_trust_score_history(agent_id=None, limit=None)` reads
  `GET /api/v1/agents/{id}/trust-score/history`, newest first; without `limit` the server returns up to 30
  entries. A `limit` that is not a positive integer raises `ConfigurationError`.
- `MCPActionWrapper` and `log_mcp_action_result` can be imported from `aim_sdk.integrations.mcp`, as
  `sdk/python/docs/MCP_INTEGRATION.md` shows. They were defined in `aim_sdk.integrations.mcp.verification`
  and not exported by the package.
- The Python SDK pages (`docs/sdk/`, `docs/guides/A2A.md`, `docs/guides/PQC.md`, `docs/integrations/`,
  `docs/troubleshooting/`, `docs/DOCUMENTATION_INDEX.md` and `sdk/python/docs/MCP_INTEGRATION.md`) no longer
  document methods, functions, modules, attributes or keyword arguments the SDK does not have, among them
  `get_audit_logs`, `export_compliance_report`, `log_action`, `submit_feedback`, `rotate_key`,
  `create_attestation`, `update_agent_pqc_key`, `sign_request`, `AIMClient.from_sdk_token`,
  `aim_sdk.crypto.generate_keypair`, `sign`, `verify`, `multisig_*` and `HSMSigner`, `auto_detect_mcp_servers`,
  `register_all_mcp_servers`, `load_config`, `aim_sdk.detection.detect_*_mcp_servers` and
  `health_check_server`, `aim_sdk.validation`, `ApprovalTimeoutError`, the `*_b64` key attributes,
  `get_trust_score(detailed=True)`, `agent.id`, `agent.private_key` and `AIMCallbackHandler(aim_agent=...)`.
  Each page shows the SDK call that does the job instead. Audit logs, compliance reports, user feedback and
  webhooks are served on routes that read a user's identity, and the pages point to those REST routes and the
  dashboard.
- The trust score breakdowns in `docs/sdk/python.md` and `docs/sdk/trust-scoring.md` print the factors under the
  keys the server's answer gives them (`verificationStatus`, `successRate`, ...), where they used the snake_case
  names, and the weighted report in `trust-scoring.md` looks the weights up by those keys.
  `TestTrustScoreDocs_PagesQuoteTheNineFactorScore` checks the values under the same keys.
- `tests/test_documented_names_exist.py` checks every python block on those pages against the package: each
  `aim_sdk` import resolves, each attribute read on an `AIMClient` exists, and each keyword passed to an SDK
  callable is one of its parameters.

### Changed — `aim-sdk login` names where an administrator turns strict mode on (#681)

- The closing text of `aim-sdk login` ends by saying that an administrator turns strict mode on with Global
  Enforcement Mode on the dashboard page `/dashboard/admin/security-policies`. It said that calls outside the granted
  capabilities are denied in strict enforcement mode and run in monitoring mode, the default, and did not say where
  strict mode is set.
- Test: `tests/test_cli_device_login.py`.

### Added — `aim-sdk init` registers a first agent from a bootstrap token

- `aim-sdk init` registers a first agent with a bootstrap token from the dashboard's onboarding screen, read from
  `AIM_BOOTSTRAP_TOKEN` or `--token`. It generates the Ed25519 keypair locally and sends only the public key to
  `POST /api/v1/onboarding/bootstrap-tokens/exchange`, saves the credentials to `~/.aim/agents/<name>.json` with mode
  0600, and prints the agent's dashboard URL. `--url` targets a self-hosted server and `--name` replaces the default
  `my-first-agent`. The token travels only in the `X-AIM-Bootstrap-Token` header, redirects are not followed, and the
  token is never printed. An existing credentials file for the name is kept and the token is not spent.

### Fixed — `aim-sdk init` checks the agent ID, organization ID and status the server answers with, and prints server text as one line

- `aim-sdk init` saves and prints the agent ID from the exchange answer only when it is a string of 1 to 128 letters,
  digits, `-` and `_` that starts with a letter or digit, a form that admits the UUID the server sends. Any other
  agent ID, a value that is not a string included, ends the command with exit status 1 and nothing saved, and the
  message says that the agent may exist on the server and is to be deleted there before running the command again
  with a new token. An organization ID outside the same form is saved as `null`, and a status that is not 1 to 32
  lowercase letters, `-` and `_` starting with a letter is saved and printed as `unknown`.
- The server's error message and a redirect's `Location` are printed as one line: each C0 character (newline,
  carriage return and tab included), DEL, each C1 character, the bidirectional formatting characters U+061C, U+200E,
  U+200F, U+202A-U+202E and U+2066-U+2069, and a lone surrogate are replaced with U+FFFD, and text over 500
  characters is cut with ` [truncated]`. The `Name:` line prints the `--name` given, not the name in the answer.
- Test: `tests/test_cli_init_bootstrap_token.py`.

### Fixed — `aim-sdk init` sends the bootstrap token over `http:` only to a loopback host

- `aim-sdk init` sends the bootstrap token over `http:` only when the server address's host is `localhost`,
  `127.0.0.1` or `[::1]`, compared as written in the URL and never resolved. For an `http:` address on any other
  host, from `--url`, it exits 1 before the exchange request: no request is made, nothing is saved and the token is
  not printed. The error names the host and prints `aim-sdk init --url` with the `https:` address of that host.
  `https:` addresses are accepted on any host. `invalid_server_url_reason`, which `aim-sdk login` also uses, is
  unchanged.
- `aim-sdk init` also exits 1 before the exchange request, with nothing sent or saved, when `--url`, once the
  whitespace around it is trimmed, has a backslash, whitespace or a control character (U+0000-U+001F,
  U+007F-U+009F), whatever its scheme. The error does not print the address. urllib3, which `requests` connects
  with, ends the host at a backslash and `urlsplit`, which the check above reads, does not: for
  `http://aim.example.test\@127.0.0.1`, `urlsplit` reads the host `127.0.0.1` and urllib3 `aim.example.test`.
- Test: `tests/test_cli_init_cleartext_url.py`.

### Fixed — request bodies carry the members the server reads, under the server's names

- `AIMClient.report_execution_status` sends `executed`, `executedAt` and, when there is one, `executionError`. The
  `strict_mode` parameter stays, for the decorators that pass it, and is not sent: the server takes the enforcement
  mode from the agent's organization, and `POST /api/v1/sdk-api/verifications/:id/execution-status` binds no member
  for it.
- `register_agent` (and `secure`) post no `organizationDomain` to `POST /api/v1/agents`, with SDK credentials or
  with an API key. The route registers the agent in the organization of the credential the request authenticates
  with and binds no domain.
- `SecretsClient.store_credential` and `rotate_credential` send the algorithm as `encryptionAlg`, the member
  `POST /api/v1/secrets/namespaces/:id/credentials` and `/rotate` read, and send no `ephemeralPublicKey`, which
  neither route reads. The `encryption_alg` parameter keeps its name.
- `A2AClient.register_skill` takes `input_modes`, `output_modes` and `examples` (keyword-only) and sends them as
  `inputModes`, `outputModes` and `examples`, members `POST /api/v1/a2a/skills` binds.
- Tests: `tests/test_client_request_bodies.py` and `tests/test_a2a_request_bodies.py` send each call through the HTTP
  layer and assert the body; the A2A tests read the members each handler binds and the server's messages from the
  backend source. `test_store_credential`, `test_store_credential_with_an_ephemeral_key` and
  `test_rotate_credential` in `tests/test_secrets.py` assert the store and rotate bodies.

### Deprecated — arguments the server does not read

- `register_agent(organization_domain=...)` is accepted for one more minor version, emits a `DeprecationWarning` and
  is not sent. Its docstring no longer describes it as a domain for auto-approval: no server route reads it.
- `register_skill(input_schema=..., output_schema=...)` emit a `DeprecationWarning` and are not sent; the skill
  route binds no schema.
- `store_credential(ephemeral_public_key=...)` and `rotate_credential(ephemeral_public_key=...)` emit a
  `DeprecationWarning` and are not sent; the store and rotate routes read `encryptedBlob` and `encryptionAlg` only.
- `A2AClient.update_trust_score` sends no request and raises `A2ATrustScoreWriteRefused`, a subclass of `A2AError`
  with `status_code` 405, the status the server answers `PUT /api/v1/a2a/trust/:id` with. The error carries the
  server's reason and names `GET /api/v1/a2a/trust/:id` and `POST /api/v1/a2a/trust/:id/interaction`
  (`record_successful_interaction`, `record_failed_interaction`). The method is removed at the next major version.

### Fixed — the docs in the SDK download point at the test suite in the repository (#735)

- The SDK download (`GET /api/v1/sdk/download?sdk=python`) carries `docs/` and leaves out `tests/`. `docs/README.md`
  links the test suite at `sdk/python/tests` in the repository instead of `../tests/`; the CrewAI and LangChain
  guides link `tests/test_crewai_integration.py` and `tests/test_langchain_integration.py` in the repository instead
  of a file beside the guide; and the test step of the release process in `docs/VERSIONING.md` clones the
  repository and installs the `dev` extra before it runs `pytest tests/`.
- Test: `TestSDKArchive_PythonDocsPointAtTheTestsInTheRepository`, which builds the archive from the repository's
  `sdk/` tree through the download handler.

### Fixed — the tests run against an empty temporary home, not the stored credentials of the machine (#726)

- `aim_sdk.credentials` computes `AIM_DIR` and the files under it from `Path.home()` when it is imported, and
  `load_sdk_credentials()` also adopts SDK credentials from a `.aim` folder in the working directory, in the folder
  that holds the `aim_sdk` package, or in the nearest folder above the working directory that has one. With the
  checkout inside a folder whose `.aim/sdk_credentials.json` held SDK credentials, and that folder as `HOME`,
  `test_logout_removes_legacy_sdk_credentials_so_status_stays_signed_out` and
  `test_logout_without_credentials_posts_nothing_and_exits_zero` failed: the CLI adopted those credentials, and the
  logout request the second test captured carried their access and refresh tokens.
- An autouse fixture in `tests/conftest.py` gives each test a new empty directory under the pytest base temp as
  `HOME` and `USERPROFILE`, points the `aim_sdk.credentials` paths and the default broker socket and token paths of
  `aim_sdk.grant_client` into it, and turns the `.aim` folder discovery off. The CLI child process in
  `tests/test_aim410_console_transport_polish.py` turns the discovery off as well and runs from the test's temporary
  home instead of the checkout. In the same layout both tests pass.
- `tests/test_isolated_home.py` checks that the home is an empty directory under the pytest base temp, that the
  paths computed at import time point into it, and that SDK credentials in a `.aim` folder above the working
  directory are not adopted.

### Changed — the two printed lines that held an em dash now read as two sentences

- `aim-sdk login` reports a device-code answer it cannot explain as `HTTP <status> from <url>. Verify the
  server URL is the AIM API base (e.g. https://api.aim.opena2a.org), not the dashboard URL`. The words are
  the ones it printed before, with a full stop and a capital letter where the dash stood.
- The line the SDK prints after a refresh that rotated its token reads `Token rotated successfully. Old
  refresh token revoked (new id: ...)`. The quick-start notes and the token rotation guide quote the new
  form. The register call's full output that the root README links, `docs/captures/quick-start.txt`, and
  its record in `docs/captures/quick-start.json` were captured again on 2026-10-16 with the SDK and a
  server with Redis built from this repository, and show the new form; the trust score and the lines the
  README shows are the same as in the earlier capture.
- Tests: `test_an_unexplained_answer_to_the_device_code_request_says_to_verify_the_url_in_two_sentences` in
  `tests/test_cli_login_server_unavailable.py`, and the server-with-redis case of
  `test_first_refresh_after_login_prints_the_rotation_line_only_when_the_server_rotates` in
  `tests/test_readme_quick_start_runs_as_written.py`. `test_rotation_print_says_old_token_revoked` in
  `tests/test_stale_creds_and_rotation.py` pins the new form of the rotation line.
  `test_full_register_capture_shows_the_token_rotation_line_this_sdk_prints` holds the recorded capture's
  rotation line to the line this SDK prints, and
  `test_full_output_text_holds_each_sidecar_capture_after_its_command` holds `docs/captures/quick-start.txt`
  to the record, both in `tests/test_readme_quick_start_runs_as_written.py`.

### Changed — the `sdk_credentials_lock` docstring says the lock does not serialize the Python SDK with the Java SDK on Linux (#722)

- `sdk_credentials_lock` keeps processes of the Python SDK apart, and nothing in the code changes. On Linux it
  does not keep a process of the Java SDK apart from a process of the Python SDK: the Python SDK locks the same
  file with `flock`, the Java SDK with a record lock (`FileChannel.tryLock`), and Linux holds the two separately.
  Measured with the calls each SDK makes, on Linux 7.0.12 with Java 21.0.12 and Python 3.12.15: each took its lock
  while the other held its own, in both orders. On macOS 26.6.2, with Java 25.0.2 and Python 3.14.3, each was
  refused its lock while the other held its own, in both orders (#722).

### Fixed — the cached-capability lookup sends the MCP server name as one query value (#718)

- The lookup that asks `GET /api/v1/sdk-api/agents/{id}/mcp-servers/by-name` for the capabilities the server holds
  for an MCP server writes the name with `urllib.parse.urlencode`, as `list_agents` writes its filters. A name
  holding `&`, `+`, `#`, `=` or a space reaches the server as that name, and the endpoint the client signs is the
  encoded one it sends.
- Tests: `test_the_server_name_is_sent_as_one_value` and `test_the_signed_endpoint_is_the_encoded_one_that_is_sent`
  in `tests/test_cached_mcp_capabilities_by_name.py`.

### Changed — the tests import only the names they use (#718)

- `ruff check --isolated --select F401 tests` reported 57 unused imports in 28 test files. 55 are removed; `import
  requests` and `import smtplib` in `tests/test_e2e.py` stay, marked `# noqa: F401`, because the capability detector
  that test runs reads them from `sys.modules`. The same command now reports none.
- `tests/test_tests_import_only_what_they_use.py` runs pyflakes, which the `dev` extra installs with flake8, over
  the test directory and fails on an unused import.

### Fixed — `log_capability_result` and `log_action_result` send no request, and the decorators post no result (#709)

- `AIMClient.log_capability_result` posted to `POST /api/v1/sdk-api/verifications/{id}/result` and discarded the
  answer. The server's handler for that route answers 403 to every request that reaches it, and the route's
  authentication answers 401 to a request it cannot authenticate. The method now sends nothing, returns `None` and
  warns with a `DeprecationWarning`. So does `log_action_result`, its older name, which the LangChain and CrewAI
  integrations call.
- `@perform_action`, `@track_action` and `@require_approval` no longer call it after the function they wrap, so a
  wrapped call that ran under a verification makes one request fewer. That request went out after the function had
  run, with the client's request timeout (30 seconds unless `timeout` is given).
- Measured against a server built from the previous commit: the request the previous `log_capability_result` sent
  from a client that holds an API key was answered 401 `No authentication token provided`. A `POST` to the same
  route with no credential was answered 401, and one that carried an administrator's access token was answered 403
  with the code `executionOutcomeNotAccepted`.
- `docs/LANGCHAIN_INTEGRATION.md` says that no result is posted. At the repository root, the decorator example in
  `docs/sdk/python.md` and the capability verification pattern in `examples/README.md` no longer call the method.
- Tests: `TestLogActionResult` and `TestPerformActionDecorator` in `tests/test_client.py`,
  `test_no_entry_point_posts_to_the_result_route` in `tests/test_enforcement_matrix.py`, and
  `test_absent_verification_id_issues_no_request` and `test_present_verification_id_issues_no_request` in
  `tests/test_release_202_p3_findings.py`.

### Fixed — `list_agents` writes each filter as one query value, and names the statuses and types the server filters by (#709)

- `GET /api/v1/agents` applies the `status` and `agent_type` filters `list_agents` sends (see the platform
  changelog). `total` in the answer then counts the agents that pass the filters. Measured on a server built from
  the commit before that change: the server ignored both filters and answered with every agent of the organization.
- `list_agents` percent-encodes each value. Read back with `urllib.parse`, the URL the previous `list_agents` built
  for a status of `a&limit=1` held a second `limit` parameter of `1`, the one for `a#fragment` ended its query at the
  `#`, and the one for `a+b` held `a b`.
- The docstring names the statuses an agent can hold (`pending`, `verified`, `suspended` and `revoked`). It listed
  `denied` as a status and `mcp_server` as an agent type; the server defines neither. Its example reads
  `trustScore`, the key the server answers with, in place of `trust_score`.
- Test: `tests/test_list_agents_filters.py`.

### Fixed — four A2A methods request the routes the server registers (#701)

- `A2AClient.get_task_history` requests `GET /api/v1/a2a/tasks?agentId=<id>&limit=<n>`, and
  `get_security_settings`, `update_security_settings` and `get_security_violations` request
  `/api/v1/a2a/security/settings` and `/api/v1/a2a/security/violations`. They put the agent or organization
  identifier in the path, on routes the server does not register.
- The server answers the settings and violations routes for the organization of the caller's credentials.
  `organization_id` is optional in the three methods; when it is given and names another organization, they raise
  `A2AError`, and `update_security_settings` writes nothing. `get_security_violations` checks it against the settings
  answer, with one more request, before it requests the violations: an answer with no violations names no
  organization.
- The server replaces every A2A security setting with the ones a `PUT` carries, so `update_security_settings` reads
  the current settings and writes them back with the given ones changed. It sends `requireRequestSigning`,
  `allowedAgentIds` and `blockedAgentIds`, the names the server reads, in place of `requireSigning`, `allowedAgents`
  and `blockedAgents`.
- `get_task_history` raises `ValueError` for a `target_agent_id` that is not a UUID, which the server would ignore and
  answer with every task the organization can see. `get_security_violations` returns the list the server answers, and
  an empty list when there is none.
- Tests: `tests/test_a2a_task_and_security_routes.py`.

### Changed — `A2AClient.update_agent_card` replaces the card document

- `PUT /api/v1/a2a/cards/:id` reads `cardUrl` or `cardData` and replaces the agent's card document: the server
  fetches the card from `cardUrl` or takes `cardData`, recomputes its hash, issues a new attestation and stores the
  card in place of the previous one. `update_agent_card` takes the keyword arguments `card_url` and `card_data` (a
  dictionary), sends them as `cardUrl` and `cardData`, and returns an `A2AAgentCard` with the stored card's
  `card_id` and `card_url`, the identifiers of its skills, each as a dictionary with an `id`, and
  `attestation_expires`.
- `name`, `description`, `version` and `skills` are deprecated. The route read them and stored none of them: after a
  call that carried all four, answered 200, the card read back held none of the four values. It now answers a body
  that carries only them 400. Given one of them, `update_agent_card` raises `ValueError` naming it and sends no
  request; without `card_url` or `card_data` the message is the server's answer, `cardUrl or cardData is required:
  this route replaces the agent card document and does not edit fields`.
- Test: `tests/test_a2a_update_agent_card_document.py`.

### Fixed — the JIT approval link opens the dashboard's JIT requests page (#701)

- `@perform_action(jit_access=True)` and `@require_approval` print `<dashboard>/dashboard/admin/jit-requests`, the
  page that lists pending verifications for approval, where `<dashboard>` is the dashboard URL `aim-sdk demo` derives
  from `aim_url` (`https://aim.opena2a.org` for `https://api.aim.opena2a.org`, port 3000 for port 8080). They printed
  `{aim_url}/dashboard/admin/verifications`, a path the server does not register and a page the dashboard does not
  have.
- `aim-sdk demo` derives its dashboard URL with the same function, which now drops only a trailing `/api` path: for
  `https://api.example.com` it keeps the host, where the demo derived `https:/.example.com`.
- Tests: `tests/test_jit_approval_dashboard_link.py`.

### Changed — `aim-sdk login` and the README say what the server checks on an `@perform_action` call (#656)

- The closing text of `aim-sdk login` says the server checks each `@perform_action` call against the agent's
  granted capabilities and the organization's security policies. It says that in strict enforcement mode a call
  outside the granted capabilities is denied unless a security policy allows it, and that in monitoring mode, the
  default, it runs and is recorded. It said each call was authorized by the server's 5-step Fine-Grained
  Authorization and that calls outside the granted capabilities were denied.
- The README says the same of the decorator, and that the 5-step engine runs on `POST /api/v1/agents/:id/authorize`,
  which the SDK does not call.

### Changed

- The PyPI "Homepage" link and the first link in this README go to the project page,
  https://opena2a.org/agent-identity-management. They pointed at the GitHub repository and at the
  PyPI page itself. The repository stays listed under "Source".
- The README Quick start says what a pending agent's call does in each enforcement mode
  ([#655](https://github.com/opena2a-org/agent-identity-management/issues/655)). It said every call
  of a pending agent is refused with `Agent not verified - all actions denied`. That refusal
  needs a strict-mode organization; on a new organization, which starts in monitoring mode, the
  call runs and AIM raises an `Unverified Agent Action (Monitoring Mode)` alert for it. The root
  README carries the same correction.

### Changed — `report_capabilities` follows the enforcement mode and raises when a registration fails

What to check before upgrading:

- In strict mode a reported capability stays pending until an administrator approves it. The
  result counts it under `pending`, not `granted`. Code that reads `granted` to decide whether
  a capability can be used should read `pending` as well.
- A registration that AIM refuses or fails now raises `VerificationError`, and the capabilities
  after it are not sent. Failures used to be swallowed, and one whose message contained "500"
  was counted as granted.

What changed:

- `report_capabilities` sends one `POST /api/v1/sdk-api/agents/{id}/capabilities/register` per
  distinct capability, one at a time in the order each first appears, with the body
  `{"capabilityType": ...}`, and reads the outcome AIM answers with. It used to post to
  `/api/v1/sdk-api/agents/{id}/capabilities`.
- The result keeps `granted` and `total` and adds `pending` and `results`. `granted` counts
  capabilities granted now or already held, `pending` counts those awaiting approval, `total`
  counts distinct capabilities, and `results` has one entry per capability with its `status`
  (`granted`, `already_exists` or `pending`) and, for a new pending request, its `request_id`.
- The `scope` argument is still accepted. It is ignored and emits a `DeprecationWarning`.
- An entry that is not a non-empty string raises `ConfigurationError` before anything is sent.

### Fixed

- The `secure()` calls in `docs/sdk/python.md`, `docs/sdk/authentication.md`, the LangChain and CrewAI
  integration guides and the example guides under `docs/` no longer pass `private_key=`
  ([#665](https://github.com/opena2a-org/agent-identity-management/issues/665)). `secure()` has no such
  parameter, and each of those 20 calls stopped with
  `TypeError: register_agent() got an unexpected keyword argument 'private_key'`. Where a guide registers
  the agent, the call drops `private_key=` (the production example in `docs/sdk/python.md` passes an agent
  API key instead): `secure()` generates the keypair and saves it to `~/.aim/agents/<name>.json`, so the
  example and integration guides no longer ask you to register the agent in the
  dashboard or export `AIM_PRIVATE_KEY` first. Where a guide signs with a keypair you hold, it passes the
  keypair, the agent ID and `aim_url` to `AIMClient(...)`. The `secure()` parameter list in
  `docs/sdk/python.md` names the parameters `secure()` takes, and no longer lists `private_key` or
  `auto_verify`.
- `requirements.txt`, which ships in the SDK download, lists `PyJWT`, which the SDK uses to read its
  sign-in tokens ([#664](https://github.com/opena2a-org/agent-identity-management/issues/664)). So do the
  dependency install commands in `docs/sdk/python.md`, `sdk/python/docs/ENV_CONFIG.md` and the integration
  and example guides under `docs/`. They listed `keyring`, `PyNaCl`, `requests` and `cryptography` only, and with only those four
  installed, `aim-sdk login` and `from aim_sdk import secure` stopped with
  `ModuleNotFoundError: No module named 'jwt'`. `setup.py` already listed `PyJWT`.
- `register_mcp()` adds the given MCP server to the agent's list. It sends
  `PUT /api/v1/agents/{id}/mcp-servers` with `mcpServerIds`, `detectedMethod`, `confidence` and `metadata`,
  and returns what AIM answers: `message`, `talksTo`, `added_servers` and `total_count`. It used to post
  those members, spelled in snake case, to `POST /api/v1/sdk-api/agents/{id}/mcp-servers`, the route that
  creates an MCP server and reads none of them. With the agent's signing keys that call created an MCP
  server with an empty name and URL, or was answered 409 once one existed, and the given server was never
  added to the agent's list. AIM adds to an agent's list only for a signed-in user with the member role or
  higher, so the call sends the user's token when the client holds the SDK sign-in (`aim-sdk login`, then
  `secure()`). A client that holds only an API key or the agent's signing keys is refused by AIM, and the
  `AuthenticationError` says what the route admits. The method does not create an MCP server: the agent's
  list shows the server once one with that ID or name is registered in the organization. The docstring's
  return members and the README example are corrected.
- `secure()` and `register_agent()` send only tag IDs when they sync `tags` to an agent that is already on
  the server (OAuth mode). `POST /api/v1/agents/{id}/tags` reads tag IDs only and answers 204 when it adds
  them. The SDK sent it every entry except the names of tags already on the agent, so a tag name that was
  not on the agent yet made the route answer 400 `Invalid tag ID format` and no tag in the list was added,
  tag IDs included; and it read the 204 as a failure, so tags that had been added were reported as
  `Failed to sync tags`. It now sends the tag IDs that are not on the agent yet, in the lowercase
  36-character form, and reports a 204 as `Applied N tag(s)`. A tag name that is not on the agent yet is
  not sent: a warning names it and says that AIM applies a tag name only when the agent is first
  registered. An entry counts as a tag ID when it is a UUID in its 36-character form or as 32 hexadecimal
  digits, in either letter case. The `tags` entry of the `register_agent` docstring describes both cases.
- `aim-sdk login` reports an unavailable AIM server as unavailable and exits 75. When the server answers
  HTTP 502, 503 or 504, or gives no HTTP answer (refused, reset, unresolvable, unanswered), to the pre-flight
  probe, the device-code request or a token poll, the login ends at that request: nothing is retried, slept
  on or stored, no browser is opened unless a code was already issued, and the last two lines name the
  server URL, the status or the failure, and when to run the same command again (the server's `Retry-After`
  in whole seconds, otherwise "later"). These outcomes used to exit 1 as "Login failed: ..." or "Could not
  start the device login: ... verify the server URL", and a 503 whose body named `authorization_pending`
  kept the CLI polling until the code's lifetime ran out and then reported an expired code. The HTTP status
  now decides before the body is read: `authorization_pending`, `slow_down`, `expired_token` and
  `access_denied` count only on a 400 or 403, and a 429 is always a slow-down. A rejected TLS certificate
  and a URL that cannot be requested keep exit 1. `aim-sdk login --help` lists the exit codes (0, 1, 2, 75).
- `client.secrets.resolve()` refuses a namespace or operation that contains a line feed, a carriage
  return or `|`, raising `SecretsError` before anything is signed or sent. The request signature covers
  the text `namespace|operation|nonce`, so a field cannot carry the separator or a line break; such a
  value used to be signed and sent as given.
- Agent credential files are owner-only from the moment they are created. `~/.aim/agents/` is
  created with mode 0700, and each `~/.aim/agents/<name>.json`, which holds the agent's private
  key, is written to a new file opened with mode 0600 that then replaces the old one. The file
  used to be written with the process's default mode (0644 under the usual umask 022) and set to
  0600 only after the key was on disk, in a directory left at the default mode. A directory or
  file left wider by an earlier version is narrowed on the next save, and a save that fails part
  way now leaves the previous file as it was instead of a truncated one.

- `aim-sdk status` always states a verdict, and its exit code follows it. On an upgraded install holding only a
  legacy `~/.aim/credentials.json` (snake_case keys, a refresh token, no access token) it printed Server, `User:
  Unknown` and Credentials, then nothing, with exit 0. It now reads the legacy keys, prints `Authenticated ...` or
  `Not authenticated ...` for every token state, and says the check was local. It cites only a credentials file that
  exists: when adopting the legacy file fails, it names the file it actually read instead of a
  `sdk_credentials.json` that was never written, and with no stored credentials at all `status --json` reports
  `credentialsPath` as `null` instead of naming the `sdk_credentials.json` that login would write. A missing server
  URL is shown with login's default. The adoption and migration notices no longer claim `[OK]` when the write failed,
  and they go to stderr, so `status --json` stays one JSON object on stdout (#409).

- `aim-sdk login` checks `--url` before touching the network. An empty value or a string that is not an http(s)
  URL with a host now fails at once with `Error: --url ... is not an http(s) URL` (or `is empty`, `has no host`)
  and an example; it used to reach the reachability probe and report `could not reach the AIM server ... no HTTP
  response within 5s`, a network failure that never happened. An unreachable but well-formed URL still fails
  within the probe bound with the reachability message (#408).

- `register_pqc_key`, `rotate_pqc_key` and `set_hybrid_mode` now send the request members the server reads
  (`publicKey`, `algorithm` and `enableHybrid`; `newPublicKey` and `algorithm`; `enable`). They sent
  `pqcPublicKey`, `enableHybridMode`, `newPqcPublicKey` and `enabled`, which the server does not read, so
  before this release `register_pqc_key` and `rotate_pqc_key` always failed, because the key arrived empty,
  and `set_hybrid_mode(True)` turned hybrid mode off. `enable_hybrid_mode` now defaults to `False`, and
  `register_pqc_key(..., enable_hybrid_mode=True)` and `set_hybrid_mode(True)` raise `ConfigurationError`
  before sending anything: hybrid mode marks the agent as requiring both an Ed25519 and an ML-DSA
  signature, and no AIM SDK signs requests in that form yet. The three routes admit an AIM user's access
  token with role member, manager or admin, not the agent's own signature or an API key; the docstrings
  now say so.
- Processes that share `~/.aim/sdk_credentials.json` now refresh it once. Each refresh token is accepted
  once, and every process used to present the token it had read at startup, so when two refreshed together
  the second presented a token the first had already rotated. The server refused it, and for a sign-in from
  `aim-sdk login` it treats that as reuse and ends the sign-in for every process. Before refreshing, the SDK
  now takes a lock on the file (`sdk_credentials.json.lock` beside it) and re-reads the file under the lock.
  If another process already refreshed, the SDK uses that process's access token, stored in the file as
  `sharedAccessToken` together with a hash of the refresh token it was issued with, and makes no request.
  Otherwise it presents the refresh token the file holds now, never a copy another process rotated. A
  refresh token the server refused is not presented again in the same process until the file holds a
  different one. If another process holds the lock for more than 60 seconds, the refresh is skipped with a
  warning instead of being sent without the lock. The file is written to a temporary file and moved into
  place, so another process never reads a partly written file. The exception is Windows when another process
  has the file open: the move is refused there, and the file is rewritten in place, so a reader at that moment
  can see it partly written.
- The "SDK REFRESH TOKEN REJECTED" banner gives the fix that matches the stored credential. A credential
  written by `aim-sdk login` (it carries `accessToken` and `organizationId`, which a dashboard SDK download
  never does) is told to sign in again with `aim-sdk login --url <server> --force`, which replaces the file;
  its causes now include the sign-in reaching the server's maximum session age. It used to be told to
  `rm` the file and download a fresh SDK. A downloaded credential keeps the download steps, with signing
  in as the alternative. Every SDK error that named the dashboard page "Settings → SDK Downloads" now names
  the page that exists, Developers, then SDK & docs, and the two agent-registration errors raised after a
  refused token give the same per-credential fix.
- **Every entry point now reports a blocked action before raising.** In 2.0.0 only `@aim_verify` and its four
  convenience wrappers sent the execution report on the blocked path; `perform_action` (the decorator the README
  Quick start teaches), `track_action`, `require_approval` and the LangChain `aim_verify` raised the denial without
  it, so a denial from them left no record that the action stopped. All six now send the same report
  (`executed: false`, `strictMode: true`, `executionError: "Blocked by AIM: ..."`) through one shared path, widened
  in a single change so the execution record means the same thing for every entry point. The report is
  fire-and-forget with its short timeout, and a failure to send it, including one that raises, never lets the
  denied action run; before this, an exception out of the report on the `@aim_verify` path replaced the denial
  the caller saw (#382).
- `aim-sdk logout` revokes the stored session on the server: it posts `POST /api/v1/auth/logout`
  with the access token as the bearer and the refresh token in the body, and prints a confirmed
  sign-out only when the server reports the refresh token revoked; otherwise it names the reason
  and the token's expiry, still clears `~/.aim/sdk_credentials.json`, and exits 1. It used to post
  `/api/v1/auth/revoke`, a route no backend registers, and print `[OK] Token revoked` whatever the
  server answered, so the refresh token stayed usable for up to seven days after a logout.
  Requires a backend that reports `revoked` on logout (this repository from this change; a
  published image from the first platform release that carries it).
- `secure(name, aim_url=..., api_key=...)` registers through `POST /api/v1/agents` with the
  key in `X-API-Key` and a locally generated keypair, the route and header the backend admits
  and the TypeScript SDK already uses. It used to post to `/api/v1/public/agents/register`
  with `X-AIM-API-Key`, a header no backend reads, and failed with 400 on every self-hosted
  stack. `X-AIM-API-Key` is retired; every API-key site sends `X-API-Key`. Requires a backend
  built on or after 2026-08-26 (the image `latest` names and every platform release from
  1.1.0 on qualify). A 401 in api-key mode now names the credential and where a valid one
  comes from.
- `aim_sdk.capability_detection` no longer carries a module-level `__version__ = "1.1.0"`,
  which disagreed with `aim_sdk.__version__` on every 2.0.x release
  ([#475](https://github.com/opena2a-org/agent-identity-management/issues/475)). It was the last
  such literal: the test that pinned `detection.py` and `protocol_detection.py` (#464) now walks
  every module under `aim_sdk/` for a string-constant `__version__`, and every importable
  submodule that exposes `__version__` must equal the package version.

- The README Quick start runs as written. It called `db.query` and `db.execute` on a `db` it
  never defined, never made the allowed call, and showed the sign-in step after the code. It
  now signs in first, registers the agent with `capabilities=["db:read"]`, says an
  administrator verifies the agent before its first call, makes one allowed and one refused
  call against a dictionary it defines, and places the strict-mode step the refusal needs
  (a new organization starts in monitoring mode) between the two calls.

### Changed

- `aim-sdk login` authenticates through the OAuth 2.0 device grant (RFC 8628): the CLI
  prints a short code and opens your dashboard's `/device` page, you sign in and approve
  the code there, and the same access and refresh tokens the dashboard login issues are
  stored at `~/.aim/sdk_credentials.json`. The browser-callback flow it replaces posted an
  authorization code to `/api/v1/auth/token`, a route only AIM Cloud served, so a stack
  built from this repository never completed the login (measured 2026-09-22). The local
  callback server, the redirect URI and the PKCE parameters are gone; `--url`, `--force`,
  the exit codes and the credentials file are unchanged.
- `A2ATrustScore.a2a_trust_score` is `None` while the agent is unscored (no task data
  yet) and `score_status` reads `unscored`; responses are no longer parsed with a `0.0`
  default that read as a measurement.
- `A2AClient.update_trust_score` is deprecated: the server refuses the write with 405
  because the A2A trust score is measured from recorded interactions, not asserted.

### Changed

- **Console output no longer uses U+2713 CHECK MARK / U+2717 BALLOT X**
  (#479). The 52 lines across 10 modules (`console`, `client`, `oauth`,
  `credentials`, `secure_storage`, and the CrewAI, LangChain and MCP
  integrations) that printed those marks now print the plain-text markers
  `[OK]` and `[FAIL]` instead; where rich is available the markers keep the
  same green/red markup as before. The no-emoji test suite's package-source
  walk no longer exempts these characters, so they cannot return; its
  non-vacuity control now asserts the `[OK]`/`[FAIL]` vocabulary is present
  instead of the old marks. Documentation prose showing pre-change output is
  unchanged and out of this change's scope.

### Fixed

The five P2 findings from the 2.0.2 release test, re-measured at head and
fixed as a batch.

- **`aim-sdk login` is bounded and fails fast on a dead URL.** The command now
  probes the server before anything else: with an unreachable `--url` it exits
  non-zero within the 5s probe bound, names the URL it could not reach, and
  never opens a browser. After a successful browser open, the callback wait is
  bounded by a 180s deadline; on expiry the command prints a timeout message
  and exits non-zero. Previously there was no reachability check before
  `webbrowser.open`, and the wait loop re-entered `handle_request()` after
  every socket timeout, so "Waiting for authentication... (Ctrl+C to cancel)"
  waited forever against a closed port.
- **The README manual-mode example works as written.** `secure(name,
  api_key=...)` requires `aim_url` — there is no default server URL and no
  environment-variable fallback in `secure()` — but the README's manual-mode
  one-liner omitted it, so pasting the documented line raised
  `ConfigurationError`. The example now carries `aim_url=` beside `api_key=`
  and the manual-mode prose states the requirement.
- **The no-credentials error names the real fixes.** The `ConfigurationError`
  raised when neither SDK credentials nor an `api_key` are present told a pip
  user to "download SDK from dashboard ... or provide api_key parameter",
  naming neither the `aim-sdk login` command (the README's own step for a pip
  install) nor the fact that api_key mode also requires `aim_url`. It now
  names both.
- **The security log distinguishes "AIM was never asked" from "AIM said no",
  and records unverified execution.** Every non-answer used to render as
  `result: DENIED`: the `CAPABILITY_CHECK` event for a transport failure now
  carries `result: UNAVAILABLE`, and for a 401 (`AuthenticationError`)
  `result: UNVERIFIED`; only an actual policy denial renders `DENIED`. And
  when the enforcement rule executes the wrapped function without a completed
  verification (monitoring mode or the unresolved-mode transport window), an
  `ACTION_EXECUTED` event with `result: EXECUTED_UNVERIFIED` now records that
  the action ran — previously `ACTION_EXECUTED` was defined but emitted
  nowhere, so the log showed a "denial" followed by nothing.
- **No success line on a rejected capability registration.** The
  `@perform_action` auto-registration path announced "Registered ..." whenever
  `register_capability` returned without raising — including a 404's
  `not_tracked` result and a 401 swallowed into an `error` result — so a
  rejected registration printed a success line (on the 401 path, above the
  failure warning). The wrapper now announces only a plain registered success;
  `register_capability`'s own per-status output (granted, pending, warnings)
  is unchanged, a granted registration is announced exactly once instead of
  twice, and the 500 path still prints only a warning.

The twelve P3 findings from the 2.0.2 fresh-user release test, each fixed
(none declined, none narrowed to a README note alone unless listed as such):

- **`AIM_SECURITY_LOG_STDOUT` accepts the same spellings as `AIM_STRICT_MODE`**
  (item 1). `true`/`1`/`yes`/`on` install the stdout handler and
  `false`/`0`/`no`/`off` do not, case-insensitively with surrounding
  whitespace stripped, instead of an equality against the single literal
  `"true"`. `AIM_SECURITY_LOGGING_ENABLED` parses with the same spellings
  (still enabled by default). The README passage naming the variable now
  enumerates the accepted true spellings.
- **README's default-logging claim matches the installed handler** (item 2,
  narrowed in the README). The package attaches a stderr handler at ERROR
  level on import by design; the README no longer claims "under the SDK's own
  defaults nothing is written" and instead states that ERROR-severity records
  go to stderr and WARNING-severity records do not.
- **`set_quiet` is exported and documented, and an unreachable verification
  says each thing once** (item 3). `from aim_sdk import set_quiet` silences
  the SDK's informational stdout output (info/warning/success lines,
  registration banners, detection results); the README documents it. The
  pending-enforcement-change sentence is emitted once, through the typed
  `PendingEnforcementChange` warning, no longer duplicated onto stdout via
  the console.
- **A registration that cannot connect raises a typed, actionable error**
  (item 4). `secure()` against an unreachable server raises
  `ConfigurationError` naming the URL and the remedies (correct the URL,
  start a server, `aim-sdk login --url`), with the `requests`/`urllib3`
  chain suppressed (`raise ... from None`); a 401 on either registration
  path raises `AuthenticationError`, the same class the verification path
  raises for a 401.
- **No request URL ever carries an absent identifier, and the result body is
  camelCase** (item 5). `log_capability_result` returns without issuing a
  request when `verification_id` is absent (the guard
  `report_execution_status` already had), so the SDK never POSTs to
  `/verifications/None/result`; its body now sends `resultSummary`/
  `errorMessage`, the same key convention as the SDK's other write bodies.
- **Every request to an AIM URL carries `User-Agent: AIM-Python-SDK/<version>`**
  (item 6), including the agent-exists check, both registration requests,
  MCP-server registration, and the cached-credential validation probe, which
  previously went out under the `requests` default.
- **The PyPI page is self-contained and the classifiers cover
  `python_requires`** (item 7). Every README link that pointed outside the
  packaged tree (`../../README.md`, `examples/`, `docs/`, sibling SDKs,
  LICENSE) is now an absolute GitHub URL; the version pointer is
  `aim_sdk.__version__` (resolvable from a wheel-only install) instead of
  "see `VERSION` file"; classifiers now include 3.13 and 3.14 and
  `python_requires` carries the matching `<3.15` upper bound.
- **The login banner is rectangular** (item 8): all four lines of
  `aim_sdk.cli.print_banner` are 61 columns, so the right border is a single
  column.
- **One public spelling of the security-logging entry point** (item 9).
  `configure_security_logging` is now bound in `aim_sdk.security_logging`
  itself (with `configure_from_environment` kept as a compatibility alias),
  so both `from aim_sdk import configure_security_logging` and
  `from aim_sdk.security_logging import configure_security_logging` work,
  and the module docstring's usage block shows that spelling.
- **An unreachable AIM costs a bounded, stated amount** (item 10). A refused
  connection is no longer retried with 1+2+4s exponential backoff (~7s per
  request); it fails on the first attempt. The README states the worst-case
  cost of an unreachable server and how to change it (`timeout`,
  `enforcement_timeout`, `max_retries`, `auto_retry`).
- **A malformed 200 is named as malformed** (item 11). Both registration
  paths agree that 200 and 201 are success; a success status whose body
  lacks the agent's credentials (e.g. `200` with `{}`) raises
  `ConfigurationError` saying the response was malformed or unexpected and
  naming the statuses the SDK requires, instead of "Unknown error", and the
  security event records `body_malformed`.
- **`PolicyCache` is no longer exported from `aim_sdk`** (item 12). No
  released AIM server serves the route it fetches, so it cannot answer a
  check against any released backend; it is removed from `aim_sdk.__all__`
  and the top-level namespace and remains importable from
  `aim_sdk.auto_hooks` (unchanged behaviour) for the day a server registers
  the route.

The three P2 findings and the seven P3 findings from the 2.0.3 release test
(#487). P2.1, the `aim-sdk login` pre-flight and bounded callback wait, is
not repeated here: it is the same defect as the bounded, fail-fast
`aim-sdk login` of the five P2 findings from the 2.0.2 release test, and ships
above.

- **Protocol detection never scores what it did not find** (P2.2). With none
  of the seven indicator environment variables set and no protocol module
  loaded, `ProtocolDetector.get_detection_confidence` and the `confidence`
  field of `get_protocol_details` are `0.0` with `indicators_found` empty,
  instead of the 50.0 "base confidence" that reported a guess as a half-
  certain detection. A protocol name that is not in the indicator table at
  all — `get_protocol_details("bogus")` — scores 0.0 for the same reason.
  `detect_protocol()` still returns `"mcp"` when nothing matched; the default
  is unchanged, it is simply no longer reported as a 50% detection. Env
  indicator matches keep their 90+ scores and import matches their 60+.
- **Transport failures have one shape across every method** (P2.3). A refused
  connection or a request timeout now raises a `VerificationError` whose
  message names the host taken from `aim_url`, the failure class (connection
  refused / timed out, with the exception type beside it) and the next step
  naming the URL to check — from `_make_request` and therefore from
  `report_detections`, `report_sdk_integration`, `register_mcp` and every
  other method routed through it. The verification path builds the same
  sentence for its `UNKNOWN`/transport reason, so the decision's `reason`,
  the `VerificationUnavailableError` raised from it under a strict override
  and the `VerificationError` raised from a reporting call all read alike.
  Previously the two branches raised the bare literals `"Request timeout"`
  and `"Connection failed"` while the verification path forwarded the
  `requests` text verbatim. A failure that DID reach the server keeps saying
  what the server said: a 4xx/5xx from `raise_for_status` still renders as
  `Request failed: <status> ...`, not as "could not reach", because sending an
  operator to check that their server is running because it answered 404 is
  the same defect pointing the other way.
- **No raw library chain, and no print beside a raise** (P2.3). No message the
  SDK raises or returns for a transport failure contains the
  `requests`/`urllib3` chain text (`HTTPConnectionPool`, `Max retries
  exceeded`, `NewConnectionError`), and `_decide_capability` no longer prints
  `Warning: Network error during verification: ...` to stdout immediately
  before `verify_capability` raises about the same failure. The console
  warning stays on the path that acts on the decision permissively —
  `_verify_and_enforce` prints the verdict's warning before it runs the
  wrapped body — so a monitoring-mode caller is told exactly as before, and a
  caller who gets an exception gets only the exception. Every other message
  the SDK composes from a `requests` failure now goes through the same
  renderer rather than interpolating `str(e)`: `attest_mcp` (which posts with
  the session directly), the `error` a failed `_register_single_mcp` RETURNS,
  the CLI's `exchange_code_for_tokens`, the approval-poll warning, the
  `aim-sdk demo --cleanup` messages, and the best-effort capability/tag
  warnings. Anything a `requests` exception carries that is really urllib3's
  chain is dropped in favour of the exception's type name, so no future
  handler can reintroduce the chain by interpolating it.
- **Tool discovery reports the real cause and keeps third-party noise off
  stderr** (P2.4). `details.discoveryError` from `detect_with_tools()` now
  names the server, the command that was run and the underlying failure — the
  `OSError` text for a command that is not on PATH, the exit status for a
  process that stops without speaking MCP — instead of `str()` of an anyio
  exception group, whose text is "unhandled errors in a TaskGroup". The
  exception chain and its traceback go to the `aim_sdk` logger at DEBUG
  level, so nothing containing "Traceback" reaches stderr from a discovery
  run. Without the MCP client library installed, the configured command is
  probed directly so the answer is still the real cause rather than "MCP SDK
  not installed". The docstrings of `detect_with_tools` and `auto_detect_mcps`
  now state plainly that every configured server command is executed as a
  child process when `discover_tools` is true.
- **Nonsense arguments are refused at construction** (P3.1). `AIMClient`
  raises `ConfigurationError` for a `timeout` that is not a positive number
  (`0`, `-1`, `"abc"`), for a `max_retries` that is not a non-negative
  integer (`-1`), and for an `aim_url` with no http/https scheme and host
  (`"not a url"`). `MCPDetector(sdk_version=...)` raises `TypeError` for a
  non-string and `ValueError` for an empty string; `None` still resolves the
  installed version. `agent_id` is deliberately NOT validated as a UUID —
  API-key mode ids are server-issued strings. Relatedly, an `aimUrl` in a
  registration response is honoured only when it is a URL the SDK could send
  a request to; otherwise the URL the caller registered against is kept.
- **A corrupt Claude Desktop config is reported, not swallowed** (P3.2).
  `detect_from_claude_config()`, `detect_all()` and `detect_with_tools()`
  still return `[]` without raising when the config file is not valid JSON,
  but each now emits exactly one warning naming the file path and the JSON
  decode error, instead of `except Exception: pass`. The other readers of the
  same file (`get_mcp_server_config`, `discover_mcp_capabilities`,
  `discover_mcp_metadata`) go through the same reader.
- **The config-search docstring and the search agree** (P3.3).
  `detect_from_claude_config`'s docstring lists exactly the paths
  `_get_claude_config_path` searches, in search order, and the search now
  includes `~/.config/Claude/claude_desktop_config.json` — the location
  Claude Desktop uses on Linux — beside the legacy
  `~/.claude/claude_desktop_config.json`, with the platform-native macOS and
  Windows locations first. The docstring also names the locations that are
  NOT searched (`~/.cursor/mcp.json`, `./.cursor/mcp.json`, `./mcp.json`).
- **The detection modules declare their public surface** (P3.4).
  `aim_sdk/detection.py`, `aim_sdk/protocol_detection.py` and
  `aim_sdk/capability_detection.py` each define `__all__`, so
  `from aim_sdk.detection import *` no longer binds `json`, `os`, `sys`,
  `pathlib`, `Optional` or `datetime` into the caller's namespace.
- **The CLI answers in JSON and knows the word `help`** (P3.5).
  `aim-sdk status --json` writes exactly one JSON object
  (`authenticated`, `server`, `user`, `credentialsPath`, `tokenState`) and
  nothing else, keeping the exit codes (1 when not authenticated);
  `aim-sdk version --json` writes `{"version": "..."}`; `aim-sdk help` prints
  the command list and exits 0, where argparse used to reject it as an
  invalid choice; and a bare `aim-sdk` prints the same help and now exits 0
  instead of 1.
- **`docs/testing/release-smoke.md` carries a Python SDK row** (P3.6) in its
  component smoke matrix, with a run command that asserts an MCP detection's
  `sdkVersion` is the installed package's version.
- **`report_sdk_integration` defaults to the installed version, and the client
  docstring names the replacement API** (P3.7). `sdk_version` is now optional
  and resolves `aim-sdk-python@<aim_sdk.__version__>` through the same
  resolver `MCPDetector` uses when it is omitted or `None`; an explicit string
  is sent unchanged. `AIMClient`'s class docstring names `verify_capability`
  and `aim_sdk.decision.VerificationDecision`, the two names its deprecation
  warnings point callers to, instead of only `perform_action`.

### Fixed — console and transport polish (#410)

- Agent ids of 15 characters or fewer are shown whole in panels; `agt_123` rendered as `agt_123..._123`.
- Every row of the plain-text registration, existing-credentials and approval panels is as wide as its border; values that overflowed a row are cut with `...`.
- A trust score outside 0-100% renders as `invalid` instead of being graded: `150` printed `150% Excellent` and `-5` printed `-500% Low`.
- The A2A client and the telemetry relay send the package version in `User-Agent` (`AIM-Python-SDK/<version> (A2A)`, `OpenA2A-AIM-SDK-Relay/<version>`), where they sent a hardcoded `A2A-1.0.0` and no version.
- `aim-sdk logout` also removes the SDK tokens from the legacy `~/.aim/credentials.json`. The next command migrated that file again, so `aim-sdk status` read as signed in after a logout. Agent credentials in the same file are kept.

## [2.0.3] - 2026-09-09

### Fixed

- **MCP detection payloads report the installed package version**
  ([#464](https://github.com/opena2a-org/agent-identity-management/issues/464)).
  `aim_sdk/detection.py` and `aim_sdk/protocol_detection.py` each carried a
  stale module-level `__version__ = "1.0.0"`, and the three `sdk_version`
  parameters (`MCPDetector()`, `MCPDetector.get_runtime_detections()` and
  `auto_detect_mcps()`) froze `aim-sdk-python@1.0.0` into their defaults at
  import time, so every detection row built without an explicit `sdk_version`
  reported `sdkVersion: aim-sdk-python@1.0.0` regardless of the installed
  release. The module literals are deleted; the three defaults are now `None`
  and each entry point resolves `aim_sdk.__version__` at call time, so MCP
  detection payloads report the installed package version. Passing an explicit
  `sdk_version` argument behaves exactly as before.

## [2.0.2] - 2026-09-08

### Security

- **`PolicyCache.check()` no longer answers allow from an empty cache**
  ([GHSA-44hm-jvq6-r4gc](https://github.com/opena2a-org/agent-identity-management/security/advisories/GHSA-44hm-jvq6-r4gc)).
  `PolicyCache`, exported since 0.5.3 and described in its docstring as
  offline enforcement, fetches `GET /api/v1/agents/{agent_id}/policies`. No
  released AIM server registers that route, so the cache was always empty,
  and `check()` treated an empty cache as allow: it returned `True` for every
  capability, with no warning and no exception. No SDK code path calls it.
  The decorators, which ask the server on every call and raise
  `ActionDeniedError` on an explicit denial, are unaffected and were re-tested
  against the published 2.0.1 wheel.

  `check()` now raises `VerificationUnavailableError` whenever no policy
  document has been loaded (fetch failed, non-200, or a body that is not a
  JSON object), naming the URL it tried and stating that no released AIM
  server serves it. With a document loaded, a matching rule allows only on
  `action: "allow"`, and with no matching rule only a present `defaultAction`
  equal to `"allow"` allows. The class stays exported and its docstring now
  says what it does. Nothing in this release makes it enforce anything:
  nothing serves it.

  Affected on PyPI: 0.5.3 and 1.22.1 through 2.0.1. The `aim-server`
  container serves the same SDK as a zip from `GET /api/v1/sdk/download`; a
  zip named for an affected version carries the defect. The measured table
  is in the advisory.

  If you call `PolicyCache.check()` today, that call raises from this
  version on. Remove it; the decorators are the enforcement path.

  On the deprecation window: `check()` previously returned and now raises,
  and `docs/VERSIONING.md` would normally put a `DeprecationWarning` release
  in front of a change like that. As with 2.0.0, there is none, because the
  fix is the raise, and a warning release would ship the fail-open for one
  more version. A named exception, not a policy change.

- **The OpenAI and Anthropic hooks now emit a local security log event per
  hooked call**
  ([GHSA-hg62-664p-89xg](https://github.com/opena2a-org/agent-identity-management/security/advisories/GHSA-hg62-664p-89xg)).
  Since the hooks landed (0.5.3, 2026-03-21) the wrappers around
  `chat.completions.create` and `messages.create` named
  `AgentEventType.AGENT_ACTION`, a member that was never defined. The
  `AttributeError` was swallowed by the same `except Exception` that keeps a
  logging failure from blocking a model call, so no hooked model call was
  recorded in any affected version and nothing said so; the README said each
  model call was recorded to the audit trail. No test exercised a hooked
  call.

  `AGENT_ACTION` now exists, and a hooked call emits one event on the
  `aim.security` logger: a local record, written where
  `AIM_SECURITY_LOG_FILE` or `AIM_SECURITY_LOG_STDOUT` points it and
  propagated to the application's root logger, not the server-side audit
  log. A regression test installs the OpenAI hook and asserts that one
  hooked call emits exactly one event; it fails on 2.0.1. A second test
  fails on any `AgentEventType` member the package references without
  defining. Affected: 0.5.3 and 1.22.1 through 2.0.1, and the `aim-server`
  container zip named for those versions.

### Changed

- **`secure()` accepts `auto_hooks`** as a keyword-only parameter, default
  `True`. `secure("my-agent", auto_hooks=False)` installs none of the
  framework hooks. The README has documented this call; until this release
  it raised `TypeError`, because the parameter did not exist.
- `setup.py` declares `license="Apache-2.0"` and the wheel ships the
  canonical Apache-2.0 LICENSE text.

### Fixed

- **A hook that cannot be installed now logs a warning.** `activate_hooks()`
  detects LangChain, CrewAI, OpenAI and Anthropic from `sys.modules`; a
  detected library whose hook failed to install was skipped silently. It is
  now reported through the `aim_sdk.auto_hooks` logger at WARNING and left
  untouched. The hooks still never raise; a failure inside an installed hook is still
  swallowed so the model call proceeds, which is the property that hid the
  recording defect above, so on that path the regression test is the guard.

## [2.0.1] - 2026-08-22

Closes both defects 2.0.0 carried as known issues, and hardens every path a
server-sent string takes to a developer's terminal.

### Security

- **Server-sent text is sanitized before it can reach a terminal**
  ([#384](https://github.com/opena2a-org/agent-identity-management/issues/384)).
  A denial reason flowed from the server into exception messages and the
  monitoring-mode warning with no control-byte stripping and no length bound,
  so a hostile or compromised AIM server could carry ANSI escapes that rewrite
  what a developer sees, or scroll the real message away. Decision-borne
  strings are sanitized where they enter `VerificationDecision` (total over
  any JSON value — a non-string reason is coerced, never raised on, because a
  constructor raise would turn an explicit DENY into UNKNOWN). The pre-decision
  console warnings, the JIT poll warning, the 401 exception message and the
  403 denial-detail parse sanitize at their own entry sites, and the console's
  free-text methods strip control bytes as defence in depth. C0 bytes other
  than newline and tab, DEL and the C1 range are removed; text over 500 chars
  is truncated with an explicit marker.

### Fixed

- **The registration panel no longer reports a trust score the server did not
  send** ([#390](https://github.com/opena2a-org/agent-identity-management/issues/390)).
  Three defects sat on this one path, each of which independently put a number on
  screen that nothing measured:
  - `client.py` defaulted to `0.85`, so an unscored agent was announced as
    `Trust Score: 85%` in the same form as a real reading.
  - The same line used `or` rather than a presence test, so a genuine server-sent
    `0` — a real, maximally-untrusted score — is falsy and fell through to that
    same fabricated 85%. Verified before fixing: a payload of
    `{"trust_score": 0}` printed `Trust: 85%`.
  - `AIMConsole._normalize_trust_score` mapped `None` to `0.0`, which rendered an
    unknown score as a measured, alarming red `0%` — and, because callers
    normalize before formatting, made the `N/A` branch in both formatters
    unreachable. That branch existed and looked like a control; nothing could
    reach it.

  An absent score now stays absent and prints `N/A`. A genuine `0` still prints
  `0%` and remains distinguishable from unknown.

- **Emoji removed from user-facing output**
  ([#391](https://github.com/opena2a-org/agent-identity-management/issues/391)).
  U+23F3 hourglass becomes U+25CB, the mark this console already uses for
  pending; U+26A0 warning sign (usually carrying U+FE0F, which forces colour
  emoji presentation) becomes the word `Warning:`, which most of those lines
  already carried; U+2139 is dropped, since the label states what it is. U+2713,
  U+2717 and U+25CB are the house style and are unchanged.

  Characters are named by codepoint here rather than shown, because the
  regression guard below scans this file too.

  The sweep went wider than the issue's table, which listed three codepoints.
  The same standard was being broken by U+2705 heavy check mark (23 uses), U+274C
  cross mark (12) and sixteen pictographs in the same files — fixing only the
  tabled three would have shipped a panel with house-style U+2713 and emoji
  U+2705 on adjacent lines. U+2705 and U+274C map to U+2713 and U+2717; the
  pictographs are dropped.

  Correction to the issue's framing: `docs/*.md` are described there as shipped.
  They are not — `MANIFEST.in` ships `VERSION`, `README.md`, `CHANGELOG.md`,
  `LICENSE` and `aim_sdk/*.py` only. They are swept anyway because they are
  public on GitHub, but no pip user was receiving them.

### Added

- A regression guard that enumerates the package and the prose from disk and
  fails on any emoji, rather than checking the sites that were once wrong.
  Checking only known-bad sites is how this class returns. The guard is
  property-based for the same reason: the first pass at this fix used a
  hand-written list of pictographs to strip, and the list was the defect — it
  missed ten others sitting in the same documents.

## [2.0.0] - 2026-08-11

**On the deprecation window.** `docs/VERSIONING.md` documents a mandatory N+1 minor
release — one version carrying a `DeprecationWarning` for a change before the
change itself lands — before any breaking removal. This release does not have
that precursor: there was no prior minor version warning that a previously
falsy-returning path would start raising. The exception is deliberate, not an
oversight. Every published version silently fails open on a denial; the fix
*is* the raise. Holding it for a warning-only release would mean shipping a
known fail-open bug for one more minor version to satisfy a process meant to
protect callers, which is the opposite of what the process is for. Treat this
as a named, one-time exception to the timeline in `docs/VERSIONING.md`, not a
change to the policy itself.

### Changed — read this before upgrading

**Strict mode now actually blocks. Check which enforcement mode your organization
is in before you upgrade** (dashboard: Settings → Security → Policies).

Before this release the organization's enforcement mode never reached the Python
SDK at all, so a strict-mode organization behaved exactly like a monitoring-mode
one and denied actions executed. Three things can therefore newly stop a running
agent, and all three are the policy you already configured finally taking effect:

1. **A denied action now raises, in EVERY enforcement mode — including
   monitoring, which is the default.** This is the change most likely to affect
   you, because monitoring is the schema default and every organization created
   before this release was backfilled to it.

   Monitoring mode governs what happens when AIM cannot give an answer, not what
   happens when AIM says no. A verification that could not be completed is logged
   and the action proceeds; an explicit denial blocks in every mode, because it is
   a decision AIM already made with your organization's enforcement mode in hand.

   Concretely: the backend applies your enforcement mode before it answers, so
   under monitoring a policy refusal is already converted to an approval and never
   reaches your process as a denial. What does still arrive as a denial is the
   short list the server refuses to override — an agent that is not found, an
   agent **marked compromised**, a capability lookup that failed — plus an
   administrator pressing Deny on a JIT request. Until this release the SDK
   overrode all of those a second time and ran the action anyway.

   If you rely on monitoring mode to observe without blocking, that still works
   for everything except an explicit denial. If an agent of yours is currently
   denied and executing regardless, it will now stop; grant the capability in the
   dashboard under Agents, or check the agent's status there.
2. **`AIM_STRICT_MODE=1`, `=yes` and `=on` now enforce.** They previously did
   nothing — only the literal `true` was recognised — while our own
   `demo_agent.py` accepted them and printed `Strict Mode: ENABLED -
   Unauthorized actions WILL BE BLOCKED`. If you set one of these believing the
   control was on, it is on now.
3. **`AIM_STRICT_MODE=false` no longer disables enforcement.** The variable is a
   ratchet: it can only raise enforcement, never lower it. It is ignored with a
   warning rather than being a hard error. Remove it from your configuration —
   it no longer does what `docs/ENV_CONFIG.md` previously described. Monitoring
   mode is not a way to run without enforcement either: it stops AIM blocking on
   verifications it could not complete, and does not stop it blocking on ones it
   refused. There is no setting that makes a denied action execute.

**The return value of `verify_capability()` keeps its shape in 2.0.0** on the
approved path. `result["verified"]`, `result["verification_id"]`,
`result["approved_by"]` and `result["expires_at"]` all keep working, and two keys
are added: `mode` and `mode_source`.

Two of those keys change *value* rather than shape. `approved_by` and
`expires_at` were read from the response as `approved_by` and `expires_at`, while
the backend has only ever sent `approvedBy` and `expiresAt`, so both were `None`
on every call in every published version. They now carry the values the server
sends. Code that treated them as always-absent — `if not result["approved_by"]`,
or a `None` check standing in for "auto-approved" — will take a different branch.

What *has* changed is that paths which previously returned a falsy dict now
raise. `verify_capability()` returns only when the action is permitted:

| Situation | Before | Now |
|---|---|---|
| Permitted | dict with `verified: True` | unchanged, plus `mode` / `mode_source` |
| Explicitly denied | raised `ActionDeniedError` | unchanged |
| 403 from the server | raised `AuthenticationError` | raises `ActionDeniedError` |
| 404, 429, 5xx, bad JSON | returned `{"verified": False, ...}` | raises `VerificationUnavailableError` |
| Timeout, connection, DNS, TLS | returned `{"verified": False, ...}` | raises `VerificationUnavailableError` |

The broadest compatible catch is `except AIMError`, which every SDK exception
still descends from.

#### One narrow break, stated plainly

If you run with `AIM_STRICT_MODE=true` and catch `PermissionError` around a
decorated function, an *unavailable* AIM no longer lands in that handler.
`VerificationUnavailableError` deliberately does not subclass `PermissionError`,
because a handler written for "AIM said no" must not silently absorb "AIM was
never asked". The action is blocked either way — only the label changed, and the
old label was false.

```python
# before
except PermissionError:
    ...
# after
except (PermissionError, VerificationUnavailableError):
    ...
```

#### Coming in 3.0.0

When AIM cannot be reached **and** this process has no cached enforcement mode,
the action currently executes and emits a `PendingEnforcementChange` warning. In
3.0.0 it will be blocked instead. To adopt that behaviour now, set
`AIM_STRICT_MODE=true`.

### Fixed

- **A denial arriving as HTTP 403 — the only wire form a real denial takes — was
  parsed as if it were an error envelope.** `CreateVerification` returns a full
  verification response with status 403: `id`, `denialReason`, `enforcementMode`.
  The SDK read a key called `error`, which that route has never sent, and nothing
  else. Three consequences, all fixed together because they share six lines:
  every production denial reached the developer as the literal string
  `insufficient permissions`, naming no policy and no capability; the
  organization's enforcement mode was resolved from a hard-coded `None` on the
  one response that states it; and the verification id was dropped, so
  `report_execution_status` returned at its first line and AIM never recorded
  that the blocked action was in fact blocked. Those execution columns are not
  read by anything today, so nothing was displaying the wrong answer; what was
  missing is the evidence itself, for every verification written by every
  published version.
- **The administrator's stated reason now reaches the developer on a JIT denial.**
  The polling path read `denial_reason`; `writeVerificationResponse` sends
  `denialReason`. A developer stopped by a human decision was told only
  `Action denied`, never why. Same root cause as the 403 above: the SDK read
  snake_case keys off a response whose fields are all camelCase. Test fixtures
  reproduced the wrong shape, which is why 165 tests could not see any of it.
- **An explicit denial no longer executes the action.** `ActionDeniedError` was
  not a `PermissionError`, and the decorators' only two handlers were
  `except PermissionError` and `except Exception` — so every denial fell into the
  fail-open branch and the wrapped function ran. No outage, no attacker, no
  misconfiguration required; this was the normal denial path in every published
  version. `ActionDeniedError` now also subclasses `PermissionError`, so the
  contract the README always documented is true. (Note that `PermissionError`
  descends from `OSError`, so a denial now also satisfies `except OSError`.)
- **The organization's enforcement mode now reaches the SDK.** The decorator read
  `enforcementMode` off the client's return value; no return site in `client.py`
  ever set that key, so it fell back to `"monitoring"` on every call and the
  dashboard setting had no effect. An absent or unrecognised mode is now
  `unknown`, never `monitoring` — a degraded upstream must not be able to emit
  the lenient value by failing.
- **Execution reports now actually send.** The decorator read the id as
  `verificationId`/`id` while the client emits `verification_id`, so
  `report_execution_status` returned early on a falsy id before building a URL.
  All eight call sites were fed by that one value: AIM had never received an
  execution report from the Python SDK. The report also now carries its own short
  timeout instead of inheriting the client's 30-second request timeout.
- **Which entry points send an execution report, and which do not.** Reporting runs
  from `@aim_verify` in `aim_sdk/decorators.py`, and therefore also from the four
  convenience wrappers `aim_verify_api_call`, `aim_verify_database`,
  `aim_verify_file_access` and `aim_verify_external_service`, each of which
  delegates to it. It does **not** run from `perform_action`, `track_action`,
  `require_approval`, or the separate `aim_verify` in
  `aim_sdk/integrations/langchain/decorators.py`. Those four resolve a decision and
  act on it without writing an execution row, so an integration built only on them
  produces no execution evidence in 2.0.0. Widening them is deferred deliberately
  rather than done piecemeal, so that all four change in one release under one
  contract.
- **A rate-limited verification no longer disables verification.** A 429 is a
  status ≥ 400, so it returned `verified: False` and the action ran — 101
  requests in a minute from the agent's own IP turned verification off for that
  IP, with no credentials and no exploit, and it also fired by accident behind a
  NAT. A 429 is now retried once or twice honouring `Retry-After`, and if no
  decision is obtained it is not permissive. A 5xx is never retried, because the
  server may have partly executed the request.
- **`@require_approval` works at all.** It called `console.show_jit_awaiting()`,
  `show_jit_denied()` and `show_jit_approved()`, none of which exist — the real
  names are `jit_waiting`, `jit_denied` and `jit_approved`. It raised
  `AttributeError` on its first line on every call, including an approval, so it
  had never executed a wrapped function. Fixing the crash exposed a defect it had
  been hiding: `@require_approval` printed "approved - executing..." whenever the
  wrapped function ran, which is also true when a monitoring-mode organization
  ran it after an explicit denial, and when the 2.0.0 warning-window cell above
  ran it unverified after a transport failure. Neither of those is an approval.
  It now prints "approved" only on an explicit `ALLOW`, and something honest
  otherwise. The waiting panel's "Approve in dashboard" link was also always
  `http://localhost:3000/...` regardless of the configured server; it now uses
  the client's own `aim_url`.
- **`aim_verify`'s auto-init failure is now an `AIMError`.** With no client
  passed and no `AIM_AGENT_NAME` to auto-initialize one, it raised a bare
  `ValueError`, which the SDK's own documented `except AIMError:` pattern does
  not catch. It is now `ConfigurationError`, the category already used for every
  other setup failure (missing credentials, bad URL).
- **The LangChain `@aim_verify` no longer runs tools unverified.** It never
  inspected the verification result, so a 404, 429, 5xx or timeout fell through
  to the execute path. It also converted every exception into `PermissionError`,
  telling developers they had been denied when AIM was unreachable.
- **`@track_action` and `@require_approval` no longer substitute a sentinel dict
  for your function's return value** when verification does not permit the
  action. A caller that ignored the return proceeded as if the work had happened.

### Added

- `aim_sdk.decision.VerificationDecision` — the typed three-state decision
  (`ALLOW` / `DENY` / `UNKNOWN`) carrying the enforcement mode and its source.
  Attached to `ActionDeniedError` and `VerificationUnavailableError` as
  `.decision`, `.mode` and `.mode_source`.
- `aim_sdk.exceptions.VerificationUnavailableError`.
- `aim_sdk.strict_mode` — one `AIM_STRICT_MODE` parser, used by the SDK and by
  `demo_agent.py`, accepting `true|1|yes|on` case-insensitively.
- A process-scoped, in-memory enforcement-mode cache with a 300-second TTL on a
  monotonic clock. It is never written to disk, a keyring, or any cross-process
  store: a persisted cache is forgeable in the lenient direction.

### Deprecated

- The dict return from `verify_capability()`; 3.0.0 returns a
  `VerificationDecision`. Emits a `DeprecationWarning` once per process.
- The `verified` key, superseded by the decision's outcome.

### Security

All previously published versions of `aim-sdk` are affected by the denial
fail-open. Upgrading to 2.0.0 is the fix. If you cannot upgrade, set
`AIM_STRICT_MODE=true`.

Note the honest bound: everything the SDK enforces is inside the agent's own
process, and is therefore advisory with respect to the agent. This release means
an honest operator's strict-mode organization actually blocks denied actions, a
compromised agent still routing through the SDK is blocked, and the AIM console
stops implying enforcement it never performed. It does **not** enforce anything
against a hostile operator.

### Known issues

Two defects found by this release's fresh-user walkthrough are **not** fixed here.
Both reproduce identically on 1.24.1, so neither is introduced by 2.0.0 and
upgrading does not expose you to anything you were not already exposed to. Both
are scheduled for **2.0.1**. They are recorded here rather than left silent
because this release's own subject matter is the SDK reporting things that were
not true.

- **The registration panel prints `Trust Score: 85%` when the server sent no trust
  score at all** ([#390](https://github.com/opena2a-org/agent-identity-management/issues/390)).
  `client.py` reads `credentials.get("trust_score") or credentials.get("trustScore", 0.85)`,
  so an absent field renders as `85%` in the same form as a measured value, on the
  first thing a new user sees. The `or` also means a real server-sent score of `0`
  displays as `85%`. The SDK's own stored record is correct — `~/.aim/agents/<name>.json`
  holds `"trust_score": null` for the same registration — so the display contradicts
  the file. Treat the trust score shown at registration as unverified; read it from
  the dashboard instead.
- **Emoji-class characters in console output**
  ([#391](https://github.com/opena2a-org/agent-identity-management/issues/391)).
  U+23F3 in the `@require_approval` panel, U+26A0 on warning lines, and U+2139 on
  the API-key mode line. Cosmetic; no behavioural effect.

## [1.24.1] - 2026-06-08

### Fixed

- **CLI login no longer doubles the "Token exchange failed:" prefix.** `aim-sdk
  login` printed `Token exchange failed: Token exchange failed: 405` when the
  token endpoint returned a non-JSON error — the classic symptom of pointing the
  SDK at the dashboard URL (`https://aim.opena2a.org`) instead of the API base.
  The message now surfaces the server's `error_description` and, on a non-JSON
  response, a one-line hint that the URL should be the AIM API base. (#286)

### Internal

- Guard `tests/test_sdk_verification.py`'s manual smoke script under `__main__`
  so it no longer makes a network call during `pytest` collection. (#283)

## [1.24.0] - 2026-06-06

### Added

- **Causal-denial telemetry (opt-in, parity with the TypeScript SDK).** A new
  `aim_sdk.telemetry` package joins the injection cause (detection inference),
  the classified intent, and the authorization outcome (enforcement fact) into
  one correlated record so you can see *why* an action was blocked. Off by
  default, best-effort, and off the enforcement critical path — it never changes
  a verdict and never raises into `verify_capability`.
  - `AIMClient(..., telemetry={...})` enables it via two independent opt-ins:
    `telemetry.enabled` (stage 1: capture records locally) and
    `telemetry.relay.enabled` (stage 2: share anonymized indicators).
  - `verify_capability(..., telemetry={"intent": ..., "detection": ...})` is the
    seam the injection detector / intent classifier populate.
  - Two tiers: the full record stays local (`~/.opena2a/correlated-events.jsonl`,
    authoritative); only an anonymized `SharedIndicator` may leave, carrying no
    identifiers, payloads, paths, credentials, or the correlation key. The relay
    egress-validates technique fields (`^T-\d{1,6}$` + source allowlist +
    confidence in [0,1]) and uploads only `denied_injection_attempt` indicators
    to the Registry's public, count-only endpoint.
  - New public exports: `CorrelationJoiner`, `CorrelatedRelay`,
    `EnforcementInput`, `IntentInput`, `DetectionInput`, `build_correlated_record`,
    `to_shared_indicator`, `interim_technique_fields`, `mint_correlation_id`.
  - `AIMClient.close()` / `close_telemetry()` stop the managed joiner and relay
    threads.
  - Hardening: the sensor-token salt file is read/created with `O_NOFOLLOW` +
    `fstat()` on the held descriptor and an `O_EXCL` re-create, so a pre-planted
    symlink at the salt path is never followed (no TOCTOU, no write-through). The
    deferred-write queue drops + counts (`dropped_writes`) under extreme
    saturation instead of falling back to a synchronous write, keeping disk
    latency strictly off the enforcement path.

## [1.23.0] - 2026-06-01

### Added (provisional)

- **AAP grant surface — experimental.** `@perform_action(grant="grant://...")`,
  plus `BrokerClient`, `GrantSession`, `current_grant`, `BrokerGrantError`, and
  `GrantDeniedError` (#266). An agent references a grant; the Secretless broker
  verifies the ATX, authorizes, resolves a scoped credential, performs the
  operation in an ephemeral worker, and returns only the result — no credential
  value or backend identifier ever enters the agent process. **Provisional:** the
  Agent Authorization Protocol is at spec v0.1, so this surface and the broker
  wire format it depends on may change in a future minor release without a major
  bump. Opt-in only: omitting `grant` leaves all existing behavior unchanged.

### Changed

- **Richer `aim-sdk login` output and install-first README quickstart** (#270).
- **OAuth login callback pages redesigned to match the AIM dashboard.** The
  browser "Login successful" / "Login failed" pages now use the OpenA2A logo,
  the dashboard's blue-600-on-white design language, and inline SVG status icons
  (replacing the previous purple-gradient card and emoji). Fully self-contained
  (logo inlined as a data URI; the Inter webfont degrades to the system stack
  offline).
- **`aim-sdk` terminal output is now emoji-free** and reads as clean professional
  prose (`login`, `logout`, `status`); exit codes carry the success/failure signal.
- **`aim-sdk version` output unified** with the new flag: both print `aim-sdk X.Y.Z`.

### Fixed

- **`tests/` no longer ships inside the wheel.** `setup.py` now passes
  `find_packages(exclude=["tests", "tests.*"])`, so the published distribution
  contains only the `aim_sdk` package.

- **`aim-sdk --version` / `aim-sdk -V`** now print the SDK version at the top
  level, complementing the existing `aim-sdk version` subcommand.

## [1.22.1] - 2026-06-01

First PyPI release since 1.21.0 (1.22.0 was prepared but never published), so
this release also carries the 1.22.0 changes listed below.

### Fixed

- **`register_agent` no longer doubles the error prefix.** When the backend
  rejected a registration, the message read `Registration failed: Registration
  failed: <reason>` because the outer handler re-wrapped a `ConfigurationError`
  the inner helper had already raised. `AIMError` subclasses are now re-raised
  unwrapped; generic exceptions are still wrapped exactly once. Regression tests
  added in `tests/test_register_agent.py`.
- **`aim-sdk login` callback pages now render UTF-8 correctly.** The local OAuth
  callback responses sent `Content-Type: text/html` with no charset, so browsers
  decoded the page as Latin-1 and the `✓` showed up as `âœ…`. Both the success
  and error responses now send `text/html; charset=utf-8`.

## [1.22.0] - 2026-05-25

### Security

- **Verification polling now Ed25519-signed and agent-scoped** (#160). The SDK route
  `GET /api/v1/sdk-api/verifications/<id>` was unauthenticated and returned the full
  verification event to any holder of the UUID, regardless of organization. The SDK
  now signs each poll with three required headers: `X-AIM-Agent-ID`,
  `X-AIM-Timestamp`, `X-AIM-Signature`. The backend rejects requests that lack the
  signature, fall outside a +/-5-minute clock-skew window, or reference a
  verification owned by a different agent (404, no existence oracle).

### Breaking

- **`_wait_for_approval` requires `self.signing_key` and `self.agent_id`.** Agents
  created without an Ed25519 keypair can no longer poll for verification approval;
  the SDK raises `VerificationError` with recovery guidance instead of issuing
  unauthenticated requests. The normal `secure(...)` / cached-credentials paths
  already provide both; only ad-hoc clients that constructed `AIMClient` without
  registering keys are affected.
- **`api_key`-only consumers can no longer poll the SDK verification route.**
  Pre-1.22 SDKs polled `GET /api/v1/sdk-api/verifications/<id>` with `X-API-Key`
  only; this endpoint is now signature-authed (defect #160 closure) and the
  API-key fallback no longer satisfies it. Migration:
    1. Register the consumer as an AIM agent (see `secure(...)` quickstart).
       This generates an Ed25519 keypair that `_wait_for_approval` will use
       automatically.
    2. If the consumer must remain API-key-only (CI service account, etc.),
       call `/api/v1/verifications/<id>` (the JWT-authed, org-scoped route)
       instead of the SDK route. Note the JWT route requires a user-session
       JWT, not an API key, so a dashboard auth flow is required.
  CI consumers that previously held `api_key` AND polled via `verify_action`
  with a non-`None` `_wait_for_approval` timeout will now raise
  `VerificationError` on `pending` responses.

### Fixed
- **Stale agent credentials no longer trigger silent re-registration** (#178). When the
  cached `~/.aim/agents/<name>.json` references an agent that no longer exists in the
  backend, the SDK now raises `StaleCredentialsError` with the local file path and
  recovery steps instead of deleting the file and registering a fresh agent under the
  same name. Re-registering against admin-curated agent state (status, trust score,
  granted capabilities, MCP grants) was wiping that state without notice; recovery is
  now an explicit operator action.
- **Bundled-SDK install path now prints a visible adoption warning** (#178). When the
  SDK adopts credentials from a bundled `.aim/sdk_credentials.json` (e.g., a fresh
  dashboard download in the working directory), the install prints which truncated
  token ID it is adopting and the long-term home path so operators can see when stale
  bundled tokens are entering the home store.

### Changed
- **Token rotation message names the prior refresh token as revoked** (#174). The
  print after a successful rotation now reads `Token rotated successfully — old
  refresh token revoked (new id: <8-char-prefix>...)` so the cause of out-of-band
  copies failing on the next refresh is obvious.
- **`print_token_expired_error` explains the root cause and surfaces the local
  path** (#174). The error text names refresh-token rotation as the most common
  cause, lists the four concrete situations that trigger it, and includes the exact
  `rm <path>` command instead of pointing the user at the dashboard UI alone.

### Removed
- **Encrypted shadow credential file** (`~/.aim/sdk_credentials.encrypted`, audit
  #12). The shadow file written alongside the JSON store became a write-only
  secondary source that no other code path read, and operators had to delete it
  manually to recover from corruption. `OAuthTokenManager` now treats
  `~/.aim/sdk_credentials.json` (mode 0600) as the single source of truth and
  performs a one-time migration on first instantiation: any leftover `.encrypted`
  file is decrypted into the JSON store and removed, or — if decryption fails —
  preserved in place so the operator can recover manually. The `use_secure_storage`
  and `allow_plaintext_fallback` constructor parameters are preserved for ABI
  stability and ignored.

### Added
- **`StaleCredentialsError`** (subclass of `ConfigurationError`). Raised from
  `register_agent()` when cached agent credentials reference an agent missing from
  the backend.

### Planned
- JavaScript/TypeScript SDK
- GraphQL API support

## [1.21.0] - 2026-02-03

### Changed
- **CLI Authentication**: Replaced password prompt with OAuth 2.0 + PKCE browser flow
  - `aim-sdk login` now opens browser for secure authentication (Google, etc.)
  - Uses PKCE (Proof Key for Code Exchange) per RFC 8252 - same as AWS CLI
  - No more password prompts or browser permission dialogs
  - Browser redirects directly to localhost - seamless experience

### Security
- PKCE prevents authorization code interception attacks
- State parameter prevents CSRF attacks
- Authorization codes are one-time use with 5-minute TTL

## [1.8.0] - 2025-12-10

### Added
- **Smart MCP Attestation System**: Intelligent, automatic attestation that builds trust in MCP servers for supply chain security
  - Attestations triggered on: first use, new tool, stale cache (>24h), capability drift
  - Zero friction - attestations happen automatically without developer effort
  - Caching prevents redundant attestations while maintaining security
- **`AttestationCache` Class**: New persistent cache for tracking attestation state
  - `should_attest()` - Determines if attestation is needed based on triggers
  - `record_attestation()` - Records successful attestations
  - `record_tool_usage()` - Tracks tool usage for analytics
  - `get_supply_chain_report()` - Generates supply chain analytics report
- **Capability Drift Detection**: Automatically detects when MCP servers change their tools
  - Severity levels: low (tools added), medium (tools removed), high (>30% change)
  - Triggers re-attestation when drift is detected
- **Supply Chain Analytics**: Track MCP tool usage for security visibility
  - `get_mcp_supply_chain_report()` - Local analytics for this agent
  - `report_mcp_supply_chain()` - Sync analytics to backend for dashboard
  - Usage patterns visible in AIM dashboard
- **Enhanced `use_mcp_tool()`**: Now includes smart attestation
  - `auto_attest` parameter (default: True) - Enable smart attestation
  - `force_attest` parameter - Force attestation even if cached
  - Returns attestation info and tool usage stats
- **New AIMClient Methods**:
  - `use_mcp_tool()` - Convenience method with smart attestation
  - `get_attestation_cache()` - Access attestation cache
  - `get_mcp_supply_chain_report()` - Local supply chain report
  - `report_mcp_supply_chain()` - Sync to backend

### Changed
- `use_mcp_tool()` now automatically triggers attestations when appropriate
- Tool usage is tracked locally for supply chain analytics
- Documentation updated with smart attestation and supply chain features

### Supply Chain Security Value
This release positions AIM as an MCP supply chain security platform:
- **Trust Graphs**: Visualize which agents trust which MCP servers
- **Anomaly Detection**: Alert when MCP servers suddenly change capabilities
- **Compliance**: Full audit trail of tool usage across the organization
- **Risk Assessment**: Identify high-risk MCP servers (low attestations, frequent changes)

## [1.7.0] - 2025-12-10

### Added
- **Dynamic MCP Capability Discovery**: SDK now automatically queries MCP servers for their actual tools, resources, and prompts using the official MCP protocol (`tools/list`, `resources/list`, `prompts/list`)
  - No more hardcoded capability lists - capabilities are discovered at runtime
  - Works with any MCP server, not just known ones
  - `discover_mcp_capabilities()` function for programmatic discovery
  - `auto_detect_mcps(discover_tools=True)` for full detection with tools
- **Auto-Attestation on Registration**: When agents register MCP servers via `secure()` or `register_mcp()`, the SDK automatically:
  - Discovers actual capabilities from the MCP server
  - Creates a cryptographically signed attestation
  - Submits the attestation to build trust in the MCP server
- **New `attest_mcp()` method**: Manual attestation API on AIMClient for attesting to MCP server capabilities
- **MCP Connection Recording**: `use_mcp_tool()` function to record agent-MCP connections

### Changed
- Removed hardcoded `KNOWN_MCP_CAPABILITIES` lookup table - capabilities now always discovered dynamically
- `_detect_mcp_capabilities_from_config()` now uses dynamic discovery exclusively
- MCP server registration flow now includes automatic attestation with discovered capabilities

### Improved
- MCP integration documentation with new sections on dynamic discovery and auto-attestation
- README updated with MCP capability discovery examples

## [1.4.0] - 2025-12-05

### Added
- **Server-Side Enforcement Control**: Admins can now configure enforcement mode in the dashboard UI (Settings → Security → Policies)
  - **Monitoring Mode** (default): Actions are logged but allowed to proceed when verification fails
  - **Strict Mode**: Actions are blocked immediately when verification fails
- SDK now respects the organization's enforcement mode from the backend
- Environment variable `AIM_STRICT_MODE` can still override the backend setting for testing purposes

### Changed
- Verification response now includes `enforcementMode` field to inform SDK of the organization's setting
- `@aim_verify` decorator uses backend enforcement mode by default, with env var as optional override
- Dashboard Policies page now shows enforcement mode toggle with clear explanations of each mode

### Fixed
- SDK no longer requires `AIM_STRICT_MODE` environment variable - it now reads the setting from the backend

## [1.3.0] - 2025-12-05

### Added
- **Execution Status Reporting**: SDK now reports whether decorated functions actually executed back to the backend
  - New `report_execution_status()` method on `AIMClient`
  - Decorators automatically report execution status after function calls
  - Dashboard shows accurate status: "Executed", "Blocked", or "Executed despite denial"
- **Strict Mode Documentation**: Comprehensive documentation for `AIM_STRICT_MODE` environment variable
  - Explains difference between monitoring mode (default) and strict mode (production)
  - Code examples for production deployments

### Changed
- `@aim_verify` decorator now tracks and reports:
  - Whether function was executed
  - Whether strict mode was enabled
  - Any execution errors that occurred
- Alert detail panel now displays execution status with clear messaging

### Fixed
- Dashboard alert messaging now accurately reflects what actually happened (blocked vs allowed)

## [1.2.4] - 2025-12-04

### Fixed
- OAuth token refresh flow fixed (camelCase consistency)
- Improved error handling in credential storage

## [1.1.0] - 2025-12-03

### Added
- **MCP Server Registration**: `agent.register_mcp()` method for programmatic MCP server registration
- **Capability Requests**: `agent.request_capability()` method for requesting additional capabilities
- **JIT Access Demo**: Interactive demo showing just-in-time access request workflows
- **Consolidated Demo Agent**: Single `demo_agent.py` with comprehensive feature demonstrations
- **Standardized Capability Format**: All capabilities now use `namespace:action` format consistently

### Changed
- Demo agents consolidated into single interactive `demo_agent.py`
- Improved credential migration and stale credential handling
- Enhanced demo UX with better credential discovery and alerts

### Fixed
- Trust score nil pointer handling
- Duplicate plaintext file deletion in credential migration
- Stale encrypted credentials cleared on new SDK install

## [1.0.0] - 2025-11-06

### Added
- **New Decorators**:
  - `@agent.perform_action()` - Verify and track actions with optional JIT access for admin approval
  - `@agent.require_approval()` - Require admin approval before executing critical actions
- **Versioning**: SDK download filename now includes version (e.g., `aim-sdk-python-v1.0.0.zip`)
- **VERSION File**: Single source of truth for SDK version at `/sdk/python/VERSION`
- **Documentation**:
  - Comprehensive decorator documentation with examples
  - Versioning strategy guide at `docs/VERSIONING.md`
  - Updated README with new decorator usage patterns

### Fixed
- **Critical: Ed25519 Signature Verification**:
  - Fixed JSON formatting mismatch between Python SDK and Go backend
  - SDK now uses `json.dumps(sort_keys=True, separators=(', ', ': '))` for consistent message format
  - Backend uses custom `customJSONFormat()` function to match Python exactly
  - Resolves signature verification failures caused by:
    - `"resource": null` vs `"resource": ""` differences
    - Space placement inconsistencies in JSON serialization
- **Critical: Credential Encryption**:
  - Fixed encryption bug in `secure_storage.py` when storing agent credentials
  - Credentials now properly encrypted and saved to `~/.aim/credentials.json`
- **API Key Middleware**:
  - Fixed middleware blocking verification endpoints with 401 errors
  - Verification endpoints now correctly use Ed25519 signature auth instead of API keys
- **Public Key Handling**:
  - Backend now accepts and uses SDK-provided public keys during agent registration
  - Fixes public key mismatch errors where backend was generating its own keys
- **Decorator Response Parsing**:
  - Fixed `AttributeError: 'dict' object has no attribute 'approved'`
  - Decorators now use `dict.get("verified", False)` for response parsing

### Changed
- **SDK Download Format**: Filename changed from `aim-sdk-python.zip` to `aim-sdk-python-v{version}.zip`
- **Agent Registration**: Backend now supports optional `public_key` field in `CreateAgentRequest`
- **Verification Flow**: Improved decorator implementation with proper error handling

### Deprecated
- `@agent.track_action()` - Use `@agent.perform_action()` instead
  - Will be removed in v2.0.0
  - Deprecation warnings added in v1.6.0

## [0.9.0] - 2025-11-05 (Pre-release)

### Added
- Initial SDK implementation
- `secure()` function for zero-config agent registration
- Ed25519 cryptographic signing
- Automatic capability detection
- MCP server detection from Claude Desktop config
- OAuth token management
- Basic decorator support with `@agent.perform_action()`

### Security
- Ed25519 cryptographic signatures for all agent communications
- Secure credential storage using OS keyrings
- Encrypted private key storage
- SHA-256 API key hashing

---

## Version Support

| Version | Status | Support Level | End of Support |
|---------|--------|---------------|----------------|
| 1.0.x   | ✓ Current | Full support | N/A |
| 0.9.x   | Warning: Pre-release | No support | Immediately |

## Migration Guides

### Upgrading from 0.9.x to 1.0.0

**Breaking Changes**: None

**New Features**:
- `@agent.perform_action()` decorator for verification and tracking
- `@agent.require_approval()` decorator for critical actions

**Recommended Usage**:

```python
# Standard usage - all actions verified and tracked
@agent.perform_action(risk_level="low", resource="database:users")
def query_users():
    return db.query("SELECT * FROM users")

# Critical actions with JIT access - requires admin approval
@agent.perform_action(risk_level="critical", jit_access=True, resource="database:users")
def delete_all_users():
    return db.execute("DELETE FROM users")  # ○ Pauses until admin approves

# Alternative: use @require_approval for critical actions
@agent.require_approval(risk_level="critical", resource="database:users")
def purge_data():
    return db.execute("TRUNCATE TABLE users")
```

**Action Required**:
- ✓ No immediate action required - 0.9.x code continues to work
- Warning: Update decorators to new style before v2.0.0 (recommended)

---

## Reporting Issues

Found a bug? Please report it:
- **GitHub Issues**: https://github.com/opena2a-org/agent-identity-management/issues
- **Email**: info@opena2a.org
- **Discord**: https://discord.gg/uRZa3KXgEn

---

**Last Updated**: 2025-12-10
