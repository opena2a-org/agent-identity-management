package domain

import (
	"testing"
)

func TestDetectRiskLevel_ExplicitOverride(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		override   string
		expected   string
	}{
		{"Override low", "payment:process", "low", RiskLevelLow},
		{"Override medium", "weather:fetch", "medium", RiskLevelMedium},
		{"Override high", "api:read", "high", RiskLevelHigh},
		{"Override critical", "db:read", "critical", RiskLevelCritical},
		{"Invalid override falls through", "weather:fetch", "invalid", RiskLevelLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectRiskLevel(tt.capability, tt.override)
			if result != tt.expected {
				t.Errorf("DetectRiskLevel(%q, %q) = %q, want %q", tt.capability, tt.override, result, tt.expected)
			}
		})
	}
}

func TestDetectRiskLevel_SpecificCapabilityMap(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		expected   string
	}{
		// User operations
		{"user:delete is critical", "user:delete", RiskLevelCritical},
		{"user:impersonate is critical", "user:impersonate", RiskLevelCritical},
		{"user:admin is critical", "user:admin", RiskLevelCritical},

		// Auth operations
		{"auth:login is medium", "auth:login", RiskLevelMedium},
		{"auth:logout is low", "auth:logout", RiskLevelLow},
		{"auth:reset_password is high", "auth:reset_password", RiskLevelHigh},
		{"auth:change_password is high", "auth:change_password", RiskLevelHigh},

		// API calls
		{"api:call is low", "api:call", RiskLevelLow},
		{"api:webhook is medium", "api:webhook", RiskLevelMedium},
		{"api:external is high", "api:external", RiskLevelHigh},

		// Database
		{"db:read is low", "db:read", RiskLevelLow},
		{"db:query is low", "db:query", RiskLevelLow},
		{"db:write is medium", "db:write", RiskLevelMedium},
		{"db:delete is high", "db:delete", RiskLevelHigh},
		{"db:admin is critical", "db:admin", RiskLevelCritical},
		{"db:drop is critical", "db:drop", RiskLevelCritical},

		// File operations
		{"file:read is low", "file:read", RiskLevelLow},
		{"file:write is medium", "file:write", RiskLevelMedium},
		{"file:delete is high", "file:delete", RiskLevelHigh},
		{"file:execute is critical", "file:execute", RiskLevelCritical},

		// MCP operations
		{"mcp:tool_use is medium", "mcp:tool_use", RiskLevelMedium},
		{"mcp:resource_read is low", "mcp:resource_read", RiskLevelLow},

		// System operations
		{"system:read is medium", "system:read", RiskLevelMedium},
		{"system:exec is critical", "system:exec", RiskLevelCritical},
		{"system:admin is critical", "system:admin", RiskLevelCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectRiskLevel(tt.capability, "")
			if result != tt.expected {
				t.Errorf("DetectRiskLevel(%q, \"\") = %q, want %q", tt.capability, result, tt.expected)
			}
		})
	}
}

