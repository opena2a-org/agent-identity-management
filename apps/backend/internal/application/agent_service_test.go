package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/crypto"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ===========================
// Mock Definitions (unique to agent_service_test.go)
// ===========================

// AgentServiceMockTrustScoreCalculator for testing
type AgentServiceMockTrustScoreCalculator struct {
	mock.Mock
}

func (m *AgentServiceMockTrustScoreCalculator) Calculate(agent *domain.Agent) (*domain.TrustScore, error) {
	args := m.Called(agent)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScore), args.Error(1)
}

func (m *AgentServiceMockTrustScoreCalculator) CalculateFactors(agent *domain.Agent) (*domain.TrustScoreFactors, error) {
	args := m.Called(agent)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScoreFactors), args.Error(1)
}

// AgentServiceMockTrustScoreRepository for testing
type AgentServiceMockTrustScoreRepository struct {
	mock.Mock
}

func (m *AgentServiceMockTrustScoreRepository) Create(score *domain.TrustScore) error {
	args := m.Called(score)
	return args.Error(0)
}

func (m *AgentServiceMockTrustScoreRepository) GetByAgent(agentID uuid.UUID) (*domain.TrustScore, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScore), args.Error(1)
}

func (m *AgentServiceMockTrustScoreRepository) GetLatest(agentID uuid.UUID) (*domain.TrustScore, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.TrustScore), args.Error(1)
}

func (m *AgentServiceMockTrustScoreRepository) GetHistory(agentID uuid.UUID, limit int) ([]*domain.TrustScore, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TrustScore), args.Error(1)
}

func (m *AgentServiceMockTrustScoreRepository) GetHistoryAuditTrail(agentID uuid.UUID, limit int) ([]*domain.TrustScoreHistoryEntry, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.TrustScoreHistoryEntry), args.Error(1)
}

// AgentServiceMockSecurityPolicyRepository for testing
type AgentServiceMockSecurityPolicyRepository struct {
	mock.Mock
}

func (m *AgentServiceMockSecurityPolicyRepository) Create(policy *domain.SecurityPolicy) error {
	args := m.Called(policy)
	return args.Error(0)
}

func (m *AgentServiceMockSecurityPolicyRepository) GetByID(id uuid.UUID) (*domain.SecurityPolicy, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SecurityPolicy), args.Error(1)
}

func (m *AgentServiceMockSecurityPolicyRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SecurityPolicy), args.Error(1)
}

func (m *AgentServiceMockSecurityPolicyRepository) GetActiveByOrganization(orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SecurityPolicy), args.Error(1)
}

func (m *AgentServiceMockSecurityPolicyRepository) GetByType(orgID uuid.UUID, policyType domain.PolicyType) ([]*domain.SecurityPolicy, error) {
	args := m.Called(orgID, policyType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SecurityPolicy), args.Error(1)
}

func (m *AgentServiceMockSecurityPolicyRepository) Update(policy *domain.SecurityPolicy) error {
	args := m.Called(policy)
	return args.Error(0)
}

func (m *AgentServiceMockSecurityPolicyRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// ===========================
// Test Utilities
// ===========================

func createTestAgentForService() *domain.Agent {
	publicKey := "test-public-key"
	encryptedPrivateKey := "encrypted-private-key"
	now := time.Now()

	return &domain.Agent{
		ID:                  uuid.New(),
		OrganizationID:      uuid.New(),
		Name:                "test-agent",
		DisplayName:         "Test Agent",
		Description:         "A test agent for unit testing",
		AgentType:           domain.AgentTypeAI,
		Status:              domain.AgentStatusVerified,
		Version:             "1.0.0",
		PublicKey:           &publicKey,
		EncryptedPrivateKey: &encryptedPrivateKey,
		KeyAlgorithm:        "Ed25519",
		TrustScore:          0.85,
		VerifiedAt:          &now,
		IsCompromised:       false,
		Capabilities:        []string{"file:read", "api:call"},
		TalksTo:             []string{"mcp-server-1"},
		CreatedAt:           now,
		UpdatedAt:           now,
		CreatedBy:           uuid.New(),
	}
}

// ===========================
// GetAgent Tests
// ===========================

func TestAgentService_GetAgent_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockKeyVault := &crypto.KeyVault{}
	mockAlertRepo := new(MockAlertRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockPolicyRepo := new(AgentServiceMockSecurityPolicyRepository)
	
	policyService := &SecurityPolicyService{
		policyRepo: mockPolicyRepo,
		alertRepo:  mockAlertRepo,
	}

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
		keyVault:       mockKeyVault,
		alertRepo:      mockAlertRepo,
		policyService:  policyService,
		capabilityRepo: mockCapabilityRepo,
	}

	expectedAgent := createTestAgentForService()
	mockAgentRepo.On("GetByID", expectedAgent.ID).Return(expectedAgent, nil)

	ctx := context.Background()
	agent, err := service.GetAgent(ctx, expectedAgent.ID)

	assert.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, expectedAgent.ID, agent.ID)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_GetAgent_NotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	ctx := context.Background()
	agent, err := service.GetAgent(ctx, agentID)

	assert.Error(t, err)
	assert.Nil(t, agent)
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// DeleteAgent Tests
// ===========================

