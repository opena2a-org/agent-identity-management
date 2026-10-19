package middleware

import (
	"errors"
	"log"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// PasswordChangeRequiredCode is the code of the 403 a session of an account
// that must change its password is answered with: by AuthMiddleware on every
// route but GET /api/v1/auth/me and POST /api/v1/auth/change-password, and by
// the refresh route.
const PasswordChangeRequiredCode = "passwordChangeRequired"

// PasswordChangeRequired answers 403 with PasswordChangeRequiredCode and the
// route that clears it.
func PasswordChangeRequired(c fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"error": "The account's password must be changed first: POST /api/v1/auth/change-password",
		"code":  PasswordChangeRequiredCode,
	})
}

// passwordChangeRoute reports whether the request is one a session of an
// account that must change its password may make through AuthMiddleware:
// GET /api/v1/auth/me (who is signed in) and POST /api/v1/auth/change-password
// (the change itself). POST /api/v1/auth/logout sits in the public auth group
// and reads no session here. The path is compared as the router matches it,
// which is case-insensitive and admits one trailing slash.
func passwordChangeRoute(c fiber.Ctx) bool {
	path := strings.ToLower(strings.TrimSuffix(c.Path(), "/"))
	switch c.Method() {
	case fiber.MethodGet:
		return path == "/api/v1/auth/me"
	case fiber.MethodPost:
		return path == "/api/v1/auth/change-password"
	}
	return false
}

// sessionStateRefusal reads the user record behind a login access token and
// answers the request when the record refuses the session. A deactivated
// account (status deactivated, or deleted_at set, as sign-in reads it) is
// answered 401: deactivating an account, by an admin or by the user's own
// deletion request, ends its sessions. A token whose sign-in precedes the
// account's last password change is answered 401: the change ends every
// session from before it (the session that made the change is issued a new
// pair). While the account must change its password, every request but
// passwordChangeRoute is answered 403 PasswordChangeRequiredCode. A user that
// no longer exists is answered 401, and a record that cannot be read 503.
// Nothing is read, and nothing refused, when the JWTService has no reader
// attached.
func sessionStateRefusal(c fiber.Ctx, jwtService *auth.JWTService, claims *auth.JWTClaims, userID uuid.UUID) (refused bool, answer error) {
	state, attached, err := jwtService.SessionState(c.Context(), userID)
	if !attached {
		return false, nil
	}
	if errors.Is(err, domain.ErrUserNotFound) {
		return true, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired token",
		})
	}
	if err != nil {
		log.Printf("Auth: the user record of user %s could not be read: %v", userID, err)
		return true, c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "The account could not be read; try again",
		})
	}
	if state.Deactivated {
		return true, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token has been revoked: the account is deactivated",
		})
	}
	if auth.SignedInBeforePasswordChange(claims, state.PasswordChangedAt) {
		return true, c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token has been revoked: the account's password changed after this sign-in",
		})
	}
	if state.ForcePasswordChange && !passwordChangeRoute(c) {
		return true, PasswordChangeRequired(c)
	}
	return false, nil
}
