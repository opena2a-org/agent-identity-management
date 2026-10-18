package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

type SDKTokenRecoveryHandler struct {
	sdkTokenService *application.SDKTokenService
	jwtService      *auth.JWTService
	users           domain.UserRepository
	audit           *application.AuditService
}

// NewSDKTokenRecoveryHandler builds the recovery handler. The recovered pair's
// role and email are read from the user record, so the user repository is
// mandatory (boot-time refusal on nil, no fallback).
func NewSDKTokenRecoveryHandler(
	sdkTokenService *application.SDKTokenService,
	jwtService *auth.JWTService,
	users domain.UserRepository,
	audit *application.AuditService,
) *SDKTokenRecoveryHandler {
	if users == nil {
		panic("NewSDKTokenRecoveryHandler: user repository is required")
	}
	return &SDKTokenRecoveryHandler{
		sdkTokenService: sdkTokenService,
		jwtService:      jwtService,
		users:           users,
		audit:           audit,
	}
}

type RecoverTokenRequest struct {
	OldRefreshToken string `json:"oldRefreshToken" validate:"required"`
}

type RecoverTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	TokenType    string `json:"tokenType"`
	ExpiresIn    int    `json:"expiresIn"`
	Message      string `json:"message"`
}

// RecoverRevokedToken allows users to get a new SDK token when their old one was revoked
// This prevents the need to re-download the entire SDK package
func (h *SDKTokenRecoveryHandler) RecoverRevokedToken(c fiber.Ctx) error {
	var req RecoverTokenRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request format",
		})
	}

	// A revoked session cannot recover an SDK credential with its still-valid
	// access token, and neither can an access token minted from an
	// SDK-download token: a recovery starts a new family, and that needs a
	// sign-in.
	if !refuseUnlessLiveSignIn(c, h.jwtService, h.audit, "sdk_recover") {
		return nil
	}

	// Validate old token and extract user info (even if revoked)
	tokenID, err := h.jwtService.GetTokenID(req.OldRefreshToken)
	if err != nil || tokenID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid refresh token format",
		})
	}

	// Get hash of old token
	hasher := sha256.New()
	hasher.Write([]byte(req.OldRefreshToken))
	oldTokenHash := hex.EncodeToString(hasher.Sum(nil))

	// Get old token info from database (even if revoked)
	oldToken, err := h.sdkTokenService.GetByTokenHash(c.Context(), oldTokenHash)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Token not found - it may have been deleted",
		})
	}

	// Verify the old token was actually revoked (not just expired)
	// The route mints only for the acting principal: the bearer's user and
	// organisation, set by the auth middleware, must be the revoked token's
	// owner. Any other account, or no principal at all, is answered exactly as
	// not-found, so the route can neither resurrect another user's credential
	// nor reveal which tokens exist.
	actingUser, _ := c.Locals("user_id").(uuid.UUID)
	actingOrg, _ := c.Locals("organization_id").(uuid.UUID)
	if actingUser == uuid.Nil || actingUser != oldToken.UserID || actingOrg != oldToken.OrganizationID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Token not found - it may have been deleted",
		})
	}

	if oldToken.RevokedAt == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Token is still valid - use /auth/refresh instead",
		})
	}

	// A token family a reuse, a logout, its maximum age or a password change
	// ended is not recovered: the chain that grew from a reused token cannot
	// be told apart from the owner's, a logout ends the session it belongs to,
	// a family past SDK_TOKEN_FAMILY_MAX_AGE is over, and a password change
	// refuses every token minted before it, so the holder downloads a new SDK
	// instead. The family is ended when any of its rows was revoked for one of
	// those, or when the revocation store says so (an unanswered store follows
	// the fail-open setting). The answer is the refresh route's answer to a
	// revoked token. A family whose rows cannot be read is not recovered
	// either.
	ended, err := h.familyEnded(c, req.OldRefreshToken, oldToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to read the token family",
		})
	}
	if ended {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Token has been revoked or is invalid",
		})
	}

	// The recovered pair carries the account's CURRENT role and email, read
	// from the database; an account that may no longer hold a session cannot
	// recover one.
	user, err := h.users.GetByID(oldToken.UserID)
	if err != nil || user == nil || user.OrganizationID != oldToken.OrganizationID {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired refresh token",
		})
	}
	if !user.CanHoldSession() {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Account is not active",
		})
	}

	// Generate new SDK token pair for the same user
	newAccessToken, newRefreshToken, err := h.jwtService.GenerateTokenPair(
		oldToken.UserID.String(),
		oldToken.OrganizationID.String(),
		user.Email,
		string(user.Role),
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate new tokens",
		})
	}

	// Get new token ID from refresh token
	newTokenID, err := h.jwtService.GetTokenID(newRefreshToken)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to extract new token ID",
		})
	}

	// Hash the new refresh token
	newHasher := sha256.New()
	newHasher.Write([]byte(newRefreshToken))
	newTokenHash := hex.EncodeToString(newHasher.Sum(nil))

	// Get client info
	ipAddress := c.IP()
	userAgent := c.Get("User-Agent")

	// Create new SDK token entry. The recovered credential is the root of a
	// new family, so its row's birth is its own creation time.
	now := time.Now()
	newSDKToken := &domain.SDKToken{
		ID:                uuid.New(),
		UserID:            oldToken.UserID,
		OrganizationID:    oldToken.OrganizationID,
		TokenHash:         newTokenHash,
		TokenID:           newTokenID,
		DeviceName:        oldToken.DeviceName,
		DeviceFingerprint: oldToken.DeviceFingerprint,
		IPAddress:         &ipAddress,
		UserAgent:         &userAgent,
		CreatedAt:         now,
		ExpiresAt:         now.Add(90 * 24 * time.Hour), // 90 days
		Metadata: map[string]interface{}{
			"source":         "token_recovery",
			"recoveredFrom":  tokenID,
			"recoveryReason": "token_revoked",
		},
	}
	newSDKToken.SetFamilyCreatedAt(now)

	// Save new token to database
	if err := h.sdkTokenService.CreateToken(c.Context(), newSDKToken); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save new token",
		})
	}
	recordSignIn(c, h.audit, h.jwtService, newRefreshToken, fiber.Map{"method": "sdk_token_recovery", "recoveredFrom": tokenID})

	return c.JSON(RecoverTokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    86400, // 24 hours
		Message:      "Token recovered successfully - SDK credentials updated automatically",
	})
}

