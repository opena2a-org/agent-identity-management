package handlers

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// recordSignIn writes the audit row for a sign-in that issued a login pair:
// one login row in the user's organization naming the family it issued
// (familyId, the sid the issued tokens carry), so a family met later on a
// refresh, a logout or a refusal resolves to the user and organization that
// signed in. The user, organization and family are read from the issued
// refresh token itself, so the row names exactly what was issued. meta holds
// the route's own fields; the row carries identifiers only, never a token. A
// row that cannot be written is logged and does not fail the sign-in. nil-safe
// on the audit service for handlers built by struct literal.
func recordSignIn(c fiber.Ctx, audit *application.AuditService, jwtService *auth.JWTService, refreshToken string, meta fiber.Map) {
	if audit == nil || jwtService == nil {
		return
	}
	claims, err := jwtService.ValidateToken(refreshToken)
	if err != nil {
		log.Printf("⚠️  Sign-in: issued refresh token not readable, audit row not written: %v", err)
		return
	}
	userID, err1 := uuid.Parse(claims.UserID)
	orgID, err2 := uuid.Parse(claims.OrganizationID)
	if err1 != nil || err2 != nil {
		log.Printf("⚠️  Sign-in: issued refresh token names no user or organization, audit row not written")
		return
	}
	family := claims.FamilyID()
	meta["familyId"] = family
	if err := audit.LogAction(c.Context(), orgID, userID, domain.AuditActionLogin, "user", userID, c.IP(), c.Get("User-Agent"), meta); err != nil {
		log.Printf("⚠️  Sign-in: audit row for family %s not written: %v", family, err)
	}
}
