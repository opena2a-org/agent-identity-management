package telemetry

import (
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// gen_ai.agent.* span attribute keys.
//
// These are the eight authorization-decision attributes AIM emits on the
// `fga.authorize` span. They are the namespaced form of the signals AIM has
// always recorded under the legacy `agent.*` keys; both sets are emitted in
// parallel ([CHIEF-CA] decision D1, 2026-06-26) until the internal Slide-14 /
// Grafana dashboards repoint, after which the `agent.*` set is retired.
//
// The names track the OTel GenAI SemConv proposal
// (open-telemetry/semantic-conventions-genai#291). They are emitted because AIM
// genuinely produces these signals at its authorization decision point — not to
// manufacture a producer for the proposal. The public otel-demo example emits
// ONLY this namespaced set.
const (
	GenAIAgentCapability    = "gen_ai.agent.capability"
	GenAIAgentPublicKeyAlgo = "gen_ai.agent.public_key.algorithm"
	GenAIAgentTrustScore    = "gen_ai.agent.trust.score"
	GenAIAgentTrustMethod   = "gen_ai.agent.trust.method"
	GenAIAgentDriftScore    = "gen_ai.agent.drift.score"
	GenAIAgentDriftMethod   = "gen_ai.agent.drift.method"
	GenAIAgentScanVerdict   = "gen_ai.agent.scan.verdict"
	GenAIAgentScanMethod    = "gen_ai.agent.scan.method"
)

// Producer-label method constants ([CHIEF-CA] decision D2, 2026-06-26).
//
// Each `*.method` attribute names HOW the paired signal is derived in AIM. They
// are stable, honest producer labels — not per-record provenance. In
// particular ScanMethodAIM reflects that AIM reads the scan verdict replicated
// from the Registry Agent Security Context; AIM does not observe which
// underlying scanner produced it, so claiming a specific scanner here would be
// fabricated provenance. If true per-scanner provenance is ever required it
// becomes a Registry schema change (escalation), not a constant edit here.
const (
	TrustMethodAIM = "aim/trust-calculator"      // TrustCalculator multi-factor trust score
	DriftMethodAIM = "aim/drift-capability-diff" // DriftDetectionService capability/MCP diff
	ScanMethodAIM  = "aim/registry-asc"          // verdict replicated from the Registry ASC
)

// AgentAuthzSignals carries the authorization signals available at the FGA
// decision point. A signal is emitted only when present, so a `*.method`
// attribute never lands without its paired score/verdict:
//   - Capability / PublicKeyAlgo / ScanVerdict: emitted when non-empty.
//   - TrustScore / DriftScore: emitted when non-nil (a real 0.0 is still a
//     value, so presence is modeled with a pointer, not a zero check).
//
// This mirrors the fail-open semantics of the legacy `agent.*` emission: a
// missing agent row or ASC row simply omits the corresponding attributes and
// never blocks or alters the authorization decision.
type AgentAuthzSignals struct {
	Capability    string
	PublicKeyAlgo string
	TrustScore    *float64
	DriftScore    *float64
	ScanVerdict   string
}

// SetGenAIAgentAttributes sets the present gen_ai.agent.* attributes (and their
// paired method labels) on span. It is the single source of truth for the
// namespaced emission so the /authorize and SDK-verification span paths cannot
// drift apart. No-op on a nil/non-recording span with no present signals.
func SetGenAIAgentAttributes(span trace.Span, s AgentAuthzSignals) {
	if span == nil {
		return
	}
	var kv []attribute.KeyValue
	if s.Capability != "" {
		kv = append(kv, attribute.String(GenAIAgentCapability, s.Capability))
	}
	if s.PublicKeyAlgo != "" {
		kv = append(kv, attribute.String(GenAIAgentPublicKeyAlgo, s.PublicKeyAlgo))
	}
	if s.TrustScore != nil {
		kv = append(kv,
			attribute.Float64(GenAIAgentTrustScore, *s.TrustScore),
			attribute.String(GenAIAgentTrustMethod, TrustMethodAIM),
		)
	}
	if s.DriftScore != nil {
		kv = append(kv,
			attribute.Float64(GenAIAgentDriftScore, *s.DriftScore),
			attribute.String(GenAIAgentDriftMethod, DriftMethodAIM),
		)
	}
	if s.ScanVerdict != "" {
		kv = append(kv,
			attribute.String(GenAIAgentScanVerdict, s.ScanVerdict),
			attribute.String(GenAIAgentScanMethod, ScanMethodAIM),
		)
	}
	if len(kv) > 0 {
		span.SetAttributes(kv...)
	}
}
