package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/opena2a/identity/backend/internal/infrastructure/auth"
)

type SDKTokenRecoveryHandler struct {
	sdkTokenService *application.SDKTokenService
	jwtService      *auth.JWTService
}

func NewSDKTokenRecoveryHandler(
	sdkTokenService *application.SDKTokenService,
	jwtService *auth.JWTService,
) *SDKTokenRecoveryHandler {
	return &SDKTokenRecoveryHandler{
		sdkTokenService: sdkTokenService,
		jwtService:      jwtService,
	}
}

type RecoverTokenRequest struct {
	OldRefreshToken string `json:"old_refresh_token" validate:"required"`
}

type RecoverTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
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
	if oldToken.RevokedAt == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Token is still valid - use /auth/refresh instead",
		})
	}

	// Generate new SDK token pair for the same user
	newAccessToken, newRefreshToken, err := h.jwtService.GenerateTokenPair(
		oldToken.UserID.String(),
		oldToken.OrganizationID.String(),
		"", // email will be populated from DB
		"", // role will be populated from DB
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

	// Create new SDK token entry
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
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(90 * 24 * time.Hour), // 90 days
		Metadata: map[string]interface{}{
			"source":          "token_recovery",
			"recoveredFrom":  tokenID,
			"recoveryReason": "token_revoked",
		},
	}

	// Save new token to database
	if err := h.sdkTokenService.CreateToken(c.Context(), newSDKToken); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save new token",
		})
	}

	return c.JSON(RecoverTokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    86400, // 24 hours
		Message:      "Token recovered successfully - SDK credentials updated automatically",
	})
}
