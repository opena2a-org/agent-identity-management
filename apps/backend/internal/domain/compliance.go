package domain

import (
	"time"

	"github.com/google/uuid"
)

// ComplianceFramework represents a compliance framework
type ComplianceFramework string

const (
	FrameworkAIM ComplianceFramework = "aim" // AIM-specific compliance
)

// EvidenceType represents the type of compliance evidence
type EvidenceType string

const (
	EvidenceTypeAuditLog       EvidenceType = "audit_log"
	EvidenceTypeConfiguration  EvidenceType = "configuration"
	EvidenceTypeScreenshot     EvidenceType = "screenshot"
	EvidenceTypeDocument       EvidenceType = "document"
	EvidenceTypeAttestation    EvidenceType = "attestation"
	EvidenceTypeSystemCheck    EvidenceType = "system_check"
	EvidenceTypeAccessReview   EvidenceType = "access_review"
	EvidenceTypeSecurityScan   EvidenceType = "security_scan"
)

// ComplianceEvidence represents evidence collected for compliance audits
type ComplianceEvidence struct {
	ID              uuid.UUID              `json:"id"`
	OrganizationID  uuid.UUID              `json:"organizationId"`
	Framework       ComplianceFramework    `json:"framework"`
	CheckName       string                 `json:"checkName"`       // Which compliance check this evidence supports
	EvidenceType    EvidenceType           `json:"evidenceType"`
	Title           string                 `json:"title"`
	Description     string                 `json:"description"`
	Data            map[string]interface{} `json:"data"`            // Structured evidence data
	FileURL         string                 `json:"fileUrl,omitempty"` // Optional file attachment
	CollectedAt     time.Time              `json:"collectedAt"`
	CollectedBy     uuid.UUID              `json:"collectedBy"`     // User or system that collected
	IsAutomatic     bool                   `json:"isAutomatic"`     // Auto-collected vs manual
	ValidUntil      *time.Time             `json:"validUntil,omitempty"` // Evidence expiration
	CreatedAt       time.Time              `json:"createdAt"`
}

// ComplianceSnapshot represents a point-in-time compliance score
type ComplianceSnapshot struct {
	ID              uuid.UUID           `json:"id"`
	OrganizationID  uuid.UUID           `json:"organizationId"`
	Framework       ComplianceFramework `json:"framework"`
	Score           float64             `json:"score"`           // 0-100
	PassedChecks    int                 `json:"passedChecks"`
	FailedChecks    int                 `json:"failedChecks"`
	TotalChecks     int                 `json:"totalChecks"`
	CheckResults    map[string]bool     `json:"checkResults"`    // Individual check results
	SnapshotDate    time.Time           `json:"snapshotDate"`
	CreatedAt       time.Time           `json:"createdAt"`
}

// ComplianceEvidenceRepository defines the interface for evidence persistence
type ComplianceEvidenceRepository interface {
	Create(evidence *ComplianceEvidence) error
	GetByID(id uuid.UUID) (*ComplianceEvidence, error)
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*ComplianceEvidence, error)
	GetByFramework(orgID uuid.UUID, framework ComplianceFramework) ([]*ComplianceEvidence, error)
	GetByCheckName(orgID uuid.UUID, checkName string) ([]*ComplianceEvidence, error)
	GetRecent(orgID uuid.UUID, since time.Time) ([]*ComplianceEvidence, error)
	Delete(id uuid.UUID) error
	DeleteExpired(orgID uuid.UUID) (int, error)
}

// ComplianceSnapshotRepository defines the interface for snapshot persistence
type ComplianceSnapshotRepository interface {
	Create(snapshot *ComplianceSnapshot) error
	GetByID(id uuid.UUID) (*ComplianceSnapshot, error)
	GetByOrganization(orgID uuid.UUID, limit, offset int) ([]*ComplianceSnapshot, error)
	GetByFramework(orgID uuid.UUID, framework ComplianceFramework, limit int) ([]*ComplianceSnapshot, error)
	GetTrending(orgID uuid.UUID, framework ComplianceFramework, startDate, endDate time.Time) ([]*ComplianceSnapshot, error)
	GetLatest(orgID uuid.UUID, framework ComplianceFramework) (*ComplianceSnapshot, error)
	DeleteOlderThan(orgID uuid.UUID, before time.Time) (int, error)
}

// ComplianceCheckResult represents a detailed compliance check result
type ComplianceCheckResult struct {
	CheckName       string                 `json:"checkName"`
	Category        string                 `json:"category"`
	Passed          bool                   `json:"passed"`
	Severity        string                 `json:"severity"`        // critical, high, medium, low
	Details         string                 `json:"details"`
	AffectedCount   int                    `json:"affectedCount"`
	AffectedItems   []map[string]interface{} `json:"affectedItems,omitempty"`
	ActionURL       string                 `json:"actionUrl,omitempty"`
	Evidence        []ComplianceEvidence   `json:"evidence,omitempty"`
	LastChecked     time.Time              `json:"lastChecked"`
}

