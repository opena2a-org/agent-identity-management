package domain

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateCapabilityFormat(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		wantErr    bool
		errMsg     string
	}{
		// Valid capabilities
		{name: "valid file:read", capability: "file:read", wantErr: false},
		{name: "valid api:call", capability: "api:call", wantErr: false},
		{name: "valid db:query", capability: "db:query", wantErr: false},
		{name: "valid with underscore", capability: "payment:process_refund", wantErr: false},
		{name: "valid with numbers", capability: "api2:call3", wantErr: false},
		{name: "valid custom namespace", capability: "myapp:custom_action", wantErr: false},

		// Invalid capabilities - empty
		{name: "empty string", capability: "", wantErr: true, errMsg: "cannot be empty"},

		// Invalid capabilities - colon issues
		{name: "no colon", capability: "fileread", wantErr: true, errMsg: "must match format"},
		{name: "multiple colons", capability: "file:read:write", wantErr: true, errMsg: "must match format"},
		{name: "only colon", capability: ":", wantErr: true, errMsg: "namespace cannot be empty"},

		// Invalid capabilities - namespace issues
		{name: "empty namespace", capability: ":read", wantErr: true, errMsg: "namespace cannot be empty"},
		{name: "uppercase namespace", capability: "File:read", wantErr: true, errMsg: "must start with a lowercase letter"},
		{name: "namespace starts with number", capability: "1file:read", wantErr: true, errMsg: "must start with a lowercase letter"},
		{name: "namespace with special char", capability: "file-name:read", wantErr: true, errMsg: "lowercase letters and numbers"},

		// Invalid capabilities - action issues
		{name: "empty action", capability: "file:", wantErr: true, errMsg: "action cannot be empty"},
		{name: "uppercase action", capability: "file:Read", wantErr: true, errMsg: "must start with a lowercase letter"},
		{name: "action starts with number", capability: "file:1read", wantErr: true, errMsg: "must start with a lowercase letter"},
		{name: "action starts with underscore", capability: "file:_read", wantErr: true, errMsg: "must start with a lowercase letter"},
		{name: "action with dash", capability: "file:read-write", wantErr: true, errMsg: "lowercase letters, numbers, and underscores"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCapabilityFormat(tt.capability)
			if tt.wantErr {
				require.Error(t, err)
				capErr, ok := err.(*CapabilityError)
				require.True(t, ok, "error should be *CapabilityError")
				assert.Contains(t, capErr.Error(), tt.errMsg)
				assert.Equal(t, tt.capability, capErr.Capability)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseCapability(t *testing.T) {
	tests := []struct {
		name          string
		capability    string
		wantNamespace string
		wantAction    string
		wantErr       bool
	}{
		{name: "file:read", capability: "file:read", wantNamespace: "file", wantAction: "read", wantErr: false},
		{name: "api:call", capability: "api:call", wantNamespace: "api", wantAction: "call", wantErr: false},
		{name: "payment:process_refund", capability: "payment:process_refund", wantNamespace: "payment", wantAction: "process_refund", wantErr: false},
		{name: "invalid format", capability: "invalid", wantErr: true},
		{name: "empty string", capability: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace, action, err := ParseCapability(tt.capability)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantNamespace, namespace)
				assert.Equal(t, tt.wantAction, action)
			}
		})
	}
}

func TestIsReservedNamespace(t *testing.T) {
	tests := []struct {
		namespace string
		reserved  bool
	}{
		{"file", true},
		{"db", true},
		{"api", true},
		{"network", true},
		{"system", true},
		{"user", true},
		{"mcp", true},
		{"data", true},
		{"myapp", false},
		{"custom", false},
		{"payment", false},
		{"inventory", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.namespace, func(t *testing.T) {
			result := IsReservedNamespace(tt.namespace)
			assert.Equal(t, tt.reserved, result)
		})
	}
}

func TestCapabilityDefinitionType(t *testing.T) {
	tests := []struct {
		namespace string
		action    string
		expected  string
	}{
		{"file", "read", "file:read"},
		{"api", "call", "api:call"},
		{"payment", "process", "payment:process"},
		{"", "action", ":action"},
		{"namespace", "", "namespace:"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			def := CapabilityDefinition{
				Namespace: tt.namespace,
				Action:    tt.action,
			}
			assert.Equal(t, tt.expected, def.Type())
		})
	}
}

func TestCapabilityErrorInterface(t *testing.T) {
	err := &CapabilityError{
		Capability: "invalid",
		Message:    "test error message",
	}

	assert.Equal(t, "test error message", err.Error())
	assert.Equal(t, "invalid", err.Capability)
}

