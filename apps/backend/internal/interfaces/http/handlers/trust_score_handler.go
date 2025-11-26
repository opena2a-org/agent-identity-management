package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type TrustScoreHandler struct {
	trustCalculator *application.TrustCalculator
	agentService    *application.AgentService
	auditService    *application.AuditService
}

func NewTrustScoreHandler(
	trustCalculator *application.TrustCalculator,
	agentService *application.AgentService,
	auditService *application.AuditService,
) *TrustScoreHandler {
	return &TrustScoreHandler{
		trustCalculator: trustCalculator,
		agentService:    agentService,
		auditService:    auditService,
	}
}

// CalculateTrustScore recalculates trust score for an agent
func (h *TrustScoreHandler) CalculateTrustScore(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Calculate trust score
	score, err := h.trustCalculator.CalculateTrustScore(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to calculate trust score",
		})
	}

	// Log audit
	h.auditService.LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCalculate,
		"trust_score",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName":  agent.Name,
			"trustScore": score.Score,
			"factors":     score.Factors,
		},
	)

	return c.JSON(fiber.Map{
		"agentId":      agentID,
		"score":         score.Score,
		"factors":       score.Factors,
		"calculated_at": score.LastCalculated,
	})
}

// GetTrustScore returns current trust score for an agent
func (h *TrustScoreHandler) GetTrustScore(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Get latest trust score
	score, err := h.trustCalculator.GetLatestTrustScore(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "No trust score found",
		})
	}

	return c.JSON(fiber.Map{
		"agentId":      agentID,
		"agentName":    agent.Name,
		"score":         score.Score,
		"factors":       score.Factors,
		"calculated_at": score.LastCalculated,
	})
}

// GetTrustScoreBreakdown returns detailed trust score breakdown with weights and contributions
func (h *TrustScoreHandler) GetTrustScoreBreakdown(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Get latest trust score
	score, err := h.trustCalculator.GetLatestTrustScore(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "No trust score found",
		})
	}

	// Define weights (matches trust_calculator.go weights)
	weights := map[string]float64{
		"verificationStatus": 0.25,
		"uptime":             0.15,
		"successRate":        0.15,
		"securityAlerts":     0.15,
		"compliance":         0.10,
		"age":                0.10,
		"driftDetection":     0.05,
		"userFeedback":       0.05,
	}

	// Calculate contributions (factor value × weight)
	contributions := map[string]float64{
		"verificationStatus": score.Factors.VerificationStatus * weights["verificationStatus"],
		"uptime":             score.Factors.Uptime * weights["uptime"],
		"successRate":        score.Factors.SuccessRate * weights["successRate"],
		"securityAlerts":     score.Factors.SecurityAlerts * weights["securityAlerts"],
		"compliance":         score.Factors.Compliance * weights["compliance"],
		"age":                score.Factors.Age * weights["age"],
		"driftDetection":     score.Factors.DriftDetection * weights["driftDetection"],
		"userFeedback":       score.Factors.UserFeedback * weights["userFeedback"],
	}

	return c.JSON(fiber.Map{
		"agentId":   agentID,
		"agentName": agent.Name,
		"overall":   score.Score,
		"factors": map[string]float64{
			"verificationStatus": score.Factors.VerificationStatus,
			"uptime":             score.Factors.Uptime,
			"successRate":        score.Factors.SuccessRate,
			"securityAlerts":     score.Factors.SecurityAlerts,
			"compliance":         score.Factors.Compliance,
			"age":                score.Factors.Age,
			"driftDetection":     score.Factors.DriftDetection,
			"userFeedback":       score.Factors.UserFeedback,
		},
		"weights":       weights,
		"contributions": contributions,
		"confidence":    score.Confidence,
		"calculatedAt":  score.LastCalculated,
	})
}

// GetTrustScoreHistory returns trust score audit trail for an agent
// Returns complete audit trail with who changed it, when, and why
func (h *TrustScoreHandler) GetTrustScoreHistory(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to organization
	agent, err := h.agentService.GetAgent(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Agent not found",
		})
	}

	if agent.OrganizationID != orgID {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied",
		})
	}

	// Optional: limit results
	limit := 30 // Default to last 30 entries
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get trust score audit trail from trust_score_history table
	history, err := h.trustCalculator.GetTrustScoreHistoryAuditTrail(c.Context(), agentID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch trust score history",
		})
	}

	// Return audit trail with proper JSON field names for frontend
	// Domain model already has correct JSON tags mapping to frontend expectations
	return c.JSON(fiber.Map{
		"agentId":   agentID,
		"agentName": agent.Name,
		"history":    history,
		"total":      len(history),
	})
}
