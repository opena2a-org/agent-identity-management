package main

import "github.com/gofiber/fiber/v3"

// Onboarding telemetry routes, relative to /api/v1.
const (
	onboardingEventsPath  = "/onboarding/events"
	onboardingMetricsPath = "/platform-admin/onboarding-metrics"
)

// onboardingTelemetryRouteDeps is the middleware and handlers the onboarding
// telemetry routes mount. Production passes the real ones; a test passes
// stand-ins and still gets the real paths and the real middleware order.
type onboardingTelemetryRouteDeps struct {
	Authenticate         fiber.Handler // dashboard session (JWT)
	RequirePlatformAdmin fiber.Handler // org admin on the platform admin allowlist
	EventLimit           fiber.Handler
	MetricsLimit         fiber.Handler

	RecordEvent fiber.Handler
	Metrics     fiber.Handler
}

// registerOnboardingTelemetryRoutes mounts the event and metrics routes with
// middleware attached per route, so neither gate leaks onto a sibling path.
// Any signed-in role may report an event for its own organization; the
// metrics read every organization and sit behind the platform admin gate.
func registerOnboardingTelemetryRoutes(r fiber.Router, d onboardingTelemetryRouteDeps) {
	r.Post(onboardingEventsPath, d.Authenticate, d.EventLimit, d.RecordEvent)
	r.Get(onboardingMetricsPath, d.Authenticate, d.RequirePlatformAdmin, d.MetricsLimit, d.Metrics)
}
