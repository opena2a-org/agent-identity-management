package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockCapabilityRepository for testing
type MockCapabilityRepository struct {
	mock.Mock
}

func (m *MockCapabilityRepository) CreateCapability(capability *domain.AgentCapability) error {
	args := m.Called(capability)
	return args.Error(0)
}

func (m *MockCapabilityRepository) GetCapabilityByID(id uuid.UUID) (*domain.AgentCapability, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepository) GetCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepository) GetActiveCapabilitiesByAgentID(agentID uuid.UUID) ([]*domain.AgentCapability, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentCapability), args.Error(1)
}

func (m *MockCapabilityRepository) RevokeCapability(id uuid.UUID, revokedAt time.Time) error {
	args := m.Called(id, revokedAt)
	return args.Error(0)
}

func (m *MockCapabilityRepository) DeleteCapability(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockCapabilityRepository) CreateViolation(violation *domain.CapabilityViolation) error {
	args := m.Called(violation)
	return args.Error(0)
}

func (m *MockCapabilityRepository) GetViolationByID(id uuid.UUID) (*domain.CapabilityViolation, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.CapabilityViolation), args.Error(1)
}

func (m *MockCapabilityRepository) GetViolationsByAgentID(agentID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	args := m.Called(agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.CapabilityViolation), args.Int(1), args.Error(2)
}

func (m *MockCapabilityRepository) GetRecentViolations(orgID uuid.UUID, minutes int) ([]*domain.CapabilityViolation, error) {
	args := m.Called(orgID, minutes)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.CapabilityViolation), args.Error(1)
}

func (m *MockCapabilityRepository) GetViolationsByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.CapabilityViolation, int, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Int(1), args.Error(2)
	}
	return args.Get(0).([]*domain.CapabilityViolation), args.Int(1), args.Error(2)
}

// Test 1: Agent with no capabilities (neutral baseline)
func TestCalculateCapabilityRisk_NoCapabilities(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "test-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)
	// Mock: No violations
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.7, score, "Agent with no capabilities should have neutral score of 0.7")
	mockRepo.AssertExpectations(t)
}

// Test 2: Agent with only low-risk capabilities
func TestCalculateCapabilityRisk_LowRiskOnly(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "low-risk-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: Low-risk capabilities (file:read, db:query)
	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityFileRead,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityDBQuery,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	expectedScore := 0.7 - 0.03 - 0.03 // 0.64
	assert.InDelta(t, expectedScore, score, 0.001, "Low-risk capabilities should have minor penalties")
	mockRepo.AssertExpectations(t)
}

// Test 3: Agent with high-risk capabilities
func TestCalculateCapabilityRisk_HighRiskCapabilities(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "high-risk-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: High-risk capabilities (system:admin, user:impersonate)
	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilitySystemAdmin,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityUserImpersonate,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	expectedScore := 0.7 - 0.20 - 0.20 // 0.30
	assert.InDelta(t, expectedScore, score, 0.001, "High-risk capabilities should have major penalties")
	mockRepo.AssertExpectations(t)
}

// Test 4: Agent with medium-risk capabilities
func TestCalculateCapabilityRisk_MediumRiskCapabilities(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "medium-risk-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: Medium-risk capabilities (file:write, db:write, api:call)
	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityFileWrite,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityDBWrite,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityAPICall,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	expectedScore := 0.7 - 0.08 - 0.08 - 0.05 // 0.49
	assert.InDelta(t, expectedScore, score, 0.001, "Medium-risk capabilities should have moderate penalties")
	mockRepo.AssertExpectations(t)
}

// Test 5: Agent with recent CRITICAL violations
func TestCalculateCapabilityRisk_CriticalViolations(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "violation-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: Single low-risk capability
	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityFileRead,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	// Mock: 3 recent CRITICAL violations (last 7 days)
	now := time.Now()
	violations := []*domain.CapabilityViolation{
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -1), // 1 day ago
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -5), // 5 days ago
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -7), // 7 days ago
		},
	}

	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 3, nil)

	score := calculator.calculateCapabilityRisk(agent)

	// Expected: 0.7 - 0.03 (file:read) - (3 * 0.15) (critical violations) = 0.22
	expectedScore := 0.7 - 0.03 - (3 * 0.15)
	assert.InDelta(t, expectedScore, score, 0.001, "CRITICAL violations should heavily impact trust")
	mockRepo.AssertExpectations(t)
}

