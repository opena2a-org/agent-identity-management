package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAlertRepository for alert service tests
type MockAlertRepoForAlerts struct {
	mock.Mock
}

func (m *MockAlertRepoForAlerts) Create(alert *domain.Alert) error {
	args := m.Called(alert)
	return args.Error(0)
}

func (m *MockAlertRepoForAlerts) GetByID(id uuid.UUID) (*domain.Alert, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForAlerts) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForAlerts) GetByOrganizationFiltered(orgID uuid.UUID, status string, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(orgID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForAlerts) GetUnacknowledged(orgID uuid.UUID) ([]*domain.Alert, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForAlerts) Acknowledge(id uuid.UUID, userID uuid.UUID) error {
	args := m.Called(id, userID)
	return args.Error(0)
}

func (m *MockAlertRepoForAlerts) BulkAcknowledge(orgID uuid.UUID, userID uuid.UUID) (int, error) {
	args := m.Called(orgID, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockAlertRepoForAlerts) CountByOrganization(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockAlertRepoForAlerts) CountByOrganizationFiltered(orgID uuid.UUID, status string) (int, error) {
	args := m.Called(orgID, status)
	return args.Int(0), args.Error(1)
}

func (m *MockAlertRepoForAlerts) CountBySeverity(orgID uuid.UUID, status string) (critical, high, warning, info int, err error) {
	args := m.Called(orgID, status)
	return args.Int(0), args.Int(1), args.Int(2), args.Int(3), args.Error(4)
}

func (m *MockAlertRepoForAlerts) GetByResourceID(resourceID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForAlerts) GetUnacknowledgedByResourceID(resourceID uuid.UUID) ([]*domain.Alert, error) {
	args := m.Called(resourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForAlerts) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// MockAgentRepoForAlerts for alert service tests
type MockAgentRepoForAlerts struct {
	mock.Mock
}

func (m *MockAgentRepoForAlerts) Create(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepoForAlerts) GetByID(id uuid.UUID) (*domain.Agent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAlerts) GetByOrganization(orgID uuid.UUID) ([]*domain.Agent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAlerts) GetByPublicKeyHash(hash string) (*domain.Agent, error) {
	args := m.Called(hash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAlerts) GetByName(orgID uuid.UUID, name string) (*domain.Agent, error) {
	args := m.Called(orgID, name)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAlerts) Update(agent *domain.Agent) error {
	args := m.Called(agent)
	return args.Error(0)
}

func (m *MockAgentRepoForAlerts) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForAlerts) Count(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockAgentRepoForAlerts) Search(orgID uuid.UUID, query string, status *domain.AgentStatus, limit, offset int) ([]*domain.Agent, int, error) {
	args := m.Called(orgID, query, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.Agent), args.Int(1), args.Error(2)
}

func (m *MockAgentRepoForAlerts) CountByStatus(orgID uuid.UUID) (map[domain.AgentStatus]int, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domain.AgentStatus]int), args.Error(1)
}

func (m *MockAgentRepoForAlerts) GetRecentlyActive(orgID uuid.UUID, hours int, limit int) ([]*domain.Agent, error) {
	args := m.Called(orgID, hours, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAlerts) UpdateLastActive(ctx context.Context, agentID uuid.UUID) error {
	args := m.Called(ctx, agentID)
	return args.Error(0)
}

func (m *MockAgentRepoForAlerts) List(limit, offset int) ([]*domain.Agent, error) {
	args := m.Called(limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Agent), args.Error(1)
}

func (m *MockAgentRepoForAlerts) UpdateTrustScore(id uuid.UUID, newScore float64) error {
	args := m.Called(id, newScore)
	return args.Error(0)
}

func (m *MockAgentRepoForAlerts) IncrementViolationCount(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockAgentRepoForAlerts) MarkAsCompromised(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// Helper function to create a test alert
func createTestAlertForAlerts(orgID uuid.UUID) *domain.Alert {
	return &domain.Alert{
		ID:             uuid.New(),
		OrganizationID: orgID,
		AlertType:      domain.AlertSecurityBreach,
		Severity:       domain.SeverityCritical,
		Title:          "Test Security Alert",
		Description:    "Test description",
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		IsAcknowledged: false,
		CreatedAt:      time.Now(),
	}
}

// ====================
// CreateAlert Tests
// ====================

func TestAlertService_CreateAlert_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	alert := createTestAlertForAlerts(orgID)

	mockAlertRepo.On("Create", alert).Return(nil)

	// Act
	ctx := context.Background()
	err := service.CreateAlert(ctx, alert)

	// Assert
	assert.NoError(t, err)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_CreateAlert_Error(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	alert := createTestAlertForAlerts(orgID)

	mockAlertRepo.On("Create", alert).Return(errors.New("database error"))

	// Act
	ctx := context.Background()
	err := service.CreateAlert(ctx, alert)

	// Assert
	assert.Error(t, err)

	mockAlertRepo.AssertExpectations(t)
}

// ====================
// GetUnacknowledgedAlerts Tests
// ====================

func TestAlertService_GetUnacknowledgedAlerts_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	alerts := []*domain.Alert{
		createTestAlertForAlerts(orgID),
		createTestAlertForAlerts(orgID),
	}

	mockAlertRepo.On("GetUnacknowledged", orgID).Return(alerts, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetUnacknowledgedAlerts(ctx, orgID)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 2)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_GetUnacknowledgedAlerts_Empty(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	mockAlertRepo.On("GetUnacknowledged", orgID).Return([]*domain.Alert{}, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetUnacknowledgedAlerts(ctx, orgID)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 0)

	mockAlertRepo.AssertExpectations(t)
}

// ====================
// CountUnacknowledged Tests
// ====================

func TestAlertService_CountUnacknowledged_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	unackAlerts := []*domain.Alert{
		createTestAlertForAlerts(orgID),
		createTestAlertForAlerts(orgID),
		createTestAlertForAlerts(orgID),
	}

	mockAlertRepo.On("CountByOrganization", orgID).Return(10, nil)
	mockAlertRepo.On("GetUnacknowledged", orgID).Return(unackAlerts, nil)

	// Act
	ctx := context.Background()
	allCount, ackCount, unackCount, err := service.CountUnacknowledged(ctx, orgID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 10, allCount)
	assert.Equal(t, 7, ackCount)  // 10 - 3
	assert.Equal(t, 3, unackCount)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_CountUnacknowledged_CountError(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	mockAlertRepo.On("CountByOrganization", orgID).Return(0, errors.New("database error"))

	// Act
	ctx := context.Background()
	allCount, ackCount, unackCount, err := service.CountUnacknowledged(ctx, orgID)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, 0, allCount)
	assert.Equal(t, 0, ackCount)
	assert.Equal(t, 0, unackCount)

	mockAlertRepo.AssertExpectations(t)
}

// ====================
// CountBySeverity Tests
// ====================

func TestAlertService_CountBySeverity_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	mockAlertRepo.On("CountBySeverity", orgID, "").Return(2, 5, 10, 3, nil)

	// Act
	ctx := context.Background()
	critical, high, warning, info, err := service.CountBySeverity(ctx, orgID, "")

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 2, critical)
	assert.Equal(t, 5, high)
	assert.Equal(t, 10, warning)
	assert.Equal(t, 3, info)

	mockAlertRepo.AssertExpectations(t)
}

