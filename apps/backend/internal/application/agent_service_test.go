package application

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
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

func (m *AgentServiceMockTrustScoreRepository) UpdateScore(agentID uuid.UUID, newScore float64) error {
	args := m.Called(agentID, newScore)
	return args.Error(0)
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

// Note: VerifyAction tests removed - that method is on CapabilityService, not AgentService
// See capability_service_test.go for VerifyAction tests

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

// ===========================
// isValidAgentType Tests
// ===========================

func TestIsValidAgentType(t *testing.T) {
	tests := []struct {
		agentType domain.AgentType
		expected  bool
	}{
		// LLM Providers
		{domain.AgentTypeClaude, true},
		{domain.AgentTypeGPT, true},
		{domain.AgentTypeGemini, true},
		{domain.AgentTypeLlama, true},
		{domain.AgentTypeMistral, true},
		{domain.AgentTypeCohere, true},
		// Frameworks
		{domain.AgentTypeLangChain, true},
		{domain.AgentTypeLlamaIndex, true},
		{domain.AgentTypeAutoGen, true},
		{domain.AgentTypeCrewAI, true},
		{domain.AgentTypeLangGraph, true},
		{domain.AgentTypeHaystack, true},
		{domain.AgentTypeSemanticKernel, true},
		// Copilots & Assistants
		{domain.AgentTypeCopilot, true},
		{domain.AgentTypeAssistant, true},
		{domain.AgentTypeChatbot, true},
		// Autonomous
		{domain.AgentTypeAutoGPT, true},
		{domain.AgentTypeBabyAGI, true},
		// Generic
		{domain.AgentTypeCustom, true},
		{domain.AgentTypeAI, true},
		// Invalid type
		{domain.AgentType("invalid_type"), false},
		{domain.AgentType(""), false},
	}

	for _, tt := range tests {
		t.Run(string(tt.agentType), func(t *testing.T) {
			result := isValidAgentType(tt.agentType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ===========================
// sanitizeTalksTo Tests
// ===========================

func TestSanitizeTalksTo(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "nil slice",
			input:    nil,
			expected: nil,
		},
		{
			name:     "clean entries",
			input:    []string{"memory", "aws-terraform", "github"},
			expected: []string{"memory", "aws-terraform", "github"},
		},
		{
			name:     "entries with whitespace",
			input:    []string{"  memory  ", "  aws-terraform  "},
			expected: []string{"memory", "aws-terraform"},
		},
		{
			name:     "comma-separated malformed entry",
			input:    []string{"memory,aws-terraform,github"},
			expected: []string{"memory", "aws-terraform", "github"},
		},
		{
			name:     "mixed clean and malformed",
			input:    []string{"clean-server", "server1,server2"},
			expected: []string{"clean-server", "server1", "server2"},
		},
		{
			name:     "duplicate entries",
			input:    []string{"memory", "memory", "github"},
			expected: []string{"memory", "github"},
		},
		{
			name:     "empty entries filtered out",
			input:    []string{"memory", "", "github", ""},
			expected: []string{"memory", "github"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeTalksTo(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ===========================
// ListAgents Tests
// ===========================

func TestAgentService_ListAgents_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	orgID := uuid.New()
	expectedAgents := []*domain.Agent{
		createTestAgentForService(),
		createTestAgentForService(),
	}

	mockAgentRepo.On("GetByOrganization", orgID).Return(expectedAgents, nil)

	ctx := context.Background()
	agents, err := service.ListAgents(ctx, orgID)

	assert.NoError(t, err)
	assert.Len(t, agents, 2)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_ListAgents_Empty(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	orgID := uuid.New()
	mockAgentRepo.On("GetByOrganization", orgID).Return([]*domain.Agent{}, nil)

	ctx := context.Background()
	agents, err := service.ListAgents(ctx, orgID)

	assert.NoError(t, err)
	assert.Empty(t, agents)
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// VerifyAgent Tests
// ===========================

func TestAgentService_VerifyAgent_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
	}

	agent := createTestAgentForService()
	agent.Status = domain.AgentStatusPending

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{
		ID:      uuid.New(),
		AgentID: agent.ID,
		Score:   0.90,
	}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", newTrustScore).Return(nil)

	ctx := context.Background()
	err := service.VerifyAgent(ctx, agent.ID)

	assert.NoError(t, err)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_VerifyAgent_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	ctx := context.Background()
	err := service.VerifyAgent(ctx, agentID)

	assert.Error(t, err)
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// SuspendAgent Tests
// ===========================

func TestAgentService_SuspendAgent_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
	}

	agent := createTestAgentForService()
	agent.Status = domain.AgentStatusVerified

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{
		ID:      uuid.New(),
		AgentID: agent.ID,
		Score:   0.30, // Lower score after suspension
	}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", newTrustScore).Return(nil)

	ctx := context.Background()
	err := service.SuspendAgent(ctx, agent.ID)

	assert.NoError(t, err)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_SuspendAgent_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	ctx := context.Background()
	err := service.SuspendAgent(ctx, agentID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// ReactivateAgent Tests
// ===========================

func TestAgentService_ReactivateAgent_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
	}

	agent := createTestAgentForService()
	agent.Status = domain.AgentStatusSuspended

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{
		ID:      uuid.New(),
		AgentID: agent.ID,
		Score:   0.75,
	}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", newTrustScore).Return(nil)

	ctx := context.Background()
	err := service.ReactivateAgent(ctx, agent.ID)

	assert.NoError(t, err)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_ReactivateAgent_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	ctx := context.Background()
	err := service.ReactivateAgent(ctx, agentID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// GetAgentByName Tests
// ===========================

func TestAgentService_GetAgentByName_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	orgID := uuid.New()
	agentName := "my-test-agent"
	expectedAgent := createTestAgentForService()
	expectedAgent.Name = agentName
	expectedAgent.OrganizationID = orgID

	mockAgentRepo.On("GetByName", orgID, agentName).Return(expectedAgent, nil)

	ctx := context.Background()
	agent, err := service.GetAgentByName(ctx, orgID, agentName)

	assert.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, agentName, agent.Name)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_GetAgentByName_NotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	orgID := uuid.New()
	agentName := "nonexistent-agent"
	mockAgentRepo.On("GetByName", orgID, agentName).Return(nil, errors.New("agent not found"))

	ctx := context.Background()
	agent, err := service.GetAgentByName(ctx, orgID, agentName)

	assert.Error(t, err)
	assert.Nil(t, agent)
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// HasCapability Tests
// ===========================

func TestAgentService_HasCapability_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	agentID := uuid.New()
	capabilities := []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:read"},
		{ID: uuid.New(), AgentID: agentID, CapabilityType: "api:call"},
	}

	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return(capabilities, nil)

	ctx := context.Background()

	// Should match
	hasRead, err := service.HasCapability(ctx, agentID, "file:read", "/test.txt")
	assert.NoError(t, err)
	assert.True(t, hasRead)

	// Should not match
	hasWrite, err := service.HasCapability(ctx, agentID, "file:write", "/test.txt")
	assert.NoError(t, err)
	assert.False(t, hasWrite)
}

func TestAgentService_HasCapability_NoCapabilities(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	agentID := uuid.New()
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return([]*domain.AgentCapability{}, nil)

	ctx := context.Background()
	hasCapability, err := service.HasCapability(ctx, agentID, "file:read", "/test.txt")

	assert.NoError(t, err)
	assert.False(t, hasCapability)
}

func TestAgentService_HasCapability_WildcardMatch(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	agentID := uuid.New()
	capabilities := []*domain.AgentCapability{
		{ID: uuid.New(), AgentID: agentID, CapabilityType: "file:*"},
	}

	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agentID).Return(capabilities, nil)

	ctx := context.Background()

	// Wildcard should match any file operation
	hasRead, err := service.HasCapability(ctx, agentID, "file:read", "/test.txt")
	assert.NoError(t, err)
	assert.True(t, hasRead)

	hasWrite, err := service.HasCapability(ctx, agentID, "file:write", "/test.txt")
	assert.NoError(t, err)
	assert.True(t, hasWrite)

	// Should not match different prefix
	hasDb, err := service.HasCapability(ctx, agentID, "db:query", "/database")
	assert.NoError(t, err)
	assert.False(t, hasDb)
}

// ===========================
// calculateViolationSeverity Tests
// ===========================

func TestAgentService_calculateViolationSeverity(t *testing.T) {
	service := &AgentService{}

	tests := []struct {
		name      string
		agent     *domain.Agent
		isBlocked bool
		expected  string
	}{
		{
			name:      "compromised agent - critical",
			agent:     &domain.Agent{TrustScore: 0.85, IsCompromised: true},
			isBlocked: true,
			expected:  "critical",
		},
		{
			name:      "very low trust score - critical",
			agent:     &domain.Agent{TrustScore: 0.20, IsCompromised: false},
			isBlocked: true,
			expected:  "critical",
		},
		{
			name:      "low trust score blocked - high",
			agent:     &domain.Agent{TrustScore: 0.40, IsCompromised: false},
			isBlocked: true,
			expected:  "high",
		},
		{
			name:      "medium trust score blocked - medium",
			agent:     &domain.Agent{TrustScore: 0.65, IsCompromised: false},
			isBlocked: true,
			expected:  "medium",
		},
		{
			name:      "low trust score not blocked - medium",
			agent:     &domain.Agent{TrustScore: 0.40, IsCompromised: false},
			isBlocked: false,
			expected:  "medium",
		},
		{
			name:      "high trust score not blocked - low",
			agent:     &domain.Agent{TrustScore: 0.85, IsCompromised: false},
			isBlocked: false,
			expected:  "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.calculateViolationSeverity(tt.agent, tt.isBlocked)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ===========================
// calculateTrustScoreImpact Tests
// ===========================

func TestAgentService_calculateTrustScoreImpact(t *testing.T) {
	service := &AgentService{}

	tests := []struct {
		name      string
		isBlocked bool
		expected  int
	}{
		{"blocked violation", true, -10},
		{"alert-only violation", false, -5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.calculateTrustScoreImpact(tt.isBlocked)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ===========================
// AddMCPServers Tests
// ===========================

func TestAgentService_AddMCPServers_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
	}

	agent := createTestAgentForService()
	agent.TalksTo = []string{"existing-server"}

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{
		ID:      uuid.New(),
		AgentID: agent.ID,
		Score:   0.88,
	}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", newTrustScore).Return(nil)

	ctx := context.Background()
	updatedAgent, addedServers, err := service.AddMCPServers(ctx, agent.ID, []string{"new-server-1", "new-server-2"})

	assert.NoError(t, err)
	assert.NotNil(t, updatedAgent)
	assert.Len(t, addedServers, 2)
	assert.Contains(t, addedServers, "new-server-1")
	assert.Contains(t, addedServers, "new-server-2")
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_AddMCPServers_NoDuplicates(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)

	service := &AgentService{
		agentRepo: mockAgentRepo,
	}

	agent := createTestAgentForService()
	agent.TalksTo = []string{"existing-server"}

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	// No Update called because no new servers added

	ctx := context.Background()
	updatedAgent, addedServers, err := service.AddMCPServers(ctx, agent.ID, []string{"existing-server"})

	assert.NoError(t, err)
	assert.NotNil(t, updatedAgent)
	assert.Empty(t, addedServers) // No new servers added
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// RemoveMCPServers Tests
// ===========================

func TestAgentService_RemoveMCPServers_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
	}

	agent := createTestAgentForService()
	agent.TalksTo = []string{"server-1", "server-2", "server-3"}

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{
		ID:      uuid.New(),
		AgentID: agent.ID,
		Score:   0.82,
	}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", newTrustScore).Return(nil)

	ctx := context.Background()
	updatedAgent, removedServers, err := service.RemoveMCPServers(ctx, agent.ID, []string{"server-1", "server-3"})

	assert.NoError(t, err)
	assert.NotNil(t, updatedAgent)
	assert.Len(t, removedServers, 2)
	assert.Contains(t, removedServers, "server-1")
	assert.Contains(t, removedServers, "server-3")
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_RemoveMCPServers_EmptyTalksTo(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)

	service := &AgentService{
		agentRepo: mockAgentRepo,
	}

	agent := createTestAgentForService()
	agent.TalksTo = nil

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)

	ctx := context.Background()
	updatedAgent, removedServers, err := service.RemoveMCPServers(ctx, agent.ID, []string{"server-1"})

	assert.NoError(t, err)
	assert.NotNil(t, updatedAgent)
	assert.Empty(t, removedServers)
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// RemoveMCPServer (single) Tests
// ===========================

func TestAgentService_RemoveMCPServer_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
	}

	agent := createTestAgentForService()
	agent.TalksTo = []string{"server-1", "server-2"}

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{
		ID:      uuid.New(),
		AgentID: agent.ID,
		Score:   0.80,
	}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", newTrustScore).Return(nil)

	ctx := context.Background()
	updatedAgent, err := service.RemoveMCPServer(ctx, agent.ID, "server-1")

	assert.NoError(t, err)
	assert.NotNil(t, updatedAgent)
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// UpdateLastActive Tests
// ===========================

func TestAgentService_UpdateLastActive_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	ctx := context.Background()

	mockAgentRepo.On("UpdateLastActive", ctx, agentID).Return(nil)

	err := service.UpdateLastActive(ctx, agentID)

	assert.NoError(t, err)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_UpdateLastActive_Error(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	ctx := context.Background()

	mockAgentRepo.On("UpdateLastActive", ctx, agentID).Return(errors.New("database error"))

	err := service.UpdateLastActive(ctx, agentID)

	assert.Error(t, err)
	mockAgentRepo.AssertExpectations(t)
}

// ===========================
// CreateSecurityAlert Tests
// ===========================

func TestAgentService_CreateSecurityAlert_Success(t *testing.T) {
	mockAlertRepo := new(MockAlertRepository)
	service := &AgentService{alertRepo: mockAlertRepo}

	alert := &domain.Alert{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		AlertType:      domain.AlertSecurityBreach,
		Severity:       domain.AlertSeverityCritical,
		Title:          "Test Alert",
		Description:    "Test description",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
	}

	mockAlertRepo.On("Create", alert).Return(nil)

	ctx := context.Background()
	err := service.CreateSecurityAlert(ctx, alert)

	assert.NoError(t, err)
	mockAlertRepo.AssertExpectations(t)
}

// ===========================
// CreateCapabilityViolation Tests
// ===========================

func TestAgentService_CreateCapabilityViolation_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	agent := createTestAgentForService()

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)
	mockCapabilityRepo.On("CreateViolation", mock.AnythingOfType("*domain.CapabilityViolation")).Return(nil)

	ctx := context.Background()
	err := service.CreateCapabilityViolation(ctx, agent.ID, "file:write", "/sensitive.txt", "high", nil)

	assert.NoError(t, err)
	mockAgentRepo.AssertExpectations(t)
	mockCapabilityRepo.AssertExpectations(t)
}

func TestAgentService_CreateCapabilityViolation_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	ctx := context.Background()
	err := service.CreateCapabilityViolation(ctx, agentID, "file:write", "/sensitive.txt", "high", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to get agent")
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_CreateCapabilityViolation_SeverityMapping(t *testing.T) {
	agent := createTestAgentForService()

	tests := []struct {
		inputSeverity    string
		expectedSeverity string
		expectedImpact   int
	}{
		{"critical", "critical", -15},
		{"high", "high", -10},
		{"warning", "medium", -7},
		{"info", "low", -5},
		{"unknown", "low", -5}, // Default case
	}

	for _, tt := range tests {
		t.Run(tt.inputSeverity, func(t *testing.T) {
			mockAgentRepo := new(MockAgentRepository)
			mockCapabilityRepo := new(MockCapabilityRepository)

			service := &AgentService{
				agentRepo:      mockAgentRepo,
				capabilityRepo: mockCapabilityRepo,
			}

			mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
			mockCapabilityRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)
			mockCapabilityRepo.On("CreateViolation", mock.MatchedBy(func(v *domain.CapabilityViolation) bool {
				return v.Severity == tt.expectedSeverity && v.TrustScoreImpact == tt.expectedImpact
			})).Return(nil)

			ctx := context.Background()
			err := service.CreateCapabilityViolation(ctx, agent.ID, "test:action", "/resource", tt.inputSeverity, nil)

			assert.NoError(t, err)
			mockCapabilityRepo.AssertExpectations(t)
		})
	}
}

// ===========================
// NewAgentService Constructor Tests
// ===========================

func TestNewAgentService_NilDependencies(t *testing.T) {
	// Test that service can be created even with nil dependencies
	// (this is used in some test scenarios)
	service := NewAgentService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)

	assert.NotNil(t, service)
	assert.Nil(t, service.agentRepo)
	assert.Nil(t, service.trustCalc)
	assert.Nil(t, service.trustScoreRepo)
	assert.Nil(t, service.keyVault)
	assert.Nil(t, service.alertRepo)
	assert.Nil(t, service.policyService)
	assert.Nil(t, service.capabilityRepo)
	assert.Nil(t, service.verificationEventService)
	assert.Nil(t, service.tagRepo)
	assert.Nil(t, service.userRepo)
	assert.Nil(t, service.orgRepo)
}

func TestNewAgentService_WithMockedDependencies(t *testing.T) {
	// Test service creation with mock repositories
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockAlertRepo := new(MockAlertRepository)
	policyService := &SecurityPolicyService{}
	mockCapabilityRepo := new(MockCapabilityRepository)
	verificationEventService := &VerificationEventService{}

	service := NewAgentService(
		mockAgentRepo,
		mockTrustCalc,
		mockTrustScoreRepo,
		nil, // keyVault
		mockAlertRepo,
		policyService,
		mockCapabilityRepo,
		verificationEventService,
		nil, // tagRepo
		nil, // userRepo
		nil, // orgRepo
	)

	assert.NotNil(t, service)
	assert.NotNil(t, service.agentRepo)
	assert.NotNil(t, service.trustCalc)
	assert.NotNil(t, service.trustScoreRepo)
	assert.NotNil(t, service.alertRepo)
	assert.NotNil(t, service.policyService)
	assert.NotNil(t, service.capabilityRepo)
}

// ===========================
// parseClaudeDesktopConfig Tests
// ===========================

func TestParseClaudeDesktopConfig_ValidConfig(t *testing.T) {
	// Create a temp file with valid Claude Desktop config
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "claude_desktop_config.json")

	configJSON := `{
		"mcpServers": {
			"memory": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-memory"]
			},
			"filesystem": {
				"command": "node",
				"args": ["/path/to/server.js"],
				"env": {
					"NODE_ENV": "production"
				}
			}
		}
	}`

	err := os.WriteFile(configPath, []byte(configJSON), 0644)
	assert.NoError(t, err)

	service := &AgentService{}
	servers, err := service.parseClaudeDesktopConfig(configPath)

	assert.NoError(t, err)
	assert.Len(t, servers, 2)

	// Check that servers are parsed correctly
	serverNames := make(map[string]bool)
	for _, server := range servers {
		serverNames[server.Name] = true
		assert.Equal(t, 100.0, server.Confidence)
		assert.Equal(t, "claude_desktop_config", server.Source)
	}
	assert.True(t, serverNames["memory"])
	assert.True(t, serverNames["filesystem"])
}

func TestParseClaudeDesktopConfig_EmptyMCPServers(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "claude_desktop_config.json")

	configJSON := `{
		"mcpServers": {}
	}`

	err := os.WriteFile(configPath, []byte(configJSON), 0644)
	assert.NoError(t, err)

	service := &AgentService{}
	servers, err := service.parseClaudeDesktopConfig(configPath)

	assert.NoError(t, err)
	assert.Empty(t, servers)
}

func TestParseClaudeDesktopConfig_FileNotFound(t *testing.T) {
	service := &AgentService{}
	_, err := service.parseClaudeDesktopConfig("/nonexistent/path/config.json")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestParseClaudeDesktopConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid_config.json")

	err := os.WriteFile(configPath, []byte("not valid json"), 0644)
	assert.NoError(t, err)

	service := &AgentService{}
	_, err = service.parseClaudeDesktopConfig(configPath)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config JSON")
}

func TestParseClaudeDesktopConfig_WithEnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config_with_env.json")

	configJSON := `{
		"mcpServers": {
			"database": {
				"command": "python",
				"args": ["server.py"],
				"env": {
					"DB_HOST": "localhost",
					"DB_PORT": "5432",
					"DEBUG": "true"
				}
			}
		}
	}`

	err := os.WriteFile(configPath, []byte(configJSON), 0644)
	assert.NoError(t, err)

	service := &AgentService{}
	servers, err := service.parseClaudeDesktopConfig(configPath)

	assert.NoError(t, err)
	assert.Len(t, servers, 1)
	assert.Equal(t, "database", servers[0].Name)
	assert.Equal(t, "python", servers[0].Command)
	assert.Equal(t, []string{"server.py"}, servers[0].Args)
	assert.Equal(t, "localhost", servers[0].Env["DB_HOST"])
	assert.Equal(t, "5432", servers[0].Env["DB_PORT"])
}

func TestParseClaudeDesktopConfig_NoMCPServersKey(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "no_mcp_servers.json")

	configJSON := `{
		"someOtherKey": "value"
	}`

	err := os.WriteFile(configPath, []byte(configJSON), 0644)
	assert.NoError(t, err)

	service := &AgentService{}
	servers, err := service.parseClaudeDesktopConfig(configPath)

	assert.NoError(t, err)
	assert.Empty(t, servers)
}

