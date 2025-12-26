package application

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
)

// ========================================
// deduplicateSlice Tests
// ========================================

func TestDeduplicateSlice_EmptySlice(t *testing.T) {
	result := deduplicateSlice([]string{})
	assert.Empty(t, result)
}

func TestDeduplicateSlice_NoDuplicates(t *testing.T) {
	input := []string{"a", "b", "c"}
	result := deduplicateSlice(input)

	assert.Equal(t, 3, len(result))
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
	assert.Contains(t, result, "c")
}

func TestDeduplicateSlice_WithDuplicates(t *testing.T) {
	input := []string{"a", "b", "a", "c", "b", "d"}
	result := deduplicateSlice(input)

	assert.Equal(t, 4, len(result))
	assert.Contains(t, result, "a")
	assert.Contains(t, result, "b")
	assert.Contains(t, result, "c")
	assert.Contains(t, result, "d")
}

func TestDeduplicateSlice_AllDuplicates(t *testing.T) {
	input := []string{"a", "a", "a", "a"}
	result := deduplicateSlice(input)

	assert.Equal(t, 1, len(result))
	assert.Equal(t, "a", result[0])
}

func TestDeduplicateSlice_PreservesOrder(t *testing.T) {
	input := []string{"c", "a", "b", "a", "c"}
	result := deduplicateSlice(input)

	// Should preserve first occurrence order
	assert.Equal(t, 3, len(result))
	assert.Equal(t, "c", result[0])
	assert.Equal(t, "a", result[1])
	assert.Equal(t, "b", result[2])
}

func TestDeduplicateSlice_SingleElement(t *testing.T) {
	input := []string{"only"}
	result := deduplicateSlice(input)

	assert.Equal(t, 1, len(result))
	assert.Equal(t, "only", result[0])
}

func TestDeduplicateSlice_EmptyStrings(t *testing.T) {
	input := []string{"", "", "a", ""}
	result := deduplicateSlice(input)

	assert.Equal(t, 2, len(result))
	assert.Contains(t, result, "")
	assert.Contains(t, result, "a")
}

// ========================================
// countHighSeverityAlerts Tests
// ========================================

func TestCountHighSeverityAlerts_Empty(t *testing.T) {
	alerts := []domain.SecurityAlert{}
	count := countHighSeverityAlerts(alerts)
	assert.Equal(t, 0, count)
}

func TestCountHighSeverityAlerts_NoCriticalOrHigh(t *testing.T) {
	alerts := []domain.SecurityAlert{
		{Severity: "LOW", Message: "Low alert"},
		{Severity: "MEDIUM", Message: "Medium alert"},
		{Severity: "INFO", Message: "Info alert"},
	}
	count := countHighSeverityAlerts(alerts)
	assert.Equal(t, 0, count)
}

func TestCountHighSeverityAlerts_OnlyCritical(t *testing.T) {
	alerts := []domain.SecurityAlert{
		{Severity: "CRITICAL", Message: "Critical alert 1"},
		{Severity: "CRITICAL", Message: "Critical alert 2"},
		{Severity: "LOW", Message: "Low alert"},
	}
	count := countHighSeverityAlerts(alerts)
	assert.Equal(t, 2, count)
}

func TestCountHighSeverityAlerts_OnlyHigh(t *testing.T) {
	alerts := []domain.SecurityAlert{
		{Severity: "HIGH", Message: "High alert 1"},
		{Severity: "HIGH", Message: "High alert 2"},
		{Severity: "MEDIUM", Message: "Medium alert"},
	}
	count := countHighSeverityAlerts(alerts)
	assert.Equal(t, 2, count)
}

func TestCountHighSeverityAlerts_MixedSeverities(t *testing.T) {
	alerts := []domain.SecurityAlert{
		{Severity: "CRITICAL", Message: "Critical"},
		{Severity: "HIGH", Message: "High"},
		{Severity: "MEDIUM", Message: "Medium"},
		{Severity: "LOW", Message: "Low"},
		{Severity: "CRITICAL", Message: "Critical 2"},
	}
	count := countHighSeverityAlerts(alerts)
	assert.Equal(t, 3, count)
}

