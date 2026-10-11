package telemetry

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/log/global"
)

// An OTEL_EXPORTER_OTLP_ENDPOINT set to the empty string turns telemetry off:
// Init installs no provider, so nothing dials a collector and there is no
// exporter to flush at shutdown.
func TestInit_EmptyEndpointEnvInstallsNoProvider(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")

	tracerBefore := otel.GetTracerProvider()
	meterBefore := otel.GetMeterProvider()
	loggerBefore := global.GetLoggerProvider()

	shutdown, err := Init(context.Background(), Config{Insecure: true})
	if shutdown != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		_ = shutdown(ctx)
		cancel()
	}
	t.Cleanup(func() {
		if otel.GetTracerProvider() != tracerBefore {
			otel.SetTracerProvider(tracerBefore)
		}
		if otel.GetMeterProvider() != meterBefore {
			otel.SetMeterProvider(meterBefore)
		}
		if global.GetLoggerProvider() != loggerBefore {
			global.SetLoggerProvider(loggerBefore)
		}
	})

	if !errors.Is(err, ErrDisabled) {
		t.Errorf("Init error = %v; want ErrDisabled", err)
	}
	if shutdown != nil {
		t.Errorf("Init returned a shutdown func; want none when telemetry is disabled")
	}
	if otel.GetTracerProvider() != tracerBefore {
		t.Errorf("Init installed a tracer provider")
	}
	if otel.GetMeterProvider() != meterBefore {
		t.Errorf("Init installed a meter provider")
	}
	if global.GetLoggerProvider() != loggerBefore {
		t.Errorf("Init installed a logger provider")
	}
}

// Only the empty value disables telemetry: an unset variable keeps the
// localhost default, and an endpoint passed in Config wins over the variable.
func TestInit_EnabledUnlessEndpointEnvIsSetAndEmpty(t *testing.T) {
	cases := []struct {
		name     string
		unset    bool
		envValue string
		endpoint string
	}{
		{name: "variable unset", unset: true},
		{name: "variable set", envValue: "127.0.0.1:1"},
		{name: "Config.Endpoint with the variable empty", endpoint: "127.0.0.1:1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", tc.envValue)
			if tc.unset {
				os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")
			}
			tracerBefore := otel.GetTracerProvider()
			meterBefore := otel.GetMeterProvider()
			loggerBefore := global.GetLoggerProvider()
			t.Cleanup(func() {
				otel.SetTracerProvider(tracerBefore)
				otel.SetMeterProvider(meterBefore)
				global.SetLoggerProvider(loggerBefore)
			})

			shutdown, err := Init(context.Background(), Config{Endpoint: tc.endpoint, Insecure: true})
			if err != nil {
				t.Fatalf("Init error = %v; want nil", err)
			}
			if shutdown == nil {
				t.Fatalf("Init returned no shutdown func")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_ = shutdown(ctx) // nothing listens on the endpoint; only the call matters
		})
	}
}
