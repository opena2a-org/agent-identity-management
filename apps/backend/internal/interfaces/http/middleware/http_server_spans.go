package middleware

import (
	"net"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	oteltrace "go.opentelemetry.io/otel/trace"
)

// httpServerSpansScope is the instrumentation scope of the request spans.
const httpServerSpansScope = "aim/http"

// knownHTTPMethods are the methods the HTTP semantic conventions name; any
// other is recorded as _OTHER.
var knownHTTPMethods = map[string]bool{
	fiber.MethodConnect: true, fiber.MethodDelete: true, fiber.MethodGet: true,
	fiber.MethodHead: true, fiber.MethodOptions: true, fiber.MethodPatch: true,
	fiber.MethodPost: true, fiber.MethodPut: true, fiber.MethodTrace: true,
}

// HTTPServerSpans starts a SERVER span for each request, as the OpenTelemetry
// HTTP semantic conventions define it
// (https://opentelemetry.io/docs/specs/semconv/http/http-spans/), on the
// tracer provider installed when the request arrives. The span is named
// "{method} {route}" from the route Fiber matched, read after the handler
// ran, and carries http.request.method, url.path, url.scheme, http.route,
// http.response.status_code, server.address, server.port,
// network.protocol.version, client.address and user_agent.original. A
// request no route matched, or one a middleware answered before its route,
// has no http.route and is named by its method alone. A 5xx answer sets the
// span's status to Error and error.type to the status code; a 4xx leaves it
// unset.
//
// It records no query string, no header other than User-Agent and no body.
// It sets the request's context to one that carries the span, so a span a
// handler starts on that context is its child. It does not extract an
// incoming traceparent: every request span starts a trace of its own.
//
// An error the chain returns is answered by the app's error handler here,
// as the Prometheus middleware does, so the recorded status is the one the
// client receives.
func HTTPServerSpans() fiber.Handler {
	return httpServerSpans(otel.GetTracerProvider)
}

func httpServerSpans(provider func() oteltrace.TracerProvider) fiber.Handler {
	return func(c fiber.Ctx) error {
		method, methodAttrs := spanMethod(c.Method())
		ctx, span := provider().Tracer(httpServerSpansScope).Start(c.Context(), method,
			oteltrace.WithSpanKind(oteltrace.SpanKindServer),
			oteltrace.WithAttributes(append(methodAttrs, requestAttributes(c)...)...),
		)
		c.SetContext(ctx)

		ended := false
		defer func() {
			if !ended {
				// The chain panicked; the recovery middleware answers 500.
				endServerSpan(span, method, "", fiber.StatusInternalServerError)
			}
		}()

		if err := c.Next(); err != nil {
			if handlerErr := c.App().ErrorHandler(c, err); handlerErr != nil {
				_ = c.SendStatus(fiber.StatusInternalServerError)
			}
		}
		route := ""
		if c.Matched() {
			route = strings.Clone(c.Route().Path)
		}
		ended = true
		endServerSpan(span, method, route, c.Response().StatusCode())
		return nil
	}
}

// spanMethod returns the method the span name carries and the method
// attributes: a method the conventions do not name is HTTP in the name,
// _OTHER in http.request.method and itself in http.request.method_original.
func spanMethod(method string) (string, []attribute.KeyValue) {
	method = strings.Clone(method)
	if knownHTTPMethods[method] {
		return method, []attribute.KeyValue{semconv.HTTPRequestMethodKey.String(method)}
	}
	return "HTTP", []attribute.KeyValue{semconv.HTTPRequestMethodOther, semconv.HTTPRequestMethodOriginal(method)}
}

// requestAttributes reads the request attributes known before the handler
// runs. Fiber's strings point into buffers it reuses after the request, so
// each is copied before the span outlives the request.
func requestAttributes(c fiber.Ctx) []attribute.KeyValue {
	scheme := strings.Clone(c.Scheme())
	attrs := []attribute.KeyValue{
		semconv.URLPath(strings.Clone(c.Path())),
		semconv.URLScheme(scheme),
	}
	if host := c.Host(); host != "" {
		address, port := strings.Trim(host, "[]"), ""
		if h, p, err := net.SplitHostPort(host); err == nil {
			address, port = h, p
		}
		attrs = append(attrs, semconv.ServerAddress(strings.Clone(address)))
		if n, err := strconv.Atoi(port); err == nil {
			attrs = append(attrs, semconv.ServerPort(n))
		} else if scheme == "https" {
			attrs = append(attrs, semconv.ServerPort(443))
		} else {
			attrs = append(attrs, semconv.ServerPort(80))
		}
	}
	if version, ok := strings.CutPrefix(c.Protocol(), "HTTP/"); ok {
		attrs = append(attrs, semconv.NetworkProtocolVersion(strings.Clone(version)))
	}
	if ip := c.IP(); ip != "" {
		attrs = append(attrs, semconv.ClientAddress(strings.Clone(ip)))
	}
	if ua := c.Get(fiber.HeaderUserAgent); ua != "" {
		attrs = append(attrs, semconv.UserAgentOriginal(strings.Clone(ua)))
	}
	return attrs
}

// endServerSpan names the span from its route, records the answer's status
// code and ends it.
func endServerSpan(span oteltrace.Span, method, route string, status int) {
	if route != "" {
		span.SetName(method + " " + route)
		span.SetAttributes(semconv.HTTPRoute(route))
	}
	span.SetAttributes(semconv.HTTPResponseStatusCode(status))
	if status >= fiber.StatusInternalServerError {
		code := strconv.Itoa(status)
		span.SetAttributes(semconv.ErrorTypeKey.String(code))
		span.SetStatus(codes.Error, code)
	}
	span.End()
}
