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

// MockAgentRepository for API key service tests
type MockAgentRepoForAPIKey struct {
	mock.Mock
}

func (m *MockAgentRepoForAPIKey) Create(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepoForAPIKey) GetByID(id uuid.UUID) (*domain.Agent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAPIKey) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAPIKey) GetByPublicKeyHash(hash string) (*domain.Agent, error) {
	args := m.Called(hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAPIKey) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAPIKey) Update(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepoForAPIKey) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForAPIKey) Count(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockAgentRepoForAPIKey) Search(orgID uuid.UUID, query string, status *domain.AgentStatus, limit, offset int) ([]*domain.Agent, int, error) {
	args := m.Called(orgID, query, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Agent), args.Int(1), args.Error(2)
}

func (m *MockAgentRepoForAPIKey) CountByStatus(orgID uuid.UUID) (map[domain.AgentStatus]int, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.AgentStatus]int), args.Error(1)
}

func (m *MockAgentRepoForAPIKey) GetRecentlyActive(orgID uuid.UUID, hours int, limit int) ([]*domain.Agent, error) {
	args := m.Called(orgID, hours, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAPIKey) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockAgentRepoForAPIKey) List(limit, offset int) ([]*domain.Agent, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAPIKey) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	args := m.Called(id, newScore)
	return args.Error(0)
}

func (m *MockAgentRepoForAPIKey) IncrementViolationCount(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForAPIKey) MarkAsCompromised(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// MockAPIKeyRepoForAPIKey for API key service tests
type MockAPIKeyRepoForAPIKey struct {
	mock.Mock
}

func (m *MockAPIKeyRepoForAPIKey) Create(apiKey *domain.APIKey) error {
	args := m.Called(apiKey)
	return args.Error(0)
}

func (m *MockAPIKeyRepoForAPIKey) GetByID(id uuid.UUID) (*domain.APIKey, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepoForAPIKey) GetByHash(hash string) (*domain.APIKey, error) {
	args := m.Called(hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepoForAPIKey) GetByOrganization(orgID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepoForAPIKey) GetByAgent(agentID uuid.UUID) ([]*domain.APIKey, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.APIKey), args.Error(1)
}

func (m *MockAPIKeyRepoForAPIKey) UpdateLastUsed(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAPIKeyRepoForAPIKey) Revoke(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAPIKeyRepoForAPIKey) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// Helper function to create a test agent
func createTestAgentForAPIKey(orgID uuid.UUID) *domain.Agent {
	return &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
		DisplayName:    "Test Agent",
		AgentType:      domain.AgentTypeClaude,
		Status:         domain.AgentStatusVerified,
		TrustScore:     0.85,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
}

// ====================
// GenerateAPIKey Tests
// ====================

func TestAPIKeyService_GenerateAPIKey_Success(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	userID := uuid.New()
	agent := createTestAgentForAPIKey(orgID)

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAPIKeyRepo.On("Create", mock.AnythingOfType("*domain.APIKey")).Return(nil)

	// Act
	ctx := context.Background()
	fullKey, apiKey, err := service.GenerateAPIKey(ctx, agent.ID, orgID, userID, "Test Key", 30)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, fullKey)
	assert.NotNil(t, apiKey)
	assert.Contains(t, fullKey, "aim_live_")
	assert.Equal(t, "Test Key", apiKey.Name)
	assert.Equal(t, orgID, apiKey.OrganizationID)
	assert.Equal(t, agent.ID, apiKey.AgentID)
	assert.True(t, apiKey.IsActive)
	assert.NotNil(t, apiKey.ExpiresAt)

	mockAgentRepo.AssertExpectations(t)
	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_GenerateAPIKey_NoExpiry(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	userID := uuid.New()
	agent := createTestAgentForAPIKey(orgID)

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAPIKeyRepo.On("Create", mock.AnythingOfType("*domain.APIKey")).Return(nil)

	// Act - expiresInDays = 0 means no expiry
	ctx := context.Background()
	fullKey, apiKey, err := service.GenerateAPIKey(ctx, agent.ID, orgID, userID, "No Expiry Key", 0)

	// Assert
	assert.NoError(t, err)
	assert.NotEmpty(t, fullKey)
	assert.NotNil(t, apiKey)
	assert.Nil(t, apiKey.ExpiresAt)

	mockAgentRepo.AssertExpectations(t)
	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_GenerateAPIKey_AgentNotFound(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	agentID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("agent not found"))

	// Act
	ctx := context.Background()
	fullKey, apiKey, err := service.GenerateAPIKey(ctx, agentID, orgID, userID, "Test Key", 30)

	// Assert
	assert.Error(t, err)
	assert.Empty(t, fullKey)
	assert.Nil(t, apiKey)
	assert.Contains(t, err.Error(), "agent not found")

	mockAgentRepo.AssertExpectations(t)
}

func TestAPIKeyService_GenerateAPIKey_WrongOrganization(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	differentOrgID := uuid.New()
	userID := uuid.New()
	agent := createTestAgentForAPIKey(orgID)

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)

	// Act - Try to create API key with different org
	ctx := context.Background()
	fullKey, apiKey, err := service.GenerateAPIKey(ctx, agent.ID, differentOrgID, userID, "Test Key", 30)

	// Assert
	assert.Error(t, err)
	assert.Empty(t, fullKey)
	assert.Nil(t, apiKey)
	assert.Contains(t, err.Error(), "does not belong to organization")

	mockAgentRepo.AssertExpectations(t)
}

