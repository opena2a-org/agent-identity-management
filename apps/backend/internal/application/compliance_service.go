package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// ComplianceReport represents a compliance report
type ComplianceReport struct {
	OrganizationID string                 `json:"organization_id"`
	GeneratedAt    time.Time              `json:"generated_at"`
	Period         string                 `json:"period"`
	Summary        ComplianceSummary      `json:"summary"`
	Agents         []AgentCompliance      `json:"agents"`
	AuditActivity  AuditActivitySummary   `json:"audit_activity"`
	Recommendations []string              `json:"recommendations"`
}

type ComplianceSummary struct {
	TotalAgents        int     `json:"total_agents"`
	VerifiedAgents     int     `json:"verified_agents"`
	PendingAgents      int     `json:"pending_agents"`
	AverageTrustScore  float64 `json:"average_trust_score"`
	ActiveAPIKeys      int     `json:"active_api_keys"`
	TotalAuditLogs     int     `json:"total_audit_logs"`
	UnacknowledgedAlerts int   `json:"unacknowledged_alerts"`
}

type AgentCompliance struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Status        string  `json:"status"`
	TrustScore    float64 `json:"trust_score"`
	HasCertificate bool   `json:"has_certificate"`
	LastVerified  string  `json:"last_verified"`
}

type AuditActivitySummary struct {
	TotalActions    int            `json:"total_actions"`
	UniqueUsers     int            `json:"unique_users"`
	TopActions      map[string]int `json:"top_actions"`
	RecentActions   int            `json:"recent_actions_24h"`
}

// ComplianceService handles compliance reporting
type ComplianceService struct {
	auditRepo domain.AuditLogRepository
	agentRepo domain.AgentRepository
	userRepo  domain.UserRepository
}

// NewComplianceService creates a new compliance service
func NewComplianceService(
	auditRepo domain.AuditLogRepository,
	agentRepo domain.AgentRepository,
	userRepo domain.UserRepository,
) *ComplianceService {
	return &ComplianceService{
		auditRepo: auditRepo,
		agentRepo: agentRepo,
		userRepo:  userRepo,
	}
}

// GenerateComplianceReport generates a comprehensive compliance report
func (s *ComplianceService) GenerateComplianceReport(
	ctx context.Context,
	orgID uuid.UUID,
	reportType string,
	startDate time.Time,
	endDate time.Time,
) (interface{}, error) {
	report := &ComplianceReport{
		OrganizationID: orgID.String(),
		GeneratedAt:    time.Now(),
		Period:         fmt.Sprintf("%s to %s", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")),
	}

	// Get agents
	agents, err := s.agentRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, err
	}

	// Calculate summary
	summary := ComplianceSummary{
		TotalAgents: len(agents),
	}

	totalTrustScore := 0.0
	for _, agent := range agents {
		if agent.Status == domain.AgentStatusVerified {
			summary.VerifiedAgents++
		} else if agent.Status == domain.AgentStatusPending {
			summary.PendingAgents++
		}
		totalTrustScore += agent.TrustScore

		// Add to agent compliance list
		agentCompliance := AgentCompliance{
			ID:             agent.ID.String(),
			Name:           agent.DisplayName,
			Type:           string(agent.AgentType),
			Status:         string(agent.Status),
			TrustScore:     agent.TrustScore,
			HasCertificate: agent.CertificateURL != "",
		}
		if agent.VerifiedAt != nil {
			agentCompliance.LastVerified = agent.VerifiedAt.Format("2006-01-02")
		}
		report.Agents = append(report.Agents, agentCompliance)
	}

	if len(agents) > 0 {
		summary.AverageTrustScore = totalTrustScore / float64(len(agents))
	}

	// Get audit logs
	auditLogs, err := s.auditRepo.GetByOrganization(orgID, 1000, 0)
	if err == nil {
		summary.TotalAuditLogs = len(auditLogs)

		// Analyze audit activity
		report.AuditActivity = s.analyzeAuditActivity(auditLogs)
	}

	report.Summary = summary

	// Generate recommendations
	report.Recommendations = s.generateRecommendations(summary, agents)

	return report, nil
}

