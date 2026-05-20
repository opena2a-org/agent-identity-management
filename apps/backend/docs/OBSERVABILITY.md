# AIM Backend Observability

The AIM Go backend emits OpenTelemetry traces, metrics, and logs over OTLP gRPC. This document covers how to run the demo stack, configure the exporter, query the data, and the canonical SemConv attribute names emitted by the backend.

This implementation is the reference for the AIM SemConv proposal pitched at the Observability Summit (Talk 2 Slides 13-15). Attribute names emitted here are LOCKED to that proposal.

## Quick start

The fastest way to verify the stack works end-to-end is to run the smoke test, which boots the stack and confirms a synthetic OTLP trace + counter + log lands in Tempo, Prometheus, and Loki:

```bash
cd apps/backend/deployments/otel-demo
./smoke-test.sh
```

Expected output ends with `==> PASS: all three signals landed end-to-end` plus three clickable Grafana/Prometheus URLs pointing at the verified signals. Runtime: 60-90s warm, 3-5 min on first run (image pulls).

The smoke test also handles port conflicts: if any of `4317 4318 3200 9090 3100 3001` are taken on your machine, it fails fast with a clear message and tells you to copy `env.example` to `.env` and pick free ports. See `docs/testing/release-smoke.md` for the full smoke-test contract.

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
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `localhost:4317` | OTLP gRPC endpoint. The backend always uses gRPC; HTTP is not wired in this build. |
| `OTEL_SERVICE_NAME` | `aim-backend` | Sets the `service.name` resource attribute. |

The backend currently dials with TLS disabled (`Insecure: true` in `internal/telemetry/init.go`). Production deployments should flip that and configure a CA bundle.

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
| `agent.scan_verdict` | string | resource attribute | Producer-emitted security scan verdict. Read from `agent_security_contexts.scan_verdict`. Producer is expected to write this from a real scanner; the HackMyAgent integration that performs that write is on the roadmap. Enum values: `clean`, `warnings`, `findings`, `critical`, `unknown`. |
| `fga.step` | string | span attribute on child spans | One of: `capability_check`, `attribute_check`, `context_check`, `chain_check`, `intent_check_sync`, `intent_check_async` |
| `fga.outcome` | string | span attribute, log attribute, metric label | `ALLOW`, `DENY`, `DENY_INTENT`, `DENY_CONTEXT`, `DENY_CHAIN`, `DENY_ATTRIBUTE`, `ERROR` (transient infra failures: `loadPolicy` / `HasCapability` returned an error) |
| `fga.denied_by` | string | span attribute, log attribute, metric label | Step that denied (set when `fga.outcome != ALLOW`) |

## What the backend emits

### Traces

`FGAEngine.Authorize` produces one parent span and up to five child spans:

```
fga.authorize  (agent.id, agent.capability, fga.outcome, fga.latency_ms, fga.steps_triggered, [fga.denied_by])
├── fga.capability_check  (fga.step, fga.allowed)
├── fga.attribute_check   (fga.step, fga.allowed, [fga.denied_reason])
├── fga.context_check     (fga.step, fga.allowed, [fga.denied_reason])
├── fga.chain_check       (fga.step, fga.allowed, [fga.denied_reason])
└── fga.intent_check_sync (fga.step, fga.allowed, fga.intent_class, fga.intent_confidence)
    OR
    fga.intent_check_async (fga.step, fga.dispatched=true)
```

Spans whose check denies set status `codes.Error` with the deny reason.

### Metrics

| OTel metric name | Type | Labels | Prometheus query name | Notes |
|---|---|---|---|---|
| `fga.decisions` | counter | `fga.outcome`, `fga.denied_by` | `fga_decisions_total` | Incremented on every Authorize return. The `_total` suffix is added by Prometheus's OTLP receiver per OpenMetrics convention; do NOT include it in the OTel name (Prometheus rejects with "invalid temporality and type combination"). |
| `fga.latency_ms` | histogram | none | `fga_latency_ms_bucket`, `_count`, `_sum` | Total Authorize latency in ms |
| `agent.drift_score` | gauge | `agent.id` | `agent_drift_score` | Saturated 0-1 score from `tanh(drift_count / 2)`. Emitted on every DetectDrift call (including 0 when no drift) so dashboards distinguish "clean" from "silent". |

Attributes are flattened to label names (`fga_outcome`, `agent_id`, etc.) per the OTLP-to-Prometheus convention.

### Logs

`FGAEngine.Authorize` emits one `fga.decision` log line per decision via the OTel slog bridge. Trace context (`trace_id`, `span_id`) auto-attaches when the call happens inside an active span. Level: WARN for denies, INFO for allows.

Attributes: `agent.id`, `agent.capability`, `fga.outcome`, `fga.denied_by`, `fga.denied_reason`, `fga.latency_ms`.

In Loki, structured metadata for `trace.id`, `span.id`, `agent.id`, `fga.outcome`, `fga.denied_by`, `fga.step` is indexed (see `loki-config.yaml`).

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

Find traces that hit the intent check:

```
{name="fga.intent_check_sync"}
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
