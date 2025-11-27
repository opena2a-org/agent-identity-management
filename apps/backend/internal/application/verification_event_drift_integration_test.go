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
