package handlers

import (
	"context"
	"errors"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// onboardingTelemetryService is what the handler needs from
// application.OnboardingTelemetryService.
type onboardingTelemetryService interface {
	RecordClientEvent(ctx context.Context, orgID uuid.UUID, event domain.OnboardingEventType, tab string) error
	Baseline(ctx context.Context) (*application.OnboardingBaseline, error)
}

// OnboardingTelemetryHandler serves onboarding event recording and the
// platform onboarding baseline.
type OnboardingTelemetryHandler struct {
	service onboardingTelemetryService
}

// NewOnboardingTelemetryHandler creates an OnboardingTelemetryHandler.
func NewOnboardingTelemetryHandler(service onboardingTelemetryService) *OnboardingTelemetryHandler {
	return &OnboardingTelemetryHandler{service: service}
}

// RecordOnboardingEventRequest is the body of an event report. Tab is set
// only for tab_selected.
type RecordOnboardingEventRequest struct {
	Event string `json:"event"`
	Tab   string `json:"tab"`
}

// RecordEvent records an onboarding step the dashboard reports for the
// caller's organization. Only the organization and the time are stored with
// it; the caller's identity is not. An event the organization has already
// reported as often as the daily cap allows is accepted and not stored
// ("recorded": false).
// @Summary Record an onboarding event for the caller's organization
// @Tags onboarding
// @Accept json
// @Produce json
// @Router /onboarding/events [post]
func (h *OnboardingTelemetryHandler) RecordEvent(c fiber.Ctx) error {
	orgID, _, err := RequireOrgAndUserID(c)
	if err != nil {
		return nil // the 401 response has been written
	}
	var req RecordOnboardingEventRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	err = h.service.RecordClientEvent(c.Context(), orgID, domain.OnboardingEventType(req.Event), req.Tab)
	switch {
	case err == nil:
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"recorded": true})
	case errors.Is(err, application.ErrOnboardingEventCapped):
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"recorded": false})
	case errors.Is(err, application.ErrOnboardingEventUnknown),
		errors.Is(err, application.ErrOnboardingEventServerOnly),
		errors.Is(err, application.ErrOnboardingTabInvalid):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to record onboarding event"})
	}
}

// GetBaseline returns time to first agent across every organization and the
// onboarding events of the recent window. Platform admins only.
// @Summary Onboarding baseline: time to first agent and onboarding events
// @Tags platform-admin
// @Produce json
// @Router /platform-admin/onboarding-metrics [get]
func (h *OnboardingTelemetryHandler) GetBaseline(c fiber.Ctx) error {
	baseline, err := h.service.Baseline(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to compute onboarding metrics"})
	}
	return c.JSON(baseline)
}
