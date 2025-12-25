package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBehavioralAnomalyType_Constants(t *testing.T) {
	tests := []struct {
		constant BehavioralAnomalyType
		expected string
	}{
		{BehavioralAnomalyVelocitySpike, "velocity_spike"},
		{BehavioralAnomalyNewCapability, "new_capability"},
		{BehavioralAnomalyNewResource, "new_resource"},
		{BehavioralAnomalyPatternBreak, "pattern_break"},
		{BehavioralAnomalyCrossAgent, "cross_agent"},
		{BehavioralAnomalyRiskSpike, "risk_spike"},
		{BehavioralAnomalyCapabilityDrift, "capability_drift"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, BehavioralAnomalyType(tt.expected), tt.constant)
		})
	}
}

func TestAnomalySeverity_Constants(t *testing.T) {
	assert.Equal(t, AnomalySeverity("low"), AnomalySeverityLow)
	assert.Equal(t, AnomalySeverity("medium"), AnomalySeverityMedium)
	assert.Equal(t, AnomalySeverity("high"), AnomalySeverityHigh)
	assert.Equal(t, AnomalySeverity("critical"), AnomalySeverityCritical)
}

func TestAgentBehaviorBaseline_Structure(t *testing.T) {
	now := time.Now()
	agentID := uuid.New()
	orgID := uuid.New()

	baseline := AgentBehaviorBaseline{
		ID:              uuid.New(),
		AgentID:         agentID,
		OrganizationID:  orgID,
		BaselineStartAt: now.Add(-7 * 24 * time.Hour),
		BaselineEndAt:   now,
		SamplesCount:    1000,
		AvgCallsPerHour:    50.5,
		StddevCallsPerHour: 15.2,
		MaxCallsPerHour:    120,
		CapabilityUsage: map[string]int{
			"read":    500,
			"write":   300,
			"execute": 200,
		},
		ResourcePatterns: map[string]int{
			"database": 400,
			"file":     350,
			"api":      250,
		},
		ActionSequences: []ActionSequence{
			{Sequence: []string{"read", "process", "write"}, Count: 100},
			{Sequence: []string{"authenticate", "read"}, Count: 50},
		},
		AvgRiskScore:      3.5,
		StddevRiskScore:   1.2,
		IsMature:          true,
		MaturityThreshold: 500,
		LastUpdatedAt:     now,
		CreatedAt:         now.Add(-7 * 24 * time.Hour),
	}

	assert.NotEqual(t, uuid.Nil, baseline.ID)
	assert.Equal(t, agentID, baseline.AgentID)
	assert.Equal(t, orgID, baseline.OrganizationID)
	assert.Equal(t, 1000, baseline.SamplesCount)
	assert.Equal(t, 50.5, baseline.AvgCallsPerHour)
	assert.Equal(t, 15.2, baseline.StddevCallsPerHour)
	assert.Equal(t, 120, baseline.MaxCallsPerHour)
	assert.True(t, baseline.IsMature)
	assert.Len(t, baseline.ActionSequences, 2)
}

func TestAgentBehaviorBaseline_Immature(t *testing.T) {
	baseline := AgentBehaviorBaseline{
		ID:                uuid.New(),
		AgentID:           uuid.New(),
		OrganizationID:    uuid.New(),
		SamplesCount:      100, // Below threshold
		IsMature:          false,
		MaturityThreshold: 500,
	}

	assert.False(t, baseline.IsMature)
	assert.Less(t, baseline.SamplesCount, baseline.MaturityThreshold)
}

func TestActionSequence_Structure(t *testing.T) {
	seq := ActionSequence{
		Sequence: []string{"authenticate", "read", "transform", "write"},
		Count:    150,
	}

	assert.Len(t, seq.Sequence, 4)
	assert.Equal(t, "authenticate", seq.Sequence[0])
	assert.Equal(t, 150, seq.Count)
}