func TestAgentService_DeleteAgent_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("Delete", agentID).Return(nil)

	ctx := context.Background()
	err := service.DeleteAgent(ctx, agentID)

	assert.NoError(t, err)
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// RecalculateTrustScore Tests
// ===========================

func TestAgentService_RecalculateTrustScore_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
	}

	agent := createTestAgentForService()
	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)

	newTrustScore := &domain.TrustScore{
		ID:      uuid.New(),
		AgentID: agent.ID,
		Score:   0.95,
	}
	mockTrustCalc.On("Calculate", agent).Return(newTrustScore, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)
	mockTrustScoreRepo.On("Create", newTrustScore).Return(nil)

	ctx := context.Background()
	trustScore, err := service.RecalculateTrustScore(ctx, agent.ID)

	assert.NoError(t, err)
	assert.NotNil(t, trustScore)
	assert.Equal(t, 0.95, trustScore.Score)
	mockAgentRepo.AssertExpectations(t)
	mockTrustCalc.AssertExpectations(t)
}

// ===========================
// UpdateTrustScore Tests
// ===========================

func TestAgentService_UpdateTrustScore_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	orgID := uuid.New()
	newScore := 0.75

	// Agent with previous score slightly higher (no significant drop)
	existingAgent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
		DisplayName:    "Test Agent",
		TrustScore:     0.80, // Only 0.05 drop - not significant
	}

	mockAgentRepo.On("GetByID", agentID).Return(existingAgent, nil)
	mockAgentRepo.On("UpdateTrustScore", agentID, newScore).Return(nil)

	ctx := context.Background()
	err := service.UpdateTrustScore(ctx, agentID, newScore)

	assert.NoError(t, err)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_UpdateTrustScore_InvalidScore(t *testing.T) {
	service := &AgentService{}

	tests := []struct {
		name  string
		score float64
	}{
		{"negative score", -0.1},
		{"score too high", 10.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			err := service.UpdateTrustScore(ctx, uuid.New(), tt.score)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "trust score must be between")
		})
	}
}

// ===========================
// VerifyAction Tests (EchoLeak Prevention - CRITICAL)
// ===========================

func TestAgentService_VerifyAction_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockPolicyRepo := new(AgentServiceMockSecurityPolicyRepository)
	mockAlertRepo := new(MockAlertRepository)

	// Create policy service with no active policies (will bypass policy checks)
	policyService := &SecurityPolicyService{
		policyRepo: mockPolicyRepo,
		alertRepo:  mockAlertRepo,
	}

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
		policyService:  policyService,
		alertRepo:      mockAlertRepo,
	}

	agent := createTestAgentForService()
	agent.Status = domain.AgentStatusVerified
	agent.IsCompromised = false

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)

	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: "file:read",
		},
	}
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)

	// Mock policy repo to return empty policies (no blocking)
	mockPolicyRepo.On("GetActiveByOrganization", agent.OrganizationID).Return([]*domain.SecurityPolicy{}, nil).Maybe()
	mockPolicyRepo.On("GetByType", agent.OrganizationID, mock.Anything).Return([]*domain.SecurityPolicy{}, nil).Maybe()

	ctx := context.Background()
	allowed, reason, auditID, err := service.VerifyAction(ctx, agent.ID, "file:read", "/test.txt", nil)

	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Contains(t, reason, "Action matches registered capabilities")
	assert.NotEqual(t, uuid.Nil, auditID)
	mockAgentRepo.AssertExpectations(t)
	mockCapabilityRepo.AssertExpectations(t)
}