// ===========================
// CreateAgent Tests
// ===========================

func TestAgentService_CreateAgent_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	// Initialize key vault for testing (base64 encoded 32-byte key)
	masterKey := base64.StdEncoding.EncodeToString([]byte("test-master-key-32-bytes-long!!!"))
	keyVault, _ := crypto.NewKeyVault(masterKey)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
		keyVault:       keyVault,
		capabilityRepo: mockCapabilityRepo,
	}

	orgID := uuid.New()
	userID := uuid.New()

	req := &CreateAgentRequest{
		Name:        "test-agent",
		DisplayName: "Test Agent",
		Description: "A test agent",
		AgentType:   domain.AgentTypeAI,
		Version:     "1.0.0",
	}

	// Setup expectations
	mockAgentRepo.On("Create", mock.AnythingOfType("*domain.Agent")).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{
		ID:      uuid.New(),
		Score:   0.75,
	}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, req, orgID, userID, nil, nil, "")

	assert.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, "test-agent", agent.Name)
	assert.Equal(t, "Test Agent", agent.DisplayName)
	assert.Equal(t, orgID, agent.OrganizationID)
	assert.NotNil(t, agent.PublicKey)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_CreateAgent_WithPublicKey(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	orgID := uuid.New()
	userID := uuid.New()

	// SDK provides its own public key
	req := &CreateAgentRequest{
		Name:        "sdk-agent",
		DisplayName: "SDK Agent",
		AgentType:   domain.AgentTypeAI,
		PublicKey:   "sdk-provided-public-key-base64",
	}

	mockAgentRepo.On("Create", mock.AnythingOfType("*domain.Agent")).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{
		ID:    uuid.New(),
		Score: 0.80,
	}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, req, orgID, userID, nil, nil, "")

	assert.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, "sdk-provided-public-key-base64", *agent.PublicKey)
	assert.Equal(t, "Ed25519", agent.KeyAlgorithm)
	// No encrypted private key since SDK provided its own
	assert.Nil(t, agent.EncryptedPrivateKey)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_CreateAgent_MissingName(t *testing.T) {
	service := &AgentService{}

	req := &CreateAgentRequest{
		DisplayName: "Test Agent",
		AgentType:   domain.AgentTypeAI,
	}

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, req, uuid.New(), uuid.New(), nil, nil, "")

	assert.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "name and display_name are required")
}

