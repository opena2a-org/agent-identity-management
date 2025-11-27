package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type APIKeyHandler struct {
	apiKeyService *application.APIKeyService
	auditService  *application.AuditService
}

func NewAPIKeyHandler(
	apiKeyService *application.APIKeyService,
	auditService *application.AuditService,
) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
		auditService:  auditService,
	}
}

// ListAPIKeys returns all API keys for the organization
func (h *APIKeyHandler) ListAPIKeys(c fiber.Ctx) error {
	// 🔍 Safe type assertion with error checking
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID type in context",
		})
	}

	// Optional: filter by agent
	agentIDStr := c.Query("agent_id")
	var agentID *uuid.UUID
	if agentIDStr != "" {
		parsed, err := uuid.Parse(agentIDStr)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid agent ID",
			})
		}
		agentID = &parsed
	}

	apiKeys, err := h.apiKeyService.ListAPIKeys(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch API keys",
		})
	}

	// Filter by agent ID if provided
	if agentID != nil {
		filtered := []*domain.APIKey{}
		for _, key := range apiKeys {
			if key.AgentID == *agentID {
				filtered = append(filtered, key)
			}
		}
		apiKeys = filtered
	}

	return c.JSON(fiber.Map{
		"apiKeys": apiKeys,
		"total":   len(apiKeys),
	})
}

// CreateAPIKey generates a new API key
func (h *APIKeyHandler) CreateAPIKey(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req struct {
		AgentID   string  `json:"agent_id"`
		Name      string  `json:"name"`
		ExpiresAt *string `json:"expires_at"`
	}

	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.AgentID == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "agent_id and name are required",
		})
	}

	agentID, err := uuid.Parse(req.AgentID)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Parse expiration days (default to 90 days if not provided)
	expiresInDays := 90
	if req.ExpiresAt != nil {
		// TODO: Parse expires_at timestamp and convert to days
		// For now, using default
	}

	plainKey, apiKey, err := h.apiKeyService.GenerateAPIKey(
		c.Context(),
		agentID,
		orgID,
		userID,
		req.Name,
		expiresInDays,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCreate,
		"api_key",
		apiKey.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"keyName": req.Name,
			"agentId": agentID.String(),
		},
	)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":        apiKey.ID,
		"apiKey":    plainKey, // Only returned once!
		"name":      apiKey.Name,
		"agentId":   apiKey.AgentID,
		"expiresAt": apiKey.ExpiresAt,
		"createdAt": apiKey.CreatedAt,
	})
}

// DisableAPIKey disables an API key (sets is_active=false)
func (h *APIKeyHandler) DisableAPIKey(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	keyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid API key ID",
		})
	}

	if err := h.apiKeyService.RevokeAPIKey(c.Context(), keyID, orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionRevoke,
		"api_key",
		keyID,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.JSON(fiber.Map{
		"message": "API key disabled successfully",
	})
}

// DeleteAPIKey permanently deletes an API key (only if disabled)
func (h *APIKeyHandler) DeleteAPIKey(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	keyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid API key ID",
		})
	}

	if err := h.apiKeyService.DeleteAPIKey(c.Context(), keyID, orgID); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionDelete,
		"api_key",
		keyID,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.SendStatus(fiber.StatusNoContent)
}
