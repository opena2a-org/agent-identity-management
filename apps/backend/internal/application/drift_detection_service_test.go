package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentRepository mocks the AgentRepository interface
type MockAgentRepository struct {
	mock.Mock
}

func (m *MockAgentRepository) Create(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepository) GetByID(id uuid.UUID) (*domain.Agent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepository) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepository) List(limit, offset int) ([]*domain.Agent, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepository) Update(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepository) UpdatePublicKey(agentID uuid.UUID, publicKey string) error {
	args := m.Called(agentID, publicKey)
	return args.Error(0)
}

func (m *MockAgentRepository) UpdateTrustScore(agentID uuid.UUID, score float64) error {
	args := m.Called(agentID, score)
	return args.Error(0)
}

func (m *MockAgentRepository) MarkAsCompromised(agentID uuid.UUID) error {
	args := m.Called(agentID)
	return args.Error(0)
}

func (m *MockAgentRepository) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockAgentRepository) IncrementViolationCount(agentID uuid.UUID) error {
	args := m.Called(agentID)
	return args.Error(0)
}

func (m *MockAgentRepository) GetStaleAgents(ctx context.Context, staleSince time.Time) ([]*domain.Agent, error) {
	args := m.Called(ctx, staleSince)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(ctx, ids)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

// MockAlertRepository mocks the AlertRepository interface
type MockAlertRepository struct {
	mock.Mock
}

func (m *MockAlertRepository) Create(alert *domain.Alert) error {
	args := m.Called(alert)
	return args.Error(0)
}

func (m *MockAlertRepository) GetByID(id uuid.UUID) (*domain.Alert, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Alert), args.Error(1)
}

func (m *MockAlertRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepository) GetUnacknowledged(orgID uuid.UUID) ([]*domain.Alert, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepository) Acknowledge(id, userID uuid.UUID) error {
	args := m.Called(id, userID)
	return args.Error(0)
}

