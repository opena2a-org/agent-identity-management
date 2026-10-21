# AIM Backend Observability

The AIM Go backend emits OpenTelemetry traces, metrics, and logs over OTLP gRPC. This document covers how to run the demo stack, configure the exporter, query the data, and the canonical SemConv attribute names emitted by the backend.

This implementation is the reference for the AIM SemConv proposal pitched at the Observability Summit (Talk 2 Slides 13-15). Attribute names emitted here are LOCKED to that proposal.

## Quick start

The fastest way to verify the stack works end-to-end is to run the smoke test, which boots the stack and confirms a synthetic OTLP trace + counter + log lands in Tempo, Prometheus, and Loki:

```bash
cd apps/backend/deployments/otel-demo
./smoke-test.sh
```

Expected output ends with `==> PASS: all three signals landed end-to-end`. Runtime: 60-90s warm, 3-5 min on first run (image pulls).

Each run boots its own compose project (`aim-otel-smoke-<time>-<pid>`, container names to match) and removes only that project, with its volumes, on exit. A demo stack you started yourself with `docker compose up -d` is never stopped or removed. To keep the run's stack up and get three clickable Grafana/Prometheus URLs pointing at the verified signals, run `KEEP_STACKS=1 ./smoke-test.sh`; the output then prints the `docker compose -p <project> down -v` that removes it.

If any of `4317 4318 3200 9090 3100 3001` are taken on your machine (for example by a running demo stack), the smoke test fails fast with a clear message: stop the demo stack with `docker compose stop` (keeps its data), or copy `env.example` to `.env` and pick free ports. See `docs/testing/release-smoke.md` for the full smoke-test contract.

## Manual run

If you want to drive the stack with the real backend instead of synthetic signals:

```bash
cd apps/backend/deployments/otel-demo
docker compose up -d

# In another terminal, run the backend pointed at the local collector
cd apps/backend
go run ./cmd/server
```

Then open Grafana at http://localhost:3001 (anonymous admin, no login). Three dashboards are pre-loaded under the `AIM` folder:

- **AIM FGA Traces**: Tempo trace search filtered by `agent.id` and `fga.outcome`
- **AIM Agent Drift**: Prometheus time-series of `agent.drift_score` with watch / alert / critical bands
- **AIM FGA Audit Log**: Loki logs of `fga.decision` events, with click-through to Tempo via `trace.id`

Tear down without losing data:

```bash
docker compose stop
```

Tear down and wipe volumes:

```bash
docker compose down -v
```

## Environment variables

| Variable | Default | Notes |
|---|---|---|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` when unset | OTLP gRPC endpoint. The backend always uses gRPC; HTTP is not wired in this build. Set to the empty string, it turns telemetry off: the backend installs no exporter, dials no collector and logs `OpenTelemetry disabled`. |
| `OTEL_SERVICE_NAME` | `aim-backend` | Sets the `service.name` resource attribute. |

The endpoint's scheme decides TLS: `https://` is TLS and `http://` plaintext. `host:port` is plaintext with `OTEL_EXPORTER_OTLP_INSECURE=true`, TLS with `false`, and with neither TLS unless the host is loopback. `OTEL_EXPORTER_OTLP_CERTIFICATE`, `OTEL_EXPORTER_OTLP_CLIENT_CERTIFICATE` and `OTEL_EXPORTER_OTLP_CLIENT_KEY` name the CA and client key pair. The full rules are in [`docs/DEPLOYMENT.md` § Observability](../../../docs/DEPLOYMENT.md#observability).

If the collector is unreachable at boot, the backend logs a warning and continues without telemetry. Spans/metrics/logs are buffered in batch processors and the connection is retried.

## SemConv attributes (LOCKED)

These names match Slide 14 of the May 22 Observability Summit talk and the AIM SemConv proposal. Do not rename without updating the proposal.

