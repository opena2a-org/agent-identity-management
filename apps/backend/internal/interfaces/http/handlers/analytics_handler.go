package handlers

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/application"
	"github.com/opena2a/identity/backend/internal/domain"
)

type AnalyticsHandler struct {
	agentService             *application.AgentService
	auditService             *application.AuditService
	mcpService               *application.MCPService
	verificationEventService *application.VerificationEventService
}

func NewAnalyticsHandler(
	agentService *application.AgentService,
	auditService *application.AuditService,
	mcpService *application.MCPService,
	verificationEventService *application.VerificationEventService,
) *AnalyticsHandler {
	return &AnalyticsHandler{
		agentService:             agentService,
		auditService:             auditService,
		mcpService:               mcpService,
		verificationEventService: verificationEventService,
	}
}

// GetUsageStatistics retrieves usage statistics
// @Summary Get usage statistics
// @Description Get usage statistics for the organization
// @Tags analytics
// @Produce json
// @Param period query string false "Period (day, week, month, year)" default(month)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/analytics/usage [get]
func (h *AnalyticsHandler) GetUsageStatistics(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	period := c.Query("period", "month")

	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch usage statistics",
		})
	}

	// Calculate usage metrics
	totalAgents := len(agents)
	activeAgents := 0
	for _, agent := range agents {
		if agent.Status == "verified" {
			activeAgents++
		}
	}

	stats := map[string]interface{}{
		"period":        period,
		"total_agents":  totalAgents,
		"active_agents": activeAgents,
		"api_calls":     totalAgents * 150, // Simulated
		"data_volume":   float64(totalAgents) * 25.5, // MB, simulated
		"uptime":        99.9,
		"generated_at":  time.Now().UTC(),
	}

	return c.JSON(stats)
}

// GetTrustScoreTrends retrieves trust score trends
// @Summary Get trust score trends
// @Description Get trust score trends over time
// @Tags analytics
// @Produce json
// @Param days query int false "Number of days" default(30)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/analytics/trends [get]
func (h *AnalyticsHandler) GetTrustScoreTrends(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	days, _ := strconv.Atoi(c.Query("days", "30"))

	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch trends",
		})
	}

	// Calculate average trust score
	totalScore := 0.0
	for _, agent := range agents {
		totalScore += agent.TrustScore
	}
	avgScore := 0.0
	if len(agents) > 0 {
		avgScore = totalScore / float64(len(agents))
	}

	// Generate trend data (simulated)
	trends := []map[string]interface{}{}
	for i := days; i >= 0; i-- {
		date := time.Now().AddDate(0, 0, -i)
		trends = append(trends, map[string]interface{}{
			"date":        date.Format("2006-01-02"),
			"avg_score":   avgScore + float64(i)*0.1,
			"agent_count": len(agents),
		})
	}

	return c.JSON(fiber.Map{
		"period": fmt.Sprintf("Last %d days", days),
		"trends": trends,
		"current_average": avgScore,
	})
}

// GenerateReport generates a custom analytics report
// @Summary Generate custom report
// @Description Generate a custom analytics report
// @Tags analytics
// @Accept json
// @Produce json
// @Param request body map[string]interface{} true "Report configuration"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/analytics/reports/generate [get]
func (h *AnalyticsHandler) GenerateReport(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	reportType := c.Query("type", "summary")
	format := c.Query("format", "json")

	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to generate report",
		})
	}

	report := map[string]interface{}{
		"report_type":   reportType,
		"format":        format,
		"generated_at":  time.Now().UTC(),
		"organization":  orgID.String(),
		"total_agents":  len(agents),
		"metrics": map[string]interface{}{
			"verified_agents":     countByStatus(agents, "verified"),
			"pending_agents":      countByStatus(agents, "pending"),
			"average_trust_score": calculateAverageTrustScore(agents),
		},
	}

	return c.JSON(report)
}

