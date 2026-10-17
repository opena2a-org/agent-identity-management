package middleware

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// spanTestApp returns an app whose requests pass through the span
// middleware on a tracer provider of their own, and the recorder of the
// spans that provider ends.
func spanTestApp(t *testing.T, outer ...fiber.Handler) (*fiber.App, *tracetest.SpanRecorder) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	app := fiber.New()
	for _, h := range outer {
		app.Use(h)
	}
	app.Use(httpServerSpans(func() oteltrace.TracerProvider { return tp }))
	return app, recorder
}

func spanRequest(t *testing.T, app *fiber.App, method, target string, header map[string]string) int {
	t.Helper()
	req := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	return resp.StatusCode
}

func onlySpan(t *testing.T, recorder *tracetest.SpanRecorder) sdktrace.ReadOnlySpan {
	t.Helper()
	ended := recorder.Ended()
	require.Len(t, ended, 1, "ended spans")
	return ended[0]
}

func attributesOf(span sdktrace.ReadOnlySpan) map[attribute.Key]attribute.Value {
	attrs := map[attribute.Key]attribute.Value{}
	for _, kv := range span.Attributes() {
		attrs[kv.Key] = kv.Value
	}
	return attrs
}

// A request a route answers gives one SERVER span named from the matched
// route, with the HTTP semantic-convention attributes, and none of the query
// string or of a header other than User-Agent.
func TestHTTPServerSpans_NamesTheSpanFromTheMatchedRoute(t *testing.T) {
	app, recorder := spanTestApp(t)
	app.Get("/items/:id", func(c fiber.Ctx) error { return c.SendString("ok") })

	status := spanRequest(t, app, fiber.MethodGet, "/items/42?filter=query-marker", map[string]string{
		"User-Agent":    "span-test/1.0",
		"Authorization": "Bearer header-marker",
	})
	require.Equal(t, fiber.StatusOK, status)

	span := onlySpan(t, recorder)
	assert.Equal(t, "GET /items/:id", span.Name())
	assert.Equal(t, oteltrace.SpanKindServer, span.SpanKind())
	attrs := attributesOf(span)
	assert.Equal(t, "GET", attrs["http.request.method"].AsString())
	assert.Equal(t, "/items/42", attrs["url.path"].AsString())
	assert.Equal(t, "http", attrs["url.scheme"].AsString())
	assert.Equal(t, "/items/:id", attrs["http.route"].AsString())
	assert.Equal(t, int64(200), attrs["http.response.status_code"].AsInt64())
	assert.Equal(t, "example.com", attrs["server.address"].AsString())
	assert.Equal(t, int64(80), attrs["server.port"].AsInt64())
	assert.Equal(t, "1.1", attrs["network.protocol.version"].AsString())
	assert.NotEmpty(t, attrs["client.address"].AsString())
	assert.Equal(t, "span-test/1.0", attrs["user_agent.original"].AsString())
	assert.Equal(t, codes.Unset, span.Status().Code)
	for key, value := range attrs {
		for _, marker := range []string{"query-marker", "filter=", "header-marker", "Bearer"} {
			assert.NotContains(t, string(key)+"="+value.Emit(), marker, "attribute %s records the query string or a header", key)
		}
	}
}

// A 5xx answer sets the span's status to Error and error.type; a 4xx leaves
// the status unset. The status code is the one the client receives, also
// when the handler returns an error.
func TestHTTPServerSpans_StatusIsErrorFor5xxAndUnsetFor4xx(t *testing.T) {
	cases := []struct {
		name       string
		handler    fiber.Handler
		wantStatus int
		wantError  bool
	}{
		{"a 503 the handler sends", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusServiceUnavailable) }, 503, true},
		{"a 502 error the handler returns", func(c fiber.Ctx) error { return fiber.ErrBadGateway }, 502, true},
		{"a plain error the handler returns", func(c fiber.Ctx) error { return context.DeadlineExceeded }, 500, true},
		{"a 404 the handler sends", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNotFound) }, 404, false},
		{"a 401 error the handler returns", func(c fiber.Ctx) error { return fiber.ErrUnauthorized }, 401, false},
		{"a 201 the handler sends", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusCreated) }, 201, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, recorder := spanTestApp(t)
			app.Post("/things", tc.handler)

			status := spanRequest(t, app, fiber.MethodPost, "/things", nil)
			assert.Equal(t, tc.wantStatus, status, "the status the client receives")

			span := onlySpan(t, recorder)
			assert.Equal(t, "POST /things", span.Name())
			attrs := attributesOf(span)
			assert.Equal(t, int64(tc.wantStatus), attrs["http.response.status_code"].AsInt64())
			errorType, hasErrorType := attrs["error.type"]
			if tc.wantError {
				assert.Equal(t, codes.Error, span.Status().Code)
				assert.True(t, hasErrorType, "a 5xx sets error.type")
				assert.Equal(t, attrs["http.response.status_code"].Emit(), errorType.AsString())
			} else {
				assert.Equal(t, codes.Unset, span.Status().Code)
				assert.False(t, hasErrorType, "error.type is set below 500")
			}
		})
	}
}

