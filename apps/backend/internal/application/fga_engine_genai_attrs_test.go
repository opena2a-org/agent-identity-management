package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/telemetry"
)

// TestEmitSDKVerificationSpan_DualEmitsGenAIAttrs proves the real
// EmitSDKVerificationSpan call site dual-emits both the legacy agent.* keys and
// the namespaced gen_ai.agent.* set on the fga.authorize span (the dual-emit
// decision of 2026-06-26), sharing telemetry.SetGenAIAgentAttributes with the
// /authorize finalizer.
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
		uuid.Nil,
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

// spanAttrsByName returns the attributes of the one ended span named name.
func spanAttrsByName(t *testing.T, rec *tracetest.SpanRecorder, name string) map[string]attribute.Value {
	t.Helper()
	var found sdktrace.ReadOnlySpan
	for _, s := range rec.Ended() {
		if s.Name() == name {
			require.Nil(t, found, "expected exactly one %s span", name)
			found = s
		}
	}
	require.NotNil(t, found, "expected an %s span", name)
	attrs := make(map[string]attribute.Value, len(found.Attributes()))
	for _, kv := range found.Attributes() {
		attrs[string(kv.Key)] = kv.Value
	}
	return attrs
}

// genaiAgentIDRepo answers GetByID with a fixed row (or with err), whatever ID
// it is asked for, so a test controls the row the /authorize finalizer resolves.
type genaiAgentIDRepo struct {
	domain.AgentRepository
	row *domain.Agent
	err error
}

func (r *genaiAgentIDRepo) GetByID(id uuid.UUID) (*domain.Agent, error) {
	return r.row, r.err
}

// authorizeSpanAttrs runs one /authorize decision through the real Authorize
// path (no FGA policy, so only the capability check runs and is granted) with
// a span recorder installed, and returns the fga.authorize span's attributes.
func authorizeSpanAttrs(t *testing.T, repo domain.AgentRepository, req *FGARequest) map[string]attribute.Value {
	t.Helper()
	rec := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	agentSvc := &AgentService{
		capabilityRepo: &aim08CapabilityRepo{capability: req.Capability},
		agentRepo:      repo,
	}
	engine := NewFGAEngine(nil, agentSvc, slog.Default())
	engine.tracer = tp.Tracer("test")
	t.Cleanup(func() { _ = engine.Shutdown(context.Background()) })

	result, err := engine.Authorize(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, "ALLOW", result.Outcome)
	return spanAttrsByName(t, rec, "fga.authorize")
}