// familyEnded reports whether the presented token's family was ended. The
// family's own rows decide it without a revocation store: a logout writes the
// family key only where a store is configured, and a revoke reason on one of
// the family's rows records it with or without one. The presented row, the
// rows its rotations grew from and the rows that grew from it are read
// (SDKTokenService.FamilyRows), so a logout with the family's live token ends
// recovery of the family's earlier tokens. The revocation store's family key
// ends the family as well. A failed read of the rows gives its error.
func (h *SDKTokenRecoveryHandler) familyEnded(c fiber.Ctx, token string, row *domain.SDKToken) (bool, error) {
	if endsFamily(row) {
		return true, nil
	}
	members, err := h.sdkTokenService.FamilyRows(c.Context(), row)
	if err != nil {
		return false, err
	}
	for _, member := range members {
		if endsFamily(member) {
			return true, nil
		}
	}
	claims, err := h.jwtService.ValidateToken(token)
	if err != nil {
		return false, nil
	}
	revoked, _ := h.jwtService.CheckFamilyRevoked(c.Context(), claims.FamilyID())
	return revoked, nil
}

// endsFamily reports whether the row was revoked for a reason that ends its
// token family: a reuse, a logout, the family's maximum age, or a password
// change. A row retired by a rotation does not end its family by itself.
func endsFamily(row *domain.SDKToken) bool {
	return row.RevokedFor(domain.SDKTokenRevokeReasonFamilyRevoked) ||
		row.RevokedFor(sdkTokenLogoutReason) ||
		row.RevokedFor(domain.SDKTokenRevokeReasonFamilyMaxAge) ||
		row.RevokedFor(domain.SDKTokenRevokeReasonPasswordChanged)
}