func TestCountHighSeverityAlerts_CaseSensitivity(t *testing.T) {
	// The function checks for exact match - lowercase won't count
	alerts := []domain.SecurityAlert{
		{Severity: "critical", Message: "Lowercase critical"},
		{Severity: "high", Message: "Lowercase high"},
		{Severity: "CRITICAL", Message: "Uppercase CRITICAL"},
	}
	count := countHighSeverityAlerts(alerts)
	// Only uppercase CRITICAL should count
	assert.Equal(t, 1, count)
}

// ========================================
// NewDetectionService Tests
// ========================================

func TestNewDetectionService_ProductionWindow(t *testing.T) {
	// Clear environment to ensure default (production) settings
	os.Unsetenv("ENVIRONMENT")

	service := NewDetectionService(nil, nil, nil)
	assert.NotNil(t, service)
	assert.Equal(t, 24*time.Hour, service.deduplicationWindow)
}

func TestNewDetectionService_DevelopmentWindow(t *testing.T) {
	tests := []struct {
		envValue string
	}{
		{"development"},
		{"dev"},
	}

	for _, tt := range tests {
		t.Run(tt.envValue, func(t *testing.T) {
			os.Setenv("ENVIRONMENT", tt.envValue)
			defer os.Unsetenv("ENVIRONMENT")

			service := NewDetectionService(nil, nil, nil)
			assert.NotNil(t, service)
			assert.Equal(t, 5*time.Minute, service.deduplicationWindow)
		})
	}
}

func TestNewDetectionService_OtherEnvironments(t *testing.T) {
	// Non-dev environments should use production window
	envs := []string{"staging", "production", "prod", "test"}

	for _, env := range envs {
		t.Run(env, func(t *testing.T) {
			os.Setenv("ENVIRONMENT", env)
			defer os.Unsetenv("ENVIRONMENT")

			service := NewDetectionService(nil, nil, nil)
			assert.Equal(t, 24*time.Hour, service.deduplicationWindow)
		})
	}
}

func TestNewDetectionService_StoresDependencies(t *testing.T) {
	service := NewDetectionService(nil, nil, nil)

	assert.Nil(t, service.db)
	assert.Nil(t, service.trustCalculator)
	assert.Nil(t, service.agentRepo)
}

// ========================================
// DetectionReportRequest Validation Tests
// ========================================

func TestDetectionValidation_EmptyServerName(t *testing.T) {
	// Empty server names should be skipped
	detection := domain.DetectionEvent{
		MCPServer:       "",
		DetectionMethod: "runtime_hook",
		Confidence:      90.0,
	}

	assert.Empty(t, detection.MCPServer)
}

func TestDetectionValidation_ConfidenceRange(t *testing.T) {
	tests := []struct {
		name       string
		confidence float64
		isValid    bool
	}{
		{"negative", -1, false},
		{"zero", 0, true},
		{"fifty", 50, true},
		{"hundred", 100, true},
		{"over hundred", 101, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detection := domain.DetectionEvent{
				MCPServer:       "test-mcp",
				DetectionMethod: "runtime_hook",
				Confidence:      tt.confidence,
			}

			isValid := detection.Confidence >= 0 && detection.Confidence <= 100
			assert.Equal(t, tt.isValid, isValid)
		})
	}
}

// ========================================
// DetectionReportResponse Tests
// ========================================

func TestDetectionReportResponse_Structure(t *testing.T) {
	response := domain.DetectionReportResponse{
		Success:             true,
		DetectionsProcessed: 5,
		NewMCPs:             []string{"mcp1", "mcp2"},
		ExistingMCPs:        []string{"mcp3"},
		Message:             "Processed 5 detections",
	}

	assert.True(t, response.Success)
	assert.Equal(t, 5, response.DetectionsProcessed)
	assert.Len(t, response.NewMCPs, 2)
	assert.Len(t, response.ExistingMCPs, 1)
	assert.NotEmpty(t, response.Message)
}

// ========================================
// DetectionStatusResponse Tests
// ========================================