func (m *MockAlertRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAlertRepository) CountByOrganization(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockAlertRepository) GetByResourceID(resourceID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepository) GetUnacknowledgedByResourceID(resourceID uuid.UUID) ([]*domain.Alert, error) {
	args := m.Called(resourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepository) GetByOrganizationFiltered(orgID uuid.UUID, status string, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(orgID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepository) CountByOrganizationFiltered(orgID uuid.UUID, status string) (int, error) {
	args := m.Called(orgID, status)
	return args.Int(0), args.Error(1)
}

func (m *MockAlertRepository) BulkAcknowledge(orgID uuid.UUID, userID uuid.UUID) (int, error) {
	args := m.Called(orgID, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockAlertRepository) CountBySeverity(orgID uuid.UUID, status string) (critical, high, warning, info int, err error) {
	args := m.Called(orgID, status)
	return args.Int(0), args.Int(1), args.Int(2), args.Int(3), args.Error(4)
}

func TestDetectDrift_NoDrift(t *testing.T) {
	// Setup
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	agentID := uuid.New()
	orgID := uuid.New()

	// Agent with registered MCP servers
	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
		TalksTo:        []string{"filesystem-mcp", "github-mcp"},
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)

	// Test: Runtime matches registered configuration
	result, err := service.DetectDrift(
		agentID,
		[]string{"filesystem-mcp", "github-mcp"},
		[]string{},
	)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.DriftDetected)
	assert.Empty(t, result.MCPServerDrift)
	assert.Empty(t, result.CapabilityDrift)
	assert.Nil(t, result.Alert)

	mockAgentRepo.AssertExpectations(t)
	mockAlertRepo.AssertExpectations(t)
}

func TestDetectDrift_MCPServerDrift(t *testing.T) {
	// Setup
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	agentID := uuid.New()
	orgID := uuid.New()

	// Agent with registered MCP servers (first violation)
	agent := &domain.Agent{
		ID:                        agentID,
		OrganizationID:            orgID,
		Name:                      "test-agent",
		TalksTo:                   []string{"filesystem-mcp"},
		TrustScore:                85.0,
		CapabilityViolationCount:  0, // First violation
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	// Expect first violation penalty: 85.0 - 5.0 = 80.0
	mockAgentRepo.On("UpdateTrustScore", agentID, 80.0).Return(nil)

	// Test: Runtime includes unregistered MCP server
	result, err := service.DetectDrift(
		agentID,
		[]string{"filesystem-mcp", "external-api-mcp"},
		[]string{},
	)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.DriftDetected)
	assert.Equal(t, []string{"external-api-mcp"}, result.MCPServerDrift)
	assert.Empty(t, result.CapabilityDrift)
	assert.NotNil(t, result.Alert)

	// Verify alert details
	assert.Equal(t, domain.AlertTypeConfigurationDrift, result.Alert.AlertType)
	assert.Equal(t, domain.AlertSeverityHigh, result.Alert.Severity)
	assert.Equal(t, "Configuration Drift Detected: test-agent", result.Alert.Title)
	assert.Contains(t, result.Alert.Description, "external-api-mcp")
	assert.Contains(t, result.Alert.Description, "not registered")

	mockAgentRepo.AssertExpectations(t)
	mockAlertRepo.AssertExpectations(t)
}

func TestDetectDrift_MultipleUnauthorizedServers(t *testing.T) {
	// Setup
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	agentID := uuid.New()
	orgID := uuid.New()

	// Agent with NO registered MCP servers
	agent := &domain.Agent{
		ID:                       agentID,
		OrganizationID:           orgID,
		Name:                     "rogue-agent",
		TalksTo:                  []string{},
		TrustScore:               90.0,
		CapabilityViolationCount: 0,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	// First violation penalty: 90.0 - 5.0 = 85.0
	mockAgentRepo.On("UpdateTrustScore", agentID, 85.0).Return(nil)

	// Test: Runtime includes multiple unregistered MCP servers
	result, err := service.DetectDrift(
		agentID,
		[]string{"unauthorized-mcp-1", "unauthorized-mcp-2", "malicious-mcp"},
		[]string{},
	)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.DriftDetected)
	assert.ElementsMatch(t, []string{"unauthorized-mcp-1", "unauthorized-mcp-2", "malicious-mcp"}, result.MCPServerDrift)
	assert.NotNil(t, result.Alert)

	// Verify alert includes all unauthorized servers
	assert.Contains(t, result.Alert.Description, "unauthorized-mcp-1")
	assert.Contains(t, result.Alert.Description, "unauthorized-mcp-2")
	assert.Contains(t, result.Alert.Description, "malicious-mcp")

	mockAgentRepo.AssertExpectations(t)
	mockAlertRepo.AssertExpectations(t)
}

func TestDetectDrift_RepeatedViolation(t *testing.T) {
	// Setup
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	agentID := uuid.New()
	orgID := uuid.New()

	// Agent with previous violations
	agent := &domain.Agent{
		ID:                       agentID,
		OrganizationID:           orgID,
		Name:                     "repeat-offender",
		TalksTo:                  []string{"filesystem-mcp"},
		TrustScore:               70.0,
		CapabilityViolationCount: 2, // Already has violations
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	// Repeated violation penalty: 70.0 - 10.0 = 60.0
	mockAgentRepo.On("UpdateTrustScore", agentID, 60.0).Return(nil)

	// Test: Repeated drift violation
	result, err := service.DetectDrift(
		agentID,
		[]string{"filesystem-mcp", "malicious-mcp"},
		[]string{},
	)

	// Verify
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.DriftDetected)
	assert.Equal(t, []string{"malicious-mcp"}, result.MCPServerDrift)

	mockAgentRepo.AssertExpectations(t)
	mockAlertRepo.AssertExpectations(t)
}

func TestDetectDrift_TrustScoreFloor(t *testing.T) {
	// Setup
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	agentID := uuid.New()
	orgID := uuid.New()

	// Agent with very low trust score
	agent := &domain.Agent{
		ID:                       agentID,
		OrganizationID:           orgID,
		Name:                     "low-trust-agent",
		TalksTo:                  []string{"filesystem-mcp"},
		TrustScore:               3.0, // Very low
		CapabilityViolationCount: 5,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	// Should hit floor: 3.0 - 10.0 = -7.0 -> 0.0 (minimum)
	mockAgentRepo.On("UpdateTrustScore", agentID, 0.0).Return(nil)

	// Test: Drift violation should not go below 0
	result, err := service.DetectDrift(
		agentID,
		[]string{"filesystem-mcp", "evil-mcp"},
		[]string{},
	)

	// Verify
	assert.NoError(t, err)
	assert.True(t, result.DriftDetected)

	mockAgentRepo.AssertExpectations(t)
	mockAlertRepo.AssertExpectations(t)
}

func TestDetectArrayDrift(t *testing.T) {
	tests := []struct {
		name       string
		registered []string
		runtime    []string
		expected   []string
	}{
		{
			name:       "no drift - exact match",
			registered: []string{"a", "b", "c"},
			runtime:    []string{"a", "b", "c"},
			expected:   []string{},
		},
		{
			name:       "no drift - runtime subset of registered",
			registered: []string{"a", "b", "c"},
			runtime:    []string{"a", "b"},
			expected:   []string{},
		},
		{
			name:       "drift detected - one unregistered item",
			registered: []string{"a", "b"},
			runtime:    []string{"a", "b", "c"},
			expected:   []string{"c"},
		},
		{
			name:       "drift detected - multiple unregistered items",
			registered: []string{"a"},
			runtime:    []string{"a", "b", "c", "d"},
			expected:   []string{"b", "c", "d"},
		},
		{
			name:       "drift detected - all unregistered",
			registered: []string{},
			runtime:    []string{"a", "b", "c"},
			expected:   []string{"a", "b", "c"},
		},
		{
			name:       "no drift - empty runtime",
			registered: []string{"a", "b"},
			runtime:    []string{},
			expected:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectArrayDrift(tt.registered, tt.runtime)
			assert.ElementsMatch(t, tt.expected, result)
		})
	}
}

// ====================
// Error Handling Tests
// ====================

func TestDetectDrift_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	agentID := uuid.New()

	mockAgentRepo.On("GetByID", agentID).Return(nil, assert.AnError)

	// Test: Agent not found should return error
	result, err := service.DetectDrift(
		agentID,
		[]string{"some-mcp"},
		[]string{},
	)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get agent")

	mockAgentRepo.AssertExpectations(t)
}

