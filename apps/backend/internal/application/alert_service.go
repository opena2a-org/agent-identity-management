package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// AlertService handles alert management
type AlertService struct {
	alertRepo domain.AlertRepository
	agentRepo domain.AgentRepository
	db        *sql.DB // For anomaly detection queries
}

// NewAlertService creates a new alert service
func NewAlertService(
	alertRepo domain.AlertRepository,
	agentRepo domain.AgentRepository,
	db *sql.DB,
) *AlertService {
	return &AlertService{
		alertRepo: alertRepo,
		agentRepo: agentRepo,
		db:        db,
	}
}

// CreateAlert creates a new alert
func (s *AlertService) CreateAlert(ctx context.Context, alert *domain.Alert) error {
	return s.alertRepo.Create(alert)
}

// GetUnacknowledgedAlerts retrieves unacknowledged alerts
func (s *AlertService) GetUnacknowledgedAlerts(ctx context.Context, orgID uuid.UUID) ([]*domain.Alert, error) {
	return s.alertRepo.GetUnacknowledged(orgID)
}

// CountUnacknowledged returns counts for all alerts, acknowledged alerts, and unacknowledged alerts for an organization
func (s *AlertService) CountUnacknowledged(ctx context.Context, orgID uuid.UUID) (allCount, acknowledgedCount, unacknowledgedCount int, err error) {
	// Get total count of all alerts
	allCount, err = s.alertRepo.CountByOrganization(orgID)
	if err != nil {
		return 0, 0, 0, err
	}

	// Get unacknowledged alerts
	unacknowledgedAlerts, err := s.alertRepo.GetUnacknowledged(orgID)
	if err != nil {
		return 0, 0, 0, err
	}
	unacknowledgedCount = len(unacknowledgedAlerts)

	// Calculate acknowledged count
	acknowledgedCount = allCount - unacknowledgedCount

	return allCount, acknowledgedCount, unacknowledgedCount, nil
}

// CheckAPIKeyExpiry checks for expiring API keys and creates alerts
// NOTE: This method is not currently used but kept for future expansion
// when API key expiry tracking is added to the system
func (s *AlertService) CheckAPIKeyExpiry(ctx context.Context, orgID uuid.UUID) error {
	// TODO: Implement when API key repository is added to AlertService
	// For now, this is a no-op
	return nil
}

// CheckTrustScores checks for low trust scores and creates alerts
func (s *AlertService) CheckTrustScores(ctx context.Context, orgID uuid.UUID) error {
	agents, err := s.agentRepo.GetByOrganization(orgID)
	if err != nil {
		return err
	}

	lowScoreThreshold := 0.4

	for _, agent := range agents {
		if agent.TrustScore < lowScoreThreshold && agent.Status == domain.AgentStatusVerified {
			alert := &domain.Alert{
				OrganizationID: orgID,
				AlertType:      domain.AlertTrustScoreLow,
				Severity:       domain.SeverityCritical,
				Title:          fmt.Sprintf("Low Trust Score for '%s'", agent.DisplayName),
				Description:    fmt.Sprintf("Agent trust score is %.1f%%, below the recommended threshold", agent.TrustScore*100),
				ResourceType:   "agent",
				ResourceID:     agent.ID,
			}

			// Check if alert already exists
			existing, _ := s.alertRepo.GetUnacknowledged(orgID)
			exists := false
			for _, a := range existing {
				if a.ResourceID == agent.ID && a.AlertType == domain.AlertTrustScoreLow {
					exists = true
					break
				}
			}

			if !exists {
				s.alertRepo.Create(alert)
			}
		}
	}

	return nil
}

// RunProactiveChecks runs all proactive alert checks
func (s *AlertService) RunProactiveChecks(ctx context.Context, orgID uuid.UUID) error {
	if err := s.CheckAPIKeyExpiry(ctx, orgID); err != nil {
		return fmt.Errorf("API key expiry check failed: %w", err)
	}

	if err := s.CheckTrustScores(ctx, orgID); err != nil {
		return fmt.Errorf("trust score check failed: %w", err)
	}

	return nil
}