// TestAuthorize_GenAIAgentIDIsTheResolvedAgentsAIPIdentifier pins the source of
// gen_ai.agent.id on the /authorize fga.authorize span: the AIP identifier
// (domain.BuildAgentDID) of the agent row GetAgent resolved, never a request
// field. The request carries the agent as a bare UUID, a different form from
// the expected did:aip value, so a value copied from the request fails.
func TestAuthorize_GenAIAgentIDIsTheResolvedAgentsAIPIdentifier(t *testing.T) {
	t.Run("a resolved agent carries its AIP identifier", func(t *testing.T) {
		agentID := uuid.New()
		req := &FGARequest{AgentID: agentID, Capability: "file:read", Resource: "/tmp/genai-agent-id"}
		attrs := authorizeSpanAttrs(t, &genaiAgentIDRepo{row: &domain.Agent{ID: agentID, TrustScore: 0.8}}, req)

		require.Contains(t, attrs, telemetry.GenAIAgentID)
		assert.Equal(t, domain.BuildAgentDID(agentID), attrs[telemetry.GenAIAgentID].AsString())
		assert.Equal(t, "did:aip:aim_"+agentID.String(), attrs[telemetry.GenAIAgentID].AsString())
		assert.NotEqual(t, req.AgentID.String(), attrs[telemetry.GenAIAgentID].AsString())

		// The legacy key is unchanged: the request's agent UUID.
		require.Contains(t, attrs, "agent.id")
		assert.Equal(t, agentID.String(), attrs["agent.id"].AsString())
	})

	t.Run("the identifier is the resolved row's, not the request's", func(t *testing.T) {
		req := &FGARequest{AgentID: uuid.New(), Capability: "file:read", Resource: "/tmp/genai-agent-id"}
		rowID := uuid.New()
		attrs := authorizeSpanAttrs(t, &genaiAgentIDRepo{row: &domain.Agent{ID: rowID}}, req)

		require.Contains(t, attrs, telemetry.GenAIAgentID)
		assert.Equal(t, domain.BuildAgentDID(rowID), attrs[telemetry.GenAIAgentID].AsString())
		assert.NotEqual(t, domain.BuildAgentDID(req.AgentID), attrs[telemetry.GenAIAgentID].AsString())
	})

	t.Run("a missing agent row emits no gen_ai.agent.id", func(t *testing.T) {
		req := &FGARequest{AgentID: uuid.New(), Capability: "file:read", Resource: "/tmp/genai-agent-id"}
		attrs := authorizeSpanAttrs(t, &genaiAgentIDRepo{err: errors.New("agent not found")}, req)

		assert.NotContains(t, attrs, telemetry.GenAIAgentID)
		// Fail-open: the decision and the request-derived attributes still land.
		assert.Equal(t, "ALLOW", attrs["fga.outcome"].AsString())
		assert.Equal(t, req.AgentID.String(), attrs["agent.id"].AsString())
		assert.Equal(t, "file:read", attrs[telemetry.GenAIAgentCapability].AsString())
	})

	t.Run("a nil agent row emits no gen_ai.agent.id", func(t *testing.T) {
		req := &FGARequest{AgentID: uuid.New(), Capability: "file:read", Resource: "/tmp/genai-agent-id"}
		attrs := authorizeSpanAttrs(t, &genaiAgentIDRepo{}, req)

		assert.NotContains(t, attrs, telemetry.GenAIAgentID)
		assert.Equal(t, "ALLOW", attrs["fga.outcome"].AsString())
	})
}

// TestEmitSDKVerificationSpan_GenAIAgentIDIsTheVerifiedAgentsAIPIdentifier pins
// the source of gen_ai.agent.id on the SDK-verification fga.authorize span: the
// AIP identifier of the agent the handler verified, never the request's agent
// id. The two IDs differ here so a value built from the request fails.
func TestEmitSDKVerificationSpan_GenAIAgentIDIsTheVerifiedAgentsAIPIdentifier(t *testing.T) {
	emit := func(t *testing.T, requestAgentID, verifiedAgentID uuid.UUID) map[string]attribute.Value {
		t.Helper()
		rec := tracetest.NewSpanRecorder()
		tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(rec))
		t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

		engine := &FGAEngine{tracer: tp.Tracer("test")}
		engine.EmitSDKVerificationSpan(
			context.Background(),
			requestAgentID,
			verifiedAgentID,
			"Ed25519",
			0.73,
			"payments:transfer",
			true,
			"Action matches registered capabilities and passes all security policies",
		)
		return spanAttrsByName(t, rec, "fga.authorize")
	}

	t.Run("a verified agent carries its AIP identifier", func(t *testing.T) {
		requestAgentID, verifiedAgentID := uuid.New(), uuid.New()
		attrs := emit(t, requestAgentID, verifiedAgentID)

		require.Contains(t, attrs, telemetry.GenAIAgentID)
		assert.Equal(t, domain.BuildAgentDID(verifiedAgentID), attrs[telemetry.GenAIAgentID].AsString())
		assert.NotEqual(t, domain.BuildAgentDID(requestAgentID), attrs[telemetry.GenAIAgentID].AsString())
		assert.NotEqual(t, requestAgentID.String(), attrs[telemetry.GenAIAgentID].AsString())

		// The legacy key is unchanged.
		assert.Equal(t, requestAgentID.String(), attrs["agent.id"].AsString())
	})

	t.Run("no verified agent emits no gen_ai.agent.id", func(t *testing.T) {
		attrs := emit(t, uuid.New(), uuid.Nil)
		assert.NotContains(t, attrs, telemetry.GenAIAgentID)
		assert.Contains(t, attrs, telemetry.GenAIAgentCapability)
	})
}