// ====================
// GetAlerts Tests
// ====================

func TestAlertService_GetAlerts_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	alerts := []*domain.Alert{
		createTestAlertForAlerts(orgID),
		createTestAlertForAlerts(orgID),
	}

	mockAlertRepo.On("GetByOrganizationFiltered", orgID, "", 10, 0).Return(alerts, nil)
	mockAlertRepo.On("CountByOrganizationFiltered", orgID, "").Return(2, nil)

	// Act
	ctx := context.Background()
	result, total, err := service.GetAlerts(ctx, orgID, "", "", 10, 0)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, 2, total)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_GetAlerts_WithFilter(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	alerts := []*domain.Alert{
		createTestAlertForAlerts(orgID),
	}

	mockAlertRepo.On("GetByOrganizationFiltered", orgID, "unacknowledged", 10, 0).Return(alerts, nil)
	mockAlertRepo.On("CountByOrganizationFiltered", orgID, "unacknowledged").Return(1, nil)

	// Act
	ctx := context.Background()
	result, total, err := service.GetAlerts(ctx, orgID, "", "unacknowledged", 10, 0)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 1, total)

	mockAlertRepo.AssertExpectations(t)
}

// ====================
// AcknowledgeAlert Tests
// ====================

func TestAlertService_AcknowledgeAlert_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	alertID := uuid.New()
	userID := uuid.New()
	alert := createTestAlertForAlerts(orgID)
	alert.ID = alertID

	mockAlertRepo.On("GetByID", alertID).Return(alert, nil)
	mockAlertRepo.On("Acknowledge", alertID, userID).Return(nil)

	// Act
	ctx := context.Background()
	err := service.AcknowledgeAlert(ctx, alertID, orgID, userID)

	// Assert
	assert.NoError(t, err)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_AcknowledgeAlert_NotFound(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	alertID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockAlertRepo.On("GetByID", alertID).Return(nil, errors.New("not found"))

	// Act
	ctx := context.Background()
	err := service.AcknowledgeAlert(ctx, alertID, orgID, userID)

	// Assert
	assert.Error(t, err)

	mockAlertRepo.AssertExpectations(t)
}