func TestBehavioralAnomaly_Structure(t *testing.T) {
	now := time.Now()
	agentID := uuid.New()
	orgID := uuid.New()
	alertID := uuid.New()
	expectedVal := 50.0
	actualVal := 150.0
	deviationSigma := 6.6

	anomaly := BehavioralAnomaly{
		ID:                uuid.New(),
		AgentID:           agentID,
		OrganizationID:    orgID,
		AnomalyType:       BehavioralAnomalyVelocitySpike,
		Severity:          AnomalySeverityHigh,
		Description:       "Agent activity rate spiked significantly above baseline",
		ExpectedValue:     &expectedVal,
		ActualValue:       &actualVal,
		DeviationSigma:    &deviationSigma,
		TriggerAction:     "api_call",
		TriggerResource:   "database",
		TriggerCapability: "execute",
		Metadata: map[string]interface{}{
			"baseline_period": "7d",
			"detection_time":  now.Format(time.RFC3339),
		},
		DetectedAt: now,
		AlertID:    &alertID,
	}

	assert.NotEqual(t, uuid.Nil, anomaly.ID)
	assert.Equal(t, agentID, anomaly.AgentID)
	assert.Equal(t, BehavioralAnomalyVelocitySpike, anomaly.AnomalyType)
	assert.Equal(t, AnomalySeverityHigh, anomaly.Severity)
	assert.Equal(t, 50.0, *anomaly.ExpectedValue)
	assert.Equal(t, 150.0, *anomaly.ActualValue)
	assert.Equal(t, 6.6, *anomaly.DeviationSigma)
	assert.Equal(t, &alertID, anomaly.AlertID)
}

func TestBehavioralAnomaly_NewCapability(t *testing.T) {
	anomaly := BehavioralAnomaly{
		ID:                uuid.New(),
		AgentID:           uuid.New(),
		OrganizationID:    uuid.New(),
		AnomalyType:       BehavioralAnomalyNewCapability,
		Severity:          AnomalySeverityMedium,
		Description:       "Agent used a capability for the first time: admin:delete",
		TriggerCapability: "admin:delete",
		Metadata: map[string]interface{}{
			"first_seen": time.Now().Format(time.RFC3339),
		},
		DetectedAt: time.Now(),
	}

	assert.Equal(t, BehavioralAnomalyNewCapability, anomaly.AnomalyType)
	assert.Equal(t, "admin:delete", anomaly.TriggerCapability)
}

func TestBehavioralAnomaly_PatternBreak(t *testing.T) {
	anomaly := BehavioralAnomaly{
		ID:             uuid.New(),
		AgentID:        uuid.New(),
		OrganizationID: uuid.New(),
		AnomalyType:    BehavioralAnomalyPatternBreak,
		Severity:       AnomalySeverityHigh,
		Description:    "Agent performed unusual action sequence",
		TriggerAction:  "delete_all",
		Metadata: map[string]interface{}{
			"expected_sequence": []string{"read", "update"},
			"actual_sequence":   []string{"read", "delete_all"},
		},
		DetectedAt: time.Now(),
	}

	assert.Equal(t, BehavioralAnomalyPatternBreak, anomaly.AnomalyType)
	assert.Equal(t, "delete_all", anomaly.TriggerAction)
}

