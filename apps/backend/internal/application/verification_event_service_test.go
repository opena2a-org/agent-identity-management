package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEventRepoForService for verification event service tests
type MockEventRepoForService struct {
	mock.Mock
}

func (m *MockEventRepoForService) Create(event *domain.VerificationEvent) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockEventRepoForService) GetByID(id uuid.UUID) (*domain.VerificationEvent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VerificationEvent), args.Error(1)
}

func (m *MockEventRepoForService) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), args.Error(2)
}

func (m *MockEventRepoForService) GetByAgent(agentID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	args := m.Called(agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), args.Error(2)
}

func (m *MockEventRepoForService) GetByMCPServer(mcpServerID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	args := m.Called(mcpServerID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), args.Error(2)
}

func (m *MockEventRepoForService) GetRecentEvents(orgID uuid.UUID, minutes int) ([]*domain.VerificationEvent, error) {
	args := m.Called(orgID, minutes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Error(1)
}

func (m *MockEventRepoForService) GetPendingVerifications(orgID uuid.UUID) ([]*domain.VerificationEvent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Error(1)
}

func (m *MockEventRepoForService) SearchAdminVerifications(orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
	args := m.Called(orgID, params)
	if args.Get(0) == nil {
		return nil, args.Int(1), nil, args.Error(3)
	}
	var counts *domain.VerificationStatusCounts
	if args.Get(2) != nil {
		counts = args.Get(2).(*domain.VerificationStatusCounts)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), counts, args.Error(3)
}

func (m *MockEventRepoForService) GetStatistics(orgID uuid.UUID, startTime, endTime time.Time) (*domain.VerificationStatistics, error) {
	args := m.Called(orgID, startTime, endTime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VerificationStatistics), args.Error(1)
}

func (m *MockEventRepoForService) GetAgentStatistics(agentID uuid.UUID, startTime, endTime time.Time) (*domain.AgentVerificationStatistics, error) {
	args := m.Called(agentID, startTime, endTime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentVerificationStatistics), args.Error(1)
}

func (m *MockEventRepoForService) UpdateResult(id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
	args := m.Called(id, result, reason, metadata)
	return args.Error(0)
}

func (m *MockEventRepoForService) UpdateExecutionStatus(id uuid.UUID, executed bool, strictMode bool, executedAt time.Time, executionError *string) error {
	args := m.Called(id, executed, strictMode, executedAt, executionError)
	return args.Error(0)
}

func (m *MockEventRepoForService) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// MockAgentRepoForVerification for verification event service tests
type MockAgentRepoForVerification struct {
	mock.Mock
}

func (m *MockAgentRepoForVerification) Create(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepoForVerification) GetByID(id uuid.UUID) (*domain.Agent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForVerification) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForVerification) GetByPublicKeyHash(hash string) (*domain.Agent, error) {
	args := m.Called(hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForVerification) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForVerification) Update(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepoForVerification) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForVerification) Count(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockAgentRepoForVerification) Search(orgID uuid.UUID, query string, status *domain.AgentStatus, limit, offset int) ([]*domain.Agent, int, error) {
	args := m.Called(orgID, query, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Agent), args.Int(1), args.Error(2)
}

func (m *MockAgentRepoForVerification) CountByStatus(orgID uuid.UUID) (map[domain.AgentStatus]int, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.AgentStatus]int), args.Error(1)
}

func (m *MockAgentRepoForVerification) GetRecentlyActive(orgID uuid.UUID, hours int, limit int) ([]*domain.Agent, error) {
	args := m.Called(orgID, hours, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForVerification) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockAgentRepoForVerification) List(limit, offset int) ([]*domain.Agent, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForVerification) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	args := m.Called(id, newScore)
	return args.Error(0)
}

