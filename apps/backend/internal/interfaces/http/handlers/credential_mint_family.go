package handlers

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// Causes a credential route refuses an acting access token for, recorded on
// the credential_mint_refused row and its SECURITY line.
const (
	mintRefusedSessionRevoked = "sessionRevoked"
	mintRefusedNoSignIn       = "noSignIn"
)

// refuseUnlessLiveSignIn guards a route that starts a new credential family
// or sign-in (an SDK download, a device sign-in approval, an SDK credential
// recovery) or creates a credential that outlives the acting session (an API
// key's creation): the acting access token must belong to a session that
// records a sign-in, and then refuseIfFamilyRevoked applies.
//
// An access token minted from an SDK-download token carries the download's
// family (set by the auth middleware as sid) and no auth_time: nobody signed
// in, a stored credential was presented. Such a token is refused before
// anything is read, with the unchanged 401 the refresh route uses, so a
// credential cannot mint a credential and an SDK-download family cannot start
// a new family, and a new birth, or create an API key without a sign-in. The
// refusal records one credential_mint_refused audit row and one SECURITY line
// whose cause is noSignIn. A token with no family (minted before families
// existed) is not checked. Reports false when the route must stop; it has
// then written the answer.
func refuseUnlessLiveSignIn(c fiber.Ctx, jwtService *auth.JWTService, audit AuditServicer, route string) bool {
	family, _ := c.Locals("sid").(string)
	if family == "" {
		return true
	}
	if signedIn, _ := c.Locals("auth_time").(time.Time); signedIn.IsZero() {
		recordMintRefused(c, audit, route, family, mintRefusedNoSignIn)
		return refuseMint(c)
	}
	return refuseIfFamilyRevoked(c, jwtService, audit, route)
}

// refuseIfFamilyRevoked guards a route that hands out a credential, sets one
// the caller then holds, or chooses where webhook deliveries go: an SDK
// download, a device sign-in approval, an SDK credential recovery, an API
// key's creation, an agent's key read, rotated or replaced, a secret's
// rotation, a webhook's creation or update. After a session is revoked (a
// reused refresh token, or logout) its access token stays valid until it
// expires; this is the window in which a stolen access token could turn into
// a credential that outlives the session or a new sign-in. The route reads the
// acting token's family (set by the auth middleware) through the uncached
// store read and, when the family is revoked, answers the unchanged 401 the
// refresh route uses (so the refusal tells the holder nothing more) and
// reports false. When the store confirmed the revocation, one
// credential_mint_refused audit row and one SECURITY line are recorded,
// identifiers and the cause sessionRevoked only. When the store did not
// answer, enforcement follows the fail-open setting and nothing is recorded.
// A token with no family (minted before families existed) is not checked.
// nil-safe on the audit service, including a nil *application.AuditService.
func refuseIfFamilyRevoked(c fiber.Ctx, jwtService *auth.JWTService, audit AuditServicer, route string) bool {
	family, _ := c.Locals("sid").(string)
	if family == "" || jwtService == nil {
		return true
	}
	revoked, known := jwtService.CheckFamilyRevoked(c.Context(), family)
	if !revoked {
		return true
	}
	if known {
		recordMintRefused(c, audit, route, family, mintRefusedSessionRevoked)
	}
	return refuseMint(c)
}

func refuseMint(c fiber.Ctx) bool {
	_ = c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		"error": "Token has been revoked or is invalid",
	})
	return false
}

func recordMintRefused(c fiber.Ctx, audit AuditServicer, route, family, cause string) {
	jti, _ := c.Locals("jti").(string)
	userID, _ := c.Locals("user_id").(uuid.UUID)
	orgID, _ := c.Locals("organization_id").(uuid.UUID)
	ip, ua := c.IP(), c.Get("User-Agent")
	log.Printf("SECURITY %s route=%s cause=%s user=%s org=%s family=%s jti=%s ip=%s ua=%q", domain.AuditActionCredentialMintRefused, route, cause, userID, orgID, family, jti, ip, ua)
	// A handler built without an audit service passes a nil *AuditService.
	if s, ok := audit.(*application.AuditService); ok && s == nil {
		audit = nil
	}
	if audit == nil || userID == uuid.Nil || orgID == uuid.Nil {
		return
	}
	meta := map[string]interface{}{"familyId": family, "jti": jti, "route": route, "cause": cause}
	if err := audit.LogAction(c.Context(), orgID, userID, domain.AuditActionCredentialMintRefused, "user", userID, ip, ua, meta); err != nil {
		log.Printf("⚠️  %s: audit row for family %s not written: %v", route, family, err)
	}
}
