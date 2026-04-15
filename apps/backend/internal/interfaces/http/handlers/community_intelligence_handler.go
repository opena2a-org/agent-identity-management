package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

// CommunityIntelligenceHandler handles community intelligence opt-in endpoints
type CommunityIntelligenceHandler struct {
	ciService *application.CommunityIntelligenceService
}

// NewCommunityIntelligenceHandler creates a new handler for community intelligence operations
func NewCommunityIntelligenceHandler(ciService *application.CommunityIntelligenceService) *CommunityIntelligenceHandler {
	return &CommunityIntelligenceHandler{ciService: ciService}
}

// EnableOptIn enables community intelligence telemetry for the organization.
// POST /api/v1/community-intelligence/enable
func (h *CommunityIntelligenceHandler) EnableOptIn(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	if err := h.ciService.EnableOptIn(c.Context(), orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to enable community intelligence",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Community intelligence enabled",
		"status":  "enabled",
	})
}

// DisableOptIn disables community intelligence telemetry for the organization.
// POST /api/v1/community-intelligence/disable
func (h *CommunityIntelligenceHandler) DisableOptIn(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	if err := h.ciService.DisableOptIn(c.Context(), orgID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to disable community intelligence",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Community intelligence disabled",
		"status":  "disabled",
	})
}

// GetOptInStatus returns the current opt-in status.
// GET /api/v1/community-intelligence/status
func (h *CommunityIntelligenceHandler) GetOptInStatus(c fiber.Ctx) error {
	orgID, err := RequireOrganizationID(c)
	if err != nil {
		return err
	}

	enabled, consentedAt, err := h.ciService.GetOptInStatus(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get opt-in status",
		})
	}

	return c.JSON(fiber.Map{
		"enabled":     enabled,
		"consentedAt": consentedAt,
	})
}

// GetCommunityBenchmarks returns aggregated community benchmarks.
// Available when 10+ agents have opted in across all orgs.
// GET /api/v1/community-intelligence/benchmarks
func (h *CommunityIntelligenceHandler) GetCommunityBenchmarks(c fiber.Ctx) error {
	benchmarks, err := h.ciService.GetCommunityBenchmarks(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get community benchmarks",
		})
	}

	if !benchmarks.SufficientData {
		return c.JSON(fiber.Map{
			"message":        "Insufficient data for community benchmarks",
			"sufficientData": false,
			"totalAgents":    benchmarks.TotalAgentsReported,
			"threshold":      10,
		})
	}

	return c.JSON(benchmarks)
}

// TriggerPush manually triggers a community intelligence data push (admin only).
// POST /api/v1/admin/community-intelligence/push
func (h *CommunityIntelligenceHandler) TriggerPush(c fiber.Ctx) error {
	if err := h.ciService.CollectAndPushTelemetry(c.Context()); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to push community intelligence data",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Community intelligence data pushed to registry",
	})
}
