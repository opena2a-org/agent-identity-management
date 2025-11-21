package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type SecurityHandler struct {
	securityService *application.SecurityService
	auditService    *application.AuditService
	alertService    *application.AlertService
	agentService    *application.AgentService
}

func NewSecurityHandler(
	securityService *application.SecurityService,
	auditService *application.AuditService,
	alertService *application.AlertService,
	agentService *application.AgentService,
) *SecurityHandler {
	return &SecurityHandler{
		securityService: securityService,
		auditService:    auditService,
		alertService:    alertService,
		agentService:    agentService,
	}
}

// GetThreats retrieves detected security threats
// @Summary List security threats
// @Description Get all detected security threats for the organization
// @Tags security
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/security/threats [get]
func (h *SecurityHandler) GetThreats(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	threats, err := h.securityService.GetThreats(c.Context(), orgID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch security threats",
		})
	}

	return c.JSON(fiber.Map{
		"threats": threats,
		"total":   len(threats),
		"limit":   limit,
		"offset":  offset,
	})
}

// GetAnomalies retrieves detected anomalies
// @Summary List anomalies
// @Description Get all detected anomalies for the organization
// @Tags security
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/security/anomalies [get]
func (h *SecurityHandler) GetAnomalies(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	anomalies, err := h.securityService.GetAnomalies(c.Context(), orgID, limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch anomalies",
		})
	}

	return c.JSON(fiber.Map{
		"anomalies": anomalies,
		"total":     len(anomalies),
		"limit":     limit,
		"offset":    offset,
	})
}

// GetSecurityMetrics retrieves overall security metrics
// @Summary Get security metrics
// @Description Get overall security metrics for the organization
// @Tags security
// @Produce json
// @Success 200 {object} domain.SecurityMetrics
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/security/metrics [get]
func (h *SecurityHandler) GetSecurityMetrics(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	metrics, err := h.securityService.GetSecurityMetrics(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch security metrics",
		})
	}

	return c.JSON(metrics)
}

// GetSecurityDashboard retrieves comprehensive security dashboard data
// @Summary Get security dashboard
// @Description Get comprehensive security dashboard data including threats, alerts, and metrics
// @Tags security
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/security/dashboard [get]
func (h *SecurityHandler) GetSecurityDashboard(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	// Get security metrics
	metrics, err := h.securityService.GetSecurityMetrics(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch security metrics",
		})
	}

	// Get recent threats (limit 10)
	threats, err := h.securityService.GetThreats(c.Context(), orgID, 10, 0)
	if err != nil || threats == nil {
		threats = make([]*domain.Threat, 0)
	}

	// Get recent anomalies (limit 10)
	anomalies, err := h.securityService.GetAnomalies(c.Context(), orgID, 10, 0)
	if err != nil || anomalies == nil {
		anomalies = make([]*domain.Anomaly, 0)
	}

	// Get unacknowledged alerts count
	_, _, unacknowledgedAlerts, err := h.alertService.CountUnacknowledged(c.Context(), orgID)
	if err != nil {
		unacknowledgedAlerts = 0
	}

	// Get recent alerts (limit 5)
	recentAlerts, _, err := h.alertService.GetAlerts(c.Context(), orgID, "", "", 5, 0)
	if err != nil || recentAlerts == nil {
		recentAlerts = make([]*domain.Alert, 0)
	}

	// Get agent security status
	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents",
		})
	}

	// Calculate agent security status
	verifiedAgents := 0
	suspendedAgents := 0
	pendingAgents := 0
	lowTrustAgents := 0

	for _, agent := range agents {
		switch agent.Status {
		case "verified":
			verifiedAgents++
		case "suspended":
			suspendedAgents++
		case "pending":
			pendingAgents++
		}

		if agent.TrustScore < 50.0 {
			lowTrustAgents++
		}
	}

	return c.JSON(fiber.Map{
		"metrics": metrics,
		"threats": fiber.Map{
			"recent": threats,
			"total":  len(threats),
		},
		"anomalies": fiber.Map{
			"recent": anomalies,
			"total":  len(anomalies),
		},
		"alerts": fiber.Map{
			"recent":         recentAlerts,
			"unacknowledged": unacknowledgedAlerts,
		},
		"agents": fiber.Map{
			"total":      len(agents),
			"verified":   verifiedAgents,
			"suspended":  suspendedAgents,
			"pending":    pendingAgents,
			"low_trust":  lowTrustAgents,
		},
	})
}

// ListSecurityAlerts retrieves security alerts
// @Summary List security alerts
// @Description Get all security alerts for the organization
// @Tags security
// @Produce json
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/security/alerts [get]
func (h *SecurityHandler) ListSecurityAlerts(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	alerts, total, err := h.alertService.GetAlerts(c.Context(), orgID, "", "", limit, offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch security alerts",
		})
	}

	// Get alert counts (all, acknowledged, unacknowledged)
	allCount, acknowledgedCount, unacknowledgedCount, err := h.alertService.CountUnacknowledged(c.Context(), orgID)
	if err != nil {
		// If count fails, set defaults but don't fail the request
		allCount = total
		acknowledgedCount = 0
		unacknowledgedCount = 0
	}

	return c.JSON(fiber.Map{
		"alerts":              alerts,
		"total":               total,
		"all_count":           allCount,
		"acknowledged_count":  acknowledgedCount,
		"unacknowledged_count": unacknowledgedCount,
		"limit":               limit,
		"offset":              offset,
	})
}
