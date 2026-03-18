package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCapabilityRequestRepository implements CapabilityRequestRepository
type MockCapabilityRequestRepository struct {
	requests          map[uuid.UUID]*domain.CapabilityRequestWithDetails
	createErr         error
	getByIDErr        error
	listErr           error
	updateStatusErr   error
}

func NewMockCapabilityRequestRepository() *MockCapabilityRequestRepository {
	return &MockCapabilityRequestRepository{
		requests: make(map[uuid.UUID]*domain.CapabilityRequestWithDetails),
	}
}

func (m *MockCapabilityRequestRepository) Create(req *domain.CapabilityRequest) error {
	if m.createErr != nil {
		return m.createErr
	}
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	req.Status = domain.CapabilityRequestStatusPending
	req.RequestedAt = time.Now()
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	m.requests[req.ID] = &domain.CapabilityRequestWithDetails{
		CapabilityRequest: *req,
	}
	return nil
}

func (m *MockCapabilityRequestRepository) GetByID(id uuid.UUID) (*domain.CapabilityRequestWithDetails, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	req, ok := m.requests[id]
	if !ok {
		return nil, errors.New("capability request not found")
	}
	return req, nil
}

func (m *MockCapabilityRequestRepository) List(filter domain.CapabilityRequestFilter) ([]*domain.CapabilityRequestWithDetails, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*domain.CapabilityRequestWithDetails
	for _, req := range m.requests {
		// Apply filters
		if filter.AgentID != nil && req.AgentID != *filter.AgentID {
			continue
		}
		if filter.Status != nil && req.Status != *filter.Status {
			continue
		}
		result = append(result, req)
	}
	return result, nil
}

func (m *MockCapabilityRequestRepository) UpdateStatus(id uuid.UUID, status domain.CapabilityRequestStatus, reviewedBy uuid.UUID) error {
	if m.updateStatusErr != nil {
		return m.updateStatusErr
	}
	req, ok := m.requests[id]
	if !ok {
		return errors.New("capability request not found")
	}
	req.Status = status
	req.ReviewedBy = &reviewedBy
	now := time.Now()
	req.ReviewedAt = &now
	req.UpdatedAt = now
	return nil
}

func (m *MockCapabilityRequestRepository) Delete(id uuid.UUID) error {
	delete(m.requests, id)
	return nil
}

// MockCapabilityRepoForCapReq implements CapabilityRepository for capability request tests
type MockCapabilityRepoForCapReq struct {
	capabilities          map[uuid.UUID]*domain.AgentCapability
	capabilitiesByAgent   map[uuid.UUID][]*domain.AgentCapability
	createErr             error
}

func NewMockCapabilityRepoForCapReq() *MockCapabilityRepoForCapReq {
	return &MockCapabilityRepoForCapReq{
		capabilities:        make(map[uuid.UUID]*domain.AgentCapability),
		capabilitiesByAgent: make(map[uuid.UUID][]*domain.AgentCapability),
	}
}

func (m *MockCapabilityRepoForCapReq) CreateCapability(cap *domain.AgentCapability) error {
	if m.createErr != nil {
		return m.createErr
	}
	if cap.ID == uuid.Nil {
		cap.ID = uuid.New()
	}
	cap.CreatedAt = time.Now()
	cap.UpdatedAt = time.Now()
	m.capabilities[cap.ID] = cap
	m.capabilitiesByAgent[cap.AgentID] = append(m.capabilitiesByAgent[cap.AgentID], cap)
	return nil
}

func (m *MockCapabilityRepoForCapReq) GetCapabilityByID(id uuid.UUID) (*domain.AgentCapability, error) {
	cap, ok := m.capabilities[id]
	if !ok {
		return nil, errors.New("capability not found")
	}
	return cap, nil
}

func (m *MockCapabilityRepoForCapReq) GetCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	return m.capabilitiesByAgent[agentID], nil
}

