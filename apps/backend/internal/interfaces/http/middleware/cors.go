package middleware

import (
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
)

// vercelPreviewOriginRe matches AIM's own Vercel preview deployments only
// (the aim-cloud / aim-frontend projects under the opena2a team). Used by an
// opt-in dynamic CORS matcher so per-branch preview URLs can be verified
// without a wildcard. Scoped to our project prefixes to avoid allowing
// arbitrary *.vercel.app origins.
var vercelPreviewOriginRe = regexp.MustCompile(`^https://aim-(cloud|frontend)-[a-z0-9-]+-opena2a\.vercel\.app$`)

// vercelPreviewOriginAllowed reports whether origin is an AIM Vercel preview.
func vercelPreviewOriginAllowed(origin string) bool {
	return vercelPreviewOriginRe.MatchString(origin)
}

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

	cfg := cors.Config{
		AllowOrigins:     safeOrigins,
		AllowMethods:     corsAllowMethods,
		AllowHeaders:     corsAllowHeaders,
		ExposeHeaders:    corsExposeHeaders,
		AllowCredentials: true,
		MaxAge:           3600,
	}

	// Opt-in: also allow AIM's own Vercel preview deployments (per-branch URLs
	// are dynamic, so they can't be a static allowlist entry). Off by default;
	// enable in non-prod with CORS_ALLOW_VERCEL_PREVIEWS=true. The matcher is
	// scoped to our project prefixes — never a blanket *.vercel.app.
	if strings.EqualFold(os.Getenv("CORS_ALLOW_VERCEL_PREVIEWS"), "true") {
		cfg.AllowOriginsFunc = vercelPreviewOriginAllowed
		log.Println("ℹ️  CORS: AIM Vercel preview origins allowed (CORS_ALLOW_VERCEL_PREVIEWS=true)")
	}

	return cors.New(cfg)
}
