package domain

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComplianceFramework_Constants(t *testing.T) {
	assert.Equal(t, ComplianceFramework("aim"), FrameworkAIM)
}

func TestEvidenceType_Constants(t *testing.T) {
	tests := []struct {
		constant EvidenceType
		expected string
	}{
		{EvidenceTypeAuditLog, "audit_log"},
		{EvidenceTypeConfiguration, "configuration"},
		{EvidenceTypeScreenshot, "screenshot"},
		{EvidenceTypeDocument, "document"},
		{EvidenceTypeAttestation, "attestation"},
		{EvidenceTypeSystemCheck, "system_check"},
		{EvidenceTypeAccessReview, "access_review"},
		{EvidenceTypeSecurityScan, "security_scan"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, EvidenceType(tt.expected), tt.constant)
		})
	}
}

func TestComplianceEvidence_Structure(t *testing.T) {
	now := time.Now()
	orgID := uuid.New()
	userID := uuid.New()
	validUntil := now.Add(30 * 24 * time.Hour)

	evidence := ComplianceEvidence{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Framework:      FrameworkAIM,
		CheckName:      "agent_verification",
		EvidenceType:   EvidenceTypeSystemCheck,
		Title:          "Agent Verification Check",
		Description:    "Automated verification of all active agents",
		Data: map[string]interface{}{
			"total_agents":    50,
			"verified_agents": 48,
			"pending_agents":  2,
		},
		FileURL:     "https://storage.example.com/evidence/scan-001.pdf",
		CollectedAt: now,
		CollectedBy: userID,
		IsAutomatic: true,
		ValidUntil:  &validUntil,
		CreatedAt:   now,
	}

	assert.NotEqual(t, uuid.Nil, evidence.ID)
	assert.Equal(t, orgID, evidence.OrganizationID)
	assert.Equal(t, FrameworkAIM, evidence.Framework)
	assert.Equal(t, "agent_verification", evidence.CheckName)
	assert.Equal(t, EvidenceTypeSystemCheck, evidence.EvidenceType)
	assert.True(t, evidence.IsAutomatic)
	assert.NotNil(t, evidence.ValidUntil)
}

func TestComplianceEvidence_Manual(t *testing.T) {
	now := time.Now()

	evidence := ComplianceEvidence{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Framework:      FrameworkAIM,
		CheckName:      "access_review",
		EvidenceType:   EvidenceTypeAccessReview,
		Title:          "Quarterly Access Review",
		Description:    "Manual review of user access permissions",
		Data: map[string]interface{}{
			"reviewed_by":   "security-team",
			"users_reviewed": 25,
		},
		CollectedAt: now,
		CollectedBy: uuid.New(),
		IsAutomatic: false,
		CreatedAt:   now,
	}

	assert.False(t, evidence.IsAutomatic)
	assert.Equal(t, EvidenceTypeAccessReview, evidence.EvidenceType)
}

func TestComplianceSnapshot_Structure(t *testing.T) {
	now := time.Now()
	orgID := uuid.New()

	snapshot := ComplianceSnapshot{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Framework:      FrameworkAIM,
		Score:          85.5,
		PassedChecks:   17,
		FailedChecks:   3,
		TotalChecks:    20,
		CheckResults: map[string]bool{
			"agent_verification": true,
			"mfa_enabled":        true,
			"api_key_rotation":   false,
			"audit_logging":      true,
		},
		SnapshotDate: now,
		CreatedAt:    now,
	}

	assert.NotEqual(t, uuid.Nil, snapshot.ID)
	assert.Equal(t, orgID, snapshot.OrganizationID)
	assert.Equal(t, 85.5, snapshot.Score)
	assert.Equal(t, 17, snapshot.PassedChecks)
	assert.Equal(t, 3, snapshot.FailedChecks)
	assert.Equal(t, 20, snapshot.TotalChecks)
	assert.Len(t, snapshot.CheckResults, 4)
	assert.True(t, snapshot.CheckResults["agent_verification"])
	assert.False(t, snapshot.CheckResults["api_key_rotation"])
}

