package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

// AuthReauthHandler serves POST /api/v1/auth/reauth. A signed-in user presents
// their password again together with the refresh token of the session the
// bearer access token belongs to; the session's pair is rotated into one whose
// auth_time is the time of this re-authentication.
type AuthReauthHandler struct {
	refresh     *AuthRefreshHandler
	authService *application.AuthService
}

// NewAuthReauthHandler builds the re-authentication handler on the refresh
// route's collaborators, so the refresh token is checked, retired and rotated
// through the same paths and the account is read from the user record. The
// auth service verifies the password with the sign-in's own checks (lockout,
// deactivation, an account without a password, failure records); it is
// mandatory, as the user repository and the audit service are.
func NewAuthReauthHandler(jwtService *auth.JWTService, sdkTokenService *application.SDKTokenService, users domain.UserRepository, audit *application.AuditService, authService *application.AuthService) *AuthReauthHandler {
	if authService == nil {
		panic("NewAuthReauthHandler: auth service is required")
	}
	return &AuthReauthHandler{
		refresh:     NewAuthRefreshHandler(jwtService, sdkTokenService, users, audit),
		authService: authService,
	}
}

// ReauthRequest is the body of POST /api/v1/auth/reauth.
type ReauthRequest struct {
	Password     string `json:"password"`
	RefreshToken string `json:"refreshToken"`
}

// Reauthenticate godoc
// @Summary Re-authenticate the signed-in user
// @Description Verifies the signed-in user's password again and rotates the session's login refresh token into a pair whose auth_time is now. refreshToken must be the current login refresh token of the session the bearer access token belongs to. The answer has the shape of POST /api/v1/auth/refresh.
// @Tags auth
// @Accept json
// @Produce json
// @Param body body ReauthRequest true "The user's password and the session's refresh token"
// @Success 200 {object} RefreshTokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/reauth [post]
func (h *AuthReauthHandler) Reauthenticate(c fiber.Ctx) error {
	var req ReauthRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	if req.Password == "" || req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "password and refreshToken are required",
		})
	}

	jwtService := h.refresh.jwtService
	ctx := c.Context()

	// The refresh token must be a login refresh token of the bearer's own
	// user, organization and session (family). An access token without a
	// session (one minted before sessions carried one, or a standalone token)
	// has no refresh token that matches, and neither has one minted from an
	// SDK-download token, whose session is the download's.
	userID, _ := c.Locals("user_id").(uuid.UUID)
	orgID, _ := c.Locals("organization_id").(uuid.UUID)
	session, _ := c.Locals("sid").(string)
	claims, err := jwtService.ValidateToken(req.RefreshToken)
	if err != nil || userID == uuid.Nil || session == "" ||
		claims.TokenType != auth.TokenTypeRefresh || claims.Issuer != auth.IssuerUser ||
		claims.FamilyID() != session || claims.UserID != userID.String() || claims.OrganizationID != orgID.String() {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired refresh token",
		})
	}

	// A retired refresh token presented again is a reuse, handled as the
	// refresh route handles it: the whole session is revoked and the reuse
	// recorded. A member of a revoked session is refused and recorded the same
	// way: the session middleware refuses one first, and this read catches a
	// session another instance revoked while the middleware's cached answer
	// stands. Both reads are uncached; when the store did not answer,
	// enforcement follows the fail-open setting and nothing is recorded.
	if revoked, known := jwtService.CheckRevoked(ctx, claims.ID); revoked {
		if known {
			h.refresh.reuseDetected(c, req.RefreshToken)
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token has been revoked or is invalid",
		})
	}
	if revoked, known := jwtService.CheckFamilyRevoked(ctx, session); revoked {
		if known {
			h.refresh.recordRefusal(c, claims, domain.AuditActionRefreshSessionRevoked, nil, "")
		}
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token has been revoked or is invalid",
		})
	}

	user, refusal := h.refresh.refreshPrincipal(claims)
	if refusal != "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": refusal,
		})
	}

	// The password is verified as at sign-in: a lockout, a deactivated
	// account and an account without a password are refused there, and a
	// wrong password is recorded as a failed attempt that counts toward a
	// lockout. Every refusal answers the same line.
	verified, err := h.authService.LoginWithPasswordExtended(ctx, user.Email, req.Password, c.IP(), c.Get("User-Agent"))
	if err != nil || verified == nil || verified.ID != user.ID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid password",
		})
	}

	// Mint, retire, answer, as the refresh route rotates a login token. The
	// successor pair is handed out only when the presented refresh token was
	// retired. A presentation that loses the retirement to another one is a
	// reuse. Where the token cannot be retired (no revocation store, or a
	// failed write) the presented refresh token goes back unchanged
	// (rotated=false) with an access token that carries the new auth_time,
	// so the session never holds two live refresh tokens.
	newAccessToken, newRefreshToken, err := jwtService.ReauthenticatedPair(claims, verified.Email, string(verified.Role))
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	rotated := false
	retired, lost, retireErr := jwtService.RetireTokenCheckedFrom(ctx, req.RefreshToken, presenter(c))
	if lost {
		h.refresh.reuseDetected(c, req.RefreshToken)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token has been revoked or is invalid",
		})
	}
	if retired {
		rotated = true
	} else {
		newRefreshToken = req.RefreshToken
		if retireErr != nil {
			log.Printf("warning: Re-authentication: denylist write failed for jti %s; the presented refresh token was returned unchanged: %v", claims.ID, retireErr)
		}
	}

	// A login refresh token with an sdk_tokens row (a recovered SDK
	// credential) keeps its row in step with the rotation, as at refresh, so
	// the user's SDK token list still names the live token. A failure is
	// logged without changing the answer.
	if rotated && h.refresh.sdkTokenService != nil {
		sum := sha256.Sum256([]byte(req.RefreshToken))
		oldTokenHash := hex.EncodeToString(sum[:])
		if row, lookupErr := h.refresh.sdkTokenService.ValidateToken(ctx, oldTokenHash); lookupErr == nil && row != nil {
			if rotateErr := h.refresh.rotateSDKTokenRow(c, row, oldTokenHash, claims.ID, newRefreshToken); rotateErr != nil {
				log.Printf("warning: Re-authentication: sdk_tokens rotation failed for token id %s: %v", claims.ID, rotateErr)
			}
		}
	}

	recordSignIn(c, h.refresh.audit, jwtService, newRefreshToken, fiber.Map{"method": "reauthentication"})

	return c.JSON(RefreshTokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    jwtService.AccessTTLSeconds(),
		Rotated:      rotated,
	})
}
