package application

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// An agent with no stored FGA policy for the capability is decided under the
// default policy, so the attribute, context and chain checks run and the
// intent check is dispatched by the request's risk level; and a request may
// carry its own step 1 (#656). Before, an agent without a policy got step 1
// alone, and step 1 was always AgentService.HasCapability.

// defaultPolicyEngine wires an engine over sqlmock whose policy query finds
// no row. Every other DB touch (the attestation and tool-call inserts, the
// finalizer's ASC read) is best-effort in the engine, so sqlmock's unexpected
// call errors exercise those paths. Spans land in the returned recorder.
func defaultPolicyEngine(t *testing.T, agentSvc *AgentService) (*FGAEngine, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	prevTP := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		_ = tp.Shutdown(context.Background())
	})

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	mock.ExpectQuery("FROM fga_policies").WillReturnRows(sqlmock.NewRows(aim08PolicyColumns))

	engine := NewFGAEngine(db, agentSvc, slog.Default())
	// Nothing listens here, so a dispatched intent check fails open at once.
	engine.SetDaemonURL("http://127.0.0.1:1")
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = engine.Shutdown(ctx)
	})
	return engine, recorder
}

// fgaStepSpans returns the ended fga.authorize span and the fga.step values
// of its ended children in start order, without the worker span of the
// detached intent check, which carries the same step and may still run.
func fgaStepSpans(t *testing.T, rec *tracetest.SpanRecorder) (sdktrace.ReadOnlySpan, []string) {
	t.Helper()
	var parent sdktrace.ReadOnlySpan
	for _, s := range rec.Ended() {
		if s.Name() == "fga.authorize" {
			parent = s
		}
	}
	require.NotNil(t, parent, "expected an fga.authorize span")
	var steps []string
	for _, s := range rec.Ended() {
		if s.Parent().SpanID() != parent.SpanContext().SpanID() || strings.HasSuffix(s.Name(), ".worker") {
			continue
		}
		steps = append(steps, spanAttributes(s)["fga.step"].AsString())
	}
	return parent, steps
}

func spanAttributes(s sdktrace.ReadOnlySpan) map[string]attribute.Value {
	attrs := make(map[string]attribute.Value, len(s.Attributes()))
	for _, kv := range s.Attributes() {
		attrs[string(kv.Key)] = kv.Value
	}
	return attrs
}

var (
	fourFGASteps = []string{"capability_check", "attribute_check", "context_check", "chain_check"}
	fiveFGASteps = append(append([]string{}, fourFGASteps...), "intent_check_async")
)

func TestAuthorize_NoStoredPolicyRunsEveryStepUnderTheDefaultPolicy(t *testing.T) {
	for _, tc := range []struct {
		name       string
		capability string
		riskLevel  string
		wantSteps  []string
	}{
		{name: "a critical capability dispatches the intent check", capability: "payment:refund", wantSteps: fiveFGASteps},
		{name: "a high capability dispatches the intent check", capability: "db:delete", wantSteps: fiveFGASteps},
		{name: "a medium capability dispatches the intent check", capability: "db:write", wantSteps: fiveFGASteps},
		{name: "a low capability skips the intent check", capability: "db:read", wantSteps: fourFGASteps},
		{name: "the request's risk level sets the tier", capability: "db:read", riskLevel: "high", wantSteps: fiveFGASteps},
		{name: "an unknown risk level on the request is detected from the capability", capability: "db:read", riskLevel: "bogus", wantSteps: fourFGASteps},
	} {
		t.Run(tc.name, func(t *testing.T) {
			agentSvc := &AgentService{
				capabilityRepo: &aim08CapabilityRepo{capability: tc.capability},
				agentRepo:      &aim08AgentRepo{},
			}
			engine, rec := defaultPolicyEngine(t, agentSvc)
			req := &FGARequest{AgentID: uuid.New(), Capability: tc.capability, Resource: "customers", RiskLevel: tc.riskLevel}

			result, err := engine.Authorize(context.Background(), req)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, "ALLOW", result.Outcome)
			assert.Equal(t, tc.wantSteps, result.StepsTriggered)

			parent, steps := fgaStepSpans(t, rec)
			attrs := spanAttributes(parent)
			assert.Equal(t, "default", attrs["fga.policy"].AsString())
			assert.Equal(t, "ALLOW", attrs["fga.outcome"].AsString())
			assert.Equal(t, tc.wantSteps, attrs["fga.steps_triggered"].AsStringSlice())
			assert.Equal(t, tc.wantSteps, steps, "one child span per step that ran")
		})
	}
}