func TestComplianceCheckResult_Structure(t *testing.T) {
	result := ComplianceCheckResult{
		CheckName:     "agent_trust_scores",
		Category:      "security",
		Passed:        false,
		Severity:      "high",
		Details:       "3 agents have trust scores below 60%",
		AffectedCount: 3,
		AffectedItems: []map[string]interface{}{
			{"agent_id": "agent-1", "trust_score": 45.0},
			{"agent_id": "agent-2", "trust_score": 52.0},
			{"agent_id": "agent-3", "trust_score": 58.0},
		},
		ActionURL:   "/admin/agents?filter=low-trust",
		LastChecked: time.Now(),
	}

	assert.Equal(t, "agent_trust_scores", result.CheckName)
	assert.Equal(t, "security", result.Category)
	assert.False(t, result.Passed)
	assert.Equal(t, "high", result.Severity)
	assert.Equal(t, 3, result.AffectedCount)
	assert.Len(t, result.AffectedItems, 3)
}

func TestComplianceCheckResult_Passed(t *testing.T) {
	result := ComplianceCheckResult{
		CheckName:     "mfa_enabled",
		Category:      "authentication",
		Passed:        true,
		Severity:      "critical",
		Details:       "All admin users have MFA enabled",
		AffectedCount: 0,
		LastChecked:   time.Now(),
	}

	assert.True(t, result.Passed)
	assert.Equal(t, 0, result.AffectedCount)
	assert.Empty(t, result.AffectedItems)
}

func TestExecutiveSummary_Structure(t *testing.T) {
	summary := ExecutiveSummary{
		OverallStatus:     "Needs Attention",
		ComplianceScore:   78.5,
		CriticalFindings:  1,
		HighFindings:      3,
		MediumFindings:    5,
		LowFindings:       8,
		TotalAgents:       50,
		VerifiedAgents:    47,
		AverageTrustScore: 82.3,
		KeyFindings: []string{
			"3 agents with low trust scores",
			"API key rotation overdue",
			"Pending capability requests need review",
		},
		ImmediateActions: []string{
			"Review and address low-trust agents",
			"Rotate expired API keys",
		},
		TrendDirection: "stable",
	}

	assert.Equal(t, "Needs Attention", summary.OverallStatus)
	assert.Equal(t, 78.5, summary.ComplianceScore)
	assert.Equal(t, 1, summary.CriticalFindings)
	assert.Len(t, summary.KeyFindings, 3)
	assert.Len(t, summary.ImmediateActions, 2)
	assert.Equal(t, "stable", summary.TrendDirection)
}

func TestAgentInventory_Structure(t *testing.T) {
	inventory := AgentInventory{
		TotalAgents:       50,
		VerifiedAgents:    45,
		PendingAgents:     3,
		SuspendedAgents:   1,
		RevokedAgents:     1,
		AverageTrustScore: 81.5,
		HighRiskAgents:    2,
		Agents: []AgentComplianceDetail{
			{
				ID:          "agent-1",
				Name:        "test-agent",
				DisplayName: "Test Agent",
				Type:        "service",
				Status:      "verified",
				TrustScore:  95.0,
				RiskLevel:   "low",
			},
		},
	}

	assert.Equal(t, 50, inventory.TotalAgents)
	assert.Equal(t, 45, inventory.VerifiedAgents)
	assert.Equal(t, 2, inventory.HighRiskAgents)
	assert.Len(t, inventory.Agents, 1)
}

func TestAgentComplianceDetail_Structure(t *testing.T) {
	detail := AgentComplianceDetail{
		ID:                   "agent-123",
		Name:                 "data-processor",
		DisplayName:          "Data Processor Agent",
		Type:                 "automation",
		Status:               "verified",
		TrustScore:           45.0,
		CapabilityViolations: 2,
		DaysInactive:         0,
		LastActivity:         time.Now().Format(time.RFC3339),
		CreatedAt:            time.Now().Add(-30 * 24 * time.Hour).Format(time.RFC3339),
		VerifiedAt:           time.Now().Add(-29 * 24 * time.Hour).Format(time.RFC3339),
		RiskLevel:            "high",
		ComplianceIssues: []string{
			"Low trust score (45%)",
			"Capability violations detected",
		},
	}

	assert.Equal(t, "data-processor", detail.Name)
	assert.Equal(t, 45.0, detail.TrustScore)
	assert.Equal(t, "high", detail.RiskLevel)
	assert.Len(t, detail.ComplianceIssues, 2)
}

