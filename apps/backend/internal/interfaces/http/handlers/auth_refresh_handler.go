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

// AuthRefreshHandler handles token refresh operations
type AuthRefreshHandler struct {
	jwtService      *auth.JWTService
	sdkTokenService *application.SDKTokenService
}

// NewAuthRefreshHandler creates a new auth refresh handler
func NewAuthRefreshHandler(jwtService *auth.JWTService, sdkTokenService *application.SDKTokenService) *AuthRefreshHandler {
	return &AuthRefreshHandler{
		jwtService:      jwtService,
		sdkTokenService: sdkTokenService,
	}
}

// RefreshToken godoc
// @Summary Refresh access token
// @Description Refresh access token using refresh token (with token rotation)
// @Tags auth
// @Accept json
// @Produce json
// @Param body body RefreshTokenRequest true "Refresh token"
// @Success 200 {object} RefreshTokenResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/auth/refresh [post]
func (h *AuthRefreshHandler) RefreshToken(c fiber.Ctx) error {
	var req RefreshTokenRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.RefreshToken == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "refresh_token is required",
		})
	}

	// Check if this is an SDK token and verify it's not revoked BEFORE rotating
	tokenID, err := h.jwtService.GetTokenID(req.RefreshToken)
	if err == nil && tokenID != "" {
		// Hash the token to check if it's tracked and revoked
		hasher := sha256.New()
		hasher.Write([]byte(req.RefreshToken))
		tokenHash := hex.EncodeToString(hasher.Sum(nil))

		// Check if token is tracked and not revoked
		_, err := h.sdkTokenService.ValidateToken(c.Context(), tokenHash)
		if err != nil {
			// Token is revoked or invalid in database
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Token has been revoked or is invalid",
			})
		}
	}

	// Validate refresh token and generate new tokens (with rotation)
	newAccessToken, newRefreshToken, err := h.jwtService.RefreshTokenPair(req.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired refresh token",
		})
	}

	// If this is a tracked SDK token, track usage and create new token entry
	// NOTE: We do NOT revoke old tokens on rotation - this allows multiple SDK instances
	// to work independently (like GitHub, Google, etc. handle device sessions)
	// Old tokens expire naturally after 90 days
	if tokenID != "" {
		hasher := sha256.New()
		hasher.Write([]byte(req.RefreshToken))
		oldTokenHash := hex.EncodeToString(hasher.Sum(nil))

		// Get old token info for creating new token entry
		oldToken, _ := h.sdkTokenService.ValidateToken(c.Context(), oldTokenHash)

		// Record usage on the old token (updates last_used_at, usage_count)
		ipAddress := c.IP()
		_ = h.sdkTokenService.RecordTokenUsage(c.Context(), tokenID, ipAddress)

		// IMPORTANT: We do NOT revoke the old token anymore!
		// This was causing issues with multiple SDK instances:
		// - SDK A downloads → Token A
		// - SDK B downloads → Token B
		// - SDK A refreshes → Old behavior: Token A revoked, Token A' created
		// - SDK B refreshes → Token B is still valid, Token B' created
		// - Now both A' and B' work independently!
		//
		// Old tokens will expire naturally after 90 days.
		// For security, we still track token lineage via metadata.

		// Save the new rotated SDK token to database
		if oldToken != nil {
			// Get new token ID from rotated refresh token
			newTokenID, err := h.jwtService.GetTokenID(newRefreshToken)
			if err == nil && newTokenID != "" {
				// Hash the new token
				newHasher := sha256.New()
				newHasher.Write([]byte(newRefreshToken))
				newTokenHash := hex.EncodeToString(newHasher.Sum(nil))

				// Get client info
				newIPAddress := c.IP()
				userAgent := c.Get("User-Agent")

				// Get rotation count from old token metadata
				rotationCount := 1
				if oldToken.Metadata != nil {
					if count, ok := oldToken.Metadata["rotationCount"].(float64); ok {
						rotationCount = int(count) + 1
					}
				}

				// Create new SDK token entry (old token remains valid until expiry)
				newSDKToken := &domain.SDKToken{
					ID:                uuid.New(),
					UserID:            oldToken.UserID,
					OrganizationID:    oldToken.OrganizationID,
					TokenHash:         newTokenHash,
					TokenID:           newTokenID,
					DeviceName:        oldToken.DeviceName,
					DeviceFingerprint: oldToken.DeviceFingerprint,
					IPAddress:         &newIPAddress,
					UserAgent:         &userAgent,
					CreatedAt:         time.Now(),
					ExpiresAt:         time.Now().Add(90 * 24 * time.Hour), // 90 days
					Metadata: map[string]interface{}{
						"source":        "token_rotation",
						"rotated_from":  tokenID,
						"rotationCount": rotationCount,
						"parent_token":  oldToken.ID.String(), // Track token lineage
					},
				}

				// Save to database (critical for next rotation)
				_ = h.sdkTokenService.CreateToken(c.Context(), newSDKToken)
			}
		}
	}

	// Return new tokens
	return c.JSON(RefreshTokenResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    86400, // 24 hours in seconds
	})
}

// Request/Response types
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"` // New refresh token (token rotation)
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}
