package middleware

import (
	"github.com/gofiber/fiber/v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// AuthMiddlewareUnlessAgentSigned passes on a request that PQCAgentMiddleware
// authenticated, whichever algorithm the agent signed it with (Ed25519, ML-DSA, or both
// for an agent in hybrid mode), and answers any other request with AuthMiddleware.
// AuthMiddleware itself passes on a request authenticated with any of these algorithms;
// this guard names that pass at the route that mounts it.
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