func (m *MockCapabilityRepoForCapReq) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	var result []*domain.AgentCapability
	for _, cap := range m.capabilitiesByAgent[agentID] {
		if cap.RevokedAt == nil {
			result = append(result, cap)
		}
	}
	return result, nil
}

func (m *MockCapabilityRepoForCapReq) RevokeCapability(id uuid.UUID, revokedAt time.Time) error {
	cap, ok := m.capabilities[id]
	if !ok {
		return errors.New("capability not found")
	}
	cap.RevokedAt = &revokedAt
	return nil
}

func (m *MockCapabilityRepoForCapReq) DeleteCapability(id uuid.UUID) error {
	delete(m.capabilities, id)
	return nil
}

func (m *MockCapabilityRepoForCapReq) CreateViolation(violation *domain.CapabilityViolation) error {
	return nil
}

func (m *MockCapabilityRepoForCapReq) GetViolationByID(id uuid.UUID) (*domain.CapabilityViolation, error) {
	return nil, nil
}

func (m *MockCapabilityRepoForCapReq) GetViolationsByAgentID(agentID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	return nil, 0, nil
}

func (m *MockCapabilityRepoForCapReq) GetRecentViolations(orgID uuid.UUID, minutes int) ([]*domain.CapabilityViolation, error) {
	return nil, nil
}

func (m *MockCapabilityRepoForCapReq) GetViolationsByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	return nil, 0, nil
}

func (m *MockCapabilityRepoForCapReq) ListCapabilityDefinitions(orgID *uuid.UUID) ([]*domain.CapabilityDefinition, error) {
	return nil, nil
}

func (m *MockCapabilityRepoForCapReq) GetCapabilityDefinition(namespace, action string, orgID *uuid.UUID) (*domain.CapabilityDefinition, error) {
	return nil, nil
}

func (m *MockCapabilityRepoForCapReq) CreateCapabilityDefinition(def *domain.CapabilityDefinition) error {
	return nil
}

func (m *MockCapabilityRepoForCapReq) UpdateCapabilityDefinition(def *domain.CapabilityDefinition) error {
	return nil
}

// MockAgentRepositoryForCapReq implements AgentRepository for capability request tests
type MockAgentRepositoryForCapReq struct {
	agents    map[uuid.UUID]*domain.Agent
	getByIDErr error
}

func NewMockAgentRepositoryForCapReq() *MockAgentRepositoryForCapReq {
	return &MockAgentRepositoryForCapReq{
		agents: make(map[uuid.UUID]*domain.Agent),
	}
}

func (m *MockAgentRepositoryForCapReq) AddAgent(agent *domain.Agent) {
	m.agents[agent.ID] = agent
}

func (m *MockAgentRepositoryForCapReq) GetByID(id uuid.UUID) (*domain.Agent, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	agent, ok := m.agents[id]
	if !ok {
		return nil, errors.New("agent not found")
	}
	return agent, nil
}

// Implement remaining AgentRepository methods as no-ops
func (m *MockAgentRepositoryForCapReq) Create(agent *domain.Agent) error { return nil }
func (m *MockAgentRepositoryForCapReq) Update(agent *domain.Agent) error { return nil }
func (m *MockAgentRepositoryForCapReq) Delete(id uuid.UUID) error { return nil }
func (m *MockAgentRepositoryForCapReq) List(limit, offset int) ([]*domain.Agent, error) { return nil, nil }
func (m *MockAgentRepositoryForCapReq) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) { return nil, nil }
func (m *MockAgentRepositoryForCapReq) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) { return nil, nil }
func (m *MockAgentRepositoryForCapReq) UpdateTrustScore(id uuid.UUID, newScore float64) error { return nil }
func (m *MockAgentRepositoryForCapReq) IncrementViolationCount(id uuid.UUID) error { return nil }
func (m *MockAgentRepositoryForCapReq) MarkAsCompromised(id uuid.UUID) error { return nil }
func (m *MockAgentRepositoryForCapReq) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error { return nil }
func (m *MockAgentRepositoryForCapReq) GetStaleAgents(ctx context.Context, staleSince time.Time) ([]*domain.Agent, error) { return nil, nil }
func (m *MockAgentRepositoryForCapReq) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Agent, error) { return nil, nil }