func TestDetectionResult_Structure(t *testing.T) {
	tests := []struct {
		name        string
		result      DetectionResult
		expectedAnomalous bool
		expectedBlock     bool
		expectedAlert     bool
	}{
		{
			name: "no anomaly detected",
			result: DetectionResult{
				IsAnomalous: false,
				Anomalies:   nil,
				ShouldBlock: false,
				ShouldAlert: false,
				Reason:      "",
			},
			expectedAnomalous: false,
			expectedBlock:     false,
			expectedAlert:     false,
		},
		{
			name: "low severity anomaly",
			result: DetectionResult{
				IsAnomalous: true,
				Anomalies: []BehavioralAnomaly{
					{
						AnomalyType: BehavioralAnomalyNewResource,
						Severity:    AnomalySeverityLow,
					},
				},
				ShouldBlock: false,
				ShouldAlert: false,
				Reason:      "New resource access detected",
			},
			expectedAnomalous: true,
			expectedBlock:     false,
			expectedAlert:     false,
		},
		{
			name: "critical anomaly - block and alert",
			result: DetectionResult{
				IsAnomalous: true,
				Anomalies: []BehavioralAnomaly{
					{
						AnomalyType: BehavioralAnomalyRiskSpike,
						Severity:    AnomalySeverityCritical,
					},
				},
				ShouldBlock: true,
				ShouldAlert: true,
				Reason:      "Critical risk spike detected - action blocked",
			},
			expectedAnomalous: true,
			expectedBlock:     true,
			expectedAlert:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedAnomalous, tt.result.IsAnomalous)
			assert.Equal(t, tt.expectedBlock, tt.result.ShouldBlock)
			assert.Equal(t, tt.expectedAlert, tt.result.ShouldAlert)
		})
	}
}

func TestCalculateSigma(t *testing.T) {
	tests := []struct {
		name          string
		value         float64
		mean          float64
		stddev        float64
		expectedSigma float64
	}{
		{
			name:          "value equals mean",
			value:         50.0,
			mean:          50.0,
			stddev:        10.0,
			expectedSigma: 0.0,
		},
		{
			name:          "value one stddev above mean",
			value:         60.0,
			mean:          50.0,
			stddev:        10.0,
			expectedSigma: 1.0,
		},
		{
			name:          "value two stddevs below mean",
			value:         30.0,
			mean:          50.0,
			stddev:        10.0,
			expectedSigma: -2.0,
		},
		{
			name:          "value three stddevs above mean",
			value:         80.0,
			mean:          50.0,
			stddev:        10.0,
			expectedSigma: 3.0,
		},
		{
			name:          "zero stddev returns zero",
			value:         100.0,
			mean:          50.0,
			stddev:        0.0,
			expectedSigma: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sigma := CalculateSigma(tt.value, tt.mean, tt.stddev)
			assert.InDelta(t, tt.expectedSigma, sigma, 0.001)
		})
	}
}