func TestAPIKeyService_GenerateAPIKey_CreateFails(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	userID := uuid.New()
	agent := createTestAgentForAPIKey(orgID)

	mockAgentRepo.On("GetByID", agent.ID).Return(agent, nil)
	mockAPIKeyRepo.On("Create", mock.AnythingOfType("*domain.APIKey")).Return(errors.New("database error"))

	// Act
	ctx := context.Background()
	fullKey, apiKey, err := service.GenerateAPIKey(ctx, agent.ID, orgID, userID, "Test Key", 30)

	// Assert
	assert.Error(t, err)
	assert.Empty(t, fullKey)
	assert.Nil(t, apiKey)
	assert.Contains(t, err.Error(), "failed to create API key")

	mockAgentRepo.AssertExpectations(t)
	mockAPIKeyRepo.AssertExpectations(t)
}

// ====================
// ListAPIKeys Tests
// ====================

func TestAPIKeyService_ListAPIKeys_Success(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	keys := []*domain.APIKey{
		{ID: uuid.New(), OrganizationID: orgID, Name: "Key 1"},
		{ID: uuid.New(), OrganizationID: orgID, Name: "Key 2"},
	}

	mockAPIKeyRepo.On("GetByOrganization", orgID).Return(keys, nil)

	// Act
	ctx := context.Background()
	result, err := service.ListAPIKeys(ctx, orgID)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Key 1", result[0].Name)
	assert.Equal(t, "Key 2", result[1].Name)

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_ListAPIKeys_Empty(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()

	mockAPIKeyRepo.On("GetByOrganization", orgID).Return([]*domain.APIKey{}, nil)

	// Act
	ctx := context.Background()
	result, err := service.ListAPIKeys(ctx, orgID)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 0)

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_ListAPIKeys_Error(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()

	mockAPIKeyRepo.On("GetByOrganization", orgID).Return(nil, errors.New("database error"))

	// Act
	ctx := context.Background()
	result, err := service.ListAPIKeys(ctx, orgID)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	mockAPIKeyRepo.AssertExpectations(t)
}

// ====================
// RevokeAPIKey Tests
// ====================

func TestAPIKeyService_RevokeAPIKey_Success(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	keyID := uuid.New()
	apiKey := &domain.APIKey{
		ID:             keyID,
		OrganizationID: orgID,
		Name:           "Test Key",
		IsActive:       true,
	}

	mockAPIKeyRepo.On("GetByID", keyID).Return(apiKey, nil)
	mockAPIKeyRepo.On("Revoke", keyID).Return(nil)

	// Act
	ctx := context.Background()
	err := service.RevokeAPIKey(ctx, keyID, orgID)

	// Assert
	assert.NoError(t, err)

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_RevokeAPIKey_NotFound(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	keyID := uuid.New()

	mockAPIKeyRepo.On("GetByID", keyID).Return(nil, errors.New("not found"))

	// Act
	ctx := context.Background()
	err := service.RevokeAPIKey(ctx, keyID, orgID)

	// Assert
	assert.Error(t, err)

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_RevokeAPIKey_WrongOrganization(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	differentOrgID := uuid.New()
	keyID := uuid.New()
	apiKey := &domain.APIKey{
		ID:             keyID,
		OrganizationID: orgID,
		Name:           "Test Key",
	}

	mockAPIKeyRepo.On("GetByID", keyID).Return(apiKey, nil)

	// Act
	ctx := context.Background()
	err := service.RevokeAPIKey(ctx, keyID, differentOrgID)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to organization")

	mockAPIKeyRepo.AssertExpectations(t)
}

// ====================
// DeleteAPIKey Tests
// ====================

func TestAPIKeyService_DeleteAPIKey_Success(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	keyID := uuid.New()
	apiKey := &domain.APIKey{
		ID:             keyID,
		OrganizationID: orgID,
		Name:           "Test Key",
		IsActive:       false, // Must be inactive to delete
	}

	mockAPIKeyRepo.On("GetByID", keyID).Return(apiKey, nil)
	mockAPIKeyRepo.On("Delete", keyID).Return(nil)

	// Act
	ctx := context.Background()
	err := service.DeleteAPIKey(ctx, keyID, orgID)

	// Assert
	assert.NoError(t, err)

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_DeleteAPIKey_StillActive(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	keyID := uuid.New()
	apiKey := &domain.APIKey{
		ID:             keyID,
		OrganizationID: orgID,
		Name:           "Test Key",
		IsActive:       true, // Still active, should fail
	}

	mockAPIKeyRepo.On("GetByID", keyID).Return(apiKey, nil)

	// Act
	ctx := context.Background()
	err := service.DeleteAPIKey(ctx, keyID, orgID)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be disabled before deletion")

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_DeleteAPIKey_WrongOrganization(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	differentOrgID := uuid.New()
	keyID := uuid.New()
	apiKey := &domain.APIKey{
		ID:             keyID,
		OrganizationID: orgID,
		IsActive:       false,
	}

	mockAPIKeyRepo.On("GetByID", keyID).Return(apiKey, nil)

	// Act
	ctx := context.Background()
	err := service.DeleteAPIKey(ctx, keyID, differentOrgID)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to organization")

	mockAPIKeyRepo.AssertExpectations(t)
}

// ====================
// ValidateAPIKey Tests
// ====================

func TestAPIKeyService_ValidateAPIKey_Success(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	// Note: The actual hash would be different, but we mock it
	orgID := uuid.New()
	keyID := uuid.New()
	apiKey := &domain.APIKey{
		ID:             keyID,
		OrganizationID: orgID,
		Name:           "Test Key",
		IsActive:       true,
		ExpiresAt:      nil, // No expiry
	}

	mockAPIKeyRepo.On("GetByHash", mock.AnythingOfType("string")).Return(apiKey, nil)
	mockAPIKeyRepo.On("UpdateLastUsed", keyID).Return(nil)

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, "aim_live_testkey123456789")

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, keyID, result.ID)

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_ValidateAPIKey_KeyNotFound(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	mockAPIKeyRepo.On("GetByHash", mock.AnythingOfType("string")).Return(nil, errors.New("not found"))

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, "aim_live_nonexistent")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_ValidateAPIKey_NilKey(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	mockAPIKeyRepo.On("GetByHash", mock.AnythingOfType("string")).Return(nil, nil)

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, "aim_live_nilkey")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid API key")

	mockAPIKeyRepo.AssertExpectations(t)
}

func TestAPIKeyService_ValidateAPIKey_Expired(t *testing.T) {
	// Arrange
	mockAgentRepo := new(MockAgentRepoForAPIKey)
	mockAPIKeyRepo := new(MockAPIKeyRepoForAPIKey)

	service := NewAPIKeyService(mockAPIKeyRepo, mockAgentRepo)

	orgID := uuid.New()
	keyID := uuid.New()
	expiredTime := time.Now().Add(-24 * time.Hour)
	apiKey := &domain.APIKey{
		ID:             keyID,
		OrganizationID: orgID,
		Name:           "Expired Key",
		IsActive:       true,
		ExpiresAt:      &expiredTime,
	}

	mockAPIKeyRepo.On("GetByHash", mock.AnythingOfType("string")).Return(apiKey, nil)

	// Act
	ctx := context.Background()
	result, err := service.ValidateAPIKey(ctx, "aim_live_expiredkey")

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "expired")

	mockAPIKeyRepo.AssertExpectations(t)
}