// A request no route matched has no http.route and is named by its method.
func TestHTTPServerSpans_UnmatchedRequestIsNamedByItsMethod(t *testing.T) {
	app, recorder := spanTestApp(t)
	app.Get("/items/:id", func(c fiber.Ctx) error { return c.SendString("ok") })

	status := spanRequest(t, app, fiber.MethodGet, "/no-such-route", nil)
	require.Equal(t, fiber.StatusNotFound, status)

	span := onlySpan(t, recorder)
	assert.Equal(t, "GET", span.Name())
	attrs := attributesOf(span)
	_, hasRoute := attrs["http.route"]
	assert.False(t, hasRoute, "an unmatched request carries http.route")
	assert.Equal(t, int64(404), attrs["http.response.status_code"].AsInt64())
	assert.Equal(t, codes.Unset, span.Status().Code)
}

// A request a group middleware answers before its route is reached, as an
// authentication refusal is, has no http.route and is named by its method.
func TestHTTPServerSpans_RequestAnsweredBeforeItsRouteHasNoRoute(t *testing.T) {
	app, recorder := spanTestApp(t)
	group := app.Group("/guarded")
	group.Use(func(c fiber.Ctx) error { return fiber.ErrUnauthorized })
	group.Get("/:id", func(c fiber.Ctx) error { return c.SendString("ok") })

	require.Equal(t, fiber.StatusUnauthorized, spanRequest(t, app, fiber.MethodGet, "/guarded/7", nil))

	span := onlySpan(t, recorder)
	assert.Equal(t, "GET", span.Name())
	attrs := attributesOf(span)
	_, hasRoute := attrs["http.route"]
	assert.False(t, hasRoute, "a request answered before its route carries http.route")
	assert.Equal(t, int64(401), attrs["http.response.status_code"].AsInt64())
	assert.Equal(t, "/guarded/7", attrs["url.path"].AsString())
}

// A span the handler starts on the request's context is a child of the
// request's server span.
func TestHTTPServerSpans_ParentsTheSpansAHandlerStarts(t *testing.T) {
	app, recorder := spanTestApp(t)
	var tp oteltrace.TracerProvider
	app.Use(func(c fiber.Ctx) error {
		tp = oteltrace.SpanFromContext(c.Context()).TracerProvider()
		return c.Next()
	})
	app.Get("/work", func(c fiber.Ctx) error {
		_, child := tp.Tracer("test").Start(c.Context(), "handler.work")
		child.End()
		return c.SendString("done")
	})

	require.Equal(t, fiber.StatusOK, spanRequest(t, app, fiber.MethodGet, "/work", nil))

	ended := recorder.Ended()
	require.Len(t, ended, 2)
	byName := map[string]sdktrace.ReadOnlySpan{}
	for _, s := range ended {
		byName[s.Name()] = s
	}
	server, child := byName["GET /work"], byName["handler.work"]
	require.NotNil(t, server)
	require.NotNil(t, child)
	assert.Equal(t, server.SpanContext().TraceID(), child.SpanContext().TraceID())
	assert.Equal(t, server.SpanContext().SpanID(), child.Parent().SpanID())
}

// An incoming traceparent is not extracted: the request's span starts a
// trace of its own.
func TestHTTPServerSpans_ExtractsNoIncomingTraceparent(t *testing.T) {
	app, recorder := spanTestApp(t)
	app.Get("/items", func(c fiber.Ctx) error { return c.SendString("ok") })

	const incomingTrace = "4bf92f3577b34da6a3ce929d0e0e4736"
	status := spanRequest(t, app, fiber.MethodGet, "/items", map[string]string{
		"traceparent": "00-" + incomingTrace + "-00f067aa0ba902b7-01",
	})
	require.Equal(t, fiber.StatusOK, status)

	span := onlySpan(t, recorder)
	assert.False(t, span.Parent().IsValid(), "the request span has a parent")
	assert.NotEqual(t, incomingTrace, span.SpanContext().TraceID().String())
}

// A request whose handler panics still ends its span, as a 500 with status
// Error, and the recovery middleware answers it.
func TestHTTPServerSpans_EndsTheSpanOfAPanickingRequest(t *testing.T) {
	app, recorder := spanTestApp(t, RecoveryMiddleware())
	app.Get("/boom", func(c fiber.Ctx) error { panic("handler failure") })

	require.Equal(t, fiber.StatusInternalServerError, spanRequest(t, app, fiber.MethodGet, "/boom", nil))

	span := onlySpan(t, recorder)
	assert.Equal(t, codes.Error, span.Status().Code)
	assert.Equal(t, int64(500), attributesOf(span)["http.response.status_code"].AsInt64())
}

// A method the conventions do not name is _OTHER, with the method kept in
// http.request.method_original, and HTTP in the span name.
func TestSpanMethod_UnknownMethodIsOther(t *testing.T) {
	name, attrs := spanMethod("PROPFIND")
	assert.Equal(t, "HTTP", name)
	got := map[attribute.Key]string{}
	for _, kv := range attrs {
		got[kv.Key] = kv.Value.AsString()
	}
	assert.Equal(t, map[attribute.Key]string{
		"http.request.method":          "_OTHER",
		"http.request.method_original": "PROPFIND",
	}, got)

	name, attrs = spanMethod("PATCH")
	assert.Equal(t, "PATCH", name)
	require.Len(t, attrs, 1)
	assert.Equal(t, "PATCH", attrs[0].Value.AsString())
}
