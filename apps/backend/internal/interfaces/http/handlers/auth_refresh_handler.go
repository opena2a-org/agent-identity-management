package handlers

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a/identity/backend/internal/application"
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

	// Validate refresh token and generate new tokens (with rotation)
	newAccessToken, newRefreshToken, err := h.jwtService.RefreshTokenPair(req.RefreshToken)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Invalid or expired refresh token",
		})
	}

	// Check if this is an SDK token (for tracking)
	tokenID, err := h.jwtService.GetTokenID(req.RefreshToken)
	if err == nil && tokenID != "" {
		// Hash the old token to check if it's tracked
		hasher := sha256.New()
		hasher.Write([]byte(req.RefreshToken))
		oldTokenHash := hex.EncodeToString(hasher.Sum(nil))

		// Check if token is tracked in SDK tokens table
		sdkToken, err := h.sdkTokenService.ValidateToken(c.Context(), oldTokenHash)
		if err == nil && sdkToken != nil {
			// This is a tracked SDK token - record usage
			ipAddress := c.IP()
			_ = h.sdkTokenService.RecordTokenUsage(c.Context(), tokenID, ipAddress)

			// TODO: If token rotation is enabled, we should:
			// 1. Revoke the old token in database
			// 2. Track the new refresh token
			// For now, we'll just update usage stats
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