func TestAgentService_CreateAgent_MissingDisplayName(t *testing.T) {
	service := &AgentService{}

	req := &CreateAgentRequest{
		Name:      "test-agent",
		AgentType: domain.AgentTypeAI,
	}

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, req, uuid.New(), uuid.New(), nil, nil, "")

	assert.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "name and display_name are required")
}

func TestAgentService_CreateAgent_InvalidAgentType(t *testing.T) {
	service := &AgentService{}

	req := &CreateAgentRequest{
		Name:        "test-agent",
		DisplayName: "Test Agent",
		AgentType:   domain.AgentType("invalid_type"),
	}

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, req, uuid.New(), uuid.New(), nil, nil, "")

	assert.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "invalid agent_type")
}

func TestAgentService_CreateAgent_WithCapabilities(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	orgID := uuid.New()
	userID := uuid.New()

	req := &CreateAgentRequest{
		Name:         "agent-with-caps",
		DisplayName:  "Agent with Capabilities",
		AgentType:    domain.AgentTypeAI,
		PublicKey:    "test-key",
		Capabilities: []string{"file:read", "api:call"},
	}

	mockAgentRepo.On("Create", mock.AnythingOfType("*domain.Agent")).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{
		ID:    uuid.New(),
		Score: 0.75,
	}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)
	mockCapabilityRepo.On("CreateCapability", mock.AnythingOfType("*domain.AgentCapability")).Return(nil)

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, req, orgID, userID, nil, nil, "")

	assert.NoError(t, err)
	assert.NotNil(t, agent)
	// Verify capabilities were passed to repo
	mockCapabilityRepo.AssertNumberOfCalls(t, "CreateCapability", 2)
}

