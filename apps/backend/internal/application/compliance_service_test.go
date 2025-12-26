package application

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/opena2a-org/agent-identity-management/apps/backend/internal/domain"
	"github.com/stretchr/testify/assert"
)

// ========================================
// Struct Tests
// ========================================

func TestComplianceReport_Structure(t *testing.T) {
	orgID := uuid.New()
	now := time.Now()

	report := ComplianceReport{
		OrganizationID: orgID.String(),
		GeneratedAt:    now,
		Period:         "2025-01-01 to 2025-01-31",
		Summary: ComplianceSummary{
			TotalAgents:       10,
			VerifiedAgents:    8,
			PendingAgents:     2,
			AverageTrustScore: 0.85,
		},
		Agents: []AgentCompliance{
			{
				ID:         uuid.New().String(),
				Name:       "test-agent",
				Type:       "claude",
				Status:     "verified",
				TrustScore: 0.90,
			},
		},
		Recommendations: []string{"Keep up the good work!"},
	}

	assert.Equal(t, orgID.String(), report.OrganizationID)
	assert.Equal(t, now, report.GeneratedAt)
	assert.Equal(t, 10, report.Summary.TotalAgents)
	assert.Equal(t, 8, report.Summary.VerifiedAgents)
	assert.Equal(t, 0.85, report.Summary.AverageTrustScore)
	assert.Len(t, report.Agents, 1)
	assert.Len(t, report.Recommendations, 1)
}

func TestComplianceSummary_Structure(t *testing.T) {
	summary := ComplianceSummary{
		TotalAgents:          100,
		VerifiedAgents:       85,
		PendingAgents:        10,
		AverageTrustScore:    0.78,
		ActiveAPIKeys:        25,
		TotalAuditLogs:       1500,
		UnacknowledgedAlerts: 3,
	}

	assert.Equal(t, 100, summary.TotalAgents)
	assert.Equal(t, 85, summary.VerifiedAgents)
	assert.Equal(t, 10, summary.PendingAgents)
	assert.Equal(t, 0.78, summary.AverageTrustScore)
	assert.Equal(t, 25, summary.ActiveAPIKeys)
	assert.Equal(t, 1500, summary.TotalAuditLogs)
	assert.Equal(t, 3, summary.UnacknowledgedAlerts)
}

func TestAgentCompliance_Structure(t *testing.T) {
	agentID := uuid.New()

	compliance := AgentCompliance{
		ID:             agentID.String(),
		Name:           "production-agent",
		Type:           "claude",
		Status:         "verified",
		TrustScore:     0.92,
		HasCertificate: true,
		LastVerified:   "2025-01-15",
	}

	assert.Equal(t, agentID.String(), compliance.ID)
	assert.Equal(t, "production-agent", compliance.Name)
	assert.Equal(t, "claude", compliance.Type)
	assert.Equal(t, "verified", compliance.Status)
	assert.Equal(t, 0.92, compliance.TrustScore)
	assert.True(t, compliance.HasCertificate)
	assert.Equal(t, "2025-01-15", compliance.LastVerified)
}

func TestAuditActivitySummary_Structure(t *testing.T) {
	summary := AuditActivitySummary{
		TotalActions:  500,
		UniqueUsers:   15,
		TopActions:    map[string]int{"verify": 100, "create": 200, "update": 150},
		RecentActions: 75,
	}

	assert.Equal(t, 500, summary.TotalActions)
	assert.Equal(t, 15, summary.UniqueUsers)
	assert.Equal(t, 100, summary.TopActions["verify"])
	assert.Equal(t, 200, summary.TopActions["create"])
	assert.Equal(t, 75, summary.RecentActions)
}

// ========================================
// determineComplianceLevel Tests
// ========================================

