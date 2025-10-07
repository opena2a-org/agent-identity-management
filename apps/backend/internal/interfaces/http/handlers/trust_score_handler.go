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
			"agent_name":  agent.Name,
			"trust_score": score.Score,
			"factors":     score.Factors,
		},
	)

	return c.JSON(fiber.Map{
		"agent_id":      agentID,
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
		"agent_id":      agentID,
		"agent_name":    agent.Name,
		"score":         score.Score,
		"factors":       score.Factors,
		"calculated_at": score.LastCalculated,
	})
}

// GetTrustScoreHistory returns trust score history for an agent
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
	limit := 30 // Default to last 30 scores
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get trust score history
	history, err := h.trustCalculator.GetTrustScoreHistory(c.Context(), agentID, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch trust score history",
		})
	}

	return c.JSON(fiber.Map{
		"agent_id":   agentID,
		"agent_name": agent.Name,
		"history":    history,
		"total":      len(history),
	})
}

// GetTrustScoreTrends returns trust score trends across all agents
func (h *TrustScoreHandler) GetTrustScoreTrends(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	// Get all agents for organization
	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents",
		})
	}

	type TrustScoreTrend struct {
		AgentID      uuid.UUID              `json:"agent_id"`
		AgentName    string                 `json:"agent_name"`
		CurrentScore float64                `json:"current_score"`
		PreviousScore float64               `json:"previous_score,omitempty"`
		Trend        string                 `json:"trend"` // "up", "down", "stable"
		Factors      map[string]interface{} `json:"factors,omitempty"`
	}

	trends := make([]TrustScoreTrend, 0)

	for _, agent := range agents {
		// Get latest score
		latestScore, err := h.trustCalculator.GetLatestTrustScore(c.Context(), agent.ID)
		if err != nil {
			// Skip agents without trust scores
			continue
		}

		// Get previous score for trend
		history, err := h.trustCalculator.GetTrustScoreHistory(c.Context(), agent.ID, 2)
		if err != nil {
			continue
		}

		// Convert Factors to map
		factorsMap := map[string]interface{}{
			"verification_status":  latestScore.Factors.VerificationStatus,
			"certificate_validity": latestScore.Factors.CertificateValidity,
			"repository_quality":   latestScore.Factors.RepositoryQuality,
			"documentation_score":  latestScore.Factors.DocumentationScore,
			"community_trust":      latestScore.Factors.CommunityTrust,
			"security_audit":       latestScore.Factors.SecurityAudit,
			"update_frequency":     latestScore.Factors.UpdateFrequency,
			"age_score":            latestScore.Factors.AgeScore,
		}

		trend := TrustScoreTrend{
			AgentID:      agent.ID,
			AgentName:    agent.Name,
			CurrentScore: latestScore.Score,
			Factors:      factorsMap,
		}

		// Calculate trend
		if len(history) >= 2 {
			trend.PreviousScore = history[1].Score
			if latestScore.Score > history[1].Score {
				trend.Trend = "up"
			} else if latestScore.Score < history[1].Score {
				trend.Trend = "down"
			} else {
				trend.Trend = "stable"
			}
		} else {
			trend.Trend = "new"
		}

		trends = append(trends, trend)
	}

	return c.JSON(fiber.Map{
		"organization_id": orgID,
		"trends":          trends,
		"total_agents":    len(trends),
	})
}
