package application

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
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