func TestAgentService_CreateAgent_WithTalksTo(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	orgID := uuid.New()
	userID := uuid.New()

	req := &CreateAgentRequest{
		Name:        "mcp-connected-agent",
		DisplayName: "MCP Connected Agent",
		AgentType:   domain.AgentTypeAI,
		PublicKey:   "test-key",
		TalksTo:     []string{"memory", "filesystem", "github"},
	}

	mockAgentRepo.On("Create", mock.MatchedBy(func(a *domain.Agent) bool {
		return len(a.TalksTo) == 3 &&
			a.TalksTo[0] == "memory" &&
			a.TalksTo[1] == "filesystem" &&
			a.TalksTo[2] == "github"
	})).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{
		ID:    uuid.New(),
		Score: 0.75,
	}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, req, orgID, userID, nil, nil, "")

	assert.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, []string{"memory", "filesystem", "github"}, agent.TalksTo)
}

func TestAgentService_CreateAgent_SanitizesMalformedTalksTo(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	orgID := uuid.New()
	userID := uuid.New()

	// Malformed talksTo with comma-separated values in single string
	req := &CreateAgentRequest{
		Name:        "malformed-talks-to-agent",
		DisplayName: "Malformed TalksTo Agent",
		AgentType:   domain.AgentTypeAI,
		PublicKey:   "test-key",
		TalksTo:     []string{"memory,filesystem,github"}, // Malformed!
	}

	// Should be sanitized to 3 separate entries
	mockAgentRepo.On("Create", mock.MatchedBy(func(a *domain.Agent) bool {
		return len(a.TalksTo) == 3
	})).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{ID: uuid.New(), Score: 0.75}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, req, orgID, userID, nil, nil, "")

	assert.NoError(t, err)
	assert.NotNil(t, agent)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_CreateAgent_RepoError(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	req := &CreateAgentRequest{
		Name:        "test-agent",
		DisplayName: "Test Agent",
		AgentType:   domain.AgentTypeAI,
		PublicKey:   "test-key",
	}

	mockAgentRepo.On("Create", mock.AnythingOfType("*domain.Agent")).Return(errors.New("database error"))

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, req, uuid.New(), uuid.New(), nil, nil, "")

	assert.Error(t, err)
	assert.Nil(t, agent)
	assert.Contains(t, err.Error(), "failed to create agent")
}