// Test 6: Agent with many violations (volume penalty)
func TestCalculateCapabilityRisk_HighViolationVolume(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "high-volume-violation-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)

	// Mock: 12 recent LOW violations (triggers volume penalty)
	now := time.Now()
	violations := make([]*domain.CapabilityViolation, 12)
	for i := 0; i < 12; i++ {
		violations[i] = &domain.CapabilityViolation{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilityFileWrite,
			Severity:            domain.ViolationSeverityLow,
			TrustScoreImpact:    -2,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -i-1),
		}
	}

	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 12, nil)

	score := calculator.calculateCapabilityRisk(agent)

	// Expected: 0.7 - (12 * 0.02) - 0.20 (volume penalty) = 0.26
	expectedScore := 0.7 - (12 * 0.02) - 0.20
	assert.InDelta(t, expectedScore, score, 0.001, "High violation volume should trigger additional penalty")
	mockRepo.AssertExpectations(t)
}

// Test 7: Score bounds enforcement (cannot go below 0)
func TestCalculateCapabilityRisk_ScoreBoundsMinimum(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "extreme-risk-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: All high-risk capabilities
	capabilities := []*domain.AgentCapability{
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilitySystemAdmin,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityUserImpersonate,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityFileDelete,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
		{
			ID:             uuid.New(),
			AgentID:        agent.ID,
			CapabilityType: domain.CapabilityDataExport,
			GrantedAt:      time.Now(),
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		},
	}

	// Mock: Many CRITICAL violations
	now := time.Now()
	violations := make([]*domain.CapabilityViolation, 20)
	for i := 0; i < 20; i++ {
		violations[i] = &domain.CapabilityViolation{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -i-1),
		}
	}

	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(capabilities, nil)
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 20, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.0, score, "Score should never go below 0")
	mockRepo.AssertExpectations(t)
}

// Test 8: Old violations should not impact score
func TestCalculateCapabilityRisk_OldViolationsIgnored(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "old-violation-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)

	// Mock: Violations older than 30 days
	now := time.Now()
	violations := []*domain.CapabilityViolation{
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -35), // 35 days ago (outside 30-day window)
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -60), // 60 days ago
		},
	}

	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 2, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.7, score, "Violations older than 30 days should not impact score")
	mockRepo.AssertExpectations(t)
}

// Test 9: Mixed severity violations
func TestCalculateCapabilityRisk_MixedSeverityViolations(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "mixed-violations-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: No capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)

	// Mock: Various severity violations
	now := time.Now()
	violations := []*domain.CapabilityViolation{
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilitySystemAdmin,
			Severity:            domain.ViolationSeverityCritical,
			TrustScoreImpact:    -15,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -1), // 1 day ago
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilityFileDelete,
			Severity:            domain.ViolationSeverityHigh,
			TrustScoreImpact:    -10,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -2), // 2 days ago
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilityFileWrite,
			Severity:            domain.ViolationSeverityMedium,
			TrustScoreImpact:    -5,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -3), // 3 days ago
		},
		{
			ID:                  uuid.New(),
			AgentID:             agent.ID,
			AttemptedCapability: domain.CapabilityFileRead,
			Severity:            domain.ViolationSeverityLow,
			TrustScoreImpact:    -2,
			IsBlocked:           true,
			CreatedAt:           now.AddDate(0, 0, -4), // 4 days ago
		},
	}

	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 4, nil)

	score := calculator.calculateCapabilityRisk(agent)

	// Expected: 0.7 - 0.15 (critical) - 0.10 (high) - 0.05 (medium) - 0.02 (low) = 0.38
	expectedScore := 0.7 - 0.15 - 0.10 - 0.05 - 0.02
	assert.InDelta(t, expectedScore, score, 0.001, "Mixed severity violations should have varying impacts")
	mockRepo.AssertExpectations(t)
}