// GetAlerts retrieves alerts with filtering
func (s *AlertService) GetAlerts(
	ctx context.Context,
	orgID uuid.UUID,
	severity string,
	status string,
	limit int,
	offset int,

) ([]*domain.Alert, int, error) {
	// Use filtered repository methods if status is provided
	alerts, err := s.alertRepo.GetByOrganizationFiltered(orgID, status, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.alertRepo.CountByOrganizationFiltered(orgID, status)
	if err != nil {
		return alerts, 0, fmt.Errorf("failed to get total alerts: %w", err)
	}
	return alerts, total, nil
}

// AcknowledgeAlert acknowledges an alert
func (s *AlertService) AcknowledgeAlert(
	ctx context.Context,
	alertID uuid.UUID,
	orgID uuid.UUID,
	userID uuid.UUID,
) error {
	return s.alertRepo.Acknowledge(alertID, userID)
}

// BulkAcknowledgeAlerts acknowledges multiple alerts in one request
func (s *AlertService) BulkAcknowledgeAlerts(
	ctx context.Context,
	orgID uuid.UUID,
	userID uuid.UUID,
) (int, error) {
	return s.alertRepo.BulkAcknowledge(orgID, userID)
}

// ResolveAlert marks an alert as resolved
func (s *AlertService) ResolveAlert(
	ctx context.Context,
	alertID uuid.UUID,
	orgID uuid.UUID,
	userID uuid.UUID,
	resolution string,
) error {
	// For now, just acknowledge it
	// TODO: Add a resolved status to the domain model
	return s.alertRepo.Acknowledge(alertID, userID)
}

// ApproveDriftRequest contains the request data for approving drift
type ApproveDriftRequest struct {
	AlertID            uuid.UUID `json:"alertId"`
	OrganizationID     uuid.UUID `json:"organizationId"`
	UserID             uuid.UUID `json:"userId"`
	ApprovedMCPServers []string  `json:"approvedMcpServers"`
}

// ============================================================================
// UNUSUAL ACCESS PATTERN DETECTION
// ============================================================================

// UnusualAccessPatternConfig defines thresholds for anomaly detection
type UnusualAccessPatternConfig struct {
	HighVolumeThreshold    int           // Number of requests that triggers high volume alert
	TimeWindowMinutes      int           // Time window for rate limiting checks
	OffHoursStart          int           // Hour when off-hours begin (e.g., 22 = 10 PM)
	OffHoursEnd            int           // Hour when off-hours end (e.g., 6 = 6 AM)
	NewResourceAlertDelay  time.Duration // Don't alert on new resources within this period
}

// DefaultUnusualAccessConfig returns default configuration
func DefaultUnusualAccessConfig() UnusualAccessPatternConfig {
	return UnusualAccessPatternConfig{
		HighVolumeThreshold:    100,             // 100+ requests in window
		TimeWindowMinutes:      5,               // 5-minute window
		OffHoursStart:          22,              // 10 PM
		OffHoursEnd:            6,               // 6 AM
		NewResourceAlertDelay:  24 * time.Hour,  // Don't alert for 24h on new resources
	}
}

// DetectUnusualAccessPatterns checks for anomalous agent behavior
func (s *AlertService) DetectUnusualAccessPatterns(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID) ([]*domain.Alert, error) {
	if s.db == nil {
		fmt.Printf("📊 [ANOMALY-DETECTION] Skipped: DB not configured (orgID=%s, agentID=%s)\n", orgID, agentID)
		return nil, nil // DB not configured, skip detection
	}

	config := DefaultUnusualAccessConfig()
	var alerts []*domain.Alert

	fmt.Printf("📊 [ANOMALY-DETECTION] Starting checks for agent %s in org %s (config: volume=%d/%dmin, offHours=%d:00-%d:00)\n",
		agentID, orgID, config.HighVolumeThreshold, config.TimeWindowMinutes, config.OffHoursStart, config.OffHoursEnd)

	// 1. Check for high volume of requests
	highVolumeAlert, err := s.checkHighVolumeAccess(ctx, orgID, agentID, config)
	if err != nil {
		fmt.Printf("⚠️  [ANOMALY-DETECTION] High volume check failed: %v\n", err)
	} else if highVolumeAlert != nil {
		fmt.Printf("🚨 [ANOMALY-DETECTION] HIGH VOLUME DETECTED: Agent %s made excessive requests (severity: %s)\n",
			agentID, highVolumeAlert.Severity)
		alerts = append(alerts, highVolumeAlert)
	}

	// 2. Check for off-hours access
	offHoursAlert, err := s.checkOffHoursAccess(ctx, orgID, agentID, config)
	if err != nil {
		fmt.Printf("⚠️  [ANOMALY-DETECTION] Off-hours check failed: %v\n", err)
	} else if offHoursAlert != nil {
		fmt.Printf("🌙 [ANOMALY-DETECTION] OFF-HOURS ACCESS: Agent %s active during unusual hours (severity: %s)\n",
			agentID, offHoursAlert.Severity)
		alerts = append(alerts, offHoursAlert)
	}

	// 3. Check for unusual resource access
	resourceAlerts, err := s.checkUnusualResourceAccess(ctx, orgID, agentID, config)
	if err != nil {
		fmt.Printf("⚠️  [ANOMALY-DETECTION] Resource access check failed: %v\n", err)
	} else if len(resourceAlerts) > 0 {
		fmt.Printf("📂 [ANOMALY-DETECTION] UNUSUAL RESOURCES: Agent %s accessed %d new resources (severity: info)\n",
			agentID, len(resourceAlerts))
		alerts = append(alerts, resourceAlerts...)
	}

	// 4. Check for failed verification spike
	failedAlert, err := s.checkFailedVerificationSpike(ctx, orgID, agentID, config)
	if err != nil {
		fmt.Printf("⚠️  [ANOMALY-DETECTION] Failed verification check failed: %v\n", err)
	} else if failedAlert != nil {
		fmt.Printf("❌ [ANOMALY-DETECTION] FAILED VERIFICATION SPIKE: Agent %s has high failure rate (severity: %s)\n",
			agentID, failedAlert.Severity)
		alerts = append(alerts, failedAlert)
	}

	// Create all detected alerts
	alertsCreated := 0
	alertsSkipped := 0
	for _, alert := range alerts {
		// Check if similar alert already exists
		existing, _ := s.alertRepo.GetUnacknowledged(orgID)
		exists := false
		for _, a := range existing {
			if a.ResourceID == alert.ResourceID && a.AlertType == alert.AlertType {
				exists = true
				break
			}
		}
		if !exists {
			if err := s.alertRepo.Create(alert); err != nil {
				fmt.Printf("⚠️  [ANOMALY-DETECTION] Failed to create alert: %v\n", err)
			} else {
				alertsCreated++
				fmt.Printf("✅ [ANOMALY-DETECTION] Alert created: type=%s, severity=%s, title='%s'\n",
					alert.AlertType, alert.Severity, alert.Title)
			}
		} else {
			alertsSkipped++
		}
	}

	fmt.Printf("📊 [ANOMALY-DETECTION] Completed for agent %s: %d anomalies detected, %d alerts created, %d skipped (duplicate)\n",
		agentID, len(alerts), alertsCreated, alertsSkipped)

	return alerts, nil
}

// checkHighVolumeAccess detects unusually high request volumes
func (s *AlertService) checkHighVolumeAccess(ctx context.Context, orgID, agentID uuid.UUID, config UnusualAccessPatternConfig) (*domain.Alert, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM verification_events
		WHERE agent_id = $1
		AND created_at >= NOW() - INTERVAL '1 minute' * $2
	`, agentID, config.TimeWindowMinutes).Scan(&count)

	if err != nil {
		return nil, err
	}

	if count >= config.HighVolumeThreshold {
		agent, _ := s.agentRepo.GetByID(agentID)
		agentName := "Unknown Agent"
		if agent != nil {
			agentName = agent.DisplayName
		}

		return &domain.Alert{
			OrganizationID: orgID,
			AlertType:      domain.AlertUnusualActivity,
			Severity:       domain.AlertSeverityWarning,
			Title:          fmt.Sprintf("High Volume Access Pattern Detected for '%s'", agentName),
			Description:    fmt.Sprintf("Agent made %d verification requests in %d minutes (threshold: %d). This may indicate automated abuse or misconfiguration.", count, config.TimeWindowMinutes, config.HighVolumeThreshold),
			ResourceType:   "agent",
			ResourceID:     agentID,
		}, nil
	}

	return nil, nil
}

// checkOffHoursAccess detects access during unusual hours
func (s *AlertService) checkOffHoursAccess(ctx context.Context, orgID, agentID uuid.UUID, config UnusualAccessPatternConfig) (*domain.Alert, error) {
	currentHour := time.Now().Hour()
	isOffHours := currentHour >= config.OffHoursStart || currentHour < config.OffHoursEnd

	if !isOffHours {
		return nil, nil
	}

	// Check if agent has activity in the last 5 minutes during off-hours
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM verification_events
		WHERE agent_id = $1
		AND created_at >= NOW() - INTERVAL '5 minutes'
	`, agentID).Scan(&count)

	if err != nil {
		return nil, err
	}

	if count > 0 {
		agent, _ := s.agentRepo.GetByID(agentID)
		agentName := "Unknown Agent"
		if agent != nil {
			agentName = agent.DisplayName
		}

		return &domain.Alert{
			OrganizationID: orgID,
			AlertType:      domain.AlertUnusualActivity,
			Severity:       domain.AlertSeverityWarning,
			Title:          fmt.Sprintf("Off-Hours Activity Detected for '%s'", agentName),
			Description:    fmt.Sprintf("Agent is active during off-hours (%02d:00-%02d:00). Verify this is expected behavior.", config.OffHoursStart, config.OffHoursEnd),
			ResourceType:   "agent",
			ResourceID:     agentID,
		}, nil
	}

	return nil, nil
}