func TestDetermineComplianceLevel(t *testing.T) {
	tests := []struct {
		name             string
		avgTrustScore    float64
		verificationRate float64
		expectedLevel    string
	}{
		{
			name:             "excellent - high trust and verification",
			avgTrustScore:    0.85,
			verificationRate: 0.95,
			expectedLevel:    "excellent",
		},
		{
			name:             "excellent - at threshold",
			avgTrustScore:    0.80,
			verificationRate: 0.90,
			expectedLevel:    "excellent",
		},
		{
			name:             "good - high trust low verification",
			avgTrustScore:    0.85,
			verificationRate: 0.75,
			expectedLevel:    "good",
		},
		{
			name:             "good - at threshold",
			avgTrustScore:    0.60,
			verificationRate: 0.70,
			expectedLevel:    "good",
		},
		{
			name:             "fair - moderate scores",
			avgTrustScore:    0.50,
			verificationRate: 0.55,
			expectedLevel:    "fair",
		},
		{
			name:             "fair - at threshold",
			avgTrustScore:    0.40,
			verificationRate: 0.50,
			expectedLevel:    "fair",
		},
		{
			name:             "needs_improvement - low trust",
			avgTrustScore:    0.30,
			verificationRate: 0.80,
			expectedLevel:    "needs_improvement",
		},
		{
			name:             "needs_improvement - low verification",
			avgTrustScore:    0.80,
			verificationRate: 0.40,
			expectedLevel:    "needs_improvement",
		},
		{
			name:             "needs_improvement - both low",
			avgTrustScore:    0.20,
			verificationRate: 0.30,
			expectedLevel:    "needs_improvement",
		},
		{
			name:             "needs_improvement - zero values",
			avgTrustScore:    0.0,
			verificationRate: 0.0,
			expectedLevel:    "needs_improvement",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineComplianceLevel(tt.avgTrustScore, tt.verificationRate)
			assert.Equal(t, tt.expectedLevel, result)
		})
	}
}

// ========================================
// getComplianceChecks Tests
// ========================================

func TestGetComplianceChecks_BaseChecks(t *testing.T) {
	service := &ComplianceService{}

	// Test default/unknown check type returns base checks
	checks := service.getComplianceChecks("unknown")

	// Verify base checks are included
	expectedBaseChecks := []string{
		"apiKeyRotationNeeded",
		"trustScoreDegradation",
		"capabilityViolations",
		"adminAccessReview",
		"auditLogGaps",
		"inactiveAgents",
		"unverifiedAgentBacklog",
		"orphanedResources",
		"inactiveMCPServers",
		"unverifiedMCPBacklog",
	}

	for _, expected := range expectedBaseChecks {
		assert.Contains(t, checks, expected, "Missing base check: %s", expected)
	}
}

func TestGetComplianceChecks_SOC2(t *testing.T) {
	service := &ComplianceService{}

	checks := service.getComplianceChecks("soc2")

	// Should have base checks plus SOC2-specific checks
	assert.Contains(t, checks, "apiKeyRotationNeeded")      // Base check
	assert.Contains(t, checks, "roleSegregation")           // SOC2 specific
	assert.Contains(t, checks, "accessControlGaps")         // SOC2 specific
	assert.Contains(t, checks, "auditCompleteness")         // SOC2 specific
}

func TestGetComplianceChecks_ISO27001(t *testing.T) {
	service := &ComplianceService{}

	checks := service.getComplianceChecks("iso27001")

	// Should have base checks plus ISO27001-specific checks
	assert.Contains(t, checks, "apiKeyRotationNeeded")      // Base check
	assert.Contains(t, checks, "riskAssessmentOverdue")     // ISO27001 specific
	assert.Contains(t, checks, "incidentResponseReadiness") // ISO27001 specific
	assert.Contains(t, checks, "assetInventory")            // ISO27001 specific
}

func TestGetComplianceChecks_HIPAA(t *testing.T) {
	service := &ComplianceService{}

	checks := service.getComplianceChecks("hipaa")

	// Should have base checks plus HIPAA-specific checks
	assert.Contains(t, checks, "apiKeyRotationNeeded")       // Base check
	assert.Contains(t, checks, "phiAccessLogging")           // HIPAA specific
	assert.Contains(t, checks, "encryptionCompliance")       // HIPAA specific
	assert.Contains(t, checks, "breachNotificationReady")    // HIPAA specific
}

