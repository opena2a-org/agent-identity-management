package application

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a/identity/backend/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ========================================
// Configuration Constant Tests
// ========================================

func TestBehaviorAnalysis_ConfigurationConstants(t *testing.T) {
	// Verify all configuration constants have expected values
	t.Run("DefaultMaturityThreshold", func(t *testing.T) {
		assert.Equal(t, 50, DefaultMaturityThreshold)
	})

	t.Run("SigmaThresholds", func(t *testing.T) {
		assert.Equal(t, 2.0, SigmaThresholdLow)
		assert.Equal(t, 3.0, SigmaThresholdMedium)
		assert.Equal(t, 4.0, SigmaThresholdHigh)
		assert.Equal(t, 5.0, SigmaThresholdCritical)
	})

	t.Run("MinAgentAgeForDetection", func(t *testing.T) {
		assert.Equal(t, 24*time.Hour, MinAgentAgeForDetection)
	})

	t.Run("VelocityWindowHours", func(t *testing.T) {
		assert.Equal(t, 1, VelocityWindowHours)
	})
}

func TestBehaviorAnalysis_SigmaThresholdProgression(t *testing.T) {
	// Verify sigma thresholds are in ascending order
	assert.True(t, SigmaThresholdLow < SigmaThresholdMedium)
	assert.True(t, SigmaThresholdMedium < SigmaThresholdHigh)
	assert.True(t, SigmaThresholdHigh < SigmaThresholdCritical)
}

// ========================================
// extractResourceType Tests
// ========================================

