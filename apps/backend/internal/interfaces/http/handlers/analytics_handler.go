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
		"api_calls":     totalAgents * 150,           // Simulated
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
// @Param weeks query int false "Number of weeks" default(4)
// @Param period query string false "Period type (weeks, days)" default(weeks)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/analytics/trends [get]
func (h *AnalyticsHandler) GetTrustScoreTrends(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	
	// Support both days and weeks parameters for backward compatibility
	period := c.Query("period", "weeks")
	weeks := 4
	days := 30
	
	if period == "weeks" {
		weeks, _ = strconv.Atoi(c.Query("weeks", "4"))
	} else {
		days, _ = strconv.Atoi(c.Query("days", "30"))
		weeks = days / 7 // Convert days to weeks
		if weeks == 0 {
			weeks = 1
		}
	}

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

	// Generate weekly trend data (simulated)
	trends := []map[string]interface{}{}
	
	if period == "weeks" {
		// Generate weekly data
		for i := weeks; i >= 1; i-- {
			// Calculate the start of each week (Monday)
			now := time.Now()
			weeksAgo := time.Duration(i-1) * 7 * 24 * time.Hour
			weekStart := now.Add(-weeksAgo)
			
			// Find the Monday of this week
			for weekStart.Weekday() != time.Monday {
				weekStart = weekStart.AddDate(0, 0, -1)
			}
			
			// Create simulated trend with some variation
			scoreVariation := avgScore + float64(i)*0.05 - 0.1 + (float64(weeks-i)*0.02)
			if scoreVariation < 0 {
				scoreVariation = avgScore * 0.8
			}
			if scoreVariation > 1.0 {
				scoreVariation = 1.0
			}
			
			trends = append(trends, map[string]interface{}{
				"date":        fmt.Sprintf("Week %d", weeks-i+1),
				"week_start":  weekStart.Format("2006-01-02"),
				"avg_score":   scoreVariation,
				"agent_count": len(agents),
			})
		}
		
		return c.JSON(fiber.Map{
			"period":          fmt.Sprintf("Last %d weeks", weeks),
			"trends":          trends,
			"current_average": avgScore,
			"data_type":       "weekly",
		})
	} else {
		// Generate daily data (backward compatibility)
		for i := days; i >= 0; i-- {
			date := time.Now().AddDate(0, 0, -i)
			trends = append(trends, map[string]interface{}{
				"date":        date.Format("2006-01-02"),
				"avg_score":   avgScore + float64(i)*0.01,
				"agent_count": len(agents),
			})
		}
		
		return c.JSON(fiber.Map{
			"period":          fmt.Sprintf("Last %d days", days),
			"trends":          trends,
			"current_average": avgScore,
			"data_type":       "daily",
		})
	}
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
		"report_type":  reportType,
		"format":       format,
		"generated_at": time.Now().UTC(),
		"organization": orgID.String(),
		"total_agents": len(agents),
		"metrics": map[string]interface{}{
			"verified_agents":     countByStatus(agents, "verified"),
			"pending_agents":      countByStatus(agents, "pending"),
			"average_trust_score": calculateAverageTrustScore(agents),
		},
	}

	return c.JSON(report)
}

// GetVerificationActivity retrieves monthly verification activity trends
// @Summary Get verification activity trends  
// @Description Get monthly verification activity showing verified vs pending agents
// @Tags analytics
// @Produce json
// @Param months query int false "Number of months" default(6)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/analytics/verification-activity [get]
func (h *AnalyticsHandler) GetVerificationActivity(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	months, _ := strconv.Atoi(c.Query("months", "6"))

	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch verification activity",
		})
	}

	// Calculate current verified and pending counts
	verifiedCount := 0
	pendingCount := 0
	for _, agent := range agents {
		if agent.Status == "verified" {
			verifiedCount++
		} else if agent.Status == "pending" {
			pendingCount++
		}
	}

	// Generate monthly verification activity data
	activity := []map[string]interface{}{}
	now := time.Now()
	
	for i := months - 1; i >= 0; i-- {
		monthDate := now.AddDate(0, -i, 0)
		monthName := monthDate.Format("Jan")
		
		// Simulate historical data with some variation
		// Current month uses real data, previous months are simulated
		if i == 0 {
			// Current month - use real data
			activity = append(activity, map[string]interface{}{
				"month":     monthName,
				"verified":  verifiedCount,
				"pending":   pendingCount,
				"month_year": monthDate.Format("2006-01"),
			})
		} else {
			// Historical months - simulate data based on current state
			historicalVerified := verifiedCount - (i * 2) + ((i % 3) * 3)
			historicalPending := pendingCount + (i % 2) + 1
			
			// Ensure non-negative values
			if historicalVerified < 0 {
				historicalVerified = verifiedCount / 2
			}
			if historicalPending < 0 {
				historicalPending = 1
			}
			
			activity = append(activity, map[string]interface{}{
				"month":     monthName,
				"verified":  historicalVerified,
				"pending":   historicalPending,
				"month_year": monthDate.Format("2006-01"),
			})
		}
	}

	return c.JSON(fiber.Map{
		"period":   fmt.Sprintf("Last %d months", months),
		"activity": activity,
		"current_stats": map[string]interface{}{
			"total_verified": verifiedCount,
			"total_pending":  pendingCount,
			"total_agents":   len(agents),
		},
	})
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
			"agent_id":       agent.ID.String(),
			"agent_name":     agent.Name,
			"status":         agent.Status,
			"trust_score":    agent.TrustScore,
			"last_active":    time.Now().Add(-time.Duration(i) * time.Hour),
			"api_calls":      150 + i*10,
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
		fmt.Printf("Error fetching MCP servers: %v", err)
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
