package domain

import (
	"strings"
)

// Default risk level when no pattern matches
const DefaultRiskLevel = RiskLevelMedium

// NamespaceRiskMap maps capability namespaces to their inherent risk levels
// More specific namespaces that indicate inherent risk
var NamespaceRiskMap = map[string]string{
	// Critical risk namespaces - financial, admin, system
	"payment":    RiskLevelCritical,
	"admin":      RiskLevelCritical,
	"system":     RiskLevelCritical,
	"billing":    RiskLevelCritical,
	"finance":    RiskLevelCritical,

	// High risk namespaces - user data, external comms
	"email":        RiskLevelHigh,
	"notification": RiskLevelHigh,
	"sms":          RiskLevelHigh,
	"user":         RiskLevelHigh, // Default for user operations
	"auth":         RiskLevelHigh,
	"secret":       RiskLevelHigh,
	"credential":   RiskLevelHigh,

	// Medium risk namespaces - data access, file system
	"db":       RiskLevelMedium,
	"database": RiskLevelMedium,
	"file":     RiskLevelMedium,
	"storage":  RiskLevelMedium,
	"cache":    RiskLevelMedium,

	// Low risk namespaces - public data, read-only services
	"api":       RiskLevelLow,
	"weather":   RiskLevelLow,
	"search":    RiskLevelLow,
	"geocode":   RiskLevelLow,
	"translate": RiskLevelLow,
	"time":      RiskLevelLow,
	"math":      RiskLevelLow,
	"util":      RiskLevelLow,
}

// ActionRiskMap maps capability actions to their inherent risk levels
// Actions that indicate inherent risk regardless of namespace
var ActionRiskMap = map[string]string{
	// Low risk actions - read-only, no side effects
	"read":     RiskLevelLow,
	"fetch":    RiskLevelLow,
	"get":      RiskLevelLow,
	"list":     RiskLevelLow,
	"search":   RiskLevelLow,
	"query":    RiskLevelLow,
	"view":     RiskLevelLow,
	"check":    RiskLevelLow,
	"validate": RiskLevelLow,
	"verify":   RiskLevelLow,
	"count":    RiskLevelLow,
	"exists":   RiskLevelLow,

	// Medium risk actions - modifications, creates
	"write":  RiskLevelMedium,
	"update": RiskLevelMedium,
	"create": RiskLevelMedium,
	"modify": RiskLevelMedium,
	"edit":   RiskLevelMedium,
	"set":    RiskLevelMedium,
	"save":   RiskLevelMedium,
	"upload": RiskLevelMedium,
	"import": RiskLevelMedium,

	// High risk actions - destructive, external comms, sensitive
	"delete":   RiskLevelHigh,
	"remove":   RiskLevelHigh,
	"send":     RiskLevelHigh,
	"execute":  RiskLevelHigh,
	"run":      RiskLevelHigh,
	"invoke":   RiskLevelHigh,
	"call":     RiskLevelHigh, // External calls
	"export":   RiskLevelHigh,
	"download": RiskLevelHigh,
	"transfer": RiskLevelHigh,
	"move":     RiskLevelHigh,
	"revoke":   RiskLevelHigh,
	"suspend":  RiskLevelHigh,
	"disable":  RiskLevelHigh,
	"block":    RiskLevelHigh,

	// Critical risk actions - irreversible, financial, admin
	"process":     RiskLevelCritical, // Usually financial
	"refund":      RiskLevelCritical,
	"charge":      RiskLevelCritical,
	"pay":         RiskLevelCritical,
	"approve":     RiskLevelCritical,
	"admin":       RiskLevelCritical,
	"impersonate": RiskLevelCritical,
	"escalate":    RiskLevelCritical,
	"purge":       RiskLevelCritical,
	"destroy":     RiskLevelCritical,
	"drop":        RiskLevelCritical,
	"truncate":    RiskLevelCritical,
	"wipe":        RiskLevelCritical,
	"terminate":   RiskLevelCritical,
}

