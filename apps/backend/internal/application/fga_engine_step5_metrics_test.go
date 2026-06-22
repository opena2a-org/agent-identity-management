package application

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// Step 5 observability (issue #131, Stage 3). These tests pin two things:
//  1. checkIntentSync labels every outcome with the right Status, so the
//     silent fail-open becomes measurable (classified / abstain / fail_open).
//  2. recordIntentCheck emits fga.intent_checks with the risk-tier / mode /
//     status / blocked attributes operators derive the rates from.

// TestCheckIntentSyncStatus asserts the Status field for the daemon response
// shapes Step 5 can encounter. fail_open has two sub-modes (daemon unreachable
// and undecodable body); both must be counted, not dropped.
func TestCheckIntentSyncStatus(t *testing.T) {
	req := &FGARequest{
		AgentID:    uuid.New(),
		Capability: "file:read",
		Resource:   "/tmp/step5-status-test",
		Action:     "read",
	}

	t.Run("non-empty class is classified", func(t *testing.T) {
		srv := stubDaemon(t, map[string]interface{}{
			"attackClass": "exfiltration_pattern",
			"confidence":  0.95,
		})
		engine := &FGAEngine{daemonURL: srv.URL, logger: slog.Default()}
		got := engine.checkIntentSync(context.Background(), req)
		require.NotNil(t, got)
		assert.Equal(t, intentStatusClassified, got.Status)
		assert.True(t, got.Blocked)
	})

	t.Run("empty class is abstain", func(t *testing.T) {
		srv := stubDaemon(t, map[string]interface{}{
			"attackClass": "",
			"confidence":  0.95,
		})
		engine := &FGAEngine{daemonURL: srv.URL, logger: slog.Default()}
		got := engine.checkIntentSync(context.Background(), req)
		require.NotNil(t, got)
		assert.Equal(t, intentStatusAbstain, got.Status)
		assert.False(t, got.Blocked)
	})

	t.Run("daemon unreachable is fail_open", func(t *testing.T) {
		// Reserved-TEST-net address that refuses fast; the 800ms client
		// timeout is the upper bound.
		engine := &FGAEngine{daemonURL: "http://127.0.0.1:1", logger: slog.Default()}
		got := engine.checkIntentSync(context.Background(), req)
		require.NotNil(t, got)
		assert.Equal(t, intentStatusFailOpen, got.Status)
		assert.False(t, got.Blocked)
	})

	t.Run("undecodable body is fail_open not nil", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("this is not json"))
		}))
		t.Cleanup(srv.Close)
		engine := &FGAEngine{daemonURL: srv.URL, logger: slog.Default()}
		got := engine.checkIntentSync(context.Background(), req)
		// Previously this returned nil and the fail-open vanished from
		// telemetry. It must now be a measured fail_open result.
		require.NotNil(t, got)
		assert.Equal(t, intentStatusFailOpen, got.Status)
		assert.False(t, got.Blocked)
	})
}

// TestRecordIntentCheckEmitsCounter asserts fga.intent_checks carries the four
// attributes per evaluation, and that a nil result is counted as fail_open
// (the worst case must never be a silent no-op).
func TestRecordIntentCheckEmitsCounter(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	prev := otel.GetMeterProvider()
	otel.SetMeterProvider(mp)
	t.Cleanup(func() { otel.SetMeterProvider(prev) })

	// Instruments bind at construction, so build the engine after the
	// provider is installed. db/agentSvc nil is fine for this path.
	engine := NewFGAEngine(nil, nil, nil)
	t.Cleanup(func() { _ = engine.Shutdown(context.Background()) })

	ctx := context.Background()
	engine.recordIntentCheck(ctx, "HIGH", "sync", &IntentCheckResult{Status: intentStatusClassified, Blocked: true})
	engine.recordIntentCheck(ctx, "MEDIUM", "async", &IntentCheckResult{Status: intentStatusAbstain, Blocked: false})
	engine.recordIntentCheck(ctx, "HIGH", "sync", nil) // worst case: count as fail_open

	dps := collectCounter(t, reader, "fga.intent_checks")
	require.Len(t, dps, 3, "expected one series per distinct attribute set")

	classified := findDP(dps, "HIGH", "sync", intentStatusClassified, true)
	require.NotNil(t, classified, "missing classified/sync/HIGH/blocked series")
	assert.Equal(t, int64(1), classified.Value)

	abstain := findDP(dps, "MEDIUM", "async", intentStatusAbstain, false)
	require.NotNil(t, abstain, "missing abstain/async/MEDIUM series")
	assert.Equal(t, int64(1), abstain.Value)

	failOpen := findDP(dps, "HIGH", "sync", intentStatusFailOpen, false)
	require.NotNil(t, failOpen, "nil result must be counted as fail_open")
	assert.Equal(t, int64(1), failOpen.Value)
}

// --- helpers ---

func stubDaemon(t *testing.T, resp map[string]interface{}) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/infer" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func collectCounter(t *testing.T, reader sdkmetric.Reader, name string) []metricdata.DataPoint[int64] {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == name {
				sum, ok := m.Data.(metricdata.Sum[int64])
				require.True(t, ok, "metric %s is not an int64 Sum", name)
				return sum.DataPoints
			}
		}
	}
	t.Fatalf("metric %s not collected", name)
	return nil
}

func findDP(dps []metricdata.DataPoint[int64], tier, mode, status string, blocked bool) *metricdata.DataPoint[int64] {
	for i := range dps {
		set := dps[i].Attributes
		if attrStr(set, "fga.risk_tier") == tier &&
			attrStr(set, "fga.intent_mode") == mode &&
			attrStr(set, "fga.intent_status") == status &&
			attrBool(set, "fga.intent_blocked") == blocked {
			return &dps[i]
		}
	}
	return nil
}

func attrStr(set attribute.Set, key string) string {
	v, ok := set.Value(attribute.Key(key))
	if !ok {
		return ""
	}
	return v.AsString()
}

func attrBool(set attribute.Set, key string) bool {
	v, ok := set.Value(attribute.Key(key))
	if !ok {
		return false
	}
	return v.AsBool()
}
