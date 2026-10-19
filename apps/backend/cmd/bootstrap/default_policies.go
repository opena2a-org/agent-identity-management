package main

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

// defaultPolicy is one row runBootstrap adds to security_policies.
type defaultPolicy struct {
	Name              string
	Description       string
	PolicyType        string
	EnforcementAction string
	SeverityThreshold string
	Rules             string // a JSON object, stored as jsonb
	IsEnabled         bool
	Priority          int
}

// defaultPolicies are the security policies migrations
// 015_add_default_security_policies.sql and 052_add_default_mcp_policies.sql
// insert for the OpenA2A Admin organization, with the values the later
// migrations that change those rows leave them: 105 disables the MCP policies,
// 114 puts minTrustScore on the 0-1 trust scale and 130 adds logging and
// completions to the allowlist's capabilities. Both migrations insert only
// when the user admin@opena2a.org already exists. Migration 013 no longer
// creates that user; `aim-bootstrap --default` does, after the migrations have
// run, so on a fresh database they insert nothing and the --default run that
// creates the admin adds these instead.
//
// A migration that changes these rows has to change this table too:
// TestDefaultPoliciesMatchWhatTheMigrationsLeave applies the migrations for an
// organization whose admin exists and fails when its rows differ from these.
var defaultPolicies = []defaultPolicy{
	{
		Name:              "Low Trust Score Alert",
		Description:       "Alert when agent trust score falls below 70",
		PolicyType:        "trust_score_low",
		EnforcementAction: "alert_only",
		SeverityThreshold: "medium",
		Rules:             `{"trust_threshold": 0.7, "duration": "1h", "notify": ["admin"]}`,
		IsEnabled:         true,
		Priority:          100,
	},
	{
		Name:              "Capability Violation Detection",
		Description:       "Detect and alert when agents exceed their defined capabilities",
		PolicyType:        "capability_violation",
		EnforcementAction: "block_and_alert",
		SeverityThreshold: "high",
		Rules:             `{"check_permissions": true, "strict_mode": false}`,
		IsEnabled:         true,
		Priority:          200,
	},
	{
		Name:              "Unusual Activity Monitoring",
		Description:       "Monitor for unusual agent behavior patterns (API call spikes, unusual times)",
		PolicyType:        "unusual_activity",
		EnforcementAction: "alert_only",
		SeverityThreshold: "medium",
		Rules:             `{"api_rate_threshold": 1000, "time_window": "1h", "check_off_hours": true}`,
		IsEnabled:         true,
		Priority:          80,
	},
	{
		Name:              "Critical Trust Score Block",
		Description:       "Block agents with critically low trust scores (below 50)",
		PolicyType:        "trust_score_low",
		EnforcementAction: "block_and_alert",
		SeverityThreshold: "critical",
		Rules:             `{"trust_threshold": 0.5, "auto_disable": true, "notify": ["admin", "security"]}`,
		IsEnabled:         true,
		Priority:          300,
	},
	{
		Name:              "Failed Authentication Monitoring",
		Description:       "Alert on repeated authentication failures",
		PolicyType:        "auth_failure",
		EnforcementAction: "alert_only",
		SeverityThreshold: "medium",
		Rules:             `{"max_attempts": 5, "time_window": "15m", "lockout_duration": "30m"}`,
		IsEnabled:         true,
		Priority:          150,
	},
	{
		Name:              "Data Exfiltration Detection",
		Description:       "Detect potential data exfiltration through unusual data transfer patterns",
		PolicyType:        "data_exfiltration",
		EnforcementAction: "block_and_alert",
		SeverityThreshold: "high",
		Rules:             `{"data_threshold_mb": 100, "time_window": "1h", "check_destinations": true}`,
		IsEnabled:         true,
		Priority:          250,
	},
	{
		Name:              "Trusted MCP Server Domains",
		Description:       "Allow MCP servers only from trusted domains (localhost, internal networks, major cloud providers)",
		PolicyType:        "mcp_allowlist",
		EnforcementAction: "alert_only",
		SeverityThreshold: "medium",
		Rules:             `{"allowedDomains": ["localhost", "127.0.0.1", "*.local", "*.internal", "*.amazonaws.com", "*.azure.com", "*.googleapis.com"], "allowedNames": [], "allowedCapabilities": ["tools", "resources", "prompts", "logging", "completions"], "requireVerified": false, "minTrustScore": 0, "minAttestations": 0}`,
		IsEnabled:         false,
		Priority:          100,
	},
	{
		Name:              "Blocked MCP Domains",
		Description:       "Block MCP servers from known malicious or untrusted domains",
		PolicyType:        "mcp_blocklist",
		EnforcementAction: "block_and_alert",
		SeverityThreshold: "critical",
		Rules:             `{"blockedDomains": ["*.malware.com", "*.suspicious.io", "*.untrusted.net"], "blockedNames": ["test-malicious", "unsafe-mcp"], "blockedUrls": []}`,
		IsEnabled:         false,
		Priority:          50,
	},
	{
		Name:              "MCP Capability Requirements",
		Description:       "Ensure MCP servers declare required capabilities and do not expose dangerous ones",
		PolicyType:        "mcp_capabilities",
		EnforcementAction: "alert_only",
		SeverityThreshold: "medium",
		Rules:             `{"requiredCapabilities": [], "forbiddenCapabilities": ["system_exec", "file_delete", "network_admin"], "maxCapabilities": 20, "requireCapabilityDeclaration": true}`,
		IsEnabled:         false,
		Priority:          150,
	},
	{
		Name:              "Unverified MCP Server Restrictions",
		Description:       "Alert when agents connect to unverified MCP servers",
		PolicyType:        "mcp_unverified",
		EnforcementAction: "alert_only",
		SeverityThreshold: "low",
		Rules:             `{"action": "alert", "allowPendingVerification": true, "maxUnverifiedConnections": 5, "requireVerificationWithin": "7d", "notifyOnNewConnection": true}`,
		IsEnabled:         false,
		Priority:          200,
	},
	{
		Name:              "MCP Minimum Trust Score",
		Description:       "Alert when MCP servers fall below minimum trust score threshold",
		PolicyType:        "mcp_allowlist",
		EnforcementAction: "alert_only",
		SeverityThreshold: "medium",
		Rules:             `{"allowedDomains": ["*"], "allowedNames": [], "allowedCapabilities": [], "requireVerified": false, "minTrustScore": 0.5, "minAttestations": 1}`,
		IsEnabled:         false,
		Priority:          175,
	},
	{
		Name:              "High-Risk MCP Server Block",
		Description:       "Block connections to MCP servers with critically low trust scores",
		PolicyType:        "mcp_allowlist",
		EnforcementAction: "block_and_alert",
		SeverityThreshold: "critical",
		Rules:             `{"allowedDomains": ["*"], "allowedNames": [], "allowedCapabilities": [], "requireVerified": true, "minTrustScore": 0.3, "minAttestations": 2}`,
		IsEnabled:         false,
		Priority:          25,
	},
}

