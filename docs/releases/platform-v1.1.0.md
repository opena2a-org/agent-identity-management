# AIM Platform 1.1.0 release notes

Draft notes for the `platform-v1.1.0` tag of the AIM platform (backend and dashboard). They are written from
`CHANGELOG.md`, section `[Unreleased]`, at commit `c694174a`; an entry added after that commit is not in these notes,
except a Security heading, which the security table cites, and the entry *Security: a password change ends every
session signed in before it, and the API enforces a required password change*, which a change that needs action also
describes.
Each item names the [CHANGELOG.md](../../CHANGELOG.md) heading that carries its detail, written as
its kind, a colon and its text; the changelog itself separates the kind from the text with a dash.

Three values exist only once the tag is published. They appear below as named placeholders:

| Placeholder | Value |
|-------------|-------|
| `<RELEASE_URL>` | the release page of the `platform-v1.1.0` tag |
| `<AIM_SERVER_DIGEST>` | the index digest of `aim-server:1.1.0` |
| `<AIM_DASHBOARD_DIGEST>` | the index digest of `aim-dashboard:1.1.0` |

## Artifacts

- Release page: `<RELEASE_URL>`.
- Container images for `linux/amd64` and `linux/arm64`, built and signed by the publish workflow from the tag:
  - `ghcr.io/opena2a-org/aim-server:1.1.0` and `docker.io/opena2a/aim-server:1.1.0`, index digest `<AIM_SERVER_DIGEST>`
  - `ghcr.io/opena2a-org/aim-dashboard:1.1.0` and `docker.io/opena2a/aim-dashboard:1.1.0`, index digest `<AIM_DASHBOARD_DIGEST>`

To check an image's signature and its build provenance attestation:

```bash
cosign verify ghcr.io/opena2a-org/aim-server@<AIM_SERVER_DIGEST> \
  --certificate-identity https://github.com/opena2a-org/agent-identity-management/.github/workflows/docker-publish.yml@refs/tags/platform-v1.1.0 \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

curl -s https://api.github.com/repos/opena2a-org/agent-identity-management/attestations/<AIM_SERVER_DIGEST>
```