// MockOrganizationRepositoryForCapReq implements OrganizationRepository for capability request tests
type MockOrganizationRepositoryForCapReq struct {
	orgs      map[uuid.UUID]*domain.Organization
	getByIDErr error
}

func NewMockOrganizationRepositoryForCapReq() *MockOrganizationRepositoryForCapReq {
	return &MockOrganizationRepositoryForCapReq{
		orgs: make(map[uuid.UUID]*domain.Organization),
	}
}

func (m *MockOrganizationRepositoryForCapReq) AddOrg(org *domain.Organization) {
	m.orgs[org.ID] = org
}

func (m *MockOrganizationRepositoryForCapReq) GetByID(id uuid.UUID) (*domain.Organization, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	org, ok := m.orgs[id]
	if !ok {
		return nil, errors.New("organization not found")
	}
	return org, nil
}

// Implement remaining OrganizationRepository methods as no-ops
func (m *MockOrganizationRepositoryForCapReq) Create(org *domain.Organization) error { return nil }
func (m *MockOrganizationRepositoryForCapReq) Update(org *domain.Organization) error { return nil }
func (m *MockOrganizationRepositoryForCapReq) Delete(id uuid.UUID) error { return nil }
func (m *MockOrganizationRepositoryForCapReq) List(limit, offset int) ([]*domain.Organization, error) { return nil, nil }
func (m *MockOrganizationRepositoryForCapReq) ListAll() ([]*domain.Organization, error) { return nil, nil }
func (m *MockOrganizationRepositoryForCapReq) GetByDomain(domain string) (*domain.Organization, error) { return nil, nil }

// Test cases

func TestNewCapabilityRequestService(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	assert.NotNil(t, service)
}

func TestCapabilityRequestService_CreateRequest(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	orgID := uuid.New()
	agentID := uuid.New()
	userID := uuid.New()

	// Set up test data
	org := &domain.Organization{
		ID:              orgID,
		EnforcementMode: domain.EnforcementModeStrict,
	}
	orgRepo.AddOrg(org)

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
	}
	agentRepo.AddAgent(agent)

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	input := &domain.CreateCapabilityRequestInput{
		AgentID:        agentID,
		CapabilityType: "file:read",
		Reason:         "Need to read configuration files",
		RequestedBy:    userID,
	}

	request, err := service.CreateRequest(ctx, input)
	require.NoError(t, err)
	assert.NotNil(t, request)
	assert.Equal(t, agentID, request.AgentID)
	assert.Equal(t, "file:read", request.CapabilityType)
	assert.Equal(t, domain.CapabilityRequestStatusPending, request.Status)
}

func TestCapabilityRequestService_CreateRequest_AgentNotFound(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	input := &domain.CreateCapabilityRequestInput{
		AgentID:        uuid.New(),
		CapabilityType: "file:read",
		Reason:         "Test reason",
		RequestedBy:    uuid.New(),
	}

	_, err := service.CreateRequest(ctx, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
}

func TestCapabilityRequestService_CreateRequest_CapabilityAlreadyGranted(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	orgID := uuid.New()
	agentID := uuid.New()
	userID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
	}
	agentRepo.AddAgent(agent)

	// Pre-grant the capability
	capabilityRepo.CreateCapability(&domain.AgentCapability{
		AgentID:        agentID,
		CapabilityType: "file:read",
	})

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	input := &domain.CreateCapabilityRequestInput{
		AgentID:        agentID,
		CapabilityType: "file:read",
		Reason:         "Test reason",
		RequestedBy:    userID,
	}

	_, err := service.CreateRequest(ctx, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already granted")
}

func TestCapabilityRequestService_CreateRequest_PendingRequestExists(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	orgID := uuid.New()
	agentID := uuid.New()
	userID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
	}
	agentRepo.AddAgent(agent)

	// Create a pending request
	requestRepo.Create(&domain.CapabilityRequest{
		AgentID:        agentID,
		CapabilityType: "file:read",
		Reason:         "First request",
		RequestedBy:    userID,
	})

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	input := &domain.CreateCapabilityRequestInput{
		AgentID:        agentID,
		CapabilityType: "file:read",
		Reason:         "Duplicate request",
		RequestedBy:    userID,
	}

	_, err := service.CreateRequest(ctx, input)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pending request already exists")
}