func TestGetComplianceChecks_GDPR(t *testing.T) {
	service := &ComplianceService{}

	checks := service.getComplianceChecks("gdpr")

	// Should have base checks plus GDPR-specific checks
	assert.Contains(t, checks, "apiKeyRotationNeeded")       // Base check
	assert.Contains(t, checks, "dataRetentionPolicy")        // GDPR specific
	assert.Contains(t, checks, "consentManagement")          // GDPR specific
	assert.Contains(t, checks, "rightToErasure")             // GDPR specific
}

// ========================================
// generateRecommendations Tests
// ========================================

func TestGenerateRecommendations_NoIssues(t *testing.T) {
	service := &ComplianceService{}

	summary := ComplianceSummary{
		TotalAgents:          10,
		VerifiedAgents:       10,
		PendingAgents:        0,
		AverageTrustScore:    0.85,
		UnacknowledgedAlerts: 0,
		TotalAuditLogs:       100,
	}

	recommendations := service.generateRecommendations(summary, nil)

	// Should have the "no issues" recommendation
	assert.Len(t, recommendations, 1)
	assert.Contains(t, recommendations[0], "No immediate compliance issues")
}

func TestGenerateRecommendations_PendingAgents(t *testing.T) {
	service := &ComplianceService{}

	summary := ComplianceSummary{
		TotalAgents:          10,
		VerifiedAgents:       7,
		PendingAgents:        3,
		AverageTrustScore:    0.85,
		UnacknowledgedAlerts: 0,
		TotalAuditLogs:       100,
	}

	recommendations := service.generateRecommendations(summary, nil)

	// Should recommend verifying pending agents
	hasRecommendation := false
	for _, rec := range recommendations {
		if rec == "Verify 3 pending agent(s) to improve security posture" {
			hasRecommendation = true
			break
		}
	}
	assert.True(t, hasRecommendation, "Should recommend verifying pending agents")
}

func TestGenerateRecommendations_LowTrustScore(t *testing.T) {
	service := &ComplianceService{}

	summary := ComplianceSummary{
		TotalAgents:          10,
		VerifiedAgents:       10,
		PendingAgents:        0,
		AverageTrustScore:    0.55, // Below 0.7 threshold
		UnacknowledgedAlerts: 0,
		TotalAuditLogs:       100,
	}

	recommendations := service.generateRecommendations(summary, nil)

	// Should recommend improving trust score
	hasRecommendation := false
	for _, rec := range recommendations {
		if rec == "Average trust score is below recommended threshold (70%). Consider reviewing agent configurations and documentation." {
			hasRecommendation = true
			break
		}
	}
	assert.True(t, hasRecommendation, "Should recommend improving trust score")
}

func TestGenerateRecommendations_UnacknowledgedAlerts(t *testing.T) {
	service := &ComplianceService{}

	summary := ComplianceSummary{
		TotalAgents:          10,
		VerifiedAgents:       10,
		PendingAgents:        0,
		AverageTrustScore:    0.85,
		UnacknowledgedAlerts: 5,
		TotalAuditLogs:       100,
	}

	recommendations := service.generateRecommendations(summary, nil)

	// Should recommend addressing alerts
	hasRecommendation := false
	for _, rec := range recommendations {
		if rec == "Address 5 unacknowledged alert(s) to maintain security compliance" {
			hasRecommendation = true
			break
		}
	}
	assert.True(t, hasRecommendation, "Should recommend addressing alerts")
}

func TestGenerateRecommendations_LowAuditActivity(t *testing.T) {
	service := &ComplianceService{}

	summary := ComplianceSummary{
		TotalAgents:          10,
		VerifiedAgents:       10,
		PendingAgents:        0,
		AverageTrustScore:    0.85,
		UnacknowledgedAlerts: 0,
		TotalAuditLogs:       5, // Below 10 threshold
	}

	recommendations := service.generateRecommendations(summary, nil)

	// Should recommend improving audit logging
	hasRecommendation := false
	for _, rec := range recommendations {
		if rec == "Low audit activity detected. Ensure all actions are being properly logged for compliance." {
			hasRecommendation = true
			break
		}
	}
	assert.True(t, hasRecommendation, "Should recommend improving audit logging")
}

