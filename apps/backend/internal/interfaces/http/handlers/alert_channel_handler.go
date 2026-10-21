package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/auth"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/interfaces/http/middleware"
)

// AlertChannelServicer is what AlertChannelHandler needs of the alert channel
// service. Every method names the caller's organization.
type AlertChannelServicer interface {
	CreateAlertChannel(ctx context.Context, orgID, userID uuid.UUID, req *application.AlertChannelRequest) (*domain.AlertChannel, error)
	GetAlertChannel(ctx context.Context, orgID, id uuid.UUID) (*domain.AlertChannel, error)
	ListAlertChannels(ctx context.Context, orgID uuid.UUID) ([]*domain.AlertChannel, error)
	UpdateAlertChannel(ctx context.Context, orgID, id uuid.UUID, req *application.AlertChannelRequest) (*domain.AlertChannel, error)
	DeleteAlertChannel(ctx context.Context, orgID, id uuid.UUID) error
	SendTestMessage(ctx context.Context, orgID, id uuid.UUID) (*application.AlertChannelTestResult, error)
}

// AlertChannelHandler serves /api/v1/alert-channels. Answers carry each
// channel with its destination masked to the last four characters; an email
// channel's recipients, which are not credentials, are answered as stored.
type AlertChannelHandler struct {
	service AlertChannelServicer
	audit   AuditServicer
	// jwtService reads the acting session's revocation before a channel is
	// created or updated.
	jwtService *auth.JWTService
}

func NewAlertChannelHandler(service AlertChannelServicer, audit AuditServicer, jwtService *auth.JWTService) *AlertChannelHandler {
	return &AlertChannelHandler{service: service, audit: audit, jwtService: jwtService}
}

// decodeAlertChannelBody decodes a create or update body into req. A field
// the body may not carry is refused by name rather than dropped; the
// service refuses an unknown config field the same way. It returns the reason
// for a 400, or "" when req holds the body.
func decodeAlertChannelBody(body []byte, req *application.AlertChannelRequest) string {
	if trimmed := bytes.TrimSpace(body); len(trimmed) == 0 || trimmed[0] != '{' {
		return "Invalid request body"
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(req); err != nil {
		if name, ok := application.UnknownJSONField(err); ok {
			return "unknown field: " + name
		}
		return "Invalid request body"
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return "Invalid request body"
	}
	return ""
}

// respondAlertChannelError answers a service error: a refused request is 400
// with its reason, a channel that is not the caller's organization's is 404,
// a name in use is 409, and anything else is respondServerError.
func respondAlertChannelError(c fiber.Ctx, err error) error {
	var invalid *application.AlertChannelRequestError
	switch {
	case errors.As(err, &invalid):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": invalid.Reason})
	case errors.Is(err, domain.ErrAlertChannelNotFound):
		respondResourceNotFound(c)
		return nil
	case errors.Is(err, domain.ErrAlertChannelNameTaken):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "name is used by another alert channel of the organization"})
	}
	return respondServerError(c, fiber.StatusInternalServerError, err)
}

// loadChannel returns the caller's organization's channel the :id parameter
// names. Otherwise it answers, 400 for an id that is not a UUID and 404 for a
// channel the organization does not have, and returns nil.
func (h *AlertChannelHandler) loadChannel(c fiber.Ctx, orgID uuid.UUID) *domain.AlertChannel {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid alert channel id"})
		return nil
	}
	return LoadOwned(c, func(id uuid.UUID) (*domain.AlertChannel, error) {
		return h.service.GetAlertChannel(c.Context(), orgID, id)
	}, id, orgID, func(channel *domain.AlertChannel) uuid.UUID { return channel.OrganizationID })
}

// logAudit writes the channel's audit row: its kind and floor, never its
// destination.
func (h *AlertChannelHandler) logAudit(c fiber.Ctx, orgID uuid.UUID, action domain.AuditAction, channel *domain.AlertChannel) {
	if h.audit == nil {
		return
	}
	userID, _ := c.Locals("user_id").(uuid.UUID)
	if err := h.audit.LogAction(c.Context(), orgID, userID, action, "alert_channel", channel.ID, c.IP(), c.Get("User-Agent"),
		map[string]interface{}{
			"kind":        string(channel.Kind),
			"minSeverity": string(channel.MinSeverity),
		}); err != nil {
		log.Printf("alert channel %s: the %s audit row was not written: %v", channel.ID, action, err)
	}
}