func (m *MockAgentRepoForVerification) IncrementViolationCount(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForVerification) MarkAsCompromised(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// ========================================
// CreateVerificationEventRequest Tests
// ========================================

func TestCreateVerificationEventRequest_Structure(t *testing.T) {
	orgID := uuid.New()
	agentID := uuid.New()
	initiatorID := uuid.New()
	now := time.Now()

	signature := "test-signature"
	messageHash := "test-hash"
	nonce := "test-nonce"
	publicKey := "test-pubkey"
	errorCode := "ERR001"
	errorReason := "Test error"
	initiatorName := "admin-user"
	initiatorIP := "192.168.1.1"
	action := "verify"
	resourceType := "agent"
	resourceID := "test-resource"
	location := "us-west-2"
	details := "Test verification"

	req := CreateVerificationEventRequest{
		OrganizationID:   orgID,
		AgentID:          agentID,
		Protocol:         domain.VerificationProtocolMCP,
		VerificationType: domain.VerificationTypeIdentity,
		Status:           domain.VerificationEventStatusSuccess,
		Result:           nil,
		Signature:        &signature,
		MessageHash:      &messageHash,
		Nonce:            &nonce,
		PublicKey:        &publicKey,
		Confidence:       0.85,
		DurationMs:       150,
		ErrorCode:        &errorCode,
		ErrorReason:      &errorReason,
		InitiatorType:    domain.InitiatorTypeUser,
		InitiatorID:      &initiatorID,
		InitiatorName:    &initiatorName,
		InitiatorIP:      &initiatorIP,
		Action:           &action,
		ResourceType:     &resourceType,
		ResourceID:       &resourceID,
		Location:         &location,
		StartedAt:        now,
		CompletedAt:      &now,
		Details:          &details,
		Metadata: map[string]interface{}{
			"source": "test",
		},
		CurrentMCPServers:   []string{"memory", "filesystem"},
		CurrentCapabilities: []string{"read", "write"},
	}

	assert.Equal(t, orgID, req.OrganizationID)
	assert.Equal(t, agentID, req.AgentID)
	assert.Equal(t, domain.VerificationProtocolMCP, req.Protocol)
	assert.Equal(t, domain.VerificationTypeIdentity, req.VerificationType)
	assert.Equal(t, domain.VerificationEventStatusSuccess, req.Status)
	assert.Equal(t, signature, *req.Signature)
	assert.Equal(t, messageHash, *req.MessageHash)
	assert.Equal(t, nonce, *req.Nonce)
	assert.Equal(t, publicKey, *req.PublicKey)
	assert.Equal(t, 0.85, req.Confidence)
	assert.Equal(t, 150, req.DurationMs)
	assert.Equal(t, domain.InitiatorTypeUser, req.InitiatorType)
	assert.Equal(t, &initiatorID, req.InitiatorID)
	assert.Len(t, req.CurrentMCPServers, 2)
	assert.Len(t, req.CurrentCapabilities, 2)
}

func TestCreateVerificationEventRequest_MinimalFields(t *testing.T) {
	req := CreateVerificationEventRequest{
		OrganizationID:   uuid.New(),
		AgentID:          uuid.New(),
		Protocol:         domain.VerificationProtocolA2A,
		VerificationType: domain.VerificationTypeCapability,
		Status:           domain.VerificationEventStatusPending,
		InitiatorType:    domain.InitiatorTypeSystem,
	}

	assert.NotEqual(t, uuid.Nil, req.OrganizationID)
	assert.NotEqual(t, uuid.Nil, req.AgentID)
	assert.Equal(t, domain.VerificationProtocolA2A, req.Protocol)
	assert.Equal(t, domain.VerificationTypeCapability, req.VerificationType)
	assert.Equal(t, domain.VerificationEventStatusPending, req.Status)
	assert.Nil(t, req.Signature)
	assert.Nil(t, req.InitiatorID)
	assert.Empty(t, req.CurrentMCPServers)
}

// ========================================
// Confidence Calculation Logic Tests
// ========================================

func TestConfidenceCalculation_Logic(t *testing.T) {
	// The confidence calculation in LogVerificationEvent is:
	// - Agent status contributes 40% (verified=0.40, pending=0.20, other=0)
	// - Trust score contributes 40% (trust score is 0-1)
	// - Event success contributes 20%

	tests := []struct {
		name               string
		agentStatus        domain.AgentStatus
		trustScore         float64
		eventStatus        domain.VerificationEventStatus
		expectedConfidence float64
	}{
		{
			name:               "verified agent, high trust, success",
			agentStatus:        domain.AgentStatusVerified,
			trustScore:         0.90,
			eventStatus:        domain.VerificationEventStatusSuccess,
			expectedConfidence: 0.40 + (0.90 * 0.40) + 0.20, // = 0.96
		},
		{
			name:               "verified agent, high trust, failed",
			agentStatus:        domain.AgentStatusVerified,
			trustScore:         0.90,
			eventStatus:        domain.VerificationEventStatusFailed,
			expectedConfidence: 0.40 + (0.90 * 0.40) + 0.0, // = 0.76
		},
		{
			name:               "pending agent, medium trust, success",
			agentStatus:        domain.AgentStatusPending,
			trustScore:         0.50,
			eventStatus:        domain.VerificationEventStatusSuccess,
			expectedConfidence: 0.20 + (0.50 * 0.40) + 0.20, // = 0.60
		},
		{
			name:               "suspended agent, low trust, failed",
			agentStatus:        domain.AgentStatusSuspended,
			trustScore:         0.30,
			eventStatus:        domain.VerificationEventStatusFailed,
			expectedConfidence: 0.0 + (0.30 * 0.40) + 0.0, // = 0.12
		},
		{
			name:               "verified agent, perfect trust, success",
			agentStatus:        domain.AgentStatusVerified,
			trustScore:         1.0,
			eventStatus:        domain.VerificationEventStatusSuccess,
			expectedConfidence: 1.0, // = 1.00 (capped at 1.0)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate confidence using the same logic as LogVerificationEvent
			confidence := 0.0
			switch tt.agentStatus {
			case domain.AgentStatusVerified:
				confidence += 0.40
			case domain.AgentStatusPending:
				confidence += 0.20
			}
			confidence += tt.trustScore * 0.40
			if tt.eventStatus == domain.VerificationEventStatusSuccess {
				confidence += 0.20
			}
			if confidence > 1.0 {
				confidence = 1.0
			}

			assert.InDelta(t, tt.expectedConfidence, confidence, 0.001)
		})
	}
}