func TestDetectRiskLevel_ActionPatterns(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		expected   string
	}{
		// Low risk actions
		{"custom:read is low", "custom:read", RiskLevelLow},
		{"custom:fetch is low", "custom:fetch", RiskLevelLow},
		{"custom:get is low", "custom:get", RiskLevelLow},
		{"custom:list is low", "custom:list", RiskLevelLow},
		{"custom:search is low", "custom:search", RiskLevelLow},
		{"custom:query is low", "custom:query", RiskLevelLow},
		{"custom:view is low", "custom:view", RiskLevelLow},
		{"custom:check is low", "custom:check", RiskLevelLow},
		{"custom:validate is low", "custom:validate", RiskLevelLow},

		// Medium risk actions
		{"custom:write is medium", "custom:write", RiskLevelMedium},
		{"custom:update is medium", "custom:update", RiskLevelMedium},
		{"custom:create is medium", "custom:create", RiskLevelMedium},
		{"custom:modify is medium", "custom:modify", RiskLevelMedium},
		{"custom:edit is medium", "custom:edit", RiskLevelMedium},
		{"custom:set is medium", "custom:set", RiskLevelMedium},
		{"custom:save is medium", "custom:save", RiskLevelMedium},
		{"custom:upload is medium", "custom:upload", RiskLevelMedium},

		// High risk actions
		{"custom:delete is high", "custom:delete", RiskLevelHigh},
		{"custom:remove is high", "custom:remove", RiskLevelHigh},
		{"custom:send is high", "custom:send", RiskLevelHigh},
		{"custom:execute is high", "custom:execute", RiskLevelHigh},
		{"custom:run is high", "custom:run", RiskLevelHigh},
		{"custom:invoke is high", "custom:invoke", RiskLevelHigh},
		{"custom:export is high", "custom:export", RiskLevelHigh},
		{"custom:transfer is high", "custom:transfer", RiskLevelHigh},

		// Critical risk actions
		{"custom:process is critical", "custom:process", RiskLevelCritical},
		{"custom:refund is critical", "custom:refund", RiskLevelCritical},
		{"custom:charge is critical", "custom:charge", RiskLevelCritical},
		{"custom:pay is critical", "custom:pay", RiskLevelCritical},
		{"custom:approve is critical", "custom:approve", RiskLevelCritical},
		{"custom:admin is critical", "custom:admin", RiskLevelCritical},
		{"custom:impersonate is critical", "custom:impersonate", RiskLevelCritical},
		{"custom:destroy is critical", "custom:destroy", RiskLevelCritical},
		{"custom:purge is critical", "custom:purge", RiskLevelCritical},
		{"custom:wipe is critical", "custom:wipe", RiskLevelCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectRiskLevel(tt.capability, "")
			if result != tt.expected {
				t.Errorf("DetectRiskLevel(%q, \"\") = %q, want %q", tt.capability, result, tt.expected)
			}
		})
	}
}

func TestDetectRiskLevel_NamespacePatterns(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		expected   string
	}{
		// Critical risk namespaces
		{"payment:unknown uses namespace", "payment:unknown", RiskLevelCritical},
		{"admin:unknown uses namespace", "admin:unknown", RiskLevelCritical},
		{"system:unknown uses namespace", "system:unknown", RiskLevelCritical},
		{"billing:unknown uses namespace", "billing:unknown", RiskLevelCritical},
		{"finance:unknown uses namespace", "finance:unknown", RiskLevelCritical},

		// High risk namespaces
		{"email:unknown uses namespace", "email:unknown", RiskLevelHigh},
		{"notification:unknown uses namespace", "notification:unknown", RiskLevelHigh},
		{"sms:unknown uses namespace", "sms:unknown", RiskLevelHigh},
		{"user:unknown uses namespace", "user:unknown", RiskLevelHigh},
		{"auth:unknown uses namespace", "auth:unknown", RiskLevelHigh},
		{"secret:unknown uses namespace", "secret:unknown", RiskLevelHigh},
		{"credential:unknown uses namespace", "credential:unknown", RiskLevelHigh},

		// Medium risk namespaces
		{"db:unknown uses namespace", "db:unknown", RiskLevelMedium},
		{"database:unknown uses namespace", "database:unknown", RiskLevelMedium},
		{"file:unknown uses namespace", "file:unknown", RiskLevelMedium},
		{"storage:unknown uses namespace", "storage:unknown", RiskLevelMedium},
		{"cache:unknown uses namespace", "cache:unknown", RiskLevelMedium},

		// Low risk namespaces
		{"api:unknown uses namespace", "api:unknown", RiskLevelLow},
		{"weather:unknown uses namespace", "weather:unknown", RiskLevelLow},
		{"search:unknown uses namespace", "search:unknown", RiskLevelLow},
		{"geocode:unknown uses namespace", "geocode:unknown", RiskLevelLow},
		{"translate:unknown uses namespace", "translate:unknown", RiskLevelLow},
		{"time:unknown uses namespace", "time:unknown", RiskLevelLow},
		{"math:unknown uses namespace", "math:unknown", RiskLevelLow},
		{"util:unknown uses namespace", "util:unknown", RiskLevelLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectRiskLevel(tt.capability, "")
			if result != tt.expected {
				t.Errorf("DetectRiskLevel(%q, \"\") = %q, want %q", tt.capability, result, tt.expected)
			}
		})
	}
}

