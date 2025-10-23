package handlers

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
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
	authService              *application.AuthService    // ✅ For fetching user counts
	alertService             *application.AlertService   // ✅ For fetching alert counts
	securityService          *application.SecurityService // ✅ For fetching incident counts
	db                       *sql.DB // Database connection for real analytics queries
}

func NewAnalyticsHandler(
	agentService *application.AgentService,
	auditService *application.AuditService,
	mcpService *application.MCPService,
	verificationEventService *application.VerificationEventService,
	authService *application.AuthService,
	alertService *application.AlertService,
	securityService *application.SecurityService,
	db *sql.DB,
) *AnalyticsHandler {
	return &AnalyticsHandler{
		agentService:             agentService,
		auditService:             auditService,
		mcpService:               mcpService,
		verificationEventService: verificationEventService,
		authService:              authService,
		alertService:             alertService,
		securityService:          securityService,
		db:                       db,
	}
}

// GetUsageStatistics retrieves usage statistics
// @Summary Get usage statistics
// @Description Get usage statistics for the organization
// @Tags analytics
// @Produce json
// @Param days query int false "Number of days" default(30)
// @Param period query string false "Period (day, week, month, year)" default(month)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/analytics/usage [get]
func (h *AnalyticsHandler) GetUsageStatistics(c fiber.Ctx) error {
	orgID := c.Locals("organization_id").(uuid.UUID)
	
	// Support both 'days' and 'period' parameters
	daysStr := c.Query("days", "")
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

	// Get REAL API call stats from database
	var apiCalls int64
	var dataVolumeMB float64

	// Calculate time range based on 'days' parameter or 'period'
	var startTime time.Time
	var periodLabel string
	
	if daysStr != "" {
		// Use days parameter
		days, err := strconv.Atoi(daysStr)
		if err != nil {
			days = 30
		}
		startTime = time.Now().AddDate(0, 0, -days)
		periodLabel = fmt.Sprintf("Last %d days", days)
	} else {
		// Use period parameter
		switch period {
		case "day":
			startTime = time.Now().AddDate(0, 0, -1)
			periodLabel = "Last 24 hours"
		case "week":
			startTime = time.Now().AddDate(0, 0, -7)
			periodLabel = "Last 7 days"
		case "year":
			startTime = time.Now().AddDate(-1, 0, 0)
			periodLabel = "Last year"
		default: // month
			startTime = time.Now().AddDate(0, -1, 0)
			periodLabel = "Last 30 days"
		}
	}

	// Query real API calls
	err = h.db.QueryRow(`
		SELECT
			COUNT(*) as api_calls,
			COALESCE(SUM(request_size_bytes + response_size_bytes) / 1024.0 / 1024.0, 0) as data_volume_mb
		FROM api_calls
		WHERE organization_id = $1
			AND called_at >= $2
	`, orgID, startTime).Scan(&apiCalls, &dataVolumeMB)

	if err != nil {
		// If table doesn't exist yet (migration not run), use defaults
		apiCalls = 0
		dataVolumeMB = 0
	}

	// ✅ Calculate REAL uptime from verification events
	// Uptime = (successful_verifications / total_verifications) * 100
	var totalVerifications int64
	var successfulVerifications int64
	var uptime float64

	err = h.db.QueryRow(`
		SELECT
			COUNT(*) as total,
			COUNT(CASE WHEN status = 'success' THEN 1 END) as successful
		FROM verification_events
		WHERE organization_id = $1
			AND started_at >= $2
	`, orgID, startTime).Scan(&totalVerifications, &successfulVerifications)

	if err != nil || totalVerifications == 0 {
		// Default to 100% if no verification events yet
		uptime = 100.0
	} else {
		uptime = (float64(successfulVerifications) / float64(totalVerifications)) * 100.0
	}

	stats := map[string]interface{}{
		"period":        periodLabel,    // Human-readable period label
		"total_agents":  totalAgents,
		"active_agents": activeAgents,
		"api_calls":     apiCalls,       // ✅ REAL DATA from database
		"data_volume":   dataVolumeMB,   // ✅ REAL DATA in MB
		"uptime":        uptime,          // ✅ REAL DATA - calculated from verification events
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
	// 🔍 Safe type assertion with error checking
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID type in context",
		})
	}
	
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

	// Get REAL trust score trends from database
	trends := []map[string]interface{}{}

	if period == "weeks" {
		// Query weekly aggregated trust scores WITH score ranges
		query := `
			WITH weekly_scores AS (
				SELECT
					DATE_TRUNC('week', recorded_at) as week_start,
					AVG(trust_score) as avg_score,
					COUNT(DISTINCT agent_id) as agent_count,
					COUNT(DISTINCT CASE WHEN trust_score >= 0.90 THEN agent_id END) as excellent,
					COUNT(DISTINCT CASE WHEN trust_score >= 0.70 AND trust_score < 0.90 THEN agent_id END) as good,
					COUNT(DISTINCT CASE WHEN trust_score >= 0.50 AND trust_score < 0.70 THEN agent_id END) as fair,
					COUNT(DISTINCT CASE WHEN trust_score < 0.50 THEN agent_id END) as poor
				FROM trust_score_history
				WHERE organization_id = $1
					AND recorded_at >= NOW() - INTERVAL '1 week' * $2
				GROUP BY DATE_TRUNC('week', recorded_at)
				ORDER BY week_start DESC
			)
			SELECT week_start, avg_score, agent_count, excellent, good, fair, poor
			FROM weekly_scores
			ORDER BY week_start ASC
		`

		rows, err := h.db.Query(query, orgID, weeks)
		if err != nil {
			// Fallback: Generate trend data based on agent creation dates (weekly)
			agents, _ := h.agentService.ListAgents(c.Context(), orgID)
			
			// Group agents by week and calculate average trust score
			weekScores := make(map[string][]float64)
			for _, agent := range agents {
				// Get start of week (Monday)
				weekStart := agent.CreatedAt.AddDate(0, 0, -int(agent.CreatedAt.Weekday()-time.Monday))
				weekKey := weekStart.Format("2006-01-02")
				weekScores[weekKey] = append(weekScores[weekKey], agent.TrustScore)
			}
			
			// Convert to sorted trends array
			type weekTrend struct {
				date     time.Time
				avgScore float64
				count    int
			}
			var sortedTrends []weekTrend
			for weekKey, scores := range weekScores {
				t, _ := time.Parse("2006-01-02", weekKey)
				total := 0.0
				for _, score := range scores {
					total += score
				}
				avg := total / float64(len(scores))
				sortedTrends = append(sortedTrends, weekTrend{
					date:     t,
					avgScore: avg,
					count:    len(scores),
				})
			}
			
			// Sort by date
			sort.Slice(sortedTrends, func(i, j int) bool {
				return sortedTrends[i].date.Before(sortedTrends[j].date)
			})
			
			// Build trends response
			trendsData := []map[string]interface{}{}
			for _, trend := range sortedTrends {
				trendsData = append(trendsData, map[string]interface{}{
					"date":        trend.date.Format("2006-01-02"),
					"week_start":  trend.date.Format("2006-01-02"),
					"avg_score":   trend.avgScore,
					"agent_count": trend.count,
				})
			}
			
			// Calculate overall average
			totalScore := 0.0
			for _, agent := range agents {
				totalScore += agent.TrustScore
			}
			avgScore := 0.0
			if len(agents) > 0 {
				avgScore = totalScore / float64(len(agents))
			}

			return c.JSON(fiber.Map{
				"period":          fmt.Sprintf("Last %d weeks", weeks),
				"trends":          trendsData,
				"current_average": avgScore,
				"data_type":       "weekly",
			})
		}
		defer rows.Close()

		for rows.Next() {
			var weekStart time.Time
			var avgScore float64
			var agentCount, excellent, good, fair, poor int

			if err := rows.Scan(&weekStart, &avgScore, &agentCount, &excellent, &good, &fair, &poor); err != nil {
				continue
			}

			trends = append(trends, map[string]interface{}{
				"date":        weekStart.Format("2006-01-02"),
				"week_start":  weekStart.Format("2006-01-02"),
				"avg_score":   avgScore,
				"agent_count": agentCount,
				"scores_by_range": map[string]interface{}{
					"excellent": excellent,
					"good":      good,
					"fair":      fair,
					"poor":      poor,
				},
			})
		}

		// Get current average for comparison
		agents, _ := h.agentService.ListAgents(c.Context(), orgID)
		totalScore := 0.0
		for _, agent := range agents {
			totalScore += agent.TrustScore
		}
		currentAvg := 0.0
		if len(agents) > 0 {
			currentAvg = totalScore / float64(len(agents))
		}

		// Calculate trend direction and change percentage
		trendDirection := "stable"
		changePercentage := 0.0

		if len(trends) >= 2 {
			// Compare first and last data points
			firstScore := trends[0]["avg_score"].(float64)
			lastScore := trends[len(trends)-1]["avg_score"].(float64)

			if lastScore > firstScore {
				changePercentage = ((lastScore - firstScore) / firstScore) * 100
				if changePercentage > 1.0 { // More than 1% increase
					trendDirection = "up"
				}
			} else if lastScore < firstScore {
				changePercentage = ((firstScore - lastScore) / firstScore) * 100
				if changePercentage > 1.0 { // More than 1% decrease
					trendDirection = "down"
				}
			}
		}

		return c.JSON(fiber.Map{
			"period":     fmt.Sprintf("Last %d weeks", weeks),
			"trends":     trends,
			"data_type":  "weekly",
			"summary": fiber.Map{
				"overall_avg":       currentAvg,
				"trend_direction":   trendDirection,
				"change_percentage": changePercentage,
			},
		})
	} else {
		// Query daily aggregated trust scores WITH score ranges
		query := `
			WITH daily_scores AS (
				SELECT
					DATE(recorded_at) as date,
					AVG(trust_score) as avg_score,
					COUNT(DISTINCT agent_id) as agent_count,
					COUNT(DISTINCT CASE WHEN trust_score >= 0.90 THEN agent_id END) as excellent,
					COUNT(DISTINCT CASE WHEN trust_score >= 0.70 AND trust_score < 0.90 THEN agent_id END) as good,
					COUNT(DISTINCT CASE WHEN trust_score >= 0.50 AND trust_score < 0.70 THEN agent_id END) as fair,
					COUNT(DISTINCT CASE WHEN trust_score < 0.50 THEN agent_id END) as poor
				FROM trust_score_history
				WHERE organization_id = $1
					AND recorded_at >= NOW() - INTERVAL '1 day' * $2
				GROUP BY DATE(recorded_at)
				ORDER BY date DESC
			)
			SELECT date, avg_score, agent_count, excellent, good, fair, poor
			FROM daily_scores
			ORDER BY date ASC
		`

		rows, err := h.db.Query(query, orgID, days)
		if err != nil {
			// Fallback: Generate trend data based on agent creation dates
			agents, _ := h.agentService.ListAgents(c.Context(), orgID)
			
			// Group agents by creation date and calculate average trust score
			dateScores := make(map[string][]float64)
			for _, agent := range agents {
				dateKey := agent.CreatedAt.Format("2006-01-02")
				dateScores[dateKey] = append(dateScores[dateKey], agent.TrustScore)
			}
			
			// Convert to sorted trends array
			type dateTrend struct {
				date     time.Time
				avgScore float64
				count    int
			}
			var sortedTrends []dateTrend
			for dateKey, scores := range dateScores {
				t, _ := time.Parse("2006-01-02", dateKey)
				total := 0.0
				for _, score := range scores {
					total += score
				}
				avg := total / float64(len(scores))
				sortedTrends = append(sortedTrends, dateTrend{
					date:     t,
					avgScore: avg,
					count:    len(scores),
				})
			}
			
			// Sort by date
			sort.Slice(sortedTrends, func(i, j int) bool {
				return sortedTrends[i].date.Before(sortedTrends[j].date)
			})
			
			// Build trends response
			trendsData := []map[string]interface{}{}
			for _, trend := range sortedTrends {
				trendsData = append(trendsData, map[string]interface{}{
					"date":        trend.date.Format("2006-01-02"),
					"avg_score":   trend.avgScore,
					"agent_count": trend.count,
				})
			}
			
			// Calculate overall average
			totalScore := 0.0
			for _, agent := range agents {
				totalScore += agent.TrustScore
			}
			avgScore := 0.0
			if len(agents) > 0 {
				avgScore = totalScore / float64(len(agents))
			}

			return c.JSON(fiber.Map{
				"period":          fmt.Sprintf("Last %d days", days),
				"trends":          trendsData,
				"current_average": avgScore,
				"data_type":       "daily",
			})
		}
		defer rows.Close()

		for rows.Next() {
			var date time.Time
			var avgScore float64
			var agentCount, excellent, good, fair, poor int

			if err := rows.Scan(&date, &avgScore, &agentCount, &excellent, &good, &fair, &poor); err != nil {
				continue
			}

			trends = append(trends, map[string]interface{}{
				"date":        date.Format("2006-01-02"),
				"avg_score":   avgScore,
				"agent_count": agentCount,
				"scores_by_range": map[string]interface{}{
					"excellent": excellent,
					"good":      good,
					"fair":      fair,
					"poor":      poor,
				},
			})
		}

		// Get current average
		agents, _ := h.agentService.ListAgents(c.Context(), orgID)
		totalScore := 0.0
		for _, agent := range agents {
			totalScore += agent.TrustScore
		}
		currentAvg := 0.0
		if len(agents) > 0 {
			currentAvg = totalScore / float64(len(agents))
		}

		// Calculate trend direction and change percentage
		trendDirection := "stable"
		changePercentage := 0.0

		if len(trends) >= 2 {
			// Compare first and last data points
			firstScore := trends[0]["avg_score"].(float64)
			lastScore := trends[len(trends)-1]["avg_score"].(float64)

			if lastScore > firstScore {
				changePercentage = ((lastScore - firstScore) / firstScore) * 100
				if changePercentage > 1.0 { // More than 1% increase
					trendDirection = "up"
				}
			} else if lastScore < firstScore {
				changePercentage = ((firstScore - lastScore) / firstScore) * 100
				if changePercentage > 1.0 { // More than 1% decrease
					trendDirection = "down"
				}
			}
		}

		return c.JSON(fiber.Map{
			"period":    fmt.Sprintf("Last %d days", days),
			"trends":    trends,
			"data_type": "daily",
			"summary": fiber.Map{
				"overall_avg":       currentAvg,
				"trend_direction":   trendDirection,
				"change_percentage": changePercentage,
			},
		})
	}
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
	// 🔍 Safe type assertion with error checking
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID type in context",
		})
	}
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

	// Get verification activity from agents table based on created_at and verified_at
	activity := []map[string]interface{}{}

	// Query to get monthly agent creation and verification activity
	query := `
		WITH monthly_activity AS (
			SELECT
				DATE_TRUNC('month', created_at) as month_start,
				COUNT(*) FILTER (WHERE status = 'verified') as verified,
				COUNT(*) FILTER (WHERE status = 'pending') as pending
			FROM agents
			WHERE organization_id = $1
				AND created_at >= NOW() - INTERVAL '1 month' * $2
			GROUP BY DATE_TRUNC('month', created_at)
			ORDER BY month_start ASC
		)
		SELECT month_start, verified, pending
		FROM monthly_activity
	`

	rows, err := h.db.Query(query, orgID, months)
	if err != nil {
		// Fallback: if query fails, generate activity based on current agents
		// Group agents by creation month
		monthlyData := make(map[string]map[string]int)
		now := time.Now()
		startDate := now.AddDate(0, -months, 0)

		for _, agent := range agents {
			if agent.CreatedAt.After(startDate) {
				monthKey := agent.CreatedAt.Format("2006-01")
				if monthlyData[monthKey] == nil {
					monthlyData[monthKey] = map[string]int{"verified": 0, "pending": 0}
				}
				if agent.Status == "verified" {
					monthlyData[monthKey]["verified"]++
				} else if agent.Status == "pending" {
					monthlyData[monthKey]["pending"]++
				}
			}
		}

		// Convert map to sorted array
		type monthData struct {
			date     time.Time
			verified int
			pending  int
		}
		var sortedMonths []monthData
		for monthKey, counts := range monthlyData {
			t, _ := time.Parse("2006-01", monthKey)
			sortedMonths = append(sortedMonths, monthData{
				date:     t,
				verified: counts["verified"],
				pending:  counts["pending"],
			})
		}
		// Sort by date
		sort.Slice(sortedMonths, func(i, j int) bool {
			return sortedMonths[i].date.Before(sortedMonths[j].date)
		})

		for _, data := range sortedMonths {
			activity = append(activity, map[string]interface{}{
				"month":      data.date.Format("Jan"),
				"verified":   data.verified,
				"pending":    data.pending,
				"month_year": data.date.Format("2006-01"),
			})
		}

		// If no activity data, add current month
		if len(activity) == 0 {
			activity = append(activity, map[string]interface{}{
				"month":      now.Format("Jan"),
				"verified":   verifiedCount,
				"pending":    pendingCount,
				"month_year": now.Format("2006-01"),
			})
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
	defer rows.Close()

	for rows.Next() {
		var monthStart time.Time
		var verified, pending int

		if err := rows.Scan(&monthStart, &verified, &pending); err != nil {
			continue
		}

		activity = append(activity, map[string]interface{}{
			"month":      monthStart.Format("Jan"),
			"verified":   verified,
			"pending":    pending,
			"month_year": monthStart.Format("2006-01"),
		})
	}

	// If no activity from database, generate from agents list
	if len(activity) == 0 {
		// Group agents by creation month
		monthlyData := make(map[string]map[string]int)
		now := time.Now()
		startDate := now.AddDate(0, -months, 0)

		for _, agent := range agents {
			if agent.CreatedAt.After(startDate) {
				monthKey := agent.CreatedAt.Format("2006-01")
				if monthlyData[monthKey] == nil {
					monthlyData[monthKey] = map[string]int{"verified": 0, "pending": 0}
				}
				if agent.Status == "verified" {
					monthlyData[monthKey]["verified"]++
				} else if agent.Status == "pending" {
					monthlyData[monthKey]["pending"]++
				}
			}
		}

		// Convert map to sorted array
		type monthData struct {
			date     time.Time
			verified int
			pending  int
		}
		var sortedMonths []monthData
		for monthKey, counts := range monthlyData {
			t, _ := time.Parse("2006-01", monthKey)
			sortedMonths = append(sortedMonths, monthData{
				date:     t,
				verified: counts["verified"],
				pending:  counts["pending"],
			})
		}
		// Sort by date
		sort.Slice(sortedMonths, func(i, j int) bool {
			return sortedMonths[i].date.Before(sortedMonths[j].date)
		})

		for _, data := range sortedMonths {
			activity = append(activity, map[string]interface{}{
				"month":      data.date.Format("Jan"),
				"verified":   data.verified,
				"pending":    data.pending,
				"month_year": data.date.Format("2006-01"),
			})
		}

		// If still no activity, add current month
		if len(activity) == 0 {
			now := time.Now()
			activity = append(activity, map[string]interface{}{
				"month":      now.Format("Jan"),
				"verified":   verifiedCount,
				"pending":    pendingCount,
				"month_year": now.Format("2006-01"),
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

	// Get REAL agent activity from agent_activity_metrics table
	query := `
		SELECT
			a.id,
			a.name,
			a.status,
			a.trust_score,
			COALESCE(MAX(aam.hour_timestamp), a.created_at) as last_active,
			COALESCE(SUM(aam.api_calls_count), 0) as api_calls,
			COALESCE(SUM(aam.data_processed_bytes) / 1024.0 / 1024.0, 0) as data_processed_mb
		FROM agents a
		LEFT JOIN agent_activity_metrics aam ON a.id = aam.agent_id
		WHERE a.organization_id = $1
		GROUP BY a.id, a.name, a.status, a.trust_score, a.created_at
		ORDER BY last_active DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := h.db.Query(query, orgID, limit, offset)
	if err != nil {
		// Fallback: if agent_activity_metrics table doesn't exist, use basic agent data
		agents, err := h.agentService.ListAgents(c.Context(), orgID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch agent activity",
			})
		}

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
				"last_active":    agent.CreatedAt,
				"timestamp":      agent.CreatedAt, // Frontend expects 'timestamp' field
				"api_calls":      0,
				"data_processed": 0.0,
			})
		}

		// Calculate summary statistics for fallback case
		totalActivities := len(activities)
		successCount := 0
		failureCount := 0

		for _, activity := range activities {
			status, ok := activity["status"].(string)
			if !ok {
				continue
			}
			if status == "verified" || status == "success" {
				successCount++
			} else if status == "pending" || status == "failed" {
				failureCount++
			}
		}

		successRate := 0.0
		if totalActivities > 0 {
			successRate = (float64(successCount) / float64(totalActivities)) * 100
		}

		return c.JSON(fiber.Map{
			"activities": activities,
			"summary": fiber.Map{
				"total_activities": totalActivities,
				"success_count":    successCount,
				"failure_count":    failureCount,
				"success_rate":     successRate,
			},
			"total":  len(agents),
			"limit":  limit,
			"offset": offset,
			"note":   "Activity metrics not yet available. Install migration 010 to enable tracking.",
		})
	}
	defer rows.Close()

	// Build activity data from REAL database records
	activities := []map[string]interface{}{}
	for rows.Next() {
		var agentID uuid.UUID
		var name, status string
		var trustScore float64
		var lastActive time.Time
		var apiCalls int64
		var dataProcessedMB float64

		if err := rows.Scan(&agentID, &name, &status, &trustScore, &lastActive, &apiCalls, &dataProcessedMB); err != nil {
			continue
		}

		activities = append(activities, map[string]interface{}{
			"agent_id":       agentID.String(),
			"agent_name":     name,
			"status":         status,
			"trust_score":    trustScore,
			"last_active":    lastActive,
			"timestamp":      lastActive, // Frontend expects 'timestamp' field
			"api_calls":      apiCalls,
			"data_processed": dataProcessedMB, // in MB
		})
	}

	// Get total count for pagination
	var total int
	countQuery := `SELECT COUNT(*) FROM agents WHERE organization_id = $1`
	h.db.QueryRow(countQuery, orgID).Scan(&total)

	// Calculate summary statistics for the activity timeline
	totalActivities := len(activities)
	successCount := 0
	failureCount := 0

	for _, activity := range activities {
		status, ok := activity["status"].(string)
		if !ok {
			continue
		}
		if status == "verified" || status == "success" {
			successCount++
		} else if status == "pending" || status == "failed" {
			failureCount++
		}
	}

	successRate := 0.0
	if totalActivities > 0 {
		successRate = (float64(successCount) / float64(totalActivities)) * 100
	}

	return c.JSON(fiber.Map{
		"activities": activities,
		"summary": fiber.Map{
			"total_activities": totalActivities,
			"success_count":    successCount,
			"failure_count":    failureCount,
			"success_rate":     successRate,
		},
		"total":  total,
		"limit":  limit,
		"offset": offset,
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
	// 🔍 Safe type assertion with error checking
	orgIDValue := c.Locals("organization_id")
	if orgIDValue == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Organization ID not found in context",
		})
	}

	orgID, ok := orgIDValue.(uuid.UUID)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Invalid organization ID type in context",
		})
	}

	// Fetch agents
	agents, err := h.agentService.ListAgents(c.Context(), orgID)
	if err != nil {
		// 🔍 LOG DETAILED ERROR for debugging
		log.Printf("❌ Failed to fetch agents for org %s: %v", orgID.String(), err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": fmt.Sprintf("Failed to fetch agents: %v", err),
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

	// ✅ Fetch REAL user count from database
	users, err := h.authService.GetUsersByOrganization(c.Context(), orgID)
	totalUsers := 0
	activeUsers := 0
	if err == nil {
		totalUsers = len(users)
		// Count active users (those with status "active")
		for _, user := range users {
			if user.Status == domain.UserStatusActive {
				activeUsers++
			}
		}
	}

	// ✅ Fetch REAL security metrics from database
	activeAlerts := 0
	criticalAlerts := 0
	securityIncidents := 0
	alerts, _, err := h.alertService.GetAlerts(c.Context(), orgID, "", "open", 1000, 0)
	if err == nil {
		activeAlerts = len(alerts)
		// Count critical severity alerts
		for _, alert := range alerts {
			if alert.Severity == domain.AlertSeverityCritical {
				criticalAlerts++
			}
		}
	}
	// Get open security incidents count
	incidents, err := h.securityService.GetIncidents(c.Context(), orgID, domain.IncidentStatusOpen, 100, 0)
	if err == nil {
		securityIncidents = len(incidents)
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

		// User metrics ✅ REAL DATA
		"total_users":  totalUsers,
		"active_users": activeUsers,

		// Security metrics ✅ REAL DATA
		"active_alerts":      activeAlerts,
		"critical_alerts":    criticalAlerts,
		"security_incidents": securityIncidents,

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
