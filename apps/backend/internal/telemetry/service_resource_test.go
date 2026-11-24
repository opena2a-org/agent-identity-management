package telemetry

import (
	"context"
	"testing"

	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// The resource carries service.version as the caller gives it, and leaves it
// out when the caller gives none: Init does not invent a version.
func TestServiceResource_ServiceVersion(t *testing.T) {
	t.Run("a given version is carried as given", func(t *testing.T) {
		res, err := serviceResource(context.Background(), Config{ServiceName: "aim-backend", ServiceVersion: "1.1.0"})
		if err != nil {
			t.Fatal(err)
		}
		v, ok := res.Set().Value(semconv.ServiceVersionKey)
		if !ok || v.AsString() != "1.1.0" {
			t.Fatalf("service.version = %q (present %v), want 1.1.0", v.AsString(), ok)
		}
		if n, _ := res.Set().Value(semconv.ServiceNameKey); n.AsString() != "aim-backend" {
			t.Fatalf("service.name = %q, want aim-backend", n.AsString())
		}
	})

	t.Run("an empty version leaves the attribute out", func(t *testing.T) {
		res, err := serviceResource(context.Background(), Config{ServiceName: "aim-backend"})
		if err != nil {
			t.Fatal(err)
		}
		if v, ok := res.Set().Value(semconv.ServiceVersionKey); ok {
			t.Fatalf("service.version = %q, want it absent", v.AsString())
		}
	})
}