// Test 10: Error handling - repository error returns baseline
func TestCalculateCapabilityRisk_RepositoryError(t *testing.T) {
	mockRepo := new(MockCapabilityRepository)
	calculator := &TrustCalculator{
		capabilityRepo: mockRepo,
	}

	agent := &domain.Agent{
		ID:        uuid.New(),
		Name:      "error-agent",
		AgentType: domain.AgentTypeAI,
		Status:    domain.AgentStatusVerified,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Mock: Repository error for capabilities
	mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return(nil, assert.AnError)
	// Mock: No violations when capability fetch fails
	mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return([]*domain.CapabilityViolation{}, 0, nil)

	score := calculator.calculateCapabilityRisk(agent)

	assert.Equal(t, 0.7, score, "Repository error should return neutral baseline score")
	mockRepo.AssertExpectations(t)
}

// Test 11: Volume penalty thresholds
func TestCalculateCapabilityRisk_ViolationVolumeThresholds(t *testing.T) {
	now := time.Now()

	// Test 6 violations (should trigger -0.10 penalty)
	t.Run("6 violations", func(t *testing.T) {
		mockRepo := new(MockCapabilityRepository)
		calculator := &TrustCalculator{
			capabilityRepo: mockRepo,
		}

		agent := &domain.Agent{
			ID:        uuid.New(),
			Name:      "6-violations-agent",
			AgentType: domain.AgentTypeAI,
			Status:    domain.AgentStatusVerified,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)

		violations := make([]*domain.CapabilityViolation, 6)
		for i := 0; i < 6; i++ {
			violations[i] = &domain.CapabilityViolation{
				ID:                  uuid.New(),
				AgentID:             agent.ID,
				AttemptedCapability: domain.CapabilityFileRead,
				Severity:            domain.ViolationSeverityLow,
				TrustScoreImpact:    -2,
				IsBlocked:           true,
				CreatedAt:           now.AddDate(0, 0, -i-1),
			}
		}

		mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 6, nil)

		score := calculator.calculateCapabilityRisk(agent)

		// Expected: 0.7 - (6 * 0.02) - 0.10 (volume penalty > 5) = 0.48
		expectedScore := 0.7 - (6 * 0.02) - 0.10
		assert.InDelta(t, expectedScore, score, 0.001, "6 violations should trigger -0.10 volume penalty")
		mockRepo.AssertExpectations(t)
	})

	// Test 11 violations (should trigger -0.20 penalty)
	t.Run("11 violations", func(t *testing.T) {
		mockRepo := new(MockCapabilityRepository)
		calculator := &TrustCalculator{
			capabilityRepo: mockRepo,
		}

		agent := &domain.Agent{
			ID:        uuid.New(),
			Name:      "11-violations-agent",
			AgentType: domain.AgentTypeAI,
			Status:    domain.AgentStatusVerified,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		mockRepo.On("GetActiveCapabilitiesByAgentID", agent.ID).Return([]*domain.AgentCapability{}, nil)

		violations := make([]*domain.CapabilityViolation, 11)
		for i := 0; i < 11; i++ {
			violations[i] = &domain.CapabilityViolation{
				ID:                  uuid.New(),
				AgentID:             agent.ID,
				AttemptedCapability: domain.CapabilityFileRead,
				Severity:            domain.ViolationSeverityLow,
				TrustScoreImpact:    -2,
				IsBlocked:           true,
				CreatedAt:           now.AddDate(0, 0, -i-1),
			}
		}

		mockRepo.On("GetViolationsByAgentID", agent.ID, 100, 0).Return(violations, 11, nil)

		score := calculator.calculateCapabilityRisk(agent)

		// Expected: 0.7 - (11 * 0.02) - 0.20 (volume penalty > 10) = 0.28
		expectedScore := 0.7 - (11 * 0.02) - 0.20
		assert.InDelta(t, expectedScore, score, 0.001, "11 violations should trigger -0.20 volume penalty")
		mockRepo.AssertExpectations(t)
	})
}