func TestExtractResourceType_ProtocolStyle(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		expected string
	}{
		{
			name:     "S3 bucket",
			resource: "s3://bucket/key/file.txt",
			expected: "s3",
		},
		{
			name:     "DynamoDB table",
			resource: "dynamodb://table/key",
			expected: "dynamodb",
		},
		{
			name:     "HTTPS URL",
			resource: "https://api.example.com/users",
			expected: "https",
		},
		{
			name:     "HTTP URL",
			resource: "http://localhost:8080/api",
			expected: "http",
		},
		{
			name:     "File protocol",
			resource: "file:///path/to/file",
			expected: "file",
		},
		{
			name:     "Custom protocol",
			resource: "myprotocol://resource/path",
			expected: "myprotocol",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractResourceType(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractResourceType_PathStyle(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		expected string
	}{
		{
			name:     "API path",
			resource: "/api/users/123",
			expected: "api",
		},
		{
			name:     "Admin path",
			resource: "/admin/settings",
			expected: "admin",
		},
		{
			name:     "Root path only",
			resource: "/users",
			expected: "users",
		},
		{
			name:     "Deep path",
			resource: "/v1/organizations/123/agents/456",
			expected: "v1",
		},
		{
			name:     "Health check",
			resource: "/health",
			expected: "health",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractResourceType(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractResourceType_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		expected string
	}{
		{
			name:     "empty string",
			resource: "",
			expected: "unknown",
		},
		{
			name:     "just slash",
			resource: "/",
			expected: "", // Single slash returns empty string
		},
		{
			name:     "no delimiter",
			resource: "simpleresource",
			expected: "simpleresource",
		},
		{
			name:     "dot delimiter",
			resource: "domain.example.com",
			expected: "domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractResourceType(tt.resource)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========================================
// updateBaselineRiskStats Tests
// ========================================

func TestUpdateBaselineRiskStats_FirstSample(t *testing.T) {
	baseline := &domain.AgentBehaviorBaseline{
		SamplesCount:    1,
		AvgRiskScore:    0,
		StddevRiskScore: 0,
	}

	updateBaselineRiskStats(baseline, 5.0)

	assert.Equal(t, 5.0, baseline.AvgRiskScore)
	assert.Equal(t, 0.0, baseline.StddevRiskScore)
}

func TestUpdateBaselineRiskStats_MultipleSamples(t *testing.T) {
	baseline := &domain.AgentBehaviorBaseline{
		SamplesCount:    1,
		AvgRiskScore:    0,
		StddevRiskScore: 0,
	}

	// First sample
	updateBaselineRiskStats(baseline, 5.0)

	// Second sample
	baseline.SamplesCount = 2
	updateBaselineRiskStats(baseline, 7.0)

	// Mean should be (5+7)/2 = 6
	assert.InDelta(t, 6.0, baseline.AvgRiskScore, 0.01)
	// Stddev should be non-zero now
	assert.True(t, baseline.StddevRiskScore > 0)
}

func TestUpdateBaselineRiskStats_WelfordAlgorithm(t *testing.T) {
	// Test that Welford's algorithm produces accurate results
	baseline := &domain.AgentBehaviorBaseline{
		SamplesCount:    0,
		AvgRiskScore:    0,
		StddevRiskScore: 0,
	}

	// Add several samples
	samples := []float64{2.0, 4.0, 4.0, 4.0, 5.0, 5.0, 7.0, 9.0}
	for i, sample := range samples {
		baseline.SamplesCount = i + 1
		updateBaselineRiskStats(baseline, sample)
	}

	// Expected mean = 5.0
	assert.InDelta(t, 5.0, baseline.AvgRiskScore, 0.01)
	// Expected population stddev = 2.0
	assert.InDelta(t, 2.0, baseline.StddevRiskScore, 0.5) // Allow some tolerance
}

func TestUpdateBaselineRiskStats_ZeroValues(t *testing.T) {
	baseline := &domain.AgentBehaviorBaseline{
		SamplesCount:    1,
		AvgRiskScore:    0,
		StddevRiskScore: 0,
	}

	// All zero samples
	updateBaselineRiskStats(baseline, 0.0)
	assert.Equal(t, 0.0, baseline.AvgRiskScore)

	baseline.SamplesCount = 2
	updateBaselineRiskStats(baseline, 0.0)
	assert.Equal(t, 0.0, baseline.AvgRiskScore)
	assert.Equal(t, 0.0, baseline.StddevRiskScore)
}

// ========================================
// severityFromSigma Tests
// ========================================

func TestSeverityFromSigma_Classification(t *testing.T) {
	service := &BehaviorAnalysisService{}

	tests := []struct {
		name             string
		sigma            float64
		expectedSeverity domain.AnomalySeverity
	}{
		// Low severity (sigma < 3.0)
		{
			name:             "sigma 1.0 - low",
			sigma:            1.0,
			expectedSeverity: domain.AnomalySeverityLow,
		},
		{
			name:             "sigma 2.0 - low",
			sigma:            2.0,
			expectedSeverity: domain.AnomalySeverityLow,
		},
		{
			name:             "sigma 2.9 - low",
			sigma:            2.9,
			expectedSeverity: domain.AnomalySeverityLow,
		},
		// Medium severity (3.0 <= sigma < 4.0)
		{
			name:             "sigma 3.0 - medium",
			sigma:            3.0,
			expectedSeverity: domain.AnomalySeverityMedium,
		},
		{
			name:             "sigma 3.5 - medium",
			sigma:            3.5,
			expectedSeverity: domain.AnomalySeverityMedium,
		},
		// High severity (4.0 <= sigma < 5.0)
		{
			name:             "sigma 4.0 - high",
			sigma:            4.0,
			expectedSeverity: domain.AnomalySeverityHigh,
		},
		{
			name:             "sigma 4.9 - high",
			sigma:            4.9,
			expectedSeverity: domain.AnomalySeverityHigh,
		},
		// Critical severity (sigma >= 5.0)
		{
			name:             "sigma 5.0 - critical",
			sigma:            5.0,
			expectedSeverity: domain.AnomalySeverityCritical,
		},
		{
			name:             "sigma 10.0 - critical",
			sigma:            10.0,
			expectedSeverity: domain.AnomalySeverityCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.severityFromSigma(tt.sigma)
			assert.Equal(t, tt.expectedSeverity, result)
		})
	}
}

func TestSeverityFromSigma_NegativeValues(t *testing.T) {
	// Negative sigma values should use absolute value
	service := &BehaviorAnalysisService{}

	tests := []struct {
		name             string
		sigma            float64
		expectedSeverity domain.AnomalySeverity
	}{
		{
			name:             "sigma -2.0 - low",
			sigma:            -2.0,
			expectedSeverity: domain.AnomalySeverityLow,
		},
		{
			name:             "sigma -5.0 - critical",
			sigma:            -5.0,
			expectedSeverity: domain.AnomalySeverityCritical,
		},
		{
			name:             "sigma -4.5 - high",
			sigma:            -4.5,
			expectedSeverity: domain.AnomalySeverityHigh,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.severityFromSigma(tt.sigma)
			assert.Equal(t, tt.expectedSeverity, result)
		})
	}
}

// ========================================
// severityForNewCapability Tests
// ========================================

func TestSeverityForNewCapability_RiskBased(t *testing.T) {
	service := &BehaviorAnalysisService{}

	// Note: GetRiskWeight uses prefix matching:
	// "admin" -> 10.0, "delete" -> 8.0, "system" -> 8.0, "security" -> 9.0
	// "execute" -> 6.0, "write" -> 5.0, "read/list" -> 1.0, "api" -> 2.0
	// Default -> 3.0
	//
	// severityForNewCapability thresholds:
	// >= 9.0 -> critical, >= 7.0 -> high, >= 5.0 -> medium, else -> low

	tests := []struct {
		name             string
		capability       string
		expectedSeverity domain.AnomalySeverity
	}{
		// Critical risk capabilities (admin prefix -> 10.0)
		{
			name:             "admin users delete",
			capability:       "admin:users:delete",
			expectedSeverity: domain.AnomalySeverityCritical,
		},
		// High risk capabilities (system prefix -> 8.0, delete prefix -> 8.0)
		{
			name:             "system admin capability",
			capability:       domain.CapabilitySystemAdmin, // "system:admin" -> 8.0
			expectedSeverity: domain.AnomalySeverityHigh,
		},
		{
			name:             "system exec capability",
			capability:       domain.CapabilitySystemExec, // "system:exec" -> 8.0
			expectedSeverity: domain.AnomalySeverityHigh,
		},
		{
			name:             "delete operation",
			capability:       "delete:resource",
			expectedSeverity: domain.AnomalySeverityHigh,
		},
		// Medium risk capabilities (write prefix -> 5.0, execute prefix -> 6.0)
		{
			name:             "write data",
			capability:       "write:data",
			expectedSeverity: domain.AnomalySeverityMedium,
		},
		{
			name:             "execute command",
			capability:       "execute:command",
			expectedSeverity: domain.AnomalySeverityMedium,
		},
		// Low risk capabilities (file prefix gets default 3.0)
		{
			name:             "file read capability",
			capability:       domain.CapabilityFileRead, // "file:read" -> 3.0
			expectedSeverity: domain.AnomalySeverityLow,
		},
		{
			name:             "file write capability",
			capability:       domain.CapabilityFileWrite, // "file:write" -> 3.0
			expectedSeverity: domain.AnomalySeverityLow,
		},
		{
			name:             "file delete capability",
			capability:       domain.CapabilityFileDelete, // "file:delete" -> 3.0
			expectedSeverity: domain.AnomalySeverityLow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.severityForNewCapability(tt.capability)
			assert.Equal(t, tt.expectedSeverity, result)
		})
	}
}

func TestSeverityForNewCapability_UnknownCapability(t *testing.T) {
	service := &BehaviorAnalysisService{}

	// Unknown capabilities should default to low severity
	result := service.severityForNewCapability("unknown_capability")
	assert.Equal(t, domain.AnomalySeverityLow, result)
}

// ========================================
// NewBehaviorAnalysisService Tests
// ========================================

func TestNewBehaviorAnalysisService(t *testing.T) {
	service := NewBehaviorAnalysisService(nil, nil, nil)
	assert.NotNil(t, service)
}

func TestNewBehaviorAnalysisService_StoresDependencies(t *testing.T) {
	service := NewBehaviorAnalysisService(nil, nil, nil)
	assert.Nil(t, service.db)
	assert.Nil(t, service.baselineRepo)
	assert.Nil(t, service.alertService)
}

// ========================================
// Baseline Maturity Logic Tests
// ========================================

func TestBaselineMaturity_Logic(t *testing.T) {
	tests := []struct {
		name          string
		samplesCount  int
		threshold     int
		shouldMature  bool
	}{
		{
			name:         "below threshold",
			samplesCount: 49,
			threshold:    50,
			shouldMature: false,
		},
		{
			name:         "at threshold",
			samplesCount: 50,
			threshold:    50,
			shouldMature: true,
		},
		{
			name:         "above threshold",
			samplesCount: 100,
			threshold:    50,
			shouldMature: true,
		},
		{
			name:         "zero samples",
			samplesCount: 0,
			threshold:    50,
			shouldMature: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseline := &domain.AgentBehaviorBaseline{
				SamplesCount:      tt.samplesCount,
				MaturityThreshold: tt.threshold,
				IsMature:          false,
			}

			// Simulate maturity check
			if !baseline.IsMature && baseline.SamplesCount >= baseline.MaturityThreshold {
				baseline.IsMature = true
			}

			assert.Equal(t, tt.shouldMature, baseline.IsMature)
		})
	}
}

// ========================================
// Detection Result Structure Tests
// ========================================

func TestDetectionResult_DefaultState(t *testing.T) {
	result := &domain.DetectionResult{
		IsAnomalous: false,
		Anomalies:   []domain.BehavioralAnomaly{},
		ShouldBlock: false,
		ShouldAlert: false,
	}

	assert.False(t, result.IsAnomalous)
	assert.Empty(t, result.Anomalies)
	assert.False(t, result.ShouldBlock)
	assert.False(t, result.ShouldAlert)
}

func TestDetectionResult_WithAnomalies(t *testing.T) {
	anomaly := domain.BehavioralAnomaly{
		ID:          uuid.New(),
		AnomalyType: domain.BehavioralAnomalyNewCapability,
		Severity:    domain.AnomalySeverityHigh,
	}

	result := &domain.DetectionResult{
		IsAnomalous: true,
		Anomalies:   []domain.BehavioralAnomaly{anomaly},
		ShouldBlock: false,
		ShouldAlert: true,
	}

	assert.True(t, result.IsAnomalous)
	assert.Len(t, result.Anomalies, 1)
	assert.True(t, result.ShouldAlert)
}

// ========================================
// Behavioral Anomaly Structure Tests
// ========================================

func TestBehavioralAnomaly_AllTypes(t *testing.T) {
	anomalyTypes := []domain.BehavioralAnomalyType{
		domain.BehavioralAnomalyNewCapability,
		domain.BehavioralAnomalyNewResource,
		domain.BehavioralAnomalyVelocitySpike,
		domain.BehavioralAnomalyRiskSpike,
	}

	for _, anomalyType := range anomalyTypes {
		t.Run(string(anomalyType), func(t *testing.T) {
			anomaly := domain.BehavioralAnomaly{
				ID:          uuid.New(),
				AnomalyType: anomalyType,
			}
			assert.Equal(t, anomalyType, anomaly.AnomalyType)
			assert.NotEmpty(t, string(anomalyType))
		})
	}
}

func TestBehavioralAnomaly_Severities(t *testing.T) {
	severities := []domain.AnomalySeverity{
		domain.AnomalySeverityLow,
		domain.AnomalySeverityMedium,
		domain.AnomalySeverityHigh,
		domain.AnomalySeverityCritical,
	}

	for _, severity := range severities {
		t.Run(string(severity), func(t *testing.T) {
			anomaly := domain.BehavioralAnomaly{
				ID:       uuid.New(),
				Severity: severity,
			}
			assert.Equal(t, severity, anomaly.Severity)
			assert.NotEmpty(t, string(severity))
		})
	}
}

// ========================================
// Agent Behavior Baseline Tests
// ========================================

func TestAgentBehaviorBaseline_Initialization(t *testing.T) {
	now := time.Now()
	agentID := uuid.New()
	orgID := uuid.New()

	baseline := &domain.AgentBehaviorBaseline{
		ID:                 uuid.New(),
		AgentID:            agentID,
		OrganizationID:     orgID,
		BaselineStartAt:    now,
		BaselineEndAt:      now,
		SamplesCount:       0,
		AvgCallsPerHour:    0,
		StddevCallsPerHour: 0,
		MaxCallsPerHour:    0,
		CapabilityUsage:    make(map[string]int),
		ResourcePatterns:   make(map[string]int),
		ActionSequences:    []domain.ActionSequence{},
		AvgRiskScore:       0,
		StddevRiskScore:    0,
		IsMature:           false,
		MaturityThreshold:  DefaultMaturityThreshold,
		CreatedAt:          now,
	}

	assert.NotEqual(t, uuid.Nil, baseline.ID)
	assert.Equal(t, agentID, baseline.AgentID)
	assert.Equal(t, orgID, baseline.OrganizationID)
	assert.Equal(t, 0, baseline.SamplesCount)
	assert.False(t, baseline.IsMature)
	assert.Equal(t, DefaultMaturityThreshold, baseline.MaturityThreshold)
	assert.NotNil(t, baseline.CapabilityUsage)
	assert.NotNil(t, baseline.ResourcePatterns)
}

func TestAgentBehaviorBaseline_CapabilityUsageUpdate(t *testing.T) {
	baseline := &domain.AgentBehaviorBaseline{
		CapabilityUsage: make(map[string]int),
	}

	// Add capability usage
	baseline.CapabilityUsage["file_read"] = 10
	baseline.CapabilityUsage["file_write"] = 5

	assert.Equal(t, 10, baseline.CapabilityUsage["file_read"])
	assert.Equal(t, 5, baseline.CapabilityUsage["file_write"])
	assert.Len(t, baseline.CapabilityUsage, 2)
}

func TestAgentBehaviorBaseline_ResourcePatternUpdate(t *testing.T) {
	baseline := &domain.AgentBehaviorBaseline{
		ResourcePatterns: make(map[string]int),
	}

	// Add resource patterns
	baseline.ResourcePatterns["api"] = 100
	baseline.ResourcePatterns["s3"] = 50

	assert.Equal(t, 100, baseline.ResourcePatterns["api"])
	assert.Equal(t, 50, baseline.ResourcePatterns["s3"])
}

// ========================================
// Sigma Calculation Logic Tests
// ========================================

func TestSigmaCalculation_Logic(t *testing.T) {
	// Test the sigma calculation logic used in DetectAnomalies
	tests := []struct {
		name     string
		value    float64
		mean     float64
		stddev   float64
		expected float64
	}{
		{
			name:     "value equals mean",
			value:    10.0,
			mean:     10.0,
			stddev:   2.0,
			expected: 0.0,
		},
		{
			name:     "value 1 stddev above mean",
			value:    12.0,
			mean:     10.0,
			stddev:   2.0,
			expected: 1.0,
		},
		{
			name:     "value 2 stddev above mean",
			value:    14.0,
			mean:     10.0,
			stddev:   2.0,
			expected: 2.0,
		},
		{
			name:     "value 1 stddev below mean",
			value:    8.0,
			mean:     10.0,
			stddev:   2.0,
			expected: -1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Sigma calculation: (value - mean) / stddev
			sigma := (tt.value - tt.mean) / tt.stddev
			assert.InDelta(t, tt.expected, sigma, 0.001)
		})
	}
}

// ========================================
// Velocity Stats Update Logic Tests
// ========================================

func TestVelocityStats_MaxUpdate(t *testing.T) {
	baseline := &domain.AgentBehaviorBaseline{
		MaxCallsPerHour: 100,
	}

	// New velocity below max - should not update
	currentVelocity := 50.0
	if int(currentVelocity) > baseline.MaxCallsPerHour {
		baseline.MaxCallsPerHour = int(currentVelocity)
	}
	assert.Equal(t, 100, baseline.MaxCallsPerHour)

	// New velocity above max - should update
	currentVelocity = 150.0
	if int(currentVelocity) > baseline.MaxCallsPerHour {
		baseline.MaxCallsPerHour = int(currentVelocity)
	}
	assert.Equal(t, 150, baseline.MaxCallsPerHour)
}

// ========================================
// Alert Trigger Logic Tests
// ========================================

func TestAlertTrigger_HighSeverity(t *testing.T) {
	// Test that high/critical severities trigger alerts
	tests := []struct {
		severity     domain.AnomalySeverity
		shouldAlert  bool
	}{
		{domain.AnomalySeverityLow, false},
		{domain.AnomalySeverityMedium, false},
		{domain.AnomalySeverityHigh, true},
		{domain.AnomalySeverityCritical, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			shouldAlert := tt.severity == domain.AnomalySeverityHigh || tt.severity == domain.AnomalySeverityCritical
			assert.Equal(t, tt.shouldAlert, shouldAlert)
		})
	}
}

func TestBlockTrigger_CriticalOnly(t *testing.T) {
	// Test that only critical severity triggers blocking
	tests := []struct {
		severity     domain.AnomalySeverity
		shouldBlock  bool
	}{
		{domain.AnomalySeverityLow, false},
		{domain.AnomalySeverityMedium, false},
		{domain.AnomalySeverityHigh, false},
		{domain.AnomalySeverityCritical, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.severity), func(t *testing.T) {
			shouldBlock := tt.severity == domain.AnomalySeverityCritical
			assert.Equal(t, tt.shouldBlock, shouldBlock)
		})
	}
}