func TestCapabilityRequestService_CreateRequest_MonitoringModeAutoApproves(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	orgID := uuid.New()
	agentID := uuid.New()
	userID := uuid.New()

	// Organization in monitoring mode
	org := &domain.Organization{
		ID:              orgID,
		EnforcementMode: domain.EnforcementModeMonitoring,
	}
	orgRepo.AddOrg(org)

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
	}
	agentRepo.AddAgent(agent)

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	input := &domain.CreateCapabilityRequestInput{
		AgentID:        agentID,
		CapabilityType: "file:read",
		Reason:         "Need file access",
		RequestedBy:    userID,
	}

	request, err := service.CreateRequest(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, domain.CapabilityRequestStatusAutoApproved, request.Status)

	// Verify capability was granted
	caps, _ := capabilityRepo.GetCapabilitiesByAgentID(agentID)
	assert.Len(t, caps, 1)
	assert.Equal(t, "file:read", caps[0].CapabilityType)
}

func TestCapabilityRequestService_ListRequests(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	// Create some requests directly
	agentID1 := uuid.New()
	agentID2 := uuid.New()

	requestRepo.Create(&domain.CapabilityRequest{
		AgentID:        agentID1,
		CapabilityType: "file:read",
		Reason:         "Request 1",
		RequestedBy:    uuid.New(),
	})
	requestRepo.Create(&domain.CapabilityRequest{
		AgentID:        agentID1,
		CapabilityType: "file:write",
		Reason:         "Request 2",
		RequestedBy:    uuid.New(),
	})
	requestRepo.Create(&domain.CapabilityRequest{
		AgentID:        agentID2,
		CapabilityType: "db:read",
		Reason:         "Request 3",
		RequestedBy:    uuid.New(),
	})

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	// List all
	requests, err := service.ListRequests(ctx, domain.CapabilityRequestFilter{})
	require.NoError(t, err)
	assert.Len(t, requests, 3)

	// Filter by agent
	requests, err = service.ListRequests(ctx, domain.CapabilityRequestFilter{
		AgentID: &agentID1,
	})
	require.NoError(t, err)
	assert.Len(t, requests, 2)
}

func TestCapabilityRequestService_GetRequest(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	req := &domain.CapabilityRequest{
		AgentID:        uuid.New(),
		CapabilityType: "file:read",
		Reason:         "Test request",
		RequestedBy:    uuid.New(),
	}
	requestRepo.Create(req)

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	retrieved, err := service.GetRequest(ctx, req.ID)
	require.NoError(t, err)
	assert.Equal(t, req.ID, retrieved.ID)
	assert.Equal(t, "file:read", retrieved.CapabilityType)
}

func TestCapabilityRequestService_GetRequest_NotFound(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	_, err := service.GetRequest(ctx, uuid.New())
	assert.Error(t, err)
}

