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
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	ThreatType     ThreatType `json:"threat_type"`
	Severity       AlertSeverity `json:"severity"`
	Title          string     `json:"title"`
	Description    string     `json:"description"`
	Source         string     `json:"source"` // IP address, agent ID, etc.
	TargetType     string     `json:"target_type"` // "agent", "user", "api_key"
	TargetID       uuid.UUID  `json:"target_id"`
	TargetName     *string    `json:"target_name"` // Agent or MCP server name (joined from agents/mcp_servers table)
	IsBlocked      bool       `json:"is_blocked"`
	CreatedAt      time.Time  `json:"created_at"`
	ResolvedAt     *time.Time `json:"resolved_at"`
}

// Anomaly represents a detected anomaly
type Anomaly struct {
	ID             uuid.UUID   `json:"id"`
	OrganizationID uuid.UUID   `json:"organization_id"`
	AnomalyType    AnomalyType `json:"anomaly_type"`
	Severity       AlertSeverity `json:"severity"`
	Title          string      `json:"title"`
	Description    string      `json:"description"`
	ResourceType   string      `json:"resource_type"`
	ResourceID     uuid.UUID   `json:"resource_id"`
	Confidence     float64     `json:"confidence"` // 0-100
	CreatedAt      time.Time   `json:"created_at"`
}

// SecurityIncident represents a security incident
type SecurityIncident struct {
	ID             uuid.UUID      `json:"id"`
	OrganizationID uuid.UUID      `json:"organization_id"`
	IncidentType   string         `json:"incident_type"`
	Status         IncidentStatus `json:"status"`
	Severity       AlertSeverity  `json:"severity"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	AffectedResources []string    `json:"affected_resources"`
	AssignedTo     *uuid.UUID     `json:"assigned_to"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	ResolvedAt     *time.Time     `json:"resolved_at"`
	ResolvedBy     *uuid.UUID     `json:"resolved_by"`
	ResolutionNotes string        `json:"resolution_notes"`
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
	TotalThreats          int                     `json:"total_threats"`
	ActiveThreats         int                     `json:"active_threats"`
	BlockedThreats        int                     `json:"blocked_threats"`
	TotalAnomalies        int                     `json:"total_anomalies"`
	HighSeverityCount     int                     `json:"high_severity_count"`
	OpenIncidents         int                     `json:"open_incidents"`
	AverageTrustScore     float64                 `json:"average_trust_score"`
	SecurityScore         float64                 `json:"security_score"` // 0-100
	ThreatTrend           []ThreatTrendData       `json:"threat_trend"`
	SeverityDistribution  []SeverityDistribution  `json:"severity_distribution"`
}

// SecurityScanResult represents the result of a security scan
type SecurityScanResult struct {
	ScanID            uuid.UUID  `json:"scan_id"`
	OrganizationID    uuid.UUID  `json:"organization_id"`
	ScanType          string     `json:"scan_type"`
	Status            string     `json:"status"`
	ThreatsFound      int        `json:"threats_found"`
	AnomaliesFound    int        `json:"anomalies_found"`
	VulnerabilitiesFound int     `json:"vulnerabilities_found"`
	SecurityScore     float64    `json:"security_score"`
	StartedAt         time.Time  `json:"started_at"`
	CompletedAt       *time.Time `json:"completed_at"`
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
