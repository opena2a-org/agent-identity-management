package domain

import (
	"time"

	"github.com/google/uuid"
)

// ThreatType represents the type of security threat
type ThreatType string

const (
	ThreatTypeUnauthorizedAccess    ThreatType = "unauthorized_access"
	ThreatTypeBruteForce           ThreatType = "brute_force"
	ThreatTypeSuspiciousActivity   ThreatType = "suspicious_activity"
	ThreatTypeDataExfiltration     ThreatType = "data_exfiltration"
	ThreatTypeMaliciousAgent       ThreatType = "malicious_agent"
	ThreatTypeCredentialLeak       ThreatType = "credential_leak"
)

// AnomalyType represents the type of anomaly detected
type AnomalyType string

const (
	AnomalyTypeUnusualAPIUsage      AnomalyType = "unusual_api_usage"
	AnomalyTypeAbnormalTraffic      AnomalyType = "abnormal_traffic"
	AnomalyTypeUnexpectedLocation   AnomalyType = "unexpected_location"
	AnomalyTypeRateLimitViolation   AnomalyType = "rate_limit_violation"
	AnomalyTypeUnusualAccessPattern AnomalyType = "unusual_access_pattern"
)

// IncidentStatus represents the status of a security incident
type IncidentStatus string

const (
	IncidentStatusOpen       IncidentStatus = "open"
	IncidentStatusInvestigating IncidentStatus = "investigating"
	IncidentStatusResolved   IncidentStatus = "resolved"
	IncidentStatusFalsePositive IncidentStatus = "false_positive"
)

// Threat represents a detected security threat
type Threat struct {
	ID             uuid.UUID     `json:"id"`
	OrganizationID uuid.UUID     `json:"organizationId"`
	ThreatType     ThreatType    `json:"threatType"`
	Severity       AlertSeverity `json:"severity"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	Source         string        `json:"source"`     // IP address, agent ID, etc.
	TargetType     string        `json:"targetType"` // "agent", "user", "api_key"
	TargetID       uuid.UUID     `json:"targetId"`
	TargetName     *string       `json:"targetName"` // Agent or MCP server name (joined from agents/mcp_servers table)
	IsBlocked      bool          `json:"isBlocked"`
	CreatedAt      time.Time     `json:"createdAt"`
	ResolvedAt     *time.Time    `json:"resolvedAt"`
}

// Anomaly represents a detected anomaly
type Anomaly struct {
	ID             uuid.UUID     `json:"id"`
	OrganizationID uuid.UUID     `json:"organizationId"`
	AnomalyType    AnomalyType   `json:"anomalyType"`
	Severity       AlertSeverity `json:"severity"`
	Title          string        `json:"title"`
	Description    string        `json:"description"`
	ResourceType   string        `json:"resourceType"`
	ResourceID     uuid.UUID     `json:"resourceId"`
	Confidence     float64       `json:"confidence"` // 0-100
	CreatedAt      time.Time     `json:"createdAt"`
}

// SecurityIncident represents a security incident
type SecurityIncident struct {
	ID                uuid.UUID      `json:"id"`
	OrganizationID    uuid.UUID      `json:"organizationId"`
	IncidentType      string         `json:"incidentType"`
	Status            IncidentStatus `json:"status"`
	Severity          AlertSeverity  `json:"severity"`
	Title             string         `json:"title"`
	Description       string         `json:"description"`
	AffectedResources []string       `json:"affectedResources"`
	AssignedTo        *uuid.UUID     `json:"assignedTo"`
	CreatedAt         time.Time      `json:"createdAt"`
	UpdatedAt         time.Time      `json:"updatedAt"`
	ResolvedAt        *time.Time     `json:"resolvedAt"`
	ResolvedBy        *uuid.UUID     `json:"resolvedBy"`
	ResolutionNotes   string         `json:"resolutionNotes"`
}

// ThreatTrendData represents threat count by date
type ThreatTrendData struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

// SeverityDistribution represents threat count by severity
type SeverityDistribution struct {
	Severity string `json:"severity"`
	Count    int    `json:"count"`
}

// SecurityMetrics represents overall security metrics
type SecurityMetrics struct {
	TotalThreats         int                    `json:"totalThreats"`
	ActiveThreats        int                    `json:"activeThreats"`
	BlockedThreats       int                    `json:"blockedThreats"`
	TotalAnomalies       int                    `json:"totalAnomalies"`
	HighSeverityCount    int                    `json:"highSeverityCount"`
	OpenIncidents        int                    `json:"openIncidents"`
	AverageTrustScore    float64                `json:"averageTrustScore"`
	SecurityScore        float64                `json:"securityScore"` // 0-100
	ThreatTrend          []ThreatTrendData      `json:"threatTrend"`
	SeverityDistribution []SeverityDistribution `json:"severityDistribution"`
}

// SecurityScanResult represents the result of a security scan
type SecurityScanResult struct {
	ScanID               uuid.UUID  `json:"scanId"`
	OrganizationID       uuid.UUID  `json:"organizationId"`
	ScanType             string     `json:"scanType"`
	Status               string     `json:"status"`
	ThreatsFound         int        `json:"threatsFound"`
	AnomaliesFound       int        `json:"anomaliesFound"`
	VulnerabilitiesFound int        `json:"vulnerabilitiesFound"`
	SecurityScore        float64    `json:"securityScore"`
	StartedAt            time.Time  `json:"startedAt"`
	CompletedAt          *time.Time `json:"completedAt"`
}

// SecurityRepository defines the interface for security persistence
type SecurityRepository interface {
	// Threats
	CreateThreat(threat *Threat) error
	GetThreats(orgID uuid.UUID, limit, offset int) ([]*Threat, error)
	GetThreatByID(id uuid.UUID) (*Threat, error)
	BlockThreat(id uuid.UUID) error
	ResolveThreat(id uuid.UUID) error

	// Anomalies
	CreateAnomaly(anomaly *Anomaly) error
	GetAnomalies(orgID uuid.UUID, limit, offset int) ([]*Anomaly, error)
	GetAnomalyByID(id uuid.UUID) (*Anomaly, error)

	// Incidents
	CreateIncident(incident *SecurityIncident) error
	GetIncidents(orgID uuid.UUID, status IncidentStatus, limit, offset int) ([]*SecurityIncident, error)
	GetIncidentByID(id uuid.UUID) (*SecurityIncident, error)
	UpdateIncidentStatus(id uuid.UUID, status IncidentStatus, resolvedBy *uuid.UUID, notes string) error

	// Metrics
	GetSecurityMetrics(orgID uuid.UUID) (*SecurityMetrics, error)

	// Scans
	CreateSecurityScan(scan *SecurityScanResult) error
	GetSecurityScan(scanID uuid.UUID) (*SecurityScanResult, error)
}
