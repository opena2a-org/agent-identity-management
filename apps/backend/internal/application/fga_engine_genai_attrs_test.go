package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/telemetry"
)

// TestEmitSDKVerificationSpan_DualEmitsGenAIAttrs proves the real
// EmitSDKVerificationSpan call site dual-emits both the legacy agent.* keys and
// the namespaced gen_ai.agent.* set on the fga.authorize span ([CHIEF-CA] D1),
// sharing telemetry.SetGenAIAgentAttributes with the /authorize finalizer.
//
// db is nil, so fetchASCRiskSummary fails open and the scan/drift signals are
// absent — exactly the behavior the legacy path has when no ASC row exists.
func TestEmitSDKVerificationSpan_DualEmitsGenAIAttrs(t *testing.T) {
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	engine := &FGAEngine{tracer: tp.Tracer("test")}
	engine.EmitSDKVerificationSpan(
		context.Background(),
		uuid.New(),
		"Ed25519",
		0.73,
		"payments:transfer",
		true,
		"Action matches registered capabilities and passes all security policies",
	)

	var auth sdktrace.ReadOnlySpan
	for _, s := range rec.Ended() {
		if s.Name() == "fga.authorize" {
			auth = s
			break
		}
	}
	require.NotNil(t, auth, "expected an fga.authorize span")

	attrs := make(map[string]attribute.Value)
	for _, kv := range auth.Attributes() {
		attrs[string(kv.Key)] = kv.Value
	}

	// Dual-emit: legacy and namespaced keys both present.
	assert.Contains(t, attrs, "agent.trust_score", "legacy key retained (D1 dual-emit)")
	assert.Contains(t, attrs, "agent.public_key.algorithm")

	require.Contains(t, attrs, telemetry.GenAIAgentCapability)
	assert.Equal(t, "payments:transfer", attrs[telemetry.GenAIAgentCapability].AsString())
	require.Contains(t, attrs, telemetry.GenAIAgentPublicKeyAlgo)
	assert.Equal(t, "Ed25519", attrs[telemetry.GenAIAgentPublicKeyAlgo].AsString())
	require.Contains(t, attrs, telemetry.GenAIAgentTrustScore)
	assert.Equal(t, 0.73, attrs[telemetry.GenAIAgentTrustScore].AsFloat64())
	require.Contains(t, attrs, telemetry.GenAIAgentTrustMethod)
	assert.Equal(t, telemetry.TrustMethodAIM, attrs[telemetry.GenAIAgentTrustMethod].AsString())

	// No ASC summary (nil db) => scan/drift attrs absent, no orphan methods.
	assert.NotContains(t, attrs, telemetry.GenAIAgentScanVerdict)
	assert.NotContains(t, attrs, telemetry.GenAIAgentScanMethod)
	assert.NotContains(t, attrs, telemetry.GenAIAgentDriftScore)
	assert.NotContains(t, attrs, telemetry.GenAIAgentDriftMethod)
}
