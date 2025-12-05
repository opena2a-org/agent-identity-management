package middleware

import (
	"os"

	"github.com/gofiber/fiber/v3"
)

// SecurityHeadersMiddleware adds essential HTTP security headers to all responses
// These headers protect against common web vulnerabilities:
// - XSS attacks
// - Clickjacking
// - MIME type sniffing
// - Protocol downgrade attacks
//
// SECURITY: Enterprise-grade security headers for SOC 2 and HIPAA compliance
func SecurityHeadersMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		// X-Content-Type-Options: Prevents MIME type sniffing
		// SECURITY: Stops browsers from interpreting files as different MIME types
		c.Set("X-Content-Type-Options", "nosniff")

		// X-Frame-Options: Prevents clickjacking attacks
		// SECURITY: Blocks embedding in iframes (DENY = no embedding allowed)
		c.Set("X-Frame-Options", "DENY")

		// X-XSS-Protection: Legacy XSS filter (deprecated but still useful for older browsers)
		// SECURITY: Enables browser's built-in XSS filter
		c.Set("X-XSS-Protection", "1; mode=block")

		// Referrer-Policy: Controls referrer information sent with requests
		// SECURITY: Only send referrer to same-origin requests
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Permissions-Policy: Restricts browser features
		// SECURITY: Disable potentially dangerous browser features
		c.Set("Permissions-Policy", "accelerometer=(), camera=(), geolocation=(), gyroscope=(), magnetometer=(), microphone=(), payment=(), usb=()")

		// Cache-Control: Prevent caching of sensitive API responses
		// SECURITY: Sensitive data should never be cached
		c.Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate")
		c.Set("Pragma", "no-cache")
		c.Set("Expires", "0")

		// HSTS: HTTP Strict Transport Security (only in production)
		// SECURITY: Forces HTTPS connections, prevents protocol downgrade attacks
		if os.Getenv("ENVIRONMENT") == "production" {
			// max-age=31536000 = 1 year
			// includeSubDomains = applies to all subdomains
			// preload = allows inclusion in browser HSTS preload lists
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		}

		// Content-Security-Policy: Controls resource loading
		// SECURITY: Prevents XSS by restricting script sources
		// Note: This is a restrictive policy for API endpoints - frontend may need different policy
		c.Set("Content-Security-Policy", "default-src 'self'; frame-ancestors 'none'; form-action 'self'")

		return c.Next()
	}
}

// APISecurityHeadersMiddleware adds security headers specifically for API responses
// More permissive than full CSP, suitable for JSON API endpoints
func APISecurityHeadersMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		// Core security headers
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Prevent caching of API responses
		c.Set("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Set("Pragma", "no-cache")

		// HSTS in production
		if os.Getenv("ENVIRONMENT") == "production" {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		return c.Next()
	}
}