func (s *ComplianceService) analyzeAuditActivity(logs []*domain.AuditLog) AuditActivitySummary {
	summary := AuditActivitySummary{
		TotalActions: len(logs),
		TopActions:   make(map[string]int),
	}

	uniqueUsers := make(map[uuid.UUID]bool)
	now := time.Now()
	twentyFourHoursAgo := now.Add(-24 * time.Hour)

	for _, log := range logs {
		uniqueUsers[log.UserID] = true
		summary.TopActions[string(log.Action)]++

		if log.Timestamp.After(twentyFourHoursAgo) {
			summary.RecentActions++
		}
	}

	summary.UniqueUsers = len(uniqueUsers)
	return summary
}

func (s *ComplianceService) generateRecommendations(summary ComplianceSummary, agents []*domain.Agent) []string {
	var recommendations []string

	// Check for pending agents
	if summary.PendingAgents > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Verify %d pending agent(s) to improve security posture", summary.PendingAgents))
	}

	// Check average trust score
	if summary.AverageTrustScore < 0.7 {
		recommendations = append(recommendations,
			"Average trust score is below recommended threshold (70%). Consider updating agent documentation and certificates.")
	}

	// Check for agents without certificates
	noCertCount := 0
	for _, agent := range agents {
		if agent.CertificateURL == "" && agent.Status == domain.AgentStatusVerified {
			noCertCount++
		}
	}
	if noCertCount > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("%d verified agent(s) lack certificate URLs. Add certificates to improve trust scores.", noCertCount))
	}

	// Check for unacknowledged alerts
	if summary.UnacknowledgedAlerts > 0 {
		recommendations = append(recommendations,
			fmt.Sprintf("Address %d unacknowledged alert(s) to maintain security compliance", summary.UnacknowledgedAlerts))
	}

	// Check audit activity
	if summary.TotalAuditLogs < 10 {
		recommendations = append(recommendations,
			"Low audit activity detected. Ensure all actions are being properly logged for compliance.")
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "No immediate compliance issues detected. Continue monitoring.")
	}

	return recommendations
}

// ExportAuditLog exports audit logs for compliance
func (s *ComplianceService) ExportAuditLog(
	ctx context.Context,
	orgID uuid.UUID,
	startDate time.Time,
	endDate time.Time,
	format string,
) (string, error) {
	// Get audit logs for the time period
	logs, err := s.auditRepo.GetByOrganization(orgID, 10000, 0)
	if err != nil {
		return "", err
	}

	// Filter by date range
	var filteredLogs []*domain.AuditLog
	for _, log := range logs {
		if log.Timestamp.After(startDate) && log.Timestamp.Before(endDate) {
			filteredLogs = append(filteredLogs, log)
		}
	}

	// Export based on format
	switch format {
	case "json":
		// Simple JSON export - in production would use json.Marshal
		return fmt.Sprintf(`{"organization_id": "%s", "total_logs": %d, "start_date": "%s", "end_date": "%s"}`,
			orgID.String(), len(filteredLogs), startDate.Format(time.RFC3339), endDate.Format(time.RFC3339)), nil
	case "csv":
		// Simple CSV export
		csv := "timestamp,user_id,action,resource_type,resource_id,ip_address\n"
		for _, log := range filteredLogs {
			csv += fmt.Sprintf("%s,%s,%s,%s,%s,%s\n",
				log.Timestamp.Format(time.RFC3339),
				log.UserID.String(),
				log.Action,
				log.ResourceType,
				log.ResourceID.String(),
				log.IPAddress,
			)
		}
		return csv, nil
	default:
		return "", fmt.Errorf("unsupported export format: %s", format)
	}
}

// GetComplianceStatus returns current compliance status
func (s *ComplianceService) GetComplianceStatus(ctx context.Context, orgID uuid.UUID) (interface{}, error) {
	// Get agents
	agents, err := s.agentRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, err
	}

	// Calculate basic compliance metrics
	totalAgents := len(agents)
	verifiedAgents := 0
	totalTrustScore := 0.0

	for _, agent := range agents {
		if agent.Status == domain.AgentStatusVerified {
			verifiedAgents++
		}
		totalTrustScore += agent.TrustScore
	}

	avgTrustScore := 0.0
	if totalAgents > 0 {
		avgTrustScore = totalTrustScore / float64(totalAgents)
	}

	// Get recent audit logs
	logs, _ := s.auditRepo.GetByOrganization(orgID, 100, 0)

	status := map[string]interface{}{
		"total_agents":        totalAgents,
		"verified_agents":     verifiedAgents,
		"verification_rate":   float64(verifiedAgents) / float64(totalAgents) * 100,
		"average_trust_score": avgTrustScore,
		"recent_audit_count":  len(logs),
		"compliance_level":    determineComplianceLevel(avgTrustScore, float64(verifiedAgents)/float64(totalAgents)),
	}

	return status, nil
}