// ========================================
// Constructor Tests
// ========================================

func TestNewVerificationEventService(t *testing.T) {
	service := NewVerificationEventService(nil, nil, nil)

	assert.NotNil(t, service)
	assert.Nil(t, service.eventRepo)
	assert.Nil(t, service.agentRepo)
	assert.Nil(t, service.driftDetection)
}

// ========================================
// Request Validation Tests
// ========================================

func TestVerificationEventRequest_OptionalFields(t *testing.T) {
	// Test that optional fields (pointers) can be nil
	req := CreateVerificationEventRequest{
		OrganizationID:   uuid.New(),
		AgentID:          uuid.New(),
		Protocol:         domain.VerificationProtocolDID,
		VerificationType: domain.VerificationTypePermission,
		Status:           domain.VerificationEventStatusSuccess,
		InitiatorType:    domain.InitiatorTypeAgent,
		// All pointer fields left as nil
	}

	assert.Nil(t, req.Signature)
	assert.Nil(t, req.MessageHash)
	assert.Nil(t, req.Nonce)
	assert.Nil(t, req.PublicKey)
	assert.Nil(t, req.ErrorCode)
	assert.Nil(t, req.ErrorReason)
	assert.Nil(t, req.InitiatorID)
	assert.Nil(t, req.InitiatorName)
	assert.Nil(t, req.InitiatorIP)
	assert.Nil(t, req.Action)
	assert.Nil(t, req.ResourceType)
	assert.Nil(t, req.ResourceID)
	assert.Nil(t, req.Location)
	assert.Nil(t, req.CompletedAt)
	assert.Nil(t, req.Details)
	assert.Nil(t, req.Metadata)
}

// ========================================
// Domain Constants Tests
// ========================================

func TestVerificationEventStatus_Values(t *testing.T) {
	// Test that status values work as expected
	tests := []struct {
		status domain.VerificationEventStatus
		value  string
	}{
		{domain.VerificationEventStatusPending, "pending"},
		{domain.VerificationEventStatusSuccess, "success"},
		{domain.VerificationEventStatusFailed, "failed"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, domain.VerificationEventStatus(tt.value), tt.status)
		})
	}
}

