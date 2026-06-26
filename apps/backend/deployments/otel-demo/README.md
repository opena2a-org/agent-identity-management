# AIM OTel demo: `gen_ai.agent.*` authorization attributes

A runnable example of the authorization-decision telemetry the AIM backend emits
on every `fga.authorize` span. The stack (OpenTelemetry Collector → Tempo /
Prometheus / Loki → Grafana) receives one synthetic trace, metric, and log so you
can see the attributes land end to end.

## The eight attributes

AIM's fine-grained authorization engine computes an allow/deny decision from
trust, drift, and scan signals. At that decision point it sets eight namespaced
attributes on the `fga.authorize` span:

| Attribute | Type | Example | Meaning |
|---|---|---|---|
| `gen_ai.agent.capability` | string | `payments:transfer` | capability the agent requested |
| `gen_ai.agent.public_key.algorithm` | string | `Ed25519+ML-DSA-65` | agent signing-key algorithm (classical, or classical+PQC) |
| `gen_ai.agent.trust.score` | double | `0.92` | trust score used in the decision |
| `gen_ai.agent.trust.method` | string | `aim/trust-calculator` | how the trust score was derived |
| `gen_ai.agent.drift.score` | double | `0.0` | configuration/behavior drift score |
| `gen_ai.agent.drift.method` | string | `aim/drift-capability-diff` | how drift was measured |
| `gen_ai.agent.scan.verdict` | string | `clean` | security scan verdict |
| `gen_ai.agent.scan.method` | string | `aim/registry-asc` | source of the scan verdict |

The names track the OpenTelemetry GenAI SemConv proposal
[open-telemetry/semantic-conventions-genai#291](https://github.com/open-telemetry/semantic-conventions-genai/pull/291).
AIM emits them because it genuinely produces these signals at its authorization
decision point.

### `*.method` semantics

Each `*.method` is a stable producer label naming *how* the paired signal is
derived — not per-record provenance. `gen_ai.agent.scan.method = aim/registry-asc`
means AIM read the verdict replicated from the Registry Agent Security Context;
AIM does not observe which underlying scanner produced it, so it does not claim
one.

A `*.method` is only emitted alongside its paired score/verdict — there are no
orphan method labels.

## Run it

```bash
./smoke-test.sh
```

This boots the stack, sends one trace/metric/log, verifies each landed in its
backend, and prints `PASS` / `FAIL`. To change ports, copy `env.example` to
`.env` first. `gen-signal.sh` is the standalone signal generator the smoke test
uses; run it directly against a collector with
`./gen-signal.sh <otlp-grpc-port>`.

Open Grafana (default `http://localhost:3000`) and inspect the `fga.authorize`
trace in Tempo to see the eight attributes on the span.

## Notes

- This example emits **only** the `gen_ai.agent.*` namespace for the eight
  authorization attributes. The production backend additionally emits the legacy
  `agent.*` keys in parallel during an internal dashboard-migration window; that
  parallel set is deliberately omitted here.
- `agent.drift_score` is also emitted as a Prometheus **gauge** (separate from the
  span attribute) for alerting. The metric name is intentionally unchanged.