func TestAgentService_CreateAgent_WithSDKToken(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)
	mockCapabilityRepo := new(MockCapabilityRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
		capabilityRepo: mockCapabilityRepo,
	}

	orgID := uuid.New()
	userID := uuid.New()
	sdkTokenID := uuid.New()

	req := &CreateAgentRequest{
		Name:        "sdk-token-agent",
		DisplayName: "SDK Token Agent",
		AgentType:   domain.AgentTypeAI,
		PublicKey:   "test-key",
	}

	// Verify SDK token ID is stored
	mockAgentRepo.On("Create", mock.MatchedBy(func(a *domain.Agent) bool {
		return a.CreatedBySDKTokenID != nil && *a.CreatedBySDKTokenID == sdkTokenID
	})).Return(nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{ID: uuid.New(), Score: 0.75}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", mock.AnythingOfType("*domain.TrustScore")).Return(nil)

	ctx := context.Background()
	agent, err := service.CreateAgent(ctx, req, orgID, userID, &sdkTokenID, nil, "")

	assert.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, &sdkTokenID, agent.CreatedBySDKTokenID)
}

// ===========================
// UpdateAgent Tests
// ===========================

func TestAgentService_UpdateAgent_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockTrustCalc := new(AgentServiceMockTrustScoreCalculator)
	mockTrustScoreRepo := new(AgentServiceMockTrustScoreRepository)

	service := &AgentService{
		agentRepo:      mockAgentRepo,
		trustCalc:      mockTrustCalc,
		trustScoreRepo: mockTrustScoreRepo,
	}

	existingAgent := createTestAgentForService()
	mockAgentRepo.On("GetByID", existingAgent.ID).Return(existingAgent, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(nil)

	newTrustScore := &domain.TrustScore{ID: uuid.New(), AgentID: existingAgent.ID, Score: 0.88}
	mockTrustCalc.On("Calculate", mock.AnythingOfType("*domain.Agent")).Return(newTrustScore, nil)
	mockTrustScoreRepo.On("Create", newTrustScore).Return(nil)

	// UpdateAgent uses CreateAgentRequest
	req := &CreateAgentRequest{
		DisplayName: "Updated Display Name",
		Description: "Updated description",
		Version:     "2.0.0",
	}

	ctx := context.Background()
	agent, err := service.UpdateAgent(ctx, existingAgent.ID, req)

	assert.NoError(t, err)
	assert.NotNil(t, agent)
	assert.Equal(t, "Updated Display Name", agent.DisplayName)
	assert.Equal(t, "Updated description", agent.Description)
	assert.Equal(t, "2.0.0", agent.Version)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_UpdateAgent_NotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	req := &CreateAgentRequest{DisplayName: "New Name"}

	ctx := context.Background()
	agent, err := service.UpdateAgent(ctx, agentID, req)

	assert.Error(t, err)
	assert.Nil(t, agent)
}