func TestCapabilityRequestService_ApproveRequest(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	agentID := uuid.New()
	reviewerID := uuid.New()

	req := &domain.CapabilityRequest{
		AgentID:        agentID,
		CapabilityType: "file:read",
		Reason:         "Test request",
		RequestedBy:    uuid.New(),
	}
	requestRepo.Create(req)

	// Need to add agent name for logging
	requestRepo.requests[req.ID].AgentName = "test-agent"

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	err := service.ApproveRequest(ctx, req.ID, reviewerID)
	require.NoError(t, err)

	// Verify status was updated
	updated, _ := requestRepo.GetByID(req.ID)
	assert.Equal(t, domain.CapabilityRequestStatusApproved, updated.Status)
	assert.Equal(t, &reviewerID, updated.ReviewedBy)

	// Verify capability was granted
	caps, _ := capabilityRepo.GetCapabilitiesByAgentID(agentID)
	assert.Len(t, caps, 1)
	assert.Equal(t, "file:read", caps[0].CapabilityType)
}

func TestCapabilityRequestService_ApproveRequest_NotPending(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	reviewerID := uuid.New()

	req := &domain.CapabilityRequest{
		AgentID:        uuid.New(),
		CapabilityType: "file:read",
		Reason:         "Test request",
		RequestedBy:    uuid.New(),
	}
	requestRepo.Create(req)

	// Mark as already approved
	requestRepo.UpdateStatus(req.ID, domain.CapabilityRequestStatusApproved, reviewerID)

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	err := service.ApproveRequest(ctx, req.ID, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
}

func TestCapabilityRequestService_ApproveRequest_CapabilityGrantFails(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	capabilityRepo.createErr = errors.New("database error")
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	agentID := uuid.New()
	reviewerID := uuid.New()

	req := &domain.CapabilityRequest{
		AgentID:        agentID,
		CapabilityType: "file:read",
		Reason:         "Test request",
		RequestedBy:    uuid.New(),
	}
	requestRepo.Create(req)
	requestRepo.requests[req.ID].AgentName = "test-agent"

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	err := service.ApproveRequest(ctx, req.ID, reviewerID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to grant capability")

	// Request should still be pending (rollback)
	updated, _ := requestRepo.GetByID(req.ID)
	assert.Equal(t, domain.CapabilityRequestStatusPending, updated.Status)
}

func TestCapabilityRequestService_RejectRequest(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	reviewerID := uuid.New()

	req := &domain.CapabilityRequest{
		AgentID:        uuid.New(),
		CapabilityType: "file:read",
		Reason:         "Test request",
		RequestedBy:    uuid.New(),
	}
	requestRepo.Create(req)
	requestRepo.requests[req.ID].AgentName = "test-agent"

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	err := service.RejectRequest(ctx, req.ID, reviewerID)
	require.NoError(t, err)

	// Verify status was updated
	updated, _ := requestRepo.GetByID(req.ID)
	assert.Equal(t, domain.CapabilityRequestStatusRejected, updated.Status)
	assert.Equal(t, &reviewerID, updated.ReviewedBy)
}

func TestCapabilityRequestService_RejectRequest_NotPending(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	reviewerID := uuid.New()

	req := &domain.CapabilityRequest{
		AgentID:        uuid.New(),
		CapabilityType: "file:read",
		Reason:         "Test request",
		RequestedBy:    uuid.New(),
	}
	requestRepo.Create(req)

	// Mark as already rejected
	requestRepo.UpdateStatus(req.ID, domain.CapabilityRequestStatusRejected, reviewerID)

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	err := service.RejectRequest(ctx, req.ID, uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not pending")
}

func TestCapabilityRequestService_RejectRequest_NotFound(t *testing.T) {
	requestRepo := NewMockCapabilityRequestRepository()
	capabilityRepo := NewMockCapabilityRepoForCapReq()
	agentRepo := NewMockAgentRepositoryForCapReq()
	orgRepo := NewMockOrganizationRepositoryForCapReq()

	service := NewCapabilityRequestService(requestRepo, capabilityRepo, agentRepo, orgRepo)
	ctx := context.Background()

	err := service.RejectRequest(ctx, uuid.New(), uuid.New())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