// CreateAlertChannel creates an alert channel
// @Summary Create alert channel
// @Description Send the organization's alerts of a severity floor or above to Slack, PagerDuty, a Microsoft Teams workflow or email addresses
// @Tags alert-channels
// @Accept json
// @Produce json
// @Param request body application.AlertChannelRequest true "Alert channel"
// @Success 201 {object} domain.AlertChannelView
// @Failure 400 {object} map[string]interface{}
// @Router /api/v1/alert-channels [post]
func (h *AlertChannelHandler) CreateAlertChannel(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	userID, _ := c.Locals("user_id").(uuid.UUID)

	// A revoked session cannot have the organization's alerts sent to a destination it chooses.
	if !refuseIfFamilyRevoked(c, h.jwtService, h.audit, middleware.CredentialRouteAlertChannelCreate) {
		return nil
	}

	var req application.AlertChannelRequest
	if reason := decodeAlertChannelBody(c.Body(), &req); reason != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": reason})
	}
	channel, err := h.service.CreateAlertChannel(c.Context(), orgID, userID, &req)
	if err != nil {
		return respondAlertChannelError(c, err)
	}
	h.logAudit(c, orgID, domain.AuditActionCreate, channel)
	return c.Status(fiber.StatusCreated).JSON(channel.View())
}

// ListAlertChannels lists the organization's alert channels
// @Summary List alert channels
// @Tags alert-channels
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/alert-channels [get]
func (h *AlertChannelHandler) ListAlertChannels(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	channels, err := h.service.ListAlertChannels(c.Context(), orgID)
	if err != nil {
		return respondServerError(c, fiber.StatusInternalServerError, err)
	}
	views := make([]domain.AlertChannelView, 0, len(channels))
	for _, channel := range channels {
		views = append(views, channel.View())
	}
	return c.JSON(fiber.Map{"alertChannels": views, "total": len(views)})
}

// GetAlertChannel returns one of the organization's alert channels
// @Summary Get alert channel
// @Tags alert-channels
// @Produce json
// @Param id path string true "Alert channel ID"
// @Success 200 {object} domain.AlertChannelView
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/alert-channels/{id} [get]
func (h *AlertChannelHandler) GetAlertChannel(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	channel := h.loadChannel(c, orgID)
	if channel == nil {
		return nil
	}
	return c.JSON(channel.View())
}

// UpdateAlertChannel changes one of the organization's alert channels
// @Summary Update alert channel
// @Description Every field is optional; an omitted config keeps the stored destination
// @Tags alert-channels
// @Accept json
// @Produce json
// @Param id path string true "Alert channel ID"
// @Param request body application.AlertChannelRequest true "Fields to change"
// @Success 200 {object} domain.AlertChannelView
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/alert-channels/{id} [put]
func (h *AlertChannelHandler) UpdateAlertChannel(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	existing := h.loadChannel(c, orgID)
	if existing == nil {
		return nil
	}

	// A revoked session cannot have the organization's alerts sent to a destination it chooses.
	if !refuseIfFamilyRevoked(c, h.jwtService, h.audit, middleware.CredentialRouteAlertChannelUpdate) {
		return nil
	}

	var req application.AlertChannelRequest
	if reason := decodeAlertChannelBody(c.Body(), &req); reason != "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": reason})
	}
	channel, err := h.service.UpdateAlertChannel(c.Context(), orgID, existing.ID, &req)
	if err != nil {
		return respondAlertChannelError(c, err)
	}
	h.logAudit(c, orgID, domain.AuditActionUpdate, channel)
	return c.JSON(channel.View())
}

// DeleteAlertChannel deletes one of the organization's alert channels
// @Summary Delete alert channel
// @Tags alert-channels
// @Param id path string true "Alert channel ID"
// @Success 204
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/alert-channels/{id} [delete]
func (h *AlertChannelHandler) DeleteAlertChannel(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	channel := h.loadChannel(c, orgID)
	if channel == nil {
		return nil
	}
	if err := h.service.DeleteAlertChannel(c.Context(), orgID, channel.ID); err != nil {
		return respondAlertChannelError(c, err)
	}
	h.logAudit(c, orgID, domain.AuditActionDelete, channel)
	return c.SendStatus(fiber.StatusNoContent)
}

// TestAlertChannel sends one test message through an alert channel
// @Summary Send a test message through an alert channel
// @Description One attempt, no retry. A PagerDuty test triggers an incident on the PagerDuty service; an email test is sent to each recipient.
// @Tags alert-channels
// @Produce json
// @Param id path string true "Alert channel ID"
// @Success 200 {object} map[string]interface{}
// @Failure 502 {object} map[string]interface{}
// @Router /api/v1/alert-channels/{id}/test [post]
func (h *AlertChannelHandler) TestAlertChannel(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}
	channel := h.loadChannel(c, orgID)
	if channel == nil {
		return nil
	}
	result, err := h.service.SendTestMessage(c.Context(), orgID, channel.ID)
	if err != nil {
		return respondAlertChannelError(c, err)
	}
	if result.Delivered {
		return c.JSON(fiber.Map{"delivered": true, "statusCode": result.StatusCode})
	}
	return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
		"delivered":  false,
		"statusCode": result.StatusCode,
		"error":      result.Error,
	})
}