// ===========================
// MCPServerRepository Mock for GetAgentMCPServers tests
// ===========================

type AgentServiceMockMCPServerRepository struct {
	mock.Mock
}

func (m *AgentServiceMockMCPServerRepository) Create(server *domain.MCPServer) error {
	args := m.Called(server)
	return args.Error(0)
}

func (m *AgentServiceMockMCPServerRepository) GetByID(id uuid.UUID) (*domain.MCPServer, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

func (m *AgentServiceMockMCPServerRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.MCPServer, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MCPServer), args.Error(1)
}

func (m *AgentServiceMockMCPServerRepository) GetByURL(url string) (*domain.MCPServer, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

func (m *AgentServiceMockMCPServerRepository) Update(server *domain.MCPServer) error {
	args := m.Called(server)
	return args.Error(0)
}

func (m *AgentServiceMockMCPServerRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *AgentServiceMockMCPServerRepository) List(limit, offset int) ([]*domain.MCPServer, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MCPServer), args.Error(1)
}

func (m *AgentServiceMockMCPServerRepository) GetVerificationStatus(id uuid.UUID) (*domain.MCPServerVerificationStatus, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServerVerificationStatus), args.Error(1)
}

// ===========================
// RotateCredentials Tests
// ===========================

func TestAgentService_RotateCredentials_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)

	// Initialize key vault for testing
	masterKey := base64.StdEncoding.EncodeToString([]byte("test-master-key-32-bytes-long!!!"))
	keyVault, err := crypto.NewKeyVault(masterKey)
	assert.NoError(t, err)

	service := &AgentService{
		agentRepo: mockAgentRepo,
		keyVault:  keyVault,
	}

	agent := createTestAgentForService()
	oldPublicKey := "old-public-key-base64"
	agent.PublicKey = &oldPublicKey

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAgentRepo.On("Update", mock.MatchedBy(func(a *domain.Agent) bool {
		// Verify new keys were generated and old key is preserved
		return a.PublicKey != nil &&
			*a.PublicKey != oldPublicKey &&
			a.PreviousPublicKey != nil &&
			*a.PreviousPublicKey == oldPublicKey &&
			a.RotationCount == 1
	})).Return(nil)

	ctx := context.Background()
	publicKey, privateKey, err := service.RotateCredentials(ctx, agent.ID)

	assert.NoError(t, err)
	assert.NotEmpty(t, publicKey)
	assert.NotEmpty(t, privateKey)
	assert.NotEqual(t, oldPublicKey, publicKey)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_RotateCredentials_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	ctx := context.Background()
	publicKey, privateKey, err := service.RotateCredentials(ctx, agentID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
	assert.Empty(t, publicKey)
	assert.Empty(t, privateKey)
}

