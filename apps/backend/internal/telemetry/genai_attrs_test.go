package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func f64(v float64) *float64 { return &v }

// recordGenAIAttrs runs SetGenAIAgentAttributes against a recorded span and
// returns the resulting attributes keyed by name.
func recordGenAIAttrs(t *testing.T, s AgentAuthzSignals) map[string]attribute.Value {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	_, span := tp.Tracer("test").Start(context.Background(), "fga.authorize")
	SetGenAIAgentAttributes(span, s)
	span.End()

	ended := rec.Ended()
	require.Len(t, ended, 1)
	out := make(map[string]attribute.Value, len(ended[0].Attributes()))
	for _, kv := range ended[0].Attributes() {
		out[string(kv.Key)] = kv.Value
	}
	return out
}

// TestSetGenAIAgentAttributes_Full asserts all eight gen_ai.agent.* attributes
// land with the right values, and that each *.method carries the [CHIEF-CA] D2
// producer-label constant.
func TestSetGenAIAgentAttributes_Full(t *testing.T) {
	attrs := recordGenAIAttrs(t, AgentAuthzSignals{
		Capability:    "payments:transfer",
		PublicKeyAlgo: "Ed25519+ML-DSA-65",
		TrustScore:    f64(0.73),
		DriftScore:    f64(0.12),
		ScanVerdict:   "clean",
	})

	require.Len(t, attrs, 8, "expected exactly the eight gen_ai.agent.* attrs")

	assert.Equal(t, "payments:transfer", attrs[GenAIAgentCapability].AsString())
	assert.Equal(t, "Ed25519+ML-DSA-65", attrs[GenAIAgentPublicKeyAlgo].AsString())
	assert.Equal(t, 0.73, attrs[GenAIAgentTrustScore].AsFloat64())
	assert.Equal(t, TrustMethodAIM, attrs[GenAIAgentTrustMethod].AsString())
	assert.Equal(t, 0.12, attrs[GenAIAgentDriftScore].AsFloat64())
	assert.Equal(t, DriftMethodAIM, attrs[GenAIAgentDriftMethod].AsString())
	assert.Equal(t, "clean", attrs[GenAIAgentScanVerdict].AsString())
	assert.Equal(t, ScanMethodAIM, attrs[GenAIAgentScanMethod].AsString())

	// Method labels are honest, stable producer identifiers.
	assert.Equal(t, "aim/trust-calculator", TrustMethodAIM)
	assert.Equal(t, "aim/drift-capability-diff", DriftMethodAIM)
	assert.Equal(t, "aim/registry-asc", ScanMethodAIM)
}

// TestSetGenAIAgentAttributes_Empty asserts a fully-empty signal set emits
// nothing — no orphan method labels, no zero-valued scores.
func TestSetGenAIAgentAttributes_Empty(t *testing.T) {
	attrs := recordGenAIAttrs(t, AgentAuthzSignals{})
	assert.Empty(t, attrs, "no signals present => no gen_ai.agent.* attributes")
}

// TestSetGenAIAgentAttributes_Partial asserts a *.method never lands without
// its paired score/verdict: trust present, drift and scan absent.
func TestSetGenAIAgentAttributes_Partial(t *testing.T) {
	attrs := recordGenAIAttrs(t, AgentAuthzSignals{
		Capability: "fs:read",
		TrustScore: f64(0.5),
	})

	assert.Contains(t, attrs, GenAIAgentCapability)
	assert.Contains(t, attrs, GenAIAgentTrustScore)
	assert.Contains(t, attrs, GenAIAgentTrustMethod)

	assert.NotContains(t, attrs, GenAIAgentDriftScore)
	assert.NotContains(t, attrs, GenAIAgentDriftMethod, "no orphan drift.method without drift.score")
	assert.NotContains(t, attrs, GenAIAgentScanVerdict)
	assert.NotContains(t, attrs, GenAIAgentScanMethod, "no orphan scan.method without scan.verdict")
	assert.NotContains(t, attrs, GenAIAgentPublicKeyAlgo)
}

// TestSetGenAIAgentAttributes_ZeroScoreIsEmitted asserts a real 0.0 score is a
// value (presence is modeled with a pointer, not a zero check), so trust.score
// and its method still land.
func TestSetGenAIAgentAttributes_ZeroScoreIsEmitted(t *testing.T) {
	attrs := recordGenAIAttrs(t, AgentAuthzSignals{
		TrustScore: f64(0.0),
		DriftScore: f64(0.0),
	})

	require.Contains(t, attrs, GenAIAgentTrustScore)
	assert.Equal(t, 0.0, attrs[GenAIAgentTrustScore].AsFloat64())
	assert.Contains(t, attrs, GenAIAgentTrustMethod)
	require.Contains(t, attrs, GenAIAgentDriftScore)
	assert.Equal(t, 0.0, attrs[GenAIAgentDriftScore].AsFloat64())
	assert.Contains(t, attrs, GenAIAgentDriftMethod)
}
