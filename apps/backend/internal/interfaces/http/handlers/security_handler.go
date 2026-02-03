package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type SecurityHandler struct {
	// Concrete service pointers (used by existing code)
	securityService   *application.SecurityService
	auditService      *application.AuditService
	alertService      *application.AlertService
	agentService      *application.AgentService
	capabilityService *application.CapabilityService

	// Interface fields for testability (used when set)
	securityServicer   SecurityServicerExtended
	auditServicer      AuditServicer
	alertServicer      AlertServicerExtended
	agentServicer      AgentServicer
	capabilityServicer CapabilityServicer
}

func NewSecurityHandler(
	securityService *application.SecurityService,
	auditService *application.AuditService,
	alertService *application.AlertService,
	agentService *application.AgentService,
	capabilityService *application.CapabilityService,
) *SecurityHandler {
	return &SecurityHandler{
		securityService:   securityService,
		auditService:      auditService,
		alertService:      alertService,
		agentService:      agentService,
		capabilityService: capabilityService,
	}
}

// NewSecurityHandlerWithInterfaces creates a SecurityHandler using interfaces for testability
func NewSecurityHandlerWithInterfaces(
	securityService SecurityServicerExtended,
	auditService AuditServicer,
	alertService AlertServicerExtended,
	agentService AgentServicer,
	capabilityService CapabilityServicer,
) *SecurityHandler {
	return &SecurityHandler{
		securityServicer:   securityService,
		auditServicer:      auditService,
		alertServicer:      alertService,
		agentServicer:      agentService,
		capabilityServicer: capabilityService,
	}
}

// Helper methods to use interfaces when available, otherwise use concrete types
func (h *SecurityHandler) getSecurityService() SecurityServicerExtended {
	if h.securityServicer != nil {
		return h.securityServicer
	}
	return h.securityService
}

func (h *SecurityHandler) getAlertService() AlertServicerExtended {
	if h.alertServicer != nil {
		return h.alertServicer
	}
	return h.alertService
}

func (h *SecurityHandler) getAgentService() AgentServicer {
	if h.agentServicer != nil {
		return h.agentServicer
	}
	return h.agentService
}

func (h *SecurityHandler) getCapabilityService() CapabilityServicer {
	if h.capabilityServicer != nil {
		return h.capabilityServicer
	}
	return h.capabilityService
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

	// SECURITY: Validate pagination to prevent DoS
	p := ParsePagination(c)

	secSvc := h.getSecurityService()
	threats, err := secSvc.GetThreats(c.Context(), orgID, p.Limit, p.Offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch security threats",
		})
	}

	return c.JSON(fiber.Map{
		"threats": threats,
		"total":   len(threats),
		"limit":   p.Limit,
		"offset":  p.Offset,
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

	// SECURITY: Validate pagination to prevent DoS
	p := ParsePagination(c)

	secSvc := h.getSecurityService()
	anomalies, err := secSvc.GetAnomalies(c.Context(), orgID, p.Limit, p.Offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch anomalies",
		})
	}

	return c.JSON(fiber.Map{
		"anomalies": anomalies,
		"total":     len(anomalies),
		"limit":     p.Limit,
		"offset":    p.Offset,
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

	secSvc := h.getSecurityService()
	metrics, err := secSvc.GetSecurityMetrics(c.Context(), orgID)
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

	secSvc := h.getSecurityService()
	alertSvc := h.getAlertService()
	agentSvc := h.getAgentService()

	// Get security metrics
	metrics, err := secSvc.GetSecurityMetrics(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch security metrics",
		})
	}

	// Get recent threats (limit 10)
	threats, err := secSvc.GetThreats(c.Context(), orgID, 10, 0)
	if err != nil || threats == nil {
		threats = make([]*domain.Threat, 0)
	}

	// Get recent anomalies (limit 10)
	anomalies, err := secSvc.GetAnomalies(c.Context(), orgID, 10, 0)
	if err != nil || anomalies == nil {
		anomalies = make([]*domain.Anomaly, 0)
	}

	// Get unacknowledged alerts count
	_, _, unacknowledgedAlerts, err := alertSvc.CountUnacknowledged(c.Context(), orgID)
	if err != nil {
		unacknowledgedAlerts = 0
	}

	// Get recent alerts (limit 5)
	recentAlerts, _, err := alertSvc.GetAlerts(c.Context(), orgID, "", "", 5, 0)
	if err != nil || recentAlerts == nil {
		recentAlerts = make([]*domain.Alert, 0)
	}

	// Get agent security status
	agents, err := agentSvc.ListAgents(c.Context(), orgID)
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

		// Trust score is stored as 0-1 (e.g., 0.50 = 50%)
		if agent.TrustScore < 0.50 {
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
			"lowTrust":  lowTrustAgents,
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

	// SECURITY: Validate pagination to prevent DoS
	p := ParsePaginationWithDefaults(c, 20, 100)

	alertSvc := h.getAlertService()
	alerts, total, err := alertSvc.GetAlerts(c.Context(), orgID, "", "", p.Limit, p.Offset)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch security alerts",
		})
	}

	// Get alert counts (all, acknowledged, unacknowledged)
	allCount, acknowledgedCount, unacknowledgedCount, err := alertSvc.CountUnacknowledged(c.Context(), orgID)
	if err != nil {
		// If count fails, set defaults but don't fail the request
		allCount = total
		acknowledgedCount = 0
		unacknowledgedCount = 0
	}

	return c.JSON(fiber.Map{
		"alerts":              alerts,
		"total":               total,
		"allCount":            allCount,
		"acknowledgedCount":   acknowledgedCount,
		"unacknowledgedCount": unacknowledgedCount,
		"limit":               p.Limit,
		"offset":              p.Offset,
	})
}

// GetViolations retrieves capability violations (blocked actions)
// @Summary List capability violations
// @Description Get all capability violations (blocked actions) for the organization
// @Tags security
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Param blocked query bool false "Filter by blocked status"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/security/violations [get]
func (h *SecurityHandler) GetViolations(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	// SECURITY: Validate pagination to prevent DoS
	p := ParsePagination(c)

	capSvc := h.getCapabilityService()
	violations, total, err := capSvc.GetViolationsByOrganization(
		c.Context(),
		orgID,
		p.Limit,
		p.Offset,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch violations",
		})
	}

	return c.JSON(fiber.Map{
		"violations": violations,
		"total":      total,
		"limit":      p.Limit,
		"offset":     p.Offset,
	})
}