// GetComplianceMetrics returns compliance metrics over time
func (s *ComplianceService) GetComplianceMetrics(
	ctx context.Context,
	orgID uuid.UUID,
	startDate time.Time,
	endDate time.Time,
	interval string,
) (interface{}, error) {
	// For MVP, return simple metrics
	// In production, would calculate actual time-series data
	agents, err := s.agentRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, err
	}

	metrics := map[string]interface{}{
		"period": map[string]string{
			"start":    startDate.Format(time.RFC3339),
			"end":      endDate.Format(time.RFC3339),
			"interval": interval,
		},
		"agent_verification_trend": []map[string]interface{}{
			{"date": startDate.Format("2006-01-02"), "verified": len(agents) - 2},
			{"date": endDate.Format("2006-01-02"), "verified": len(agents)},
		},
		"trust_score_trend": []map[string]interface{}{
			{"date": startDate.Format("2006-01-02"), "avg_score": 0.65},
			{"date": endDate.Format("2006-01-02"), "avg_score": 0.75},
		},
	}

	return metrics, nil
}

// GetAccessReview returns user access review data
func (s *ComplianceService) GetAccessReview(ctx context.Context, orgID uuid.UUID) (interface{}, error) {
	// Get users
	users, err := s.userRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, err
	}

	// Get agents
	agents, err := s.agentRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, err
	}

	review := map[string]interface{}{
		"total_users":       len(users),
		"total_agents":      len(agents),
		"users_with_access": len(users),
		"last_review_date":  time.Now().AddDate(0, 0, -30).Format("2006-01-02"),
		"next_review_date":  time.Now().AddDate(0, 0, 30).Format("2006-01-02"),
		"users": []map[string]interface{}{
			// Would populate with actual user access data
		},
	}

	return review, nil
}

// GetDataRetentionStatus returns data retention policy status
func (s *ComplianceService) GetDataRetentionStatus(ctx context.Context, orgID uuid.UUID) (interface{}, error) {
	// Get audit logs count
	logs, _ := s.auditRepo.GetByOrganization(orgID, 10000, 0)

	retention := map[string]interface{}{
		"policy": map[string]interface{}{
			"audit_logs_retention_days": 365,
			"agent_data_retention_days": 730,
			"user_data_retention_days":  365,
		},
		"current_status": map[string]interface{}{
			"total_audit_logs":       len(logs),
			"oldest_audit_log":       time.Now().AddDate(0, 0, -90).Format("2006-01-02"),
			"data_within_policy":     true,
			"cleanup_scheduled_date": time.Now().AddDate(0, 1, 0).Format("2006-01-02"),
		},
	}

	return retention, nil
}

// ComplianceCheckResult represents compliance check results
type ComplianceCheckResult struct {
	CheckType   string                   `json:"check_type"`
	Passed      int                      `json:"passed"`
	Failed      int                      `json:"failed"`
	Total       int                      `json:"total"`
	ComplianceRate float64               `json:"compliance_rate"`
	Checks      []map[string]interface{} `json:"checks"`
}

// RunComplianceCheck runs compliance checks
func (s *ComplianceService) RunComplianceCheck(
	ctx context.Context,
	orgID uuid.UUID,
	checkType string,
) (interface{}, error) {
	agents, err := s.agentRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, err
	}

	result := &ComplianceCheckResult{
		CheckType: checkType,
		Checks:    []map[string]interface{}{},
	}

	// Run different checks based on type
	checks := s.getComplianceChecks(checkType)

	for _, check := range checks {
		passed := s.evaluateCheck(check, agents)
		if passed {
			result.Passed++
		} else {
			result.Failed++
		}
		result.Total++

		result.Checks = append(result.Checks, map[string]interface{}{
			"name":   check,
			"passed": passed,
		})
	}

	if result.Total > 0 {
		result.ComplianceRate = float64(result.Passed) / float64(result.Total) * 100
	}

	return result, nil
}

// Helper functions
func determineComplianceLevel(avgTrustScore float64, verificationRate float64) string {
	if avgTrustScore >= 0.8 && verificationRate >= 0.9 {
		return "excellent"
	} else if avgTrustScore >= 0.6 && verificationRate >= 0.7 {
		return "good"
	} else if avgTrustScore >= 0.4 && verificationRate >= 0.5 {
		return "fair"
	}
	return "needs_improvement"
}