func TestIsSignificantDeviation(t *testing.T) {
	tests := []struct {
		name        string
		sigma       float64
		threshold   float64
		expected    bool
	}{
		{
			name:      "below threshold - not significant",
			sigma:     2.0,
			threshold: 3.0,
			expected:  false,
		},
		{
			name:      "at threshold - not significant",
			sigma:     3.0,
			threshold: 3.0,
			expected:  false,
		},
		{
			name:      "above threshold - significant",
			sigma:     3.5,
			threshold: 3.0,
			expected:  true,
		},
		{
			name:      "negative below threshold - significant",
			sigma:     -4.0,
			threshold: 3.0,
			expected:  true,
		},
		{
			name:      "default threshold (0) uses 3.0",
			sigma:     3.5,
			threshold: 0,
			expected:  true,
		},
		{
			name:      "default threshold - not significant",
			sigma:     2.5,
			threshold: 0,
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSignificantDeviation(tt.sigma, tt.threshold)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetRiskWeight(t *testing.T) {
	tests := []struct {
		capability     string
		expectedWeight float64
	}{
		{"admin:users:delete", 10.0},
		{"admin_panel", 10.0},
		{"delete:resource", 8.0},
		{"delete_file", 8.0},
		{"write:data", 5.0},
		{"write_log", 5.0},
		{"execute:command", 6.0},
		{"execute_script", 6.0},
		{"security:audit", 9.0},
		{"security_check", 9.0},
		{"system:config", 8.0},
		{"system_restart", 8.0},
		{"read:document", 1.0},
		{"read_only", 1.0},
		{"list:users", 1.0},
		{"list_files", 1.0},
		{"api:call", 2.0},
		{"api_request", 2.0},
		{"unknown_capability", 3.0},
		{"", 3.0},
	}

	for _, tt := range tests {
		t.Run(tt.capability, func(t *testing.T) {
			weight := GetRiskWeight(tt.capability)
			assert.Equal(t, tt.expectedWeight, weight)
		})
	}
}

func TestAgentBehaviorBaseline_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	baseline := AgentBehaviorBaseline{
		ID:                 uuid.New(),
		AgentID:            uuid.New(),
		OrganizationID:     uuid.New(),
		BaselineStartAt:    now.Add(-7 * 24 * time.Hour),
		BaselineEndAt:      now,
		SamplesCount:       500,
		AvgCallsPerHour:    25.0,
		StddevCallsPerHour: 5.0,
		MaxCallsPerHour:    50,
		CapabilityUsage:    map[string]int{"read": 300, "write": 200},
		ResourcePatterns:   map[string]int{"api": 500},
		ActionSequences:    []ActionSequence{{Sequence: []string{"a", "b"}, Count: 10}},
		AvgRiskScore:       2.5,
		StddevRiskScore:    0.5,
		IsMature:           true,
		MaturityThreshold:  500,
		LastUpdatedAt:      now,
		CreatedAt:          now.Add(-7 * 24 * time.Hour),
	}

	// Serialize to JSON
	data, err := json.Marshal(baseline)
	require.NoError(t, err)

	// Deserialize back
	var restored AgentBehaviorBaseline
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, baseline.ID, restored.ID)
	assert.Equal(t, baseline.AgentID, restored.AgentID)
	assert.Equal(t, baseline.SamplesCount, restored.SamplesCount)
	assert.Equal(t, baseline.AvgCallsPerHour, restored.AvgCallsPerHour)
	assert.Equal(t, baseline.IsMature, restored.IsMature)
}

func TestBehavioralAnomaly_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	expectedVal := 50.0

	anomaly := BehavioralAnomaly{
		ID:             uuid.New(),
		AgentID:        uuid.New(),
		OrganizationID: uuid.New(),
		AnomalyType:    BehavioralAnomalyVelocitySpike,
		Severity:       AnomalySeverityHigh,
		Description:    "Test anomaly",
		ExpectedValue:  &expectedVal,
		Metadata:       map[string]interface{}{"test": true},
		DetectedAt:     now,
	}

	// Serialize to JSON
	data, err := json.Marshal(anomaly)
	require.NoError(t, err)

	// Deserialize back
	var restored BehavioralAnomaly
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, anomaly.ID, restored.ID)
	assert.Equal(t, anomaly.AnomalyType, restored.AnomalyType)
	assert.Equal(t, anomaly.Severity, restored.Severity)
	assert.Equal(t, *anomaly.ExpectedValue, *restored.ExpectedValue)
}

func TestBehavioralAnomaly_AllTypes(t *testing.T) {
	types := []BehavioralAnomalyType{
		BehavioralAnomalyVelocitySpike,
		BehavioralAnomalyNewCapability,
		BehavioralAnomalyNewResource,
		BehavioralAnomalyPatternBreak,
		BehavioralAnomalyCrossAgent,
		BehavioralAnomalyRiskSpike,
		BehavioralAnomalyCapabilityDrift,
	}

	for _, anomalyType := range types {
		t.Run(string(anomalyType), func(t *testing.T) {
			anomaly := BehavioralAnomaly{
				ID:             uuid.New(),
				AgentID:        uuid.New(),
				OrganizationID: uuid.New(),
				AnomalyType:    anomalyType,
				Severity:       AnomalySeverityMedium,
				Description:    "Test anomaly of type: " + string(anomalyType),
				DetectedAt:     time.Now(),
			}

			assert.Equal(t, anomalyType, anomaly.AnomalyType)
			assert.NotEmpty(t, string(anomalyType))
		})
	}
}