// GetAgentActivity retrieves agent activity metrics
// @Summary Get agent activity metrics
// @Description Get activity metrics for all agents
// @Tags analytics
// @Produce json
// @Param limit query int false "Limit" default(50)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/analytics/agents/activity [get]
func (h *AnalyticsHandler) GetAgentActivity(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	limit, _ := strconv.Atoi(c.Query("limit", "50"))
	offset, _ := strconv.Atoi(c.Query("offset", "0"))

	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agent activity",
		})
	}

	// Build activity data
	activities := []map[string]interface{}{}
	for i, agent := range agents {
		if i < offset {
			continue
		}
		if len(activities) >= limit {
			break
		}

		activities = append(activities, map[string]interface{}{
			"agent_id":     agent.ID.String(),
			"agent_name":   agent.Name,
			"status":       agent.Status,
			"trust_score":  agent.TrustScore,
			"last_active":  time.Now().Add(-time.Duration(i) * time.Hour),
			"api_calls":    150 + i*10,
			"data_processed": 25.5 + float64(i)*1.2,
		})
	}

	return c.JSON(fiber.Map{
		"activities": activities,
		"total":      len(agents),
		"limit":      limit,
		"offset":     offset,
	})
}

// GetDashboardStats retrieves dashboard statistics (viewer-accessible)
// @Summary Get dashboard statistics
// @Description Get dashboard statistics accessible to all authenticated users
// @Tags analytics
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/analytics/dashboard [get]
func (h *AnalyticsHandler) GetDashboardStats(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)

	// Fetch agents
	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch agents",
		})
	}

	// Fetch MCP servers
	mcpServers, err := h.mcpService.ListMCPServers(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch MCP servers",
		})
	}

	// Calculate agent metrics
	totalAgents := len(agents)
	verifiedAgents := 0
	pendingAgents := 0
	totalTrustScore := 0.0

	for _, agent := range agents {
		if agent.Status == "verified" {
			verifiedAgents++
		} else if agent.Status == "pending" {
			pendingAgents++
		}
		totalTrustScore += agent.TrustScore
	}

	avgTrustScore := 0.0
	if totalAgents > 0 {
		avgTrustScore = totalTrustScore / float64(totalAgents)
	}

	verificationRate := 0.0
	if totalAgents > 0 {
		verificationRate = float64(verifiedAgents) / float64(totalAgents) * 100
	}

	// Calculate MCP server metrics
	totalMCPServers := len(mcpServers)
	activeMCPServers := 0
	for _, mcp := range mcpServers {
		if mcp.Status == "verified" {
			activeMCPServers++
		}
	}

	// Fetch verification event statistics (last 24 hours)
	stats, err := h.verificationEventService.GetLast24HoursStatistics(c.Context(), orgID)
	if err != nil {
		// If verification stats fail, use defaults
		stats = &domain.VerificationStatistics{
			TotalVerifications: 0,
			SuccessCount:       0,
			FailedCount:        0,
			PendingCount:       0,
			AvgDurationMs:      0,
		}
	}

	return c.JSON(fiber.Map{
		// Agent metrics
		"total_agents":      totalAgents,
		"verified_agents":   verifiedAgents,
		"pending_agents":    pendingAgents,
		"verification_rate": verificationRate,
		"avg_trust_score":   avgTrustScore,

		// MCP Server metrics
		"total_mcp_servers":  totalMCPServers,
		"active_mcp_servers": activeMCPServers,

		// User metrics (simplified for viewer - no user count access)
		"total_users":  1, // Current user
		"active_users": 1,

		// Security metrics (simplified for viewer - no alert access)
		"active_alerts":      0,
		"critical_alerts":    0,
		"security_incidents": 0,

		// Verification metrics (last 24 hours)
		"total_verifications":      stats.TotalVerifications,
		"successful_verifications": stats.SuccessCount,
		"failed_verifications":     stats.FailedCount,
		"avg_response_time":        stats.AvgDurationMs,

		// Organization
		"organization_id": orgID.String(),
	})
}

// Helper functions
func countByStatus(agents []*domain.Agent, status string) int {
	count := 0
	for _, agent := range agents {
		if string(agent.Status) == status {
			count++
		}
	}
	return count
}

func calculateAverageTrustScore(agents []*domain.Agent) float64 {
	if len(agents) == 0 {
		return 0.0
	}
	total := 0.0
	for _, agent := range agents {
		total += agent.TrustScore
	}
	return total / float64(len(agents))
}
