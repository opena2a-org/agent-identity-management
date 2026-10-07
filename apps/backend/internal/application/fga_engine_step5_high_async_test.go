package application

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// Issue #131: Step 5 never denies. A HIGH-risk request takes the async path,
// and a daemon verdict naming a bucket alias reaches no part of the response
// or the trace. The request below is benign (deleting a build cache), the
// case the old synchronous HIGH path denied as tool_misuse.
func TestStep5HighRiskIsAsyncAndNeverDenies(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	prevMP, prevTP := otel.GetMeterProvider(), otel.GetTracerProvider()
	otel.SetMeterProvider(mp)
	otel.SetTracerProvider(tp)
	t.Cleanup(func() {
		otel.SetMeterProvider(prevMP)
		otel.SetTracerProvider(prevTP)
		_ = tp.Shutdown(context.Background())
	})

	var daemonCalls atomic.Int32
	daemon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/infer" {
			http.NotFound(w, r)
			return
		}
		daemonCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"attackClass":"tool_misuse","confidence":0.99,"classification":"classified"}`))
	}))
	t.Cleanup(daemon.Close)

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	req := &FGARequest{
		AgentID:    uuid.New(),
		Capability: "files:delete",
		Resource:   "/home/ci/project/.cache/build/",
		Action:     "delete",
	}
	mock.ExpectQuery("FROM fga_policies").WillReturnRows(
		sqlmock.NewRows(aim08PolicyColumns).AddRow(
			uuid.New().String(), req.AgentID.String(), req.Capability,
			"{}", "{}", "{}", "{}",
			[]byte(`{}`), []byte(`{}`), []byte(`{}`), []byte(`{}`), "HIGH",
		),
	)
	agentSvc := &AgentService{
		capabilityRepo: &aim08CapabilityRepo{capability: req.Capability},
		agentRepo:      &aim08AgentRepo{},
	}
	engine := NewFGAEngine(db, agentSvc, slog.Default())
	engine.SetDaemonURL(daemon.URL)

	result, err := engine.Authorize(context.Background(), req)
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, "ALLOW", result.Outcome)
	assert.Empty(t, result.DeniedBy)
	assert.Empty(t, result.DeniedReason)
	assert.Nil(t, result.IntentCheck)
	assert.Contains(t, result.StepsTriggered, "intent_check_async")
	assert.NotContains(t, result.StepsTriggered, "intent_check_sync")

	body, err := json.Marshal(result)
	require.NoError(t, err)
	for _, alias := range []string{"prompt_injection", "tool_misuse", "exfiltration_pattern", "data_extraction", "Intent classified"} {
		assert.NotContains(t, string(body), alias)
	}
	assert.NotContains(t, string(body), "intentCheck")

	// Drain the worker pool so the detached check has run before the
	// counter and spans are read.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, engine.Shutdown(shutdownCtx))
	assert.Equal(t, int32(1), daemonCalls.Load(), "the async check must still reach the daemon")

	dps := collectCounter(t, reader, "fga.intent_checks")
	require.Len(t, dps, 1)
	dp := findDP(dps, "HIGH", "async", intentStatusClassified, false)
	require.NotNil(t, dp, "expected one HIGH/async/classified/unblocked series")
	assert.Equal(t, int64(1), dp.Value)

	require.NoError(t, tp.ForceFlush(context.Background()))
	spans := recorder.Ended()
	require.NotEmpty(t, spans)
	for _, s := range spans {
		for _, kv := range s.Attributes() {
			assert.NotEqual(t, "fga.intent_class", string(kv.Key), "span %s carries fga.intent_class", s.Name())
		}
	}
}
