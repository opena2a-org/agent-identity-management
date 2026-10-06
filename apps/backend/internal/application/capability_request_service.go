package application

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/infrastructure/repository"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/record/transition"
)

type CapabilityRequestService struct {
	requestRepo    domain.CapabilityRequestRepository
	capabilityRepo domain.CapabilityRepository
	agentRepo      domain.AgentRepository
	orgRepo        domain.OrganizationRepository
	transitions    *transition.Recorder // Optional: set by SetTransitionRecorder
}

func NewCapabilityRequestService(
	requestRepo domain.CapabilityRequestRepository,
	capabilityRepo domain.CapabilityRepository,
	agentRepo domain.AgentRepository,
	orgRepo domain.OrganizationRepository,
) *CapabilityRequestService {
	return &CapabilityRequestService{
		requestRepo:    requestRepo,
		capabilityRepo: capabilityRepo,
		agentRepo:      agentRepo,
		orgRepo:        orgRepo,
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

	// Check if organization is in monitoring mode for auto-approval
	isMonitoringMode := false
	if s.orgRepo != nil {
		org, orgErr := s.orgRepo.GetByID(agent.OrganizationID)
		if orgErr == nil && org != nil {
			isMonitoringMode = org.EnforcementMode == domain.EnforcementModeMonitoring
		}
	}

	// Create the request
	request := &domain.CapabilityRequest{
		AgentID:        input.AgentID,
		CapabilityType: input.CapabilityType,
		Reason:         input.Reason,
		Metadata:       input.Metadata,
		RequestedBy:    input.RequestedBy,
	}

	if isMonitoringMode && s.transitions != nil {
		return s.autoApprove(ctx, agent, request)
	}

	if err := s.requestRepo.Create(request); err != nil {
		return nil, fmt.Errorf("failed to create capability request: %w", err)
	}

	// In monitoring mode, auto-approve the request immediately
	if isMonitoringMode {
		// Auto-approve: update status and grant capability
		if err := s.requestRepo.UpdateStatus(request.ID, domain.CapabilityRequestStatusAutoApproved, input.RequestedBy); err != nil {
			fmt.Printf("⚠️ Failed to auto-approve capability request: %v\n", err)
		} else {
			// Grant the capability
			capability := &domain.AgentCapability{
				AgentID:        request.AgentID,
				CapabilityType: request.CapabilityType,
				GrantedBy:      &input.RequestedBy,
				GrantedAt:      time.Now(),
			}
			if err := s.capabilityRepo.CreateCapability(capability); err != nil {
				fmt.Printf("⚠️ Failed to grant auto-approved capability: %v\n", err)
			} else {
				request.Status = domain.CapabilityRequestStatusAutoApproved
				fmt.Printf("✅ Capability request AUTO-APPROVED (monitoring mode): agent=%s, capability=%s\n",
					agent.Name, input.CapabilityType)
				return request, nil
			}
		}
	}

	fmt.Printf("✅ Capability request created: agent=%s, capability=%s, reason=%s\n",
		agent.Name, input.CapabilityType, input.Reason)

	return request, nil
}

// autoApprove creates request as automatically approved and grants its
// capability, in one transaction with the request_auto_approved record. The
// actor is the one the context names, else the requesting user.
func (s *CapabilityRequestService) autoApprove(ctx context.Context, agent *domain.Agent, request *domain.CapabilityRequest) (*domain.CapabilityRequest, error) {
	requester := request.RequestedBy
	capability := &domain.AgentCapability{
		AgentID:        request.AgentID,
		CapabilityType: request.CapabilityType,
		GrantedBy:      &requester,
		GrantedAt:      time.Now(),
	}
	err := recordAgentChange(ctx, s.transitions, agent, transition.TriggerRequestAutoApproved, requesterActor(requester),
		func(ctx context.Context, tx *sql.Tx) error {
			if err := repository.CreateCapabilityRequestTx(ctx, tx, request); err != nil {
				return fmt.Errorf("create capability request: %w", err)
			}
			if err := repository.DecideCapabilityRequestTx(ctx, tx, request.ID, domain.CapabilityRequestStatusAutoApproved, requester); err != nil {
				return err
			}
			return repository.CreateCapabilityTx(ctx, tx, capability)
		})
	if err != nil {
		return nil, fmt.Errorf("failed to auto-approve capability request: %w", err)
	}
	request.Status = domain.CapabilityRequestStatusAutoApproved
	fmt.Printf("✅ Capability request AUTO-APPROVED (monitoring mode): agent=%s, capability=%s\n",
		agent.Name, request.CapabilityType)
	return request, nil
}

// requesterActor is the user who asked for a capability, or the system when
// the request names none.
func requesterActor(requester uuid.UUID) transition.Actor {
	if requester == uuid.Nil {
		return transition.System()
	}
	return transition.User(requester)
}

// decideRequest moves a pending request to status, and runs grant (when set)
// in the same transaction, together with the decision's record. The actor is
// the reviewer.
func (s *CapabilityRequestService) decideRequest(
	ctx context.Context,
	request *domain.CapabilityRequestWithDetails,
	status domain.CapabilityRequestStatus,
	trigger transition.Trigger,
	reviewerID uuid.UUID,
	grant *domain.AgentCapability,
) error {
	agent, err := s.agentRepo.GetByID(request.AgentID)
	if err != nil {
		return fmt.Errorf("agent not found: %w", err)
	}
	return recordAgentChange(transition.WithActor(ctx, transition.User(reviewerID)), s.transitions, agent, trigger,
		transition.User(reviewerID),
		func(ctx context.Context, tx *sql.Tx) error {
			if err := repository.DecideCapabilityRequestTx(ctx, tx, request.ID, status, reviewerID); err != nil {
				return err
			}
			if grant == nil {
				return nil
			}
			return repository.CreateCapabilityTx(ctx, tx, grant)
		})
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

	// Grant the capability to the agent
	capability := &domain.AgentCapability{
		AgentID:        request.AgentID,
		CapabilityType: request.CapabilityType,
		GrantedBy:      &reviewerID,
		GrantedAt:      time.Now(),
	}

	if s.transitions != nil {
		// The approval and the grant commit together or not at all.
		if err := s.decideRequest(ctx, request, domain.CapabilityRequestStatusApproved,
			transition.TriggerRequestApproved, reviewerID, capability); err != nil {
			return fmt.Errorf("failed to approve capability request: %w", err)
		}
		fmt.Printf("✅ Capability request approved and capability granted: agent=%s, capability=%s, reviewer=%s\n",
			request.AgentName, request.CapabilityType, reviewerID)
		return nil
	}

	// Update request status to approved
	if err := s.requestRepo.UpdateStatus(id, domain.CapabilityRequestStatusApproved, reviewerID); err != nil {
		return fmt.Errorf("failed to approve capability request: %w", err)
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

	// Update request status to rejected. With a recorder, the rejection is
	// recorded as a transition that leaves the agent's state as it was.
	if s.transitions != nil {
		if err := s.decideRequest(ctx, request, domain.CapabilityRequestStatusRejected,
			transition.TriggerRequestRejected, reviewerID, nil); err != nil {
			return fmt.Errorf("failed to reject capability request: %w", err)
		}
	} else if err := s.requestRepo.UpdateStatus(id, domain.CapabilityRequestStatusRejected, reviewerID); err != nil {
		return fmt.Errorf("failed to reject capability request: %w", err)
	}

	fmt.Printf("❌ Capability request rejected: agent=%s, capability=%s, reviewer=%s\n",
		request.AgentName, request.CapabilityType, reviewerID)

	return nil
}
