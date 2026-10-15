package handlers

import (
	"errors"
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/utils"
)

// SDKTokenHandler handles SDK token management operations
type SDKTokenHandler struct {
	sdkTokenService *application.SDKTokenService
	jwtService      *auth.JWTService
}

// NewSDKTokenHandler creates a new SDK token handler. jwtService writes the
// family key when a revoke ends an SDK token's family; nil leaves the revoke
// to the rows.
func NewSDKTokenHandler(sdkTokenService *application.SDKTokenService, jwtService *auth.JWTService) *SDKTokenHandler {
	return &SDKTokenHandler{
		sdkTokenService: sdkTokenService,
		jwtService:      jwtService,
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

	// Metadata is stored as each issuing path wrote it, and refresh-token
	// rotation writes snake_case lineage keys (parent_token, rotated_from).
	// Map the keys to camelCase on a copy so the response keeps the API's
	// naming convention without rewriting the stored rows.
	response := make([]domain.SDKToken, len(tokens))
	for i, token := range tokens {
		response[i] = *token
		response[i].Metadata = utils.CamelCaseKeys(token.Metadata)
	}

	return c.JSON(fiber.Map{
		"tokens": response,
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
		"activeCount": count,
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

	row, err := h.sdkTokenService.RevokeTokenRow(c.Context(), tokenID, userID, req.Reason)
	if err != nil {
		// SECURITY: the service collapses "not found" and
		// "wrong user" to ErrSDKTokenNotFound. The handler maps
		// the sentinel to a fixed 404, closing the existence
		// oracle that previously distinguished 403 (cross-user)
		// from 500 (not-found) with distinct body strings.
		if errors.Is(err, application.ErrSDKTokenNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": "not found",
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to revoke token",
		})
	}
	h.endFamilies(c, "sdk_token_revoke", row)

	return c.JSON(fiber.Map{
		"message": "Token revoked successfully",
	})
}

// endFamilies writes the family key of every token family among the revoked
// rows, so the access token the SDK path minted for a family is refused at
// the credential-minting routes (/sdk/download, a recovery) at once rather
// than until it expires, and the family's refresh token at /auth/refresh, as
// after a reuse or a logout. A row's family is read from its rotation lineage
// (SDKTokenService.FamilyOf) and the key lasts for the longer of the refresh
// lifetime and the row's own lifetime, plus a minute (JWTService.
// RevokeFamilyFor). The rows are revoked whatever happens here: a family
// that cannot be read or a key that cannot be written is logged with
// identifiers only, and the revoke is answered as before. Nothing is written
// without a jwt service or a revocation store.
func (h *SDKTokenHandler) endFamilies(c fiber.Ctx, route string, rows ...*domain.SDKToken) {
	if h.jwtService == nil {
		return
	}
	done := map[string]bool{}
	for _, row := range rows {
		if row == nil {
			continue
		}
		family, err := h.sdkTokenService.FamilyOf(c.Context(), row)
		if err != nil {
			log.Printf("%s: family of sdk token row %s not found, its family key is not written: %v", route, row.ID, err)
			continue
		}
		if done[family] {
			continue
		}
		done[family] = true
		if _, err := h.jwtService.RevokeFamilyFor(c.Context(), family, row.ExpiresAt.Sub(row.CreatedAt)); err != nil {
			log.Printf("%s: family %s could not be revoked: %v", route, family, err)
		}
	}
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

	rows, err := h.sdkTokenService.RevokeAllUserTokenRows(c.Context(), userID, req.Reason)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to revoke tokens",
		})
	}
	h.endFamilies(c, "sdk_token_revoke_all", rows...)

	return c.JSON(fiber.Map{
		"message": "All SDK tokens revoked successfully",
	})
}

// Request types
type RevokeTokenRequest struct {
	Reason string `json:"reason,omitempty"`
}
