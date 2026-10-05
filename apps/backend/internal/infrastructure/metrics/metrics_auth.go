package metrics

import (
	"crypto/sha256"
	"crypto/subtle"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// MetricsAuthMiddleware gates a /metrics route behind a bearer token.
//
// The Prometheus scrape exposes endpoint topology and operational counters
// (info-disclosure, CWE-200 — issue #348). A request must present
// `Authorization: Bearer <token>` matching token exactly; otherwise it is
// rejected with 401. Prometheus supports this natively via `authorization`
// with `credentials_file` in scrape_configs.
//
// An empty token refuses every request. A surface meant to be open (the
// dedicated metrics listener on a loopback address, see cmd/server) mounts the
// handler without this middleware; it never passes an empty token to open it.
//
// The comparison runs crypto/subtle.ConstantTimeCompare over the SHA-256
// digests of the presented and the configured token. ConstantTimeCompare
// returns at once when its inputs differ in length, so comparing the raw
// values would let response timing tell a presented value of the right length
// from one of the wrong length. Both digests are 32 bytes whatever the inputs,
// so the compare does the same work for every presented value and response
// timing reveals neither the configured token's length nor its content.
func MetricsAuthMiddleware(token string) fiber.Handler {
	want := sha256.Sum256([]byte(token))
	return func(c fiber.Ctx) error {
		if token == "" {
			return metricsUnauthorized(c)
		}

		const prefix = "Bearer "
		authHeader := c.Get("Authorization")
		if !strings.HasPrefix(authHeader, prefix) {
			return metricsUnauthorized(c)
		}

		got := sha256.Sum256([]byte(strings.TrimPrefix(authHeader, prefix)))
		if subtle.ConstantTimeCompare(got[:], want[:]) != 1 {
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
