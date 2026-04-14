package middleware

import (
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

var (
	corsAllowMethods  = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	corsAllowHeaders  = []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Agent-ID", "X-Signature", "X-Timestamp", "X-Public-Key", "X-API-Key"}
	corsExposeHeaders = []string{"Content-Disposition", "Content-Length"}
)

// ValidateAndSanitizeCORSOrigins validates CORS origins and rejects dangerous configurations
// SECURITY: Prevents wildcard "*" and validates origin format
func ValidateAndSanitizeCORSOrigins(origins []string) []string {
	var sanitized []string
	for _, origin := range origins {
		origin = strings.TrimSpace(origin)

		// SECURITY: Reject wildcard origins - this is a critical security risk
		if origin == "*" {
			log.Println("⚠️  SECURITY WARNING: Wildcard CORS origin '*' rejected. Use specific origins instead.")
			continue
		}

		// SECURITY: Reject empty origins
		if origin == "" {
			continue
		}

		// SECURITY: Validate origin format (must start with http:// or https://)
		if !strings.HasPrefix(origin, "http://") && !strings.HasPrefix(origin, "https://") {
			log.Printf("⚠️  SECURITY WARNING: Invalid CORS origin format rejected: %s", origin)
			continue
		}

		// SECURITY: Reject origins with wildcards in subdomain (*.example.com)
		if strings.Contains(origin, "*") {
			log.Printf("⚠️  SECURITY WARNING: Wildcard subdomain CORS origin rejected: %s", origin)
			continue
		}

		sanitized = append(sanitized, origin)
	}

	// SECURITY: If no valid origins remain, default to localhost only
	if len(sanitized) == 0 {
		log.Println("⚠️  SECURITY WARNING: No valid CORS origins configured. Defaulting to localhost:3000 only.")
		return []string{"http://localhost:3000"}
	}

	return sanitized
}

// CORSMiddleware configures CORS for the application
// SECURITY: Validates and sanitizes origins to prevent CORS misconfiguration attacks
func CORSMiddleware(allowedOrigins []string) fiber.Handler {
	// Validate and sanitize origins before applying
	safeOrigins := ValidateAndSanitizeCORSOrigins(allowedOrigins)

	return cors.New(cors.Config{
		AllowOrigins:     safeOrigins,
		AllowMethods:     corsAllowMethods,
		AllowHeaders:     corsAllowHeaders,
		ExposeHeaders:    corsExposeHeaders,
		AllowCredentials: true,
		MaxAge:           3600,
	})
}