func TestUserAccessReview_Structure(t *testing.T) {
	review := UserAccessReview{
		TotalUsers:    25,
		AdminUsers:    2,
		ManagerUsers:  5,
		MemberUsers:   15,
		ViewerUsers:   3,
		ActiveUsers:   20,
		InactiveUsers: 4,
		PendingUsers:  1,
		Users: []UserAccessDetail{
			{
				ID:             "user-1",
				Email:          "admin@example.com",
				Name:           "Admin User",
				Role:           "admin",
				Status:         "active",
				DaysSinceLogin: 0,
				AccessLevel:    "full",
				ReviewRequired: false,
			},
		},
	}

	assert.Equal(t, 25, review.TotalUsers)
	assert.Equal(t, 2, review.AdminUsers)
	assert.Equal(t, 4, review.InactiveUsers)
}

func TestUserAccessDetail_ReviewRequired(t *testing.T) {
	tests := []struct {
		name           string
		daysSinceLogin int
		reviewRequired bool
	}{
		{"active user", 5, false},
		{"recently inactive", 25, false},
		{"needs review", 35, true},
		{"very inactive", 90, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			detail := UserAccessDetail{
				ID:             uuid.New().String(),
				Email:          "test@example.com",
				DaysSinceLogin: tt.daysSinceLogin,
				ReviewRequired: tt.daysSinceLogin >= 30,
			}
			assert.Equal(t, tt.reviewRequired, detail.ReviewRequired)
		})
	}
}

func TestRiskDistribution_Structure(t *testing.T) {
	dist := RiskDistribution{
		LowRisk:      40,
		MediumRisk:   7,
		HighRisk:     2,
		CriticalRisk: 1,
	}

	total := dist.LowRisk + dist.MediumRisk + dist.HighRisk + dist.CriticalRisk
	assert.Equal(t, 50, total)
}

func TestRiskItem_Structure(t *testing.T) {
	item := RiskItem{
		Type:       "agent",
		ID:         "agent-456",
		Name:       "risky-agent",
		RiskLevel:  "critical",
		RiskScore:  92.5,
		Issue:      "Trust score dropped below 40%",
		Mitigation: "Review agent activity and consider suspension",
	}

	assert.Equal(t, "agent", item.Type)
	assert.Equal(t, "critical", item.RiskLevel)
	assert.Equal(t, 92.5, item.RiskScore)
}

func TestRecommendationItem_Structure(t *testing.T) {
	rec := RecommendationItem{
		Priority:    "high",
		Category:    "security",
		Title:       "Enable MFA for all admin accounts",
		Description: "Multi-factor authentication is not enabled for 2 admin accounts",
		Impact:      "Reduces risk of unauthorized administrative access",
		ActionURL:   "/admin/users?filter=no-mfa",
	}

	assert.Equal(t, "high", rec.Priority)
	assert.Equal(t, "security", rec.Category)
	assert.NotEmpty(t, rec.ActionURL)
}

func TestReportPeriod_Structure(t *testing.T) {
	now := time.Now()
	period := ReportPeriod{
		StartDate: now.Add(-30 * 24 * time.Hour),
		EndDate:   now,
	}

	assert.True(t, period.EndDate.After(period.StartDate))
	duration := period.EndDate.Sub(period.StartDate)
	assert.InDelta(t, 30*24*time.Hour, duration, float64(time.Hour))
}

