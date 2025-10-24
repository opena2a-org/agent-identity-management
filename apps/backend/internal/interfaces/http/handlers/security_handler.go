package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
)

type SecurityHandler struct {
	securityService *application.SecurityService
	auditService    *application.AuditService
}

func NewSecurityHandler(
	securityService *application.SecurityService,
	auditService *application.AuditService,
) *SecurityHandler {
	return &SecurityHandler{
		securityService: securityService,
		auditService:    auditService,
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