func TestAgentService_RotateCredentials_UpdateError(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)

	masterKey := base64.StdEncoding.EncodeToString([]byte("test-master-key-32-bytes-long!!!"))
	keyVault, _ := crypto.NewKeyVault(masterKey)

	service := &AgentService{
		agentRepo: mockAgentRepo,
		keyVault:  keyVault,
	}

	agent := createTestAgentForService()
	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAgentRepo.On("Update", mock.AnythingOfType("*domain.Agent")).Return(errors.New("database error"))

	ctx := context.Background()
	publicKey, privateKey, err := service.RotateCredentials(ctx, agent.ID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update")
	assert.Empty(t, publicKey)
	assert.Empty(t, privateKey)
}

// ===========================
// GetAgentMCPServers Tests
// ===========================

func TestAgentService_GetAgentMCPServers_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockMCPRepo := new(AgentServiceMockMCPServerRepository)

	service := &AgentService{
		agentRepo: mockAgentRepo,
	}

	agent := createTestAgentForService()
	agent.TalksTo = []string{"memory", "github"}

	memoryServer := &domain.MCPServer{
		ID:             uuid.New(),
		Name:           "memory",
		OrganizationID: agent.OrganizationID,
	}
	githubServer := &domain.MCPServer{
		ID:             uuid.New(),
		Name:           "github",
		OrganizationID: agent.OrganizationID,
	}
	otherServer := &domain.MCPServer{
		ID:             uuid.New(),
		Name:           "slack",
		OrganizationID: agent.OrganizationID,
	}

	allServers := []*domain.MCPServer{memoryServer, githubServer, otherServer}

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockMCPRepo.On("GetByOrganization", agent.OrganizationID).Return(allServers, nil)

	ctx := context.Background()
	result, err := service.GetAgentMCPServers(ctx, agent.ID, mockMCPRepo)

	assert.NoError(t, err)
	assert.Len(t, result, 2) // Only memory and github, not slack
	mockAgentRepo.AssertExpectations(t)
	mockMCPRepo.AssertExpectations(t)
}

func TestAgentService_GetAgentMCPServers_NoTalksTo(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)

	service := &AgentService{
		agentRepo: mockAgentRepo,
	}

	agent := createTestAgentForService()
	agent.TalksTo = nil // No talks_to

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)

	ctx := context.Background()
	result, err := service.GetAgentMCPServers(ctx, agent.ID, nil)

	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestAgentService_GetAgentMCPServers_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	ctx := context.Background()
	result, err := service.GetAgentMCPServers(ctx, agentID, nil)

	assert.Error(t, err)
	assert.Nil(t, result)
}

// ===========================
// UpdateAgentPublicKey Tests
// ===========================

func TestAgentService_UpdateAgentPublicKey_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)

	masterKey := base64.StdEncoding.EncodeToString([]byte("test-master-key-32-bytes-long!!!"))
	keyVault, _ := crypto.NewKeyVault(masterKey)

	service := &AgentService{
		agentRepo: mockAgentRepo,
		keyVault:  keyVault,
	}

	agent := createTestAgentForService()
	newPublicKey := "new-sdk-public-key-base64"

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAgentRepo.On("Update", mock.MatchedBy(func(a *domain.Agent) bool {
		return a.PublicKey != nil && *a.PublicKey == newPublicKey
	})).Return(nil)

	ctx := context.Background()
	err := service.UpdateAgentPublicKey(ctx, agent.ID, newPublicKey)

	assert.NoError(t, err)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_UpdateAgentPublicKey_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	ctx := context.Background()
	err := service.UpdateAgentPublicKey(ctx, agentID, "new-key")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
}

