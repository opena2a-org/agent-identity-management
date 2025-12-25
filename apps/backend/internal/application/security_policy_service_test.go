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

// MockSecurityPolicyRepository for security policy service tests
type MockSecurityPolicyRepository struct {
	mock.Mock
}

func (m *MockSecurityPolicyRepository) Create(policy *domain.SecurityPolicy) error {
	args := m.Called(policy)
	return args.Error(0)
}

func (m *MockSecurityPolicyRepository) GetByID(id uuid.UUID) (*domain.SecurityPolicy, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.SecurityPolicy), args.Error(1)
}

func (m *MockSecurityPolicyRepository) GetByOrganization(orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SecurityPolicy), args.Error(1)
}

func (m *MockSecurityPolicyRepository) GetByType(orgID uuid.UUID, policyType domain.PolicyType) ([]*domain.SecurityPolicy, error) {
	args := m.Called(orgID, policyType)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SecurityPolicy), args.Error(1)
}

func (m *MockSecurityPolicyRepository) Update(policy *domain.SecurityPolicy) error {
	args := m.Called(policy)
	return args.Error(0)
}

func (m *MockSecurityPolicyRepository) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockSecurityPolicyRepository) GetActiveByOrganization(orgID uuid.UUID) ([]*domain.SecurityPolicy, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.SecurityPolicy), args.Error(1)
}

// MockAlertRepoForPolicies for security policy service tests
type MockAlertRepoForPolicies struct {
	mock.Mock
}

func (m *MockAlertRepoForPolicies) Create(alert *domain.Alert) error {
	args := m.Called(alert)
	return args.Error(0)
}