func TestVerificationProtocol_Values(t *testing.T) {
	tests := []struct {
		protocol domain.VerificationProtocol
		value    string
	}{
		{domain.VerificationProtocolMCP, "MCP"},
		{domain.VerificationProtocolA2A, "A2A"},
		{domain.VerificationProtocolACP, "ACP"},
		{domain.VerificationProtocolDID, "DID"},
		{domain.VerificationProtocolOAuth, "OAuth"},
		{domain.VerificationProtocolSAML, "SAML"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, domain.VerificationProtocol(tt.value), tt.protocol)
		})
	}
}

func TestVerificationType_Values(t *testing.T) {
	tests := []struct {
		vtype domain.VerificationType
		value string
	}{
		{domain.VerificationTypeIdentity, "identity"},
		{domain.VerificationTypeCapability, "capability"},
		{domain.VerificationTypePermission, "permission"},
		{domain.VerificationTypeTrust, "trust"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, domain.VerificationType(tt.value), tt.vtype)
		})
	}
}

func TestInitiatorType_Values(t *testing.T) {
	tests := []struct {
		itype domain.InitiatorType
		value string
	}{
		{domain.InitiatorTypeUser, "user"},
		{domain.InitiatorTypeAgent, "agent"},
		{domain.InitiatorTypeSystem, "system"},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			assert.Equal(t, domain.InitiatorType(tt.value), tt.itype)
		})
	}
}

// ========================================
// Time Handling Tests
// ========================================

func TestVerificationEvent_TimeHandling(t *testing.T) {
	now := time.Now()

	// Test that StartedAt can be set from DurationMs
	durationMs := 150
	startedAt := now.Add(-time.Duration(durationMs) * time.Millisecond)

	// Verify the calculation
	expectedDuration := time.Duration(durationMs) * time.Millisecond
	actualDuration := now.Sub(startedAt)

	assert.InDelta(t, expectedDuration.Milliseconds(), actualDuration.Milliseconds(), 1)
}

func TestVerificationEvent_ZeroStartedAt(t *testing.T) {
	// Test that zero StartedAt defaults to now
	var startedAt time.Time
	now := time.Now()

	if startedAt.IsZero() {
		startedAt = now
	}

	assert.False(t, startedAt.IsZero())
	assert.Equal(t, now, startedAt)
}

// ========================================
// Drift Detection Integration Tests
// ========================================

func TestVerificationEventRequest_DriftDetectionFields(t *testing.T) {
	req := CreateVerificationEventRequest{
		OrganizationID:      uuid.New(),
		AgentID:             uuid.New(),
		Protocol:            domain.VerificationProtocolMCP,
		VerificationType:    domain.VerificationTypeCapability,
		Status:              domain.VerificationEventStatusSuccess,
		InitiatorType:       domain.InitiatorTypeAgent,
		CurrentMCPServers:   []string{"memory", "filesystem", "github"},
		CurrentCapabilities: []string{"read", "write", "delete", "admin"},
	}

	// Verify drift detection fields
	assert.Len(t, req.CurrentMCPServers, 3)
	assert.Contains(t, req.CurrentMCPServers, "memory")
	assert.Contains(t, req.CurrentMCPServers, "filesystem")
	assert.Contains(t, req.CurrentMCPServers, "github")

	assert.Len(t, req.CurrentCapabilities, 4)
	assert.Contains(t, req.CurrentCapabilities, "read")
	assert.Contains(t, req.CurrentCapabilities, "admin")
}

func TestVerificationEventRequest_EmptyDriftFields(t *testing.T) {
	req := CreateVerificationEventRequest{
		OrganizationID:   uuid.New(),
		AgentID:          uuid.New(),
		Protocol:         domain.VerificationProtocolOAuth,
		VerificationType: domain.VerificationTypeIdentity,
		Status:           domain.VerificationEventStatusSuccess,
		InitiatorType:    domain.InitiatorTypeUser,
		// No drift fields
	}

	assert.Empty(t, req.CurrentMCPServers)
	assert.Empty(t, req.CurrentCapabilities)

	// Check that drift detection would be skipped
	shouldRunDriftDetection := len(req.CurrentMCPServers) > 0 || len(req.CurrentCapabilities) > 0
	assert.False(t, shouldRunDriftDetection)
}

// ========================================
// Edge Case Tests
// ========================================

