package application

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTagRepository for tag service tests
type MockTagRepoForTags struct {
	mock.Mock
}

func (m *MockTagRepoForTags) Create(ctx context.Context, tag *domain.Tag) error {
	args := m.Called(ctx, tag)
	return args.Error(0)
}

func (m *MockTagRepoForTags) GetByID(ctx context.Context, id uuid.UUID) (*domain.Tag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Tag), args.Error(1)
}

func (m *MockTagRepoForTags) List(ctx context.Context, orgID uuid.UUID, category *domain.TagCategory) ([]*domain.Tag, error) {
	args := m.Called(ctx, orgID, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func (m *MockTagRepoForTags) Update(ctx context.Context, tag *domain.Tag) error {
	args := m.Called(ctx, tag)
	return args.Error(0)
}

func (m *MockTagRepoForTags) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTagRepoForTags) AddTagsToAgent(ctx context.Context, agentID uuid.UUID, tagIDs []uuid.UUID) error {
	args := m.Called(ctx, agentID, tagIDs)
	return args.Error(0)
}

func (m *MockTagRepoForTags) RemoveTagFromAgent(ctx context.Context, agentID, tagID uuid.UUID) error {
	args := m.Called(ctx, agentID, tagID)
	return args.Error(0)
}

func (m *MockTagRepoForTags) GetAgentTags(ctx context.Context, agentID uuid.UUID) ([]*domain.Tag, error) {
	args := m.Called(ctx, agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func (m *MockTagRepoForTags) AddTagsToMCPServer(ctx context.Context, mcpServerID uuid.UUID, tagIDs []uuid.UUID) error {
	args := m.Called(ctx, mcpServerID, tagIDs)
	return args.Error(0)
}

func (m *MockTagRepoForTags) RemoveTagFromMCPServer(ctx context.Context, mcpServerID, tagID uuid.UUID) error {
	args := m.Called(ctx, mcpServerID, tagID)
	return args.Error(0)
}

func (m *MockTagRepoForTags) GetMCPServerTags(ctx context.Context, mcpServerID uuid.UUID) ([]*domain.Tag, error) {
	args := m.Called(ctx, mcpServerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func (m *MockTagRepoForTags) GetPopularTags(ctx context.Context, orgID uuid.UUID, limit int) ([]*domain.Tag, error) {
	args := m.Called(ctx, orgID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

func (m *MockTagRepoForTags) SearchTags(ctx context.Context, orgID uuid.UUID, query string, category *domain.TagCategory) ([]*domain.Tag, error) {
	args := m.Called(ctx, orgID, query, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Tag), args.Error(1)
}

// MockAgentRepoForTags for tag service tests
type MockAgentRepoForTags struct {
	mock.Mock
}

func (m *MockAgentRepoForTags) Create(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepoForTags) GetByID(id uuid.UUID) (*domain.Agent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForTags) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForTags) GetByPublicKeyHash(hash string) (*domain.Agent, error) {
	args := m.Called(hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForTags) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForTags) Update(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepoForTags) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForTags) Count(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockAgentRepoForTags) Search(orgID uuid.UUID, query string, status *domain.AgentStatus, limit, offset int) ([]*domain.Agent, int, error) {
	args := m.Called(orgID, query, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Agent), args.Int(1), args.Error(2)
}

func (m *MockAgentRepoForTags) CountByStatus(orgID uuid.UUID) (map[domain.AgentStatus]int, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.AgentStatus]int), args.Error(1)
}

func (m *MockAgentRepoForTags) GetRecentlyActive(orgID uuid.UUID, hours int, limit int) ([]*domain.Agent, error) {
	args := m.Called(orgID, hours, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForTags) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockAgentRepoForTags) List(limit, offset int) ([]*domain.Agent, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForTags) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	args := m.Called(id, newScore)
	return args.Error(0)
}

func (m *MockAgentRepoForTags) IncrementViolationCount(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForTags) MarkAsCompromised(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// MockMCPServerRepoForTags for tag service tests
type MockMCPServerRepoForTags struct {
	mock.Mock
}

func (m *MockMCPServerRepoForTags) Create(server *domain.MCPServer) error {
	args := m.Called(server)
	return args.Error(0)
}

func (m *MockMCPServerRepoForTags) GetByID(id uuid.UUID) (*domain.MCPServer, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

func (m *MockMCPServerRepoForTags) GetByOrganization(orgID uuid.UUID) ([]*domain.MCPServer, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MCPServer), args.Error(1)
}

func (m *MockMCPServerRepoForTags) GetByPublicKeyHash(hash string) (*domain.MCPServer, error) {
	args := m.Called(hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

func (m *MockMCPServerRepoForTags) Update(server *domain.MCPServer) error {
	args := m.Called(server)
	return args.Error(0)
}

func (m *MockMCPServerRepoForTags) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockMCPServerRepoForTags) Count(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockMCPServerRepoForTags) Search(orgID uuid.UUID, query string, limit, offset int) ([]*domain.MCPServer, int, error) {
	args := m.Called(orgID, query, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.MCPServer), args.Int(1), args.Error(2)
}

func (m *MockMCPServerRepoForTags) GetByName(orgID uuid.UUID, name string) (*domain.MCPServer, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

func (m *MockMCPServerRepoForTags) GetByURL(url string) (*domain.MCPServer, error) {
	args := m.Called(url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServer), args.Error(1)
}

func (m *MockMCPServerRepoForTags) List(limit, offset int) ([]*domain.MCPServer, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.MCPServer), args.Error(1)
}

func (m *MockMCPServerRepoForTags) GetVerificationStatus(id uuid.UUID) (*domain.MCPServerVerificationStatus, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.MCPServerVerificationStatus), args.Error(1)
}

// ====================
// CreateTag Tests
// ====================

func TestTagService_CreateTag_Success(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	userID := uuid.New()
	input := CreateTagInput{
		OrganizationID: orgID,
		Key:            "environment",
		Value:          "production",
		Category:       domain.TagCategoryEnvironment,
		Description:    "Production environment tag",
		Color:          "#00FF00",
		CreatedBy:      userID,
	}

	mockTagRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Tag")).Return(nil)

	// Act
	ctx := context.Background()
	result, err := service.CreateTag(ctx, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "environment", result.Key)
	assert.Equal(t, "production", result.Value)
	assert.Equal(t, domain.TagCategoryEnvironment, result.Category)

	mockTagRepo.AssertExpectations(t)
}

func TestTagService_CreateTag_EmptyKey(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	input := CreateTagInput{
		OrganizationID: uuid.New(),
		Key:            "",
		Value:          "production",
		Category:       domain.TagCategoryEnvironment,
	}

	// Act
	ctx := context.Background()
	result, err := service.CreateTag(ctx, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tag key is required")
}

func TestTagService_CreateTag_EmptyValue(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	input := CreateTagInput{
		OrganizationID: uuid.New(),
		Key:            "environment",
		Value:          "",
		Category:       domain.TagCategoryEnvironment,
	}

	// Act
	ctx := context.Background()
	result, err := service.CreateTag(ctx, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tag value is required")
}

func TestTagService_CreateTag_InvalidCategory(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	input := CreateTagInput{
		OrganizationID: uuid.New(),
		Key:            "test",
		Value:          "value",
		Category:       domain.TagCategory("invalid"),
	}

	// Act
	ctx := context.Background()
	result, err := service.CreateTag(ctx, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid tag category")
}

func TestTagService_CreateTag_InvalidColor(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	input := CreateTagInput{
		OrganizationID: uuid.New(),
		Key:            "test",
		Value:          "value",
		Category:       domain.TagCategoryCustom,
		Color:          "not-a-color",
	}

	// Act
	ctx := context.Background()
	result, err := service.CreateTag(ctx, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "color must be a valid hex color")
}

func TestTagService_CreateTag_KeyTooLong(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	// Key longer than 100 characters
	longKey := ""
	for i := 0; i < 101; i++ {
		longKey += "a"
	}

	input := CreateTagInput{
		OrganizationID: uuid.New(),
		Key:            longKey,
		Value:          "value",
		Category:       domain.TagCategoryCustom,
	}

	// Act
	ctx := context.Background()
	result, err := service.CreateTag(ctx, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tag key must be 100 characters or less")
}

func TestTagService_CreateTag_ValueTooLong(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	// Value longer than 255 characters
	longValue := ""
	for i := 0; i < 256; i++ {
		longValue += "a"
	}

	input := CreateTagInput{
		OrganizationID: uuid.New(),
		Key:            "key",
		Value:          longValue,
		Category:       domain.TagCategoryCustom,
	}

	// Act
	ctx := context.Background()
	result, err := service.CreateTag(ctx, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tag value must be 255 characters or less")
}

// ====================
// GetTagsByOrganization Tests
// ====================

func TestTagService_GetTagsByOrganization_Success(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	tags := []*domain.Tag{
		{ID: uuid.New(), Key: "env", Value: "prod", OrganizationID: orgID},
		{ID: uuid.New(), Key: "team", Value: "backend", OrganizationID: orgID},
	}

	mockTagRepo.On("List", mock.Anything, orgID, (*domain.TagCategory)(nil)).Return(tags, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetTagsByOrganization(ctx, orgID, nil)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 2)

	mockTagRepo.AssertExpectations(t)
}

func TestTagService_GetTagsByOrganization_WithCategory(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	category := domain.TagCategoryEnvironment
	tags := []*domain.Tag{
		{ID: uuid.New(), Key: "env", Value: "prod", Category: category, OrganizationID: orgID},
	}

	mockTagRepo.On("List", mock.Anything, orgID, &category).Return(tags, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetTagsByOrganization(ctx, orgID, &category)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, domain.TagCategoryEnvironment, result[0].Category)

	mockTagRepo.AssertExpectations(t)
}

func TestTagService_GetTagsByOrganization_Error(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()

	mockTagRepo.On("List", mock.Anything, orgID, (*domain.TagCategory)(nil)).Return(nil, errors.New("database error"))

	// Act
	ctx := context.Background()
	result, err := service.GetTagsByOrganization(ctx, orgID, nil)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	mockTagRepo.AssertExpectations(t)
}

// ====================
// UpdateTag Tests
// ====================

func TestTagService_UpdateTag_Success(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	tagID := uuid.New()
	existingTag := &domain.Tag{
		ID:             tagID,
		OrganizationID: orgID,
		Key:            "old-key",
		Value:          "old-value",
		Category:       domain.TagCategoryCustom,
	}

	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(existingTag, nil)
	mockTagRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Tag")).Return(nil)

	input := UpdateTagInput{
		Key:   "new-key",
		Value: "new-value",
	}

	// Act
	ctx := context.Background()
	result, err := service.UpdateTag(ctx, tagID, orgID, input)

	// Assert
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "new-key", result.Key)
	assert.Equal(t, "new-value", result.Value)

	mockTagRepo.AssertExpectations(t)
}

func TestTagService_UpdateTag_NotFound(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	tagID := uuid.New()
	orgID := uuid.New()

	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(nil, errors.New("not found"))

	input := UpdateTagInput{Key: "key"}

	// Act
	ctx := context.Background()
	result, err := service.UpdateTag(ctx, tagID, orgID, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)

	mockTagRepo.AssertExpectations(t)
}

func TestTagService_UpdateTag_WrongOrganization(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	differentOrgID := uuid.New()
	tagID := uuid.New()
	existingTag := &domain.Tag{
		ID:             tagID,
		OrganizationID: orgID,
		Key:            "key",
	}

	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(existingTag, nil)

	input := UpdateTagInput{Key: "new-key"}

	// Act
	ctx := context.Background()
	result, err := service.UpdateTag(ctx, tagID, differentOrgID, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "does not belong to organization")

	mockTagRepo.AssertExpectations(t)
}

func TestTagService_UpdateTag_InvalidCategory(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	tagID := uuid.New()
	existingTag := &domain.Tag{
		ID:             tagID,
		OrganizationID: orgID,
		Key:            "key",
		Category:       domain.TagCategoryCustom,
	}

	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(existingTag, nil)

	input := UpdateTagInput{Category: "invalid-category"}

	// Act
	ctx := context.Background()
	result, err := service.UpdateTag(ctx, tagID, orgID, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid tag category")

	mockTagRepo.AssertExpectations(t)
}

func TestTagService_UpdateTag_InvalidColor(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	tagID := uuid.New()
	existingTag := &domain.Tag{
		ID:             tagID,
		OrganizationID: orgID,
		Key:            "key",
		Category:       domain.TagCategoryCustom,
	}

	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(existingTag, nil)

	input := UpdateTagInput{Color: "invalid"}

	// Act
	ctx := context.Background()
	result, err := service.UpdateTag(ctx, tagID, orgID, input)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "color must be a valid hex color")

	mockTagRepo.AssertExpectations(t)
}

// ====================
// DeleteTag Tests
// ====================

func TestTagService_DeleteTag_Success(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	tagID := uuid.New()

	mockTagRepo.On("Delete", mock.Anything, tagID).Return(nil)

	// Act
	ctx := context.Background()
	err := service.DeleteTag(ctx, tagID)

	// Assert
	assert.NoError(t, err)

	mockTagRepo.AssertExpectations(t)
}

func TestTagService_DeleteTag_Error(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	tagID := uuid.New()

	mockTagRepo.On("Delete", mock.Anything, tagID).Return(errors.New("database error"))

	// Act
	ctx := context.Background()
	err := service.DeleteTag(ctx, tagID)

	// Assert
	assert.Error(t, err)

	mockTagRepo.AssertExpectations(t)
}

// ====================
// AddTagsToAgent Tests
// ====================

func TestTagService_AddTagsToAgent_Success(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	agentID := uuid.New()
	tagID1 := uuid.New()
	tagID2 := uuid.New()
	userID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
	}
	tag1 := &domain.Tag{ID: tagID1, OrganizationID: orgID, Key: "env", Value: "prod"}
	tag2 := &domain.Tag{ID: tagID2, OrganizationID: orgID, Key: "team", Value: "backend"}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockTagRepo.On("GetByID", mock.Anything, tagID1).Return(tag1, nil)
	mockTagRepo.On("GetByID", mock.Anything, tagID2).Return(tag2, nil)
	mockTagRepo.On("AddTagsToAgent", mock.Anything, agentID, []uuid.UUID{tagID1, tagID2}).Return(nil)

	// Act
	ctx := context.Background()
	err := service.AddTagsToAgent(ctx, agentID, []uuid.UUID{tagID1, tagID2}, userID)

	// Assert
	assert.NoError(t, err)

	mockAgentRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_AddTagsToAgent_AgentNotFound(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	agentID := uuid.New()

	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("not found"))

	// Act
	ctx := context.Background()
	err := service.AddTagsToAgent(ctx, agentID, []uuid.UUID{uuid.New()}, uuid.New())

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "agent not found")

	mockAgentRepo.AssertExpectations(t)
}

func TestTagService_AddTagsToAgent_TagWrongOrganization(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	differentOrgID := uuid.New()
	agentID := uuid.New()
	tagID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
	}
	tag := &domain.Tag{ID: tagID, OrganizationID: differentOrgID}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(tag, nil)

	// Act
	ctx := context.Background()
	err := service.AddTagsToAgent(ctx, agentID, []uuid.UUID{tagID}, uuid.New())

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to agent's organization")

	mockAgentRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

// ====================
// GetPopularTags Tests
// ====================

func TestTagService_GetPopularTags_Success(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	tags := []*domain.Tag{
		{ID: uuid.New(), Key: "env", Value: "prod"},
		{ID: uuid.New(), Key: "team", Value: "backend"},
	}

	mockTagRepo.On("GetPopularTags", mock.Anything, orgID, 10).Return(tags, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetPopularTags(ctx, orgID, 10)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 2)

	mockTagRepo.AssertExpectations(t)
}

// ====================
// SearchTags Tests
// ====================

func TestTagService_SearchTags_Success(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	tags := []*domain.Tag{
		{ID: uuid.New(), Key: "environment", Value: "production"},
	}

	mockTagRepo.On("SearchTags", mock.Anything, orgID, "prod", (*domain.TagCategory)(nil)).Return(tags, nil)

	// Act
	ctx := context.Background()
	result, err := service.SearchTags(ctx, orgID, "prod", "")

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "environment", result[0].Key)

	mockTagRepo.AssertExpectations(t)
}

func TestTagService_SearchTags_WithCategory(t *testing.T) {
	// Arrange
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	category := domain.TagCategoryEnvironment
	tags := []*domain.Tag{
		{ID: uuid.New(), Key: "env", Value: "staging", Category: category},
	}

	mockTagRepo.On("SearchTags", mock.Anything, orgID, "stag", &category).Return(tags, nil)

	// Act
	ctx := context.Background()
	result, err := service.SearchTags(ctx, orgID, "stag", string(domain.TagCategoryEnvironment))

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 1)

	mockTagRepo.AssertExpectations(t)
}

// ====================
// Tag Suggestion Tests
// ====================

func TestTagService_suggestTagsForAgentType(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	tests := []struct {
		agentType      domain.AgentType
		expectedValues []string
	}{
		{domain.AgentTypeClaude, []string{"llm-provider", "claude"}},
		{domain.AgentTypeGPT, []string{"llm-provider", "gpt"}},
		{domain.AgentTypeLangChain, []string{"framework", "langchain"}},
		{domain.AgentTypeCopilot, []string{"assistant", "copilot"}},
		{domain.AgentTypeAutoGPT, []string{"autonomous", "autogpt"}},
		{domain.AgentTypeCustom, []string{"ai-agent"}},
	}

	for _, tt := range tests {
		t.Run(string(tt.agentType), func(t *testing.T) {
			suggestions := service.suggestTagsForAgentType(tt.agentType)
			assert.NotNil(t, suggestions)

			// Check that expected values are in suggestions
			for _, expected := range tt.expectedValues {
				found := false
				for _, s := range suggestions {
					if s.Value == expected {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected value %s not found in suggestions", expected)
			}
		})
	}
}

func TestTagService_suggestTagsForTrustScore(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	tests := []struct {
		score            float64
		expectedTrust    string
		expectedEnv      string
	}{
		{9.0, "high", "production"},
		{8.0, "high", "production"},
		{6.0, "medium", "staging"},
		{5.0, "medium", "staging"},
		{3.0, "low", "development"},
		{0.0, "low", "development"},
	}

	for _, tt := range tests {
		suggestions := service.suggestTagsForTrustScore(tt.score)
		assert.NotNil(t, suggestions)

		// Should have trust-level and environment suggestions
		hasTrustLevel := false
		hasEnvironment := false
		for _, s := range suggestions {
			if s.Key == "trust-level" && s.Value == tt.expectedTrust {
				hasTrustLevel = true
			}
			if s.Key == "environment" && s.Value == tt.expectedEnv {
				hasEnvironment = true
			}
		}
		assert.True(t, hasTrustLevel, "Expected trust-level=%s for score %f", tt.expectedTrust, tt.score)
		assert.True(t, hasEnvironment, "Expected environment=%s for score %f", tt.expectedEnv, tt.score)
	}
}

func TestTagService_suggestTagsForCapabilities(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	capabilities := []string{"file:read", "database:query", "network:connect", "code:execute"}

	suggestions := service.suggestTagsForCapabilities(capabilities)
	assert.NotEmpty(t, suggestions)

	// Check for expected resource types
	hasFilesystem := false
	hasDatabase := false
	hasNetwork := false
	hasCodeExecution := false

	for _, s := range suggestions {
		if s.Key == "resource" && s.Value == "filesystem" {
			hasFilesystem = true
		}
		if s.Key == "resource" && s.Value == "database" {
			hasDatabase = true
		}
		if s.Key == "resource" && s.Value == "network" {
			hasNetwork = true
		}
		if s.Key == "resource" && s.Value == "code-execution" {
			hasCodeExecution = true
		}
	}

	assert.True(t, hasFilesystem, "Should suggest filesystem tag for file capability")
	assert.True(t, hasDatabase, "Should suggest database tag for database capability")
	assert.True(t, hasNetwork, "Should suggest network tag for network capability")
	assert.True(t, hasCodeExecution, "Should suggest code-execution tag for code capability")
}

// ====================
// RemoveTagFromAgent Tests
// ====================

func TestTagService_RemoveTagFromAgent_Success(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	agentID := uuid.New()
	tagID := uuid.New()

	mockTagRepo.On("RemoveTagFromAgent", mock.Anything, agentID, tagID).Return(nil)

	ctx := context.Background()
	err := service.RemoveTagFromAgent(ctx, agentID, tagID)

	assert.NoError(t, err)
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_RemoveTagFromAgent_Error(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	agentID := uuid.New()
	tagID := uuid.New()

	mockTagRepo.On("RemoveTagFromAgent", mock.Anything, agentID, tagID).Return(errors.New("database error"))

	ctx := context.Background()
	err := service.RemoveTagFromAgent(ctx, agentID, tagID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove tag from agent")
	mockTagRepo.AssertExpectations(t)
}

// ====================
// GetAgentTags Tests
// ====================

func TestTagService_GetAgentTags_Success(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	agentID := uuid.New()
	orgID := uuid.New()
	tags := []*domain.Tag{
		{ID: uuid.New(), Key: "env", Value: "prod", OrganizationID: orgID},
		{ID: uuid.New(), Key: "team", Value: "backend", OrganizationID: orgID},
	}

	mockTagRepo.On("GetAgentTags", mock.Anything, agentID).Return(tags, nil)

	ctx := context.Background()
	result, err := service.GetAgentTags(ctx, agentID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_GetAgentTags_Error(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	agentID := uuid.New()

	mockTagRepo.On("GetAgentTags", mock.Anything, agentID).Return(nil, errors.New("database error"))

	ctx := context.Background()
	result, err := service.GetAgentTags(ctx, agentID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get agent tags")
	mockTagRepo.AssertExpectations(t)
}

// ====================
// AddTagsToMCPServer Tests
// ====================

func TestTagService_AddTagsToMCPServer_Success(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	mcpServerID := uuid.New()
	tagID1 := uuid.New()
	tagID2 := uuid.New()
	userID := uuid.New()

	mcpServer := &domain.MCPServer{
		ID:             mcpServerID,
		OrganizationID: orgID,
		Name:           "test-mcp-server",
	}
	tag1 := &domain.Tag{ID: tagID1, OrganizationID: orgID, Key: "env", Value: "prod"}
	tag2 := &domain.Tag{ID: tagID2, OrganizationID: orgID, Key: "type", Value: "database"}

	mockMCPRepo.On("GetByID", mcpServerID).Return(mcpServer, nil)
	mockTagRepo.On("GetByID", mock.Anything, tagID1).Return(tag1, nil)
	mockTagRepo.On("GetByID", mock.Anything, tagID2).Return(tag2, nil)
	mockTagRepo.On("AddTagsToMCPServer", mock.Anything, mcpServerID, []uuid.UUID{tagID1, tagID2}).Return(nil)

	ctx := context.Background()
	err := service.AddTagsToMCPServer(ctx, mcpServerID, []uuid.UUID{tagID1, tagID2}, userID)

	assert.NoError(t, err)
	mockMCPRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_AddTagsToMCPServer_MCPServerNotFound(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	mcpServerID := uuid.New()

	mockMCPRepo.On("GetByID", mcpServerID).Return(nil, errors.New("not found"))

	ctx := context.Background()
	err := service.AddTagsToMCPServer(ctx, mcpServerID, []uuid.UUID{uuid.New()}, uuid.New())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mcp server not found")
	mockMCPRepo.AssertExpectations(t)
}

func TestTagService_AddTagsToMCPServer_TagWrongOrganization(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	differentOrgID := uuid.New()
	mcpServerID := uuid.New()
	tagID := uuid.New()

	mcpServer := &domain.MCPServer{
		ID:             mcpServerID,
		OrganizationID: orgID,
	}
	tag := &domain.Tag{ID: tagID, OrganizationID: differentOrgID}

	mockMCPRepo.On("GetByID", mcpServerID).Return(mcpServer, nil)
	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(tag, nil)

	ctx := context.Background()
	err := service.AddTagsToMCPServer(ctx, mcpServerID, []uuid.UUID{tagID}, uuid.New())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "does not belong to mcp server's organization")
	mockMCPRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_AddTagsToMCPServer_TagNotFound(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	mcpServerID := uuid.New()
	tagID := uuid.New()

	mcpServer := &domain.MCPServer{
		ID:             mcpServerID,
		OrganizationID: orgID,
	}

	mockMCPRepo.On("GetByID", mcpServerID).Return(mcpServer, nil)
	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(nil, errors.New("tag not found"))

	ctx := context.Background()
	err := service.AddTagsToMCPServer(ctx, mcpServerID, []uuid.UUID{tagID}, uuid.New())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	mockMCPRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

// ====================
// RemoveTagFromMCPServer Tests
// ====================

func TestTagService_RemoveTagFromMCPServer_Success(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	mcpServerID := uuid.New()
	tagID := uuid.New()

	mockTagRepo.On("RemoveTagFromMCPServer", mock.Anything, mcpServerID, tagID).Return(nil)

	ctx := context.Background()
	err := service.RemoveTagFromMCPServer(ctx, mcpServerID, tagID)

	assert.NoError(t, err)
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_RemoveTagFromMCPServer_Error(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	mcpServerID := uuid.New()
	tagID := uuid.New()

	mockTagRepo.On("RemoveTagFromMCPServer", mock.Anything, mcpServerID, tagID).Return(errors.New("database error"))

	ctx := context.Background()
	err := service.RemoveTagFromMCPServer(ctx, mcpServerID, tagID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to remove tag from mcp server")
	mockTagRepo.AssertExpectations(t)
}

// ====================
// GetMCPServerTags Tests
// ====================

func TestTagService_GetMCPServerTags_Success(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	mcpServerID := uuid.New()
	orgID := uuid.New()
	tags := []*domain.Tag{
		{ID: uuid.New(), Key: "type", Value: "database", OrganizationID: orgID},
		{ID: uuid.New(), Key: "env", Value: "production", OrganizationID: orgID},
	}

	mockTagRepo.On("GetMCPServerTags", mock.Anything, mcpServerID).Return(tags, nil)

	ctx := context.Background()
	result, err := service.GetMCPServerTags(ctx, mcpServerID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_GetMCPServerTags_Error(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	mcpServerID := uuid.New()

	mockTagRepo.On("GetMCPServerTags", mock.Anything, mcpServerID).Return(nil, errors.New("database error"))

	ctx := context.Background()
	result, err := service.GetMCPServerTags(ctx, mcpServerID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get mcp server tags")
	mockTagRepo.AssertExpectations(t)
}

// ====================
// SuggestTagsForAgent Tests
// ====================

func TestTagService_SuggestTagsForAgent_Success(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	agentID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
		AgentType:      domain.AgentTypeClaude,
		TrustScore:     9.0,
		Status:         domain.AgentStatusVerified,
		Capabilities:   []string{"file:read"},
		TalksTo:        []string{"github-mcp"},
	}

	// Organization tags that match suggestions
	orgTags := []*domain.Tag{
		{ID: uuid.New(), Key: "type", Value: "llm-provider", OrganizationID: orgID, Category: domain.TagCategoryAgentType},
		{ID: uuid.New(), Key: "trust-level", Value: "high", OrganizationID: orgID, Category: domain.TagCategoryDataClassification},
		{ID: uuid.New(), Key: "environment", Value: "production", OrganizationID: orgID, Category: domain.TagCategoryEnvironment},
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockTagRepo.On("GetAgentTags", mock.Anything, agentID).Return([]*domain.Tag{}, nil)
	mockTagRepo.On("List", mock.Anything, orgID, (*domain.TagCategory)(nil)).Return(orgTags, nil)

	ctx := context.Background()
	result, err := service.SuggestTagsForAgent(ctx, agentID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	// Should suggest tags that exist in org and match agent metadata
	mockAgentRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_SuggestTagsForAgent_AgentNotFound(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	agentID := uuid.New()

	mockAgentRepo.On("GetByID", agentID).Return(nil, errors.New("not found"))

	ctx := context.Background()
	result, err := service.SuggestTagsForAgent(ctx, agentID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "agent not found")
	mockAgentRepo.AssertExpectations(t)
}

func TestTagService_SuggestTagsForAgent_ExistingTagsFilteredOut(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	agentID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		Name:           "test-agent",
		AgentType:      domain.AgentTypeClaude,
		TrustScore:     9.0,
		Status:         domain.AgentStatusVerified,
	}

	// Already applied tag
	existingTags := []*domain.Tag{
		{ID: uuid.New(), Key: "type", Value: "llm-provider", OrganizationID: orgID},
	}

	orgTags := []*domain.Tag{
		{ID: uuid.New(), Key: "type", Value: "llm-provider", OrganizationID: orgID},
		{ID: uuid.New(), Key: "trust-level", Value: "high", OrganizationID: orgID},
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockTagRepo.On("GetAgentTags", mock.Anything, agentID).Return(existingTags, nil)
	mockTagRepo.On("List", mock.Anything, orgID, (*domain.TagCategory)(nil)).Return(orgTags, nil)

	ctx := context.Background()
	result, err := service.SuggestTagsForAgent(ctx, agentID)

	assert.NoError(t, err)
	// "type:llm-provider" should be filtered out because it's already applied
	for _, tag := range result {
		if tag.Key == "type" && tag.Value == "llm-provider" {
			t.Error("Already applied tag should be filtered out")
		}
	}
	mockAgentRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

// ====================
// SuggestTagsForMCPServer Tests
// ====================

func TestTagService_SuggestTagsForMCPServer_Success(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	mcpServerID := uuid.New()

	mcpServer := &domain.MCPServer{
		ID:             mcpServerID,
		OrganizationID: orgID,
		Name:           "postgres-mcp",
		TrustScore:     8.5,
		Status:         domain.MCPServerStatusVerified,
		IsVerified:     true,
		Capabilities:   []string{"tools", "resources"},
	}

	orgTags := []*domain.Tag{
		{ID: uuid.New(), Key: "type", Value: "database", OrganizationID: orgID},
		{ID: uuid.New(), Key: "status", Value: "verified", OrganizationID: orgID},
		{ID: uuid.New(), Key: "capability", Value: "tools", OrganizationID: orgID},
	}

	mockMCPRepo.On("GetByID", mcpServerID).Return(mcpServer, nil)
	mockTagRepo.On("GetMCPServerTags", mock.Anything, mcpServerID).Return([]*domain.Tag{}, nil)
	mockTagRepo.On("List", mock.Anything, orgID, (*domain.TagCategory)(nil)).Return(orgTags, nil)

	ctx := context.Background()
	result, err := service.SuggestTagsForMCPServer(ctx, mcpServerID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	mockMCPRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_SuggestTagsForMCPServer_MCPServerNotFound(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	mcpServerID := uuid.New()

	mockMCPRepo.On("GetByID", mcpServerID).Return(nil, errors.New("not found"))

	ctx := context.Background()
	result, err := service.SuggestTagsForMCPServer(ctx, mcpServerID)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "mcp server not found")
	mockMCPRepo.AssertExpectations(t)
}

// ====================
// Helper Method Tests - MCP Connections
// ====================

func TestTagService_suggestTagsForMCPConnections(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	tests := []struct {
		name          string
		talksTo       []string
		expectedKeys  []string
		expectedVals  []string
	}{
		{
			name:         "github connection",
			talksTo:      []string{"github-mcp"},
			expectedKeys: []string{"mcp", "integration"},
			expectedVals: []string{"github", "source-control"},
		},
		{
			name:         "slack connection",
			talksTo:      []string{"slack-mcp"},
			expectedKeys: []string{"mcp", "integration"},
			expectedVals: []string{"slack", "messaging"},
		},
		{
			name:         "aws connection",
			talksTo:      []string{"aws-bedrock"},
			expectedKeys: []string{"mcp", "cloud"},
			expectedVals: []string{"aws", "aws"},
		},
		{
			name:         "postgres connection",
			talksTo:      []string{"postgres-db"},
			expectedKeys: []string{"mcp"},
			expectedVals: []string{"database"},
		},
		{
			name:         "filesystem connection",
			talksTo:      []string{"filesystem-mcp"},
			expectedKeys: []string{"mcp"},
			expectedVals: []string{"filesystem"},
		},
		{
			name:         "browser connection",
			talksTo:      []string{"puppeteer-mcp"},
			expectedKeys: []string{"mcp", "automation"},
			expectedVals: []string{"browser", "web"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := service.suggestTagsForMCPConnections(tt.talksTo)
			assert.NotEmpty(t, suggestions)

			for i, expectedKey := range tt.expectedKeys {
				found := false
				for _, s := range suggestions {
					if s.Key == expectedKey && s.Value == tt.expectedVals[i] {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected key=%s value=%s not found", expectedKey, tt.expectedVals[i])
			}
		})
	}
}

// ====================
// Helper Method Tests - Agent Status
// ====================

func TestTagService_suggestTagsForAgentStatus(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	tests := []struct {
		status        domain.AgentStatus
		expectedValue string
		hasAlert      bool
	}{
		{domain.AgentStatusVerified, "verified", false},
		{domain.AgentStatusPending, "pending-review", false},
		{domain.AgentStatusSuspended, "suspended", true},
		{domain.AgentStatusRevoked, "revoked", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			suggestions := service.suggestTagsForAgentStatus(tt.status)
			assert.NotEmpty(t, suggestions)

			hasStatus := false
			hasAlertTag := false
			for _, s := range suggestions {
				if s.Key == "status" && s.Value == tt.expectedValue {
					hasStatus = true
				}
				if s.Key == "alert" && s.Value == "requires-attention" {
					hasAlertTag = true
				}
			}
			assert.True(t, hasStatus, "Expected status=%s for agent status %s", tt.expectedValue, tt.status)
			if tt.hasAlert {
				assert.True(t, hasAlertTag, "Expected alert tag for suspended status")
			}
		})
	}
}

// ====================
// Helper Method Tests - MCP Name
// ====================

func TestTagService_suggestTagsForMCPName(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	tests := []struct {
		name          string
		mcpName       string
		expectedKey   string
		expectedValue string
	}{
		{"filesystem", "filesystem-server", "type", "filesystem"},
		{"postgres", "postgres-mcp", "type", "database"},
		{"mysql", "mysql-connector", "type", "database"},
		{"github", "github-mcp", "type", "source-control"},
		{"slack", "slack-integration", "type", "messaging"},
		{"aws", "aws-bedrock-mcp", "cloud", "aws"},
		{"gcp", "gcp-storage", "cloud", "gcp"},
		{"azure", "azure-services", "cloud", "azure"},
		{"memory", "memory-store", "type", "memory"},
		{"browser", "browser-automation", "type", "browser-automation"},
		{"search", "brave-search", "type", "search"},
		{"email", "gmail-mcp", "type", "email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := service.suggestTagsForMCPName(tt.mcpName)
			assert.NotEmpty(t, suggestions)

			found := false
			for _, s := range suggestions {
				if s.Key == tt.expectedKey && s.Value == tt.expectedValue {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected key=%s value=%s for MCP name %s", tt.expectedKey, tt.expectedValue, tt.mcpName)
		})
	}
}

// ====================
// Helper Method Tests - MCP Capabilities
// ====================

func TestTagService_suggestTagsForMCPCapabilities(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	tests := []struct {
		name         string
		capabilities []string
		expected     map[string]string
	}{
		{
			name:         "tools capability",
			capabilities: []string{"tools"},
			expected:     map[string]string{"capability": "tools"},
		},
		{
			name:         "prompts capability",
			capabilities: []string{"prompts"},
			expected:     map[string]string{"capability": "prompts"},
		},
		{
			name:         "resources capability",
			capabilities: []string{"resources"},
			expected:     map[string]string{"capability": "resources"},
		},
		{
			name:         "sampling capability",
			capabilities: []string{"sampling"},
			expected:     map[string]string{"capability": "sampling", "classification": "advanced"},
		},
		{
			name:         "multiple capabilities",
			capabilities: []string{"tools", "prompts", "resources"},
			expected:     map[string]string{"capability": "tools"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := service.suggestTagsForMCPCapabilities(tt.capabilities)
			assert.NotEmpty(t, suggestions)

			for key, value := range tt.expected {
				found := false
				for _, s := range suggestions {
					if s.Key == key && s.Value == value {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected key=%s value=%s for capabilities %v", key, value, tt.capabilities)
			}
		})
	}
}

// ====================
// Helper Method Tests - MCP Status
// ====================

func TestTagService_suggestTagsForMCPStatus(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	tests := []struct {
		name            string
		status          domain.MCPServerStatus
		isVerified      bool
		expectedStatus  string
		expectedAvail   string
		hasAlert        bool
	}{
		{
			name:           "verified and active",
			status:         domain.MCPServerStatusVerified,
			isVerified:     true,
			expectedStatus: "verified",
			expectedAvail:  "active",
			hasAlert:       false,
		},
		{
			name:           "suspended",
			status:         domain.MCPServerStatusSuspended,
			isVerified:     false,
			expectedStatus: "unverified",
			expectedAvail:  "suspended",
			hasAlert:       true,
		},
		{
			name:           "revoked",
			status:         domain.MCPServerStatusRevoked,
			isVerified:     false,
			expectedStatus: "unverified",
			expectedAvail:  "revoked",
			hasAlert:       false,
		},
		{
			name:           "pending",
			status:         domain.MCPServerStatusPending,
			isVerified:     false,
			expectedStatus: "unverified",
			expectedAvail:  "",
			hasAlert:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := service.suggestTagsForMCPStatus(tt.status, tt.isVerified)
			assert.NotEmpty(t, suggestions)

			hasStatusTag := false
			hasAvailTag := false
			hasAlertTag := false

			for _, s := range suggestions {
				if s.Key == "status" && s.Value == tt.expectedStatus {
					hasStatusTag = true
				}
				if s.Key == "availability" && s.Value == tt.expectedAvail {
					hasAvailTag = true
				}
				if s.Key == "alert" && s.Value == "requires-attention" {
					hasAlertTag = true
				}
			}

			assert.True(t, hasStatusTag, "Expected status=%s", tt.expectedStatus)
			if tt.expectedAvail != "" {
				assert.True(t, hasAvailTag, "Expected availability=%s", tt.expectedAvail)
			}
			if tt.hasAlert {
				assert.True(t, hasAlertTag, "Expected alert tag for suspended status")
			}
		})
	}
}

// ====================
// GetPopularTags Error Test
// ====================

func TestTagService_GetPopularTags_Error(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()

	mockTagRepo.On("GetPopularTags", mock.Anything, orgID, 10).Return(nil, errors.New("database error"))

	ctx := context.Background()
	result, err := service.GetPopularTags(ctx, orgID, 10)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get popular tags")
	mockTagRepo.AssertExpectations(t)
}

// ====================
// SearchTags Error Test
// ====================

func TestTagService_SearchTags_Error(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()

	mockTagRepo.On("SearchTags", mock.Anything, orgID, "query", (*domain.TagCategory)(nil)).Return(nil, errors.New("database error"))

	ctx := context.Background()
	result, err := service.SearchTags(ctx, orgID, "query", "")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to search tags")
	mockTagRepo.AssertExpectations(t)
}

// ====================
// AddTagsToAgent Error Tests
// ====================

func TestTagService_AddTagsToAgent_TagNotFound(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	agentID := uuid.New()
	tagID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
	}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(nil, errors.New("tag not found"))

	ctx := context.Background()
	err := service.AddTagsToAgent(ctx, agentID, []uuid.UUID{tagID}, uuid.New())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
	mockAgentRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_AddTagsToAgent_RepoError(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	agentID := uuid.New()
	tagID := uuid.New()

	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
	}
	tag := &domain.Tag{ID: tagID, OrganizationID: orgID}

	mockAgentRepo.On("GetByID", agentID).Return(agent, nil)
	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(tag, nil)
	mockTagRepo.On("AddTagsToAgent", mock.Anything, agentID, []uuid.UUID{tagID}).Return(errors.New("database error"))

	ctx := context.Background()
	err := service.AddTagsToAgent(ctx, agentID, []uuid.UUID{tagID}, uuid.New())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to add tags to agent")
	mockAgentRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

// ====================
// AddTagsToMCPServer Error Test
// ====================

func TestTagService_AddTagsToMCPServer_RepoError(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	mcpServerID := uuid.New()
	tagID := uuid.New()

	mcpServer := &domain.MCPServer{
		ID:             mcpServerID,
		OrganizationID: orgID,
	}
	tag := &domain.Tag{ID: tagID, OrganizationID: orgID}

	mockMCPRepo.On("GetByID", mcpServerID).Return(mcpServer, nil)
	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(tag, nil)
	mockTagRepo.On("AddTagsToMCPServer", mock.Anything, mcpServerID, []uuid.UUID{tagID}).Return(errors.New("database error"))

	ctx := context.Background()
	err := service.AddTagsToMCPServer(ctx, mcpServerID, []uuid.UUID{tagID}, uuid.New())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to add tags to mcp server")
	mockMCPRepo.AssertExpectations(t)
	mockTagRepo.AssertExpectations(t)
}

// ====================
// UpdateTag Error Tests
// ====================

func TestTagService_UpdateTag_KeyTooLong(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	tagID := uuid.New()
	existingTag := &domain.Tag{
		ID:             tagID,
		OrganizationID: orgID,
		Key:            "key",
		Category:       domain.TagCategoryCustom,
	}

	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(existingTag, nil)

	longKey := ""
	for i := 0; i < 101; i++ {
		longKey += "a"
	}

	input := UpdateTagInput{Key: longKey}

	ctx := context.Background()
	result, err := service.UpdateTag(ctx, tagID, orgID, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tag key must be 100 characters or less")
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_UpdateTag_ValueTooLong(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	tagID := uuid.New()
	existingTag := &domain.Tag{
		ID:             tagID,
		OrganizationID: orgID,
		Key:            "key",
		Category:       domain.TagCategoryCustom,
	}

	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(existingTag, nil)

	longValue := ""
	for i := 0; i < 256; i++ {
		longValue += "a"
	}

	input := UpdateTagInput{Value: longValue}

	ctx := context.Background()
	result, err := service.UpdateTag(ctx, tagID, orgID, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "tag value must be 255 characters or less")
	mockTagRepo.AssertExpectations(t)
}

func TestTagService_UpdateTag_RepoError(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	orgID := uuid.New()
	tagID := uuid.New()
	existingTag := &domain.Tag{
		ID:             tagID,
		OrganizationID: orgID,
		Key:            "key",
		Category:       domain.TagCategoryCustom,
	}

	mockTagRepo.On("GetByID", mock.Anything, tagID).Return(existingTag, nil)
	mockTagRepo.On("Update", mock.Anything, mock.AnythingOfType("*domain.Tag")).Return(errors.New("database error"))

	input := UpdateTagInput{Key: "new-key"}

	ctx := context.Background()
	result, err := service.UpdateTag(ctx, tagID, orgID, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to update tag")
	mockTagRepo.AssertExpectations(t)
}

// ====================
// CreateTag Repo Error Test
// ====================

func TestTagService_CreateTag_RepoError(t *testing.T) {
	mockTagRepo := new(MockTagRepoForTags)
	mockAgentRepo := new(MockAgentRepoForTags)
	mockMCPRepo := new(MockMCPServerRepoForTags)

	service := NewTagService(mockTagRepo, mockAgentRepo, mockMCPRepo)

	input := CreateTagInput{
		OrganizationID: uuid.New(),
		Key:            "test",
		Value:          "value",
		Category:       domain.TagCategoryCustom,
		Color:          "#FFFFFF",
		CreatedBy:      uuid.New(),
	}

	mockTagRepo.On("Create", mock.Anything, mock.AnythingOfType("*domain.Tag")).Return(errors.New("database error"))

	ctx := context.Background()
	result, err := service.CreateTag(ctx, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to create tag")
	mockTagRepo.AssertExpectations(t)
}
