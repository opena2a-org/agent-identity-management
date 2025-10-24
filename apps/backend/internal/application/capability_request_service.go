package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
)

type CapabilityRequestService struct {
	requestRepo    domain.CapabilityRequestRepository
	capabilityRepo domain.CapabilityRepository
	agentRepo      domain.AgentRepository
}

func NewCapabilityRequestService(
	requestRepo domain.CapabilityRequestRepository,
	capabilityRepo domain.CapabilityRepository,
	agentRepo domain.AgentRepository,
) *CapabilityRequestService {
	return &CapabilityRequestService{
		requestRepo:    requestRepo,
		capabilityRepo: capabilityRepo,
		agentRepo:      agentRepo,
	}
}

// CreateRequest creates a new capability request
func (s *CapabilityRequestService) CreateRequest(ctx context.Context, input *domain.CreateCapabilityRequestInput) (*domain.CapabilityRequest, error) {
	// Verify agent exists
	agent, err := s.agentRepo.GetByID(input.AgentID)
	if err != nil {
		return nil, fmt.Errorf("agent not found: %w", err)
	}

	// Check if capability already granted
	capabilities, err := s.capabilityRepo.GetCapabilitiesByAgentID(input.AgentID)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing capabilities: %w", err)
	}

	for _, cap := range capabilities {
		if cap.CapabilityType == input.CapabilityType {
			return nil, fmt.Errorf("capability '%s' already granted to agent '%s'", input.CapabilityType, agent.Name)
		}
	}

	// Check if there's already a pending request
	existingRequests, err := s.requestRepo.List(domain.CapabilityRequestFilter{
		AgentID: &input.AgentID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to check existing requests: %w", err)
	}

	for _, req := range existingRequests {
		if req.CapabilityType == input.CapabilityType && req.Status == domain.CapabilityRequestStatusPending {
			return nil, fmt.Errorf("pending request already exists for capability '%s'", input.CapabilityType)
		}
	}

	// Create the request
	request := &domain.CapabilityRequest{
		AgentID:        input.AgentID,
		CapabilityType: input.CapabilityType,
		Reason:         input.Reason,
		RequestedBy:    input.RequestedBy,
	}

	if err := s.requestRepo.Create(request); err != nil {
		return nil, fmt.Errorf("failed to create capability request: %w", err)
	}

	fmt.Printf("✅ Capability request created: agent=%s, capability=%s, reason=%s\n",
		agent.Name, input.CapabilityType, input.Reason)

	return request, nil
}

// ListRequests lists capability requests with optional filtering
func (s *CapabilityRequestService) ListRequests(ctx context.Context, filter domain.CapabilityRequestFilter) ([]*domain.CapabilityRequestWithDetails, error) {
	requests, err := s.requestRepo.List(filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list capability requests: %w", err)
	}

	return requests, nil
}

// GetRequest retrieves a single capability request by ID
func (s *CapabilityRequestService) GetRequest(ctx context.Context, id uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
	request, err := s.requestRepo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get capability request: %w", err)
	}

	return request, nil
}

// ApproveRequest approves a capability request and grants the capability
func (s *CapabilityRequestService) ApproveRequest(ctx context.Context, id uuid.UUID, reviewerID uuid.UUID) error {
	// Get the request details
	request, err := s.requestRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("capability request not found: %w", err)
	}

	// Verify status is pending
	if request.Status != domain.CapabilityRequestStatusPending {
		return fmt.Errorf("capability request is not pending (current status: %s)", request.Status)
	}

	// Update request status to approved
	if err := s.requestRepo.UpdateStatus(id, domain.CapabilityRequestStatusApproved, reviewerID); err != nil {
		return fmt.Errorf("failed to approve capability request: %w", err)
	}

	// Grant the capability to the agent
	capability := &domain.AgentCapability{
		AgentID:        request.AgentID,
		CapabilityType: request.CapabilityType,
		GrantedBy:      &reviewerID,
		GrantedAt:      time.Now(),
	}

	if err := s.capabilityRepo.CreateCapability(capability); err != nil {
		// Rollback the approval if capability grant fails
		_ = s.requestRepo.UpdateStatus(id, domain.CapabilityRequestStatusPending, reviewerID)
		return fmt.Errorf("failed to grant capability: %w", err)
	}

	fmt.Printf("✅ Capability request approved and capability granted: agent=%s, capability=%s, reviewer=%s\n",
		request.AgentName, request.CapabilityType, reviewerID)

	return nil
}

// RejectRequest rejects a capability request
func (s *CapabilityRequestService) RejectRequest(ctx context.Context, id uuid.UUID, reviewerID uuid.UUID) error {
	// Get the request details
	request, err := s.requestRepo.GetByID(id)
	if err != nil {
		return fmt.Errorf("capability request not found: %w", err)
	}

	// Verify status is pending
	if request.Status != domain.CapabilityRequestStatusPending {
		return fmt.Errorf("capability request is not pending (current status: %s)", request.Status)
	}

	// Update request status to rejected
	if err := s.requestRepo.UpdateStatus(id, domain.CapabilityRequestStatusRejected, reviewerID); err != nil {
		return fmt.Errorf("failed to reject capability request: %w", err)
	}

	fmt.Printf("❌ Capability request rejected: agent=%s, capability=%s, reviewer=%s\n",
		request.AgentName, request.CapabilityType, reviewerID)

	return nil
}