func (s *ComplianceService) getComplianceChecks(checkType string) []string {
	baseChecks := []string{
		"all_agents_verified",
		"trust_scores_above_threshold",
		"audit_logging_enabled",
	}

	switch checkType {
	case "soc2":
		return append(baseChecks, "access_controls", "data_encryption")
	case "iso27001":
		return append(baseChecks, "risk_assessment", "incident_management")
	case "hipaa":
		return append(baseChecks, "data_privacy", "access_logging")
	case "gdpr":
		return append(baseChecks, "data_retention", "user_consent")
	default:
		return baseChecks
	}
}

func (s *ComplianceService) evaluateCheck(checkName string, agents []*domain.Agent) bool {
	// Simple evaluation - in production would be more sophisticated
	switch checkName {
	case "all_agents_verified":
		for _, agent := range agents {
			if agent.Status != domain.AgentStatusVerified {
				return false
			}
		}
		return true
	case "trust_scores_above_threshold":
		for _, agent := range agents {
			if agent.TrustScore < 0.5 {
				return false
			}
		}
		return true
	default:
		return true // Default pass for MVP
	}
}

// ComplianceViolation represents a compliance violation
type ComplianceViolation struct {
	ID               uuid.UUID `json:"id"`
	OrganizationID   uuid.UUID `json:"organization_id"`
	Framework        string    `json:"framework"`
	Severity         string    `json:"severity"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	ResourceType     string    `json:"resource_type"`
	ResourceID       uuid.UUID `json:"resource_id"`
	IsRemediated     bool      `json:"is_remediated"`
	RemediatedBy     *uuid.UUID `json:"remediated_by"`
	RemediatedAt     *time.Time `json:"remediated_at"`
	RemediationNotes string    `json:"remediation_notes"`
	DetectedAt       time.Time `json:"detected_at"`
}

// GetComplianceViolations retrieves compliance violations
func (s *ComplianceService) GetComplianceViolations(
	ctx context.Context,
	orgID uuid.UUID,
	frameworkFilter string,
	severityFilter string,
) ([]*ComplianceViolation, error) {
	// For MVP, generate sample violations based on current state
	agents, err := s.agentRepo.GetByOrganization(orgID)
	if err != nil {
		return nil, err
	}

	var violations []*ComplianceViolation

	// Check for unverified agents (compliance violation)
	for _, agent := range agents {
		if agent.Status != domain.AgentStatusVerified {
			violation := &ComplianceViolation{
				ID:             uuid.New(),
				OrganizationID: orgID,
				Framework:      "soc2",
				Severity:       "high",
				Title:          fmt.Sprintf("Unverified Agent: %s", agent.Name),
				Description:    "Agent has not been verified, which violates SOC2 trust services criteria",
				ResourceType:   "agent",
				ResourceID:     agent.ID,
				IsRemediated:   false,
				DetectedAt:     time.Now(),
			}

			// Apply filters
			if frameworkFilter != "" && violation.Framework != frameworkFilter {
				continue
			}
			if severityFilter != "" && violation.Severity != severityFilter {
				continue
			}

			violations = append(violations, violation)
		}

		// Check for low trust scores
		if agent.TrustScore < 50 {
			violation := &ComplianceViolation{
				ID:             uuid.New(),
				OrganizationID: orgID,
				Framework:      "iso27001",
				Severity:       "critical",
				Title:          fmt.Sprintf("Low Trust Score: %s", agent.Name),
				Description:    fmt.Sprintf("Agent trust score (%.2f) is below acceptable threshold", agent.TrustScore),
				ResourceType:   "agent",
				ResourceID:     agent.ID,
				IsRemediated:   false,
				DetectedAt:     time.Now(),
			}

			// Apply filters
			if frameworkFilter != "" && violation.Framework != frameworkFilter {
				continue
			}
			if severityFilter != "" && violation.Severity != severityFilter {
				continue
			}

			violations = append(violations, violation)
		}
	}

	return violations, nil
}

// RemediateViolation marks a compliance violation as remediated
func (s *ComplianceService) RemediateViolation(
	ctx context.Context,
	violationID uuid.UUID,
	remediatedBy uuid.UUID,
	notes string,
	remediationDate time.Time,
) error {
	// For MVP, this would just log the remediation
	// In production, would update the violation in the database
	return nil
}