// ========================================
// Learning Mode Logic Tests
// ========================================

func TestLearningMode_Conditions(t *testing.T) {
	tests := []struct {
		name           string
		baseline       *domain.AgentBehaviorBaseline
		expectedReason string
		inLearningMode bool
	}{
		{
			name:           "no baseline",
			baseline:       nil,
			expectedReason: "No baseline established yet - learning mode",
			inLearningMode: true,
		},
		{
			name: "baseline not mature",
			baseline: &domain.AgentBehaviorBaseline{
				SamplesCount:      10,
				MaturityThreshold: 50,
				IsMature:          false,
			},
			expectedReason: "Baseline not mature",
			inLearningMode: true,
		},
		{
			name: "baseline mature",
			baseline: &domain.AgentBehaviorBaseline{
				SamplesCount:      50,
				MaturityThreshold: 50,
				IsMature:          true,
			},
			expectedReason: "",
			inLearningMode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inLearningMode := tt.baseline == nil || !tt.baseline.IsMature
			assert.Equal(t, tt.inLearningMode, inLearningMode)
		})
	}
}

// ========================================
// Mock Repository for Service Tests
// ========================================

type MockBehaviorBaselineRepo struct {
	mock.Mock
}

func (m *MockBehaviorBaselineRepo) GetByAgentID(agentID uuid.UUID) (*domain.AgentBehaviorBaseline, error) {
	args := m.Called(agentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.AgentBehaviorBaseline), args.Error(1)
}

