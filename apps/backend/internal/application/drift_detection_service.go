package application

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// Trust score penalty constants
const (
	// FirstViolationPenalty is the penalty for first-time drift violation (-5 points)
	FirstViolationPenalty = 5.0

	// RepeatedViolationPenalty is the penalty for repeated drift violations (-10 points)
	RepeatedViolationPenalty = 10.0

	// MinimumTrustScore is the lowest trust score allowed
	MinimumTrustScore = 0.0
)

// DriftDetectionService handles configuration drift detection for agents
type DriftDetectionService struct {
	agentRepo domain.AgentRepository
	alertRepo domain.AlertRepository
}

// NewDriftDetectionService creates a new drift detection service
func NewDriftDetectionService(agentRepo domain.AgentRepository, alertRepo domain.AlertRepository) *DriftDetectionService {
	return &DriftDetectionService{
		agentRepo: agentRepo,
		alertRepo: alertRepo,
	}
}

// DriftResult contains the results of drift detection
type DriftResult struct {
	DriftDetected     bool
	MCPServerDrift    []string
	CapabilityDrift   []string
	Alert             *domain.Alert
}

// DetectDrift checks if an agent's runtime configuration drifts from registered values
func (s *DriftDetectionService) DetectDrift(
	agentID uuid.UUID,
	currentMCPServers []string,
	currentCapabilities []string,
) (*DriftResult, error) {
	// 1. Get agent's registered configuration
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	// 2. Detect MCP server drift
	mcpDrift := detectArrayDrift(agent.TalksTo, currentMCPServers)

	// 3. Detect capability drift (if agent has registered capabilities)
	// Note: Capabilities are currently stored in separate table, so this is for future use
	capabilityDrift := []string{}

	// 4. If no drift detected, return early
	if len(mcpDrift) == 0 && len(capabilityDrift) == 0 {
		return &DriftResult{
			DriftDetected:     false,
			MCPServerDrift:    []string{},
			CapabilityDrift:   []string{},
		}, nil
	}

	// 5. Drift detected - create high-severity alert
	alert, err := s.createDriftAlert(agent, mcpDrift, capabilityDrift)
	if err != nil {
		// Log error but don't fail the drift detection
		fmt.Printf("Failed to create drift alert: %v\n", err)
	}

	// 6. Apply trust score penalty
	if err := s.applyTrustScorePenalty(agent, mcpDrift, capabilityDrift); err != nil {
		// Log error but don't fail the drift detection
		fmt.Printf("Failed to apply trust score penalty: %v\n", err)
	}

	return &DriftResult{
		DriftDetected:     true,
		MCPServerDrift:    mcpDrift,
		CapabilityDrift:   capabilityDrift,
		Alert:             alert,
	}, nil
}

// createDriftAlert creates a high-severity alert for configuration drift
func (s *DriftDetectionService) createDriftAlert(
	agent *domain.Agent,
	mcpDrift []string,
	capabilityDrift []string,
) (*domain.Alert, error) {
	// Build alert message
	message := fmt.Sprintf("Agent '%s' is deviating from registered configuration.", agent.Name)

	if len(mcpDrift) > 0 {
		message += fmt.Sprintf("\n\n**Unauthorized MCP Server Communication:**\n")
		for _, mcp := range mcpDrift {
			message += fmt.Sprintf("- `%s` (not registered)\n", mcp)
		}
	}

	if len(capabilityDrift) > 0 {
		message += fmt.Sprintf("\n\n**Undeclared Capability Usage:**\n")
		for _, cap := range capabilityDrift {
			message += fmt.Sprintf("- `%s` (not declared)\n", cap)
		}
	}

	message += "\n\n**Registered Configuration:**\n"
	if len(agent.TalksTo) > 0 {
		message += "- MCP Servers: "
		for i, mcp := range agent.TalksTo {
			if i > 0 {
				message += ", "
			}
			message += fmt.Sprintf("`%s`", mcp)
		}
		message += "\n"
	} else {
		message += "- MCP Servers: None registered\n"
	}

	message += "\n**Recommended Actions:**\n"
	message += "1. Investigate why agent is using undeclared resources\n"
	message += "2. If legitimate, approve drift and update registration\n"
	message += "3. If suspicious, investigate for potential compromise\n"

	// Create alert
	alert := &domain.Alert{
		ID:             uuid.New(),
		OrganizationID: agent.OrganizationID,
		AlertType:      domain.AlertTypeConfigurationDrift,
		Severity:       domain.AlertSeverityHigh,
		Title:          fmt.Sprintf("Configuration Drift Detected: %s", agent.Name),
		Description:    message,
		ResourceType:   "agent",
		ResourceID:     agent.ID,
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}

	// Save alert
	if err := s.alertRepo.Create(alert); err != nil {
		return nil, fmt.Errorf("failed to create alert: %w", err)
	}

	return alert, nil
}

// applyTrustScorePenalty reduces agent trust score based on drift severity
func (s *DriftDetectionService) applyTrustScorePenalty(
	agent *domain.Agent,
	mcpDrift []string,
	capabilityDrift []string,
) error {
	// Calculate penalty based on violation history
	// capability_violation_count is incremented by UpdateTrustScore
	penalty := FirstViolationPenalty

	// If agent already has violations, use higher penalty
	if agent.CapabilityViolationCount > 0 {
		penalty = RepeatedViolationPenalty
	}

	// Calculate new trust score
	newScore := agent.TrustScore - penalty

	// Ensure score doesn't go below minimum
	if newScore < MinimumTrustScore {
		newScore = MinimumTrustScore
	}

	// Update agent trust score
	if err := s.agentRepo.UpdateTrustScore(agent.ID, newScore); err != nil {
		return fmt.Errorf("failed to update trust score: %w", err)
	}

	fmt.Printf("✅ Applied trust score penalty to agent %s: %.2f -> %.2f (-%0.f points)\n",
		agent.Name, agent.TrustScore, newScore, penalty)

	return nil
}

// detectArrayDrift finds items in 'runtime' that are not in 'registered'
func detectArrayDrift(registered []string, runtime []string) []string {
	if len(runtime) == 0 {
		return []string{}
	}

	// Create map of registered items for O(1) lookup
	registeredMap := make(map[string]bool)
	for _, item := range registered {
		registeredMap[item] = true
	}

	// Find drift: items in runtime but not in registered
	drift := []string{}
	for _, item := range runtime {
		if !registeredMap[item] {
			drift = append(drift, item)
		}
	}

	return drift
}
