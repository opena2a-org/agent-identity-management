package handlers

import (
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// refuseIfFamilyRevoked guards a credential-minting route (an SDK download, a
// device sign-in approval, an SDK credential recovery). After a session is
// revoked (a reused refresh token, or logout) its access token stays valid
// until it expires; this is the window in which a stolen access token could
// turn into a 90-day credential or a new sign-in. The route reads the acting
// token's family (set by the auth middleware) through the uncached store read
// and, when the family is revoked, answers the unchanged 401 the refresh route
// uses (so the refusal tells the holder nothing more) and reports false. When
// the store confirmed the revocation, one credential_mint_refused audit row
// and one SECURITY line are recorded, identifiers only. When the store did
// not answer, enforcement follows the fail-open setting and nothing is
// recorded. A token with no family (minted before families existed) is not
// checked. nil-safe on the audit service.
func refuseIfFamilyRevoked(c fiber.Ctx, jwtService *auth.JWTService, audit *application.AuditService, route string) bool {
	family, _ := c.Locals("sid").(string)
	if family == "" || jwtService == nil {
		return true
	}
	revoked, known := jwtService.CheckFamilyRevoked(c.Context(), family)
	if !revoked {
		return true
	}
	if known {
		jti, _ := c.Locals("jti").(string)
		userID, _ := c.Locals("user_id").(uuid.UUID)
		orgID, _ := c.Locals("organization_id").(uuid.UUID)
		ip, ua := c.IP(), c.Get("User-Agent")
		log.Printf("SECURITY %s route=%s user=%s org=%s family=%s jti=%s ip=%s ua=%q", domain.AuditActionCredentialMintRefused, route, userID, orgID, family, jti, ip, ua)
		if audit != nil && userID != uuid.Nil && orgID != uuid.Nil {
			meta := map[string]interface{}{"familyId": family, "jti": jti, "route": route}
			if err := audit.LogAction(c.Context(), orgID, userID, domain.AuditActionCredentialMintRefused, "user", userID, ip, ua, meta); err != nil {
				log.Printf("⚠️  %s: audit row for family %s not written: %v", route, family, err)
			}
		}
	}
	_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": "Token has been revoked or is invalid",
	})
	return false
}
