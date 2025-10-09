package handlers

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

// DetectionHandler handles detection-related HTTP requests
type DetectionHandler struct {
	detectionService *application.DetectionService
	auditService     *application.AuditService
}

// NewDetectionHandler creates a new detection handler
func NewDetectionHandler(
	detectionService *application.DetectionService,
	auditService *application.AuditService,
) *DetectionHandler {
	return &DetectionHandler{
		detectionService: detectionService,
		auditService:     auditService,
	}
}

// ReportDetection handles detection reports from SDKs or Direct API calls
// POST /api/v1/agents/:id/detection/report
// @Summary Report MCP detections
// @Description Report detected MCP servers from SDK or Direct API
// @Tags detection
// @Accept json
// @Produce json
// @Param id path string true "Agent ID"
// @Param request body domain.DetectionReportRequest true "Detection events"
// @Success 200 {object} domain.DetectionReportResponse
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /agents/{id}/detection/report [post]
func (h *DetectionHandler) ReportDetection(c fiber.Ctx) error {
	// Get agent ID from URL
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Get organization ID from auth context
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	userID, ok := c.Locals("user_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Parse request body
	var req domain.DetectionReportRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate request
	if len(req.Detections) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "detections array is required and must not be empty",
		})
	}

	// Process detections
	response, err := h.detectionService.ReportDetections(
		c.Context(), agentID, orgID, &req)

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
		"mcp_detection",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"detections_processed": response.DetectionsProcessed,
			"new_mcps":            len(response.NewMCPs),
			"existing_mcps":       len(response.ExistingMCPs),
		},
	)

	return c.Status(fiber.StatusOK).JSON(response)
}

// GetDetectionStatus returns the current detection status for an agent
// GET /api/v1/agents/:id/detection/status
// @Summary Get detection status
// @Description Get current MCP detection status for an agent including SDK installation and detected MCPs
// @Tags detection
// @Produce json
// @Param id path string true "Agent ID"
// @Success 200 {object} domain.DetectionStatusResponse
// @Failure 400 {object} ErrorResponse "Invalid agent ID"
// @Failure 403 {object} ErrorResponse "Access denied"
// @Failure 404 {object} ErrorResponse "Agent not found"
// @Router /agents/{id}/detection/status [get]
func (h *DetectionHandler) GetDetectionStatus(c fiber.Ctx) error {
	// Get agent ID from URL
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Get organization ID from auth context
	orgID, ok := c.Locals("organization_id").(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	}

	// Get detection status
	status, err := h.detectionService.GetDetectionStatus(c.Context(), agentID, orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(status)
}
