package trace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/telemetry"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// logSink keeps every log record exported to it.
type logSink struct {
	mu      sync.Mutex
	records []sdklog.Record
}

func (s *logSink) Export(_ context.Context, records []sdklog.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range records {
		s.records = append(s.records, r.Clone())
	}
	return nil
}

func (s *logSink) Shutdown(context.Context) error   { return nil }
func (s *logSink) ForceFlush(context.Context) error { return nil }

// text is every exported log record's body, attributes and trace context.
func (s *logSink) text() (string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var b strings.Builder
	for _, r := range s.records {
		fmt.Fprintf(&b, "%s trace=%s span=%s", r.Body().String(), r.TraceID(), r.SpanID())
		r.WalkAttributes(func(kv otellog.KeyValue) bool {
			fmt.Fprintf(&b, " %s=%s", kv.Key, kv.Value.String())
			return true
		})
		b.WriteString("\n")
	}
	return b.String(), len(s.records)
}

// baggageToSpans copies the baggage of a span's parent context onto the span,
// as a deployment's baggage-copying span processor does. With it installed,
// any baggage that reaches a request's context reaches its exported spans.
type baggageToSpans struct{}

func (baggageToSpans) OnStart(parent context.Context, s sdktrace.ReadWriteSpan) {
	for _, m := range baggage.FromContext(parent).Members() {
		s.SetAttributes(attribute.String("baggage."+m.Key(), m.Value()))
	}
}
func (baggageToSpans) OnEnd(sdktrace.ReadOnlySpan)      {}
func (baggageToSpans) Shutdown(context.Context) error   { return nil }
func (baggageToSpans) ForceFlush(context.Context) error { return nil }

// baggageToLogs does the same for log records.
type baggageToLogs struct{}

func (baggageToLogs) OnEmit(ctx context.Context, r *sdklog.Record) error {
	for _, m := range baggage.FromContext(ctx).Members() {
		r.AddAttributes(otellog.String("baggage."+m.Key(), m.Value()))
	}
	return nil
}
func (baggageToLogs) Enabled(context.Context, sdklog.EnabledParameters) bool { return true }
func (baggageToLogs) Shutdown(context.Context) error                         { return nil }
func (baggageToLogs) ForceFlush(context.Context) error                       { return nil }

// installTelemetry makes in-memory exporters the global trace and log
// destinations, with the propagator the server registers, until the test
// ends.
func installTelemetry(t *testing.T) (*tracetest.InMemoryExporter, *logSink) {
	t.Helper()
	spans := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(baggageToSpans{}), sdktrace.WithSyncer(spans))
	logs := &logSink{}
	lp := sdklog.NewLoggerProvider(sdklog.WithProcessor(baggageToLogs{}), sdklog.WithProcessor(sdklog.NewSimpleProcessor(logs)))

	prevTP, prevProp, prevLP := otel.GetTracerProvider(), otel.GetTextMapPropagator(), global.GetLoggerProvider()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(telemetry.Propagator())
	global.SetLoggerProvider(lp)
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
		global.SetLoggerProvider(prevLP)
		_ = tp.Shutdown(context.Background())
		_ = lp.Shutdown(context.Background())
	})
	return spans, logs
}

// telemetryApp serves one route behind the trace middleware. Its handler
// does what AIM's request handlers do with telemetry: it starts and ends a
// span, writes a log line through the slog bridge, and stamps a record. When
// applyPropagator is set, a middleware ahead of the trace middleware applies
// the registered propagator to the request, as the idiomatic middleware does;
// that is the control the assertions must detect.
func telemetryApp(applyPropagator bool) *fiber.App {
	app := fiber.New()
	if applyPropagator {
		app.Use(func(c fiber.Ctx) error {
			header := http.Header(c.GetReqHeaders())
			c.SetContext(otel.GetTextMapPropagator().Extract(c.Context(), propagation.HeaderCarrier(header)))
			return c.Next()
		})
	}
	app.Use(Middleware())
	app.Get("/", func(c fiber.Ctx) error {
		ctx, span := otel.Tracer("aim/fga").Start(c.Context(), "fga.decision")
		span.SetAttributes(attribute.String("aim.decision", "allow"))
		otelslog.NewLogger("aim/fga").InfoContext(ctx, "decision", "decision", "allow")
		span.End()

		d := draft(eventA)
		if err := Stamp(c.Context(), &d, nil); err != nil {
			return err
		}
		return c.JSON(map[string]any{
			"drafts":  []record.Draft{d},
			"headers": c.GetReqHeaders(),
		})
	})
	return app
}