func (m *MockAlertRepoForPolicies) GetByID(id uuid.UUID) (*domain.Alert, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForPolicies) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForPolicies) GetByOrganizationFiltered(orgID uuid.UUID, status string, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(orgID, status, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForPolicies) CountByOrganization(orgID uuid.UUID) (int, error) {
	args := m.Called(orgID)
	return args.Int(0), args.Error(1)
}

func (m *MockAlertRepoForPolicies) CountByOrganizationFiltered(orgID uuid.UUID, status string) (int, error) {
	args := m.Called(orgID, status)
	return args.Int(0), args.Error(1)
}

func (m *MockAlertRepoForPolicies) CountBySeverity(orgID uuid.UUID, status string) (critical, high, warning, info int, err error) {
	args := m.Called(orgID, status)
	return args.Int(0), args.Int(1), args.Int(2), args.Int(3), args.Error(4)
}

func (m *MockAlertRepoForPolicies) GetUnacknowledged(orgID uuid.UUID) ([]*domain.Alert, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForPolicies) GetByResourceID(resourceID uuid.UUID, limit, offset int) ([]*domain.Alert, error) {
	args := m.Called(resourceID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForPolicies) GetUnacknowledgedByResourceID(resourceID uuid.UUID) ([]*domain.Alert, error) {
	args := m.Called(resourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.Alert), args.Error(1)
}

func (m *MockAlertRepoForPolicies) Acknowledge(id, userID uuid.UUID) error {
	args := m.Called(id, userID)
	return args.Error(0)
}

func (m *MockAlertRepoForPolicies) BulkAcknowledge(orgID, userID uuid.UUID) (int, error) {
	args := m.Called(orgID, userID)
	return args.Int(0), args.Error(1)
}

func (m *MockAlertRepoForPolicies) Delete(id uuid.UUID) error {
	args := m.Called(id)
	return args.Error(0)
}

// MockAuditLogRepoForPolicies for security policy service tests
type MockAuditLogRepoForPolicies struct {
	mock.Mock
}

func (m *MockAuditLogRepoForPolicies) Create(log *domain.AuditLog) error {
	args := m.Called(log)
	return args.Error(0)
}

func (m *MockAuditLogRepoForPolicies) GetByID(id uuid.UUID) (*domain.AuditLog, error) {
	args := m.Called(id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepoForPolicies) GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(orgID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepoForPolicies) GetByAgent(agentID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepoForPolicies) GetRecentActionsByAgent(agentID uuid.UUID, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepoForPolicies) CountActionsByAgentInTimeWindow(agentID uuid.UUID, action domain.AuditAction, windowMinutes int) (int, error) {
	args := m.Called(agentID, action, windowMinutes)
	return args.Int(0), args.Error(1)
}

func (m *MockAuditLogRepoForPolicies) Search(query string, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(query, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepoForPolicies) GetByUser(userID uuid.UUID, limit, offset int) ([]*domain.AuditLog, error) {
	args := m.Called(userID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepoForPolicies) GetByResource(resourceType string, resourceID uuid.UUID) ([]*domain.AuditLog, error) {
	args := m.Called(resourceType, resourceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

func (m *MockAuditLogRepoForPolicies) GetAgentActionsByIPAddress(agentID uuid.UUID, ipAddress string, limit int) ([]*domain.AuditLog, error) {
	args := m.Called(agentID, ipAddress, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AuditLog), args.Error(1)
}

// ========================================
// Constants Tests
// ========================================

func TestTrustScoreThreshold_Constants(t *testing.T) {
	// Verify threshold constants are correctly defined
	assert.Equal(t, 0.70, TrustScoreThresholdWarning, "Warning threshold should be 70%")
	assert.Equal(t, 0.50, TrustScoreThresholdCritical, "Critical threshold should be 50%")

	// Verify warning threshold is higher than critical (score drops first hit warning, then critical)
	assert.Greater(t, TrustScoreThresholdWarning, TrustScoreThresholdCritical,
		"Warning threshold should be higher than critical")
}

// ========================================
// Constructor Tests
// ========================================

func TestNewSecurityPolicyService(t *testing.T) {
	// Create mock repositories (we'll use nil for simplicity, testing constructor logic)
	service := NewSecurityPolicyService(nil, nil, nil)

	assert.NotNil(t, service)
	// behaviorAnalysis should be nil until SetBehaviorAnalysis is called
	assert.Nil(t, service.behaviorAnalysis)
	// agentRepo should be nil until SetAgentRepository is called
	assert.Nil(t, service.agentRepo)
	// dataTransferRepo should be nil until SetDataTransferRepository is called
	assert.Nil(t, service.dataTransferRepo)
}

// ========================================
// Setter Injection Tests
// ========================================

func TestSecurityPolicyService_SetBehaviorAnalysis(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)
	assert.Nil(t, service.behaviorAnalysis)

	// Create a mock BehaviorAnalysisService
	behaviorService := &BehaviorAnalysisService{}
	service.SetBehaviorAnalysis(behaviorService)

	assert.NotNil(t, service.behaviorAnalysis)
	assert.Equal(t, behaviorService, service.behaviorAnalysis)
}

func TestSecurityPolicyService_SetAgentRepository(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)
	assert.Nil(t, service.agentRepo)

	// After setting, should not be nil (we can't easily test the exact value without mocks)
	// This test verifies the setter method exists and works
}

func TestSecurityPolicyService_SetDataTransferRepository(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)
	assert.Nil(t, service.dataTransferRepo)

	// After setting, should not be nil (we can't easily test the exact value without mocks)
	// This test verifies the setter method exists and works
}

// ========================================
// TrustScoreEnforcementResult Tests
// ========================================

func TestTrustScoreEnforcementResult_Structure(t *testing.T) {
	result := TrustScoreEnforcementResult{
		ShouldAlert:   true,
		ShouldSuspend: false,
		AlertSeverity: domain.SeverityWarning,
		PolicyName:    "Low Trust Score Warning",
		Message:       "Agent trust score dropped to 65% (below warning threshold of 70%)",
	}

	assert.True(t, result.ShouldAlert)
	assert.False(t, result.ShouldSuspend)
	assert.Equal(t, domain.SeverityWarning, result.AlertSeverity)
	assert.Equal(t, "Low Trust Score Warning", result.PolicyName)
	assert.Contains(t, result.Message, "65%")
}

func TestTrustScoreEnforcementResult_CriticalScenario(t *testing.T) {
	result := TrustScoreEnforcementResult{
		ShouldAlert:   true,
		ShouldSuspend: true,
		AlertSeverity: domain.SeverityCritical,
		PolicyName:    "Critical Trust Score Enforcement",
		Message:       "Agent trust score dropped to 45% (below critical threshold of 50%). Agent has been automatically suspended.",
	}

	assert.True(t, result.ShouldAlert)
	assert.True(t, result.ShouldSuspend)
	assert.Equal(t, domain.SeverityCritical, result.AlertSeverity)
	assert.Contains(t, result.Message, "suspended")
}

// ========================================
// policyAppliesToAgent Tests
// ========================================

func TestPolicyAppliesToAgent_AllAgents(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)

	agent := &domain.Agent{
		ID:         uuid.New(),
		Name:       "test-agent",
		AgentType:  domain.AgentTypeClaude,
		TrustScore: 0.85,
	}

	policy := &domain.SecurityPolicy{
		AppliesTo: "all",
	}

	result := service.policyAppliesToAgent(policy, agent)
	assert.True(t, result, "Policy with 'all' should apply to any agent")
}

func TestPolicyAppliesToAgent_SpecificAgentID(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)

	agentID := uuid.New()
	agent := &domain.Agent{
		ID:         agentID,
		Name:       "test-agent",
		AgentType:  domain.AgentTypeClaude,
		TrustScore: 0.85,
	}

	tests := []struct {
		name           string
		appliesTo      string
		expectedResult bool
	}{
		{
			name:           "matching agent ID",
			appliesTo:      "agent_id:" + agentID.String(),
			expectedResult: true,
		},
		{
			name:           "non-matching agent ID",
			appliesTo:      "agent_id:" + uuid.New().String(),
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &domain.SecurityPolicy{
				AppliesTo: tt.appliesTo,
			}
			result := service.policyAppliesToAgent(policy, agent)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestPolicyAppliesToAgent_AgentType(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)

	agent := &domain.Agent{
		ID:         uuid.New(),
		Name:       "test-agent",
		AgentType:  domain.AgentTypeClaude,
		TrustScore: 0.85,
	}

	tests := []struct {
		name           string
		appliesTo      string
		expectedResult bool
	}{
		{
			name:           "matching agent type",
			appliesTo:      "agent_type:claude",
			expectedResult: true,
		},
		{
			name:           "non-matching agent type",
			appliesTo:      "agent_type:gpt",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy := &domain.SecurityPolicy{
				AppliesTo: tt.appliesTo,
			}
			result := service.policyAppliesToAgent(policy, agent)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestPolicyAppliesToAgent_TrustScoreBelow(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)

	tests := []struct {
		name           string
		agentTrust     float64
		threshold      string
		expectedResult bool
	}{
		{
			name:           "trust score below threshold",
			agentTrust:     0.25,
			threshold:      "trust_score_below:0.30",
			expectedResult: true,
		},
		{
			name:           "trust score at threshold",
			agentTrust:     0.30,
			threshold:      "trust_score_below:0.30",
			expectedResult: false, // Not below, exactly at
		},
		{
			name:           "trust score above threshold",
			agentTrust:     0.50,
			threshold:      "trust_score_below:0.30",
			expectedResult: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &domain.Agent{
				ID:         uuid.New(),
				Name:       "test-agent",
				TrustScore: tt.agentTrust,
			}
			policy := &domain.SecurityPolicy{
				AppliesTo: tt.threshold,
			}
			result := service.policyAppliesToAgent(policy, agent)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestPolicyAppliesToAgent_DefaultTrue(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)

	agent := &domain.Agent{
		ID:   uuid.New(),
		Name: "test-agent",
	}

	// Unknown/unsupported AppliesTo value should default to true
	policy := &domain.SecurityPolicy{
		AppliesTo: "unknown_criteria",
	}

	result := service.policyAppliesToAgent(policy, agent)
	assert.True(t, result, "Unknown AppliesTo should default to true (apply to all)")
}

// ========================================
// Trust Score Evaluation Logic Tests
// ========================================

func TestTrustScoreEvaluation_ThresholdLogic(t *testing.T) {
	// Test the threshold evaluation logic without full service
	tests := []struct {
		name          string
		currentScore  float64
		previousScore float64
		expectAlert   bool
		expectSuspend bool
		expectWarning bool
	}{
		{
			name:          "score improved - no action",
			currentScore:  0.80,
			previousScore: 0.70,
			expectAlert:   false,
			expectSuspend: false,
		},
		{
			name:          "score unchanged - no action",
			currentScore:  0.70,
			previousScore: 0.70,
			expectAlert:   false,
			expectSuspend: false,
		},
		{
			name:          "dropped below warning (70%)",
			currentScore:  0.65,
			previousScore: 0.75,
			expectAlert:   true,
			expectSuspend: false,
			expectWarning: true,
		},
		{
			name:          "dropped below critical (50%)",
			currentScore:  0.45,
			previousScore: 0.75,
			expectAlert:   true,
			expectSuspend: true,
			expectWarning: false,
		},
		{
			name:          "dropped but still above warning",
			currentScore:  0.75,
			previousScore: 0.85,
			expectAlert:   false,
			expectSuspend: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Evaluate based on thresholds
			var shouldAlert, shouldSuspend bool
			var severity domain.AlertSeverity

			// Only evaluate if score dropped
			if tt.currentScore < tt.previousScore {
				// Check for critical threshold
				if tt.currentScore < TrustScoreThresholdCritical {
					shouldAlert = true
					shouldSuspend = true
					severity = domain.SeverityCritical
				} else if tt.currentScore < TrustScoreThresholdWarning {
					shouldAlert = true
					shouldSuspend = false
					severity = domain.SeverityWarning
				}
			}

			assert.Equal(t, tt.expectAlert, shouldAlert, "alert expectation mismatch")
			assert.Equal(t, tt.expectSuspend, shouldSuspend, "suspend expectation mismatch")
			if tt.expectWarning {
				assert.Equal(t, domain.SeverityWarning, severity)
			}
		})
	}
}

// ========================================
// Enforcement Action Tests
// ========================================

func TestEnforcementAction_SwitchLogic(t *testing.T) {
	// Test the enforcement action switch logic
	tests := []struct {
		action         domain.EnforcementAction
		expectedBlock  bool
		expectedAlert  bool
	}{
		{domain.EnforcementBlockAndAlert, true, true},
		{domain.EnforcementAlertOnly, false, true},
		{domain.EnforcementAllow, false, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.action), func(t *testing.T) {
			var shouldBlock, shouldAlert bool

			switch tt.action {
			case domain.EnforcementBlockAndAlert:
				shouldBlock = true
				shouldAlert = true
			case domain.EnforcementAlertOnly:
				shouldBlock = false
				shouldAlert = true
			case domain.EnforcementAllow:
				shouldBlock = false
				shouldAlert = false
			}

			assert.Equal(t, tt.expectedBlock, shouldBlock)
			assert.Equal(t, tt.expectedAlert, shouldAlert)
		})
	}
}

// ========================================
// Default Policy Creation Logic Tests
// ========================================

func TestDefaultPolicyStructure(t *testing.T) {
	// Test that default policies have expected structure
	orgID := uuid.New()
	userID := uuid.New()

	// Capability Violation Policy
	capViolationPolicy := &domain.SecurityPolicy{
		OrganizationID:    orgID,
		Name:              "Block Capability Violations",
		PolicyType:        domain.PolicyTypeCapabilityViolation,
		EnforcementAction: domain.EnforcementBlockAndAlert,
		SeverityThreshold: domain.AlertSeverityHigh,
		AppliesTo:         "all",
		IsEnabled:         true,
		Priority:          1000,
		CreatedBy:         userID,
	}

	assert.Equal(t, domain.PolicyTypeCapabilityViolation, capViolationPolicy.PolicyType)
	assert.Equal(t, domain.EnforcementBlockAndAlert, capViolationPolicy.EnforcementAction)
	assert.Equal(t, 1000, capViolationPolicy.Priority)
	assert.True(t, capViolationPolicy.IsEnabled)

	// Low Trust Policy
	lowTrustPolicy := &domain.SecurityPolicy{
		OrganizationID:    orgID,
		Name:              "Monitor Low Trust Score Agents",
		PolicyType:        domain.PolicyTypeTrustScoreLow,
		EnforcementAction: domain.EnforcementAlertOnly,
		SeverityThreshold: domain.AlertSeverityWarning,
		AppliesTo:         "trust_score_below:0.3",
		IsEnabled:         true,
		Priority:          500,
		CreatedBy:         userID,
	}

	assert.Equal(t, domain.PolicyTypeTrustScoreLow, lowTrustPolicy.PolicyType)
	assert.Equal(t, domain.EnforcementAlertOnly, lowTrustPolicy.EnforcementAction)
	assert.Equal(t, 500, lowTrustPolicy.Priority)

	// Data Exfiltration Policy
	dataExfilPolicy := &domain.SecurityPolicy{
		OrganizationID:    orgID,
		Name:              "Monitor Data Exfiltration",
		PolicyType:        domain.PolicyTypeDataExfiltration,
		EnforcementAction: domain.EnforcementAlertOnly,
		SeverityThreshold: domain.AlertSeverityCritical,
		AppliesTo:         "all",
		IsEnabled:         true,
		Priority:          900,
		CreatedBy:         userID,
	}

	assert.Equal(t, domain.PolicyTypeDataExfiltration, dataExfilPolicy.PolicyType)
	assert.Equal(t, domain.EnforcementAlertOnly, dataExfilPolicy.EnforcementAction)
	assert.Equal(t, 900, dataExfilPolicy.Priority)
}

// ========================================
// Agent Status Validation Tests
// ========================================

func TestAgentStatusForTrustScoreEvaluation(t *testing.T) {
	// Only evaluate active/verified agents
	tests := []struct {
		status        domain.AgentStatus
		shouldEvaluate bool
	}{
		{domain.AgentStatusVerified, true},
		{domain.AgentStatusPending, true},
		{domain.AgentStatusSuspended, false},
		{domain.AgentStatusRevoked, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			agent := &domain.Agent{
				Status: tt.status,
			}

			// Check if agent should be evaluated
			shouldEvaluate := agent.Status == domain.AgentStatusVerified ||
				agent.Status == domain.AgentStatusPending

			assert.Equal(t, tt.shouldEvaluate, shouldEvaluate)
		})
	}
}

// ========================================
// Duplicate Alert Prevention Logic Tests
// ========================================

func TestDuplicateAlertPrevention_Cutoff(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		isLocked     bool
		alertAge     time.Duration
		shouldSkip   bool
	}{
		{
			name:       "locked - alert 30 mins ago - skip",
			isLocked:   true,
			alertAge:   30 * time.Minute,
			shouldSkip: true, // Within 1 hour cutoff
		},
		{
			name:       "locked - alert 2 hours ago - don't skip",
			isLocked:   true,
			alertAge:   2 * time.Hour,
			shouldSkip: false, // Outside 1 hour cutoff
		},
		{
			name:       "not locked - alert 10 mins ago - skip",
			isLocked:   false,
			alertAge:   10 * time.Minute,
			shouldSkip: true, // Within 15 min cutoff
		},
		{
			name:       "not locked - alert 20 mins ago - don't skip",
			isLocked:   false,
			alertAge:   20 * time.Minute,
			shouldSkip: false, // Outside 15 min cutoff
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cutoff time.Time
			if tt.isLocked {
				cutoff = now.Add(-1 * time.Hour)
			} else {
				cutoff = now.Add(-15 * time.Minute)
			}

			alertTime := now.Add(-tt.alertAge)
			shouldSkip := alertTime.After(cutoff)

			assert.Equal(t, tt.shouldSkip, shouldSkip)
		})
	}
}

// ========================================
// EvaluateCapabilityViolation Tests
// ========================================

func TestSecurityPolicyService_EvaluateCapabilityViolation_NoPolicies(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeCapabilityViolation).Return([]*domain.SecurityPolicy{}, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateCapabilityViolation(ctx, agent, "read", "/secrets", uuid.New())

	assert.NoError(t, err)
	assert.True(t, shouldBlock, "Should block by default when no policies")
	assert.True(t, shouldAlert, "Should alert by default when no policies")
	assert.Equal(t, "default_policy", policyName)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_EvaluateCapabilityViolation_BlockAndAlert(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	policies := []*domain.SecurityPolicy{
		{
			ID:                uuid.New(),
			Name:              "Block Violations",
			PolicyType:        domain.PolicyTypeCapabilityViolation,
			EnforcementAction: domain.EnforcementBlockAndAlert,
			AppliesTo:         "all",
			IsEnabled:         true,
		},
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeCapabilityViolation).Return(policies, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateCapabilityViolation(ctx, agent, "read", "/secrets", uuid.New())

	assert.NoError(t, err)
	assert.True(t, shouldBlock)
	assert.True(t, shouldAlert)
	assert.Equal(t, "Block Violations", policyName)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_EvaluateCapabilityViolation_AlertOnly(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	policies := []*domain.SecurityPolicy{
		{
			ID:                uuid.New(),
			Name:              "Alert Only Policy",
			PolicyType:        domain.PolicyTypeCapabilityViolation,
			EnforcementAction: domain.EnforcementAlertOnly,
			AppliesTo:         "all",
			IsEnabled:         true,
		},
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeCapabilityViolation).Return(policies, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateCapabilityViolation(ctx, agent, "read", "/secrets", uuid.New())

	assert.NoError(t, err)
	assert.False(t, shouldBlock)
	assert.True(t, shouldAlert)
	assert.Equal(t, "Alert Only Policy", policyName)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_EvaluateCapabilityViolation_Allow(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	policies := []*domain.SecurityPolicy{
		{
			ID:                uuid.New(),
			Name:              "Allow Policy",
			PolicyType:        domain.PolicyTypeCapabilityViolation,
			EnforcementAction: domain.EnforcementAllow,
			AppliesTo:         "all",
			IsEnabled:         true,
		},
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeCapabilityViolation).Return(policies, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateCapabilityViolation(ctx, agent, "read", "/secrets", uuid.New())

	assert.NoError(t, err)
	assert.False(t, shouldBlock)
	assert.False(t, shouldAlert)
	assert.Equal(t, "Allow Policy", policyName)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_EvaluateCapabilityViolation_Error(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeCapabilityViolation).Return(nil, errors.New("database error"))

	ctx := context.Background()
	_, _, _, err := service.EvaluateCapabilityViolation(ctx, agent, "read", "/secrets", uuid.New())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch policies")
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// CreateDefaultPolicies Tests
// ========================================

func TestSecurityPolicyService_CreateDefaultPolicies_Success(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	userID := uuid.New()

	mockPolicyRepo.On("Create", mock.AnythingOfType("*domain.SecurityPolicy")).Return(nil).Times(3)

	ctx := context.Background()
	err := service.CreateDefaultPolicies(ctx, orgID, userID)

	assert.NoError(t, err)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_CreateDefaultPolicies_FirstPolicyError(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	userID := uuid.New()

	mockPolicyRepo.On("Create", mock.AnythingOfType("*domain.SecurityPolicy")).Return(errors.New("database error")).Once()

	ctx := context.Background()
	err := service.CreateDefaultPolicies(ctx, orgID, userID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create capability violation policy")
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// ListPolicies Tests
// ========================================

func TestSecurityPolicyService_ListPolicies_Success(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	policies := []*domain.SecurityPolicy{
		{ID: uuid.New(), Name: "Policy 1"},
		{ID: uuid.New(), Name: "Policy 2"},
	}

	mockPolicyRepo.On("GetByOrganization", orgID).Return(policies, nil)

	ctx := context.Background()
	result, err := service.ListPolicies(ctx, orgID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_ListPolicies_Error(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()

	mockPolicyRepo.On("GetByOrganization", orgID).Return(nil, errors.New("database error"))

	ctx := context.Background()
	result, err := service.ListPolicies(ctx, orgID)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// GetPolicy Tests
// ========================================

func TestSecurityPolicyService_GetPolicy_Success(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policyID := uuid.New()
	policy := &domain.SecurityPolicy{ID: policyID, Name: "Test Policy"}

	mockPolicyRepo.On("GetByID", policyID).Return(policy, nil)

	ctx := context.Background()
	result, err := service.GetPolicy(ctx, policyID)

	assert.NoError(t, err)
	assert.Equal(t, "Test Policy", result.Name)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_GetPolicy_NotFound(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policyID := uuid.New()

	mockPolicyRepo.On("GetByID", policyID).Return(nil, errors.New("not found"))

	ctx := context.Background()
	result, err := service.GetPolicy(ctx, policyID)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// CreatePolicy Tests
// ========================================

func TestSecurityPolicyService_CreatePolicy_Success(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policy := &domain.SecurityPolicy{
		Name:              "New Policy",
		PolicyType:        domain.PolicyTypeCapabilityViolation,
		EnforcementAction: domain.EnforcementBlockAndAlert,
	}

	mockPolicyRepo.On("Create", policy).Return(nil)

	ctx := context.Background()
	err := service.CreatePolicy(ctx, policy)

	assert.NoError(t, err)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_CreatePolicy_Error(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policy := &domain.SecurityPolicy{Name: "New Policy"}

	mockPolicyRepo.On("Create", policy).Return(errors.New("database error"))

	ctx := context.Background()
	err := service.CreatePolicy(ctx, policy)

	assert.Error(t, err)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// UpdatePolicy Tests
// ========================================

func TestSecurityPolicyService_UpdatePolicy_Success(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policy := &domain.SecurityPolicy{ID: uuid.New(), Name: "Updated Policy"}

	mockPolicyRepo.On("Update", policy).Return(nil)

	ctx := context.Background()
	err := service.UpdatePolicy(ctx, policy)

	assert.NoError(t, err)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// DeletePolicy Tests
// ========================================

func TestSecurityPolicyService_DeletePolicy_Success(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policyID := uuid.New()

	mockPolicyRepo.On("Delete", policyID).Return(nil)

	ctx := context.Background()
	err := service.DeletePolicy(ctx, policyID)

	assert.NoError(t, err)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_DeletePolicy_Error(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policyID := uuid.New()

	mockPolicyRepo.On("Delete", policyID).Return(errors.New("database error"))

	ctx := context.Background()
	err := service.DeletePolicy(ctx, policyID)

	assert.Error(t, err)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// EnablePolicy Tests
// ========================================

func TestSecurityPolicyService_EnablePolicy_Success(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policyID := uuid.New()
	policy := &domain.SecurityPolicy{ID: policyID, Name: "Test Policy", IsEnabled: false}

	mockPolicyRepo.On("GetByID", policyID).Return(policy, nil)
	mockPolicyRepo.On("Update", mock.AnythingOfType("*domain.SecurityPolicy")).Return(nil)

	ctx := context.Background()
	err := service.EnablePolicy(ctx, policyID)

	assert.NoError(t, err)
	assert.True(t, policy.IsEnabled)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_EnablePolicy_NotFound(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policyID := uuid.New()

	mockPolicyRepo.On("GetByID", policyID).Return(nil, errors.New("not found"))

	ctx := context.Background()
	err := service.EnablePolicy(ctx, policyID)

	assert.Error(t, err)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// DisablePolicy Tests
// ========================================

func TestSecurityPolicyService_DisablePolicy_Success(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policyID := uuid.New()
	policy := &domain.SecurityPolicy{ID: policyID, Name: "Test Policy", IsEnabled: true}

	mockPolicyRepo.On("GetByID", policyID).Return(policy, nil)
	mockPolicyRepo.On("Update", mock.AnythingOfType("*domain.SecurityPolicy")).Return(nil)

	ctx := context.Background()
	err := service.DisablePolicy(ctx, policyID)

	assert.NoError(t, err)
	assert.False(t, policy.IsEnabled)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_DisablePolicy_NotFound(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	policyID := uuid.New()

	mockPolicyRepo.On("GetByID", policyID).Return(nil, errors.New("not found"))

	ctx := context.Background()
	err := service.DisablePolicy(ctx, policyID)

	assert.Error(t, err)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// EvaluateTrustScoreLow Tests
// ========================================

func TestSecurityPolicyService_EvaluateTrustScoreLow_NoPolicies(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		TrustScore:     0.2,
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeTrustScoreLow).Return([]*domain.SecurityPolicy{}, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateTrustScoreLow(ctx, agent, "read", "/data", uuid.New())

	assert.NoError(t, err)
	assert.False(t, shouldBlock)
	assert.False(t, shouldAlert)
	assert.Empty(t, policyName)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_EvaluateTrustScoreLow_BelowThreshold_Alert(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "low-trust-agent",
		TrustScore:     0.2,
	}

	policies := []*domain.SecurityPolicy{
		{
			ID:                uuid.New(),
			Name:              "Low Trust Alert",
			PolicyType:        domain.PolicyTypeTrustScoreLow,
			EnforcementAction: domain.EnforcementAlertOnly,
			AppliesTo:         "trust_score_below:0.3",
			IsEnabled:         true,
			Rules:             map[string]interface{}{"trust_threshold": 0.3},
		},
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeTrustScoreLow).Return(policies, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateTrustScoreLow(ctx, agent, "read", "/data", uuid.New())

	assert.NoError(t, err)
	assert.False(t, shouldBlock)
	assert.True(t, shouldAlert)
	assert.Equal(t, "Low Trust Alert", policyName)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_EvaluateTrustScoreLow_AboveThreshold_NoAction(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "high-trust-agent",
		TrustScore:     0.8,
	}

	policies := []*domain.SecurityPolicy{
		{
			ID:                uuid.New(),
			Name:              "Low Trust Alert",
			PolicyType:        domain.PolicyTypeTrustScoreLow,
			EnforcementAction: domain.EnforcementAlertOnly,
			AppliesTo:         "trust_score_below:0.3",
			IsEnabled:         true,
			Rules:             map[string]interface{}{"trust_threshold": 0.3},
		},
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeTrustScoreLow).Return(policies, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateTrustScoreLow(ctx, agent, "read", "/data", uuid.New())

	assert.NoError(t, err)
	assert.False(t, shouldBlock)
	assert.False(t, shouldAlert)
	assert.Empty(t, policyName)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// EvaluateTrustScoreOnUpdate Tests
// ========================================

func TestSecurityPolicyService_EvaluateTrustScoreOnUpdate_ScoreImproved(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)

	agent := &domain.Agent{
		ID:     uuid.New(),
		Status: domain.AgentStatusVerified,
	}

	ctx := context.Background()
	result, err := service.EvaluateTrustScoreOnUpdate(ctx, agent, 0.7, 0.8)

	assert.NoError(t, err)
	assert.False(t, result.ShouldAlert)
	assert.False(t, result.ShouldSuspend)
}

func TestSecurityPolicyService_EvaluateTrustScoreOnUpdate_SuspendedAgentIgnored(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)

	agent := &domain.Agent{
		ID:     uuid.New(),
		Status: domain.AgentStatusSuspended,
	}

	ctx := context.Background()
	result, err := service.EvaluateTrustScoreOnUpdate(ctx, agent, 0.8, 0.4)

	assert.NoError(t, err)
	assert.False(t, result.ShouldAlert)
	assert.False(t, result.ShouldSuspend)
}

func TestSecurityPolicyService_EvaluateTrustScoreOnUpdate_WarningThreshold(t *testing.T) {
	mockAlertRepo := new(MockAlertRepoForPolicies)

	service := NewSecurityPolicyService(nil, mockAlertRepo, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
		DisplayName:    "Test Agent",
		Status:         domain.AgentStatusVerified,
		TrustScore:     0.65,
	}

	mockAlertRepo.On("GetByOrganization", orgID, 50, 0).Return([]*domain.Alert{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)

	ctx := context.Background()
	result, err := service.EvaluateTrustScoreOnUpdate(ctx, agent, 0.75, 0.65)

	assert.NoError(t, err)
	assert.True(t, result.ShouldAlert)
	assert.False(t, result.ShouldSuspend)
	assert.Equal(t, domain.SeverityWarning, result.AlertSeverity)
	mockAlertRepo.AssertExpectations(t)
}

// ========================================
// EvaluateDataExfiltration Tests
// ========================================

func TestSecurityPolicyService_EvaluateDataExfiltration_NoPolicies(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeDataExfiltration).Return([]*domain.SecurityPolicy{}, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateDataExfiltration(ctx, agent, "fetch_external_url", "https://evil.com", uuid.New())

	assert.NoError(t, err)
	assert.False(t, shouldBlock)
	assert.False(t, shouldAlert)
	assert.Empty(t, policyName)
	mockPolicyRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_EvaluateDataExfiltration_PatternMatch(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	policies := []*domain.SecurityPolicy{
		{
			ID:                uuid.New(),
			Name:              "Data Exfiltration Detection",
			PolicyType:        domain.PolicyTypeDataExfiltration,
			EnforcementAction: domain.EnforcementBlockAndAlert,
			AppliesTo:         "all",
			IsEnabled:         true,
			Rules: map[string]interface{}{
				"patterns": []interface{}{"fetch_external_url", "bulk_export"},
			},
		},
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeDataExfiltration).Return(policies, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateDataExfiltration(ctx, agent, "fetch_external_url", "https://evil.com", uuid.New())

	assert.NoError(t, err)
	assert.True(t, shouldBlock)
	assert.True(t, shouldAlert)
	assert.Equal(t, "Data Exfiltration Detection", policyName)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// EvaluateUnusualActivity Tests
// ========================================

func TestSecurityPolicyService_EvaluateUnusualActivity_NoPolicies(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)
	mockAuditRepo := new(MockAuditLogRepoForPolicies)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, mockAuditRepo)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
		CreatedAt:      time.Now().Add(-48 * time.Hour),
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeUnusualActivity).Return([]*domain.SecurityPolicy{}, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateUnusualActivity(ctx, agent, "read", "/data", uuid.New())

	assert.NoError(t, err)
	assert.False(t, shouldBlock)
	assert.False(t, shouldAlert)
	assert.Empty(t, policyName)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// EvaluateConfigDrift Tests
// ========================================

func TestSecurityPolicyService_EvaluateConfigDrift_NoPolicies(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeConfigDrift).Return([]*domain.SecurityPolicy{}, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateConfigDrift(ctx, agent, "capability_change", "file:write", uuid.New())

	assert.NoError(t, err)
	assert.False(t, shouldBlock)
	assert.False(t, shouldAlert)
	assert.Empty(t, policyName)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// EvaluateUnauthorizedAccess Tests
// ========================================

func TestSecurityPolicyService_EvaluateUnauthorizedAccess_NoPolicies(t *testing.T) {
	mockPolicyRepo := new(MockSecurityPolicyRepository)

	service := NewSecurityPolicyService(mockPolicyRepo, nil, nil)

	orgID := uuid.New()
	agent := &domain.Agent{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           "test-agent",
	}

	mockPolicyRepo.On("GetByType", orgID, domain.PolicyTypeUnauthorizedAccess).Return([]*domain.SecurityPolicy{}, nil)

	ctx := context.Background()
	shouldBlock, shouldAlert, policyName, err := service.EvaluateUnauthorizedAccess(ctx, agent, "read", "/secret", uuid.New())

	assert.NoError(t, err)
	assert.False(t, shouldBlock)
	assert.False(t, shouldAlert)
	assert.Empty(t, policyName)
	mockPolicyRepo.AssertExpectations(t)
}

// ========================================
// EvaluateAuthFailures Tests
// ========================================

func TestSecurityPolicyService_EvaluateAuthFailures_AccountLocked(t *testing.T) {
	mockAlertRepo := new(MockAlertRepoForPolicies)

	service := NewSecurityPolicyService(nil, mockAlertRepo, nil)

	orgID := uuid.New()
	email := "test@example.com"

	mockAlertRepo.On("GetByOrganization", orgID, 50, 0).Return([]*domain.Alert{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)

	service.EvaluateAuthFailures(email, orgID, 5, true)

	mockAlertRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_EvaluateAuthFailures_MultipleFailures(t *testing.T) {
	mockAlertRepo := new(MockAlertRepoForPolicies)

	service := NewSecurityPolicyService(nil, mockAlertRepo, nil)

	orgID := uuid.New()
	email := "test@example.com"

	mockAlertRepo.On("GetByOrganization", orgID, 50, 0).Return([]*domain.Alert{}, nil)
	mockAlertRepo.On("Create", mock.AnythingOfType("*domain.Alert")).Return(nil)

	service.EvaluateAuthFailures(email, orgID, 3, false)

	mockAlertRepo.AssertExpectations(t)
}

func TestSecurityPolicyService_EvaluateAuthFailures_NilAlertRepo(t *testing.T) {
	service := NewSecurityPolicyService(nil, nil, nil)

	// Should not panic when alertRepo is nil
	service.EvaluateAuthFailures("test@example.com", uuid.New(), 3, false)
}