// SpecificCapabilityMap maps specific capabilities to their risk levels
// These take highest precedence over namespace/action mappings
var SpecificCapabilityMap = map[string]string{
	// User operations - escalate delete/impersonate to critical
	"user:delete":      RiskLevelCritical,
	"user:impersonate": RiskLevelCritical,
	"user:admin":       RiskLevelCritical,

	// Auth operations - sensitive
	"auth:login":           RiskLevelMedium,
	"auth:logout":          RiskLevelLow,
	"auth:reset_password":  RiskLevelHigh,
	"auth:change_password": RiskLevelHigh,

	// API calls - generally low unless external
	"api:call":     RiskLevelLow,
	"api:webhook":  RiskLevelMedium,
	"api:external": RiskLevelHigh,

	// Database - read is low, others escalate
	"db:read":  RiskLevelLow,
	"db:query": RiskLevelLow,
	"db:write": RiskLevelMedium,
	"db:delete": RiskLevelHigh,
	"db:admin": RiskLevelCritical,
	"db:drop":  RiskLevelCritical,

	// File operations
	"file:read":    RiskLevelLow,
	"file:write":   RiskLevelMedium,
	"file:delete":  RiskLevelHigh,
	"file:execute": RiskLevelCritical,

	// Network operations
	"network:read":     RiskLevelLow,
	"network:connect":  RiskLevelMedium,
	"network:listen":   RiskLevelHigh,
	"network:external": RiskLevelHigh,

	// MCP operations
	"mcp:tool_use":      RiskLevelMedium,
	"mcp:resource_read": RiskLevelLow,

	// System operations - all high/critical
	"system:read":  RiskLevelMedium,
	"system:exec":  RiskLevelCritical,
	"system:admin": RiskLevelCritical,
}

// DetectRiskLevel automatically detects the risk level of a capability based on its name.
//
// The detection follows this priority order:
// 1. Explicit override (if provided and non-empty)
// 2. Specific capability mapping (e.g., "user:delete" -> critical)
// 3. Action-based mapping (e.g., ":delete" -> high)
// 4. Namespace-based mapping (e.g., "payment:" -> critical)
// 5. Default to "medium"
func DetectRiskLevel(capability string, override string) string {
	// Priority 1: Explicit override always wins
	if override != "" {
		if isValidRiskLevel(override) {
			return override
		}
		// Invalid override, fall through to detection
	}

	// Normalize capability to lowercase
	capability = strings.ToLower(strings.TrimSpace(capability))

	// Priority 2: Check specific capability map
	if risk, ok := SpecificCapabilityMap[capability]; ok {
		return risk
	}

	// Parse namespace and action
	if !strings.Contains(capability, ":") {
		return DefaultRiskLevel
	}

	parts := strings.SplitN(capability, ":", 2)
	if len(parts) != 2 {
		return DefaultRiskLevel
	}

	namespace := parts[0]
	action := parts[1]

	// Priority 3: Check action-based mapping (actions are more specific)
	if actionRisk, ok := ActionRiskMap[action]; ok {
		// If namespace has higher inherent risk, use that instead
		if namespaceRisk, nsOk := NamespaceRiskMap[namespace]; nsOk {
			return higherRisk(actionRisk, namespaceRisk)
		}
		return actionRisk
	}

	// Priority 4: Check namespace-based mapping
	if namespaceRisk, ok := NamespaceRiskMap[namespace]; ok {
		return namespaceRisk
	}

	// Priority 5: Default
	return DefaultRiskLevel
}

// DetectRiskLevelWithoutOverride detects risk level from capability only
func DetectRiskLevelWithoutOverride(capability string) string {
	return DetectRiskLevel(capability, "")
}

// isValidRiskLevel checks if a risk level string is valid
func isValidRiskLevel(level string) bool {
	switch level {
	case RiskLevelLow, RiskLevelMedium, RiskLevelHigh, RiskLevelCritical:
		return true
	default:
		return false
	}
}

