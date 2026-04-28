package application

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
)

// Trust score penalty constants (on 0-100 scale)
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

	// driftScore is the OTel gauge keyed by agent.id. Emitted on every
	// DetectDrift call so dashboards see steady-state zeros and a spike
	// at the moment drift is observed. SemConv attribute name pinned to
	// `agent.drift_score` per the May 22 talk Slide 14.
	driftScore metric.Float64Gauge
}

// NewDriftDetectionService creates a new drift detection service
func NewDriftDetectionService(agentRepo domain.AgentRepository, alertRepo domain.AlertRepository) *DriftDetectionService {
	gauge, err := otel.Meter("aim/drift").Float64Gauge(
		"agent.drift_score",
		metric.WithDescription("Agent behavioral drift score (0-1, higher = more drift)"),
	)
	if err != nil {
		// Log via fmt to match existing service style; metric will be nil
		// and gauge emission becomes a no-op.
		fmt.Printf("⚠️  drift score gauge init failed: %v\n", err)
	}
	return &DriftDetectionService{
		agentRepo:  agentRepo,
		alertRepo:  alertRepo,
		driftScore: gauge,
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

	// Emit drift gauge on every check, including 0 for "no drift". Steady-state
	// zeros let the dashboard distinguish "agent active and clean" from "agent
	// silent" — without them, missing data points look identical to clean checks.
	s.emitDriftScore(agentID, mcpDrift, capabilityDrift)

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
		AgentName:      agent.Name, // Denormalized for display in alerts
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

// emitDriftScore computes a 0-1 score from drift counts and records it on
// the OTel gauge. The score is a tanh-shaped saturation: 0 drift → 0.0,
// 1 drift item → ~0.46, 2 → ~0.76, 5+ → ~0.99. Lets the demo dashboard
// show meaningful spikes from a single drift event.
func (s *DriftDetectionService) emitDriftScore(agentID uuid.UUID, mcpDrift, capabilityDrift []string) {
	if s.driftScore == nil {
		return
	}
	count := float64(len(mcpDrift) + len(capabilityDrift))
	score := math.Tanh(count / 2.0)
	s.driftScore.Record(context.Background(), score, metric.WithAttributes(
		attribute.String("agent.id", agentID.String()),
	))
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