// checkUnusualResourceAccess detects access to resources the agent hasn't used before
func (s *AlertService) checkUnusualResourceAccess(ctx context.Context, orgID, agentID uuid.UUID, config UnusualAccessPatternConfig) ([]*domain.Alert, error) {
	// Find resources accessed in the last hour that weren't accessed in the previous 7 days
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT resource_type, resource_id
		FROM verification_events
		WHERE agent_id = $1
		AND created_at >= NOW() - INTERVAL '1 hour'
		AND resource_type IS NOT NULL
		AND (resource_type, resource_id) NOT IN (
			SELECT DISTINCT resource_type, resource_id
			FROM verification_events
			WHERE agent_id = $1
			AND created_at >= NOW() - INTERVAL '7 days'
			AND created_at < NOW() - INTERVAL '1 hour'
			AND resource_type IS NOT NULL
		)
	`, agentID)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var alerts []*domain.Alert
	agent, _ := s.agentRepo.GetByID(agentID)
	agentName := "Unknown Agent"
	if agent != nil {
		agentName = agent.DisplayName
	}

	for rows.Next() {
		var resourceType, resourceID sql.NullString
		if err := rows.Scan(&resourceType, &resourceID); err != nil {
			continue
		}

		if resourceType.Valid {
			alerts = append(alerts, &domain.Alert{
				OrganizationID: orgID,
				AlertType:      domain.AlertUnusualActivity,
				Severity:       domain.AlertSeverityInfo,
				Title:          fmt.Sprintf("New Resource Access Pattern for '%s'", agentName),
				Description:    fmt.Sprintf("Agent accessed resource type '%s' for the first time in 7 days. Review if this access is authorized.", resourceType.String),
				ResourceType:   "agent",
				ResourceID:     agentID,
			})
		}
	}

	return alerts, nil
}

// checkFailedVerificationSpike detects sudden increase in failed verifications
func (s *AlertService) checkFailedVerificationSpike(ctx context.Context, orgID, agentID uuid.UUID, config UnusualAccessPatternConfig) (*domain.Alert, error) {
	var recentFailed, totalRecent int

	// Count failed and total verifications in last 5 minutes
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE status = 'failed') as failed_count,
			COUNT(*) as total_count
		FROM verification_events
		WHERE agent_id = $1
		AND created_at >= NOW() - INTERVAL '5 minutes'
	`, agentID).Scan(&recentFailed, &totalRecent)

	if err != nil {
		return nil, err
	}

	// Alert if more than 50% failures with at least 5 attempts
	if totalRecent >= 5 && float64(recentFailed)/float64(totalRecent) > 0.5 {
		agent, _ := s.agentRepo.GetByID(agentID)
		agentName := "Unknown Agent"
		if agent != nil {
			agentName = agent.DisplayName
		}

		return &domain.Alert{
			OrganizationID: orgID,
			AlertType:      domain.AlertSecurityBreach,
			Severity:       domain.AlertSeverityCritical,
			Title:          fmt.Sprintf("High Failure Rate Detected for '%s'", agentName),
			Description:    fmt.Sprintf("Agent has %d failed verifications out of %d attempts (%.0f%% failure rate) in the last 5 minutes. This may indicate credential compromise or misconfiguration.", recentFailed, totalRecent, float64(recentFailed)/float64(totalRecent)*100),
			ResourceType:   "agent",
			ResourceID:     agentID,
		}, nil
	}

	return nil, nil
}

