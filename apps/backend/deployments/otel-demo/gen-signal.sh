#!/usr/bin/env bash
# Build and run a throwaway Go program that sends one trace + one metric +
# one log to the local OTel collector via OTLP gRPC. Prints the trace_id
# on stdout so smoke-test.sh can verify it lands in Tempo.
#
# Arg 1: OTLP gRPC port (default 4317)
set -euo pipefail

OTEL_PORT="${1:-4317}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Stage a tiny module so the helper does not pollute the main go.mod.
cat > "$TMP/go.mod" <<EOF
module otelgen

go 1.21
EOF

cat > "$TMP/main.go" <<'GOEOF'
package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	otellog "go.opentelemetry.io/otel/log"
	logglobal "go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	apitrace "go.opentelemetry.io/otel/trace"
)

func main() {
	endpoint := os.Getenv("OTEL_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:4317"
	}
	ctx := context.Background()

	res, _ := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName("aim-smoke-test"),
		semconv.ServiceVersion("smoke"),
	))

	// Trace
	texp, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil { fmt.Fprintln(os.Stderr, "trace:", err); os.Exit(1) }
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(texp, sdktrace.WithBatchTimeout(500*time.Millisecond)),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(ctx)

	// Metric — Cumulative temporality required by Prometheus OTLP receiver.
	mexp, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint),
		otlpmetricgrpc.WithInsecure(),
		otlpmetricgrpc.WithTemporalitySelector(func(sdkmetric.InstrumentKind) metricdata.Temporality {
			return metricdata.CumulativeTemporality
		}),
	)
	if err != nil { fmt.Fprintln(os.Stderr, "metric:", err); os.Exit(1) }
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(mexp, sdkmetric.WithInterval(500*time.Millisecond))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)
	defer mp.Shutdown(ctx)

	// Log
	lexp, err := otlploggrpc.New(ctx, otlploggrpc.WithEndpoint(endpoint), otlploggrpc.WithInsecure())
	if err != nil { fmt.Fprintln(os.Stderr, "log:", err); os.Exit(1) }
	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(lexp, sdklog.WithExportInterval(500*time.Millisecond))),
		sdklog.WithResource(res),
	)
	logglobal.SetLoggerProvider(lp)
	defer lp.Shutdown(ctx)

	tracer := otel.Tracer("aim/smoke")
	parent, span := tracer.Start(ctx, "fga.authorize",
		apitrace.WithAttributes(
			attribute.String("agent.id", "00000000-0000-0000-0000-deadbeefcafe"),
			attribute.String("agent.capability", "smoke:test"),
			attribute.String("fga.outcome", "ALLOW"),
		),
	)
	traceID := span.SpanContext().TraceID()
	for _, step := range []string{"capability_check","attribute_check","context_check","chain_check","intent_check_sync"} {
		_, child := tracer.Start(parent, "fga."+step, apitrace.WithAttributes(
			attribute.String("fga.step", step),
			attribute.Bool("fga.allowed", true),
		))
		child.End()
	}
	span.End()

	meter := otel.Meter("aim/smoke")
	c, _ := meter.Int64Counter("fga.decisions")
	c.Add(parent, 1, metric.WithAttributes(attribute.String("fga.outcome","ALLOW")))
	g, _ := meter.Float64Gauge("agent.drift_score")
	g.Record(parent, 0.0, metric.WithAttributes(attribute.String("agent.id","00000000-0000-0000-0000-deadbeefcafe")))

	logger := logglobal.GetLoggerProvider().Logger("aim/smoke")
	var rec otellog.Record
	rec.SetTimestamp(time.Now())
	rec.SetSeverity(otellog.SeverityInfo)
	rec.SetBody(otellog.StringValue("fga.decision"))
	rec.AddAttributes(
		otellog.String("agent.id","00000000-0000-0000-0000-deadbeefcafe"),
		otellog.String("fga.outcome","ALLOW"),
	)
	logger.Emit(parent, rec)

	// Force flush so the smoke test does not race the batcher.
	if err := tp.ForceFlush(ctx); err != nil { fmt.Fprintln(os.Stderr, "trace flush:", err) }
	if err := mp.ForceFlush(ctx); err != nil { fmt.Fprintln(os.Stderr, "metric flush:", err) }
	if err := lp.ForceFlush(ctx); err != nil { fmt.Fprintln(os.Stderr, "log flush:", err) }

	fmt.Println(hex.EncodeToString(traceID[:]))
}
GOEOF

cd "$TMP"
# Resolve deps in the throwaway module. Output goes to stderr so it does
# not corrupt the trace_id we print on stdout.
go mod tidy >&2 2>&1
# Surface go errors to caller's stderr; only the trace_id goes to stdout.
OTEL_ENDPOINT="localhost:${OTEL_PORT}" go run .
