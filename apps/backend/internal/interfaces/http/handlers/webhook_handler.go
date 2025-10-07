package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type WebhookHandler struct {
	webhookService *application.WebhookService
	auditService   *application.AuditService
}

func NewWebhookHandler(
	webhookService *application.WebhookService,
	auditService *application.AuditService,
) *WebhookHandler {
	return &WebhookHandler{
		webhookService: webhookService,
		auditService:   auditService,
	}
}

// CreateWebhook creates a new webhook subscription
// @Summary Create webhook
// @Description Create a new webhook subscription for event notifications
// @Tags webhooks
// @Accept json
// @Produce json
// @Param request body application.CreateWebhookRequest true "Webhook details"
// @Success 201 {object} domain.Webhook
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/webhooks [post]
func (h *WebhookHandler) CreateWebhook(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	var req application.CreateWebhookRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	webhook, err := h.webhookService.CreateWebhook(c.Context(), &req, orgID, userID)
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
		"webhook",
		webhook.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"webhook_name": webhook.Name,
			"webhook_url":  webhook.URL,
		},
	)

	return c.Status(fiber.StatusCreated).JSON(webhook)
}

// ListWebhooks lists all webhooks for the organization
// @Summary List webhooks
// @Description Get all webhook subscriptions for the organization
// @Tags webhooks
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/webhooks [get]
func (h *WebhookHandler) ListWebhooks(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	webhooks, err := h.webhookService.ListWebhooks(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch webhooks",
		})
	}

	return c.JSON(fiber.Map{
		"webhooks": webhooks,
		"total":    len(webhooks),
	})
}

// GetWebhook retrieves a single webhook
// @Summary Get webhook
// @Description Get details of a specific webhook
// @Tags webhooks
// @Produce json
// @Param id path string true "Webhook ID"
// @Success 200 {object} domain.Webhook
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/webhooks/{id} [get]
func (h *WebhookHandler) GetWebhook(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	webhookID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid webhook ID",
		})
	}

	webhook, err := h.webhookService.GetWebhook(c.Context(), webhookID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Webhook not found",
		})
	}

	// Verify webhook belongs to organization
	if webhook.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	return c.JSON(webhook)
}

// DeleteWebhook deletes a webhook
// @Summary Delete webhook
// @Description Delete a webhook subscription
// @Tags webhooks
// @Param id path string true "Webhook ID"
// @Success 204
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/webhooks/{id} [delete]
func (h *WebhookHandler) DeleteWebhook(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	webhookID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid webhook ID",
		})
	}

	// Verify webhook belongs to organization
	webhook, err := h.webhookService.GetWebhook(c.Context(), webhookID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Webhook not found",
		})
	}
	if webhook.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	if err := h.webhookService.DeleteWebhook(c.Context(), webhookID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionDelete,
		"webhook",
		webhookID,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.SendStatus(fiber.StatusNoContent)
}

// TestWebhook sends a test payload to a webhook
// @Summary Test webhook
// @Description Send a test payload to verify webhook functionality
// @Tags webhooks
// @Produce json
// @Param id path string true "Webhook ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/webhooks/{id}/test [post]
func (h *WebhookHandler) TestWebhook(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	webhookID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid webhook ID",
		})
	}

	// Verify webhook belongs to organization
	webhook, err := h.webhookService.GetWebhook(c.Context(), webhookID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Webhook not found",
		})
	}
	if webhook.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	if err := h.webhookService.TestWebhook(c.Context(), webhookID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionTest,
		"webhook",
		webhookID,
		c.IP(),
		c.Get("User-Agent"),
		nil,
	)

	return c.JSON(fiber.Map{
		"message":    "Test webhook sent successfully",
		"webhook_id": webhookID,
	})
}