// ============================================================================
// TRUST SCORE DROP DETECTION
// ============================================================================

// TrustScoreDropConfig defines thresholds for trust score drop detection
type TrustScoreDropConfig struct {
	SignificantDropThreshold float64 // Percentage drop to trigger warning (e.g., 0.1 = 10%)
	CriticalDropThreshold    float64 // Percentage drop to trigger critical alert (e.g., 0.2 = 20%)
	LowScoreThreshold        float64 // Absolute score below which any drop is concerning
}

// DefaultTrustScoreDropConfig returns default configuration
func DefaultTrustScoreDropConfig() TrustScoreDropConfig {
	return TrustScoreDropConfig{
		SignificantDropThreshold: 0.1,  // 10% drop
		CriticalDropThreshold:    0.2,  // 20% drop
		LowScoreThreshold:        0.5,  // 50% trust score
	}
}

// CheckTrustScoreDrop creates an alert if trust score dropped significantly
// previousScore: the agent's trust score before the change
// currentScore: the agent's trust score after the change
func (s *AlertService) CheckTrustScoreDrop(ctx context.Context, orgID uuid.UUID, agentID uuid.UUID, agentName string, previousScore, currentScore float64) error {
	config := DefaultTrustScoreDropConfig()

	// Calculate the drop percentage relative to previous score
	if previousScore <= 0 {
		return nil // No meaningful comparison possible
	}

	drop := previousScore - currentScore
	dropPercentage := drop / previousScore

	// Determine if alert is needed and its severity
	var alert *domain.Alert

	// Critical drop (>20% drop OR new score below 50%)
	if dropPercentage >= config.CriticalDropThreshold || (drop > 0 && currentScore < config.LowScoreThreshold) {
		alert = &domain.Alert{
			OrganizationID: orgID,
			AlertType:      domain.AlertTrustScoreDrop,
			Severity:       domain.AlertSeverityCritical,
			Title:          fmt.Sprintf("Critical Trust Score Drop for '%s'", agentName),
			Description:    fmt.Sprintf("Agent trust score dropped from %.1f%% to %.1f%% (%.1f%% decrease). This may indicate a security issue or policy violation.", previousScore*100, currentScore*100, drop*100),
			ResourceType:   "agent",
			ResourceID:     agentID,
		}
	} else if dropPercentage >= config.SignificantDropThreshold {
		// Significant drop (>10% drop)
		alert = &domain.Alert{
			OrganizationID: orgID,
			AlertType:      domain.AlertTrustScoreDrop,
			Severity:       domain.AlertSeverityWarning,
			Title:          fmt.Sprintf("Trust Score Drop Detected for '%s'", agentName),
			Description:    fmt.Sprintf("Agent trust score dropped from %.1f%% to %.1f%% (%.1f%% decrease). Monitor this agent's behavior.", previousScore*100, currentScore*100, drop*100),
			ResourceType:   "agent",
			ResourceID:     agentID,
		}
	}

	if alert == nil {
		return nil // No significant drop
	}

	// Check if similar alert already exists (avoid duplicates)
	existing, _ := s.alertRepo.GetUnacknowledged(orgID)
	for _, a := range existing {
		if a.ResourceID == agentID && a.AlertType == domain.AlertTrustScoreDrop {
			// Alert already exists, don't create duplicate
			return nil
		}
	}

	return s.alertRepo.Create(alert)
}