func TestGenerateRecommendations_MultipleIssues(t *testing.T) {
	service := &ComplianceService{}

	summary := ComplianceSummary{
		TotalAgents:          10,
		VerifiedAgents:       5,
		PendingAgents:        5,
		AverageTrustScore:    0.50,
		UnacknowledgedAlerts: 10,
		TotalAuditLogs:       3,
	}

	recommendations := service.generateRecommendations(summary, nil)

	// Should have multiple recommendations (4 issues: pending, low trust, alerts, low audit)
	assert.Equal(t, 4, len(recommendations), "Should have 4 recommendations for all issues")
}

// ========================================
// analyzeAuditActivity Tests
// ========================================

func TestAnalyzeAuditActivity_EmptyLogs(t *testing.T) {
	service := &ComplianceService{}

	logs := []*domain.AuditLog{}
	summary := service.analyzeAuditActivity(logs)

	assert.Equal(t, 0, summary.TotalActions)
	assert.Equal(t, 0, summary.UniqueUsers)
	assert.Equal(t, 0, summary.RecentActions)
	assert.Empty(t, summary.TopActions)
}

func TestAnalyzeAuditActivity_WithLogs(t *testing.T) {
	service := &ComplianceService{}

	now := time.Now()
	userID := uuid.New()
	agentID := uuid.New()

	logs := []*domain.AuditLog{
		{
			UserID:    &userID,
			Action:    domain.AuditActionCreate,
			Timestamp: now.Add(-1 * time.Hour), // Recent (within 24h)
		},
		{
			AgentID:   &agentID,
			Action:    domain.AuditActionVerify,
			Timestamp: now.Add(-30 * time.Hour), // Not recent
		},
		{
			UserID:    &userID, // Same user
			Action:    domain.AuditActionCreate,
			Timestamp: now.Add(-2 * time.Hour), // Recent
		},
	}

	summary := service.analyzeAuditActivity(logs)

	assert.Equal(t, 3, summary.TotalActions)
	assert.Equal(t, 2, summary.UniqueUsers) // 1 user + 1 agent
	assert.Equal(t, 2, summary.RecentActions)
	assert.Equal(t, 2, summary.TopActions["create"])
	assert.Equal(t, 1, summary.TopActions["verify"])
}

// ========================================
// Constructor Tests
// ========================================

func TestNewComplianceService(t *testing.T) {
	service := NewComplianceService(nil, nil, nil, nil)

	assert.NotNil(t, service)
	// Repos set to nil should be nil
	assert.Nil(t, service.alertRepo)
	assert.Nil(t, service.evidenceRepo)
	assert.Nil(t, service.snapshotRepo)
}

func TestNewComplianceServiceFull(t *testing.T) {
	service := NewComplianceServiceFull(nil, nil, nil, nil, nil, nil, nil)

	assert.NotNil(t, service)
}

// ========================================
// Edge Cases Tests
// ========================================

func TestDetermineComplianceLevel_EdgeCases(t *testing.T) {
	// Test boundary conditions
	tests := []struct {
		name             string
		avgTrustScore    float64
		verificationRate float64
		expectedLevel    string
	}{
		{
			name:             "just below excellent trust",
			avgTrustScore:    0.79,
			verificationRate: 0.95,
			expectedLevel:    "good", // Trust is 0.79, not >= 0.80
		},
		{
			name:             "just below excellent verification",
			avgTrustScore:    0.85,
			verificationRate: 0.89,
			expectedLevel:    "good", // Verification is 0.89, not >= 0.90
		},
		{
			name:             "perfect scores",
			avgTrustScore:    1.0,
			verificationRate: 1.0,
			expectedLevel:    "excellent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineComplianceLevel(tt.avgTrustScore, tt.verificationRate)
			assert.Equal(t, tt.expectedLevel, result)
		})
	}
}