// ComplianceReport represents a comprehensive compliance report for export
type ComplianceExportReport struct {
	ID               uuid.UUID                `json:"id"`
	OrganizationID   uuid.UUID                `json:"organizationId"`
	OrganizationName string                   `json:"organizationName"`
	Framework        ComplianceFramework      `json:"framework"`
	ReportPeriod     ReportPeriod             `json:"reportPeriod"`
	GeneratedAt      time.Time                `json:"generatedAt"`
	GeneratedBy      uuid.UUID                `json:"generatedBy"`

	// Executive Summary
	ExecutiveSummary ExecutiveSummary         `json:"executiveSummary"`

	// Summary
	OverallScore     float64                  `json:"overallScore"`
	ComplianceStatus string                   `json:"complianceStatus"` // compliant, partial, non_compliant

	// Detailed Results
	CheckResults     []ComplianceCheckResult  `json:"checkResults"`

	// Agent Inventory
	AgentInventory   AgentInventory           `json:"agentInventory"`

	// User Access Review
	UserAccessReview UserAccessReview         `json:"userAccessReview"`

	// Audit Activity
	AuditActivity    AuditActivityReport      `json:"auditActivity"`

	// Security Alerts
	SecurityAlerts   SecurityAlertsReport     `json:"securityAlerts"`

	// Risk Assessment
	RiskAssessment   RiskAssessmentReport     `json:"riskAssessment"`

	// Recommendations
	Recommendations  []RecommendationItem     `json:"recommendations"`

	// Evidence
	EvidenceItems    []ComplianceEvidence     `json:"evidenceItems"`

	// Trending
	ScoreTrend       []ComplianceSnapshot     `json:"scoreTrend"`

	// Metadata
	Metadata         map[string]interface{}   `json:"metadata"`
}

// ExecutiveSummary provides a high-level overview for executives
type ExecutiveSummary struct {
	OverallStatus       string   `json:"overallStatus"`       // "Compliant", "Needs Attention", "Non-Compliant"
	ComplianceScore     float64  `json:"complianceScore"`     // 0-100
	CriticalFindings    int      `json:"criticalFindings"`    // Count of critical issues
	HighFindings        int      `json:"highFindings"`        // Count of high-severity issues
	MediumFindings      int      `json:"mediumFindings"`      // Count of medium-severity issues
	LowFindings         int      `json:"lowFindings"`         // Count of low-severity issues
	TotalAgents         int      `json:"totalAgents"`
	VerifiedAgents      int      `json:"verifiedAgents"`
	AverageTrustScore   float64  `json:"averageTrustScore"`   // 0-100
	KeyFindings         []string `json:"keyFindings"`         // Top 5 key findings
	ImmediateActions    []string `json:"immediateActions"`    // Top 3 immediate actions needed
	TrendDirection      string   `json:"trendDirection"`      // "improving", "stable", "declining"
}

// AgentInventory provides detailed agent compliance information
type AgentInventory struct {
	TotalAgents      int                     `json:"totalAgents"`
	VerifiedAgents   int                     `json:"verifiedAgents"`
	PendingAgents    int                     `json:"pendingAgents"`
	SuspendedAgents  int                     `json:"suspendedAgents"`
	RevokedAgents    int                     `json:"revokedAgents"`
	AverageTrustScore float64                `json:"averageTrustScore"`
	HighRiskAgents   int                     `json:"highRiskAgents"`   // Trust score < 60%
	Agents           []AgentComplianceDetail `json:"agents"`
}

// AgentComplianceDetail provides detailed compliance info for a single agent
type AgentComplianceDetail struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	DisplayName           string    `json:"displayName"`
	Type                  string    `json:"type"`
	Status                string    `json:"status"`
	TrustScore            float64   `json:"trustScore"`
	CapabilityViolations  int       `json:"capabilityViolations"`
	DaysInactive          int       `json:"daysInactive"`
	LastActivity          string    `json:"lastActivity"`
	CreatedAt             string    `json:"createdAt"`
	VerifiedAt            string    `json:"verifiedAt,omitempty"`
	RiskLevel             string    `json:"riskLevel"`            // "low", "medium", "high", "critical"
	ComplianceIssues      []string  `json:"complianceIssues"`     // List of specific issues
}

// UserAccessReview provides user access audit information
type UserAccessReview struct {
	TotalUsers       int                    `json:"totalUsers"`
	AdminUsers       int                    `json:"adminUsers"`
	ManagerUsers     int                    `json:"managerUsers"`
	MemberUsers      int                    `json:"memberUsers"`
	ViewerUsers      int                    `json:"viewerUsers"`
	ActiveUsers      int                    `json:"activeUsers"`
	InactiveUsers    int                    `json:"inactiveUsers"`      // No login in 30+ days
	PendingUsers     int                    `json:"pendingUsers"`
	Users            []UserAccessDetail     `json:"users"`
}

