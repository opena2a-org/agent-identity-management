package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/application"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

type TrustScoreHandler struct {
	// Concrete service pointers (used by existing code)
	trustCalculator *application.TrustCalculator
	agentService    *application.AgentService
	auditService    *application.AuditService

	// Interface fields for testability (used when set)
	trustCalculatorServicer TrustCalculatorServicer
	agentServicer           AgentServicer
	auditServicer           AuditServicer
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

// NewTrustScoreHandlerWithInterfaces creates a trust score handler with interface dependencies for testing
func NewTrustScoreHandlerWithInterfaces(
	trustCalculator TrustCalculatorServicer,
	agentService AgentServicer,
	auditService AuditServicer,
) *TrustScoreHandler {
	return &TrustScoreHandler{
		trustCalculatorServicer: trustCalculator,
		agentServicer:           agentService,
		auditServicer:           auditService,
	}
}

// Helper methods to get the appropriate service (interface or concrete)
func (h *TrustScoreHandler) getTrustCalculator() TrustCalculatorServicer {
	if h.trustCalculatorServicer != nil {
		return h.trustCalculatorServicer
	}
	return h.trustCalculator
}

func (h *TrustScoreHandler) getAgentService() AgentServicer {
	if h.agentServicer != nil {
		return h.agentServicer
	}
	return h.agentService
}

func (h *TrustScoreHandler) getAuditService() AuditServicer {
	if h.auditServicer != nil {
		return h.auditServicer
	}
	return h.auditService
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
	agent, err := h.getAgentService().GetAgent(c.Context(), agentID)
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
	score, err := h.getTrustCalculator().CalculateTrustScore(c.Context(), agentID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to calculate trust score",
		})
	}

	// Log audit
	h.getAuditService().LogAction(
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
		"score":        score.Score,
		"factors":      score.Factors,
		"calculatedAt": score.LastCalculated,
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
	agent, err := h.getAgentService().GetAgent(c.Context(), agentID)
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

	// Get latest trust score, or calculate on-the-fly if none exists
	score, err := h.getTrustCalculator().GetLatestTrustScore(c.Context(), agentID)
	if err != nil || score == nil {
		// No stored score - calculate one on-the-fly
		score, err = h.getTrustCalculator().CalculateTrustScore(c.Context(), agentID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to calculate trust score",
			})
		}
	}

	// Defensive nil check
	if score == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Trust score calculation returned nil",
		})
	}

	return c.JSON(fiber.Map{
		"agentId":       agentID,
		"agentName":     agent.Name,
		"score":         score.Score,
		"factors":       score.Factors,
		"calculatedAt":  score.LastCalculated,
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
	agent, err := h.getAgentService().GetAgent(c.Context(), agentID)
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

	// Get latest trust score, or calculate on-the-fly if none exists
	score, err := h.getTrustCalculator().GetLatestTrustScore(c.Context(), agentID)
	if err != nil || score == nil {
		// No stored score - calculate one on-the-fly (this also updates agent.TrustScore)
		score, err = h.getTrustCalculator().CalculateTrustScore(c.Context(), agentID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to calculate trust score",
			})
		}
	}
	// NOTE: We intentionally do NOT auto-sync/recalculate trust scores here.
	// The agent.TrustScore may differ from stored score.Score due to direct penalties
	// from violations. Recalculating would cause double-counting because the 9-factor
	// algorithm also considers security alerts created by violations.
	// Trust score recalculation should only happen on explicit user request or scheduled intervals.

	// Verify score is not nil (defensive check)
	if score == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Trust score calculation returned invalid data",
		})
	}

	// Define weights (must match the 9-factor weights in trust_calculator.go;
	// these sum to exactly 1.0). Age, drift, and user feedback were reduced to
	// make room for the 10% execution isolation factor.
	weights := map[string]float64{
		"verificationStatus": 0.25,
		"uptime":             0.15,
		"successRate":        0.15,
		"securityAlerts":     0.15,
		"compliance":         0.10,
		"age":                0.05,
		"driftDetection":     0.03,
		"userFeedback":       0.02,
		"executionIsolation": 0.10,
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
		"executionIsolation": score.Factors.ExecutionIsolation * weights["executionIsolation"],
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
			"executionIsolation": score.Factors.ExecutionIsolation,
		},
		"weights":       weights,
		"contributions": contributions,
		"confidence":    score.Confidence,
		"calculatedAt":  score.LastCalculated,
	})
}

// SubmitUserFeedback records an explicit user rating (1-5) for an agent. This is
// the collection side of the user feedback trust factor: ratings persisted here
// are what calculateUserFeedback reads on the next score computation.
func (h *TrustScoreHandler) SubmitUserFeedback(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	userID := c.Locals("user_id").(uuid.UUID)
	agentID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid agent ID",
		})
	}

	// Verify agent belongs to the caller's organization before accepting feedback
	agent, err := h.getAgentService().GetAgent(c.Context(), agentID)
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

	var body struct {
		Rating  int    `json:"rating"`
		Comment string `json:"comment"`
	}
	if err := c.Bind().JSON(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	if body.Rating < 1 || body.Rating > 5 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Rating must be an integer between 1 and 5",
		})
	}

	feedback, err := h.getTrustCalculator().RecordUserFeedback(c.Context(), agentID, orgID, &userID, body.Rating, body.Comment)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to record user feedback",
		})
	}

	h.getAuditService().LogAction(
		c.Context(),
		orgID,
		userID,
		domain.AuditActionCreate,
		"user_feedback",
		agentID,
		c.IP(),
		c.Get("User-Agent"),
		map[string]interface{}{
			"agentName": agent.Name,
			"rating":    body.Rating,
		},
	)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"id":        feedback.ID,
		"agentId":   feedback.AgentID,
		"rating":    feedback.Rating,
		"comment":   feedback.Comment,
		"createdAt": feedback.CreatedAt,
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
	agent, err := h.getAgentService().GetAgent(c.Context(), agentID)
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
	history, err := h.getTrustCalculator().GetTrustScoreHistoryAuditTrail(c.Context(), agentID, limit)
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
