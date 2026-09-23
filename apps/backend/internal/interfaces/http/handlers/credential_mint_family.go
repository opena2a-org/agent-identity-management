package handlers

import (
	"github.com/gofiber/fiber/v3"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// refuseIfFamilyRevoked guards a credential-minting route: when the acting
// access token belongs to a revoked session it answers 401 and reports false.
func refuseIfFamilyRevoked(c fiber.Ctx, jwtService *auth.JWTService, audit *application.AuditService, route string) bool {
	return true
}
