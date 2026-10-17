// Package telemetry wires OpenTelemetry trace, metric, and log providers
// to an OTLP collector. This is the reference implementation for the
// AIM SemConv proposal pitched at the May 22 Observability Summit.
//
// Attribute names emitted by this package and its callers MUST match the
// proposal in briefs/may18-talk-revision/talk2-slides.md (Slide 14).
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// Defaults.
const (
	defaultEndpoint    = "localhost:4317"
	defaultServiceName = "aim-backend"
	dialTimeout        = 5 * time.Second
)

// ErrDisabled is returned by Init when Config.Endpoint is empty and
// OTEL_EXPORTER_OTLP_ENDPOINT is set to the empty string. Init then installs
// no provider and dials nothing, so there is no exporter to flush at shutdown.
var ErrDisabled = errors.New("telemetry: disabled, OTEL_EXPORTER_OTLP_ENDPOINT is set and empty")

// Config controls provider wiring.
type Config struct {
	// ServiceName populates the `service.name` resource attribute.
	ServiceName string
	// ServiceVersion populates the `service.version` resource attribute.
	ServiceVersion string
	// Endpoint is the OTLP gRPC endpoint for every signal, a URL
	// (https://host:port or http://host:port) or host:port. Empty = the
	// value of OTEL_EXPORTER_OTLP_<SIGNAL>_ENDPOINT or
	// OTEL_EXPORTER_OTLP_ENDPOINT, or "localhost:4317" when neither is set.
	Endpoint string
	// Insecure makes an endpoint given without a scheme plaintext, as
	// OTEL_EXPORTER_OTLP_INSECURE=true does. A scheme wins over it.
	Insecure bool
}

// Init wires trace + metric + log providers to an OTLP gRPC collector.
//
// Returns a shutdown func that flushes and closes all exporters. Always
// defer it in main; it is safe to call once. If the collector is
// unreachable at boot, providers are still installed and exporters retry
// in the background — the server does not crash.
//
// Reads OTEL_EXPORTER_OTLP_ENDPOINT and OTEL_SERVICE_NAME if Config fields
// are empty. An OTEL_EXPORTER_OTLP_ENDPOINT set to the empty string turns
// telemetry off: Init returns ErrDisabled and a nil shutdown func. Each
// signal's endpoint and TLS setting are resolved as resolveTransport
// describes; a certificate variable naming a file that cannot be read fails
// Init with an error naming the variable, and installs no provider.
func Init(ctx context.Context, cfg Config) (shutdown func(context.Context) error, err error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = os.Getenv("OTEL_SERVICE_NAME")
		if cfg.ServiceName == "" {
			cfg.ServiceName = defaultServiceName
		}
	}
	if cfg.Endpoint == "" {
		if endpoint, set := os.LookupEnv("OTEL_EXPORTER_OTLP_ENDPOINT"); set && endpoint == "" {
			return nil, ErrDisabled
		}
	}
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = "1.0.0"
	}

	transports := map[string]transport{}
	for _, signal := range []string{signalTraces, signalMetrics, signalLogs} {
		t, err := resolveTransport(os.LookupEnv, signal, cfg.Endpoint, cfg.Insecure)
		if err != nil {
			return nil, err
		}
		transports[signal] = t
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("telemetry: build resource: %w", err)
	}

	otel.SetTextMapPropagator(Propagator())

	tp, traceShutdown, err := initTraceProvider(ctx, transports[signalTraces], res)
	if err != nil {
		return nil, err
	}
	otel.SetTracerProvider(tp)

	mp, metricShutdown, err := initMeterProvider(ctx, transports[signalMetrics], res)
	if err != nil {
		// Trace already up — tear it down before bailing.
		_ = traceShutdown(context.Background())
		return nil, err
	}
	otel.SetMeterProvider(mp)

	lp, logShutdown, err := initLoggerProvider(ctx, transports[signalLogs], res)
	if err != nil {
		_ = metricShutdown(context.Background())
		_ = traceShutdown(context.Background())
		return nil, err
	}
	global.SetLoggerProvider(lp)

	shutdown = func(ctx context.Context) error {
		var errs []error
		if e := logShutdown(ctx); e != nil {
			errs = append(errs, fmt.Errorf("log shutdown: %w", e))
		}
		if e := metricShutdown(ctx); e != nil {
			errs = append(errs, fmt.Errorf("metric shutdown: %w", e))
		}
		if e := traceShutdown(ctx); e != nil {
			errs = append(errs, fmt.Errorf("trace shutdown: %w", e))
		}
		return errors.Join(errs...)
	}
	return shutdown, nil
}

// Propagator is the text map propagator Init registers as the global one.
// No request handler applies it to an incoming request: the audit record
// trace middleware reads the one header field it keeps itself.
func Propagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func initTraceProvider(ctx context.Context, t transport, res *resource.Resource) (*sdktrace.TracerProvider, func(context.Context) error, error) {
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	exp, err := otlptracegrpc.New(dialCtx,
		otlptracegrpc.WithEndpoint(t.endpoint),
		otlptracegrpc.WithTLSCredentials(t.credentials()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("telemetry: trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	return tp, tp.Shutdown, nil
}

func initMeterProvider(ctx context.Context, t transport, res *resource.Resource) (*sdkmetric.MeterProvider, func(context.Context) error, error) {
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	opts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(t.endpoint),
		otlpmetricgrpc.WithTLSCredentials(t.credentials()),
		// Cumulative is mandatory for the Prometheus OTLP write receiver.
		// Without this, monotonic counters are rejected with "invalid
		// temporality and type combination" because the SDK default for
		// synchronous counters in v1.43 is Delta.
		otlpmetricgrpc.WithTemporalitySelector(func(sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.CumulativeTemporality
		}),
	}
	exp, err := otlpmetricgrpc.New(dialCtx, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("telemetry: metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exp)),
		sdkmetric.WithResource(res),
	)
	return mp, mp.Shutdown, nil
}

func initLoggerProvider(ctx context.Context, t transport, res *resource.Resource) (*sdklog.LoggerProvider, func(context.Context) error, error) {
	dialCtx, cancel := context.WithTimeout(ctx, dialTimeout)
	defer cancel()

	exp, err := otlploggrpc.New(dialCtx,
		// The URL form also sets the exporter's own insecure setting from the
		// scheme, so it does not parse OTEL_EXPORTER_OTLP_ENDPOINT itself.
		otlploggrpc.WithEndpointURL(t.url()),
		otlploggrpc.WithTLSCredentials(t.credentials()),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("telemetry: log exporter: %w", err)
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exp)),
		sdklog.WithResource(res),
	)
	return lp, lp.Shutdown, nil
}

// Logger returns a slog.Logger that bridges through the OTel logger
// provider. Records emitted via this logger inside an active span
// automatically carry trace.id and span.id, which is what makes the
// Loki <-> Tempo jump work in Grafana.
func Logger(name string) *slog.Logger {
	return otelslog.NewLogger(name)
}

// LoggerProvider returns the active OTel logger provider, useful for
// callers that need to construct otellog.Records directly.
func LoggerProvider() otellog.LoggerProvider { return global.GetLoggerProvider() }