func TestAgentService_VerifyAction_AgentNotVerified(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agent := createTestAgentForService()
	agent.Status = domain.AgentStatusPending

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)

	ctx := context.Background()
	allowed, reason, auditID, err := service.VerifyAction(ctx, agent.ID, "file:read", "/test.txt", nil)

	assert.NoError(t, err)
	assert.False(t, allowed)
	assert.Contains(t, reason, "Agent not verified")
	assert.NotEqual(t, uuid.Nil, auditID)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_VerifyAction_AgentCompromised(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agent := createTestAgentForService()
	agent.Status = domain.AgentStatusVerified
	agent.IsCompromised = true

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)

	ctx := context.Background()
	allowed, reason, auditID, err := service.VerifyAction(ctx, agent.ID, "file:read", "/test.txt", nil)

	assert.NoError(t, err)
	assert.False(t, allowed)
	assert.Contains(t, reason, "compromised")
	assert.NotEqual(t, uuid.Nil, auditID)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_VerifyAction_NoCapabilities(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	agent := createTestAgentForService()
	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).
		Return([]*domain.AgentCapability{}, nil)

	ctx := context.Background()
	allowed, reason, _, err := service.VerifyAction(ctx, agent.ID, "file:read", "/test.txt", nil)

	assert.NoError(t, err)
	assert.False(t, allowed)
	assert.Contains(t, reason, "no granted capabilities")
	mockAgentRepo.AssertExpectations(t)
	mockCapabilityRepo.AssertExpectations(t)
}

func TestAgentService_VerifyAction_WildcardCapability(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)
	mockPolicyRepo := new(AgentServiceMockSecurityPolicyRepository)
	mockAlertRepo := new(MockAlertRepository)

	policyService := &SecurityPolicyService{
		policyRepo: mockPolicyRepo,
		alertRepo:  mockAlertRepo,
	}

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
		policyService:  policyService,
		alertRepo:      mockAlertRepo,
	}

	agent := createTestAgentForService()
	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil).Times(2)

	capabilities := []*domain.AgentCapability{
		{CapabilityType: "file:*"},
	}
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).
		Return(capabilities, nil).Times(2)

	// Mock policy repo to return empty policies (no blocking)
	mockPolicyRepo.On("GetActiveByOrganization", agent.OrganizationID).Return([]*domain.SecurityPolicy{}, nil).Maybe()
	mockPolicyRepo.On("GetByType", agent.OrganizationID, mock.Anything).Return([]*domain.SecurityPolicy{}, nil).Maybe()

	ctx := context.Background()

	// Test wildcard matches read
	allowed, _, _, err := service.VerifyAction(ctx, agent.ID, "file:read", "/test.txt", nil)
	assert.NoError(t, err)
	assert.True(t, allowed)

	// Test wildcard matches write
	allowed, _, _, err = service.VerifyAction(ctx, agent.ID, "file:write", "/test.txt", nil)
	assert.NoError(t, err)
	assert.True(t, allowed)

	mockAgentRepo.AssertExpectations(t)
	mockCapabilityRepo.AssertExpectations(t)
}

// ===========================
// matchesCapability Tests
// ===========================

func TestAgentService_matchesCapability_Patterns(t *testing.T) {
	service := &AgentService{}

	tests := []struct {
		name       string
		actionType string
		resource   string
		capability string
		expected   bool
	}{
		{"exact match", "file:read", "/test.txt", "file:read", true},
		{"wildcard match", "file:read", "/test.txt", "file:*", true},
		{"no match", "file:write", "/test.txt", "file:read", false},
		{"wrong prefix", "db:query", "/database", "file:*", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.matchesCapability(tt.actionType, tt.resource, tt.capability)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ===========================
// shouldAutoVerifyAgent Tests
// ===========================

func TestAgentService_shouldAutoVerifyAgent_Conditions(t *testing.T) {
	service := &AgentService{}

	stringPtr := func(s string) *string { return &s }

	tests := []struct {
		name     string
		agent    *domain.Agent
		expected bool
	}{
		{
			name: "valid agent - should auto-verify",
			agent: &domain.Agent{
				Name:                "test",
				DisplayName:         "Test",
				Description:         "Test description",
				TrustScore:          0.85,
				PublicKey:           stringPtr("key"),
				EncryptedPrivateKey: stringPtr("encrypted"),
			},
			expected: true,
		},
		{
			name: "low trust score - should NOT auto-verify",
			agent: &domain.Agent{
				Name:                "test",
				DisplayName:         "Test",
				Description:         "Test description",
				TrustScore:          0.2,
				PublicKey:           stringPtr("key"),
				EncryptedPrivateKey: stringPtr("encrypted"),
			},
			expected: false,
		},
		{
			name: "missing keys - should NOT auto-verify",
			agent: &domain.Agent{
				Name:                "test",
				DisplayName:         "Test",
				Description:         "Test description",
				TrustScore:          0.85,
				PublicKey:           nil,
				EncryptedPrivateKey: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.shouldAutoVerifyAgent(tt.agent)
			assert.Equal(t, tt.expected, result)
		})
	}
}

