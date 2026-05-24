package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type APIKeyHandler struct {
	apiKeyService *application.APIKeyService
	auditService  *application.AuditService

	// Repository handle for handler-layer tenant scoping. Used by
	// DisableAPIKey / DeleteAPIKey to LoadOwned the target key
	// before invoking the service. The service-layer check exists
	// (api_key_service.go:102, 117) but returns a distinct error
	// string for cross-tenant vs not-found which the handler echoes
	// via err.Error() — existence side channel. Handler-layer
	// LoadOwned collapses both to a fixed 404.
	apiKeyRepo domain.APIKeyRepository
}

func NewAPIKeyHandler(
	apiKeyService *application.APIKeyService,
	auditService *application.AuditService,
	apiKeyRepo domain.APIKeyRepository,
) *APIKeyHandler {
	return &APIKeyHandler{
		apiKeyService: apiKeyService,
		auditService:  auditService,
		apiKeyRepo:    apiKeyRepo,
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
	// Safe type assertion with error checking
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

	var req struct {
		AgentID   string  `json:"agentId"`
		Name      string  `json:"name"`
		ExpiresAt *string `json:"expiresAt"`
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
	// Safe type assertion with error checking
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

	keyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid API key ID",
		})
	}

	// SECURITY: verify the key belongs to the caller's org BEFORE
	// invoking the revoke flow. The service-layer check returns a
	// distinct error string for cross-tenant vs not-found which the
	// pre-fix handler echoed via err.Error() — existence side
	// channel for API key UUID enumeration across tenants.
	// LoadOwned collapses both branches to a fixed 404.
	if LoadOwned(c, h.apiKeyRepo.GetByID, keyID, orgID, apiKeyOrgID) == nil {
		return nil
	}

	if err := h.apiKeyService.RevokeAPIKey(c.Context(), keyID, orgID); err != nil {
		// Cross-tenant + not-found already collapsed to 404 above; any
		// remaining service error is operational. Use a generic body
		// rather than echoing err.Error() to avoid leaking server-side
		// detail.
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to revoke API key",
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
	// Safe type assertion with error checking
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

	keyID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid API key ID",
		})
	}

	// SECURITY: see DisableAPIKey for rationale. LoadOwned collapses
	// cross-tenant and not-found to a fixed 404.
	if LoadOwned(c, h.apiKeyRepo.GetByID, keyID, orgID, apiKeyOrgID) == nil {
		return nil
	}

	if err := h.apiKeyService.DeleteAPIKey(c.Context(), keyID, orgID); err != nil {
		// The "key must be disabled before deletion" rule is the only
		// remaining service-level rejection here (the cross-tenant and
		// not-found cases were collapsed by LoadOwned above). Echo
		// err.Error() to surface that specific guard message — it
		// carries no cross-tenant information.
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
