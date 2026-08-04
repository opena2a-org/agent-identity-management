package metrics

import (
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// MetricsAuthMiddleware optionally gates the /metrics endpoint behind a bearer
// token.
//
// The Prometheus scrape exposes endpoint topology and operational counters
// (info-disclosure, CWE-200 — issue #348). When token is non-empty, a request
// must present `Authorization: Bearer <token>` with an exact, constant-time
// match; otherwise it is rejected with 401. Prometheus supports this natively
// via `authorization`/`bearer_token` in scrape_configs.
//
// When token is empty the endpoint stays open, preserving the prior behaviour
// for deployments that rely on network-layer isolation instead. main.go logs a
// startup warning in that case so the exposure is not silent.
//
// Constant-time comparison (crypto/subtle) avoids leaking the token length or
// prefix through response timing.
func MetricsAuthMiddleware(token string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if token == "" {
			return c.Next()
		}

		const prefix = "Bearer "
		authHeader := c.Get("Authorization")
		if !strings.HasPrefix(authHeader, prefix) {
			return metricsUnauthorized(c)
		}

		presented := strings.TrimPrefix(authHeader, prefix)
		if subtle.ConstantTimeCompare([]byte(presented), []byte(token)) != 1 {
			return metricsUnauthorized(c)
		}

		return c.Next()
	}
}

func metricsUnauthorized(c fiber.Ctx) error {
	// WWW-Authenticate advertises the scheme so a scraper (or operator) sees why
	// the request was refused rather than a bare 401.
	c.Set("WWW-Authenticate", `Bearer realm="metrics"`)
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": "Unauthorized: /metrics requires a bearer token",
	})
}