// higherRisk returns the higher of two risk levels
func higherRisk(risk1, risk2 string) string {
	riskOrder := map[string]int{
		RiskLevelLow:      0,
		RiskLevelMedium:   1,
		RiskLevelHigh:     2,
		RiskLevelCritical: 3,
	}

	r1 := riskOrder[risk1]
	r2 := riskOrder[risk2]

	if r1 >= r2 {
		return risk1
	}
	return risk2
}

// RiskLevelDescription returns a human-readable description of a risk level
func RiskLevelDescription(riskLevel string) string {
	descriptions := map[string]string{
		RiskLevelLow:      "Logged for audit trail. Minimal monitoring.",
		RiskLevelMedium:   "Logged with pattern analysis. Monitored for unusual behavior.",
		RiskLevelHigh:     "Logged with alerts. Closely monitored. May affect trust score.",
		RiskLevelCritical: "Logged with alerts. May require admin approval (JIT access).",
	}

	if desc, ok := descriptions[riskLevel]; ok {
		return desc
	}
	return "Unknown risk level"
}

// RiskDetectionResult contains the result of risk detection with explanation
type RiskDetectionResult struct {
	Capability string `json:"capability"`
	RiskLevel  string `json:"riskLevel"`
	Reason     string `json:"reason"`
	Source     string `json:"source"`
}

// ExplainRiskDetection explains why a capability was assigned a particular risk level
func ExplainRiskDetection(capability string) RiskDetectionResult {
	capability = strings.ToLower(strings.TrimSpace(capability))

	// Check specific mapping
	if risk, ok := SpecificCapabilityMap[capability]; ok {
		return RiskDetectionResult{
			Capability: capability,
			RiskLevel:  risk,
			Reason:     "Specific mapping for '" + capability + "'",
			Source:     "specific_capability_map",
		}
	}

	if !strings.Contains(capability, ":") {
		return RiskDetectionResult{
			Capability: capability,
			RiskLevel:  DefaultRiskLevel,
			Reason:     "Invalid capability format (missing namespace:action)",
			Source:     "default",
		}
	}

	parts := strings.SplitN(capability, ":", 2)
	if len(parts) != 2 {
		return RiskDetectionResult{
			Capability: capability,
			RiskLevel:  DefaultRiskLevel,
			Reason:     "Invalid capability format",
			Source:     "default",
		}
	}

	namespace := parts[0]
	action := parts[1]

	// Check action mapping
	if actionRisk, ok := ActionRiskMap[action]; ok {
		if namespaceRisk, nsOk := NamespaceRiskMap[namespace]; nsOk {
			finalRisk := higherRisk(actionRisk, namespaceRisk)
			return RiskDetectionResult{
				Capability: capability,
				RiskLevel:  finalRisk,
				Reason:     "Action '" + action + "' (" + actionRisk + ") combined with namespace '" + namespace + "' (" + namespaceRisk + ")",
				Source:     "action_and_namespace",
			}
		}
		return RiskDetectionResult{
			Capability: capability,
			RiskLevel:  actionRisk,
			Reason:     "Action '" + action + "' maps to '" + actionRisk + "' risk",
			Source:     "action_map",
		}
	}

	// Check namespace mapping
	if namespaceRisk, ok := NamespaceRiskMap[namespace]; ok {
		return RiskDetectionResult{
			Capability: capability,
			RiskLevel:  namespaceRisk,
			Reason:     "Namespace '" + namespace + "' maps to '" + namespaceRisk + "' risk",
			Source:     "namespace_map",
		}
	}

	return RiskDetectionResult{
		Capability: capability,
		RiskLevel:  DefaultRiskLevel,
		Reason:     "No matching pattern found, using default '" + DefaultRiskLevel + "'",
		Source:     "default",
	}
}