func TestDetectRiskLevel_CombinedActionAndNamespace(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		expected   string
	}{
		// Action + Namespace combination (should use higher risk)
		{"payment:read uses higher (namespace critical > action low)", "payment:read", RiskLevelCritical},
		{"admin:fetch uses higher (namespace critical > action low)", "admin:fetch", RiskLevelCritical},
		{"email:get uses higher (namespace high > action low)", "email:get", RiskLevelHigh},
		{"weather:delete uses higher (action high > namespace low)", "weather:delete", RiskLevelHigh},
		{"api:process uses higher (action critical > namespace low)", "api:process", RiskLevelCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectRiskLevel(tt.capability, "")
			if result != tt.expected {
				t.Errorf("DetectRiskLevel(%q, \"\") = %q, want %q", tt.capability, result, tt.expected)
			}
		})
	}
}

func TestDetectRiskLevel_CaseInsensitive(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		expected   string
	}{
		{"uppercase PAYMENT:PROCESS", "PAYMENT:PROCESS", RiskLevelCritical},
		{"mixed case Payment:Process", "Payment:Process", RiskLevelCritical},
		{"mixed case DB:Read", "DB:Read", RiskLevelLow},
		{"uppercase WEATHER:FETCH", "WEATHER:FETCH", RiskLevelLow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectRiskLevel(tt.capability, "")
			if result != tt.expected {
				t.Errorf("DetectRiskLevel(%q, \"\") = %q, want %q", tt.capability, result, tt.expected)
			}
		})
	}
}

func TestDetectRiskLevel_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		capability string
		expected   string
	}{
		{"empty capability defaults to medium", "", RiskLevelMedium},
		{"no colon defaults to medium", "justaction", RiskLevelMedium},
		{"whitespace trimmed", "  weather:fetch  ", RiskLevelLow},
		{"unknown namespace and action defaults to medium", "foo:bar", RiskLevelMedium},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectRiskLevel(tt.capability, "")
			if result != tt.expected {
				t.Errorf("DetectRiskLevel(%q, \"\") = %q, want %q", tt.capability, result, tt.expected)
			}
		})
	}
}

func TestExplainRiskDetection(t *testing.T) {
	tests := []struct {
		name           string
		capability     string
		expectedRisk   string
		expectedSource string
	}{
		{"specific mapping", "user:delete", RiskLevelCritical, "specific_capability_map"},
		{"action mapping", "custom:delete", RiskLevelHigh, "action_map"},
		{"namespace mapping", "payment:unknown", RiskLevelCritical, "namespace_map"},
		{"action and namespace", "payment:read", RiskLevelCritical, "action_and_namespace"},
		{"default", "foo:bar", RiskLevelMedium, "default"},
		{"invalid format", "nocolon", RiskLevelMedium, "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExplainRiskDetection(tt.capability)
			if result.RiskLevel != tt.expectedRisk {
				t.Errorf("ExplainRiskDetection(%q).RiskLevel = %q, want %q", tt.capability, result.RiskLevel, tt.expectedRisk)
			}
			if result.Source != tt.expectedSource {
				t.Errorf("ExplainRiskDetection(%q).Source = %q, want %q", tt.capability, result.Source, tt.expectedSource)
			}
		})
	}
}

func TestHigherRisk(t *testing.T) {
	tests := []struct {
		name     string
		risk1    string
		risk2    string
		expected string
	}{
		{"low vs medium", RiskLevelLow, RiskLevelMedium, RiskLevelMedium},
		{"medium vs low", RiskLevelMedium, RiskLevelLow, RiskLevelMedium},
		{"high vs low", RiskLevelHigh, RiskLevelLow, RiskLevelHigh},
		{"critical vs high", RiskLevelCritical, RiskLevelHigh, RiskLevelCritical},
		{"same level", RiskLevelMedium, RiskLevelMedium, RiskLevelMedium},
		{"low vs critical", RiskLevelLow, RiskLevelCritical, RiskLevelCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := higherRisk(tt.risk1, tt.risk2)
			if result != tt.expected {
				t.Errorf("higherRisk(%q, %q) = %q, want %q", tt.risk1, tt.risk2, result, tt.expected)
			}
		})
	}
}

func TestRiskLevelDescription(t *testing.T) {
	tests := []struct {
		riskLevel string
		contains  string
	}{
		{RiskLevelLow, "audit"},
		{RiskLevelMedium, "pattern analysis"},
		{RiskLevelHigh, "alerts"},
		{RiskLevelCritical, "admin approval"},
		{"invalid", "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.riskLevel, func(t *testing.T) {
			result := RiskLevelDescription(tt.riskLevel)
			if len(result) == 0 {
				t.Errorf("RiskLevelDescription(%q) returned empty string", tt.riskLevel)
			}
		})
	}
}