func spansText(spans []tracetest.SpanStub) string {
	var b strings.Builder
	for _, s := range spans {
		fmt.Fprintf(&b, "%s trace=%s state=%q parent=%s/%s parentstate=%q attrs=%v events=%v links=%v\n",
			s.Name, s.SpanContext.TraceID(), s.SpanContext.TraceState().String(),
			s.Parent.TraceID(), s.Parent.SpanID(), s.Parent.TraceState().String(),
			s.Attributes, s.Events, s.Links)
	}
	return b.String()
}

func telemetryGet(t *testing.T, app *fiber.App, header http.Header) stampedResponse {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for name, values := range header {
		for _, v := range values {
			req.Header.Add(name, v)
		}
	}
	res, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	require.NoError(t, err)
	defer res.Body.Close()
	var body bytes.Buffer
	_, err = body.ReadFrom(res.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, res.StatusCode, body.String())
	var out stampedResponse
	require.NoError(t, json.Unmarshal(body.Bytes(), &out))
	return out
}

// Canaries sent in tracestate, baggage and traceparent's parent-id reach no
// span or log record AIM exports, and AIM's spans do not continue the
// caller's trace. The control applies the registered propagator to the same
// request and finds every canary in the exported telemetry, so the check can
// see what it looks for.
func TestNoExportedSpanOrLogCarriesTheCallersTraceContext(t *testing.T) {
	const (
		baggageCanary    = "canary-baggage-user-5d4c3b2a"
		tracestateCanary = "canary-tracestate-7f3e"
	)
	header := http.Header{
		"Traceparent": {callerHeader},
		"Tracestate":  {"vendor=" + tracestateCanary},
		"Baggage":     {"userId=" + baggageCanary},
	}
	canaries := []string{baggageCanary, tracestateCanary, callerParentID, callerTraceID}

	t.Run("the trace middleware", func(t *testing.T) {
		spans, logs := installTelemetry(t)
		out := telemetryGet(t, telemetryApp(false), header)

		// The canaries were in the request as sent.
		require.Equal(t, []string{"userId=" + baggageCanary}, out.Headers["Baggage"])
		require.Equal(t, []string{"vendor=" + tracestateCanary}, out.Headers["Tracestate"])
		require.Equal(t, []string{callerHeader}, out.Headers["Traceparent"])

		exported := spans.GetSpans()
		require.Len(t, exported, 1, "the handler's span was exported")
		require.False(t, exported[0].Parent.IsValid(), "the span does not continue the caller's trace")
		require.Empty(t, exported[0].SpanContext.TraceState().String())
		spanText := spansText(exported)
		logText, logCount := logs.text()
		require.Equal(t, 1, logCount, "the handler's log line was exported")
		require.Contains(t, logText, "decision=allow")
		for _, canary := range canaries {
			require.NotContains(t, spanText, canary)
			require.NotContains(t, logText, canary)
		}
		// The one field kept still reached the record.
		require.Equal(t, callerTraceID, out.Drafts[0].Personal["opena2a"].(map[string]any)["request_trace_id"])
	})

	t.Run("control: the registered propagator applied to the request", func(t *testing.T) {
		spans, logs := installTelemetry(t)
		telemetryGet(t, telemetryApp(true), header)

		exported := spans.GetSpans()
		require.Len(t, exported, 1)
		spanText := spansText(exported)
		logText, _ := logs.text()
		for _, canary := range canaries {
			require.Contains(t, spanText, canary, "the span check sees %s", canary)
		}
		require.Contains(t, logText, baggageCanary, "the log check sees the baggage")
		require.Contains(t, logText, callerTraceID, "the log check sees the caller's trace")
	})
}