func TestComplianceEvidence_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	evidence := ComplianceEvidence{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Framework:      FrameworkAIM,
		CheckName:      "test_check",
		EvidenceType:   EvidenceTypeAuditLog,
		Title:          "Test Evidence",
		Description:    "Test description",
		Data:           map[string]interface{}{"key": "value"},
		CollectedAt:    now,
		CollectedBy:    uuid.New(),
		IsAutomatic:    true,
		CreatedAt:      now,
	}

	// Serialize to JSON
	data, err := json.Marshal(evidence)
	require.NoError(t, err)

	// Deserialize back
	var restored ComplianceEvidence
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, evidence.ID, restored.ID)
	assert.Equal(t, evidence.Framework, restored.Framework)
	assert.Equal(t, evidence.EvidenceType, restored.EvidenceType)
	assert.Equal(t, evidence.IsAutomatic, restored.IsAutomatic)
}

func TestComplianceSnapshot_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)

	snapshot := ComplianceSnapshot{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Framework:      FrameworkAIM,
		Score:          90.0,
		PassedChecks:   18,
		FailedChecks:   2,
		TotalChecks:    20,
		CheckResults:   map[string]bool{"check1": true, "check2": false},
		SnapshotDate:   now,
		CreatedAt:      now,
	}

	// Serialize to JSON
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)

	// Deserialize back
	var restored ComplianceSnapshot
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify key fields
	assert.Equal(t, snapshot.ID, restored.ID)
	assert.Equal(t, snapshot.Score, restored.Score)
	assert.Equal(t, snapshot.PassedChecks, restored.PassedChecks)
}

func TestComplianceExportReport_Structure(t *testing.T) {
	now := time.Now()

	report := ComplianceExportReport{
		ID:               uuid.New(),
		OrganizationID:   uuid.New(),
		OrganizationName: "Test Organization",
		Framework:        FrameworkAIM,
		ReportPeriod: ReportPeriod{
			StartDate: now.Add(-30 * 24 * time.Hour),
			EndDate:   now,
		},
		GeneratedAt:      now,
		GeneratedBy:      uuid.New(),
		OverallScore:     85.0,
		ComplianceStatus: "partial",
		ExecutiveSummary: ExecutiveSummary{
			OverallStatus:   "Needs Attention",
			ComplianceScore: 85.0,
		},
		AgentInventory: AgentInventory{
			TotalAgents:    50,
			VerifiedAgents: 48,
		},
		UserAccessReview: UserAccessReview{
			TotalUsers: 25,
		},
		RiskAssessment: RiskAssessmentReport{
			OverallRiskLevel: "medium",
			RiskScore:        35.0,
		},
	}

	assert.Equal(t, "Test Organization", report.OrganizationName)
	assert.Equal(t, FrameworkAIM, report.Framework)
	assert.Equal(t, "partial", report.ComplianceStatus)
	assert.Equal(t, 85.0, report.OverallScore)
}

func TestSecurityAlertsReport_Structure(t *testing.T) {
	report := SecurityAlertsReport{
		TotalAlerts:         100,
		CriticalAlerts:      2,
		HighAlerts:          8,
		MediumAlerts:        30,
		LowAlerts:           60,
		UnacknowledgedCount: 15,
		AcknowledgedCount:   85,
		AlertsByType: map[string]int{
			"trust_score_low": 25,
			"auth_failure":    40,
			"api_key_expiring": 20,
			"unusual_activity": 15,
		},
	}

	total := report.CriticalAlerts + report.HighAlerts + report.MediumAlerts + report.LowAlerts
	assert.Equal(t, 100, total)
	assert.Equal(t, 15, report.UnacknowledgedCount)
}

func TestAuditActivityReport_Structure(t *testing.T) {
	report := AuditActivityReport{
		TotalActions:   5000,
		UniqueUsers:    25,
		ActionsLast24h: 150,
		ActionsLast7d:  1000,
		ActionsLast30d: 4500,
		ActionBreakdown: map[string]int{
			"create": 1000,
			"update": 2500,
			"delete": 500,
			"read":   1000,
		},
		ResourceBreakdown: map[string]int{
			"agent":   2000,
			"user":    1500,
			"api_key": 1000,
			"webhook": 500,
		},
	}

	assert.Equal(t, 5000, report.TotalActions)
	assert.Equal(t, 25, report.UniqueUsers)
	assert.Len(t, report.ActionBreakdown, 4)
}