func TestAuthorize_TheRequestsCapabilityCheckIsStep1(t *testing.T) {
	const notVerified = "Agent not verified - all actions denied"
	newRequest := func(check CapabilityCheck) *FGARequest {
		return &FGARequest{AgentID: uuid.New(), Capability: "payment:refund", Resource: "orders", CapabilityCheck: check}
	}

	t.Run("a denial carries the check's reason and stops at step 1", func(t *testing.T) {
		// No AgentService: a step 1 that read it would fail.
		engine, rec := defaultPolicyEngine(t, nil)
		result, err := engine.Authorize(context.Background(), newRequest(func(context.Context) (bool, string, error) {
			return false, notVerified, nil
		}))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, "DENY", result.Outcome)
		assert.Equal(t, "capability_check", result.DeniedBy)
		assert.Equal(t, notVerified, result.DeniedReason)
		assert.Equal(t, []string{"capability_check"}, result.StepsTriggered)

		parent, steps := fgaStepSpans(t, rec)
		assert.Equal(t, []string{"capability_check"}, steps)
		attrs := spanAttributes(parent)
		assert.Equal(t, "DENY", attrs["fga.outcome"].AsString())
		assert.Equal(t, "capability_check", attrs["fga.denied_by"].AsString())
		for _, s := range rec.Ended() {
			if s.Name() == "fga.capability_check" {
				child := spanAttributes(s)
				assert.False(t, child["fga.allowed"].AsBool())
				assert.Equal(t, notVerified, child["fga.denied_reason"].AsString())
			}
		}
	})

	t.Run("a denial without a reason gets the engine's", func(t *testing.T) {
		engine, _ := defaultPolicyEngine(t, nil)
		result, err := engine.Authorize(context.Background(), newRequest(func(context.Context) (bool, string, error) {
			return false, "", nil
		}))
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, capabilityDeniedReason, result.DeniedReason)
	})

	t.Run("an allow runs the remaining steps", func(t *testing.T) {
		engine, rec := defaultPolicyEngine(t, nil)
		result, err := engine.Authorize(context.Background(), newRequest(func(context.Context) (bool, string, error) {
			return true, "Action allowed by MONITORING mode (no capabilities registered yet) - logged for review", nil
		}))
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, "ALLOW", result.Outcome)
		assert.Equal(t, fiveFGASteps, result.StepsTriggered)
		_, steps := fgaStepSpans(t, rec)
		assert.Equal(t, fiveFGASteps, steps)
	})

	t.Run("a failed check is the engine's error", func(t *testing.T) {
		engine, _ := defaultPolicyEngine(t, nil)
		result, err := engine.Authorize(context.Background(), newRequest(func(context.Context) (bool, string, error) {
			return false, "", errors.New("enforcement mode unreadable")
		}))
		require.EqualError(t, err, "enforcement mode unreadable")
		require.NotNil(t, result)
		assert.Equal(t, "ERROR", result.Outcome)
		assert.False(t, result.Allowed)
	})
}

func TestDefaultPolicyRiskLevel(t *testing.T) {
	for detected, want := range map[string]string{"critical": "HIGH", "high": "HIGH", "medium": "MEDIUM", "low": "LOW"} {
		assert.Equal(t, want, policyRiskLevel(detected), detected)
	}
	for _, tc := range []struct {
		capability, riskLevel, want string
	}{
		{"payment:refund", "", "HIGH"},
		{"db:read", "", "LOW"},
		{"db:read", "HIGH", "HIGH"},
		{"db:read", "medium", "MEDIUM"},
		{"db:read", "bogus", "LOW"},
	} {
		policy := defaultPolicy(&FGARequest{AgentID: uuid.New(), Capability: tc.capability, RiskLevel: tc.riskLevel})
		assert.Equal(t, tc.want, policy.RiskLevel, "%s with risk level %q", tc.capability, tc.riskLevel)
		assert.Equal(t, tc.capability, policy.Capability)
		assert.Empty(t, policy.AllowedAttributes)
		assert.Empty(t, policy.DeniedAttributes)
		assert.Empty(t, policy.AllowedActions)
		assert.Empty(t, policy.ContextRules)
		assert.Empty(t, policy.ChainRules)
	}
}