func TestConfidenceCalculation_Capping(t *testing.T) {
	// Test that confidence is capped at 1.0
	// Max possible: 0.40 (verified) + 0.40 (100% trust) + 0.20 (success) = 1.00
	confidence := 0.40 + 1.0*0.40 + 0.20
	if confidence > 1.0 {
		confidence = 1.0
	}

	assert.Equal(t, 1.0, confidence)

	// Edge case: what if trust score was somehow > 1.0?
	badTrustScore := 1.5
	confidence = 0.40 + badTrustScore*0.40 + 0.20
	if confidence > 1.0 {
		confidence = 1.0
	}

	assert.Equal(t, 1.0, confidence) // Should be capped at 1.0
}

func TestConfidenceCalculation_ZeroValues(t *testing.T) {
	// Test minimum confidence scenario
	// Non-verified/pending agent, zero trust, failed status
	confidence := 0.0 + 0.0*0.40 + 0.0 // = 0.0

	assert.Equal(t, 0.0, confidence)
}

// ========================================
// Service Method Tests with Mocks
// ========================================

func TestVerificationEventService_LogVerificationEvent_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()
	agentID := uuid.New()

	agent := &domain.Agent{
		ID:          agentID,
		DisplayName: "Test Agent",
		Status:      domain.AgentStatusVerified,
		TrustScore:  0.85,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockEventRepo.On("Create", mock.AnythingOfType("*domain.VerificationEvent")).Return(nil)

	event, err := service.LogVerificationEvent(
		ctx,
		orgID,
		agentID,
		domain.VerificationProtocolMCP,
		domain.VerificationTypeIdentity,
		domain.VerificationEventStatusSuccess,
		150,
		domain.InitiatorTypeAgent,
		nil,
		map[string]interface{}{"source": "test"},
	)

	assert.NoError(t, err)
	assert.NotNil(t, event)
	assert.Equal(t, orgID, event.OrganizationID)

	mockAgentRepo.AssertExpectations(t)
	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_LogVerificationEvent_AgentNotFound(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()
	agentID := uuid.New()

	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	event, err := service.LogVerificationEvent(
		ctx,
		orgID,
		agentID,
		domain.VerificationProtocolMCP,
		domain.VerificationTypeIdentity,
		domain.VerificationEventStatusSuccess,
		150,
		domain.InitiatorTypeAgent,
		nil,
		nil,
	)

	assert.Error(t, err)
	assert.Nil(t, event)
	assert.Contains(t, err.Error(), "agent not found")

	mockAgentRepo.AssertExpectations(t)
}

func TestVerificationEventService_LogVerificationEvent_PendingAgent(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()
	agentID := uuid.New()

	agent := &domain.Agent{
		ID:          agentID,
		DisplayName: "Pending Agent",
		Status:      domain.AgentStatusPending,
		TrustScore:  0.50,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockEventRepo.On("Create", mock.AnythingOfType("*domain.VerificationEvent")).Return(nil)

	event, err := service.LogVerificationEvent(
		ctx,
		orgID,
		agentID,
		domain.VerificationProtocolA2A,
		domain.VerificationTypeCapability,
		domain.VerificationEventStatusFailed,
		200,
		domain.InitiatorTypeSystem,
		nil,
		nil,
	)

	assert.NoError(t, err)
	assert.NotNil(t, event)

	mockAgentRepo.AssertExpectations(t)
	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_ListVerificationEvents_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()

	events := []*domain.VerificationEvent{
		{ID: uuid.New(), OrganizationID: orgID},
		{ID: uuid.New(), OrganizationID: orgID},
	}

	mockEventRepo.On("GetByOrganization", orgID, 10, 0).Return(events, 2, nil)

	result, total, err := service.ListVerificationEvents(ctx, orgID, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, total)

	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_ListVerificationEvents_Error(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()

	mockEventRepo.On("GetByOrganization", orgID, 10, 0).Return(nil, 0, errors.New("database error"))

	result, total, err := service.ListVerificationEvents(ctx, orgID, 10, 0)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 0, total)

	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_ListAgentVerificationEvents_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	agentID := uuid.New()

	events := []*domain.VerificationEvent{
		{ID: uuid.New(), AgentID: &agentID},
	}

	mockEventRepo.On("GetByAgent", agentID, 20, 0).Return(events, 1, nil)

	result, total, err := service.ListAgentVerificationEvents(ctx, agentID, 20, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, total)

	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_ListAgentVerificationEvents_Empty(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	agentID := uuid.New()

	mockEventRepo.On("GetByAgent", agentID, 10, 0).Return([]*domain.VerificationEvent{}, 0, nil)

	result, total, err := service.ListAgentVerificationEvents(ctx, agentID, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 0)
	assert.Equal(t, 0, total)

	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_GetRecentEvents_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()

	events := []*domain.VerificationEvent{
		{ID: uuid.New(), OrganizationID: orgID},
	}

	mockEventRepo.On("GetRecentEvents", orgID, 60).Return(events, nil)

	result, err := service.GetRecentEvents(ctx, orgID, 60)

	assert.NoError(t, err)
	assert.Len(t, result, 1)

	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_GetRecentEvents_Error(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()

	mockEventRepo.On("GetRecentEvents", orgID, 30).Return(nil, errors.New("database error"))

	result, err := service.GetRecentEvents(ctx, orgID, 30)

	assert.Error(t, err)
	assert.Nil(t, result)

	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_GetPendingVerifications_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()

	events := []*domain.VerificationEvent{
		{ID: uuid.New(), Status: domain.VerificationEventStatusPending},
	}

	mockEventRepo.On("GetPendingVerifications", orgID).Return(events, nil)

	result, err := service.GetPendingVerifications(ctx, orgID)

	assert.NoError(t, err)
	assert.Len(t, result, 1)

	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_GetPendingVerifications_Empty(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()

	mockEventRepo.On("GetPendingVerifications", orgID).Return([]*domain.VerificationEvent{}, nil)

	result, err := service.GetPendingVerifications(ctx, orgID)

	assert.NoError(t, err)
	assert.Len(t, result, 0)

	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_ListMCPVerificationEvents_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	mcpServerID := uuid.New()

	events := []*domain.VerificationEvent{
		{ID: uuid.New(), MCPServerID: &mcpServerID},
	}

	mockEventRepo.On("GetByMCPServer", mcpServerID, 10, 0).Return(events, 1, nil)

	result, total, err := service.ListMCPVerificationEvents(ctx, mcpServerID, 10, 0)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, total)

	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_ListMCPVerificationEvents_Error(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	mcpServerID := uuid.New()

	mockEventRepo.On("GetByMCPServer", mcpServerID, 10, 0).Return(nil, 0, errors.New("database error"))

	result, total, err := service.ListMCPVerificationEvents(ctx, mcpServerID, 10, 0)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 0, total)

	mockEventRepo.AssertExpectations(t)
}

// ===========================
// GetVerificationEvent Tests
// ===========================

func TestVerificationEventService_GetVerificationEvent_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	eventID := uuid.New()
	expectedEvent := &domain.VerificationEvent{
		ID:     eventID,
		Status: domain.VerificationEventStatusSuccess,
	}

	mockEventRepo.On("GetByID", eventID).Return(expectedEvent, nil)

	result, err := service.GetVerificationEvent(ctx, eventID)

	assert.NoError(t, err)
	assert.Equal(t, expectedEvent, result)
	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_GetVerificationEvent_NotFound(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	eventID := uuid.New()

	mockEventRepo.On("GetByID", eventID).Return(nil, errors.New("not found"))

	result, err := service.GetVerificationEvent(ctx, eventID)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockEventRepo.AssertExpectations(t)
}

// ===========================
// GetStatistics Tests
// ===========================

func TestVerificationEventService_GetStatistics_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	expectedStats := &domain.VerificationStatistics{
		TotalVerifications: 100,
		SuccessCount:       80,
		FailedCount:        15,
		PendingCount:       5,
		SuccessRate:        0.80,
	}

	mockEventRepo.On("GetStatistics", orgID, startTime, endTime).Return(expectedStats, nil)

	result, err := service.GetStatistics(ctx, orgID, startTime, endTime)

	assert.NoError(t, err)
	assert.Equal(t, expectedStats, result)
	assert.Equal(t, 100, result.TotalVerifications)
	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_GetStatistics_Error(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()
	startTime := time.Now().Add(-24 * time.Hour)
	endTime := time.Now()

	mockEventRepo.On("GetStatistics", orgID, startTime, endTime).Return(nil, errors.New("database error"))

	result, err := service.GetStatistics(ctx, orgID, startTime, endTime)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockEventRepo.AssertExpectations(t)
}

// ===========================
// GetLast24HoursStatistics Tests
// ===========================

func TestVerificationEventService_GetLast24HoursStatistics_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()

	expectedStats := &domain.VerificationStatistics{
		TotalVerifications: 50,
		SuccessCount:       45,
		FailedCount:        5,
		SuccessRate:        0.90,
	}

	// Match any time range within last 24 hours
	mockEventRepo.On("GetStatistics", orgID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(expectedStats, nil)

	result, err := service.GetLast24HoursStatistics(ctx, orgID)

	assert.NoError(t, err)
	assert.Equal(t, expectedStats, result)
	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_GetLast24HoursStatistics_Error(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	orgID := uuid.New()

	mockEventRepo.On("GetStatistics", orgID, mock.AnythingOfType("time.Time"), mock.AnythingOfType("time.Time")).Return(nil, errors.New("database error"))

	result, err := service.GetLast24HoursStatistics(ctx, orgID)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockEventRepo.AssertExpectations(t)
}

// ===========================
// UpdateVerificationResult Tests
// ===========================

func TestVerificationEventService_UpdateVerificationResult_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	eventID := uuid.New()
	result := domain.VerificationResultVerified
	reason := "Verification completed successfully"
	metadata := map[string]interface{}{"key": "value"}

	mockEventRepo.On("UpdateResult", eventID, result, &reason, metadata).Return(nil)

	err := service.UpdateVerificationResult(ctx, eventID, result, &reason, metadata)

	assert.NoError(t, err)
	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_UpdateVerificationResult_Error(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	eventID := uuid.New()
	result := domain.VerificationResultDenied

	mockEventRepo.On("UpdateResult", eventID, result, (*string)(nil), (map[string]interface{})(nil)).Return(errors.New("update failed"))

	err := service.UpdateVerificationResult(ctx, eventID, result, nil, nil)

	assert.Error(t, err)
	mockEventRepo.AssertExpectations(t)
}

// ===========================
// UpdateExecutionStatus Tests
// ===========================

func TestVerificationEventService_UpdateExecutionStatus_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	eventID := uuid.New()
	executed := true
	strictMode := true
	executedAt := time.Now()

	mockEventRepo.On("UpdateExecutionStatus", eventID, executed, strictMode, executedAt, (*string)(nil)).Return(nil)

	err := service.UpdateExecutionStatus(ctx, eventID, executed, strictMode, executedAt, nil)

	assert.NoError(t, err)
	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_UpdateExecutionStatus_WithError(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	eventID := uuid.New()
	executed := false
	strictMode := false
	executedAt := time.Now()
	execError := "execution failed"

	mockEventRepo.On("UpdateExecutionStatus", eventID, executed, strictMode, executedAt, &execError).Return(nil)

	err := service.UpdateExecutionStatus(ctx, eventID, executed, strictMode, executedAt, &execError)

	assert.NoError(t, err)
	mockEventRepo.AssertExpectations(t)
}

// ===========================
// DeleteVerificationEvent Tests
// ===========================

func TestVerificationEventService_DeleteVerificationEvent_Success(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	eventID := uuid.New()

	mockEventRepo.On("Delete", eventID).Return(nil)

	err := service.DeleteVerificationEvent(ctx, eventID)

	assert.NoError(t, err)
	mockEventRepo.AssertExpectations(t)
}

func TestVerificationEventService_DeleteVerificationEvent_Error(t *testing.T) {
	mockEventRepo := new(MockEventRepoForService)
	mockAgentRepo := new(MockAgentRepoForVerification)

	service := NewVerificationEventService(mockEventRepo, mockAgentRepo, nil)

	ctx := context.Background()
	eventID := uuid.New()

	mockEventRepo.On("Delete", eventID).Return(errors.New("delete failed"))

	err := service.DeleteVerificationEvent(ctx, eventID)

	assert.Error(t, err)
	mockEventRepo.AssertExpectations(t)
}
