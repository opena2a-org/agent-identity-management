package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
)

// SDKTokenHandler handles SDK token management operations
type SDKTokenHandler struct {
	sdkTokenService *application.SDKTokenService
}

// NewSDKTokenHandler creates a new SDK token handler
func NewSDKTokenHandler(sdkTokenService *application.SDKTokenService) *SDKTokenHandler {
	return &SDKTokenHandler{
		sdkTokenService: sdkTokenService,
	}
}

// ListUserTokens godoc
// @Summary List user's SDK tokens
// @Description Get all SDK tokens for the authenticated user
// @Tags sdk-tokens
// @Produce json
// @Param includeRevoked query boolean false "Include revoked tokens"
// @Success 200 {array} domain.SDKToken
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/me/sdk-tokens [get]
// @Security BearerAuth
func (h *SDKTokenHandler) ListUserTokens(c fiber.Ctx) error {
	// 🔍 Safe type assertion with error checking
	userIDValue := c.Locals("user_id")
	if userIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User ID not found in context",
		})
	}

	userID, ok := userIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid user ID type in context",
		})
	}

	includeRevoked := c.Query("include_revoked", "false") == "true"

	tokens, err := h.sdkTokenService.GetUserTokens(c.Context(), userID, includeRevoked)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list SDK tokens",
		})
	}

	return c.JSON(fiber.Map{
		"tokens": tokens,
	})
}

// GetActiveTokenCount godoc
// @Summary Get active token count
// @Description Get count of active SDK tokens for the authenticated user
// @Tags sdk-tokens
// @Produce json
// @Success 200 {object} map[string]int
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/me/sdk-tokens/count [get]
// @Security BearerAuth
func (h *SDKTokenHandler) GetActiveTokenCount(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	count, err := h.sdkTokenService.GetActiveTokenCount(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get token count",
		})
	}

	return c.JSON(fiber.Map{
		"active_count": count,
	})
}

// RevokeToken godoc
// @Summary Revoke an SDK token
// @Description Revoke a specific SDK token by ID
// @Tags sdk-tokens
// @Accept json
// @Produce json
// @Param id path string true "Token ID"
// @Param body body RevokeTokenRequest false "Revocation reason"
// @Success 200 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/me/sdk-tokens/{id}/revoke [post]
// @Security BearerAuth
func (h *SDKTokenHandler) RevokeToken(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	tokenID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid token ID",
		})
	}

	// Parse optional reason
	var req RevokeTokenRequest
	if err := c.Bind().Body(&req); err != nil {
		// Ignore parse errors, reason is optional
		req.Reason = "User-initiated revocation"
	}

	if req.Reason == "" {
		req.Reason = "User-initiated revocation"
	}

	err = h.sdkTokenService.RevokeToken(c.Context(), tokenID, userID, req.Reason)
	if err != nil {
		if err.Error() == "unauthorized: token belongs to different user" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "You don't have permission to revoke this token",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to revoke token",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Token revoked successfully",
	})
}

// RevokeAllTokens godoc
// @Summary Revoke all SDK tokens
// @Description Revoke all SDK tokens for the authenticated user
// @Tags sdk-tokens
// @Accept json
// @Produce json
// @Param body body RevokeTokenRequest false "Revocation reason"
// @Success 200 {object} SuccessResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/users/me/sdk-tokens/revoke-all [post]
// @Security BearerAuth
func (h *SDKTokenHandler) RevokeAllTokens(c fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User not authenticated",
		})
	}

	// Parse optional reason
	var req RevokeTokenRequest
	if err := c.Bind().Body(&req); err != nil {
		req.Reason = "User revoked all SDK tokens"
	}

	if req.Reason == "" {
		req.Reason = "User revoked all SDK tokens"
	}

	err := h.sdkTokenService.RevokeAllUserTokens(c.Context(), userID, req.Reason)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to revoke tokens",
		})
	}

	return c.JSON(fiber.Map{
		"message": "All SDK tokens revoked successfully",
	})
}

// Request types
type RevokeTokenRequest struct {
	Reason string `json:"reason,omitempty"`
}