// UserAccessDetail provides detailed access info for a single user
type UserAccessDetail struct {
	ID              string   `json:"id"`
	Email           string   `json:"email"`
	Name            string   `json:"name"`
	Role            string   `json:"role"`
	Status          string   `json:"status"`
	LastLogin       string   `json:"lastLogin,omitempty"`
	DaysSinceLogin  int      `json:"daysSinceLogin"`
	CreatedAt       string   `json:"createdAt"`
	AccessLevel     string   `json:"accessLevel"`
	ReviewRequired  bool     `json:"reviewRequired"`       // True if inactive 30+ days
}

// AuditActivityReport provides audit log analysis
type AuditActivityReport struct {
	TotalActions        int                    `json:"totalActions"`
	UniqueUsers         int                    `json:"uniqueUsers"`
	ActionsLast24h      int                    `json:"actionsLast24h"`
	ActionsLast7d       int                    `json:"actionsLast7d"`
	ActionsLast30d      int                    `json:"actionsLast30d"`
	ActionBreakdown     map[string]int         `json:"actionBreakdown"`
	ResourceBreakdown   map[string]int         `json:"resourceBreakdown"`
	TopUsers            []UserActivitySummary  `json:"topUsers"`
	RecentActions       []AuditLogEntry        `json:"recentActions"`     // Last 50 actions
}

// UserActivitySummary summarizes a user's audit activity
type UserActivitySummary struct {
	UserID          string `json:"userId"`
	UserEmail       string `json:"userEmail"`
	ActionCount     int    `json:"actionCount"`
	LastAction      string `json:"lastAction"`
	LastActionTime  string `json:"lastActionTime"`
}

// AuditLogEntry represents a single audit log entry for export
type AuditLogEntry struct {
	ID           string `json:"id"`
	Action       string `json:"action"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	UserID       string `json:"userId"`
	UserEmail    string `json:"userEmail,omitempty"`
	IPAddress    string `json:"ipAddress"`
	Timestamp    string `json:"timestamp"`
}

// SecurityAlertsReport provides security alerts analysis
type SecurityAlertsReport struct {
	TotalAlerts         int                    `json:"totalAlerts"`
	CriticalAlerts      int                    `json:"criticalAlerts"`
	HighAlerts          int                    `json:"highAlerts"`
	MediumAlerts        int                    `json:"mediumAlerts"`
	LowAlerts           int                    `json:"lowAlerts"`
	UnacknowledgedCount int                    `json:"unacknowledgedCount"`
	AcknowledgedCount   int                    `json:"acknowledgedCount"`
	AlertsByType        map[string]int         `json:"alertsByType"`
	RecentAlerts        []AlertEntry           `json:"recentAlerts"`      // Last 20 alerts
}

// AlertEntry represents a single alert for export
type AlertEntry struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Severity       string `json:"severity"`
	AlertType      string `json:"alertType"`
	ResourceType   string `json:"resourceType,omitempty"`
	ResourceID     string `json:"resourceId,omitempty"`
	IsAcknowledged bool   `json:"isAcknowledged"`
	CreatedAt      string `json:"createdAt"`
	AcknowledgedAt string `json:"acknowledgedAt,omitempty"`
}

// RiskAssessmentReport provides risk analysis
type RiskAssessmentReport struct {
	OverallRiskLevel    string               `json:"overallRiskLevel"`  // "low", "medium", "high", "critical"
	RiskScore           float64              `json:"riskScore"`         // 0-100 (lower is better)
	RiskDistribution    RiskDistribution     `json:"riskDistribution"`
	HighRiskItems       []RiskItem           `json:"highRiskItems"`
	RiskFactors         []RiskFactor         `json:"riskFactors"`
}

// RiskDistribution shows the distribution of risk levels
type RiskDistribution struct {
	LowRisk      int `json:"lowRisk"`      // Agents with trust >= 80%
	MediumRisk   int `json:"mediumRisk"`   // Agents with trust 60-79%
	HighRisk     int `json:"highRisk"`     // Agents with trust 40-59%
	CriticalRisk int `json:"criticalRisk"` // Agents with trust < 40%
}

// RiskItem represents a high-risk item
type RiskItem struct {
	Type         string `json:"type"`         // "agent", "user", "api_key"
	ID           string `json:"id"`
	Name         string `json:"name"`
	RiskLevel    string `json:"riskLevel"`
	RiskScore    float64 `json:"riskScore"`
	Issue        string `json:"issue"`
	Mitigation   string `json:"mitigation"`
}

// RiskFactor represents a contributing risk factor
type RiskFactor struct {
	Factor       string  `json:"factor"`
	Impact       string  `json:"impact"`       // "high", "medium", "low"
	Description  string  `json:"description"`
	Contribution float64 `json:"contribution"` // Percentage contribution to overall risk
}

// RecommendationItem represents an actionable recommendation
type RecommendationItem struct {
	Priority     string `json:"priority"`     // "critical", "high", "medium", "low"
	Category     string `json:"category"`     // "security", "operations", "compliance"
	Title        string `json:"title"`
	Description  string `json:"description"`
	Impact       string `json:"impact"`
	ActionURL    string `json:"actionUrl,omitempty"`
}

// ReportPeriod represents the time period for a report
type ReportPeriod struct {
	StartDate time.Time `json:"startDate"`
	EndDate   time.Time `json:"endDate"`
}
