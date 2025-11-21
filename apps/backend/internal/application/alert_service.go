package application

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// AlertService handles alert management
type AlertService struct {
	alertRepo domain.AlertRepository
	agentRepo domain.AgentRepository
}

// NewAlertService creates a new alert service
func NewAlertService(
	alertRepo domain.AlertRepository,
	agentRepo domain.AgentRepository,
) *AlertService {
	return &AlertService{
		alertRepo: alertRepo,
		agentRepo: agentRepo,
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
	// For now, just return organization alerts
	// TODO: Implement full filtering in repository layer
	alerts, err := s.alertRepo.GetByOrganization(orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.alertRepo.CountByOrganization(orgID)
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