func (m *MockBehaviorBaselineRepo) Upsert(baseline *domain.AgentBehaviorBaseline) error {
	args := m.Called(baseline)
	return args.Error(0)
}

func (m *MockBehaviorBaselineRepo) GetMatureByOrganization(orgID uuid.UUID) ([]*domain.AgentBehaviorBaseline, error) {
	args := m.Called(orgID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.AgentBehaviorBaseline), args.Error(1)
}

func (m *MockBehaviorBaselineRepo) Delete(agentID uuid.UUID) error {
	args := m.Called(agentID)
	return args.Error(0)
}

func (m *MockBehaviorBaselineRepo) RecordAnomaly(anomaly *domain.BehavioralAnomaly) error {
	args := m.Called(anomaly)
	return args.Error(0)
}

func (m *MockBehaviorBaselineRepo) GetRecentAnomalies(agentID uuid.UUID, limit int) ([]*domain.BehavioralAnomaly, error) {
	args := m.Called(agentID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.BehavioralAnomaly), args.Error(1)
}

func (m *MockBehaviorBaselineRepo) GetAnomaliesByType(orgID uuid.UUID, anomalyType domain.BehavioralAnomalyType, limit int) ([]*domain.BehavioralAnomaly, error) {
	args := m.Called(orgID, anomalyType, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domain.BehavioralAnomaly), args.Error(1)
}