func TestStandardCapabilityConstants(t *testing.T) {
	// Verify standard capability constants are properly formatted
	capabilities := []string{
		CapabilityFileRead,
		CapabilityFileWrite,
		CapabilityFileDelete,
		CapabilityFileExecute,
		CapabilityNetworkAccess,
		CapabilityNetworkConnect,
		CapabilityNetworkListen,
		CapabilityAPICall,
		CapabilityAPIWebhook,
		CapabilityDBRead,
		CapabilityDBQuery,
		CapabilityDBWrite,
		CapabilityDBAdmin,
		CapabilityUserRead,
		CapabilityUserWrite,
		CapabilityUserImpersonate,
		CapabilityDataExport,
		CapabilityDataEncrypt,
		CapabilitySystemExec,
		CapabilitySystemAdmin,
		CapabilityMCPToolUse,
		CapabilityMCPResourceRead,
	}

	for _, cap := range capabilities {
		t.Run(cap, func(t *testing.T) {
			err := ValidateCapabilityFormat(cap)
			assert.NoError(t, err, "Standard capability %s should be valid", cap)
		})
	}
}

func TestViolationSeverityConstants(t *testing.T) {
	assert.Equal(t, "low", ViolationSeverityLow)
	assert.Equal(t, "medium", ViolationSeverityMedium)
	assert.Equal(t, "high", ViolationSeverityHigh)
	assert.Equal(t, "critical", ViolationSeverityCritical)
}

func TestRiskLevelConstants(t *testing.T) {
	assert.Equal(t, "low", RiskLevelLow)
	assert.Equal(t, "medium", RiskLevelMedium)
	assert.Equal(t, "high", RiskLevelHigh)
	assert.Equal(t, "critical", RiskLevelCritical)
}

func TestAgentCapabilityStruct(t *testing.T) {
	agentID := uuid.New()
	grantedBy := uuid.New()

	cap := AgentCapability{
		ID:             uuid.New(),
		AgentID:        agentID,
		CapabilityType: "file:read",
		GrantedBy:      &grantedBy,
	}

	assert.Equal(t, agentID, cap.AgentID)
	assert.Equal(t, "file:read", cap.CapabilityType)
	assert.Equal(t, &grantedBy, cap.GrantedBy)
}

func TestCapabilityViolationStruct(t *testing.T) {
	agentID := uuid.New()
	agentName := "test-agent"

	violation := CapabilityViolation{
		ID:                  uuid.New(),
		AgentID:             agentID,
		AgentName:           &agentName,
		AttemptedCapability: "system:admin",
		Severity:            ViolationSeverityHigh,
		TrustScoreImpact:    10,
		IsBlocked:           true,
	}

	assert.Equal(t, agentID, violation.AgentID)
	assert.Equal(t, "system:admin", violation.AttemptedCapability)
	assert.Equal(t, ViolationSeverityHigh, violation.Severity)
	assert.Equal(t, 10, violation.TrustScoreImpact)
	assert.True(t, violation.IsBlocked)
}

func TestDefaultMinTrustScoreForRiskLevel(t *testing.T) {
	assert.Equal(t, 0.0, DefaultMinTrustScoreForRiskLevel(RiskLevelLow))
	assert.Equal(t, 0.3, DefaultMinTrustScoreForRiskLevel(RiskLevelMedium))
	assert.Equal(t, 0.5, DefaultMinTrustScoreForRiskLevel(RiskLevelHigh))
	assert.Equal(t, 0.7, DefaultMinTrustScoreForRiskLevel(RiskLevelCritical))
	assert.Equal(t, 0.0, DefaultMinTrustScoreForRiskLevel("unknown"))
}

func TestDefaultExecutionModeForRiskLevel(t *testing.T) {
	assert.Equal(t, ExecutionModeAuto, DefaultExecutionModeForRiskLevel(RiskLevelLow))
	assert.Equal(t, ExecutionModeAuto, DefaultExecutionModeForRiskLevel(RiskLevelMedium))
	assert.Equal(t, ExecutionModeNotify, DefaultExecutionModeForRiskLevel(RiskLevelHigh))
	assert.Equal(t, ExecutionModeReview, DefaultExecutionModeForRiskLevel(RiskLevelCritical))
	assert.Equal(t, ExecutionModeAuto, DefaultExecutionModeForRiskLevel("unknown"))
}

func TestCapabilityDefinition_MinTrustScore(t *testing.T) {
	def := CapabilityDefinition{
		Namespace:     "system",
		Action:        "admin",
		RiskLevel:     RiskLevelCritical,
		MinTrustScore: 0.7,
	}
	assert.Equal(t, 0.7, def.MinTrustScore)
	assert.Equal(t, "system:admin", def.Type())
}

func TestAgentCapability_ExecutionMode(t *testing.T) {
	cap := AgentCapability{
		CapabilityType: "file:read",
		ExecutionMode:  ExecutionModeAuto,
	}
	assert.Equal(t, ExecutionModeAuto, cap.ExecutionMode)

	cap.ExecutionMode = ExecutionModeReview
	assert.Equal(t, ExecutionModeReview, cap.ExecutionMode)
}
