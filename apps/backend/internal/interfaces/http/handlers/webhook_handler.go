package handlers

import (
	"errors"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
)

type WebhookHandler struct {
	// Concrete service pointers (used by existing code)
	webhookService *application.WebhookService
	auditService   *application.AuditService

	// Interface fields for testability (used when set)
	webhookServicer WebhookServicer
	auditServicer   AuditServicer

	// jwtService reads the acting session's revocation before a webhook is
	// created or updated.
	jwtService *auth.JWTService
}

func NewWebhookHandler(
	webhookService *application.WebhookService,
	auditService *application.AuditService,
	jwtService *auth.JWTService,
) *WebhookHandler {
	return &WebhookHandler{
		webhookService: webhookService,
		auditService:   auditService,
		jwtService:     jwtService,
	}
}

// NewWebhookHandlerWithInterfaces creates a webhook handler with interface dependencies for testing
func NewWebhookHandlerWithInterfaces(
	webhookService WebhookServicer,
	auditService AuditServicer,
) *WebhookHandler {
	return &WebhookHandler{
		webhookServicer: webhookService,
		auditServicer:   auditService,
	}
}

// Helper methods to get the appropriate service (interface or concrete)
func (h *WebhookHandler) getWebhookService() WebhookServicer {
	if h.webhookServicer != nil {
		return h.webhookServicer
	}
	return h.webhookService
}

func (h *WebhookHandler) getAuditService() AuditServicer {
	if h.auditServicer != nil {
		return h.auditServicer
	}
	return h.auditService
}

// webhookURLRefusedMessage answers a webhook URL the server refuses. It names
// no host or address: the service's reason says why the URL was refused, and
// that stays in the server log.
const webhookURLRefusedMessage = "url must be a public http or https URL"

// respondWebhookServiceError answers a webhook create or update error. A URL
// the server refuses is a problem with the caller's input, so it is answered
// with 400 and a fixed reason; any other error is answered by
// respondServerError. Both keep the service's reason in the server log only.
func respondWebhookServiceError(c fiber.Ctx, err error) error {
	if errors.Is(err, application.ErrWebhookURLRefused) {
		log.Printf("webhook: URL refused: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": webhookURLRefusedMessage,
		})
	}
	return respondServerError(c, fiber.StatusInternalServerError, err)
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
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	userID := c.Locals("user_id").(uuid.UUID)

	// A revoked session cannot have the organization's events delivered to an address it chooses.
	if !refuseIfFamilyRevoked(c, h.jwtService, h.getAuditService(), "webhook_create") {
		return nil
	}

	var req application.CreateWebhookRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	webhook, err := h.getWebhookService().CreateWebhook(c.Context(), &req, orgID, userID)
	if err != nil {
		return respondWebhookServiceError(c, err)
	}

	// Log audit
	h.getAuditService().LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCreate,
		"webhook",
		webhook.ID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"webhookName": webhook.Name,
			"webhookUrl":  webhook.URL,
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
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	webhooks, err := h.getWebhookService().ListWebhooks(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch webhooks",
		})
	}

	// Ensure webhooks is never null (return empty array instead)
	if webhooks == nil {
		webhooks = []*domain.Webhook{}
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
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	webhookID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid webhook ID",
		})
	}

	webhook, err := h.getWebhookService().GetWebhook(c.Context(), webhookID)
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
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	userID := c.Locals("user_id").(uuid.UUID)
	webhookID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid webhook ID",
		})
	}

	// Verify webhook belongs to organization
	webhook, err := h.getWebhookService().GetWebhook(c.Context(), webhookID)
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

	if err := h.getWebhookService().DeleteWebhook(c.Context(), webhookID); err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}

	// Log audit
	h.getAuditService().LogAction(
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

// UpdateWebhook updates a webhook
// @Summary Update webhook
// @Description Update webhook settings
// @Tags webhooks
// @Accept json
// @Produce json
// @Param id path string true "Webhook ID"
// @Param request body application.CreateWebhookRequest true "Webhook details"
// @Success 200 {object} domain.Webhook
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/webhooks/{id} [put]
func (h *WebhookHandler) UpdateWebhook(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	userID := c.Locals("user_id").(uuid.UUID)

	// A revoked session cannot point a webhook's deliveries at a new address.
	if !refuseIfFamilyRevoked(c, h.jwtService, h.getAuditService(), "webhook_update") {
		return nil
	}

	webhookID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid webhook ID",
		})
	}

	// Verify webhook belongs to organization
	existingWebhook, err := h.getWebhookService().GetWebhook(c.Context(), webhookID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Webhook not found",
		})
	}
	if existingWebhook.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	var req application.CreateWebhookRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Update webhook
	webhook, err := h.getWebhookService().UpdateWebhook(c.Context(), webhookID, &req)
	if err != nil {
		return respondWebhookServiceError(c, err)
	}

	// Log audit
	h.getAuditService().LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"webhook",
		webhookID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"webhookName": webhook.Name,
			"isActive":    webhook.IsActive,
		},
	)

	return c.JSON(webhook)
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
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	userID := c.Locals("user_id").(uuid.UUID)
	webhookID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid webhook ID",
		})
	}

	// Verify webhook belongs to organization
	webhook, err := h.getWebhookService().GetWebhook(c.Context(), webhookID)
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

	// Send test payload
	result, err := h.getWebhookService().TestWebhook(c.Context(), webhookID)
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}

	// Log audit
	h.getAuditService().LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"webhook",
		webhookID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action":     "test",
			"statusCode": result.StatusCode,
			"success":    result.Success,
		},
	)

	// Always return 200 to frontend with the actual webhook response details
	message := fmt.Sprintf("Webhook responded with status %d", result.StatusCode)
	if !result.Success {
		// The answer says only whether the endpoint answered and with which
		// status; the delivery error stays in the server log.
		log.Printf("webhook %s: test delivery failed: %s", webhookID, result.ErrorMessage)
		if result.StatusCode != 0 {
			message = fmt.Sprintf("Webhook test failed: the endpoint responded with status %d", result.StatusCode)
		} else {
			message = "Webhook test failed: the endpoint could not be reached"
		}
	}

	return c.JSON(fiber.Map{
		"success":      result.Success,
		"message":      message,
		"responseCode": result.StatusCode,
		"webhook": fiber.Map{
			"id":   webhook.ID,
			"name": webhook.Name,
			"url":  webhook.URL,
		},
	})
}