// insertDefaultPolicy adds one policy to an organization. A policy the
// organization already has under that name is left as it is: security_policies
// is unique on (organization_id, name).
const insertDefaultPolicy = `
	INSERT INTO security_policies (
		organization_id, name, description, policy_type, enforcement_action,
		severity_threshold, rules, applies_to, is_enabled, priority, created_by
	) VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, 'all', $8, $9, $10)
	ON CONFLICT (organization_id, name) DO NOTHING
`

// seedDefaultPolicies adds defaultPolicies to the organization orgID, created
// by adminID, in tx, and returns how many rows it added.
func seedDefaultPolicies(ctx context.Context, tx *sql.Tx, orgID, adminID uuid.UUID) (int64, error) {
	var added int64
	for _, p := range defaultPolicies {
		result, err := tx.ExecContext(ctx, insertDefaultPolicy,
			orgID, p.Name, p.Description, p.PolicyType, p.EnforcementAction,
			p.SeverityThreshold, p.Rules, p.IsEnabled, p.Priority, adminID)
		if err != nil {
			return added, fmt.Errorf("add policy %q: %w", p.Name, err)
		}
		n, err := result.RowsAffected()
		if err != nil {
			return added, fmt.Errorf("read the result of adding policy %q: %w", p.Name, err)
		}
		added += n
	}
	return added, nil
}
