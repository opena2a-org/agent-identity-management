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

// MockAuditLogRepository is a mock implementation of AuditLogRepository
type MockAuditLogRepository struct {
	logs           []*domain.AuditLog
	logsByID       map[uuid.UUID]*domain.AuditLog
	createErr      error
	getByIDErr     error
}

func NewMockAuditLogRepository() *MockAuditLogRepository {
	return &MockAuditLogRepository{
		logs:     make([]*domain.AuditLog, 0),
		logsByID: make(map[uuid.UUID]*domain.AuditLog),
	}
}

func (m *MockAuditLogRepository) Create(log *domain.AuditLog) error {
	if m.createErr != nil {
		return m.createErr
	}
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	log.Timestamp = time.Now()
	m.logs = append(m.logs, log)
	m.logsByID[log.ID] = log
	return nil
}

func (m *MockAuditLogRepository) GetByID(id uuid.UUID) (*domain.AuditLog, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	log, ok := m.logsByID[id]
	if !ok {
		return nil, errors.New("audit log not found")
	}
	return log, nil
}

func (m *MockAuditLogRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	var result []*domain.AuditLog
	for _, log := range m.logs {
		if log.OrganizationID == orgID {
			result = append(result, log)
		}
	}
	// Apply pagination
	if offset >= len(result) {
		return []*domain.AuditLog{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (m *MockAuditLogRepository) GetByUser(userID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	var result []*domain.AuditLog
	for _, log := range m.logs {
		if log.UserID != nil && *log.UserID == userID {
			result = append(result, log)
		}
	}
	// Apply pagination
	if offset >= len(result) {
		return []*domain.AuditLog{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (m *MockAuditLogRepository) GetByAgent(agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	var result []*domain.AuditLog
	for _, log := range m.logs {
		if log.AgentID != nil && *log.AgentID == agentID {
			result = append(result, log)
		}
	}
	// Apply pagination
	if offset >= len(result) {
		return []*domain.AuditLog{}, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (m *MockAuditLogRepository) GetByResource(resourceType string, resourceID uuid.UUID) ([]*domain.AuditLog, error) {
	var result []*domain.AuditLog
	for _, log := range m.logs {
		if log.ResourceType == resourceType && log.ResourceID == resourceID {
			result = append(result, log)
		}
	}
	return result, nil
}

func (m *MockAuditLogRepository) Search(query string, limit, offset int) ([]*domain.AuditLog, error) {
	// Simple search - just return first few logs for testing
	if offset >= len(m.logs) {
		return []*domain.AuditLog{}, nil
	}
	end := offset + limit
	if end > len(m.logs) {
		end = len(m.logs)
	}
	return m.logs[offset:end], nil
}

func (m *MockAuditLogRepository) CountActionsByAgentInTimeWindow(agentID uuid.UUID, action domain.AuditAction, windowMinutes int) (int, error) {
	count := 0
	cutoff := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)
	for _, log := range m.logs {
		if log.AgentID != nil && *log.AgentID == agentID && log.Action == action && log.Timestamp.After(cutoff) {
			count++
		}
	}
	return count, nil
}

func (m *MockAuditLogRepository) GetRecentActionsByAgent(agentID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	return m.GetByAgent(agentID, limit, 0)
}

func (m *MockAuditLogRepository) GetAgentActionsByIPAddress(agentID uuid.UUID, ipAddress string, limit int) ([]*domain.AuditLog, error) {
	var result []*domain.AuditLog
	for _, log := range m.logs {
		if log.AgentID != nil && *log.AgentID == agentID && log.IPAddress == ipAddress {
			result = append(result, log)
		}
	}
	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}
	return result, nil
}

func TestNewAuditService(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)

	assert.NotNil(t, service)
}

func TestAuditService_Log(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	userID := uuid.New()

	log := &domain.AuditLog{
		OrganizationID: orgID,
		UserID:         &userID,
		Action:         domain.AuditActionCreate,
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
		IPAddress:      "192.168.1.100",
		UserAgent:      "TestClient/1.0",
		Metadata:       map[string]interface{}{"key": "value"},
	}

	err := service.Log(ctx, log)
	require.NoError(t, err)

	// Verify log was created
	assert.NotEqual(t, uuid.Nil, log.ID)
	assert.Len(t, repo.logs, 1)
}

func TestAuditService_LogAction(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	userID := uuid.New()
	resourceID := uuid.New()

	err := service.LogAction(
		ctx,
		orgID,
		userID,
		domain.AuditActionCreate,
		"agent",
		resourceID,
		"192.168.1.100",
		"TestClient/1.0",
		map[string]interface{}{"agentName": "test-agent"},
	)
	require.NoError(t, err)

	// Verify log was created with correct values
	assert.Len(t, repo.logs, 1)
	log := repo.logs[0]
	assert.Equal(t, orgID, log.OrganizationID)
	assert.Equal(t, &userID, log.UserID)
	assert.Equal(t, domain.AuditActionCreate, log.Action)
	assert.Equal(t, "agent", log.ResourceType)
	assert.Equal(t, resourceID, log.ResourceID)
	assert.Equal(t, "192.168.1.100", log.IPAddress)
}

func TestAuditService_LogAgentAction(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	agentID := uuid.New()
	resourceID := uuid.New()

	err := service.LogAgentAction(
		ctx,
		orgID,
		agentID,
		domain.AuditActionAttest,
		"mcp_server",
		resourceID,
		"10.0.0.50",
		"AIM-SDK/1.0",
		map[string]interface{}{"mcpServerDomain": "api.example.com"},
	)
	require.NoError(t, err)

	// Verify log was created with agent ID (not user ID)
	assert.Len(t, repo.logs, 1)
	log := repo.logs[0]
	assert.Nil(t, log.UserID)
	assert.Equal(t, &agentID, log.AgentID)
	assert.Equal(t, domain.AuditActionAttest, log.Action)
}

func TestAuditService_GetLogs(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	otherOrgID := uuid.New()

	// Create logs for different organizations
	for i := 0; i < 5; i++ {
		repo.Create(&domain.AuditLog{
			OrganizationID: orgID,
			Action:         domain.AuditActionCreate,
			ResourceType:   "agent",
			ResourceID:     uuid.New(),
		})
	}
	for i := 0; i < 3; i++ {
		repo.Create(&domain.AuditLog{
			OrganizationID: otherOrgID,
			Action:         domain.AuditActionCreate,
			ResourceType:   "agent",
			ResourceID:     uuid.New(),
		})
	}

	// Get logs for first org
	logs, err := service.GetLogs(ctx, orgID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 5)

	// Get logs with pagination
	logs, err = service.GetLogs(ctx, orgID, 2, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 2)

	logs, err = service.GetLogs(ctx, orgID, 10, 3)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}

func TestAuditService_GetUserLogs(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	userID := uuid.New()
	otherUserID := uuid.New()

	// Create logs for different users
	for i := 0; i < 4; i++ {
		repo.Create(&domain.AuditLog{
			OrganizationID: orgID,
			UserID:         &userID,
			Action:         domain.AuditActionLogin,
			ResourceType:   "session",
			ResourceID:     uuid.New(),
		})
	}
	repo.Create(&domain.AuditLog{
		OrganizationID: orgID,
		UserID:         &otherUserID,
		Action:         domain.AuditActionLogin,
		ResourceType:   "session",
		ResourceID:     uuid.New(),
	})

	// Get logs for first user
	logs, err := service.GetUserLogs(ctx, userID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 4)
}

func TestAuditService_GetAgentActivity(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	agentID := uuid.New()
	otherAgentID := uuid.New()

	// Create logs for different agents
	for i := 0; i < 3; i++ {
		repo.Create(&domain.AuditLog{
			OrganizationID: orgID,
			AgentID:        &agentID,
			Action:         domain.AuditActionAttest,
			ResourceType:   "mcp_server",
			ResourceID:     uuid.New(),
		})
	}
	repo.Create(&domain.AuditLog{
		OrganizationID: orgID,
		AgentID:        &otherAgentID,
		Action:         domain.AuditActionAttest,
		ResourceType:   "mcp_server",
		ResourceID:     uuid.New(),
	})

	// Get logs for first agent
	logs, err := service.GetAgentActivity(ctx, agentID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 3)
}

func TestAuditService_GetResourceLogs(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	resourceID := uuid.New()
	otherResourceID := uuid.New()

	// Create logs for different resources
	repo.Create(&domain.AuditLog{
		OrganizationID: orgID,
		Action:         domain.AuditActionCreate,
		ResourceType:   "agent",
		ResourceID:     resourceID,
	})
	repo.Create(&domain.AuditLog{
		OrganizationID: orgID,
		Action:         domain.AuditActionUpdate,
		ResourceType:   "agent",
		ResourceID:     resourceID,
	})
	repo.Create(&domain.AuditLog{
		OrganizationID: orgID,
		Action:         domain.AuditActionCreate,
		ResourceType:   "agent",
		ResourceID:     otherResourceID,
	})

	// Get logs for specific resource
	logs, err := service.GetResourceLogs(ctx, "agent", resourceID)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}

func TestAuditService_SearchLogs(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()

	// Create some logs
	for i := 0; i < 5; i++ {
		repo.Create(&domain.AuditLog{
			OrganizationID: orgID,
			Action:         domain.AuditActionCreate,
			ResourceType:   "agent",
			ResourceID:     uuid.New(),
		})
	}

	// Search (mock just returns first few)
	logs, err := service.SearchLogs(ctx, "test query", 3, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 3)
}

func TestAuditService_GetByID(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()

	// Create a log
	log := &domain.AuditLog{
		OrganizationID: orgID,
		Action:         domain.AuditActionCreate,
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
	}
	err := repo.Create(log)
	require.NoError(t, err)

	// Get by ID
	retrieved, err := service.GetByID(ctx, log.ID)
	require.NoError(t, err)
	assert.Equal(t, log.ID, retrieved.ID)
}

func TestAuditService_GetByID_NotFound(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	_, err := service.GetByID(ctx, uuid.New())
	assert.Error(t, err)
}

func TestAuditService_GetAuditLogs_ByEntityTypeAndID(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	entityID := uuid.New()

	// Create logs for specific entity
	for i := 0; i < 5; i++ {
		repo.Create(&domain.AuditLog{
			OrganizationID: orgID,
			Action:         domain.AuditActionUpdate,
			ResourceType:   "mcp_server",
			ResourceID:     entityID,
		})
	}
	// Create log for different entity
	repo.Create(&domain.AuditLog{
		OrganizationID: orgID,
		Action:         domain.AuditActionUpdate,
		ResourceType:   "mcp_server",
		ResourceID:     uuid.New(),
	})

	// Get logs for specific entity
	logs, total, err := service.GetAuditLogs(ctx, orgID, "", "mcp_server", &entityID, nil, nil, nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 5)
	assert.Equal(t, 5, total)
}

func TestAuditService_GetAuditLogs_ByEntityTypeAndID_Pagination(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	entityID := uuid.New()

	// Create many logs
	for i := 0; i < 10; i++ {
		repo.Create(&domain.AuditLog{
			OrganizationID: orgID,
			Action:         domain.AuditActionUpdate,
			ResourceType:   "agent",
			ResourceID:     entityID,
		})
	}

	// Get first page
	logs, total, err := service.GetAuditLogs(ctx, orgID, "", "agent", &entityID, nil, nil, nil, 3, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 3)
	assert.Equal(t, 10, total)

	// Get second page
	logs, total, err = service.GetAuditLogs(ctx, orgID, "", "agent", &entityID, nil, nil, nil, 3, 3)
	require.NoError(t, err)
	assert.Len(t, logs, 3)
	assert.Equal(t, 10, total)

	// Get page beyond data
	logs, total, err = service.GetAuditLogs(ctx, orgID, "", "agent", &entityID, nil, nil, nil, 10, 20)
	require.NoError(t, err)
	assert.Len(t, logs, 0)
	assert.Equal(t, 10, total)
}

func TestAuditService_GetAuditLogs_ByUserID(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()
	userID := uuid.New()

	// Create logs for specific user
	for i := 0; i < 4; i++ {
		repo.Create(&domain.AuditLog{
			OrganizationID: orgID,
			UserID:         &userID,
			Action:         domain.AuditActionLogin,
			ResourceType:   "session",
			ResourceID:     uuid.New(),
		})
	}

	// Get logs for user
	logs, total, err := service.GetAuditLogs(ctx, orgID, "", "", nil, &userID, nil, nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 4)
	assert.Equal(t, 4, total)
}

func TestAuditService_GetAuditLogs_DefaultOrgLogs(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	orgID := uuid.New()

	// Create logs
	for i := 0; i < 6; i++ {
		repo.Create(&domain.AuditLog{
			OrganizationID: orgID,
			Action:         domain.AuditActionCreate,
			ResourceType:   "agent",
			ResourceID:     uuid.New(),
		})
	}

	// Get all org logs (no filters)
	logs, total, err := service.GetAuditLogs(ctx, orgID, "", "", nil, nil, nil, nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, logs, 6)
	assert.Equal(t, 6, total)
}

func TestAuditService_Log_Error(t *testing.T) {
	repo := NewMockAuditLogRepository()
	repo.createErr = errors.New("database error")
	service := NewAuditService(repo)
	ctx := context.Background()

	log := &domain.AuditLog{
		OrganizationID: uuid.New(),
		Action:         domain.AuditActionCreate,
		ResourceType:   "agent",
		ResourceID:     uuid.New(),
	}

	err := service.Log(ctx, log)
	assert.Error(t, err)
	assert.Equal(t, "database error", err.Error())
}

func TestAuditService_LogAction_Error(t *testing.T) {
	repo := NewMockAuditLogRepository()
	repo.createErr = errors.New("database error")
	service := NewAuditService(repo)
	ctx := context.Background()

	err := service.LogAction(
		ctx,
		uuid.New(),
		uuid.New(),
		domain.AuditActionCreate,
		"agent",
		uuid.New(),
		"192.168.1.100",
		"TestClient/1.0",
		nil,
	)
	assert.Error(t, err)
}

func TestAuditService_AllAuditActions(t *testing.T) {
	repo := NewMockAuditLogRepository()
	service := NewAuditService(repo)
	ctx := context.Background()

	actions := []domain.AuditAction{
		domain.AuditActionLogin,
		domain.AuditActionLogout,
		domain.AuditActionCreate,
		domain.AuditActionUpdate,
		domain.AuditActionDelete,
		domain.AuditActionVerify,
		domain.AuditActionAttest,
		domain.AuditActionView,
		domain.AuditActionRevoke,
		domain.AuditActionCalculate,
		domain.AuditActionAcknowledge,
		domain.AuditActionResolve,
		domain.AuditActionGenerate,
		domain.AuditActionExport,
		domain.AuditActionCheck,
		domain.AuditActionTest,
	}

	orgID := uuid.New()
	userID := uuid.New()

	for _, action := range actions {
		t.Run(string(action), func(t *testing.T) {
			err := service.LogAction(
				ctx,
				orgID,
				userID,
				action,
				"resource",
				uuid.New(),
				"127.0.0.1",
				"Test/1.0",
				nil,
			)
			require.NoError(t, err)
		})
	}

	// Verify all were logged
	logs, err := service.GetLogs(ctx, orgID, 100, 0)
	require.NoError(t, err)
	assert.Len(t, logs, len(actions))
}