func TestComplianceSummary_ZeroAgents(t *testing.T) {
	// When there are zero agents, average should be handled safely
	summary := ComplianceSummary{
		TotalAgents:       0,
		VerifiedAgents:    0,
		AverageTrustScore: 0.0, // Should be 0, not NaN
	}

	assert.Equal(t, 0, summary.TotalAgents)
	assert.Equal(t, 0.0, summary.AverageTrustScore)
}

// ========================================
// determineSeverityFromScore Tests
// ========================================

func TestDetermineSeverityFromScore(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected string
	}{
		{"critical - score 0", 0, "critical"},
		{"critical - score 29", 29, "critical"},
		{"critical - score 29.9", 29.9, "critical"},
		{"high - score 30", 30, "high"},
		{"high - score 40", 40, "high"},
		{"high - score 49", 49, "high"},
		{"high - score 49.9", 49.9, "high"},
		{"medium - score 50", 50, "medium"},
		{"medium - score 55", 55, "medium"},
		{"medium - score 59", 59, "medium"},
		{"medium - score 59.9", 59.9, "medium"},
		{"low - score 60", 60, "low"},
		{"low - score 75", 75, "low"},
		{"low - score 100", 100, "low"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineSeverityFromScore(tt.score)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========================================
// determineSeverityFromViolationCount Tests
// ========================================

func TestDetermineSeverityFromViolationCount(t *testing.T) {
	tests := []struct {
		name     string
		count    int
		expected string
	}{
		{"low - count 0", 0, "low"},
		{"low - count 1", 1, "low"},
		{"medium - count 2", 2, "medium"},
		{"medium - count 3", 3, "medium"},
		{"medium - count 4", 4, "medium"},
		{"high - count 5", 5, "high"},
		{"high - count 7", 7, "high"},
		{"high - count 9", 9, "high"},
		{"critical - count 10", 10, "critical"},
		{"critical - count 15", 15, "critical"},
		{"critical - count 100", 100, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineSeverityFromViolationCount(tt.count)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineReportStatus(t *testing.T) {
	tests := []struct {
		name     string
		score    float64
		expected string
	}{
		{"compliant - score 100", 100.0, "compliant"},
		{"compliant - score 95", 95.0, "compliant"},
		{"compliant - score 90", 90.0, "compliant"},
		{"needs_attention - score 89", 89.0, "needs_attention"},
		{"needs_attention - score 80", 80.0, "needs_attention"},
		{"needs_attention - score 70", 70.0, "needs_attention"},
		{"non_compliant - score 69", 69.0, "non_compliant"},
		{"non_compliant - score 50", 50.0, "non_compliant"},
		{"non_compliant - score 0", 0.0, "non_compliant"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := determineReportStatus(tt.score)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========================================
// GenerateComplianceReport Tests
// ========================================

func TestComplianceService_GenerateComplianceReport_Success(t *testing.T) {
	mockAgentRepo := new(SharedMockAgentRepository)
	mockAuditRepo := new(SharedMockAuditLogRepository)

	orgID := uuid.New()
	now := time.Now()
	verifiedAt := now.Add(-24 * time.Hour)

	agents := []*domain.Agent{
		{
			ID:             uuid.New(),
			OrganizationID: orgID,
			Name:           "agent-1",
			DisplayName:    "Agent One",
			AgentType:      domain.AgentTypeClaude,
			Status:         domain.AgentStatusVerified,
			TrustScore:     0.85,
			VerifiedAt:     &verifiedAt,
		},
		{
			ID:             uuid.New(),
			OrganizationID: orgID,
			Name:           "agent-2",
			DisplayName:    "Agent Two",
			AgentType:      domain.AgentTypeCustom,
			Status:         domain.AgentStatusPending,
			TrustScore:     0.50,
		},
	}

	userID := uuid.New()
	auditLogs := []*domain.AuditLog{
		{
			ID:        uuid.New(),
			UserID:    &userID,
			Action:    domain.AuditActionCreate,
			Timestamp: now.Add(-1 * time.Hour),
		},
		{
			ID:        uuid.New(),
			UserID:    &userID,
			Action:    domain.AuditActionVerify,
			Timestamp: now.Add(-2 * time.Hour),
		},
	}

	mockAgentRepo.On("GetByOrganization", orgID).Return(agents, nil)
	mockAuditRepo.On("GetByOrganization", orgID, 1000, 0).Return(auditLogs, nil)

	service := NewComplianceService(mockAuditRepo, mockAgentRepo, nil, nil)

	startDate := now.Add(-7 * 24 * time.Hour)
	endDate := now

	report, err := service.GenerateComplianceReport(
		context.Background(),
		orgID,
		"standard",
		startDate,
		endDate,
	)

	assert.NoError(t, err)
	assert.NotNil(t, report)

	complianceReport := report.(*ComplianceReport)
	assert.Equal(t, orgID.String(), complianceReport.OrganizationID)
	assert.Equal(t, 2, complianceReport.Summary.TotalAgents)
	assert.Equal(t, 1, complianceReport.Summary.VerifiedAgents)
	assert.Equal(t, 1, complianceReport.Summary.PendingAgents)
	assert.Equal(t, 0.675, complianceReport.Summary.AverageTrustScore) // (0.85 + 0.50) / 2
	assert.Equal(t, 2, complianceReport.Summary.TotalAuditLogs)
	assert.Len(t, complianceReport.Agents, 2)

	mockAgentRepo.AssertExpectations(t)
	mockAuditRepo.AssertExpectations(t)
}

func TestComplianceService_GenerateComplianceReport_AgentRepoError(t *testing.T) {
	mockAgentRepo := new(SharedMockAgentRepository)
	mockAuditRepo := new(SharedMockAuditLogRepository)

	orgID := uuid.New()

	mockAgentRepo.On("GetByOrganization", orgID).Return(nil, assert.AnError)

	service := NewComplianceService(mockAuditRepo, mockAgentRepo, nil, nil)

	report, err := service.GenerateComplianceReport(
		context.Background(),
		orgID,
		"standard",
		time.Now().Add(-7*24*time.Hour),
		time.Now(),
	)

	assert.Error(t, err)
	assert.Nil(t, report)
	mockAgentRepo.AssertExpectations(t)
}

func TestComplianceService_GenerateComplianceReport_NoAgents(t *testing.T) {
	mockAgentRepo := new(SharedMockAgentRepository)
	mockAuditRepo := new(SharedMockAuditLogRepository)

	orgID := uuid.New()

	mockAgentRepo.On("GetByOrganization", orgID).Return([]*domain.Agent{}, nil)
	mockAuditRepo.On("GetByOrganization", orgID, 1000, 0).Return([]*domain.AuditLog{}, nil)

	service := NewComplianceService(mockAuditRepo, mockAgentRepo, nil, nil)

	report, err := service.GenerateComplianceReport(
		context.Background(),
		orgID,
		"standard",
		time.Now().Add(-7*24*time.Hour),
		time.Now(),
	)

	assert.NoError(t, err)
	assert.NotNil(t, report)

	complianceReport := report.(*ComplianceReport)
	assert.Equal(t, 0, complianceReport.Summary.TotalAgents)
	assert.Equal(t, 0.0, complianceReport.Summary.AverageTrustScore)

	mockAgentRepo.AssertExpectations(t)
	mockAuditRepo.AssertExpectations(t)
}

// ========================================
// mapRoleToAccessLevel Tests (method on ComplianceService)
// ========================================

func TestComplianceService_MapRoleToAccessLevel(t *testing.T) {
	service := &ComplianceService{}

	tests := []struct {
		name     string
		role     domain.UserRole
		expected string
	}{
		{"admin role", domain.RoleAdmin, "full_access"},
		{"manager role", domain.RoleManager, "elevated_access"},
		{"member role", domain.RoleMember, "standard_access"},
		{"viewer role", domain.RoleViewer, "read_only_access"},
		{"unknown role", domain.UserRole("unknown"), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.mapRoleToAccessLevel(tt.role)
			assert.Equal(t, tt.expected, result)
		})
	}
}
