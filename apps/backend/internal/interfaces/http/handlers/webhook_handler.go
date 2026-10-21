package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
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
// with 400 and a fixed reason, and the service's reason stays in the server
// log. An event or secret the service refuses is answered with 400 and the
// service's reason, which names the event or the secret's length. Any other
// error is answered by respondServerError.
func respondWebhookServiceError(c fiber.Ctx, err error) error {
	if errors.Is(err, application.ErrWebhookURLRefused) {
		log.Printf("webhook: URL refused: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": webhookURLRefusedMessage,
		})
	}
	var invalid *application.WebhookRequestError
	if errors.As(err, &invalid) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": invalid.Reason,
		})
	}
	return respondServerError(c, fiber.StatusInternalServerError, err)
}

// webhookCreateFields are the keys a webhook create body may carry: the JSON
// names of application.CreateWebhookRequest. webhookUpdateFields are the same
// without secret, which only rotate-secret changes.
var (
	webhookCreateFields = jsonFieldNames(reflect.TypeOf(application.CreateWebhookRequest{}))
	webhookUpdateFields = withoutField(webhookCreateFields, "secret")
)

// jsonFieldNames returns the JSON names of a struct type's fields.
func jsonFieldNames(t reflect.Type) map[string]bool {
	names := make(map[string]bool, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		name, _, _ := strings.Cut(t.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			names[name] = true
		}
	}
	return names
}

func withoutField(fields map[string]bool, field string) map[string]bool {
	out := make(map[string]bool, len(fields))
	for name := range fields {
		if name != field {
			out[name] = true
		}
	}
	return out
}

// decodeWebhookBody decodes a webhook create or update body into req. A key
// that is not one of fields is refused by name rather than dropped, so a
// setting the server does not store is never accepted without a word. It
// returns the reason for a 400, or "" when req holds the body.
func decodeWebhookBody(body []byte, fields map[string]bool, req *application.CreateWebhookRequest) string {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(body, &keys); err != nil || keys == nil {
		return "Invalid request body"
	}
	unknown := make([]string, 0)
	for key := range keys {
		if !fields[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) == 1 {
		return "unknown field: " + unknown[0]
	}
	if len(unknown) > 1 {
		sort.Strings(unknown)
		return "unknown fields: " + strings.Join(unknown, ", ")
	}
	if err := json.Unmarshal(body, req); err != nil {
		return "Invalid request body"
	}
	return ""
}

// WebhookCreatedResponse is the answer to a webhook create: the webhook and,
// this once, the secret its deliveries are signed with. No other answer
// carries the secret.
type WebhookCreatedResponse struct {
	*domain.Webhook
	Secret string `json:"secret"`
}

// WebhookSecretResponse is the answer to a secret rotation: the webhook's id
// and, this once, its new secret.
type WebhookSecretResponse struct {
	ID     uuid.UUID `json:"id"`
	Secret string    `json:"secret"`
}

// CreateWebhook creates a new webhook subscription
// @Summary Create webhook
// @Description Create a new webhook subscription for event notifications
// @Tags webhooks
// @Accept json
// @Produce json
// @Param request body application.CreateWebhookRequest true "Webhook details"
// @Success 201 {object} WebhookCreatedResponse
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/webhooks [post]
func (h *WebhookHandler) CreateWebhook(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	userID := c.Locals("user_id").(uuid.UUID)

	// A revoked session cannot have the organization's events delivered to an address it chooses.
	if !refuseIfFamilyRevoked(c, h.jwtService, h.getAuditService(), middleware.CredentialRouteWebhookCreate) {
		return nil
	}

	var req application.CreateWebhookRequest
	if reason := decodeWebhookBody(c.Body(), webhookCreateFields, &req); reason != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": reason,
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

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.Status(fiber.StatusCreated).JSON(WebhookCreatedResponse{Webhook: webhook, Secret: webhook.Secret})
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
	if !refuseIfFamilyRevoked(c, h.jwtService, h.getAuditService(), middleware.CredentialRouteWebhookUpdate) {
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
	if reason := decodeWebhookBody(c.Body(), webhookUpdateFields, &req); reason != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": reason,
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

// RotateWebhookSecret replaces a webhook's signing secret
// @Summary Rotate webhook secret
// @Description Replace the webhook's signing secret with a new generated one, returned once. Deliveries queued after it are signed with the new secret only.
// @Tags webhooks
// @Produce json
// @Param id path string true "Webhook ID"
// @Success 200 {object} WebhookSecretResponse
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/webhooks/{id}/rotate-secret [post]
func (h *WebhookHandler) RotateWebhookSecret(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	userID := c.Locals("user_id").(uuid.UUID)

	// A revoked session cannot obtain a secret that signs the organization's events.
	if !refuseIfFamilyRevoked(c, h.jwtService, h.getAuditService(), middleware.CredentialRouteWebhookRotateSecret) {
		return nil
	}

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

	secret, err := h.getWebhookService().RotateWebhookSecret(c.Context(), webhookID, orgID)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Webhook not found",
		})
	}
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}

	// Log audit. The secret is never written to the audit log.
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
			"webhookName":   webhook.Name,
			"rotatedSecret": true,
		},
	)

	c.Set(fiber.HeaderCacheControl, "no-store")
	return c.JSON(WebhookSecretResponse{ID: webhookID, Secret: secret})
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