func TestDetectDrift_AlertCreationFails(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	agentID := uuid.New()
	orgID := uuid.New()

	agent := &domain.Agent{
		ID:                       agentID,
		OrganizationID:           orgID,
		Name:                     "test-agent",
		TalksTo:                  []string{"filesystem-mcp"},
		TrustScore:               85.0,
		CapabilityViolationCount: 0,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(assert.AnError)
	mockAgentRepo.On("UpdateTrustScore", agentID, 80.0).Return(nil)

	// Test: Alert creation fails but drift detection still succeeds
	result, err := service.DetectDrift(
		agentID,
		[]string{"filesystem-mcp", "unauthorized-mcp"},
		[]string{},
	)

	// Should succeed despite alert creation failure (logged but not fatal)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.DriftDetected)
	assert.Nil(t, result.Alert) // Alert should be nil since creation failed

	mockAgentRepo.AssertExpectations(t)
	mockAlertRepo.AssertExpectations(t)
}

func TestDetectDrift_TrustScoreUpdateFails(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	agentID := uuid.New()
	orgID := uuid.New()

	agent := &domain.Agent{
		ID:                       agentID,
		OrganizationID:           orgID,
		Name:                     "test-agent",
		TalksTo:                  []string{"filesystem-mcp"},
		TrustScore:               85.0,
		CapabilityViolationCount: 0,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	mockAgentRepo.On("UpdateTrustScore", agentID, 80.0).Return(assert.AnError)

	// Test: Trust score update fails but drift detection still succeeds
	result, err := service.DetectDrift(
		agentID,
		[]string{"filesystem-mcp", "unauthorized-mcp"},
		[]string{},
	)

	// Should succeed despite trust score update failure (logged but not fatal)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.DriftDetected)

	mockAgentRepo.AssertExpectations(t)
	mockAlertRepo.AssertExpectations(t)
}

// ====================
// Constructor Tests
// ====================

func TestNewDriftDetectionService(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)

	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	assert.NotNil(t, service)
	assert.Equal(t, mockAgentRepo, service.agentRepo)
	assert.Equal(t, mockAlertRepo, service.alertRepo)
}

func TestNewDriftDetectionService_NilRepos(t *testing.T) {
	service := NewDriftDetectionService(nil, nil)

	assert.NotNil(t, service)
	assert.Nil(t, service.agentRepo)
	assert.Nil(t, service.alertRepo)
}

// ====================
// Constants Tests
// ====================

func TestDriftDetection_Constants(t *testing.T) {
	assert.Equal(t, 5.0, FirstViolationPenalty)
	assert.Equal(t, 10.0, RepeatedViolationPenalty)
	assert.Equal(t, 0.0, MinimumTrustScore)

	// Repeated penalty should be higher than first violation
	assert.Greater(t, RepeatedViolationPenalty, FirstViolationPenalty)
}

// ====================
// DriftResult Tests
// ====================

func TestDriftResult_Structure(t *testing.T) {
	alert := &domain.Alert{
		ID:        uuid.New(),
		AlertType: domain.AlertTypeConfigurationDrift,
	}

	result := DriftResult{
		DriftDetected:   true,
		MCPServerDrift:  []string{"unauthorized-mcp-1", "unauthorized-mcp-2"},
		CapabilityDrift: []string{"unauthorized-capability"},
		Alert:           alert,
	}

	assert.True(t, result.DriftDetected)
	assert.Len(t, result.MCPServerDrift, 2)
	assert.Contains(t, result.MCPServerDrift, "unauthorized-mcp-1")
	assert.Len(t, result.CapabilityDrift, 1)
	assert.NotNil(t, result.Alert)
	assert.Equal(t, domain.AlertTypeConfigurationDrift, result.Alert.AlertType)
}

func TestDriftResult_NoDrift(t *testing.T) {
	result := DriftResult{
		DriftDetected:   false,
		MCPServerDrift:  []string{},
		CapabilityDrift: []string{},
		Alert:           nil,
	}

	assert.False(t, result.DriftDetected)
	assert.Empty(t, result.MCPServerDrift)
	assert.Empty(t, result.CapabilityDrift)
	assert.Nil(t, result.Alert)
}

// ====================
// Empty TalksTo Tests
// ====================

func TestDetectDrift_EmptyRegisteredTalksTo(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	agentID := uuid.New()
	orgID := uuid.New()

	// Agent with no registered MCP servers
	agent := &domain.Agent{
		ID:                       agentID,
		OrganizationID:           orgID,
		Name:                     "no-mcp-agent",
		TalksTo:                  []string{}, // Empty
		TrustScore:               100.0,
		CapabilityViolationCount: 0,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)

	// Test: No runtime MCP servers - no drift
	result, err := service.DetectDrift(
		agentID,
		[]string{},
		[]string{},
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result.DriftDetected)

	mockAgentRepo.AssertExpectations(t)
}

func TestDetectDrift_NilTalksTo(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)
	service := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	agentID := uuid.New()
	orgID := uuid.New()

	// Agent with nil TalksTo
	agent := &domain.Agent{
		ID:                       agentID,
		OrganizationID:           orgID,
		Name:                     "nil-talks-to-agent",
		TalksTo:                  nil, // nil
		TrustScore:               95.0,
		CapabilityViolationCount: 0,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)
	mockAgentRepo.On("UpdateTrustScore", agentID, 90.0).Return(nil)

	// Test: Any runtime MCP server causes drift when registered is nil
	result, err := service.DetectDrift(
		agentID,
		[]string{"some-mcp"},
		[]string{},
	)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.DriftDetected)
	assert.Equal(t, []string{"some-mcp"}, result.MCPServerDrift)

	mockAgentRepo.AssertExpectations(t)
}

// GetByOrganizationPaged pages over GetByOrganization for tests.
func (m *MockAgentRepository) GetByOrganizationPaged(orgID uuid.UUID, limit, offset int) ([]*domain.Agent, int, error) {
	agents, err := m.GetByOrganization(orgID)
	if err != nil {
		return nil, 0, err
	}
	total := len(agents)
	if limit > 0 {
		if offset >= len(agents) {
			agents = nil
		} else {
			agents = agents[offset:]
		}
		if len(agents) > limit {
			agents = agents[:limit]
		}
	}
	return agents, total, nil
}
