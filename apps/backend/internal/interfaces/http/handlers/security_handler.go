package handlers

import (
	"log"
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
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

// RunSecurityScan initiates a security scan
// @Summary Run security scan
// @Description Initiate a comprehensive security scan
// @Tags security
// @Produce json
// @Param id path string false "Specific resource ID to scan"
// @Success 202 {object} domain.SecurityScanResult
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/security/scan/{id} [get]
func (h *SecurityHandler) RunSecurityScan(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)

	scanType := c.Query("scan_type", "full")

	scan, err := h.securityService.RunSecurityScan(c.Context(), orgID, scanType)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to initiate security scan",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCreate,
		"security_scan",
		scan.ScanID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"scan_type": scanType,
		},
	)

	return c.Status(fiber.StatusAccepted).JSON(scan)
}

// GetIncidents retrieves security incidents
// @Summary List security incidents
// @Description Get all security incidents for the organization
// @Tags security
// @Produce json
// @Param status query string false "Filter by status (open, investigating, resolved)"
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /api/v1/security/incidents [get]
func (h *SecurityHandler) GetIncidents(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	statusFilter := c.Query("status", "")
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	var status domain.IncidentStatus
	if statusFilter != "" {
		status = domain.IncidentStatus(statusFilter)
	}

	incidents, err := h.securityService.GetIncidents(c.Context(), orgID, status, limit, offset)
	if err != nil {
		// Log the actual error for debugging
		log.Printf("ERROR fetching incidents for org %s: %v", orgID, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch security incidents",
		})
	}

	return c.JSON(fiber.Map{
		"incidents": incidents,
		"total":     len(incidents),
		"limit":     limit,
		"offset":    offset,
	})
}

// ResolveIncident marks a security incident as resolved
// @Summary Resolve security incident
// @Description Mark a security incident as resolved
// @Tags security
// @Accept json
// @Produce json
// @Param id path string true "Incident ID"
// @Param request body map[string]string true "Resolution notes"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 404 {object} map[string]interface{}
// @Router /api/v1/security/incidents/{id}/resolve [post]
func (h *SecurityHandler) ResolveIncident(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	incidentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid incident ID",
		})
	}

	var req struct {
		Notes string `json:"notes"`
	}
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.securityService.ResolveIncident(c.Context(), incidentID, userID, req.Notes); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionUpdate,
		"security_incident",
		incidentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"action": "resolve",
			"notes":  req.Notes,
		},
	)

	return c.JSON(fiber.Map{
		"message":     "Incident resolved successfully",
		"incident_id": incidentID,
	})
}
