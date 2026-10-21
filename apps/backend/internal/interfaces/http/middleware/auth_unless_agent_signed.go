package middleware

import (
	"github.com/gofiber/fiber/v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// AuthMiddlewareUnlessAgentSigned passes on a request that PQCAgentMiddleware
// authenticated, whichever algorithm the agent signed it with (Ed25519, ML-DSA, or both
// for an agent in hybrid mode), and answers any other request with AuthMiddleware.
// AuthMiddleware passes on an agent-signed request only when it was signed with Ed25519,
// so mounted after PQCAgentMiddleware by itself it refuses an agent that signs with
// ML-DSA or with both keys.
func AuthMiddlewareUnlessAgentSigned(jwtService *auth.JWTService) fiber.Handler {
	authenticate := AuthMiddleware(jwtService)
	return func(c fiber.Ctx) error {
		switch c.Locals("authenticated_via") {
		case "ed25519", "mldsa", "hybrid":
			return c.Next()
		}
		return authenticate(c)
	}
}
