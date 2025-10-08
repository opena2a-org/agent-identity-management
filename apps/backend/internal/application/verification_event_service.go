package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

// VerificationEventService handles verification event business logic
type VerificationEventService struct {
	eventRepo         domain.VerificationEventRepository
	agentRepo         domain.AgentRepository
	driftDetection    *DriftDetectionService
}

// NewVerificationEventService creates a new verification event service
func NewVerificationEventService(
	eventRepo domain.VerificationEventRepository,
	agentRepo domain.AgentRepository,
	driftDetection *DriftDetectionService,
) *VerificationEventService {
	return &VerificationEventService{
		eventRepo:      eventRepo,
		agentRepo:      agentRepo,
		driftDetection: driftDetection,
	}
}

// LogVerificationEvent creates a new verification event (for automatic logging)
func (s *VerificationEventService) LogVerificationEvent(
	ctx context.Context,
	orgID uuid.UUID,
	agentID uuid.UUID,
	protocol domain.VerificationProtocol,
	verificationType domain.VerificationType,
	status domain.VerificationEventStatus,
	durationMs int,
	initiatorType domain.InitiatorType,
	initiatorID *uuid.UUID,
	metadata map[string]interface{},
) (*domain.VerificationEvent, error) {
	// Get agent details
	agent, err := s.agentRepo.GetByID(agentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	now := time.Now()
	agentIDPtr := &agentID
	agentNamePtr := &agent.DisplayName

	event := &domain.VerificationEvent{
		OrganizationID:   orgID,
		AgentID:          agentIDPtr,
		AgentName:        agentNamePtr,
		Protocol:         protocol,
		VerificationType: verificationType,
		Status:           status,
		Confidence:       calculateConfidence(status, agent.TrustScore),
		TrustScore:       agent.TrustScore,
		DurationMs:       durationMs,
		InitiatorType:    initiatorType,
		InitiatorID:      initiatorID,
		StartedAt:        now.Add(-time.Duration(durationMs) * time.Millisecond),
		CompletedAt:      &now,
		CreatedAt:        now,
		Metadata:         metadata,
	}

	if err := s.eventRepo.Create(event); err != nil {
		return nil, fmt.Errorf("failed to create verification event: %w", err)
	}

	return event, nil
}

// CreateVerificationEvent creates a manual verification event with full details
func (s *VerificationEventService) CreateVerificationEvent(
	ctx context.Context,
	req *CreateVerificationEventRequest,
) (*domain.VerificationEvent, error) {
	// Validate agent exists
	agent, err := s.agentRepo.GetByID(req.AgentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	agentIDPtr := &req.AgentID
	agentNamePtr := &agent.DisplayName

	event := &domain.VerificationEvent{
		OrganizationID:   req.OrganizationID,
		AgentID:          agentIDPtr,
		AgentName:        agentNamePtr,
		Protocol:         req.Protocol,
		VerificationType: req.VerificationType,
		Status:           req.Status,
		Result:           req.Result,
		Signature:        req.Signature,
		MessageHash:      req.MessageHash,
		Nonce:            req.Nonce,
		PublicKey:        req.PublicKey,
		Confidence:       req.Confidence,
		TrustScore:       agent.TrustScore,
		DurationMs:       req.DurationMs,
		ErrorCode:        req.ErrorCode,
		ErrorReason:      req.ErrorReason,
		InitiatorType:    req.InitiatorType,
		InitiatorID:      req.InitiatorID,
		InitiatorName:    req.InitiatorName,
		InitiatorIP:      req.InitiatorIP,
		Action:           req.Action,
		ResourceType:     req.ResourceType,
		ResourceID:       req.ResourceID,
		Location:         req.Location,
		StartedAt:        req.StartedAt,
		CompletedAt:      req.CompletedAt,
		Details:          req.Details,
		Metadata:         req.Metadata,

		// Store runtime configuration for drift tracking
		CurrentMCPServers:   req.CurrentMCPServers,
		CurrentCapabilities: req.CurrentCapabilities,
	}

	// Perform drift detection if runtime configuration provided
	if len(req.CurrentMCPServers) > 0 || len(req.CurrentCapabilities) > 0 {
		driftResult, err := s.driftDetection.DetectDrift(
			req.AgentID,
			req.CurrentMCPServers,
			req.CurrentCapabilities,
		)

		if err != nil {
			// Log error but don't fail the verification event creation
			fmt.Printf("Drift detection failed: %v\n", err)
		} else if driftResult != nil {
			// Store drift detection results in the event
			event.DriftDetected = driftResult.DriftDetected
			event.MCPServerDrift = driftResult.MCPServerDrift
			event.CapabilityDrift = driftResult.CapabilityDrift
		}
	}

	if err := s.eventRepo.Create(event); err != nil {
		return nil, fmt.Errorf("failed to create verification event: %w", err)
	}

	return event, nil
}

// GetVerificationEvent retrieves a verification event by ID
func (s *VerificationEventService) GetVerificationEvent(ctx context.Context, id uuid.UUID) (*domain.VerificationEvent, error) {
	return s.eventRepo.GetByID(id)
}

// ListVerificationEvents retrieves verification events for an organization
func (s *VerificationEventService) ListVerificationEvents(
	ctx context.Context,
	orgID uuid.UUID,
	limit, offset int,
) ([]*domain.VerificationEvent, int, error) {
	return s.eventRepo.GetByOrganization(orgID, limit, offset)
}

// ListAgentVerificationEvents retrieves verification events for a specific agent
func (s *VerificationEventService) ListAgentVerificationEvents(
	ctx context.Context,
	agentID uuid.UUID,
	limit, offset int,
) ([]*domain.VerificationEvent, int, error) {
	return s.eventRepo.GetByAgent(agentID, limit, offset)
}

// GetRecentEvents retrieves recent verification events (for real-time monitoring)
func (s *VerificationEventService) GetRecentEvents(ctx context.Context, orgID uuid.UUID, minutes int) ([]*domain.VerificationEvent, error) {
	return s.eventRepo.GetRecentEvents(orgID, minutes)
}

// GetStatistics calculates verification statistics for a time range
func (s *VerificationEventService) GetStatistics(
	ctx context.Context,
	orgID uuid.UUID,
	startTime, endTime time.Time,
) (*domain.VerificationStatistics, error) {
	return s.eventRepo.GetStatistics(orgID, startTime, endTime)
}

// GetLast24HoursStatistics calculates statistics for the last 24 hours
func (s *VerificationEventService) GetLast24HoursStatistics(ctx context.Context, orgID uuid.UUID) (*domain.VerificationStatistics, error) {
	endTime := time.Now()
	startTime := endTime.Add(-24 * time.Hour)
	return s.eventRepo.GetStatistics(orgID, startTime, endTime)
}

// DeleteVerificationEvent deletes a verification event
func (s *VerificationEventService) DeleteVerificationEvent(ctx context.Context, id uuid.UUID) error {
	return s.eventRepo.Delete(id)
}

// CreateVerificationEventRequest represents a request to create a verification event
type CreateVerificationEventRequest struct {
	OrganizationID   uuid.UUID
	AgentID          uuid.UUID
	Protocol         domain.VerificationProtocol
	VerificationType domain.VerificationType
	Status           domain.VerificationEventStatus
	Result           *domain.VerificationResult
	Signature        *string
	MessageHash      *string
	Nonce            *string
	PublicKey        *string
	Confidence       float64
	DurationMs       int
	ErrorCode        *string
	ErrorReason      *string
	InitiatorType    domain.InitiatorType
	InitiatorID      *uuid.UUID
	InitiatorName    *string
	InitiatorIP      *string
	Action           *string
	ResourceType     *string
	ResourceID       *string
	Location         *string
	StartedAt        time.Time
	CompletedAt      *time.Time
	Details          *string
	Metadata         map[string]interface{}

	// Configuration Drift Detection (WHO and WHAT)
	CurrentMCPServers    []string // Runtime: MCP servers being communicated with
	CurrentCapabilities  []string // Runtime: Capabilities being used
}

// calculateConfidence calculates confidence based on status and trust score
func calculateConfidence(status domain.VerificationEventStatus, trustScore float64) float64 {
	baseConfidence := trustScore / 100.0 // Convert 0-100 to 0-1

	switch status {
	case domain.VerificationEventStatusSuccess:
		return min(baseConfidence+0.1, 1.0) // Boost by 10%
	case domain.VerificationEventStatusFailed:
		return max(baseConfidence-0.2, 0.0) // Reduce by 20%
	case domain.VerificationEventStatusTimeout:
		return max(baseConfidence-0.3, 0.0) // Reduce by 30%
	case domain.VerificationEventStatusPending:
		return baseConfidence
	default:
		return baseConfidence
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