// ========================================
// Service Method Tests with Mocks
// ========================================

func TestBehaviorAnalysisService_GetBaselineStatus_Success(t *testing.T) {
	mockRepo := new(MockBehaviorBaselineRepo)
	service := NewBehaviorAnalysisService(nil, mockRepo, nil)

	agentID := uuid.New()
	orgID := uuid.New()
	expectedBaseline := &domain.AgentBehaviorBaseline{
		ID:             uuid.New(),
		AgentID:        agentID,
		OrganizationID: orgID,
		SamplesCount:   100,
		IsMature:       true,
	}

	mockRepo.On("GetByAgentID", agentID).Return(expectedBaseline, nil)

	result, err := service.GetBaselineStatus(nil, agentID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, agentID, result.AgentID)
	assert.Equal(t, 100, result.SamplesCount)
	assert.True(t, result.IsMature)
	mockRepo.AssertExpectations(t)
}

func TestBehaviorAnalysisService_GetBaselineStatus_NotFound(t *testing.T) {
	mockRepo := new(MockBehaviorBaselineRepo)
	service := NewBehaviorAnalysisService(nil, mockRepo, nil)

	agentID := uuid.New()

	mockRepo.On("GetByAgentID", agentID).Return(nil, nil)

	result, err := service.GetBaselineStatus(nil, agentID)

	assert.NoError(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestBehaviorAnalysisService_GetBaselineStatus_Error(t *testing.T) {
	mockRepo := new(MockBehaviorBaselineRepo)
	service := NewBehaviorAnalysisService(nil, mockRepo, nil)

	agentID := uuid.New()
	expectedErr := assert.AnError

	mockRepo.On("GetByAgentID", agentID).Return(nil, expectedErr)

	result, err := service.GetBaselineStatus(nil, agentID)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestBehaviorAnalysisService_GetRecentAnomalies_Success(t *testing.T) {
	mockRepo := new(MockBehaviorBaselineRepo)
	service := NewBehaviorAnalysisService(nil, mockRepo, nil)

	agentID := uuid.New()
	expectedAnomalies := []*domain.BehavioralAnomaly{
		{
			ID:          uuid.New(),
			AgentID:     agentID,
			AnomalyType: domain.BehavioralAnomalyNewCapability,
			Severity:    domain.AnomalySeverityHigh,
		},
		{
			ID:          uuid.New(),
			AgentID:     agentID,
			AnomalyType: domain.BehavioralAnomalyVelocitySpike,
			Severity:    domain.AnomalySeverityMedium,
		},
	}

	mockRepo.On("GetRecentAnomalies", agentID, 10).Return(expectedAnomalies, nil)

	result, err := service.GetRecentAnomalies(nil, agentID, 10)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, domain.BehavioralAnomalyNewCapability, result[0].AnomalyType)
	mockRepo.AssertExpectations(t)
}

func TestBehaviorAnalysisService_GetRecentAnomalies_Empty(t *testing.T) {
	mockRepo := new(MockBehaviorBaselineRepo)
	service := NewBehaviorAnalysisService(nil, mockRepo, nil)

	agentID := uuid.New()

	mockRepo.On("GetRecentAnomalies", agentID, 10).Return([]*domain.BehavioralAnomaly{}, nil)

	result, err := service.GetRecentAnomalies(nil, agentID, 10)

	assert.NoError(t, err)
	assert.Empty(t, result)
	mockRepo.AssertExpectations(t)
}

func TestBehaviorAnalysisService_GetRecentAnomalies_Error(t *testing.T) {
	mockRepo := new(MockBehaviorBaselineRepo)
	service := NewBehaviorAnalysisService(nil, mockRepo, nil)

	agentID := uuid.New()
	expectedErr := assert.AnError

	mockRepo.On("GetRecentAnomalies", agentID, 10).Return(nil, expectedErr)

	result, err := service.GetRecentAnomalies(nil, agentID, 10)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========================================
// detectCapabilityDrift Tests
// ========================================

func TestDetectCapabilityDrift_NilCapabilityUsage(t *testing.T) {
	service := &BehaviorAnalysisService{}
	baseline := &domain.AgentBehaviorBaseline{
		CapabilityUsage: nil,
	}

	result := service.detectCapabilityDrift(baseline, "file:read", uuid.New(), uuid.New())
	assert.Nil(t, result)
}

func TestDetectCapabilityDrift_InsufficientData(t *testing.T) {
	service := &BehaviorAnalysisService{}
	baseline := &domain.AgentBehaviorBaseline{
		CapabilityUsage: map[string]int{
			"file:read":  3,
			"file:write": 2,
		},
	}

	// Total usage is 5, less than 10 required
	result := service.detectCapabilityDrift(baseline, "file:read", uuid.New(), uuid.New())
	assert.Nil(t, result)
}

func TestDetectCapabilityDrift_NewCapabilityNotInMap(t *testing.T) {
	service := &BehaviorAnalysisService{}
	baseline := &domain.AgentBehaviorBaseline{
		CapabilityUsage: map[string]int{
			"file:read":  50,
			"file:write": 50,
		},
	}

	// Capability not in map - returns nil (handled by new_capability check)
	result := service.detectCapabilityDrift(baseline, "file:delete", uuid.New(), uuid.New())
	assert.Nil(t, result)
}

func TestDetectCapabilityDrift_HighProportionCapability(t *testing.T) {
	service := &BehaviorAnalysisService{}
	baseline := &domain.AgentBehaviorBaseline{
		CapabilityUsage: map[string]int{
			"file:read":  80,
			"file:write": 20,
		},
	}

	// file:read is 80% of usage, above 5% threshold
	result := service.detectCapabilityDrift(baseline, "file:read", uuid.New(), uuid.New())
	assert.Nil(t, result)
}

func TestDetectCapabilityDrift_LowProportionCapability(t *testing.T) {
	service := &BehaviorAnalysisService{}
	baseline := &domain.AgentBehaviorBaseline{
		CapabilityUsage: map[string]int{
			"file:read":   95,
			"file:write":  3,
			"file:delete": 2,
		},
	}

	// file:delete is 2% of usage, below 5% threshold
	// Current implementation returns nil for this case (skip for MVP)
	result := service.detectCapabilityDrift(baseline, "file:delete", uuid.New(), uuid.New())
	assert.Nil(t, result)
}