// ===========================
// LogCapabilityResult Tests
// ===========================

func TestAgentService_LogCapabilityResult_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)

	service := &AgentService{
		agentRepo:                mockAgentRepo,
		verificationEventService: nil, // nil is handled by the code
	}

	agentID := uuid.New()
	auditID := uuid.New()
	orgID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)

	ctx := context.Background()
	result := map[string]interface{}{"capability": "read_file"}
	err := service.LogCapabilityResult(ctx, agentID, auditID, true, "", result)

	assert.NoError(t, err)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_LogCapabilityResult_Failure(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)

	service := &AgentService{
		agentRepo:                mockAgentRepo,
		verificationEventService: nil, // nil is handled by the code
	}

	agentID := uuid.New()
	auditID := uuid.New()
	orgID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)

	ctx := context.Background()
	err := service.LogCapabilityResult(ctx, agentID, auditID, false, "access denied", nil)

	assert.NoError(t, err)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_LogCapabilityResult_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	ctx := context.Background()
	err := service.LogCapabilityResult(ctx, agentID, uuid.New(), true, "", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
}

func TestAgentService_LogCapabilityResult_WithAlertCreation(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)

	service := &AgentService{
		agentRepo:                mockAgentRepo,
		alertRepo:                mockAlertRepo,
		verificationEventService: nil, // nil is handled by the code
	}

	agentID := uuid.New()
	auditID := uuid.New()
	orgID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockAlertRepo.On("Create", mock.Anything).Return(nil)

	ctx := context.Background()
	result := map[string]interface{}{
		"create_alert": true,
	}
	err := service.LogCapabilityResult(ctx, agentID, auditID, false, "repeated failures", result)

	assert.NoError(t, err)
	mockAlertRepo.AssertExpectations(t)
}

// ===========================
// GetAgentCredentials Tests
// ===========================

func TestAgentService_GetAgentCredentials_Success(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)

	// Create a test key vault
	masterKeyBytes := make([]byte, 32)
	for i := range masterKeyBytes {
		masterKeyBytes[i] = byte(i)
	}
	masterKey := base64.StdEncoding.EncodeToString(masterKeyBytes)
	keyVault, err := crypto.NewKeyVault(masterKey)
	assert.NoError(t, err)

	service := &AgentService{
		agentRepo: mockAgentRepo,
		keyVault:  keyVault,
	}

	agentID := uuid.New()
	publicKey := "test-public-key-base64"
	privateKey := "test-private-key-base64"

	// Encrypt the private key for storage
	encryptedPrivateKey, err := keyVault.EncryptPrivateKey(privateKey)
	assert.NoError(t, err)

	agent := &domain.Agent{
		ID:                  agentID,
		PublicKey:           &publicKey,
		EncryptedPrivateKey: &encryptedPrivateKey,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)

	ctx := context.Background()
	gotPublic, gotPrivate, err := service.GetAgentCredentials(ctx, agentID)

	assert.NoError(t, err)
	assert.Equal(t, publicKey, gotPublic)
	assert.Equal(t, privateKey, gotPrivate)
	mockAgentRepo.AssertExpectations(t)
}

func TestAgentService_GetAgentCredentials_AgentNotFound(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("not found"))

	ctx := context.Background()
	_, _, err := service.GetAgentCredentials(ctx, agentID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")
}

func TestAgentService_GetAgentCredentials_KeysNotGenerated(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)
	service := &AgentService{agentRepo: mockAgentRepo}

	agentID := uuid.New()
	agent := &domain.Agent{
		ID:                  agentID,
		PublicKey:           nil,
		EncryptedPrivateKey: nil,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)

	ctx := context.Background()
	_, _, err := service.GetAgentCredentials(ctx, agentID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent keys not generated")
}

func TestAgentService_GetAgentCredentials_DecryptionFails(t *testing.T) {
	mockAgentRepo := new(MockAgentRepository)

	// Create a key vault with a different key than what was used to encrypt
	masterKeyBytes := make([]byte, 32)
	for i := range masterKeyBytes {
		masterKeyBytes[i] = byte(i)
	}
	masterKey := base64.StdEncoding.EncodeToString(masterKeyBytes)
	keyVault, err := crypto.NewKeyVault(masterKey)
	assert.NoError(t, err)

	service := &AgentService{
		agentRepo: mockAgentRepo,
		keyVault:  keyVault,
	}

	agentID := uuid.New()
	publicKey := "test-public-key"
	// Use invalid encrypted data that can't be decrypted
	invalidEncrypted := base64.StdEncoding.EncodeToString([]byte("this-is-not-valid-encrypted-data-because-its-too-short"))

	agent := &domain.Agent{
		ID:                  agentID,
		PublicKey:           &publicKey,
		EncryptedPrivateKey: &invalidEncrypted,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)

	ctx := context.Background()
	_, _, err = service.GetAgentCredentials(ctx, agentID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decrypt")
}