The SDKs release on their own tags and are not part of this release. The Java SDK is built from source in `sdk/java`;
see [For Java SDK users](#for-java-sdk-users).

## What changed for operators

### Changes that need action

A deployment that pulls the `1` or `latest` tag receives these changes with 1.1.0; see [Pinned tags](#pinned-tags).

- **Prometheus scrapes.** `/metrics` leaves the API port for its own listener at `METRICS_LISTEN_ADDR`, default
  `127.0.0.1:9464`. A scrape job that reads `/metrics` on port 8080, or through the dashboard without a token, stops
  receiving data. A listener on any address other than loopback requires `METRICS_AUTH_TOKEN`, and without it the
  backend refuses to start. CHANGELOG: *Changed, breaking: `/metrics` moves off the API port to its own listener,
  loopback by default*.
- **API listen address.** Outside a container the API listens on `127.0.0.1` unless `HOST` is set; 1.0.0 listened on
  every address. The server counts as in a container when `/.dockerenv` or `/run/.containerenv` exists or
  `KUBERNETES_SERVICE_HOST` is set, and the `aim-server` image sets `HOST=0.0.0.0`. A server started from the binary
  on a host answers only on that host until `HOST` names the address other hosts reach, such as `HOST=0.0.0.0`. The
  last line of the startup summary names the address. CHANGELOG: *Security: the API server listens on 127.0.0.1
  outside a container unless `HOST` is set (#757)*.
- **OpenTelemetry and Prometheus names.** An OTLP endpoint written as `host:port` whose host is not `localhost`, in
  `127.0.0.0/8` or `::1`, connects with TLS unless `OTEL_EXPORTER_OTLP_INSECURE=true`; 1.0.0 always connected in
  plaintext. For a collector that serves plaintext on another host, set `http://host:port` or
  `OTEL_EXPORTER_OTLP_INSECURE=true`. The gauge `aim_mcp_servers_total` is served as `aim_mcp_servers`, and a query,
  alert or dashboard on the old name matches nothing until it uses the new one. CHANGELOG: *Changed: every HTTP
  request emits an OpenTelemetry server span, the OTLP exporters choose TLS as the OTLP specification defines, and the
  gauge `aim_mcp_servers_total` is renamed `aim_mcp_servers`*.
- **Secrets the server checks at start.** The API refuses to start without `KEYVAULT_MASTER_KEY` unless `ENVIRONMENT`
  is `development` or `test`, refuses a `KEYVAULT_MASTER_KEY` of 32 zero bytes, and refuses the `JWT_SECRET`
  placeholder from `.env.example`. The refusal for a missing key names one remedy, the key and how to generate it.
  Agent private keys are encrypted under `KEYVAULT_MASTER_KEY`, so set the value 1.0.0 ran with. A 1.0.0 server that
  ran without the variable outside `production` generated a new key at every start, and agent keys stored under an
  earlier generated key cannot be decrypted; rotating those agents' credentials replaces them. CHANGELOG: *Changed:
  the API refuses to start without `KEYVAULT_MASTER_KEY` unless `ENVIRONMENT` is `development` or `test`*; *Security:
  the server refuses a `KEYVAULT_MASTER_KEY` of 32 zero bytes*; *Security: the server refuses to start with the
  `JWT_SECRET` placeholder from `.env.example`*; *Security: the startup refusal for a missing `KEYVAULT_MASTER_KEY`
  names only the key*.
- **Rate limits in development.** The general and strict rate limits now apply when `ENVIRONMENT` is `development`.
  CHANGELOG: *Security: the rate limits no longer loosen when ENVIRONMENT is development*.
- **Stored agent private keys.** At its first start, 1.1.0 re-encrypts each stored agent private key bound to its own
  agent row, under a data key of its own, and the request path refuses the earlier format. Stop every 1.0.0 instance
  before the first 1.1.0 instance starts. 1.0.0 cannot read the new format, so rolling back after that start leaves
  server-generated agent keys unreadable until the agents' credentials are rotated; take a database backup first.
  CHANGELOG: *Security: a stored agent private key decrypts only in the agent row it was written for*; *Changed: each
  agent private key the server stores is encrypted under a data key of its own, which is stored encrypted under
  `KEYVAULT_MASTER_KEY`*, and
  [Database migrations and the application role](../DEPLOYMENT.md#database-migrations-and-the-application-role).
- **Sessions and SDK credentials issued by 1.0.0.** 1.1.0 accepts only session and refresh tokens that carry a token
  type, and 1.0.0 mints none. After the upgrade every dashboard user signs in again, and an SDK install that holds
  credentials issued by 1.0.0 needs new ones. CHANGELOG: the `### Security` list entry that begins "Session tokens
  carry a type".
- **Downloaded SDK credentials.** A downloaded SDK's credential ends `SDK_TOKEN_FAMILY_MAX_AGE` after its download,
  `8760h` (365 days) by default, however often it is refreshed. A refresh past that is answered 401, and the holder
  downloads a new SDK with a sign-in. `GET /api/v1/sdk/download`, `POST /api/v1/auth/sdk/recover`,
  `POST /api/v1/oauth/device/approve` and `POST /api/v1/api-keys` answer 401 to an access token minted by refreshing
  an SDK-download token, so a downloaded SDK's credential cannot download, recover or approve a new credential or
  create an API key; that takes a sign-in. CHANGELOG: *Changed: a downloaded SDK's credential ends
  `SDK_TOKEN_FAMILY_MAX_AGE` after its download, and a credential-minting route needs a sign-in's access token*;
  *Security: creating an API key requires a live sign-in session*.
- **Password changes end sessions and SDK credentials.** A password change or reset ends every session signed in
  before it, and every SDK-download token minted before it: their tokens are answered 401, so an SDK install whose
  user changes or resets the password needs a new download. The user's list of SDK tokens shows those tokens
  revoked, with the reason `password_changed`, and SDK token recovery does not restore them. While an account must
  change its password (`force_password_change`), the auth middleware admits its session only to
  `GET /api/v1/auth/me` and `POST /api/v1/auth/change-password`; every other route behind the middleware, and
  `POST /api/v1/auth/refresh`, answers 403 with the code `passwordChangeRequired`. CHANGELOG: *Security: a password
  change ends every session signed in before it, and the API enforces a required password change*.
- **Sign-in through the `Authorization` header only.** Sign-in no longer sets `access_token` or `refresh_token`
  cookies, and the API no longer accepts them in place of the header. A sign-in ends 8 hours after it began by default
  (`JWT_SESSION_MAX_AGE`), however often it is refreshed. CHANGELOG: *Security: the API no longer uses cookies for
  sign-in*; *Security: the API accepts a sign-in only from the Authorization header*; *Security: a sign-in ends a
  fixed time after it began, however often it is refreshed*.
- **Verification events are written by the server.** `POST /api/v1/verification-events` answers every authenticated
  member with 403 and the code `verificationEventWriteNotAccepted`. The server's own verification paths still record
  events, and the read routes and `DELETE` are unchanged. CHANGELOG: the `### Security` list entry that begins
  "`POST /api/v1/verification-events` accepted a verification outcome reported by the caller".
- **Agent credentials.** A JWT is no longer accepted as an agent's ATC; the server accepts the CBOR ATC only. An agent
  credential acts only on its own agent ID on the SDK-API, detection and A2A agent and card write routes. CHANGELOG:
  *Removed: a JWT is no longer accepted as an agent's ATC*; *Security: an agent credential acts only on its own
  agent ID on the SDK-API, detection and A2A agent and card write routes*.
- **Agents in hybrid mode.** `PQCAgentMiddleware` admits an agent whose hybrid mode is on only by a request signed
  with both Ed25519 and ML-DSA (`X-Algorithm` `Ed25519+ML-DSA-44`, `Ed25519+ML-DSA-65` or `Ed25519+ML-DSA-87`). The
  routes that verify an Ed25519 signature alone answer such an agent 401 with the `reasonCode`
  `hybridSignatureRequired`: the routes behind `Ed25519AgentMiddleware`, `POST /api/v1/oauth/token`,
  `POST /api/v1/sdk-api/verifications`, `GET /api/v1/sdk-api/verifications/:id` and `POST /api/v1/a2a/verify`.
  Turning hybrid mode on takes the manager or admin role. CHANGELOG: *Security: `PQCAgentMiddleware` admits an agent
  in hybrid mode only by a request signed with both Ed25519 and ML-DSA, and `Ed25519AgentMiddleware` refuses it*;
  *Security: the token endpoint and the SDK verification routes refuse an agent in hybrid mode that signs with
  Ed25519 alone (#740)*; *Security: `POST /api/v1/a2a/verify` reports a request from an agent in hybrid mode signed
  with Ed25519 alone invalid (#748)*; *Changed: turning on hybrid mode takes the manager or admin role*.
- **Webhooks.** `POST /api/v1/webhooks` and `PUT /api/v1/webhooks/:id` answer 400 to a body key they do not take and to
  an event outside the server's list, both of which 1.0.0 accepted. The create answer carries the webhook's signing
  `secret` once, and `POST /api/v1/webhooks/:id/rotate-secret` stores a new one and returns it once. No answer of 1.0.0
  carried a webhook's secret, so rotate the secret of a webhook created on 1.0.0 before its receiver checks
  signatures. Each delivery carries `X-Webhook-Timestamp` and `X-Webhook-Signature: v1=<hex>`, the HMAC-SHA256 under
  the webhook's secret of `<timestamp>.<raw body>`; [Verifying a delivery](../API.md#verifying-a-delivery) gives the
  receiver's checks. CHANGELOG: *Changed: a webhook's signing secret reaches its owner once, can be rotated, and a
  subscription to an unknown event or with an unknown field is refused*; *Security: a webhook signature covers the
  time the attempt was sent, so a receiver can refuse a replayed delivery*.
- **First administrator.** `ADMIN_PASSWORD` seeds the first administrator and no longer resets a password that was
  changed. CHANGELOG: *Changed: `ADMIN_PASSWORD` seeds the first administrator and no longer resets a changed
  password*.
- **Dashboard addresses.** Compliance is at `/dashboard/compliance`, and `/dashboard/admin/compliance` answers with a
  308 redirect to it. API keys and SDK tokens are listed at `/dashboard/credentials`. CHANGELOG: *Changed: the
  dashboard's Compliance and Credentials pages have new addresses*.

### Added for operators

- `aim-server migrate` applies the pending migrations and exits, and can provision a database role for the server that
  is not a superuser. CHANGELOG: *Added: `aim-server migrate` applies the migrations and can provision a role that is
  not a superuser*.
- Each server signing purpose has its own key, set by `AIM_SIGNING_KEY_CARD_ATTESTATION` and
  `AIM_SIGNING_KEY_ATC_ISSUER` or derived from `KEYVAULT_MASTER_KEY` when unset, and the public keys are published at
  `/.well-known/jwks.json`. CHANGELOG: *Changed: each server signing purpose has its own key, and the public keys are
  published*.
- Audit record chains: admins can read the state of their organization's chain, and `aim-breakglass chain status`
  reads it for the operator. CHANGELOG: *Added: admins can read the state of their organization's audit record
  chain*; *Added: `aim-breakglass chain status`, the operator's read of an organization's audit record chain*.
- CHANGELOG: *Added: the server forwards every audit log entry it writes to a Splunk HTTP Event Collector (#657)*.
- With SIEM forwarding configured, failed sign-ins, FGA denials, alerts and successful secrets management requests
  reach the SIEM too. `/health/ready` reports the SIEM under `dependencies.siem`, and `/metrics` carries
  `aim_siem_events_sent_total`, `aim_siem_events_failed_total` and `aim_siem_events_dropped_total`;
  [SIEM Forwarding](../DEPLOYMENT.md#siem-forwarding) states the loss bound. CHANGELOG: *Added: failed sign-ins and
  FGA denials reach the SIEM next to the audit log entries*; *Added: alerts and secrets management requests reach the
  SIEM*; *Added: a refused or unreachable SIEM shows in `/health/ready` and `/metrics`, and the loss bound is stated*.
- Every HTTP request other than `/metrics` emits an OpenTelemetry server span, and the `fga.authorize` spans carry
  `gen_ai.agent.id`. `OTEL_EXPORTER_OTLP_CERTIFICATE`, `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` and
  `OTEL_EXPORTER_OTLP_CLIENT_KEY` name the CA that verifies the collector and a client certificate and its key, and an
  endpoint in URL form now exports; [Observability](../DEPLOYMENT.md#observability) gives the endpoint forms. CHANGELOG:
  *Changed: every HTTP request emits an OpenTelemetry server span, the OTLP exporters choose TLS as the OTLP
  specification defines, and the gauge `aim_mcp_servers_total` is renamed `aim_mcp_servers`*; *Added: the
  `fga.authorize` spans carry `gen_ai.agent.id`, the AIP identifier of the agent AIM resolved*.
- The server delivers events to the active webhooks subscribed to them. A delivery makes at most 5 retries, and at
  most 1,000 deliveries are pending at once, 100 for one webhook; the changelog entry names the events not sent yet.
  CHANGELOG: *Added: the agent and API key services send agent, trust score and API key webhook events (#672)*;
  *Added: the server delivers agent, trust score, alert and API key events to subscribed webhooks, within limits on
  retries and pending deliveries (#672)*.
- An organization's alerts reach Slack, PagerDuty, a Microsoft Teams workflow and email addresses through its alert
  channels, each with a severity floor. Members, managers and admins manage them through `/api/v1/alert-channels` and
  the dashboard's Alert channels tab (`/dashboard/alert-channels`), and a test message checks a channel; a PagerDuty
  test opens an alert on the PagerDuty service. An email channel sends through the server's email service, which
  [Email](../DEPLOYMENT.md#email) describes with its three providers; [Alert channels](../API.md#alert-channels) gives
  the routes, the message formats and the retry rule. Migration `131_alert_channels.sql` creates the table, and
  `132_alert_channels_teams_email.sql` admits the Teams and email kinds. CHANGELOG: *Added: an organization's alerts
  reach Slack and PagerDuty in each service's format, with a severity floor per channel*; *Added: alert channels also
  deliver to a Microsoft Teams workflow and to email addresses*; *Added: the dashboard adds, edits, tests and deletes
  alert channels*; *Changed: a message the `smtp` email provider sends carries a `Message-ID`, STARTTLS and
  `AUTH PLAIN` are tested against a server, and the email providers are documented*.
- `KEYVAULT_MASTER_KEY` can be rotated. `KEYVAULT_MASTER_KEY` is the current key and `KEYVAULT_MASTER_KEY_ID` its id
  (1 when unset); each `KEYVAULT_MASTER_KEY_<id>` adds an earlier key under its id, which the server decrypts under
  and never encrypts under. At each start with more than one key in the ring, a sweep moves the stored agent private
  keys to the current key. The procedure is in
  [Rotating `KEYVAULT_MASTER_KEY`](../DEPLOYMENT.md#rotating-keyvault_master_key). CHANGELOG: *Added:
  `KEYVAULT_MASTER_KEY` can be rotated with a ring of master keys and a startup sweep*.
- At start the server prints
  `KeyVault: master key source=<environment|ephemeral> storageKeyId=<n> masterKeyIdentity=<32 lowercase hex>`. The
  identity is an HMAC under the key, the same at every start with the same key, so two instances can be checked for
  the same `KEYVAULT_MASTER_KEY` without printing it. CHANGELOG: *Added: the server prints one line at start that
  names its KeyVault master key*.
- `POSTGRES_POOL_ROLE` names a database role the request pool switches to with `SET ROLE`, and
  `POSTGRES_SCOPE_CENSUS=on` makes the request pool count its statements that name a tenant-scoped table, and those of
  them that run outside a transaction that declared a tenant or the service scope. Both are off when unset.
  CHANGELOG: *Added: an opt-in request-pool role and a row security census, both off by default*.
- `GET /health/ready` reports the `version` and `commit` the image was built from. CHANGELOG: *Fixed: the backend
  image stamps the version and commit that `/health/ready` reports*.
- With `POSTGRES_PASSWORD` empty, the server reads the database password from `PGPASSWORD` or the password file. An
  empty `REDIS_HOST` or `OTEL_EXPORTER_OTLP_ENDPOINT` turns that client off. CHANGELOG: *Fixed: with
  `POSTGRES_PASSWORD` empty, the server takes the database password from `PGPASSWORD` or the password file, as
  `aim-bootstrap` does (#709)*; *Fixed: an empty `REDIS_HOST` or `OTEL_EXPORTER_OTLP_ENDPOINT` turns that client off
  (#688)*.

The other `[Unreleased]` entries of `CHANGELOG.md`, among them changes to API answers, the dashboard and the SDKs, are
not repeated here; each is under its own heading there.

## Security fixes

Every fix below is in platform 1.1.0, the `aim-server` and `aim-dashboard` 1.1.0 images, except where the table says
otherwise. Advisories this repository has published are listed at
<https://github.com/opena2a-org/agent-identity-management/security/advisories>.

**Advisory pending, 2026-10-17.** As of 2026-10-17 the advisory for the `POST /api/v1/verification-events` fix is not
published. Platform 1.0.0 carries the affected route, and 1.1.0 is the fixed version. The advisory's link is added to
this line when it is published.

### Entries with their own heading

| `CHANGELOG.md` heading | Fixed in |
|------------------------|----------|
| Security: a password change ends every session signed in before it, and the API enforces a required password change | 1.1.0 |
| Security: an ATC whose agent is suspended, revoked or not stored is refused, and an ATC whose agent's status cannot be read is answered 503 with `Retry-After` | 1.1.0 |
| Security: the API server listens on 127.0.0.1 outside a container unless `HOST` is set (#757) | 1.1.0 |
| Security: the startup refusal for a missing `KEYVAULT_MASTER_KEY` names only the key | 1.1.0 |
| Security: `POST /api/v1/a2a/verify` reports a request from an agent in hybrid mode signed with Ed25519 alone invalid (#748) | 1.1.0 |
| Security: registering an agent or an MCP server applies only tags of the caller's organization (#749) | 1.1.0 |
| Security: a webhook signature covers the time the attempt was sent, so a receiver can refuse a replayed delivery | 1.1.0 |
| Security: a logout with any token of an SDK token family ends the family, with or without a revocation store | 1.1.0 |
| Security: creating an API key requires a live sign-in session | 1.1.0 |
| Security: SDK credential recovery refuses a token family that has ended | 1.1.0 |
| Security: the token endpoint and the SDK verification routes refuse an agent in hybrid mode that signs with Ed25519 alone (#740) | 1.1.0 |
| Security: A2A policy evaluation reads no trust score of another organization's agent (#740) | 1.1.0 |
| Security: the downloaded Python SDK writes the agent's name and the server's URL as escaped Python strings (#740) | 1.1.0 |
| Security: a capability verification is denied when the organization's enforcement mode cannot be read (#740) | 1.1.0 |
| Security: four registration log lines name ids instead of the registrant's address (#740) | 1.1.0 |
| Security: `PQCAgentMiddleware` admits an agent in hybrid mode only by a request signed with both Ed25519 and ML-DSA, and `Ed25519AgentMiddleware` refuses it | 1.1.0 |
| Security: refusing an agent key that is not recognised costs the signature verification an invalid signature costs | 1.1.0 |
| Security: the demo render keeps its run secrets in a directory `mktemp` creates | `docs/demo/render.sh` at `platform-v1.1.0` |
| Security: A2A policy evaluation reads the policies of the caller's organization (#652) | 1.1.0 |
| Security: a logged-out SDK-download token is not recovered, with or without a revocation store (#612) | 1.1.0 |
| Security: a verification event logged for an unknown or unreadable agent answers the same error as a foreign agent | 1.1.0 |
| Security: a reused SDK-download token is recorded as the same client or a different one | 1.1.0 |
| Security: sign-in failure and lock alert log lines name no email address | 1.1.0 |
| Security: the SDK download leaves out the SDK's tests and tooling and states no schedule | 1.1.0 |
| Security: a refused A2A card URL is a 400 with a fixed reason, and MCP verify and detect keep the cause in the server log | 1.1.0 |
| Security: webhook test deliveries send the event-delivery headers, are recorded as deliveries and log their failures, and a refused webhook URL is a 400 with a fixed reason | 1.1.0 |
| Security: an access token ends with the sign-in it belongs to | 1.1.0 |
| Security: webhook, MCP server and agent card requests refuse the Dummy IPv6 Prefix and the unassigned part of 2001::/23 | 1.1.0 |
| Security: the welcome and approval mail lines and three approval lines log ids instead of the user's address | 1.1.0 |
| Security: webhook, MCP server and agent card requests refuse eight more special-purpose address ranges | 1.1.0 |
| Security: every message the SMTP mail service sends asks the relay not to track it | 1.1.0 |
| Security: a revoked session's access token is refused on the routes behind the session middleware | 1.1.0 |
| Security: a revoked session's access token cannot create an API key, take or replace an agent's key, rotate a secret, or create or change a webhook | 1.1.0 |
| Security: every dashboard page is served with a Content-Security-Policy | 1.1.0 |
| Security: a stored agent private key decrypts only in the agent row it was written for | 1.1.0 |
| Security: a check fails on token-shaped literals in tracked files | repository check at `platform-v1.1.0` |
| Security: the rate limits no longer loosen when ENVIRONMENT is development | 1.1.0 |
| Security: a reused SDK-download token ends the chain that grew from it | 1.1.0 |
| Security: `/metrics` was readable without authentication on the API port and the dashboard origin | 1.1.0 |
| Security: an agent credential acts only on its own agent ID on the SDK-API, detection and A2A agent and card write routes | 1.1.0 |
| Security: agent-signature authentication reads an agent's status only after the signature verifies | 1.1.0 |
| Security: the Java SDK and the Java A2A example use jackson 2.18.11 | `sdk/java` source at `platform-v1.1.0` |
| Security: the server refuses a `KEYVAULT_MASTER_KEY` of 32 zero bytes | 1.1.0 |
| Security: the server refuses to start with the `JWT_SECRET` placeholder from `.env.example` | 1.1.0 |
| Security: MCP server and tag routes answer only for the caller's organization | 1.1.0 |
| Security: a reused refresh token is recorded as the same client or a different one | 1.1.0 |
| Security: the API no longer uses cookies for sign-in | 1.1.0 |
| Security: the API accepts a sign-in only from the Authorization header | 1.1.0 |
| Security: the SDK credential recovery route mints only for the account that is signed in | 1.1.0 |
| Security: a revoked session cannot mint a new credential with its still-valid access token | 1.1.0 |
| Security: two presentations of one refresh token in the same instant no longer both rotate | 1.1.0 |
| Security: a sign-in ends a fixed time after it began, however often it is refreshed | 1.1.0 |
| Security: a reused refresh token ends the sign-in, and logout ends the whole session | 1.1.0 |

### Entries in the `### Security` list

These entries sit under the plain `### Security` heading near the end of `[Unreleased]`. Each is named by the subject
its entry begins with.

| Entry | Fixed in |
|-------|----------|
| `POST /api/v1/auth/refresh` and `POST /api/v1/auth/sdk/recover` minted access tokens from the refresh token's own claims instead of the user record | 1.1.0 |
| `GET /api/v1/a2a/cards` returned every organization's agent cards to any authenticated caller | 1.1.0 |
| `GET /api/v1/a2a/consents` and `GET /api/v1/a2a/trust-scores` returned every organization's rows, and their counts | 1.1.0 |
| `GET /api/v1/a2a/consent/check` answered for consent records of any organization | 1.1.0 |
| `POST /api/v1/a2a/consent` accepted a grantor agent the caller does not own | 1.1.0 |
| `GET /api/v1/revocations` always returned an empty list, and the list no longer carries revoked agents' names | 1.1.0 |
| Java SDK: `CrlCache` and `LocalAtxVerifier` read a revocation list with null entries as one that revokes nothing | `sdk/java` source at `platform-v1.1.0` |
| A NULL in a nullable column failed the whole row on most agent read paths | 1.1.0 |
| Dependency updates: dashboard `next` 16.3.5, `sharp`, `postcss` and `nanoid`; backend `golang.org/x/crypto`, `golang.org/x/net`, `golang.org/x/text`, `google.golang.org/grpc` and `go-jose`; dashboard runtime base alpine 3.21.8 | 1.1.0 |
| Java SDK dependency updates: jackson and bouncycastle | `sdk/java` source at `platform-v1.1.0` |
| Java SDK: a JSON string is refused wherever the SDK binds a `Double`, `double`, `Float`, `float`, `BigDecimal` or `Number` from JSON | `sdk/java` source at `platform-v1.1.0` |
| The dashboard's self-hosted image optimizer is closed, and `/_next/image` returns 404 | 1.1.0 |
| Java SDK: `LocalAtxVerifier` accepted a forged ML-DSA-65 signature when the Ed25519 signature was intact | `sdk/java` source at `platform-v1.1.0` |
| Java SDK: `LocalAtxVerifier` took the authorities allowed to sign a credential from the credential's own `issuerChain` without requiring them to be trusted issuers | `sdk/java` source at `platform-v1.1.0` |
| Java SDK: `LocalAtxVerifier` rejects a duplicate object member before any field is read | `sdk/java` source at `platform-v1.1.0` |
| `POST /api/v1/verification-events` accepted a verification outcome reported by the caller | 1.1.0 |
| `POST /api/v1/sdk-api/verifications/:id/result` and `.../execution-status` served unauthenticated writes | 1.1.0 |
| `POST /api/v1/oauth/token` minted service tokens for a revoked agent | 1.1.0 |
| Secrets namespaces took the agent ID from the request with no owner check | 1.1.0 |
| A revoked or suspended agent authenticated on the Ed25519, post-quantum signature and API-key paths | 1.1.0 |
| OAuth service tokens reached every member-gated route | 1.1.0 |
| `GET /api/v1/agents/:id/credentials` and `GET /api/v1/agents/:id/sdk` were reachable with an organization-scoped API key | 1.1.0 |
| `POST /api/v1/agents/:id/mcp-servers/detect` opened a file path from the request body without validation | 1.1.0 |
| Session tokens carry a type, and a refresh token is no longer accepted as an access token | 1.1.0 |
| The public `did:aip` resolver answers a pending agent as it answers an unknown DID | 1.1.0 |
| The device grant's token endpoint minted a session for an account suspended, deactivated or deleted after approval | 1.1.0 |
| Dashboard: after sign-in the browser follows only a same-origin return path | 1.1.0 |
| Dashboard: the landing page no longer adopts a `token=` query parameter as the session | 1.1.0 |
| The FGA context check no longer fails open | 1.1.0 |
| `GET /metrics` served the full Prometheus scrape without authentication | 1.1.0 |

## Upgrading from 1.0.0

### Pinned tags

Measured on 2026-10-12 and again on 2026-10-17, with the same digests on `ghcr.io` and `docker.io`:

| Tag | `aim-server` | `aim-dashboard` |
|-----|--------------|-----------------|
| `1.0.0`, `1.0`, `1` | `sha256:c13d7f3cb2785ba141bfe70ea522cac1d7073719cd96b281e710e7ee296397e5` | `sha256:0c176ac9c927b0f794185d547088ff3cbb1502615cc50d191c3988d8f0b41a96` |
| `latest`, `edge` | `sha256:464fcfe00d026458216e1df18cc1223b8d3b43a8241db72b365b4c4a4a98ebd7` | `sha256:65e0fb56b96213de1215fa0bc8b1a546838e075b858a04866c0900083887da58` |

The `1.0.0`, `1.0` and `1` tags name the 1.0.0 images. `latest` and `edge` name builds of `main` at commit `3ab8285e`,
made on 2026-10-02. The publish workflow now builds images only from a `platform-v*` tag, and it writes `1.1.0`, `1.1`,
`1` and `latest` for this release:

- A deployment that pulls `1` or `latest` runs 1.1.0 after its next pull, with every item under
  [Changes that need action](#changes-that-need-action). Both compose files at the repository root,
  `docker-compose.yml` and `docker-compose.quickstart.yml`, name the `aim-server` and `aim-dashboard` images by
  `latest`, so `docker compose pull` fetches 1.1.0.
- `1.0.0` and `1.0` stay on 1.0.0.
- `edge` is no longer written, and stays on the 2026-10-02 build.
- To choose when the upgrade happens, pin each image by digest: `ghcr.io/opena2a-org/aim-server@<AIM_SERVER_DIGEST>`
  and `ghcr.io/opena2a-org/aim-dashboard@<AIM_DASHBOARD_DIGEST>`.

To read the digest a tag names now:

```bash
docker buildx imagetools inspect ghcr.io/opena2a-org/aim-server:1
```

### An unsigned image on `latest` and `edge` in August 2026

From 16:37 UTC on 2026-08-12 until 12:32 UTC on 2026-08-13, the `latest` and `edge` tags of `aim-server` on both
registries named `sha256:9299a4b2e9c9264e4cf00f3bd3a293fb5c2dbf1e5d9b0eec430206c3b48bd002`, a build of `main` that
carries no cosign signature and no build provenance attestation: its
[publish run](https://github.com/opena2a-org/agent-identity-management/actions/runs/31617620055) pushed the tags and
stopped before the signing step. The next publish run moved both tags to a signed build. To check whether a host
pulled that image, and that the digest has no attestation:

```bash
docker image ls --digests ghcr.io/opena2a-org/aim-server
docker image ls --digests opena2a/aim-server
curl -s https://api.github.com/repos/opena2a-org/agent-identity-management/attestations/sha256:9299a4b2e9c9264e4cf00f3bd3a293fb5c2dbf1e5d9b0eec430206c3b48bd002
```

The last command answers `"message": "Not Found"`. Replace that image with a pinned 1.1.0 digest. An earlier
[publish run](https://github.com/opena2a-org/agent-identity-management/actions/runs/26201603961), on 2026-05-21 at
02:20 UTC, also pushed `aim-server` `latest` and `edge` and then failed at its signing step; its log has expired, so
that digest is not named here. The next publish run started at 02:22 UTC and finished with a signed build.

### Upgrade steps

1. Read [Changes that need action](#changes-that-need-action). Set `KEYVAULT_MASTER_KEY` to the value 1.0.0 ran with,
   set `JWT_SECRET` to a generated value if it still holds the `.env.example` placeholder, set
   `METRICS_LISTEN_ADDR` and `METRICS_AUTH_TOKEN` if Prometheus scrapes the backend from another host, and set `HOST`
   if the server runs outside a container and other hosts reach it.
2. Back up the database:

   ```bash
   pg_dump --format=custom --file aim-1.0.0.dump --host <host> --username <user> <database>
   ```

3. Stop every 1.0.0 instance of `aim-server`.
4. Point each deployment at the 1.1.0 images, pinned by digest, and pull them. With the root `docker-compose.yml`,
   whose images are named by `latest`:

   ```bash
   docker compose pull
   ```

5. Apply the migrations as the role that owns the schema, with the 1.1.0 image. With the root `docker-compose.yml`:

   ```bash
   docker compose run --rm backend ./aim-server migrate
   ```

   The server also applies pending migrations when it starts, so this step can be left to the first start. A 1.0.0
   image has no `migrate` command: run it only after step 4.
6. Start the 1.1.0 server and dashboard. At its first start the server converts the stored agent private keys; its log
   line that begins `Agent private key migration` reports `failures=0` and `recorded=true` when every key converted.
7. Check the running version:

   ```bash
   curl -s http://localhost:8080/health/ready
   ```

   `version` reads `1.1.0`.
8. Sign in to the dashboard again, and give each SDK install that used credentials issued by 1.0.0 new ones.

To roll back, restore the backup from step 2 and run the 1.0.0 images.

### Measured migration time

Upgrading from 1.0.0 applies 39 migrations, `098_mcp_manifest_baselines.sql` to
`137_user_delete_removes_registration_requests_and_audit_copies.sql`. Measured on 2026-10-19 on an Apple M4 Max laptop
with PostgreSQL 18.3: a database was migrated with the 97 migration files of `platform-v1.0.0`, each applied in its own
transaction and recorded in `schema_migrations` by file name, as the 1.0.0 server does at start. Then
`aim-server migrate`, built from a tree whose last migration is
`137_user_delete_removes_registration_requests_and_audit_copies.sql`, was timed against it with `/usr/bin/time -p`,
three runs on separate copies by a binary that had already run on that laptop, while the laptop ran other work and its
one-minute load average read 6.62 before the runs and 6.67 after them. Each run logged
`Successfully applied 39 pending migration(s)`:

| 1.0.0 database | Wall time, three runs |
|----------------|-----------------------|
| only the rows the migrations write, 14 MB | 0.31 s, 0.27 s, 0.21 s |
| 1,000,000 rows in `api_calls` and 1,000,000 in `audit_logs`, 542 MB | 1.26 s, 1.00 s, 0.91 s |
| the same, with 10,000 of the `audit_logs` rows role changes that hold a `user_email` and name no user, 542 MB | 7.07 s, 5.34 s, 7.13 s |

The time grows with the size of the tables the migrations alter or index. The third database is the second with
10,000 of its `audit_logs` rows written as such role changes; the last migration removes `user_email` from them once,
as it does from the audit rows about users deleted before it ran. The agent private key conversion runs at the
server's first start, not in `aim-server migrate`, and is not in these times; it reads every agent row that holds a
stored private key.

### For Java SDK users

The Java SDK is built from source in `sdk/java` and keeps the coordinates `org.opena2a:aim-sdk:1.0.0` in this release,
so the version number does not tell a build from the 1.0.0 source from one from the 1.1.0 source. The 1.0.0 source
sets `jackson.version` to 2.16.1, and the 1.1.0 source sets 2.18.11. CHANGELOG: *Security: the Java SDK and the Java
A2A example use jackson 2.18.11*.

Check the source you build from; this prints `2.18.11` for the 1.1.0 source:

```bash
mvn -q -f sdk/java/pom.xml help:evaluate -Dexpression=jackson.version -DforceStdout
```

Check what your application resolves, from its own directory; `jackson-databind`, `jackson-core` and
`jackson-annotations` should read 2.18.11:

```bash
mvn dependency:tree -Dincludes=com.fasterxml.jackson.core
```

To fix an older version, run `mvn install` in `sdk/java` from the 1.1.0 source and rebuild your application. If your
build sets the jackson version itself, through a BOM or its own `jackson.version` property, set it to 2.18.11.