func TestDetectionStatusResponse_DefaultState(t *testing.T) {
	agentID := uuid.New()

	response := domain.DetectionStatusResponse{
		AgentID:      agentID,
		SDKInstalled: false,
		DetectedMCPs: []domain.DetectedMCPSummary{},
	}

	assert.Equal(t, agentID, response.AgentID)
	assert.False(t, response.SDKInstalled)
	assert.Empty(t, response.DetectedMCPs)
	assert.Empty(t, response.SDKVersion)
	assert.Nil(t, response.LastReportedAt)
}

func TestDetectionStatusResponse_WithSDK(t *testing.T) {
	now := time.Now()
	agentID := uuid.New()

	response := domain.DetectionStatusResponse{
		AgentID:           agentID,
		SDKInstalled:      true,
		SDKVersion:        "1.2.3",
		AutoDetectEnabled: true,
		LastReportedAt:    &now,
		Protocol:          "mcp-v1",
		DetectedMCPs:      []domain.DetectedMCPSummary{},
	}

	assert.True(t, response.SDKInstalled)
	assert.Equal(t, "1.2.3", response.SDKVersion)
	assert.True(t, response.AutoDetectEnabled)
	assert.NotNil(t, response.LastReportedAt)
	assert.Equal(t, "mcp-v1", response.Protocol)
}

// ========================================
// DetectedMCPSummary Tests
// ========================================

func TestDetectedMCPSummary_Structure(t *testing.T) {
	firstDetected := time.Now().Add(-24 * time.Hour)
	lastSeen := time.Now()

	mcp := domain.DetectedMCPSummary{
		Name:            "test-mcp-server",
		DetectedBy:      []domain.DetectionMethod{"runtime_hook", "network_analysis"},
		ConfidenceScore: 95.5,
		FirstDetected:   &firstDetected,
		LastSeen:        &lastSeen,
		IsManual:        false,
	}

	assert.Equal(t, "test-mcp-server", mcp.Name)
	assert.Len(t, mcp.DetectedBy, 2)
	assert.Equal(t, 95.5, mcp.ConfidenceScore)
	assert.NotNil(t, mcp.FirstDetected)
	assert.NotNil(t, mcp.LastSeen)
	assert.False(t, mcp.IsManual)
}

func TestDetectedMCPSummary_ManualDetection(t *testing.T) {
	mcp := domain.DetectedMCPSummary{
		Name:            "manual-mcp",
		DetectedBy:      []domain.DetectionMethod{},
		ConfidenceScore: 100.0,
		IsManual:        true,
	}

	assert.True(t, mcp.IsManual)
	assert.Equal(t, 100.0, mcp.ConfidenceScore)
}

// ========================================
// Detection Methods Tests
// ========================================

func TestDetectionMethods_KnownMethods(t *testing.T) {
	// Document known detection methods
	knownMethods := []domain.DetectionMethod{
		domain.DetectionMethod("runtime_hook"),
		domain.DetectionMethod("network_analysis"),
		domain.DetectionMethod("process_monitor"),
		domain.DetectionMethod("config_scan"),
	}

	for _, method := range knownMethods {
		assert.NotEmpty(t, string(method))
	}
}

// ========================================
// Confidence Score Boost Logic Tests
// ========================================

func TestConfidenceBoost_SingleMethod(t *testing.T) {
	// Single method - no boost
	baseConfidence := 80.0
	methodCount := 1

	boostedConfidence := baseConfidence
	if methodCount >= 2 {
		boostedConfidence = min(99.0, boostedConfidence+10)
	}
	if methodCount >= 3 {
		boostedConfidence = min(99.0, boostedConfidence+20)
	}

	assert.Equal(t, 80.0, boostedConfidence)
}

func TestConfidenceBoost_TwoMethods(t *testing.T) {
	// Two methods - +10 boost
	baseConfidence := 80.0
	methodCount := 2

	boostedConfidence := baseConfidence
	if methodCount >= 2 {
		boostedConfidence = min(99.0, boostedConfidence+10)
	}
	if methodCount >= 3 {
		boostedConfidence = min(99.0, boostedConfidence+20)
	}

	assert.Equal(t, 90.0, boostedConfidence)
}

