package middleware

import (
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/google/uuid"
)

// getClientIP extracts the real client IP, respecting trusted proxy headers
// SECURITY: Only trust X-Forwarded-For and X-Real-IP headers when behind a trusted proxy
// The TRUSTED_PROXIES env var should contain comma-separated trusted proxy IPs/CIDRs
func getClientIP(c fiber.Ctx) string {
	// Check if we have trusted proxies configured
	trustedProxies := os.Getenv("TRUSTED_PROXIES")

	// If no trusted proxies configured, use direct IP only (most secure default)
	if trustedProxies == "" {
		return c.IP()
	}

	// Parse trusted proxy list
	trusted := make(map[string]bool)
	for _, proxy := range strings.Split(trustedProxies, ",") {
		trusted[strings.TrimSpace(proxy)] = true
	}

	// Get the direct connection IP
	directIP := c.IP()

	// SECURITY: Wildcard "*" is not allowed — it would let any client spoof their IP.
	// Only trust headers if the direct connection IP is in the trusted proxy list.
	if trusted[directIP] {
		// Try X-Real-IP first (set by nginx)
		if realIP := c.Get("X-Real-IP"); realIP != "" {
			return realIP
		}

		// Try X-Forwarded-For (first IP in the chain is the original client)
		if forwardedFor := c.Get("X-Forwarded-For"); forwardedFor != "" {
			ips := strings.Split(forwardedFor, ",")
			if len(ips) > 0 {
				return strings.TrimSpace(ips[0])
			}
		}
	}

	// Fallback to direct IP
	return directIP
}

// RateLimitMiddleware implements rate limiting
// SECURITY: Uses authenticated user ID when available, falls back to real client IP
func RateLimitMiddleware() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        100,             // 100 requests
		Expiration: 1 * time.Minute, // per minute
		KeyGenerator: func(c fiber.Ctx) string {
			// Rate limit by user if authenticated, otherwise by IP
			if userID := c.Locals("user_id"); userID != nil {
				if id, ok := userID.(uuid.UUID); ok {
					return "user:" + id.String()
				}
			}
			// Use real client IP (respecting trusted proxy headers)
			return "ip:" + getClientIP(c)
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Please try again later.",
			})
		},
	})
}

// StrictRateLimitMiddleware implements stricter rate limiting for sensitive endpoints
// SECURITY: Used for login, registration, password reset, and other sensitive operations
func StrictRateLimitMiddleware() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        10,              // 10 requests
		Expiration: 1 * time.Minute, // per minute
		KeyGenerator: func(c fiber.Ctx) string {
			if userID := c.Locals("user_id"); userID != nil {
				if id, ok := userID.(uuid.UUID); ok {
					return "user:" + id.String()
				}
			}
			// Use real client IP (respecting trusted proxy headers)
			return "ip:" + getClientIP(c)
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "Rate limit exceeded. Please try again later.",
			})
		},
	})
}
