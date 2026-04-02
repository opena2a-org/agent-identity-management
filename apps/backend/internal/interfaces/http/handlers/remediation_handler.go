package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
)

type RemediationHandler struct {
	service *application.RemediationService
}

func NewRemediationHandler(service *application.RemediationService) *RemediationHandler {
	return &RemediationHandler{service: service}
}

// RecordFinding records a new security finding for remediation tracking
func (h *RemediationHandler) RecordFinding(c fiber.Ctx) error {
	var req application.RecordFindingRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.FindingID == "" || req.Source == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "findingId and source are required",
		})
	}

	record, err := h.service.RecordFinding(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to record finding",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(record)
}

// RecordRemediation updates a finding with remediation results
func (h *RemediationHandler) RecordRemediation(c fiber.Ctx) error {
	var req application.RecordRemediationRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.FindingID == "" || req.AgentIdentifier == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "findingId and agentIdentifier are required",
		})
	}

	record, err := h.service.RecordRemediation(c.Context(), req)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Finding not found or remediation update failed",
		})
	}

	return c.JSON(record)
}

// GetStats returns aggregate remediation statistics
func (h *RemediationHandler) GetStats(c fiber.Ctx) error {
	stats, err := h.service.GetStats(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get remediation stats",
		})
	}

	return c.JSON(stats)
}

// ListRecent returns recent remediation records
func (h *RemediationHandler) ListRecent(c fiber.Ctx) error {
	limit, err := strconv.Atoi(c.Query("limit", "50"))
	if err != nil || limit <= 0 {
		limit = 50
	}

	records, err := h.service.ListRecent(c.Context(), limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to list remediation records",
		})
	}

	return c.JSON(fiber.Map{
		"records": records,
		"total":   len(records),
	})
}