| Attribute | Type | Where it appears | Meaning |
|---|---|---|---|
| `agent.id` | string | span attribute, log attribute, metric label | The AIM agent UUID |
| `agent.public_key.algorithm` | string | resource attribute | Public key algorithm (e.g. `Ed25519`, `ML-DSA-65`) |
| `agent.capability` | string | span attribute, log attribute | The capability being authorized (e.g. `read:bookings`) |
| `agent.trust_score` | double | resource attribute | 9-factor weighted trust score |
| `agent.drift_score` | double | metric, resource attribute | 0-1 saturated drift signal |
| `agent.scan_verdict` | string | resource attribute | Producer-emitted security scan verdict. Read from `agent_security_contexts.scan_verdict`. Producer is expected to write this from a real scanner; no shipped integration performs that write yet. Enum values: `clean`, `warnings`, `findings`, `critical`, `unknown`. |
| `fga.step` | string | span attribute on child spans | One of: `capability_check`, `attribute_check`, `context_check`, `chain_check`, `intent_check_async` |
| `fga.outcome` | string | span attribute, log attribute, metric label | `ALLOW`, `DENY`, `DENY_CONTEXT`, `DENY_CHAIN`, `DENY_ATTRIBUTE`, `ERROR` (transient infra failures: `loadPolicy` / `HasCapability` returned an error). `DENY_INTENT` is historical: Step 5 no longer denies, so the engine does not produce it, but `access_attestations` rows written before that change may still carry it (migration 080's CHECK allows it). |
| `fga.denied_by` | string | span attribute, log attribute, metric label | Step that denied (set when `fga.outcome != ALLOW`) |

## What the backend emits

### Traces

`FGAEngine.Authorize` produces one parent span and up to five child spans, on `POST /api/v1/agents/:id/authorize` and on
`POST /api/v1/sdk-api/verifications`, whose step 1 is the route's own capability check (`AgentService.VerifyCapability`):

```
fga.authorize  (agent.id, agent.capability, fga.policy, fga.outcome, fga.latency_ms, fga.steps_triggered, [fga.denied_by])
├── fga.capability_check  (fga.step, fga.allowed, [fga.denied_reason])
├── fga.attribute_check   (fga.step, fga.allowed, [fga.denied_reason])
├── fga.context_check     (fga.step, fga.allowed, [fga.denied_reason])
├── fga.chain_check       (fga.step, fga.allowed, [fga.denied_reason])
└── fga.intent_check_async (fga.step, fga.dispatched, [fga.async_dropped, fga.async_drop_reason])
```

`fga.policy` is `stored` when the agent has an `fga_policies` row for the capability and `default` when the default
policy decided (no restriction, risk level detected from the capability), under which steps 2 to 4 allow.

`fga.intent_check_async` marks the dispatch of the Step 5 check for HIGH and MEDIUM risk; the check itself runs
detached in a worker under its own `fga.intent_check_async.worker` span and never changes the decision.

Spans whose check denies set status `codes.Error` with the deny reason.

### Metrics

| OTel metric name | Type | Labels | Prometheus query name | Notes |
|---|---|---|---|---|
| `fga.decisions` | counter | `fga.outcome`, `fga.denied_by` | `fga_decisions_total` | Incremented on every Authorize return. The `_total` suffix is added by Prometheus's OTLP receiver per OpenMetrics convention; do NOT include it in the OTel name (Prometheus rejects with "invalid temporality and type combination"). |
| `fga.latency_ms` | histogram | none | `fga_latency_ms_bucket`, `_count`, `_sum` | Total Authorize latency in ms |
| `fga.intent_checks` | counter | `fga.risk_tier`, `fga.intent_mode`, `fga.intent_status`, `fga.intent_blocked` | `fga_intent_checks_total` | Step 5 (NanoMind intent) evaluations that reached the daemon call. `intent_mode` is `async` for HIGH and MEDIUM alike (series recorded before Step 5 stopped denying may carry `sync` for HIGH); `intent_status` is `classified` (daemon returned a non-empty attack class), `abstain` (clean response, no class: the common case until NanoMind is fine-tuned on the FGA prompt), or `fail_open` (daemon unreachable, request build error, or undecodable body: the action proceeds without an intent verdict). `intent_blocked` is always `false` now: the classifier's verdict is recorded, never acted on. |
| `fga.intent_skipped` | counter | `fga.risk_tier` | `fga_intent_skipped_total` | Authorizes where Step 5 was skipped (LOW risk, or any non-HIGH/MEDIUM level). Pair with `fga.intent_checks` to get the skip rate. |
| `fga.async_intent_dropped` | counter | `fga.async_drop_reason` | `fga_async_intent_dropped_total` | HIGH- and MEDIUM-risk async intent dispatches dropped before running. `async_drop_reason` is `queue_full` (worker pool saturated) or `shutdown` (engine draining). A dropped dispatch never increments `fga.intent_checks`; sum both for total HIGH and MEDIUM dispatch attempts. |
| `agent.drift_score` | gauge | `agent.id` | `agent_drift_score` | Saturated 0-1 score from `tanh(drift_count / 2)`. Emitted on every DetectDrift call (including 0 when no drift) so dashboards distinguish "clean" from "silent". |

Attributes are flattened to label names (`fga_outcome`, `agent_id`, etc.) per the OTLP-to-Prometheus convention.

### Logs

`FGAEngine.Authorize` emits one `fga.decision` log line per decision via the OTel slog bridge. Trace context (`trace_id`, `span_id`) auto-attaches when the call happens inside an active span. Level: WARN for denies, INFO for allows.

Attributes: `agent.id`, `agent.capability`, `fga.outcome`, `fga.denied_by`, `fga.denied_reason`, `fga.latency_ms`.

In Loki, structured metadata for `trace.id`, `span.id`, `agent.id`, `fga.outcome`, `fga.denied_by`, `fga.step` is indexed (see `loki-config.yaml`).

### Refused signed agent requests

An agent request signed with Ed25519, ML-DSA or both is checked by the request-signature middleware before any handler runs. A request it refuses gets a 401 (400 for an unsupported algorithm) and is recorded in two places. Both come from the backend process itself, not through the OTLP pipeline above.

**Counter**, on the backend's `/metrics` endpoint: `aim_s1_refusals_total{reason, sdk}`.

| `reason` | The request was refused because |
|---|---|
| `unsupported_algorithm` | `X-Algorithm` names an algorithm the server cannot verify |
| `invalid_agent_id` | `X-Agent-ID` is not a UUID |
| `invalid_timestamp` | `X-Timestamp` is not an integer |
| `skew_past` | the timestamp is more than 30 seconds behind the server clock |
| `skew_future` | the timestamp is more than 30 seconds ahead of the server clock |
| `agent_lookup_failed` | the agent does not exist, or loading it failed |
| `agent_status_denied` | the agent is revoked, suspended or in an unrecognised status |
| `hybrid_required` | the agent is in hybrid mode and the request is not signed with both Ed25519 and ML-DSA |
| `missing_signature_headers` | a signature or public-key header the algorithm requires is absent |
| `no_registered_key` | the agent has no registered key for the algorithm, including an ML-DSA key of another level |
| `public_key_mismatch` | the key the request presents is not the registered key |
| `registered_key_malformed` | the registered key does not decode to a key of its algorithm family |
| `signature_malformed` | a signature header is not valid base64 |
| `signature_invalid_ed25519` | the Ed25519 signature does not verify over the request |
| `signature_invalid_mldsa` | the ML-DSA signature does not verify over the request |

`agent_lookup_failed`, `no_registered_key`, `public_key_mismatch` and `registered_key_malformed` are counted apart, but the caller gets one 401 body for all four, `{"error":"the signing key is not the registered key of a known agent","reasonCode":"agentKeyNotRecognized"}`, so a caller holding only an agent id learns nothing about that agent. The agent's status is read only after the signature verifies, so `agent_status_denied` counts requests signed with the agent's own key. The hybrid-mode flag is read after the signature verifies and before the status, so `hybrid_required` counts requests signed with one of the agent's own keys.

The set is closed. `unclassified` exists only as a guard: the middleware never produces it, and seeing it means a refusal was recorded without a reason from the table.

`sdk` is `python`, `typescript` or `java` when the request's `User-Agent` starts with `AIM-Python-SDK/`, `AIM-SDK-TypeScript/` or `AIM-Java-SDK/`, and `other` for every other caller.

**Log line**, on the backend's standard log: one `s1_refusals` line per 64-second period in which at least one request was refused, with one `reason.sdk=count` field per series. A period starts at the first refusal after the previous line, so nothing is written while nothing is refused. On shutdown the open period is written before the process exits, and its `period_s` is the seconds since that period's first refusal, rounded up, at most 64.

```
s1_refusals period_s=64 total=3 signature_invalid_ed25519.typescript=1 skew_past.python=2
```

A refusal happens before the caller is authenticated, so neither the labels nor the line carry anything the request sent: no agent id, key, signature, signed bytes or raw `User-Agent`.

Two patterns the split is there to show:

- `skew_past` or `skew_future` rising on one instance for every `sdk` at once points at that instance's clock. A fast server clock shows as `skew_past`, a slow one as `skew_future`.
- `signature_invalid_ed25519` or `signature_invalid_mldsa` rising for one `sdk` only points at how that SDK signs.

```promql
sum by (reason, sdk) (rate(aim_s1_refusals_total[5m]))
```

### Refused `agent-request-v2` statements

A request that carries an `agent-request-v2` statement in `X-Agent-Request` is checked by `AgentRequestV2Middleware`. A request it refuses is counted on the backend's `/metrics` endpoint in `aim_agent_request_v2_refusals_total{reason, sdk}`, and in no `aim_s1_refusals_total` series. No route mounts the middleware yet, so no request is counted here until one does.

| `reason` | The request was refused because | Response |
|---|---|---|
| `audience_not_configured` | the server has no configured public origin, so it accepts no statement | 503 `audienceNotConfigured` |
| `conflicting_authentication` | the request also carries `Authorization`, `X-API-Key` or a header the other agent signature checks read | 400 `conflictingAuthentication` |
| `statement_malformed` | `X-Agent-Request` is not sent once, is not unpadded base64url, or does not carry one JSON object whose members are each defined, present once and in their form | 400 `invalidRequest` |
| `nonce_missing` | the payload has no `nonce` | 400 `nonceRequired` |
| `signature_header_malformed` | `X-Agent-Request-Signature` is absent, carries more than two values, a value that is not an algorithm, a colon and an unpadded base64url signature, or two values that are not one Ed25519 and one ML-DSA signature | 400 `invalidRequest` |
| `audience_mismatch` | the statement's `audience` is not the configured public origin, compared byte for byte | 401 `audienceMismatch` |
| `method_mismatch` | the statement's `method` is not the request's method | 401 `requestMismatch` |
| `target_mismatch` | the statement's `target` is not the request's path and query as received | 401 `requestMismatch` |
| `body_mismatch` | `body_sha256` is absent beside a body, present beside none, or the SHA-256 of other bytes | 401 `requestMismatch` |
| `agent_lookup_failed` | the agent does not exist, or loading it failed | 401 `agentKeyNotRecognized` |
| `no_registered_key` | the agent has no registered key for an algorithm a signature names, including an ML-DSA key of another level | 401 `agentKeyNotRecognized` |
| `registered_key_malformed` | the registered key does not decode to a key of its algorithm family | 401 `agentKeyNotRecognized` |
| `signature_invalid_ed25519` | the Ed25519 signature does not verify over the statement | 401 `signatureInvalid` |
| `signature_invalid_mldsa` | the ML-DSA signature does not verify over the statement | 401 `signatureInvalid` |
| `hybrid_required` | the agent is in hybrid mode and the request carries one signature | 401 `hybridSignatureRequired` |
| `single_signature_required` | the agent is not in hybrid mode and the request carries two signatures | 401 `singleSignatureRequired` |
| `skew_past` | the timestamp is more than 30 seconds behind the database clock | 401 `timestampOutsideWindow` |
| `skew_future` | the timestamp is more than 30 seconds ahead of the database clock | 401 `timestampOutsideWindow` |
| `nonce_reused` | the agent has already used the nonce | 401 `nonceReused` |
| `freshness_check_unavailable` | the timestamp and nonce could not be checked against the database | 503 `freshnessCheckUnavailable` |
| `agent_status_denied` | the agent's status is neither `verified` nor `pending` | 401 `agentStatusDenied` |

Where one `reasonCode` answers several causes, `reason` tells them apart for operators only: `agent_lookup_failed`, `no_registered_key` and `registered_key_malformed` all answer `{"error":"the signing key is not the registered key of a known agent","reasonCode":"agentKeyNotRecognized"}`. The agent's status is read only after the signatures verify and the nonce is admitted, so `agent_status_denied` counts requests signed with the agent's own keys.

The set is closed. `unclassified` exists only as a guard: the middleware never produces it, and seeing it means a refusal was recorded without a reason from the table. `sdk` is derived from `User-Agent` as for `aim_s1_refusals_total`. Neither label carries anything the request sent: no agent id, statement, signature or raw `User-Agent`.

Three patterns the split is there to show:

- `skew_past` or `skew_future` rising for every `sdk` at once points at the database clock. A fast database clock shows as `skew_past`, a slow one as `skew_future`.
- `hybrid_required` rising at one deployment while Ed25519 requests pass can point at a proxy that drops a repeated `X-Agent-Request-Signature` header line.
- `target_mismatch` rising for every `sdk` can point at a proxy that rewrites the path or query between the agent and the server.

```promql
sum by (reason, sdk) (rate(aim_agent_request_v2_refusals_total[5m]))
```

The share of `agent-request-v2` requests refused, with `aim_agent_requests_authenticated_total`, which counts the ones authenticated:

```promql
sum(rate(aim_agent_request_v2_refusals_total[5m]))
/
(
  sum(rate(aim_agent_request_v2_refusals_total[5m]))
  + sum(rate(aim_agent_requests_authenticated_total{type="agent-request-v2"}[5m]))
)
```

## Sample queries

### Tempo (TraceQL)

Find all FGA denies for a specific agent:

```
{name="fga.authorize" && .agent.id="<uuid>" && .fga.outcome != "ALLOW"}
```

Find slow authorizations (>500ms total):

```
{name="fga.authorize" && .fga.latency_ms > 500}
```

Find traces that dispatched the intent check, and the detached checks themselves:

```
{name="fga.intent_check_async"}
{name="fga.intent_check_async.worker"}
```

### Prometheus (PromQL)

Decisions per minute by outcome:

```promql
sum by (fga_outcome) (rate(fga_decisions_total[1m]))
```

P99 Authorize latency over last 5m:

```promql
histogram_quantile(0.99, sum by (le) (rate(fga_latency_ms_bucket[5m])))
```

Step 5 classification rate (share of evaluations that produced a real class). Today this sits near zero, which is the evidence behind issue #131:

```promql
sum(rate(fga_intent_checks_total{fga_intent_status="classified"}[5m]))
  / sum(rate(fga_intent_checks_total[5m]))
```

Step 5 fail-open rate by risk tier (daemon unreachable or undecodable):

```promql
sum by (fga_risk_tier) (rate(fga_intent_checks_total{fga_intent_status="fail_open"}[5m]))
  / sum by (fga_risk_tier) (rate(fga_intent_checks_total[5m]))
```

Step 5 skip rate (share of authorizes that never ran the intent check):

```promql
sum(rate(fga_intent_skipped_total[5m]))
  / (sum(rate(fga_intent_skipped_total[5m])) + sum(rate(fga_intent_checks_total[5m])))
```

Requests the classifier attached an attack class to, by risk tier (recorded only; none of them is denied on it):

```promql
sum by (fga_risk_tier) (rate(fga_intent_checks_total{fga_intent_status="classified"}[5m]))
```

Agents in the alert band (drift > 0.7):

```promql
agent_drift_score > 0.7
```

Top 10 agents by drift score, last 15m:

```promql
topk(10, max_over_time(agent_drift_score[15m]))
```

### Loki (LogQL)

All FGA denies, last 30m:

```logql
{service_name="aim-backend"} |= "fga.decision" | json | fga_outcome != "ALLOW"
```

Denies broken down by reason:

```logql
sum by (fga_denied_by) (count_over_time({service_name="aim-backend"} |= "fga.decision" | json | fga_outcome != "ALLOW" [5m]))
```

Single agent's full decision history:

```logql
{service_name="aim-backend", agent_id="<uuid>"} |= "fga.decision"
```

## Cross-signal navigation in Grafana

The provisioned datasources are wired so:

- **Traces → Logs**: in any Tempo span detail, the "Logs for this span" button runs `{service_name="aim-backend"} |= "<traceID>"` against Loki.
- **Logs → Traces**: any Loki log line with `trace_id=<32-hex>` renders a clickable link that opens the trace in Tempo (via the `derivedFields` config in `datasources.yml`).
- **Traces → Metrics**: span-to-metrics is configured against the Prometheus datasource for service-map and RED metrics.

## Troubleshooting

**The collector logs `connection refused` on startup.** Tempo/Loki/Prometheus are still booting. The collector retries; spans are not lost during the boot window because they are buffered in the batch processor.

**No spans appear in Tempo.** Confirm the backend logs `OpenTelemetry initialized (OTLP gRPC)`. Then check the collector container: `docker logs aim-otel-collector | grep -i traces`. The `debug` exporter logs span counts at INFO.

**Metrics appear but with wrong names.** OTLP metric names with dots (`fga.decisions_total`) are normalized to underscores by Prometheus's OTLP receiver. Use `fga_decisions_total` in PromQL.

**Loki rejects logs with `out of order`.** Usually means clock skew between the backend host and the Loki container. Ensure both run on the same host clock or use NTP.

**Grafana shows "Datasource not found" on dashboards.** Provisioned datasources use fixed UIDs (`tempo`, `prometheus`, `loki`). Don't rename them via the UI; the dashboards reference UIDs directly.

## Production deployment notes

This stack is for the Observability Summit demo and local development. For production:

1. Switch `Insecure: true` in `internal/telemetry/init.go` to false and configure mutual TLS to a managed collector.
2. Replace the in-memory single-binary Tempo / Loki with their distributed deployments.
3. Add tail-based sampling at the collector (`tail_sampling` processor). Full sampling at AIM scale will overwhelm the trace store.
4. Strip PII from span attributes via a transform processor before export.
5. Send traces to your existing tracing backend if you have one; the collector's `otlp/tempo` exporter can be replaced with `datadog`, `newrelic`, `splunk_hec`, etc.