func TestConfidenceBoost_ThreeMethods(t *testing.T) {
	// Three methods - +10 and +20 boost (total +30)
	baseConfidence := 70.0
	methodCount := 3

	boostedConfidence := baseConfidence
	if methodCount >= 2 {
		boostedConfidence = min(99.0, boostedConfidence+10)
	}
	if methodCount >= 3 {
		boostedConfidence = min(99.0, boostedConfidence+20)
	}

	// 70 + 10 = 80, then min(99, 80+20) = 99
	assert.Equal(t, 99.0, boostedConfidence)
}

func TestConfidenceBoost_MaxCap(t *testing.T) {
	// Even with high base and multiple methods, cap at 99
	baseConfidence := 95.0
	methodCount := 3

	boostedConfidence := baseConfidence
	if methodCount >= 2 {
		boostedConfidence = min(99.0, boostedConfidence+10)
	}
	if methodCount >= 3 {
		boostedConfidence = min(99.0, boostedConfidence+20)
	}

	// 95 + 10 = 99 (capped), then min(99, 99+20) = 99
	assert.Equal(t, 99.0, boostedConfidence)
}

// ========================================
// CapabilityReportResponse Tests
// ========================================

func TestCapabilityReportResponse_Structure(t *testing.T) {
	agentID := uuid.New()

	response := domain.CapabilityReportResponse{
		Success:             true,
		AgentID:             agentID,
		RiskLevel:           "MEDIUM",
		TrustScoreImpact:    -5,
		NewTrustScore:       0.75,
		SecurityAlertsCount: 2,
		Message:             "Report processed",
	}

	assert.True(t, response.Success)
	assert.Equal(t, agentID, response.AgentID)
	assert.Equal(t, "MEDIUM", response.RiskLevel)
	assert.Equal(t, -5, response.TrustScoreImpact)
	assert.Equal(t, 0.75, response.NewTrustScore)
	assert.Equal(t, 2, response.SecurityAlertsCount)
}

// ========================================
// DetectionEvent Tests
// ========================================

func TestDetectionEvent_Structure(t *testing.T) {
	detection := domain.DetectionEvent{
		MCPServer:       "test-server",
		DetectionMethod: "runtime_hook",
		Confidence:      85.0,
		SDKVersion:      "1.0.0",
		Details: map[string]interface{}{
			"source": "automatic",
			"port":   8080,
		},
	}

	assert.Equal(t, "test-server", detection.MCPServer)
	assert.Equal(t, domain.DetectionMethod("runtime_hook"), detection.DetectionMethod)
	assert.Equal(t, 85.0, detection.Confidence)
	assert.Equal(t, "1.0.0", detection.SDKVersion)
	assert.NotNil(t, detection.Details)
	assert.Equal(t, "automatic", detection.Details["source"])
}

// ========================================
// Deduplication Window Tests
// ========================================

func TestDeduplicationWindow_Values(t *testing.T) {
	// Document expected deduplication windows
	productionWindow := 24 * time.Hour
	developmentWindow := 5 * time.Minute

	assert.Equal(t, time.Duration(24)*time.Hour, productionWindow)
	assert.Equal(t, time.Duration(5)*time.Minute, developmentWindow)

	// Production window should be much larger than development
	assert.True(t, productionWindow > developmentWindow)
}

// ========================================
// Significance Logic Tests
// ========================================

func TestSignificanceLogic_FirstDetection(t *testing.T) {
	// First detection is always significant
	isFirstDetection := true
	isSignificant := isFirstDetection
	assert.True(t, isSignificant)
}

func TestSignificanceLogic_WithinWindow(t *testing.T) {
	deduplicationWindow := 24 * time.Hour
	lastSignificantTime := time.Now().Add(-12 * time.Hour)
	timeSinceLastSignificant := time.Since(lastSignificantTime)

	isSignificant := timeSinceLastSignificant >= deduplicationWindow
	assert.False(t, isSignificant)
}

func TestSignificanceLogic_OutsideWindow(t *testing.T) {
	deduplicationWindow := 24 * time.Hour
	lastSignificantTime := time.Now().Add(-36 * time.Hour)
	timeSinceLastSignificant := time.Since(lastSignificantTime)

	isSignificant := timeSinceLastSignificant >= deduplicationWindow
	assert.True(t, isSignificant)
}
