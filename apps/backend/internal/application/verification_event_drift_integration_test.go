package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockVerificationEventRepository for testing
type MockVerificationEventRepository struct {
	mock.Mock
}

func (m *MockVerificationEventRepository) Create(event *domain.VerificationEvent) error {
	args := m.Called(event)
	return args.Error(0)
}

func (m *MockVerificationEventRepository) GetByID(id uuid.UUID) (*domain.VerificationEvent, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VerificationEvent), args.Error(1)
}

func (m *MockVerificationEventRepository) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), args.Error(2)
}

func (m *MockVerificationEventRepository) GetByAgent(agentID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	args := m.Called(agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), args.Error(2)
}

func (m *MockVerificationEventRepository) GetRecentEvents(orgID uuid.UUID, minutes int) ([]*domain.VerificationEvent, error) {
	args := m.Called(orgID, minutes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Error(1)
}

func (m *MockVerificationEventRepository) GetStatistics(orgID uuid.UUID, startTime, endTime time.Time) (*domain.VerificationStatistics, error) {
	args := m.Called(orgID, startTime, endTime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.VerificationStatistics), args.Error(1)
}

func (m *MockVerificationEventRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockVerificationEventRepository) GetByMCPServer(mcpServerID uuid.UUID, limit, offset int) ([]*domain.VerificationEvent, int, error) {
	args := m.Called(mcpServerID, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), args.Error(2)
}

func (m *MockVerificationEventRepository) UpdateResult(id uuid.UUID, result domain.VerificationResult, reason *string, metadata map[string]interface{}) error {
	args := m.Called(id, result, reason, metadata)
	return args.Error(0)
}

func (m *MockVerificationEventRepository) GetAgentStatistics(agentID uuid.UUID, startTime, endTime time.Time) (*domain.AgentVerificationStatistics, error) {
	args := m.Called(agentID, startTime, endTime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentVerificationStatistics), args.Error(1)
}

func (m *MockVerificationEventRepository) GetPendingVerifications(orgID uuid.UUID) ([]*domain.VerificationEvent, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Error(1)
}

func (m *MockVerificationEventRepository) SearchAdminVerifications(orgID uuid.UUID, params domain.VerificationQueryParams) ([]*domain.VerificationEvent, int, *domain.VerificationStatusCounts, error) {
	args := m.Called(orgID, params)
	if args.Get(0) == nil {
		return nil, 0, nil, args.Error(3)
	}
	var counts *domain.VerificationStatusCounts
	if args.Get(2) != nil {
		counts = args.Get(2).(*domain.VerificationStatusCounts)
	}
	return args.Get(0).([]*domain.VerificationEvent), args.Int(1), counts, args.Error(3)
}

// TestVerificationEventWithDriftDetection tests the complete flow of verification event creation with drift detection
func TestVerificationEventWithDriftDetection(t *testing.T) {
	// Setup
	mockEventRepo := new(MockVerificationEventRepository)
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)

	// Create drift detection service
	driftService := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)

	// Create verification event service
	verificationService := NewVerificationEventService(
		mockEventRepo,
		mockAgentRepo,
		driftService,
	)

	// Test data
	orgID := uuid.New()
	agentID := uuid.New()

	// Agent with registered configuration
	agent := &domain.Agent{
		ID:             agentID,
		OrganizationID: orgID,
		DisplayName:    "test-agent",
		TrustScore:     85.0,
		TalksTo:        []string{"filesystem-mcp", "database-mcp"}, // Registered MCP servers
	}

	// Runtime configuration with unauthorized MCP server
	runtimeMCPServers := []string{"filesystem-mcp", "database-mcp", "external-api-mcp"}

	t.Run("creates verification event with drift detection", func(t *testing.T) {
		// Mock agent retrieval
		mockAgentRepo.On("GetByID", agentID).Return(agent, nil)

		// Mock trust score update (called when drift is detected)
		mockAgentRepo.On("UpdateTrustScore", mock.Anything, mock.Anything).Return(nil)

		// Mock alert creation (drift will be detected)
		mockAlertRepo.On("Create", mock.MatchedBy(func(alert *domain.Alert) bool {
			return alert.AlertType == domain.AlertTypeConfigurationDrift &&
				alert.Severity == domain.AlertSeverityHigh &&
				alert.ResourceType == "agent" &&
				alert.ResourceID == agentID
		})).Return(nil)

		// Mock verification event creation
		mockEventRepo.On("Create", mock.MatchedBy(func(event *domain.VerificationEvent) bool {
			// Verify drift was detected
			return event.DriftDetected == true &&
				len(event.MCPServerDrift) == 1 &&
				event.MCPServerDrift[0] == "external-api-mcp"
		})).Return(nil)

		// Create verification event request with runtime configuration
		req := &CreateVerificationEventRequest{
			OrganizationID:      orgID,
			AgentID:             agentID,
			Protocol:            domain.VerificationProtocolMCP,
			VerificationType:    domain.VerificationTypeIdentity,
			Status:              domain.VerificationEventStatusSuccess,
			Confidence:          0.95,
			DurationMs:          150,
			InitiatorType:       domain.InitiatorTypeSystem,
			StartedAt:           time.Now().Add(-150 * time.Millisecond),
			CurrentMCPServers:   runtimeMCPServers,
			CurrentCapabilities: []string{},
		}

		// Execute
		event, err := verificationService.CreateVerificationEvent(context.Background(), req)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, event)
		assert.True(t, event.DriftDetected, "Drift should be detected")
		assert.Equal(t, 1, len(event.MCPServerDrift), "Should detect one unauthorized MCP server")
		assert.Equal(t, "external-api-mcp", event.MCPServerDrift[0], "Should identify external-api-mcp as drift")

		// Verify all mocks were called
		mockAgentRepo.AssertExpectations(t)
		mockAlertRepo.AssertExpectations(t)
		mockEventRepo.AssertExpectations(t)
	})

	t.Run("creates verification event without drift when configuration matches", func(t *testing.T) {
		// Reset mocks
		mockAgentRepo = new(MockAgentRepository)
		mockEventRepo = new(MockVerificationEventRepository)

		// Recreate services with fresh mocks
		driftService = NewDriftDetectionService(mockAgentRepo, mockAlertRepo)
		verificationService = NewVerificationEventService(
			mockEventRepo,
			mockAgentRepo,
			driftService,
		)

		// Mock agent retrieval
		mockAgentRepo.On("GetByID", agentID).Return(agent, nil)

		// Mock verification event creation (no drift expected)
		mockEventRepo.On("Create", mock.MatchedBy(func(event *domain.VerificationEvent) bool {
			// Verify no drift was detected
			return event.DriftDetected == false &&
				len(event.MCPServerDrift) == 0
		})).Return(nil)

		// Create verification event request with matching configuration
		req := &CreateVerificationEventRequest{
			OrganizationID:    orgID,
			AgentID:           agentID,
			Protocol:          domain.VerificationProtocolMCP,
			VerificationType:  domain.VerificationTypeIdentity,
			Status:            domain.VerificationEventStatusSuccess,
			Confidence:        0.95,
			DurationMs:        150,
			InitiatorType:     domain.InitiatorTypeSystem,
			StartedAt:         time.Now().Add(-150 * time.Millisecond),
			CurrentMCPServers: []string{"filesystem-mcp", "database-mcp"}, // Matches registered
		}

		// Execute
		event, err := verificationService.CreateVerificationEvent(context.Background(), req)

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, event)
		assert.False(t, event.DriftDetected, "No drift should be detected")
		assert.Equal(t, 0, len(event.MCPServerDrift), "Should not detect any drift")

		// Verify mocks
		mockAgentRepo.AssertExpectations(t)
		mockEventRepo.AssertExpectations(t)
	})
}