// ====================
// BulkAcknowledgeAlerts Tests
// ====================

func TestAlertService_BulkAcknowledgeAlerts_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	userID := uuid.New()

	mockAlertRepo.On("BulkAcknowledge", orgID, userID).Return(5, nil)

	// Act
	ctx := context.Background()
	count, err := service.BulkAcknowledgeAlerts(ctx, orgID, userID)

	// Assert
	assert.NoError(t, err)
	assert.Equal(t, 5, count)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_BulkAcknowledgeAlerts_Error(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	userID := uuid.New()

	mockAlertRepo.On("BulkAcknowledge", orgID, userID).Return(0, errors.New("database error"))

	// Act
	ctx := context.Background()
	count, err := service.BulkAcknowledgeAlerts(ctx, orgID, userID)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, 0, count)

	mockAlertRepo.AssertExpectations(t)
}

// ====================
// CheckTrustScores Tests
// ====================

func TestAlertService_CheckTrustScores_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	// Create agents with various trust scores
	agents := []*domain.Agent{
		{
			ID:          uuid.New(),
			DisplayName: "Healthy Agent",
			TrustScore:  0.85,
			Status:      domain.AgentStatusVerified,
		},
		{
			ID:          uuid.New(),
			DisplayName: "Low Trust Agent",
			TrustScore:  0.25, // Below 0.4 threshold
			Status:      domain.AgentStatusVerified,
		},
	}

	mockAgentRepo.On("GetByOrganization", orgID).Return(agents, nil)
	mockAlertRepo.On("GetUnacknowledged", orgID).Return([]*domain.Alert{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)

	// Act
	ctx := context.Background()
	err := service.CheckTrustScores(ctx, orgID)

	// Assert
	assert.NoError(t, err)

	mockAgentRepo.AssertExpectations(t)
	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_CheckTrustScores_NoLowScores(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	// All agents have healthy trust scores
	agents := []*domain.Agent{
		{ID: uuid.New(), DisplayName: "Agent1", TrustScore: 0.85, Status: domain.AgentStatusVerified},
		{ID: uuid.New(), DisplayName: "Agent2", TrustScore: 0.90, Status: domain.AgentStatusVerified},
	}

	mockAgentRepo.On("GetByOrganization", orgID).Return(agents, nil)

	// Act
	ctx := context.Background()
	err := service.CheckTrustScores(ctx, orgID)

	// Assert
	assert.NoError(t, err)

	mockAgentRepo.AssertExpectations(t)
	// No alerts should be created
}

func TestAlertService_CheckTrustScores_Error(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	mockAgentRepo.On("GetByOrganization", orgID).Return(nil, errors.New("database error"))

	// Act
	ctx := context.Background()
	err := service.CheckTrustScores(ctx, orgID)

	// Assert
	assert.Error(t, err)

	mockAgentRepo.AssertExpectations(t)
}

// ====================
// CheckTrustScoreDrop Tests
// ====================

func TestAlertService_CheckTrustScoreDrop_SignificantDrop(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	agentID := uuid.New()

	mockAlertRepo.On("GetUnacknowledged", orgID).Return([]*domain.Alert{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)

	// Act - 15% drop (above 10% threshold)
	ctx := context.Background()
	err := service.CheckTrustScoreDrop(ctx, orgID, agentID, "Test Agent", 0.85, 0.70)

	// Assert
	assert.NoError(t, err)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_CheckTrustScoreDrop_CriticalDrop(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	agentID := uuid.New()

	mockAlertRepo.On("GetUnacknowledged", orgID).Return([]*domain.Alert{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)

	// Act - 30% drop (above 20% critical threshold)
	ctx := context.Background()
	err := service.CheckTrustScoreDrop(ctx, orgID, agentID, "Test Agent", 0.90, 0.60)

	// Assert
	assert.NoError(t, err)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_CheckTrustScoreDrop_NoDrop(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	agentID := uuid.New()

	// Act - Score increased, no alert
	ctx := context.Background()
	err := service.CheckTrustScoreDrop(ctx, orgID, agentID, "Test Agent", 0.70, 0.85)

	// Assert
	assert.NoError(t, err)
	// No mock assertions since no repo calls should happen
}

func TestAlertService_CheckTrustScoreDrop_SmallDrop(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	agentID := uuid.New()

	// Act - 5% drop (below 10% threshold)
	ctx := context.Background()
	err := service.CheckTrustScoreDrop(ctx, orgID, agentID, "Test Agent", 0.85, 0.81)

	// Assert
	assert.NoError(t, err)
	// No mock assertions since no repo calls should happen
}

func TestAlertService_CheckTrustScoreDrop_ZeroPreviousScore(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	agentID := uuid.New()

	// Act - Previous score is 0, should return early
	ctx := context.Background()
	err := service.CheckTrustScoreDrop(ctx, orgID, agentID, "Test Agent", 0.0, 0.5)

	// Assert
	assert.NoError(t, err)
	// No mock assertions since no repo calls should happen
}

// ====================
// GetAlertsByAgent Tests
// ====================

func TestAlertService_GetAlertsByAgent_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	agentID := uuid.New()
	alerts := []*domain.Alert{
		{ID: uuid.New(), ResourceID: agentID, Title: "Alert 1"},
		{ID: uuid.New(), ResourceID: agentID, Title: "Alert 2"},
	}

	mockAlertRepo.On("GetByResourceID", agentID, 10, 0).Return(alerts, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetAlertsByAgent(ctx, agentID, 10, 0)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 2)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_GetAlertsByAgent_Empty(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	agentID := uuid.New()

	mockAlertRepo.On("GetByResourceID", agentID, 10, 0).Return([]*domain.Alert{}, nil)

	// Act
	ctx := context.Background()
	result, err := service.GetAlertsByAgent(ctx, agentID, 10, 0)

	// Assert
	assert.NoError(t, err)
	assert.Len(t, result, 0)

	mockAlertRepo.AssertExpectations(t)
}

// ====================
// DefaultUnusualAccessConfig Tests
// ====================

func TestDefaultUnusualAccessConfig(t *testing.T) {
	config := DefaultUnusualAccessConfig()

	assert.Equal(t, 100, config.HighVolumeThreshold)
	assert.Equal(t, 5, config.TimeWindowMinutes)
	assert.Equal(t, 0, config.OffHoursStart)
	assert.Equal(t, 0, config.OffHoursEnd)
	assert.Equal(t, 24*time.Hour, config.NewResourceAlertDelay)
}

// ====================
// DefaultTrustScoreDropConfig Tests
// ====================

func TestDefaultTrustScoreDropConfig(t *testing.T) {
	config := DefaultTrustScoreDropConfig()

	assert.Equal(t, 0.1, config.SignificantDropThreshold)
	assert.Equal(t, 0.2, config.CriticalDropThreshold)
	assert.Equal(t, 0.5, config.LowScoreThreshold)
}

// ====================
// RunProactiveChecks Tests
// ====================

func TestAlertService_RunProactiveChecks_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	// CheckTrustScores setup - all agents healthy
	mockAgentRepo.On("GetByOrganization", orgID).Return([]*domain.Agent{
		{ID: uuid.New(), TrustScore: 0.9, Status: domain.AgentStatusVerified},
	}, nil)

	// Act
	ctx := context.Background()
	err := service.RunProactiveChecks(ctx, orgID)

	// Assert
	assert.NoError(t, err)

	mockAgentRepo.AssertExpectations(t)
}

func TestAlertService_RunProactiveChecks_TrustScoreCheckFails(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	mockAgentRepo.On("GetByOrganization", orgID).Return(nil, errors.New("database error"))

	// Act
	ctx := context.Background()
	err := service.RunProactiveChecks(ctx, orgID)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "trust score check failed")

	mockAgentRepo.AssertExpectations(t)
}

// ====================
// SetWebhookService Tests
// ====================

func TestAlertService_SetWebhookService(t *testing.T) {
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	// Initially nil
	assert.Nil(t, service.webhookService)

	// Create a mock webhook service (we don't need real implementation for this test)
	// Just check that it can be set
	service.SetWebhookService(nil)
	assert.Nil(t, service.webhookService)
}

// ====================
// ResolveAlert Tests
// ====================

func TestAlertService_ResolveAlert_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	alertID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockAlertRepo.On("Acknowledge", alertID, userID).Return(nil)

	// Act
	ctx := context.Background()
	err := service.ResolveAlert(ctx, alertID, orgID, userID, "Issue resolved")

	// Assert
	assert.NoError(t, err)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_ResolveAlert_Error(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	alertID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()

	mockAlertRepo.On("Acknowledge", alertID, userID).Return(errors.New("database error"))

	// Act
	ctx := context.Background()
	err := service.ResolveAlert(ctx, alertID, orgID, userID, "Issue resolved")

	// Assert
	assert.Error(t, err)

	mockAlertRepo.AssertExpectations(t)
}

// ====================
// CheckAPIKeyExpiry Tests
// ====================

func TestAlertService_CheckAPIKeyExpiry_Success(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	// Act - Currently a no-op, should succeed
	ctx := context.Background()
	err := service.CheckAPIKeyExpiry(ctx, orgID)

	// Assert
	assert.NoError(t, err)
}

// ====================
// DetectUnusualAccessPatterns Tests
// ====================

func TestAlertService_DetectUnusualAccessPatterns_NoDBConfigured(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil) // nil DB

	orgID := uuid.New()
	agentID := uuid.New()

	// Act
	ctx := context.Background()
	alerts, err := service.DetectUnusualAccessPatterns(ctx, orgID, agentID)

	// Assert
	assert.NoError(t, err)
	assert.Nil(t, alerts) // No alerts when DB not configured
}

// ====================
// Struct Tests
// ====================

func TestUnusualAccessPatternConfig_Structure(t *testing.T) {
	config := UnusualAccessPatternConfig{
		HighVolumeThreshold:   200,
		TimeWindowMinutes:     10,
		OffHoursStart:         22, // 10 PM
		OffHoursEnd:           6,  // 6 AM
		NewResourceAlertDelay: 48 * time.Hour,
	}

	assert.Equal(t, 200, config.HighVolumeThreshold)
	assert.Equal(t, 10, config.TimeWindowMinutes)
	assert.Equal(t, 22, config.OffHoursStart)
	assert.Equal(t, 6, config.OffHoursEnd)
	assert.Equal(t, 48*time.Hour, config.NewResourceAlertDelay)
}

func TestTrustScoreDropConfig_Structure(t *testing.T) {
	config := TrustScoreDropConfig{
		SignificantDropThreshold: 0.15,
		CriticalDropThreshold:    0.25,
		LowScoreThreshold:        0.4,
	}

	assert.Equal(t, 0.15, config.SignificantDropThreshold)
	assert.Equal(t, 0.25, config.CriticalDropThreshold)
	assert.Equal(t, 0.4, config.LowScoreThreshold)
}

func TestApproveDriftRequest_Structure(t *testing.T) {
	alertID := uuid.New()
	orgID := uuid.New()
	userID := uuid.New()

	req := ApproveDriftRequest{
		AlertID:            alertID,
		OrganizationID:     orgID,
		UserID:             userID,
		ApprovedMCPServers: []string{"memory", "filesystem", "github"},
	}

	assert.Equal(t, alertID, req.AlertID)
	assert.Equal(t, orgID, req.OrganizationID)
	assert.Equal(t, userID, req.UserID)
	assert.Len(t, req.ApprovedMCPServers, 3)
	assert.Contains(t, req.ApprovedMCPServers, "memory")
}

// ====================
// NewAlertService Tests
// ====================

func TestNewAlertService_Constructor(t *testing.T) {
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	assert.NotNil(t, service)
	assert.Equal(t, mockAlertRepo, service.alertRepo)
	assert.Equal(t, mockAgentRepo, service.agentRepo)
	assert.Nil(t, service.db)
	assert.Nil(t, service.webhookService)
}

// ====================
// CheckTrustScoreDrop Edge Cases
// ====================

func TestAlertService_CheckTrustScoreDrop_DropBelowLowScoreThreshold(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	agentID := uuid.New()

	mockAlertRepo.On("GetUnacknowledged", orgID).Return([]*domain.Alert{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)

	// Act - Small drop but ending below 50% threshold
	ctx := context.Background()
	err := service.CheckTrustScoreDrop(ctx, orgID, agentID, "Test Agent", 0.55, 0.45)

	// Assert
	assert.NoError(t, err)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_CheckTrustScoreDrop_DuplicateAlert(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	agentID := uuid.New()

	existingAlert := &domain.Alert{
		ID:         uuid.New(),
		ResourceID: agentID,
		AlertType:  domain.AlertTrustScoreDrop,
	}

	mockAlertRepo.On("GetUnacknowledged", orgID).Return([]*domain.Alert{existingAlert}, nil)

	// Act - Alert already exists, should not create duplicate
	ctx := context.Background()
	err := service.CheckTrustScoreDrop(ctx, orgID, agentID, "Test Agent", 0.90, 0.60)

	// Assert
	assert.NoError(t, err)

	mockAlertRepo.AssertExpectations(t)
	// Create should NOT have been called
}

// ====================
// CountUnacknowledged Error Cases
// ====================

func TestAlertService_CountUnacknowledged_UnackError(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	mockAlertRepo.On("CountByOrganization", orgID).Return(10, nil)
	mockAlertRepo.On("GetUnacknowledged", orgID).Return(nil, errors.New("database error"))

	// Act
	ctx := context.Background()
	allCount, ackCount, unackCount, err := service.CountUnacknowledged(ctx, orgID)

	// Assert
	assert.Error(t, err)
	assert.Equal(t, 0, allCount)
	assert.Equal(t, 0, ackCount)
	assert.Equal(t, 0, unackCount)

	mockAlertRepo.AssertExpectations(t)
}

// ====================
// GetAlerts Error Cases
// ====================

func TestAlertService_GetAlerts_FilteredError(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()

	mockAlertRepo.On("GetByOrganizationFiltered", orgID, "", 10, 0).Return(nil, errors.New("database error"))

	// Act
	ctx := context.Background()
	result, total, err := service.GetAlerts(ctx, orgID, "", "", 10, 0)

	// Assert
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, 0, total)

	mockAlertRepo.AssertExpectations(t)
}

func TestAlertService_GetAlerts_CountError(t *testing.T) {
	// Arrange
	mockAlertRepo := new(MockAlertRepoForAlerts)
	mockAgentRepo := new(MockAgentRepoForAlerts)

	service := NewAlertService(mockAlertRepo, mockAgentRepo, nil)

	orgID := uuid.New()
	alerts := []*domain.Alert{createTestAlertForAlerts(orgID)}

	mockAlertRepo.On("GetByOrganizationFiltered", orgID, "", 10, 0).Return(alerts, nil)
	mockAlertRepo.On("CountByOrganizationFiltered", orgID, "").Return(0, errors.New("count error"))

	// Act
	ctx := context.Background()
	result, total, err := service.GetAlerts(ctx, orgID, "", "", 10, 0)

	// Assert - Returns partial result with error
	assert.Error(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, 0, total)

	mockAlertRepo.AssertExpectations(t)
}