// ApproveDrift approves configuration drift by updating the agent's registered configuration
// This resolves the alert and updates the agent's talks_to array
func (s *AlertService) ApproveDrift(ctx context.Context, req *ApproveDriftRequest) error {
	// 1. Get the alert to find the agent
	alert, err := s.alertRepo.GetByID(req.AlertID)
	if err != nil {
		return fmt.Errorf("failed to get alert: %w", err)
	}

	// 2. Verify alert is a configuration drift alert
	if alert.AlertType != domain.AlertTypeConfigurationDrift {
		return fmt.Errorf("alert is not a configuration drift alert")
	}

	// 3. Verify alert belongs to the organization
	if alert.OrganizationID != req.OrganizationID {
		return fmt.Errorf("alert does not belong to organization")
	}

	// 4. Get the agent
	agent, err := s.agentRepo.GetByID(alert.ResourceID)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}

	// 5. Update agent's talks_to array (merge with approved MCP servers)
	if len(req.ApprovedMCPServers) > 0 {
		// Merge unique values
		mcpServersMap := make(map[string]bool)
		for _, mcp := range agent.TalksTo {
			mcpServersMap[mcp] = true
		}
		for _, mcp := range req.ApprovedMCPServers {
			mcpServersMap[mcp] = true
		}

		// Convert back to slice
		newTalksTo := make([]string, 0, len(mcpServersMap))
		for mcp := range mcpServersMap {
			newTalksTo = append(newTalksTo, mcp)
		}
		agent.TalksTo = newTalksTo
	}

	// 6. Update agent in database
	if err := s.agentRepo.Update(agent); err != nil {
		return fmt.Errorf("failed to update agent: %w", err)
	}

	// 7. Acknowledge the alert
	if err := s.alertRepo.Acknowledge(req.AlertID, req.UserID); err != nil {
		return fmt.Errorf("failed to acknowledge alert: %w", err)
	}

	return nil
}