// TestSearchAdminVerifications tests the SearchVerifications service method
func TestSearchAdminVerifications(t *testing.T) {
	// Setup
	mockEventRepo := new(MockVerificationEventRepository)
	mockAgentRepo := new(MockAgentRepository)
	mockAlertRepo := new(MockAlertRepository)

	driftService := NewDriftDetectionService(mockAgentRepo, mockAlertRepo)
	verificationService := NewVerificationEventService(
		mockEventRepo,
		mockAgentRepo,
		driftService,
	)

	orgID := uuid.New()
	agentID := uuid.New()

	sampleEvents := []*domain.VerificationEvent{
		{
			ID:             uuid.New(),
			OrganizationID: orgID,
			AgentID:        &agentID,
			Status:         domain.VerificationEventStatusPending,
			StartedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			OrganizationID: orgID,
			AgentID:        &agentID,
			Status:         domain.VerificationEventStatusSuccess,
			StartedAt:      time.Now().Add(-1 * time.Hour),
		},
	}

	sampleCounts := &domain.VerificationStatusCounts{
		Pending:  5,
		Approved: 10,
		Denied:   2,
	}

	t.Run("returns paginated results with status counts", func(t *testing.T) {
		params := domain.VerificationQueryParams{
			Limit:  10,
			Offset: 0,
		}

		mockEventRepo.On("SearchAdminVerifications", orgID, params).
			Return(sampleEvents, 2, sampleCounts, nil).Once()

		events, total, counts, err := verificationService.SearchVerifications(
			context.Background(),
			orgID,
			params,
		)

		assert.NoError(t, err)
		assert.Len(t, events, 2)
		assert.Equal(t, 2, total)
		assert.NotNil(t, counts)
		assert.Equal(t, 5, counts.Pending)
		assert.Equal(t, 10, counts.Approved)
		assert.Equal(t, 2, counts.Denied)
		mockEventRepo.AssertExpectations(t)
	})

	t.Run("filters by pending status", func(t *testing.T) {
		mockEventRepo := new(MockVerificationEventRepository)
		verificationService := NewVerificationEventService(
			mockEventRepo,
			mockAgentRepo,
			driftService,
		)

		params := domain.VerificationQueryParams{
			Status: "pending",
			Limit:  10,
			Offset: 0,
		}

		pendingOnly := []*domain.VerificationEvent{sampleEvents[0]}

		mockEventRepo.On("SearchAdminVerifications", orgID, params).
			Return(pendingOnly, 1, sampleCounts, nil).Once()

		events, total, _, err := verificationService.SearchVerifications(
			context.Background(),
			orgID,
			params,
		)

		assert.NoError(t, err)
		assert.Len(t, events, 1)
		assert.Equal(t, 1, total)
		assert.Equal(t, domain.VerificationEventStatusPending, events[0].Status)
		mockEventRepo.AssertExpectations(t)
	})

	t.Run("handles search query", func(t *testing.T) {
		mockEventRepo := new(MockVerificationEventRepository)
		verificationService := NewVerificationEventService(
			mockEventRepo,
			mockAgentRepo,
			driftService,
		)

		params := domain.VerificationQueryParams{
			Search: "test-agent",
			Limit:  10,
			Offset: 0,
		}

		mockEventRepo.On("SearchAdminVerifications", orgID, params).
			Return(sampleEvents, 2, sampleCounts, nil).Once()

		events, _, _, err := verificationService.SearchVerifications(
			context.Background(),
			orgID,
			params,
		)

		assert.NoError(t, err)
		assert.NotEmpty(t, events)
		mockEventRepo.AssertExpectations(t)
	})

	t.Run("handles risk level filter", func(t *testing.T) {
		mockEventRepo := new(MockVerificationEventRepository)
		verificationService := NewVerificationEventService(
			mockEventRepo,
			mockAgentRepo,
			driftService,
		)

		params := domain.VerificationQueryParams{
			RiskLevel: "high",
			Limit:     10,
			Offset:    0,
		}

		mockEventRepo.On("SearchAdminVerifications", orgID, params).
			Return([]*domain.VerificationEvent{}, 0, sampleCounts, nil).Once()

		events, total, _, err := verificationService.SearchVerifications(
			context.Background(),
			orgID,
			params,
		)

		assert.NoError(t, err)
		assert.Empty(t, events)
		assert.Equal(t, 0, total)
		mockEventRepo.AssertExpectations(t)
	})

	t.Run("handles empty results", func(t *testing.T) {
		mockEventRepo := new(MockVerificationEventRepository)
		verificationService := NewVerificationEventService(
			mockEventRepo,
			mockAgentRepo,
			driftService,
		)

		params := domain.VerificationQueryParams{
			Status: "denied",
			Limit:  10,
			Offset: 0,
		}

		emptyCounts := &domain.VerificationStatusCounts{
			Pending:  0,
			Approved: 0,
			Denied:   0,
		}

		mockEventRepo.On("SearchAdminVerifications", orgID, params).
			Return([]*domain.VerificationEvent{}, 0, emptyCounts, nil).Once()

		events, total, counts, err := verificationService.SearchVerifications(
			context.Background(),
			orgID,
			params,
		)

		assert.NoError(t, err)
		assert.Empty(t, events)
		assert.Equal(t, 0, total)
		assert.NotNil(t, counts)
		assert.Equal(t, 0, counts.Pending)
		mockEventRepo.AssertExpectations(t)
	})
}
